package codexbridge

import (
	"bytes"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestEvidenceStoreBindsHandleToSessionAndCWD(t *testing.T) {
	store := NewEvidenceStore(CacheOptions{
		Now: fixedTime, TTL: time.Minute, MaximumEntries: 2,
		Random: bytes.NewReader(bytes.Repeat([]byte{0x42}, 64)),
	})
	facts := testFacts(t, "session-a", "/repo/a")
	handle, err := store.Put(testContext("session-a", "/repo/a"), facts)
	if err != nil {
		t.Fatal(err)
	}
	foreignSession := handle
	foreignSession.SessionDigest = digestSessionID("session-b")
	if _, err := store.Get(foreignSession); Code(err) != "HOST_EVIDENCE_SESSION_MISMATCH" {
		t.Fatalf("cross-session header error = %v", err)
	}
	foreignCWD := handle
	foreignCWD.CWDDigest = digestCWD("/repo/b")
	if _, err := store.Get(foreignCWD); Code(err) != "HOST_EVIDENCE_SESSION_MISMATCH" {
		t.Fatalf("cross-cwd header error = %v", err)
	}
}

func TestEvidenceStoreExpiresAndEvictsDeterministically(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	store := NewEvidenceStore(CacheOptions{
		Now: func() time.Time { return now }, TTL: time.Minute, MaximumEntries: 2,
		Random: bytes.NewReader(bytes.Repeat([]byte{0x41}, 128)),
	})
	context := testContext("session-a", "/repo/a")
	first := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	second := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	if _, err := store.Get(first); err != nil {
		t.Fatal(err)
	}
	third := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	if _, err := store.Get(second); Code(err) != "HOST_EVIDENCE_HANDLE_INVALID" {
		t.Fatalf("evicted handle error = %v", err)
	}
	if _, err := store.Get(first); err != nil {
		t.Fatalf("first handle was evicted: %v", err)
	}
	if _, err := store.Get(third); err != nil {
		t.Fatalf("third handle unavailable: %v", err)
	}
	now = now.Add(time.Minute)
	if _, err := store.Get(first); Code(err) != "HOST_EVIDENCE_EXPIRED" {
		t.Fatalf("expired handle error = %v", err)
	}
}

func TestEvidenceStoreResetInvalidatesAllHandles(t *testing.T) {
	store := NewEvidenceStore(CacheOptions{
		Now: fixedTime, TTL: time.Minute, MaximumEntries: 2,
		Random: bytes.NewReader(bytes.Repeat([]byte{0x42}, 64)),
	})
	context := testContext("session-a", "/repo/a")
	handle := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	store.Reset()
	if _, err := store.Get(handle); Code(err) != "HOST_EVIDENCE_HANDLE_INVALID" {
		t.Fatalf("reset handle error = %v", err)
	}
}

func TestEvidenceStoreIssuesDistinctRandomTokens(t *testing.T) {
	store := NewEvidenceStore(CacheOptions{Now: fixedTime, TTL: time.Minute, MaximumEntries: 2, Random: rand.Reader})
	context := testContext("session-a", "/repo/a")
	first := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	second := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	if first.Token == second.Token {
		t.Fatal("two handles reused the same random token")
	}
}

func TestEvidenceStoreReturnsDefensiveFactCopies(t *testing.T) {
	store := NewEvidenceStore(CacheOptions{Now: fixedTime, TTL: time.Minute, MaximumEntries: 2, Random: rand.Reader})
	context := testContext("session-a", "/repo/a")
	handle := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	value, err := store.Get(handle)
	if err != nil {
		t.Fatal(err)
	}
	value.Session.SupportedTopologies[0] = execution.TopologySubagent
	value.Inventory.Observations = append(value.Inventory.Observations, host.BindingObservation{})
	value.Environment.Observations = append(value.Environment.Observations, execution.EnvironmentObservation{})
	valueAgain, err := store.Get(handle)
	if err != nil {
		t.Fatal(err)
	}
	if valueAgain.Session.SupportedTopologies[0] != execution.TopologyCurrent || len(valueAgain.Inventory.Observations) != 0 || len(valueAgain.Environment.Observations) != 0 {
		t.Fatalf("stored facts were mutated through returned copy: %#v", valueAgain)
	}
}

func TestEvidenceStoreRejectsMismatchedFactDigest(t *testing.T) {
	store := NewEvidenceStore(CacheOptions{Now: fixedTime, TTL: time.Minute, MaximumEntries: 2, Random: rand.Reader})
	facts := testFacts(t, "session-a", "/repo/a")
	facts.FactDigests.Session = strings.Repeat("f", 64)
	if _, err := store.Put(testContext("session-a", "/repo/a"), facts); Code(err) != "HOST_EVIDENCE_HANDLE_INVALID" {
		t.Fatalf("mismatched fact digest error = %v", err)
	}
}

func TestEvidenceStoreRejectsTimeBeforeIssuance(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	store := NewEvidenceStore(CacheOptions{
		Now: func() time.Time { return now }, TTL: time.Minute, MaximumEntries: 2, Random: rand.Reader,
	})
	handle := mustPut(t, store, testContext("session-a", "/repo/a"), testFacts(t, "session-a", "/repo/a"))
	now = now.Add(-time.Second)
	if _, err := store.Get(handle); Code(err) != "HOST_EVIDENCE_EXPIRED" {
		t.Fatalf("pre-issuance handle error = %v", err)
	}
}

func TestEvidenceStoreClampsTTLAndCapacity(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	store := NewEvidenceStore(CacheOptions{
		Now: func() time.Time { return now }, TTL: time.Second, MaximumEntries: 0,
		Random: bytes.NewReader(bytes.Repeat([]byte{0x42}, 64)),
	})
	context := testContext("session-a", "/repo/a")
	first := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	second := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	if _, err := store.Get(first); Code(err) != "HOST_EVIDENCE_HANDLE_INVALID" {
		t.Fatalf("minimum capacity was not clamped: %v", err)
	}
	now = now.Add(30 * time.Second)
	if _, err := store.Get(second); Code(err) != "HOST_EVIDENCE_EXPIRED" {
		t.Fatalf("minimum TTL was not clamped: %v", err)
	}

	now = time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	longStore := NewEvidenceStore(CacheOptions{Now: func() time.Time { return now }, TTL: 24 * time.Hour, MaximumEntries: 1, Random: rand.Reader})
	handle := mustPut(t, longStore, context, testFacts(t, "session-a", "/repo/a"))
	now = now.Add(15 * time.Minute)
	if _, err := longStore.Get(handle); Code(err) != "HOST_EVIDENCE_EXPIRED" {
		t.Fatalf("maximum TTL was not clamped: %v", err)
	}
}

func TestEvidenceStoreRejectsInvalidContextFactsAndRandomSource(t *testing.T) {
	store := NewEvidenceStore(CacheOptions{Now: fixedTime, TTL: time.Minute, MaximumEntries: 2, Random: rand.Reader})
	facts := testFacts(t, "session-a", "/repo/a")
	invalidSchema := testContext("session-a", "/repo/a")
	invalidSchema.SchemaVersion = "other"
	if _, err := store.Put(invalidSchema, facts); Code(err) != "HOST_BRIDGE_CONTEXT_REQUIRED" {
		t.Fatalf("schema error=%v", err)
	}
	if _, err := store.Put(testContext("session-a", "relative"), facts); Code(err) != "HOST_BRIDGE_CONTEXT_REQUIRED" {
		t.Fatalf("cwd error=%v", err)
	}
	if _, err := store.Put(testContext("other-session", "/repo/a"), facts); Code(err) != "HOST_EVIDENCE_SESSION_MISMATCH" {
		t.Fatalf("session error=%v", err)
	}
	if _, err := store.Put(testContext("session-a", "/repo/a"), Facts{}); Code(err) != "HOST_EVIDENCE_SESSION_MISMATCH" {
		t.Fatalf("empty facts error=%v", err)
	}

	brokenRandom := NewEvidenceStore(CacheOptions{Now: fixedTime, TTL: time.Minute, MaximumEntries: 2, Random: bytes.NewReader(nil)})
	if _, err := brokenRandom.Put(testContext("session-a", "/repo/a"), facts); Code(err) != "HOST_EVIDENCE_HANDLE_INVALID" {
		t.Fatalf("random source error=%v", err)
	}
}

func TestValidateHandleContextRequiresExactHeadersAndVersions(t *testing.T) {
	context := testContext("session-a", "/repo/a")
	sessionDigest, cwdDigest, err := ContextDigestHeaders(context)
	if err != nil {
		t.Fatal(err)
	}
	handle := HostEvidenceHandle{Version: EvidenceHandleVersion, SessionDigest: sessionDigest, CWDDigest: cwdDigest, Token: strings.Repeat("h", 22)}
	if err := ValidateHandleContext(handle, context); err != nil {
		t.Fatal(err)
	}
	wrongVersion := handle
	wrongVersion.Version = "other"
	if err := ValidateHandleContext(wrongVersion, context); Code(err) != "HOST_EVIDENCE_SESSION_MISMATCH" {
		t.Fatalf("version error=%v", err)
	}
	wrongSchema := context
	wrongSchema.BridgeProtocolVersion = "other"
	if err := ValidateHandleContext(handle, wrongSchema); Code(err) != "HOST_BRIDGE_CONTEXT_REQUIRED" {
		t.Fatalf("schema error=%v", err)
	}
}

func mustPut(t *testing.T, store EvidenceStore, context HookContext, facts Facts) HostEvidenceHandle {
	t.Helper()
	handle, err := store.Put(context, facts)
	if err != nil {
		t.Fatal(err)
	}
	return handle
}

func testContext(sessionID, cwd string) HookContext {
	return HookContext{
		SchemaVersion: HookContextSchemaV1, BridgeProtocolVersion: BridgeProtocolVersion,
		SessionID: sessionID, TurnID: "turn-1", ToolUseID: "tool-1", CWD: cwd,
		Model: "gpt-test", PermissionMode: "default",
	}
}

func testFacts(t *testing.T, sessionID, _ string) Facts {
	t.Helper()
	manifest, err := CodexHostManifest()
	if err != nil {
		t.Fatal(err)
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: sessionID,
		Topology: execution.TopologyCurrent, Observations: []execution.EnvironmentObservation{},
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := host.NewBindingInventory("codex", nil)
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV2, HostID: "codex", IntegrationID: BridgeIntegrationID,
		IntegrationVersion: BridgeIntegrationVersion, SessionID: sessionID,
		SupportedTopologies:     []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: inventory.Digest, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	return Facts{
		Session: session, Inventory: inventory, Environment: environment,
		FactDigests: FactDigests{Session: session.Digest, Inventory: inventory.Digest, Environment: environment.Digest},
	}
}

func digestSessionID(value string) string {
	digest, _, err := ContextDigestHeaders(HookContext{SessionID: value, CWD: "/repo/a"})
	if err != nil {
		panic(err)
	}
	return digest
}

func digestCWD(value string) string {
	_, digest, err := ContextDigestHeaders(HookContext{SessionID: "session-a", CWD: value})
	if err != nil {
		panic(err)
	}
	return digest
}

var fixedTime = func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) }
