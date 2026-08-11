package dogfood

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestDogfoodRejectsUnsafeRootsAndApproval(t *testing.T) {
	if _, err := canonicalExistingDirectory("relative/repository"); err == nil {
		t.Fatal("relative repository path unexpectedly accepted")
	}
	if _, err := createEvidenceRoot("relative/evidence"); err == nil {
		t.Fatal("relative evidence path unexpectedly accepted")
	}
	repository := makeDogfoodRepository(t, true)
	other := makeDogfoodRepository(t, true)
	evidence := filepath.Join(t.TempDir(), "evidence")
	if err := startPilot(repository, evidence, other, "session-dogfood"); err == nil {
		t.Fatal("mismatched approved repository unexpectedly accepted")
	}
	if _, err := os.Lstat(evidence); !os.IsNotExist(err) {
		t.Fatalf("rejected start left evidence root: %v", err)
	}
}

func TestDogfoodRejectsProductionAndFingerprintChanges(t *testing.T) {
	repository := makeDogfoodRepository(t, true)
	fingerprint, err := inspectRepository(repository)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := integrity.DigestTree(filepath.Dir(fingerprint.SkillPath))
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint.SkillTreeDigest != tree.RootDigest || fingerprint.SkillTreeDigest == fingerprint.SkillDigest {
		t.Fatalf("repository fingerprint does not pin the complete Binding tree: %#v", fingerprint)
	}
	if _, err := verifyRepository(fingerprint); err != nil {
		t.Fatalf("clean repository rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repository, ".oaw-production"), []byte("production\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyRepository(fingerprint); err == nil || !strings.Contains(err.Error(), "production-marked") {
		t.Fatalf("production repository error = %v", err)
	}
	if err := os.Remove(filepath.Join(repository, ".oaw-production")); err != nil {
		t.Fatal(err)
	}
	dirtyPath := filepath.Join(repository, "untracked.txt")
	if err := os.WriteFile(dirtyPath, []byte("dirty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyRepository(fingerprint); err == nil || !strings.Contains(err.Error(), "not clean") {
		t.Fatalf("dirty repository error = %v", err)
	}
	if err := os.Remove(dirtyPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fingerprint.SkillPath, []byte("changed skill\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runTestGit(repository, "add", fingerprint.SkillPath); err != nil {
		t.Fatal(err)
	}
	if err := runTestGit(repository, "commit", "-qm", "change fingerprint"); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyRepository(fingerprint); err == nil || !strings.Contains(err.Error(), "fingerprint changed") {
		t.Fatalf("changed repository error = %v", err)
	}
}

func TestDogfoodRejectsMissingSkillAndSymlinkEvidence(t *testing.T) {
	repository := makeDogfoodRepository(t, false)
	if _, err := inspectRepository(repository); err == nil {
		t.Fatal("repository without required Skill unexpectedly accepted")
	}
	requestedRoot := filepath.Join(t.TempDir(), "evidence")
	root, err := createEvidenceRoot(requestedRoot)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(requestedRoot), "evidence-link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPilot(link); err == nil || !strings.Contains(err.Error(), "regular directory") {
		t.Fatalf("symlinked evidence root error = %v", err)
	}
}

func TestDogfoodValidatesReceiptPinsAndEvidence(t *testing.T) {
	requestedRoot := filepath.Join(t.TempDir(), "evidence")
	root, err := createEvidenceRoot(requestedRoot)
	if err != nil {
		t.Fatal(err)
	}
	node := "review-scope"
	reportPath := filepath.Join(root, "evidence", node+".md")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o700); err != nil {
		t.Fatal(err)
	}
	report := []byte("scope report\n")
	if err := os.WriteFile(reportPath, report, 0o600); err != nil {
		t.Fatal(err)
	}
	packet := coordinator.DispatchPacket{
		SchemaVersion: coordinator.DispatchPacketSchemaV2, ID: "dispatch-11111111111111111111111111111111",
		WorkflowID: "workflow-11111111111111111111111111111111", BundleID: "bundle-11111111111111111111111111111111", BundleGeneration: 1,
		BundleDigest: strings.Repeat("a", 64), Cursor: execution.GraphCursor{SlotID: "review-remediation", Kind: execution.CursorBinding, UnitID: node, Ordinal: 1}, Topology: execution.TopologyCurrent,
		HostSessionDigest: strings.Repeat("b", 64), EnvironmentReportDigest: strings.Repeat("c", 64), Digest: strings.Repeat("d", 64),
	}
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV3, Kind: host.ReceiptCompleted,
		WorkflowID: packet.WorkflowID, BundleID: packet.BundleID, BundleGeneration: packet.BundleGeneration, BundleDigest: packet.BundleDigest,
		Cursor: packet.Cursor, Topology: packet.Topology, HostSessionDigest: packet.HostSessionDigest,
		DispatchDigest: packet.Digest, ContextFreshness: host.ContextShared, EnvironmentReportDigest: packet.EnvironmentReportDigest,
		Outcome: "succeeded", Outputs: []host.OutputReference{{ArtifactID: "artifact-review", Schema: "oaw.workflow-artifact/v1", Reference: "file://" + reportPath, Digest: canonicaljson.DigestBytes(report)}},
		Evidence: []host.EvidenceReference{{Kind: "report", Reference: "file://" + reportPath, Digest: canonicaljson.DigestBytes(report)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateReceiptPins(packet, receipt); err != nil {
		t.Fatalf("valid receipt rejected: %v", err)
	}
	if err := validateEvidence(root, node, receipt.Evidence); err != nil {
		t.Fatalf("valid evidence rejected: %v", err)
	}
	receipt.DispatchDigest = strings.Repeat("e", 64)
	if err := validateReceiptPins(packet, receipt); err == nil {
		t.Fatal("wrong dispatch digest unexpectedly accepted")
	}
	if err := os.WriteFile(reportPath, []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateEvidence(root, node, []host.EvidenceReference{{Kind: "report", Reference: "file://" + reportPath, Digest: canonicaljson.DigestBytes(report)}}); err == nil {
		t.Fatal("changed evidence unexpectedly accepted")
	}
}

func TestDogfoodRejectsMalformedReceipt(t *testing.T) {
	if _, err := decodeReceiptCommand([]byte("{"), "workflow-11111111111111111111111111111111", 1, "review-scope", "succeeded", "scope-complete"); err == nil {
		t.Fatal("malformed Receipt unexpectedly accepted")
	}
}

func TestDogfoodStartsWithProviderV4HostV3AndWorkflowV2(t *testing.T) {
	repository := makeDogfoodRepository(t, true)
	evidence := filepath.Join(t.TempDir(), "evidence")
	if err := startPilot(repository, evidence, repository, "session-dogfood-v4"); err != nil {
		t.Fatalf("startPilot() error = %#v, cause=%v", err, errors.Unwrap(err))
	}
	var result coordinator.Result
	if _, err := readCanonical(filepath.Join(evidence, "start-result.json"), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != coordinator.WorkflowResultSchemaV2 || result.Snapshot == nil || result.Snapshot.SchemaVersion != coordinator.WorkflowSnapshotSchemaV2 {
		t.Fatalf("Workflow Result = %#v", result)
	}
	if len(result.Snapshot.Bundles) != 1 || result.Snapshot.Bundles[0].SchemaVersion != core.LifecycleBundleSchemaV4 || result.Snapshot.Bundles[0].Graph.SchemaVersion != profile.ExecutionGraphSchemaV4 {
		t.Fatalf("Lifecycle Bundle = %#v", result.Snapshot.Bundles)
	}
	if node, err := dogfoodNode(result.Snapshot.Cursor); err != nil || node != "review-scope" {
		t.Fatalf("initial cursor = %#v, node=%q, error=%v", result.Snapshot.Cursor, node, err)
	}
	var session host.SessionSnapshot
	if _, err := readCanonical(filepath.Join(evidence, "session.json"), &session); err != nil {
		t.Fatal(err)
	}
	var inventory host.BindingInventory
	if _, err := readCanonical(filepath.Join(evidence, "inventory.json"), &inventory); err != nil {
		t.Fatal(err)
	}
	if session.SchemaVersion != host.HostSessionSchemaV3 || inventory.SchemaVersion != host.BindingInventorySchemaV3 || len(inventory.Observations) != 3 {
		t.Fatalf("Host evidence = session %#v inventory %#v", session, inventory)
	}
}

func TestDogfoodWorkflowV2ClosesThroughReceiptV3AndUserGate(t *testing.T) {
	repository := makeDogfoodRepository(t, true)
	evidence := filepath.Join(t.TempDir(), "evidence")
	if err := startPilot(repository, evidence, repository, "session-dogfood-closeout"); err != nil {
		t.Fatal(err)
	}
	pilot, err := loadPilot(evidence)
	if err != nil {
		t.Fatal(err)
	}
	evidence = pilot.EvidenceRoot
	for _, node := range []string{"review-scope", "code-review", "verification"} {
		if err := preparePilot(evidence); err != nil {
			t.Fatalf("prepare %s: %v", node, err)
		}
		var packet coordinator.DispatchPacket
		if _, err := readCanonical(filepath.Join(evidence, "dispatch-"+node+".json"), &packet); err != nil {
			t.Fatal(err)
		}
		reportPath := filepath.Join(evidence, "evidence", node+".md")
		if err := os.MkdirAll(filepath.Dir(reportPath), 0o700); err != nil {
			t.Fatal(err)
		}
		report := []byte(node + " report\n")
		if err := os.WriteFile(reportPath, report, 0o600); err != nil {
			t.Fatal(err)
		}
		if packet.Grant.Target.ProviderBinding == nil {
			t.Fatalf("Dispatch target = %#v", packet.Grant.Target)
		}
		receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
			SchemaVersion: host.HostInvocationReceiptSchemaV3, Kind: host.ReceiptCompleted,
			WorkflowID: packet.WorkflowID, BundleID: packet.BundleID, BundleGeneration: packet.BundleGeneration, BundleDigest: packet.BundleDigest,
			Cursor: packet.Cursor, Topology: packet.Topology, HostSessionDigest: packet.HostSessionDigest,
			DispatchDigest: packet.Digest, ContextFreshness: host.ContextShared, EnvironmentReportDigest: packet.EnvironmentReportDigest,
			Outcome: "succeeded", Outputs: []host.OutputReference{{
				ArtifactID: packet.Grant.Target.ProviderBinding.OutputArtifact, Schema: packet.Grant.Target.ProviderBinding.OutcomeSchema,
				Reference: "file://" + reportPath, Digest: canonicaljson.DigestBytes(report),
			}},
			Evidence: []host.EvidenceReference{{Kind: "report", Reference: "file://" + reportPath, Digest: canonicaljson.DigestBytes(report)}},
		})
		if err != nil {
			t.Fatal(err)
		}
		receiptPath := filepath.Join(evidence, "receipt-"+node+".json")
		if err := writeCanonical(receiptPath, receipt); err != nil {
			t.Fatal(err)
		}
		if err := receivePilot(evidence, receiptPath); err != nil {
			t.Fatalf("receipt %s: %v", node, err)
		}
	}
	engine, err := openPilotEngine(pilot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := inspectWorkflow(engine, pilot.WorkflowID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Snapshot == nil || result.Snapshot.Status != coordinator.StatusFinished || len(result.Snapshot.GateAttestations) != 1 || result.Snapshot.GateAttestations[0].Authority != catalog.GateUser {
		t.Fatalf("final Workflow Result = %#v", result)
	}
}

func makeDogfoodRepository(t *testing.T, withSkill bool) string {
	t.Helper()
	root := t.TempDir()
	if err := runTestGit(root, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if err := runTestGit(root, "config", "user.email", "oaw@example.test"); err != nil {
		t.Fatal(err)
	}
	if err := runTestGit(root, "config", "user.name", "OAW Test"); err != nil {
		t.Fatal(err)
	}
	if withSkill {
		path := filepath.Join(root, "skills", "open-code-review", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("# test skill\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runTestGit(root, "add", "."); err != nil {
		t.Fatal(err)
	}
	if err := runTestGit(root, "commit", "-qm", "fixture"); err != nil {
		t.Fatal(err)
	}
	return root
}

func runTestGit(root string, arguments ...string) error {
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	command.Stdout = nil
	command.Stderr = nil
	return command.Run()
}
