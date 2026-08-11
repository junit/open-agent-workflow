package codexbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestCodexBridgeConformanceTranscriptVerifiesDeclaredFeatures(t *testing.T) {
	service, facts := observedConformanceService(t)
	receipt := completedCurrentReceipt(t, facts.Session, facts.Environment)
	transcript, err := BuildConformanceTranscript(facts, []host.InvocationReceipt{receipt})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := CodexHostManifest()
	if err != nil {
		t.Fatal(err)
	}
	report, err := host.ValidateConformanceTranscript(manifest, transcript)
	if err != nil {
		t.Fatal(err)
	}
	want := []host.Feature{
		host.FeatureEnvironmentReporting,
		host.FeatureNormalizedReceipts,
		host.FeatureProviderBindingInventory,
	}
	if transcript.SchemaVersion != host.HostConformanceTranscriptSchemaV4 || report.SchemaVersion != host.HostConformanceReportSchemaV4 ||
		report.TranscriptDigest != transcript.Digest || !slices.Equal(report.VerifiedFeatures, want) || len(report.Diagnostics) != 0 {
		t.Fatalf("report = %#v", report)
	}
	if service == nil || len(facts.Inventory.Observations) != 1 || facts.Inventory.Observations[0].Reference != "acme:delivery" {
		t.Fatalf("Bridge facts = %#v", facts)
	}
}

func TestCodexBridgeConformanceTranscriptAssetIsCanonical(t *testing.T) {
	assetPath := filepath.Join("..", "assets", "conformance", "codex-host-v3.json")
	raw, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{
		[]byte("/Users/"), []byte("/home/"), []byte("oawh1."), []byte("credential"), []byte(`"command"`),
	} {
		if bytes.Contains(raw, forbidden) {
			t.Fatalf("conformance asset contains forbidden data %q", forbidden)
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var stored host.ConformanceTranscript
	if err := decoder.Decode(&stored); err != nil {
		t.Fatal(err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatalf("conformance asset has trailing JSON: %v", err)
	}
	rebuilt, err := host.NewConformanceTranscript(stored)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, canonicalTranscriptBytes(t, rebuilt)) {
		t.Fatal("conformance asset is not canonical or contains stale digests")
	}
	manifest, err := CodexHostManifest()
	if err != nil {
		t.Fatal(err)
	}
	report, err := host.ValidateConformanceTranscript(manifest, rebuilt)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(report.VerifiedFeatures, manifest.Features) || len(report.Diagnostics) != 0 {
		t.Fatalf("asset conformance report = %#v", report)
	}
}

func observedConformanceService(t *testing.T) (*Service, Facts) {
	t.Helper()
	projectRoot := t.TempDir()
	userConfigRoot := t.TempDir()
	userHome := t.TempDir()
	providerRoot := filepath.Join(userHome, ".codex", "plugins", "acme")
	skillPath := filepath.Join(providerRoot, "skills", "delivery", "SKILL.md")
	writeServiceFixtureFile(t, filepath.Join(providerRoot, "marker.txt"), "provider-evidence")

	observer := appserver.NewClient(appserver.ClientOptions{
		Transport: &conformanceTransport{t: t, cwd: projectRoot, skillPath: skillPath},
	})
	store := NewEvidenceStore(CacheOptions{MaximumEntries: 2, Random: strings.NewReader(strings.Repeat("c", 64))})
	service, err := NewService(ServiceOptions{
		Observer: observer, Store: store, StateRoot: t.TempDir(), ProjectRoot: projectRoot,
		UserConfigRoot: userConfigRoot, UserHome: userHome, BridgeVersion: BridgeIntegrationVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	installUserProvider(t, service)
	hostContext := testHookContext("session-codex-conformance", projectRoot)
	observed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, hostContext)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := store.Get(observed.HostEvidenceHandle)
	if err != nil {
		t.Fatal(err)
	}
	return service, facts
}

func completedCurrentReceipt(t *testing.T, session host.SessionSnapshot, environment host.EnvironmentReport) host.InvocationReceipt {
	t.Helper()
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion:           host.HostInvocationReceiptSchemaV3,
		Kind:                    host.ReceiptCompleted,
		WorkflowID:              "workflow-0123456789abcdef0123456789abcdef",
		BundleID:                "bundle-0123456789abcdef0123456789abcdef",
		BundleGeneration:        1,
		BundleDigest:            canonicaljson.DigestBytes([]byte("codex-host-conformance-bundle-v1")),
		Cursor:                  execution.GraphCursor{SlotID: "fresh-verification", Kind: execution.CursorBinding, UnitID: "verification", Ordinal: 1},
		Topology:                execution.TopologyCurrent,
		HostSessionDigest:       session.Digest,
		DispatchDigest:          canonicaljson.DigestBytes([]byte("codex-host-conformance-dispatch-v1")),
		ContextFreshness:        host.ContextShared,
		EnvironmentReportDigest: environment.Digest,
		Outcome:                 "succeeded",
		Outputs: []host.OutputReference{{
			ArtifactID: "artifact-verification", Schema: "oaw.workflow-artifact/v1",
			Reference: "evidence://codex-host/outputs/current-completion", Digest: canonicaljson.DigestBytes([]byte("codex-host-current-output-v1")),
		}},
		Evidence: []host.EvidenceReference{{
			Kind:      "report",
			Reference: "evidence://codex-host/conformance/current-completion",
			Digest:    canonicaljson.DigestBytes([]byte("codex-host-current-completion-v1")),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func canonicalTranscriptBytes(t *testing.T, transcript host.ConformanceTranscript) []byte {
	t.Helper()
	raw, err := canonicaljson.Marshal(transcript)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

type conformanceTransport struct {
	t         *testing.T
	cwd       string
	skillPath string
}

func (transport *conformanceTransport) Exchange(_ context.Context, requestBytes []byte, _ int) ([]byte, error) {
	transport.t.Helper()
	var request appserver.Request
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		transport.t.Fatal(err)
	}
	var result any
	switch request.Method {
	case "initialize":
		result = map[string]any{"userAgent": "codex-cli/0.146.1"}
	case "skills/list":
		result = map[string]any{"data": []any{map[string]any{
			"cwd": transport.cwd, "errors": []any{},
			"skills": []any{map[string]any{"name": "acme:delivery", "enabled": true, "path": transport.skillPath, "scope": "user"}},
		}}}
	case "hooks/list":
		result = map[string]any{"data": []any{map[string]any{
			"cwd": transport.cwd, "errors": []any{}, "warnings": []any{}, "hooks": []any{},
		}}}
	case "config/read":
		result = map[string]any{
			"config":  map[string]any{"features": map[string]any{"hooks": true}, "mcp_servers": map[string]any{}, "approval_policy": "ask", "sandbox_mode": "workspace-write"},
			"origins": map[string]any{},
		}
	default:
		transport.t.Fatalf("unexpected App Server method %q", request.Method)
	}
	rawResult, err := json.Marshal(result)
	if err != nil {
		transport.t.Fatal(err)
	}
	response, err := json.Marshal(appserver.Response{JSONRPC: "2.0", ID: request.ID, Result: rawResult})
	if err != nil {
		transport.t.Fatal(err)
	}
	return response, nil
}

func (transport *conformanceTransport) Notify(_ context.Context, _ []byte) error { return nil }
func (transport *conformanceTransport) Close() error                             { return nil }
