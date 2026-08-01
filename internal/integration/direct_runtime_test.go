package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	oawruntime "github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func TestTicket05ConcurrentStartCommitsOneDirectRevision(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	projectRoot := t.TempDir()
	const workers = 8
	engines := make([]*oawruntime.Engine, workers)
	for index := range engines {
		engine, err := oawruntime.NewEngine(oawruntime.Options{StateRoot: stateRoot})
		if err != nil {
			t.Fatalf("NewEngine(%d) error = %v", index, err)
		}
		engines[index] = engine
	}
	frame := integrationStartFrame(projectRoot, "concurrent-start")
	replies := make([]oawruntime.RunReply, workers)
	errors := make([]error, workers)
	var wait sync.WaitGroup
	wait.Add(workers)
	for index := range engines {
		go func(index int) {
			defer wait.Done()
			candidate := frame
			candidate.MessageID = fmt.Sprintf("start-%02d", index)
			replies[index], errors[index] = engines[index].Exchange(candidate)
		}(index)
	}
	wait.Wait()
	for index, err := range errors {
		if err != nil {
			t.Fatalf("concurrent START %d error = %v", index, err)
		}
		if !reflect.DeepEqual(replies[index], replies[0]) {
			t.Fatalf("concurrent START %d reply differs\n got: %#v\nwant: %#v", index, replies[index], replies[0])
		}
	}
	assertIntegrationRevisionCount(t, stateRoot, replies[0].RunID, 1)
	assertNoDirectAuthority(t, replies[0])
}

func TestTicket05ConcurrentContinueCommitsOneExpansion(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	projectRoot := t.TempDir()
	starter, err := oawruntime.NewEngine(oawruntime.Options{StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("NewEngine(starter) error = %v", err)
	}
	started, err := starter.Exchange(integrationStartFrame(projectRoot, "continue-start"))
	if err != nil {
		t.Fatalf("START error = %v", err)
	}

	const workers = 8
	errors := make([]error, workers)
	replies := make([]oawruntime.RunReply, workers)
	var wait sync.WaitGroup
	wait.Add(workers)
	for index := 0; index < workers; index++ {
		go func(index int) {
			defer wait.Done()
			engine, engineErr := oawruntime.NewEngine(oawruntime.Options{StateRoot: stateRoot})
			if engineErr != nil {
				errors[index] = engineErr
				return
			}
			replies[index], errors[index] = engine.Exchange(integrationContinueFrame(
				started.RunID,
				started.Revision,
				fmt.Sprintf("continue-%02d", index),
			))
		}(index)
	}
	wait.Wait()
	successes := 0
	conflicts := 0
	for index, err := range errors {
		switch {
		case err == nil:
			successes++
			if replies[index].Revision != 2 || replies[index].Kind != oawruntime.ReplyPaused {
				t.Fatalf("successful CONTINUE reply = %#v", replies[index])
			}
			assertNoDirectAuthority(t, replies[index])
		case oawruntime.ErrorCode(err) == "RUN_REVISION_CONFLICT":
			conflicts++
		default:
			t.Fatalf("concurrent CONTINUE %d error = %v", index, err)
		}
	}
	if successes != 1 || conflicts != workers-1 {
		t.Fatalf("concurrent CONTINUE results: successes=%d conflicts=%d", successes, conflicts)
	}
	assertIntegrationRevisionCount(t, stateRoot, started.RunID, 2)
}

func TestTicket05OrphanRevisionWithoutHeadUpdateIsIgnored(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine, err := oawruntime.NewEngine(oawruntime.Options{StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	started, err := engine.Exchange(integrationStartFrame(t.TempDir(), "orphan-start"))
	if err != nil {
		t.Fatalf("START error = %v", err)
	}
	headPath := filepath.Join(stateRoot, "runs", started.RunID, "HEAD")
	headRevisionOne, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("ReadFile(HEAD revision 1) error = %v", err)
	}
	if _, err := engine.Exchange(integrationContinueFrame(started.RunID, 1, "orphan-expansion")); err != nil {
		t.Fatalf("CONTINUE error = %v", err)
	}
	if err := os.WriteFile(headPath, headRevisionOne, 0o600); err != nil {
		t.Fatalf("restore HEAD revision 1 error = %v", err)
	}

	restarted, err := oawruntime.NewEngine(oawruntime.Options{StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("NewEngine(restart) error = %v", err)
	}
	inspected, err := restarted.Exchange(integrationInspectFrame(started.RunID))
	if err != nil {
		t.Fatalf("INSPECT orphan fixture error = %v", err)
	}
	if inspected.Revision != 1 || inspected.RevisionDigest != started.RevisionDigest || !reflect.DeepEqual(inspected.Snapshot, started.Snapshot) {
		t.Fatalf("orphan revision became authoritative: %#v", inspected)
	}
	assertIntegrationRevisionCount(t, stateRoot, started.RunID, 2)
}

func TestTicket05RuntimeUsesOwnerOnlyPermissionsAndNoDirectAuthority(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine, err := oawruntime.NewEngine(oawruntime.Options{StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	startFrame := integrationStartFrame(t.TempDir(), "permissions-start")
	started, err := engine.Exchange(startFrame)
	if err != nil {
		t.Fatalf("START error = %v", err)
	}
	replay, err := engine.Exchange(startFrame)
	if err != nil {
		t.Fatalf("START replay error = %v", err)
	}
	expanded, err := engine.Exchange(integrationContinueFrame(started.RunID, 1, "permissions-expand"))
	if err != nil {
		t.Fatalf("CONTINUE error = %v", err)
	}
	restarted, err := oawruntime.NewEngine(oawruntime.Options{StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("NewEngine(restart) error = %v", err)
	}
	inspected, err := restarted.Exchange(integrationInspectFrame(started.RunID))
	if err != nil {
		t.Fatalf("INSPECT error = %v", err)
	}
	for _, reply := range []oawruntime.RunReply{started, replay, expanded, inspected} {
		assertNoDirectAuthority(t, reply)
	}
	if len(started.Diagnostics) != 3 || len(replay.Diagnostics) != 3 || len(expanded.Diagnostics) != 0 || len(inspected.Diagnostics) != 0 {
		t.Fatalf("Direct boundary diagnostics leaked to the wrong replies: start=%#v replay=%#v expanded=%#v inspect=%#v", started.Diagnostics, replay.Diagnostics, expanded.Diagnostics, inspected.Diagnostics)
	}

	runRoot := filepath.Join(stateRoot, "runs", started.RunID)
	for _, path := range []string{stateRoot, filepath.Join(stateRoot, "runs"), runRoot, filepath.Join(runRoot, "revisions")} {
		assertIntegrationMode(t, path, 0o700)
	}
	for _, path := range []string{
		filepath.Join(runRoot, "LOCK"),
		filepath.Join(runRoot, "HEAD"),
		filepath.Join(runRoot, "revisions", "00000000000000000001.json"),
		filepath.Join(runRoot, "revisions", "00000000000000000002.json"),
	} {
		assertIntegrationMode(t, path, 0o600)
	}
}

func TestTicket05RuntimeHonorsClassificationRulesAndConfigurationDigest(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	projectRoot := t.TempDir()
	directEngine, err := oawruntime.NewEngine(oawruntime.Options{StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("NewEngine(direct) error = %v", err)
	}
	direct, err := directEngine.Exchange(integrationStartFrame(projectRoot, "classification-direct"))
	if err != nil {
		t.Fatalf("Direct START error = %v", err)
	}
	if direct.Snapshot.ConfigurationDigest != strings.Repeat("a", 64) || direct.Snapshot.Project.ConfigurationDigest != direct.Snapshot.ConfigurationDigest {
		t.Fatalf("configuration digest not pinned: %#v", direct.Snapshot)
	}

	workflowEngine, err := oawruntime.NewEngine(oawruntime.Options{
		StateRoot: stateRoot,
		Rules: classification.ClassificationRules{
			User: classification.PolicyLayer{MinimumMode: classification.RequestModeWorkflow},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine(workflow rule) error = %v", err)
	}
	_, err = workflowEngine.Exchange(integrationStartFrame(projectRoot, "classification-raised"))
	if oawruntime.ErrorCode(err) != "WORKFLOW_REQUEST_INVALID" {
		t.Fatalf("raised classification error = %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(stateRoot, "runs"))
	if err != nil {
		t.Fatalf("ReadDir(runs) error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("non-Direct classification created state: %d Run directories", len(entries))
	}
}

func integrationStartFrame(projectRoot, idempotencyKey string) oawruntime.RunFrame {
	proposal := integrationDirectProposal()
	return oawruntime.RunFrame{
		SchemaVersion:  oawruntime.RuntimeSchemaV1,
		Kind:           oawruntime.FrameStart,
		MessageID:      idempotencyKey + "-message",
		IdempotencyKey: idempotencyKey,
		Start: &oawruntime.StartInput{
			RequestID: "integration-request",
			Project: oawruntime.ProjectIdentity{
				Root:                projectRoot,
				ConfigurationDigest: strings.Repeat("a", 64),
			},
			Proposal: &proposal,
		},
	}
}

func integrationContinueFrame(runID string, expectedRevision uint64, idempotencyKey string) oawruntime.RunFrame {
	return oawruntime.RunFrame{
		SchemaVersion:    oawruntime.RuntimeSchemaV1,
		Kind:             oawruntime.FrameContinue,
		MessageID:        idempotencyKey + "-message",
		IdempotencyKey:   idempotencyKey,
		RunID:            runID,
		ExpectedRevision: expectedRevision,
		Continue:         &oawruntime.ContinueInput{Signal: oawruntime.SignalScopeExpanded},
	}
}

func integrationInspectFrame(runID string) oawruntime.RunFrame {
	return oawruntime.RunFrame{
		SchemaVersion:  oawruntime.RuntimeSchemaV1,
		Kind:           oawruntime.FrameInspect,
		MessageID:      "inspect-message",
		IdempotencyKey: "inspect-key",
		RunID:          runID,
	}
}

func assertNoDirectAuthority(t *testing.T, reply oawruntime.RunReply) {
	t.Helper()
	if reply.Snapshot.LifecycleBundles == nil || len(reply.Snapshot.LifecycleBundles) != 0 ||
		reply.Snapshot.GrantIDs == nil || len(reply.Snapshot.GrantIDs) != 0 ||
		reply.Snapshot.ResourceLeaseIDs == nil || len(reply.Snapshot.ResourceLeaseIDs) != 0 {
		t.Fatalf("Direct reply contains Runtime authority: %#v", reply.Snapshot)
	}
}

func assertIntegrationRevisionCount(t *testing.T, stateRoot, runID string, expected int) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(stateRoot, "runs", runID, "revisions"))
	if err != nil {
		t.Fatalf("ReadDir(revisions) error = %v", err)
	}
	if len(entries) != expected {
		t.Fatalf("revision count = %d, want %d", len(entries), expected)
	}
}

func assertIntegrationMode(t *testing.T, path string, expected os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	if actual := info.Mode().Perm(); actual != expected {
		t.Fatalf("mode(%s) = %#o, want %#o", path, actual, expected)
	}
}
