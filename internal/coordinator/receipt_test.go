package coordinator

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestReceiptStartedAndTerminalCompletedAdvanceFromPinnedDispatch(t *testing.T) {
	engine, start, prepared := preparedReceiptWorkflow(t, nil)
	startedCommand := receiptTestCommand(t, prepared, "receipt-started", host.ReceiptStarted, "", "")
	started, err := engine.Exchange(startedCommand)
	if err != nil {
		t.Fatalf("STARTED error = %v", err)
	}
	if started.Kind != ResultState || started.Snapshot == nil || started.Snapshot.Status != StatusInFlight ||
		started.Snapshot.ActiveGrant == nil || len(started.Snapshot.Receipts) != 1 {
		t.Fatalf("STARTED Result = %#v", started)
	}

	completedCommand := receiptTestCommand(t, prepared, "receipt-completed", host.ReceiptCompleted, "succeeded", "")
	completedCommand.ExpectedRevision = started.Revision
	completed, err := engine.Exchange(completedCommand)
	if err != nil {
		t.Fatalf("COMPLETED error = %v", err)
	}
	if completed.Snapshot == nil || completed.Snapshot.Status != StatusReady || completed.Snapshot.ActiveGrant != nil ||
		completed.Snapshot.Cursor != startTestSlotCursor(t, completed.Snapshot.Bundles[0].Graph, catalog.SlotSolutionSpecification) ||
		len(completed.Snapshot.Receipts) != 2 || completed.WorkflowID != deriveWorkflowID(start.IdempotencyKey) {
		t.Fatalf("COMPLETED Result = %#v", completed)
	}
	completedCommand.MessageID = "message-retried"
	replayed, err := engine.Exchange(completedCommand)
	if err != nil || !replayed.Replayed || replayed.Revision != completed.Revision {
		t.Fatalf("COMPLETED replay = %#v, %v", replayed, err)
	}
}

func TestReceiptRejectsDispatchPinAndStatusMismatchWithoutCommit(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*host.InvocationReceipt)
	}{
		{name: "Workflow", mutate: func(value *host.InvocationReceipt) { value.WorkflowID = "workflow-00000000000000000000000000000000" }},
		{name: "Bundle ID", mutate: func(value *host.InvocationReceipt) { value.BundleID = "bundle-00000000000000000000000000000000" }},
		{name: "Bundle generation", mutate: func(value *host.InvocationReceipt) { value.BundleGeneration++ }},
		{name: "Bundle digest", mutate: func(value *host.InvocationReceipt) { value.BundleDigest = strings.Repeat("f", 64) }},
		{name: "cursor", mutate: func(value *host.InvocationReceipt) {
			value.Cursor = execution.GraphCursor{SlotID: string(catalog.SlotImplementation), Kind: execution.CursorBinding, UnitID: "different", Ordinal: 1}
		}},
		{name: "topology", mutate: func(value *host.InvocationReceipt) {
			value.Topology = execution.TopologySubagent
			value.ContextFreshness = host.ContextFresh
			value.InvocationHandle = "different-invocation"
		}},
		{name: "Host session", mutate: func(value *host.InvocationReceipt) { value.HostSessionDigest = strings.Repeat("f", 64) }},
		{name: "Dispatch", mutate: func(value *host.InvocationReceipt) { value.DispatchDigest = strings.Repeat("f", 64) }},
		{name: "environment report", mutate: func(value *host.InvocationReceipt) { value.EnvironmentReportDigest = strings.Repeat("f", 64) }},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			engine, _, prepared := preparedReceiptWorkflow(t, nil)
			command := receiptTestCommand(t, prepared, "receipt-mismatch-"+strings.ReplaceAll(test.name, " ", "-"), host.ReceiptStarted, "", "")
			value := command.Receipt.Receipt
			value.Digest = ""
			test.mutate(&value)
			normalized, err := host.NewInvocationReceipt(value)
			if err != nil {
				t.Fatal(err)
			}
			command.Receipt.Receipt = normalized
			if _, err := engine.Exchange(command); ErrorCode(err) != "WORKFLOW_RECEIPT_INVALID" {
				t.Fatalf("mismatched Receipt error = %v", err)
			}
			current, err := engine.journal.inspect(prepared.WorkflowID)
			if err != nil || current.Revision != prepared.Revision || current.Snapshot.Status != StatusPrepared {
				t.Fatalf("mismatched Receipt changed state: %#v, %v", current, err)
			}
		})
	}

	engine, _, prepared := preparedReceiptWorkflow(t, nil)
	completed := receiptTestCommand(t, prepared, "receipt-before-start", host.ReceiptCompleted, "", "")
	if _, err := engine.Exchange(completed); ErrorCode(err) != "WORKFLOW_RECEIPT_INVALID" {
		t.Fatalf("COMPLETED before STARTED error = %v", err)
	}
}

func TestReceiptSubagentInvocationHandleRemainsPinnedToStarted(t *testing.T) {
	engine, _, prepared := preparedSubagentReceiptWorkflow(t)
	started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "subagent-started", host.ReceiptStarted, "", ""))
	completed := receiptTestCommand(t, prepared, "subagent-completed", host.ReceiptCompleted, "", "")
	completed.ExpectedRevision = started.Revision
	completed.Receipt.Receipt.Digest = ""
	completed.Receipt.Receipt.InvocationHandle = "different-invocation"
	completed.Receipt.Receipt, _ = host.NewInvocationReceipt(completed.Receipt.Receipt)
	if _, err := engine.Exchange(completed); ErrorCode(err) != "WORKFLOW_RECEIPT_INVALID" {
		t.Fatalf("changed SUBAGENT handle error = %v", err)
	}
	current, err := engine.journal.inspect(prepared.WorkflowID)
	if err != nil || current.Revision != started.Revision || current.Snapshot.Status != StatusInFlight {
		t.Fatalf("changed SUBAGENT handle changed state: %#v, %v", current, err)
	}
}

func TestReceiptCompletedIncidentPausedAndCancelledTransitions(t *testing.T) {
	t.Run("declared graph edge", func(t *testing.T) {
		engine, _, prepared := preparedReceiptWorkflow(t, receiptGraphBundle)
		started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "edge-started", host.ReceiptStarted, "", ""))
		completed := receiptTestCommand(t, prepared, "edge-completed", host.ReceiptCompleted, "succeeded", "")
		completed.ExpectedRevision = started.Revision
		result := exchangeReceipt(t, engine, completed)
		if result.Snapshot.Status != StatusReady || result.Snapshot.Cursor != startTestSlotCursor(t, result.Snapshot.Bundles[0].Graph, catalog.SlotSolutionSpecification) || result.Snapshot.ActiveGrant != nil {
			t.Fatalf("edge Result = %#v", result)
		}
	})

	t.Run("declared incident", func(t *testing.T) {
		engine, _, prepared := preparedReceiptWorkflow(t, receiptGraphBundle)
		started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "incident-started", host.ReceiptStarted, "", ""))
		failed := receiptTestCommand(t, prepared, "incident-failed", host.ReceiptFailed, "", "build-failure")
		failed.ExpectedRevision = started.Revision
		result := exchangeReceipt(t, engine, failed)
		if result.Snapshot.Status != StatusReady || result.Snapshot.Cursor != startTestSlotCursor(t, result.Snapshot.Bundles[0].Graph, catalog.SlotIncidentRecovery) || result.Snapshot.ActiveGrant != nil {
			t.Fatalf("incident Result = %#v", result)
		}
	})

	t.Run("unrouted incident pauses", func(t *testing.T) {
		engine, _, prepared := preparedReceiptWorkflow(t, receiptGraphBundle)
		started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "unrouted-started", host.ReceiptStarted, "", ""))
		failed := receiptTestCommand(t, prepared, "unrouted-failed", host.ReceiptFailed, "", "unknown-failure")
		failed.ExpectedRevision = started.Revision
		result := exchangeReceipt(t, engine, failed)
		if result.Snapshot.Status != StatusPaused || result.Snapshot.ActiveGrant != nil || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "WORKFLOW_INCIDENT_UNROUTED" {
			t.Fatalf("unrouted Result = %#v", result)
		}
	})

	for _, test := range []struct {
		name       string
		kind       host.ReceiptKind
		wantStatus Status
		active     bool
	}{
		{name: "paused", kind: host.ReceiptPaused, wantStatus: StatusPaused, active: true},
		{name: "cancelled", kind: host.ReceiptCancelled, wantStatus: StatusCancelled, active: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, _, prepared := preparedReceiptWorkflow(t, nil)
			started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, test.name+"-started", host.ReceiptStarted, "", ""))
			command := receiptTestCommand(t, prepared, test.name+"-terminal", test.kind, "", "")
			command.ExpectedRevision = started.Revision
			result := exchangeReceipt(t, engine, command)
			if result.Snapshot.Status != test.wantStatus || (result.Snapshot.ActiveGrant != nil) != test.active || len(result.Snapshot.Receipts) != 2 {
				t.Fatalf("%s Result = %#v", test.name, result)
			}
		})
	}
}

func TestOutputEvidenceReceiptTransitionRejectsIncompleteEvidence(t *testing.T) {
	engine, _, prepared := preparedReceiptWorkflow(t, receiptGraphBundle)
	started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "evidence-started", host.ReceiptStarted, "", ""))
	missing := receiptTestCommand(t, prepared, "evidence-missing", host.ReceiptCompleted, "succeeded", "")
	missing.ExpectedRevision = started.Revision
	missing.Receipt.Receipt.Evidence[0].Kind = "different"
	missing.Receipt.Receipt.Digest = ""
	missing.Receipt.Receipt, _ = host.NewInvocationReceipt(missing.Receipt.Receipt)
	if _, err := engine.Exchange(missing); ErrorCode(err) != "WORKFLOW_EVIDENCE_INCOMPLETE" {
		t.Fatalf("missing evidence error = %v", err)
	}
	undeclared := receiptTestCommand(t, prepared, "signal-undeclared", host.ReceiptCompleted, "invented", "")
	undeclared.ExpectedRevision = started.Revision
	if _, err := engine.Exchange(undeclared); ErrorCode(err) != "WORKFLOW_SIGNAL_UNDECLARED" {
		t.Fatalf("undeclared signal error = %v", err)
	}
}

func TestSnapshotRejectsAuthorityHistoriesOutsidePinnedBundle(t *testing.T) {
	engine, _, prepared := preparedReceiptWorkflow(t, receiptGraphBundle)
	started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "history-started", host.ReceiptStarted, "", ""))
	completedCommand := receiptTestCommand(t, prepared, "history-completed", host.ReceiptCompleted, "succeeded", "")
	completedCommand.ExpectedRevision = started.Revision
	completed := exchangeReceipt(t, engine, completedCommand)
	base := *completed.Snapshot
	fakeBundleID := "bundle-00000000000000000000000000000000"

	mutations := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{name: "Grant", mutate: func(snapshot *Snapshot) {
			snapshot.GrantHistory[0].BundleID = fakeBundleID
			resealGrant(t, &snapshot.GrantHistory[0])
		}},
		{name: "User Authorization", mutate: func(snapshot *Snapshot) {
			grant := snapshot.GrantHistory[0]
			authorization, err := admission.NewUserAuthorization(admission.UserAuthorization{
				SchemaVersion: admission.UserAuthorizationSchemaV1, IssuerHostID: snapshot.Bundles[0].HostID,
				HostSessionDigest: grant.HostSessionDigest, EvidenceHandleDigest: strings.Repeat("7", 64), AuthorizationNonce: "history-authorization-nonce",
				WorkflowID: snapshot.WorkflowID, BundleID: fakeBundleID, BundleGeneration: grant.BundleGeneration, BundleDigest: grant.BundleDigest,
				Cursor: grant.Cursor, Target: admission.CloneAuthorizationTarget(grant.Target), Decision: admission.AuthorizationAllowed,
				Effects: append([]string{}, grant.Effects...), Resources: append([]string{}, grant.Resources...),
				Evidence: []host.EvidenceReference{{Kind: "approval", Reference: "evidence://history/authorization", Digest: strings.Repeat("8", 64)}},
			})
			if err != nil {
				t.Fatal(err)
			}
			snapshot.UserAuthorizations = append(snapshot.UserAuthorizations, authorization)
		}},
		{name: "Invocation Attestation", mutate: func(snapshot *Snapshot) {
			grant := snapshot.GrantHistory[0]
			providerBinding := *grant.Target.ProviderBinding
			providerBinding.Invocation = catalog.InvocationHumanExplicit
			providerBinding.RequiresExplicitInvocation = true
			attestation, err := admission.NewExplicitInvocationAttestation(admission.ExplicitInvocationAttestation{
				SchemaVersion: admission.ExplicitInvocationAttestationSchemaV1, IssuerHostID: snapshot.Bundles[0].HostID,
				HostSessionDigest: grant.HostSessionDigest, EvidenceHandleDigest: strings.Repeat("7", 64), InvocationNonce: "history-invocation-nonce",
				WorkflowID: snapshot.WorkflowID, BundleID: fakeBundleID, BundleGeneration: grant.BundleGeneration, BundleDigest: grant.BundleDigest,
				Cursor: grant.Cursor, ProviderBinding: providerBinding,
				Evidence: []host.EvidenceReference{{Kind: "invocation", Reference: "evidence://history/invocation", Digest: strings.Repeat("9", 64)}},
			})
			if err != nil {
				t.Fatal(err)
			}
			snapshot.InvocationAttestations = append(snapshot.InvocationAttestations, attestation)
		}},
		{name: "Gate Attestation", mutate: func(snapshot *Snapshot) {
			var gate profile.CompiledGate
			for _, slot := range snapshot.Bundles[0].Graph.Slots {
				if len(slot.Gates) != 0 {
					gate = slot.Gates[0]
					break
				}
			}
			attestation, err := normalizeGateAttestation(GateAttestation{
				SchemaVersion: GateAttestationSchemaV1, WorkflowID: snapshot.WorkflowID, BundleID: fakeBundleID,
				BundleGeneration: snapshot.Bundles[0].Generation, BundleDigest: snapshot.Bundles[0].Digest,
				Cursor: gate.Cursor, GateID: gate.ID, Authority: gate.Authority, Decision: GateSatisfied,
				Evidence: []host.EvidenceReference{{Kind: "approval", Reference: "evidence://history/gate", Digest: strings.Repeat("a", 64)}},
			})
			if err != nil {
				t.Fatal(err)
			}
			snapshot.GateAttestations = append(snapshot.GateAttestations, attestation)
		}},
		{name: "Host Receipt", mutate: func(snapshot *Snapshot) {
			receipt := host.CloneInvocationReceipt(snapshot.Receipts[0])
			receipt.BundleID = fakeBundleID
			receipt.Digest = ""
			normalized, err := host.NewInvocationReceipt(receipt)
			if err != nil {
				t.Fatal(err)
			}
			snapshot.Receipts[0] = normalized
		}},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneSnapshot(base)
			test.mutate(&candidate)
			if err := validateSnapshot(candidate, candidate.WorkflowID, candidate.Revision, false); ErrorCode(err) != "WORKFLOW_STATE_REVISION_INVALID" {
				t.Fatalf("validateSnapshot(%s outside Bundle) error = %v", test.name, err)
			}
		})
	}
}

func TestGateOnlyPrepareCommitsPinnedAttestationAndFinishesGraph(t *testing.T) {
	engine, start, current := preparedReceiptWorkflow(t, receiptGraphBundle)
	for step := 0; current.Snapshot.Cursor.Kind != execution.CursorGate; step++ {
		if step > 16 {
			t.Fatal("Workflow did not reach the closeout gate")
		}
		startedCommand := receiptTestCommand(t, current, fmt.Sprintf("gate-path-%d-started", step), host.ReceiptStarted, "", "")
		started := exchangeReceipt(t, engine, startedCommand)
		signal := "succeeded"
		if current.Snapshot.Cursor.SlotID == string(catalog.SlotCloseout) {
			signal = ""
		}
		completedCommand := receiptTestCommand(t, current, fmt.Sprintf("gate-path-%d-completed", step), host.ReceiptCompleted, signal, "")
		completedCommand.ExpectedRevision = started.Revision
		completed := exchangeReceipt(t, engine, completedCommand)
		if completed.Snapshot.Cursor.Kind == execution.CursorGate {
			current = completed
			break
		}
		prepare := prepareTestCommand(start, fmt.Sprintf("gate-path-%d-prepare", step), []string{"read-project"}, []string{"project-worktree"})
		prepare.ExpectedRevision = completed.Revision
		prepared, err := engine.Exchange(prepare)
		if err != nil {
			t.Fatal(err)
		}
		current = prepared
	}
	bundle := current.Snapshot.Bundles[0]
	unit, err := profile.UnitAtCursor(bundle.Graph, current.Snapshot.Cursor)
	if err != nil || unit.Gate == nil {
		t.Fatalf("active gate unit = %#v, %v", unit, err)
	}
	attestation, err := normalizeGateAttestation(GateAttestation{
		SchemaVersion: GateAttestationSchemaV1, WorkflowID: current.WorkflowID, BundleID: bundle.ID,
		BundleGeneration: bundle.Generation, BundleDigest: bundle.Digest, Cursor: current.Snapshot.Cursor,
		GateID: unit.Gate.ID, Authority: unit.Gate.Authority, Decision: GateSatisfied,
		Evidence: []host.EvidenceReference{{Kind: "approval", Reference: "evidence://gate/closeout", Digest: strings.Repeat("a", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Exchange(Command{
		SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandPrepare, MessageID: "message-gate-closeout", IdempotencyKey: "command-gate-closeout",
		WorkflowID: current.WorkflowID, ExpectedRevision: current.Revision,
		Prepare: &PrepareInput{RequestedEffects: []string{}, RequestedResources: []string{}, InputReferences: []ArtifactReference{}, EvidenceRequirements: []EvidenceRequirement{}, GateAttestation: &attestation},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Snapshot == nil || result.Snapshot.Status != StatusFinished || len(result.Snapshot.GateAttestations) != 1 || result.Dispatch != nil || result.Snapshot.ActiveGrant != nil {
		if result.Snapshot == nil {
			t.Fatalf("gate-only PREPARE omitted Snapshot: %#v", result)
		}
		t.Fatalf("gate-only PREPARE state: status=%s gate_attestations=%d dispatch=%v active_grant=%v cursor=%#v", result.Snapshot.Status, len(result.Snapshot.GateAttestations), result.Dispatch != nil, result.Snapshot.ActiveGrant != nil, result.Snapshot.Cursor)
	}
}

func preparedReceiptWorkflow(t *testing.T, mutate func(*core.LifecycleBundle)) (*Engine, Command, Result) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "receipt-workflow-"+strings.ReplaceAll(t.Name(), "/", "-"))
	compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey), mutateBundle: mutate}
	options := startTestOptions(t, stateRoot, compiler)
	options.Authority = admissionAuthority([]string{"read-project"}, []string{"project-worktree"})
	engine, err := NewEngine(options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exchange(start); err != nil {
		t.Fatal(err)
	}
	prepare := prepareTestCommand(start, "receipt-prepare-"+strings.ReplaceAll(t.Name(), "/", "-"), []string{"read-project"}, []string{"project-worktree"})
	prepare.Prepare.EvidenceRequirements = []EvidenceRequirement{{Kind: "report", Minimum: 1, Description: "completion report"}}
	prepared, err := engine.Exchange(prepare)
	if err != nil {
		t.Fatal(err)
	}
	return engine, start, prepared
}

func preparedSubagentReceiptWorkflow(t *testing.T) (*Engine, Command, Result) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "receipt-subagent-"+strings.ReplaceAll(t.Name(), "/", "-"))
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: "codex", ControlSurface: host.SurfaceHostNative,
		Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []catalog.BindingKind{catalog.BindingSkill}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		Features: []host.Feature{host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory}, DelegationFeatures: []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		t.Fatal(err)
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-child", ParentSessionID: "session-current",
		Topology: execution.TopologySubagent, Observations: []execution.EnvironmentObservation{},
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := host.BuildBindingInventoryV3("codex", nil)
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: "acme/codex-host", IntegrationVersion: "3.0.0", ManifestDigest: manifest.Digest,
		SessionID: "session-current", SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		ProviderInventoryDigest: inventory.Digest, FeatureObservations: []host.FeatureObservation{}, HostActionObservations: []host.HostActionObservation{}, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	start.Start.Selection.Topology = execution.TopologySubagent
	graphSelection := startTestGraphSelection()
	graphSelection.Topology = execution.TopologySubagent
	graphSelection.Digest = ""
	graphSelection.Digest = startTestDigest(graphSelection)
	start.Start.Selection.GraphSelectionDigest = graphSelection.Digest
	start.Start.HostSession, start.Start.Environment = session, environment
	compiler := &startTestCore{
		t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey),
	}
	evidence, err := profile.NewHostEvidence(manifest, session, inventory, environment)
	if err != nil {
		t.Fatal(err)
	}
	options := startTestOptions(t, stateRoot, compiler)
	options.Host = evidence
	options.Authority = admissionAuthority([]string{"read-project"}, []string{"project-worktree"})
	engine, err := NewEngine(options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exchange(start); err != nil {
		t.Fatal(err)
	}
	prepare := prepareTestCommand(start, "receipt-subagent-prepare-"+strings.ReplaceAll(t.Name(), "/", "-"), []string{"read-project"}, []string{"project-worktree"})
	prepare.Prepare.EvidenceRequirements = []EvidenceRequirement{{Kind: "report", Minimum: 1, Description: "completion report"}}
	prepared, err := engine.Exchange(prepare)
	if err != nil {
		t.Fatal(err)
	}
	return engine, start, prepared
}

func receiptTestCommand(t *testing.T, prepared Result, key string, kind host.ReceiptKind, signal, failureCode string) Command {
	t.Helper()
	packet := prepared.Dispatch
	receipt := host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV3, Kind: kind, WorkflowID: packet.WorkflowID,
		BundleID: packet.BundleID, BundleGeneration: packet.BundleGeneration, BundleDigest: packet.BundleDigest, Cursor: packet.Cursor,
		Topology: packet.Topology, HostSessionDigest: packet.HostSessionDigest, DispatchDigest: packet.Digest,
		ContextFreshness: host.ContextShared, EnvironmentReportDigest: packet.EnvironmentReportDigest,
	}
	if packet.Topology == execution.TopologySubagent {
		receipt.ContextFreshness = host.ContextFresh
		receipt.InvocationHandle = "child-invocation-1"
	}
	switch kind {
	case host.ReceiptPaused:
		receipt.Outcome = "paused"
	case host.ReceiptCompleted:
		receipt.Outcome = "succeeded"
		artifactID, schema := receiptOutputContract(t, packet)
		receipt.Outputs = []host.OutputReference{{ArtifactID: artifactID, Schema: schema, Reference: "artifact://result/1", Digest: strings.Repeat("d", 64)}}
		receipt.Evidence = []host.EvidenceReference{{Kind: "report", Reference: "evidence://report", Digest: strings.Repeat("e", 64)}}
	case host.ReceiptFailed:
		receipt.Outcome = "failed"
		receipt.FailureCode = failureCode
		receipt.Evidence = []host.EvidenceReference{{Kind: "diagnostic", Reference: "evidence://failure", Digest: strings.Repeat("f", 64)}}
	case host.ReceiptCancelled:
		receipt.Outcome = "cancelled"
	}
	normalized, err := host.NewInvocationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return Command{
		SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandReceipt, MessageID: "message-" + key, IdempotencyKey: "command-" + key,
		WorkflowID: prepared.WorkflowID, ExpectedRevision: prepared.Revision,
		Receipt: &ReceiptInput{Receipt: normalized, Signal: signal},
	}
}

func exchangeReceipt(t *testing.T, engine *Engine, command Command) Result {
	t.Helper()
	result, err := engine.Exchange(command)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func receiptGraphBundle(bundle *core.LifecycleBundle) {
	var handler []execution.GraphCursor
	for _, slot := range bundle.Graph.Slots {
		if slot.SlotID == catalog.SlotIncidentRecovery && len(slot.Pipeline) != 0 {
			handler = []execution.GraphCursor{slot.Pipeline[0].Cursor}
		}
	}
	bundle.Graph.IncidentRoutes = []profile.CompiledIncidentRoute{{
		IncidentType: "build-failure", HandlerSlotID: catalog.SlotIncidentRecovery, HandlerPipeline: handler,
		ReturnTo: catalog.SlotProblemFraming, IfUnavailable: catalog.IncidentStop,
	}}
	bundle.Graph.StableBoundaries = []string{"closeout", "implementation", "problem-framing-complete", "review-remediation"}
}

func startTestSlotCursor(t testing.TB, graph profile.ExecutionGraphRecord, slotID catalog.SlotID) execution.GraphCursor {
	t.Helper()
	for _, slot := range graph.Slots {
		if slot.SlotID == slotID && len(slot.Pipeline) != 0 {
			return slot.Pipeline[0].Cursor
		}
	}
	t.Fatalf("slot %s has no Provider Binding cursor", slotID)
	return execution.GraphCursor{}
}

func receiptOutputContract(t testing.TB, packet *DispatchPacket) (string, string) {
	t.Helper()
	switch packet.Grant.Target.TargetKind {
	case admission.GrantProviderBinding:
		return packet.Grant.Target.ProviderBinding.OutputArtifact, packet.Grant.Target.ProviderBinding.OutcomeSchema
	case admission.GrantHostAction:
		return packet.Grant.Target.HostAction.OutputArtifact, packet.Grant.Target.HostAction.OutcomeSchema
	default:
		t.Fatalf("unsupported Grant target %q", packet.Grant.Target.TargetKind)
		return "", ""
	}
}
