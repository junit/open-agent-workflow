package codexbridge

import (
	"bytes"
	"crypto/rand"
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
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "2.0.0", HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1},
		BindingKinds:        []string{"agent", "skill", "tool"},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features:            []host.Feature{host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
	})
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
		SchemaVersion: host.HostSessionSchemaV2, HostID: "codex", IntegrationID: "oaw-test/codex",
		IntegrationVersion: "2.0.0", SessionID: sessionID,
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
