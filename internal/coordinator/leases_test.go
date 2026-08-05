package coordinator

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestResourceLeaseConflictsAcrossWorkflowsAndReleasesOnTerminalReceipt(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	projectRoot := t.TempDir()
	firstStart := startTestCommand(t, "lease-first")
	secondStart := startTestCommand(t, "lease-second")
	first := newTask6Engine(t, stateRoot, projectRoot, firstStart, nil)
	second := newTask6Engine(t, stateRoot, projectRoot, secondStart, nil)

	exchangeTask6(t, first, firstStart)
	firstPrepared := exchangeTask6(t, first, task6Prepare(firstStart, "lease-first-prepare", []string{"write-project"}))
	if len(firstPrepared.Snapshot.ResourceLeases) != 1 || firstPrepared.Snapshot.ResourceLeases[0].ReleasedRevision != 0 {
		t.Fatalf("first active lease = %#v", firstPrepared.Snapshot.ResourceLeases)
	}
	secondStarted := exchangeTask6(t, second, secondStart)
	secondPrepare := task6Prepare(secondStart, "lease-second-prepare", []string{"write-project"})
	secondPrepare.ExpectedRevision = secondStarted.Revision
	if _, err := second.Exchange(secondPrepare); ErrorCode(err) != "RESOURCE_LEASE_CONFLICT" {
		t.Fatalf("conflicting PREPARE error = %v", err)
	}

	started := exchangeReceipt(t, first, receiptTestCommand(t, firstPrepared, "lease-first-started", host.ReceiptStarted, "", ""))
	completed := receiptTestCommand(t, firstPrepared, "lease-first-completed", host.ReceiptCompleted, "", "")
	completed.ExpectedRevision = started.Revision
	finished := exchangeReceipt(t, first, completed)
	if finished.Snapshot.ResourceLeases[0].ReleasedRevision != finished.Revision {
		t.Fatalf("released lease = %#v", finished.Snapshot.ResourceLeases)
	}
	secondPrepared := exchangeTask6(t, second, secondPrepare)
	if len(secondPrepared.Snapshot.ResourceLeases) != 1 || secondPrepared.Snapshot.ResourceLeases[0].ReleasedRevision != 0 {
		t.Fatalf("second lease after release = %#v", secondPrepared.Snapshot.ResourceLeases)
	}
}

func TestConcurrentResourceLeaseAcquisitionAllowsOneWorkflow(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	projectRoot := t.TempDir()
	firstStart := startTestCommand(t, "lease-concurrent-first")
	secondStart := startTestCommand(t, "lease-concurrent-second")
	first := newTask6Engine(t, stateRoot, projectRoot, firstStart, nil)
	second := newTask6Engine(t, stateRoot, projectRoot, secondStart, nil)
	exchangeTask6(t, first, firstStart)
	exchangeTask6(t, second, secondStart)

	commands := []struct {
		engine  *Engine
		command Command
	}{
		{engine: first, command: task6Prepare(firstStart, "lease-concurrent-first-prepare", []string{"write-project"})},
		{engine: second, command: task6Prepare(secondStart, "lease-concurrent-second-prepare", []string{"write-project"})},
	}
	var wait sync.WaitGroup
	errors := make(chan error, len(commands))
	for _, item := range commands {
		wait.Add(1)
		go func(engine *Engine, command Command) {
			defer wait.Done()
			_, err := engine.Exchange(command)
			errors <- err
		}(item.engine, item.command)
	}
	wait.Wait()
	close(errors)
	succeeded, conflicted := 0, 0
	for err := range errors {
		switch ErrorCode(err) {
		case "":
			succeeded++
		case "RESOURCE_LEASE_CONFLICT":
			conflicted++
		default:
			t.Fatalf("concurrent PREPARE error = %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent PREPARE outcomes = success %d, conflict %d", succeeded, conflicted)
	}
}

func TestResourceLeaseIsNotRequiredForReadOnlyAndIsRetainedWhilePaused(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	projectRoot := t.TempDir()
	writeStart := startTestCommand(t, "lease-paused-writer")
	readStart := startTestCommand(t, "lease-reader")
	writer := newTask6Engine(t, stateRoot, projectRoot, writeStart, nil)
	reader := newTask6Engine(t, stateRoot, projectRoot, readStart, nil)

	exchangeTask6(t, writer, writeStart)
	prepared := exchangeTask6(t, writer, task6Prepare(writeStart, "lease-paused-prepare", []string{"write-project"}))
	started := exchangeReceipt(t, writer, receiptTestCommand(t, prepared, "lease-paused-started", host.ReceiptStarted, "", ""))
	pausedCommand := receiptTestCommand(t, prepared, "lease-paused", host.ReceiptPaused, "", "")
	pausedCommand.ExpectedRevision = started.Revision
	paused := exchangeReceipt(t, writer, pausedCommand)
	if paused.Snapshot.ResourceLeases[0].ReleasedRevision != 0 || paused.Snapshot.ActiveGrant == nil {
		t.Fatalf("paused lease = %#v", paused.Snapshot)
	}

	exchangeTask6(t, reader, readStart)
	readPrepared := exchangeTask6(t, reader, task6Prepare(readStart, "lease-read-prepare", []string{"read-project"}))
	if len(readPrepared.Snapshot.ResourceLeases) != 0 {
		t.Fatalf("read-only PREPARE acquired leases: %#v", readPrepared.Snapshot.ResourceLeases)
	}
}

func newTask6Engine(t *testing.T, stateRoot, projectRoot string, start Command, projection ProjectionSink) *Engine {
	t.Helper()
	compiler := &startTestCore{
		t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey),
		mutateBundle: func(bundle *core.LifecycleBundle) {
			bundle.Graph.Nodes[0].MaximumEffects = []string{"read-project", "write-project"}
			bundle.Graph.Nodes[0].Resources = []string{"project-worktree"}
		},
	}
	engine, err := NewEngine(Options{
		StateRoot: stateRoot, PhysicalProjectRoot: projectRoot, Core: compiler, Projection: projection,
		Authority: admissionAuthority([]string{"read-project", "write-project"}, []string{"project-worktree"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func task6Prepare(start Command, key string, effects []string) Command {
	return prepareTestCommand(start, key, effects, []string{"project-worktree"})
}

func exchangeTask6(t *testing.T, engine *Engine, command Command) Result {
	t.Helper()
	result, err := engine.Exchange(command)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
