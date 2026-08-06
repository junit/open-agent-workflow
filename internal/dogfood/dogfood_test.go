package dogfood

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
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
		WorkflowID: "workflow-11111111111111111111111111111111", BundleGeneration: 1,
		BundleDigest: strings.Repeat("a", 64), NodeID: node, Topology: execution.TopologyCurrent,
		HostSessionDigest: strings.Repeat("b", 64), EnvironmentReportDigest: strings.Repeat("c", 64), Digest: strings.Repeat("d", 64),
	}
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptCompleted,
		WorkflowID: packet.WorkflowID, BundleGeneration: packet.BundleGeneration, BundleDigest: packet.BundleDigest,
		NodeID: packet.NodeID, Topology: packet.Topology, HostSessionDigest: packet.HostSessionDigest,
		DispatchDigest: packet.Digest, ContextFreshness: host.ContextShared, EnvironmentReportDigest: packet.EnvironmentReportDigest,
		Outcome: "succeeded", Evidence: []host.EvidenceReference{{Kind: "report", Reference: "file://" + reportPath, Digest: canonicaljson.DigestBytes(report)}},
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
