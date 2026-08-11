package hosttest_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/hosttest"
)

func TestCurrentSessionFixtureIsSecretFree(t *testing.T) {
	session := hosttest.CurrentSession(t, "codex-test", strings.Repeat("a", 64))
	environment := hosttest.CurrentEnvironment(t, session)
	if err := host.ValidateEnvironmentReport(session, environment); err != nil {
		t.Fatalf("CurrentEnvironment() is not pinned to CurrentSession(): %v", err)
	}
	if session.SchemaVersion != host.HostSessionSchemaV3 || session.HostID != "codex-test" || session.SupportedTopologies[0] != execution.TopologyCurrent || session.FeatureDigest == "" || session.HostActionDigest == "" {
		t.Fatalf("CurrentSession() = %#v", session)
	}

	encoded, err := json.Marshal(struct {
		Session     host.SessionSnapshot
		Environment host.EnvironmentReport
	}{Session: session, Environment: environment})
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(encoded))
	for _, forbidden := range []string{
		"token", "password", "authorization", "api_key", "mcp_servers", "mcpservers",
		"codex exec", "claude --", "gemini --", "opencode --",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("CurrentSession fixture contains forbidden secret or command marker %q: %s", forbidden, encoded)
		}
	}
}

func TestCompletedReceiptPinsDispatchIdentity(t *testing.T) {
	evidence := []host.EvidenceReference{{
		Kind: "report", Reference: "evidence://hosttest/completed", Digest: strings.Repeat("e", 64),
	}}
	identity := hosttest.ReceiptIdentity{
		WorkflowID:              "workflow-0123456789abcdef0123456789abcdef",
		BundleID:                "bundle-0123456789abcdef0123456789abcdef",
		BundleGeneration:        3,
		BundleDigest:            strings.Repeat("b", 64),
		Cursor:                  execution.GraphCursor{SlotID: "fresh-verification", Kind: execution.CursorBinding, UnitID: "verification", Ordinal: 1},
		Topology:                execution.TopologyCurrent,
		HostSessionDigest:       strings.Repeat("c", 64),
		DispatchDigest:          strings.Repeat("d", 64),
		EnvironmentReportDigest: strings.Repeat("e", 64),
	}
	outputs := []host.OutputReference{{ArtifactID: "artifact-hosttest", Schema: "oaw.workflow-artifact/v1", Reference: "evidence://hosttest/output", Digest: strings.Repeat("d", 64)}}
	receipt := hosttest.CompletedReceipt(t, identity, "", outputs, evidence)
	if receipt.WorkflowID != identity.WorkflowID || receipt.BundleGeneration != identity.BundleGeneration ||
		receipt.BundleID != identity.BundleID || receipt.BundleDigest != identity.BundleDigest || receipt.Cursor != identity.Cursor ||
		receipt.Topology != identity.Topology || receipt.HostSessionDigest != identity.HostSessionDigest ||
		receipt.DispatchDigest != identity.DispatchDigest || receipt.EnvironmentReportDigest != identity.EnvironmentReportDigest {
		t.Fatalf("CompletedReceipt() lost dispatch identity: %#v", receipt)
	}
	if receipt.Kind != host.ReceiptCompleted || receipt.ContextFreshness != host.ContextShared || receipt.Outcome != "succeeded" || receipt.Digest == "" {
		t.Fatalf("CompletedReceipt() = %#v", receipt)
	}
	if len(receipt.Outputs) != 1 || receipt.Outputs[0] != outputs[0] || len(receipt.Evidence) != 1 || receipt.Evidence[0] != evidence[0] {
		t.Fatalf("CompletedReceipt() evidence = %#v", receipt.Evidence)
	}
}

func TestHostFixturesReturnDefensiveCopies(t *testing.T) {
	first := hosttest.CurrentSession(t, "codex-test", strings.Repeat("a", 64))
	second := hosttest.CurrentSession(t, "codex-test", strings.Repeat("a", 64))
	first.SupportedTopologies[0] = execution.TopologySubagent
	if second.SupportedTopologies[0] != execution.TopologyCurrent {
		t.Fatal("CurrentSession() shares supported topology storage")
	}

	session := hosttest.CurrentSession(t, "codex-test", strings.Repeat("a", 64))
	firstEnvironment := hosttest.CurrentEnvironment(t, session)
	secondEnvironment := hosttest.CurrentEnvironment(t, session)
	firstEnvironment.Observations = append(firstEnvironment.Observations, execution.EnvironmentObservation{
		Surface: "test-only", Disposition: execution.DispositionUnknown, Source: "hosttest", Digest: strings.Repeat("f", 64),
	})
	if len(secondEnvironment.Observations) != 0 {
		t.Fatal("CurrentEnvironment() shares observation storage")
	}

	identity := hosttest.ReceiptIdentity{
		WorkflowID:              "workflow-0123456789abcdef0123456789abcdef",
		BundleID:                "bundle-0123456789abcdef0123456789abcdef",
		BundleGeneration:        1,
		BundleDigest:            strings.Repeat("b", 64),
		Cursor:                  execution.GraphCursor{SlotID: "implementation", Kind: execution.CursorBinding, UnitID: "implementation", Ordinal: 1},
		Topology:                execution.TopologyCurrent,
		HostSessionDigest:       strings.Repeat("c", 64),
		DispatchDigest:          strings.Repeat("d", 64),
		EnvironmentReportDigest: strings.Repeat("e", 64),
	}
	evidence := []host.EvidenceReference{{Kind: "report", Reference: "evidence://hosttest", Digest: strings.Repeat("e", 64)}}
	outputs := []host.OutputReference{{ArtifactID: "artifact-hosttest", Schema: "oaw.workflow-artifact/v1", Reference: "evidence://hosttest/output", Digest: strings.Repeat("d", 64)}}
	receipt := hosttest.CompletedReceipt(t, identity, "", outputs, evidence)
	outputs[0].Reference = "changed"
	evidence[0].Reference = "changed"
	if receipt.Outputs[0].Reference != "evidence://hosttest/output" || receipt.Evidence[0].Reference != "evidence://hosttest" {
		t.Fatal("CompletedReceipt() shares output or evidence storage")
	}

	started := hosttest.StartedReceipt(t, identity, "")
	if started.Kind != host.ReceiptStarted || started.Outcome != "" || started.Digest == "" {
		t.Fatalf("StartedReceipt() = %#v", started)
	}
	failed := hosttest.FailedReceipt(t, identity, "", "BUILD_FAILED")
	if failed.Kind != host.ReceiptFailed || failed.FailureCode != "BUILD_FAILED" || failed.Digest == "" {
		t.Fatalf("FailedReceipt() = %#v", failed)
	}
}
