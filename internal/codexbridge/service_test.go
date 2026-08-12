package codexbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestObserveCurrentCreatesHandleFromInjectedContext(t *testing.T) {
	service := newTestService(t)
	result, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	if result.HostEvidenceHandle.Token == "" || result.HostSummary.SessionDigest == "" || result.HostSummary.InventoryDigest == "" || result.HostSummary.EnvironmentDigest == "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestObserveCurrentPropagatesOptionalMetadataDiagnostics(t *testing.T) {
	service := newTestService(t)
	service.observer.(*fakeObserver).SetDiagnostics([]appserver.ObservationDiagnostic{{
		Code: "HOST_OBSERVATION_PARTIAL", Detail: "hooks/list unavailable",
	}})
	result, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(result.HostSummary.Diagnostics, "HOST_OBSERVATION_PARTIAL") {
		t.Fatalf("summary = %#v", result.HostSummary)
	}
}

func TestObserveCurrentUsesOnlyExactLiveFeatureEvidence(t *testing.T) {
	service := newTestService(t)
	without, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	withoutFacts, err := service.getFacts(without.HostEvidenceHandle)
	if err != nil {
		t.Fatal(err)
	}
	if len(withoutFacts.Session.FeatureObservations) != 0 {
		t.Fatalf("missing evidence became available: %#v", withoutFacts.Session.FeatureObservations)
	}
	live, err := host.NewFeatureObservation(host.FeatureObservation{
		Feature: host.FeatureChildDelegation, State: host.AvailabilityAvailable, Source: host.SourceNativeAPI,
		EvidenceReference: "evidence://codex/subagent-start/test",
	})
	if err != nil {
		t.Fatal(err)
	}
	service.featureObserver = staticFeatureObserver{result: FeatureEvidenceResult{Observations: []host.FeatureObservation{live}}}
	with, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	withFacts, err := service.getFacts(with.HostEvidenceHandle)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(withFacts.Session.FeatureObservations, []host.FeatureObservation{live}) {
		t.Fatalf("live features = %#v", withFacts.Session.FeatureObservations)
	}
}

func TestObserveCurrentCapsCombinedDiagnosticsAtWireBoundary(t *testing.T) {
	service := newTestService(t)
	values := make([]Diagnostic, maximumObserveCurrentDiagnostics+7)
	for index := range values {
		values[index] = NewDiagnostic(fmt.Sprintf("TEST_MIXED_%03d", index), "observation", fmt.Sprintf("mixed diagnostic %03d", index), true)
	}
	service.featureObserver = staticFeatureObserver{result: FeatureEvidenceResult{Diagnostics: values}}
	observed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	if len(observed.HostSummary.Diagnostics) != maximumObserveCurrentDiagnostics ||
		observed.HostSummary.Diagnostics[len(observed.HostSummary.Diagnostics)-1].Code != "HOST_DIAGNOSTICS_TRUNCATED" {
		t.Fatalf("wire diagnostics=%#v", observed.HostSummary.Diagnostics)
	}
}

func TestCoreInspectV4ReturnsFourAliasesBuilderAndNoImplicitSelection(t *testing.T) {
	service := newTestService(t)
	observed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := service.CoreInspect(context.Background(), CoreInspectInput{
		HostEvidenceHandle: observed.HostEvidenceHandle, DeliverableID: "bridge-inspect-v4",
		InputDigest: testDigest("bridge-inspect-v4"), Proposal: workflowProposal(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Compilation == nil || inspected.Compilation.Bundle != nil || inspected.Compilation.SelectionPreview != nil || inspected.Builder == nil {
		t.Fatalf("inspection selected implicitly or omitted Builder: %#v", inspected)
	}
	profiles := make([]string, 0, len(inspected.Compilation.EligibleProfiles))
	for _, eligibility := range inspected.Compilation.EligibleProfiles {
		profiles = append(profiles, eligibility.Profile)
	}
	if !slices.Equal(profiles, []string{"ECC-FULL", "MATT-FULL", "MATT-SP-HYBRID", "SP-FULL"}) || inspected.Builder.TaxonomyVersion != catalog.TaxonomyVersionV1 {
		t.Fatalf("profiles=%q Builder=%#v", profiles, inspected.Builder)
	}
	if observed.HostSummary.VersionEvidenceDigest == "" || observed.HostSummary.VersionEvidenceDigest != inspected.HostSummary.VersionEvidenceDigest {
		t.Fatalf("Host summaries are not VersionEvidence-pinned: observed=%#v inspected=%#v", observed.HostSummary, inspected.HostSummary)
	}
}

func TestCoreCompileV4RequiresExactPreviewConfirmation(t *testing.T) {
	service := newTestService(t)
	observed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CoreCompile(context.Background(), CoreCompileInput{
		HostEvidenceHandle: observed.HostEvidenceHandle, DeliverableID: "bridge-compile-v4",
		InputDigest: testDigest("bridge-compile-v4"), Proposal: workflowProposal(),
		Selection: core.Selection{Profile: "SP-FULL", RecipeID: "oaw/delivery", ProfileSource: core.SelectionUser,
			Topology: execution.TopologyCurrent, TopologySource: core.SelectionUser, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}},
	})
	if Code(err) != "PROFILE_SELECTION_INVALID" {
		t.Fatalf("missing confirmation error = %v", err)
	}
}

func TestCoreCompileCannotReplaceCachedHostFacts(t *testing.T) {
	raw, err := json.Marshal(CoreCompileInput{})
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	object["host_session"] = map[string]any{"host_id": "forged"}
	forged, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeCoreCompileInput(forged); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("error = %v", err)
	}
}

func TestWorkflowExchangeCannotReplaceCachedHostFacts(t *testing.T) {
	raw := []byte(`{
		"host_evidence_handle":{"version":"v","session_digest":"s","cwd_digest":"c","token":"t"},
		"command":{
			"schema_version":"oaw.workflow-command/v2","kind":"START","message_id":"m",
			"idempotency_key":"i","workflow_id":"","expected_revision":0,
			"start":{
				"request_id":"r","deliverable_id":"d","input_digest":"digest","active_ticket":"",
				"proposal":{"schema_version":"oaw.classification-proposal/v1"},
				"selection":{},"host_session":{}
			}
		}
	}`)
	if _, err := DecodeWorkflowExchangeInput(raw); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("error = %v", err)
	}
}

func TestWorkflowReceiptV2RejectsCallerForgedAuthorityPins(t *testing.T) {
	raw := []byte(`{
		"host_evidence_handle":{"version":"oaw.host-evidence-handle/v2","session_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","cwd_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","token":"opaque"},
		"command":{"schema_version":"oaw.workflow-command/v2","kind":"RECEIPT","message_id":"message","idempotency_key":"key","workflow_id":"workflow-0123456789abcdef0123456789abcdef","expected_revision":2,
			"receipt":{"kind":"STARTED","workflow_id":"workflow-forged","bundle_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}}
	}`)
	if _, err := DecodeWorkflowExchangeInput(raw); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("error = %v", err)
	}
}

func TestCoreInspectAndCompileUseVerifiedCurrentFacts(t *testing.T) {
	service := newTestService(t)
	installUserProvider(t, service)

	observed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	if state := providerSummaryState(t, observed.HostSummary, "acme/suite"); state != registry.ProviderVerified {
		t.Fatalf("Provider acme/suite state = %s", state)
	}
	input := CoreInspectInput{
		HostEvidenceHandle: observed.HostEvidenceHandle, DeliverableID: "bridge-service-test",
		InputDigest: testDigest("input"), Proposal: workflowProposal(),
	}
	inspected, err := service.CoreInspect(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Classification.RequestMode != classification.RequestModeWorkflow || inspected.Compilation == nil {
		t.Fatalf("inspection = %#v", inspected)
	}
	eligibility := profileEligibility(t, inspected.Compilation, core.UserDefinedProfile)
	if !eligibility.Eligible || eligibility.RecipeID != "acme/current-delivery" || eligibility.Preview.Selection.ConfirmationDigest == "" {
		t.Fatalf("USER-DEFINED = %#v", eligibility)
	}
	for _, profile := range []string{"MATT-FULL", "ECC-FULL", "SP-FULL", "MATT-SP-HYBRID"} {
		if profileEligibility(t, inspected.Compilation, profile).Eligible {
			t.Fatalf("%s unexpectedly eligible", profile)
		}
	}
	selection := eligibility.Preview.Selection
	selection.ProfileSource = core.SelectionUser
	selection.TopologySource = core.SelectionUser
	compiled, err := service.CoreCompile(context.Background(), CoreCompileInput{
		HostEvidenceHandle: observed.HostEvidenceHandle, DeliverableID: input.DeliverableID,
		InputDigest: input.InputDigest, Proposal: input.Proposal, Selection: selection,
	})
	if err != nil {
		t.Fatal(err)
	}
	if compiled.SchemaVersion != core.LifecycleBundleSchemaV4 || compiled.Selection.Profile != core.UserDefinedProfile || compiled.ProviderInventoryDigest == "" || compiled.Graph.SchemaVersion != profile.ExecutionGraphSchemaV4 {
		t.Fatalf("compilation = %#v", compiled)
	}
}

func TestWorkflowPrepareRejectsChangedStableFactsBeforeMutation(t *testing.T) {
	service, handle, cancel := startedWorkflow(t)
	service.observer.(*fakeObserver).SetSandboxDisposition("unknown")
	changed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	if changed.HostEvidenceHandle.Token == handle.Token {
		t.Fatal("changed observation reused the old handle")
	}
	if _, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{
		HostEvidenceHandle: changed.HostEvidenceHandle, Command: WorkflowCommandInput{
			SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandPrepare,
			MessageID: "message-prepare-after-environment-change", IdempotencyKey: "prepare-after-environment-change",
			WorkflowID: cancel.WorkflowID, ExpectedRevision: cancel.ExpectedRevision,
			Prepare: &WorkflowPrepareInput{RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project-worktree"},
				TerminationCondition: "prepared", InputReferences: []WorkflowArtifactReference{}, EvidenceRequirements: []WorkflowEvidenceRequirement{}},
		},
	}); Code(err) != "HOST_SESSION_CHANGED" {
		t.Fatalf("error = %v", err)
	}
	inspected, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{
		HostEvidenceHandle: handle, Command: WorkflowCommandInput{
			SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandInspect, WorkflowID: cancel.WorkflowID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Revision != 1 {
		t.Fatalf("rejected changed facts committed revision %d", inspected.Revision)
	}
}

func TestWorkflowRecoveryCommandsRemainReachableAfterDelegationEvidenceChanges(t *testing.T) {
	for _, kind := range []coordinator.CommandKind{coordinator.CommandInspect, coordinator.CommandCancel, coordinator.CommandSwitch} {
		t.Run(string(kind), func(t *testing.T) {
			service, _, cancel := startedWorkflowWithFeatureEvidence(t)
			service.featureObserver = nil
			changed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
			if err != nil {
				t.Fatal(err)
			}
			command := WorkflowCommandInput{
				SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: kind,
				WorkflowID: cancel.WorkflowID, ExpectedRevision: cancel.ExpectedRevision,
			}
			switch kind {
			case coordinator.CommandInspect:
				result, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{HostEvidenceHandle: changed.HostEvidenceHandle, Command: command})
				if err != nil || result.Revision != 1 {
					t.Fatalf("INSPECT result=%#v error=%v", result, err)
				}
			case coordinator.CommandCancel:
				command.MessageID = "message-cancel-after-feature-change"
				command.IdempotencyKey = "cancel-after-feature-change"
				command.Cancel = &WorkflowCancelInput{Reason: "recover after feature evidence change", InvocationTerminal: true}
				result, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{HostEvidenceHandle: changed.HostEvidenceHandle, Command: command})
				if err != nil || result.Snapshot == nil || result.Snapshot.Status != coordinator.StatusCancelled {
					t.Fatalf("CANCEL result=%#v error=%v", result, err)
				}
			case coordinator.CommandSwitch:
				command.MessageID = "message-switch-after-feature-change"
				command.IdempotencyKey = "switch-after-feature-change"
				command.Switch = &WorkflowSwitchInput{Boundary: "not-yet-stable", Selection: core.Selection{}}
				_, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{HostEvidenceHandle: changed.HostEvidenceHandle, Command: command})
				if Code(err) == "HOST_SESSION_CHANGED" {
					t.Fatalf("SWITCH recovery was blocked before stable-boundary validation: %v", err)
				}
			}
		})
	}
}

func TestWorkflowReceiptKeepsOriginalDispatchPinsAfterDelegationEvidenceChanges(t *testing.T) {
	service, handle, cancel := startedWorkflowWithFeatureEvidence(t)
	prepared, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{
		HostEvidenceHandle: handle,
		Command: WorkflowCommandInput{
			SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandPrepare,
			MessageID: "message-prepare", IdempotencyKey: "prepare-before-feature-change",
			WorkflowID: cancel.WorkflowID, ExpectedRevision: cancel.ExpectedRevision,
			Prepare: &WorkflowPrepareInput{
				RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project-worktree"}, TerminationCondition: "started",
				InputReferences: []WorkflowArtifactReference{}, EvidenceRequirements: []WorkflowEvidenceRequirement{},
			},
		},
	})
	if err != nil || prepared.Dispatch == nil {
		t.Fatalf("PREPARE result=%#v error=%v", prepared, err)
	}
	service.featureObserver = nil
	changed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	started, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{
		HostEvidenceHandle: changed.HostEvidenceHandle,
		Command: WorkflowCommandInput{
			SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandReceipt,
			MessageID: "message-receipt-started", IdempotencyKey: "receipt-started-after-feature-change",
			WorkflowID: cancel.WorkflowID, ExpectedRevision: prepared.Revision,
			Receipt: &WorkflowReceiptInput{Kind: host.ReceiptStarted, Outputs: []host.OutputReference{}, Evidence: []host.EvidenceReference{}},
		},
	})
	if err != nil || started.Snapshot == nil || started.Snapshot.Status != coordinator.StatusInFlight {
		t.Fatalf("RECEIPT result=%#v error=%v", started, err)
	}
}

func TestWorkflowReceiptConvergesAfterAuthorityFactsChange(t *testing.T) {
	service, handle, cancel := startedWorkflowWithFeatureEvidence(t)
	prepared, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{
		HostEvidenceHandle: handle,
		Command: WorkflowCommandInput{
			SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandPrepare,
			MessageID: "message-prepare-before-authority-change", IdempotencyKey: "prepare-before-authority-change",
			WorkflowID: cancel.WorkflowID, ExpectedRevision: cancel.ExpectedRevision,
			Prepare: &WorkflowPrepareInput{
				RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project-worktree"}, TerminationCondition: "started",
				InputReferences: []WorkflowArtifactReference{}, EvidenceRequirements: []WorkflowEvidenceRequirement{},
			},
		},
	})
	if err != nil || prepared.Dispatch == nil {
		t.Fatalf("PREPARE result=%#v error=%v", prepared, err)
	}
	service.observer.(*fakeObserver).SetSandboxDisposition("unknown")
	changed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	started, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{
		HostEvidenceHandle: changed.HostEvidenceHandle,
		Command: WorkflowCommandInput{
			SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandReceipt,
			MessageID: "message-receipt-after-authority-change", IdempotencyKey: "receipt-after-authority-change",
			WorkflowID: cancel.WorkflowID, ExpectedRevision: prepared.Revision,
			Receipt: &WorkflowReceiptInput{Kind: host.ReceiptStarted, Outputs: []host.OutputReference{}, Evidence: []host.EvidenceReference{}},
		},
	})
	if err != nil || started.Snapshot == nil || started.Snapshot.Status != coordinator.StatusInFlight {
		t.Fatalf("RECEIPT result=%#v error=%v", started, err)
	}
}

func TestWorkflowReceiptRejectsDifferentReporterSession(t *testing.T) {
	service, handle, cancel := startedWorkflowWithFeatureEvidence(t)
	prepared, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{
		HostEvidenceHandle: handle,
		Command: WorkflowCommandInput{
			SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandPrepare,
			MessageID: "message-prepare-before-session-change", IdempotencyKey: "prepare-before-session-change",
			WorkflowID: cancel.WorkflowID, ExpectedRevision: cancel.ExpectedRevision,
			Prepare: &WorkflowPrepareInput{
				RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project-worktree"}, TerminationCondition: "started",
				InputReferences: []WorkflowArtifactReference{}, EvidenceRequirements: []WorkflowEvidenceRequirement{},
			},
		},
	})
	if err != nil || prepared.Dispatch == nil {
		t.Fatalf("PREPARE result=%#v error=%v", prepared, err)
	}
	changed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-2", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.WorkflowExchange(context.Background(), WorkflowExchangeInput{
		HostEvidenceHandle: changed.HostEvidenceHandle,
		Command: WorkflowCommandInput{
			SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandReceipt,
			MessageID: "message-receipt-after-session-change", IdempotencyKey: "receipt-after-session-change",
			WorkflowID: cancel.WorkflowID, ExpectedRevision: prepared.Revision,
			Receipt: &WorkflowReceiptInput{Kind: host.ReceiptStarted, Outputs: []host.OutputReference{}, Evidence: []host.EvidenceReference{}},
		},
	})
	if Code(err) != "HOST_SESSION_CHANGED" {
		t.Fatalf("different reporter session error=%v", err)
	}
}

func TestWorkflowPrepareRevalidatesOnlyCurrentUnitDelegationFeatures(t *testing.T) {
	for _, test := range []struct {
		name         string
		requireChild bool
		wantCode     string
	}{
		{name: "current unit has no delegation requirement", requireChild: false},
		{name: "current unit requires fresh child delegation", requireChild: true, wantCode: "HOST_FEATURE_UNATTESTED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, _, cancel := startedWorkflowWithDelegationRequirement(t, test.requireChild)
			service.featureObserver = nil
			changed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{
				HostEvidenceHandle: changed.HostEvidenceHandle,
				Command: WorkflowCommandInput{
					SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandPrepare,
					MessageID: "message-prepare-after-feature-change", IdempotencyKey: "prepare-after-feature-change",
					WorkflowID: cancel.WorkflowID, ExpectedRevision: cancel.ExpectedRevision,
					Prepare: &WorkflowPrepareInput{
						RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project-worktree"}, TerminationCondition: "prepared",
						InputReferences: []WorkflowArtifactReference{}, EvidenceRequirements: []WorkflowEvidenceRequirement{},
					},
				},
			})
			if test.wantCode != "" {
				if Code(err) != test.wantCode {
					t.Fatalf("PREPARE error=%v, want %s", err, test.wantCode)
				}
				return
			}
			if err != nil || prepared.Dispatch == nil {
				t.Fatalf("PREPARE result=%#v error=%v", prepared, err)
			}
		})
	}
}

func TestWorkflowPrepareCannotReuseHandleAfterLiveDelegationEvidenceDisappears(t *testing.T) {
	service, staleHandle, cancel := startedWorkflowWithDelegationRequirement(t, true)
	service.featureObserver = nil

	_, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{
		HostEvidenceHandle: staleHandle,
		Command: WorkflowCommandInput{
			SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandPrepare,
			MessageID: "message-prepare-with-stale-handle", IdempotencyKey: "prepare-with-stale-handle",
			WorkflowID: cancel.WorkflowID, ExpectedRevision: cancel.ExpectedRevision,
			Prepare: &WorkflowPrepareInput{
				RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project-worktree"}, TerminationCondition: "prepared",
				InputReferences: []WorkflowArtifactReference{}, EvidenceRequirements: []WorkflowEvidenceRequirement{},
			},
		},
	})
	if Code(err) != "HOST_FEATURE_UNATTESTED" {
		t.Fatalf("stale PREPARE handle error=%v", err)
	}
}

type fakeObserver struct {
	mu          sync.Mutex
	metadata    appserver.MetadataObservation
	diagnostics []appserver.ObservationDiagnostic
}

type staticFeatureObserver struct {
	result FeatureEvidenceResult
}

func (observer staticFeatureObserver) ObserveFeatures(HookContext) FeatureEvidenceResult {
	return FeatureEvidenceResult{
		Observations: append([]host.FeatureObservation{}, observer.result.Observations...),
		Diagnostics:  append([]Diagnostic{}, observer.result.Diagnostics...),
	}
}

func (observer *fakeObserver) SetDiagnostics(values []appserver.ObservationDiagnostic) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.diagnostics = append([]appserver.ObservationDiagnostic{}, values...)
}

func (observer *fakeObserver) AddSkill(value appserver.SkillMetadata) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.metadata.Skills.Skills = append(observer.metadata.Skills.Skills, value)
}

func (observer *fakeObserver) SetSandboxDisposition(value string) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.metadata.Config.SandboxDisposition = value
}

func (observer *fakeObserver) Observe(_ context.Context, cwd string) (appserver.MetadataObservation, error) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	value := observer.metadata
	value.Skills.CWD = cwd
	value.Hooks.CWD = cwd
	value.Diagnostics = append([]appserver.ObservationDiagnostic{}, observer.diagnostics...)
	return value, nil
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	projectRoot := t.TempDir()
	observer := &fakeObserver{metadata: appserver.MetadataObservation{
		Skills: appserver.SkillsEntry{Errors: []appserver.MetadataError{}, Skills: []appserver.SkillMetadata{}},
		Hooks:  appserver.HooksEntry{Errors: []appserver.MetadataError{}, Warnings: []string{}, Hooks: []appserver.HookMetadata{}},
		Config: appserver.ConfigProjection{
			CWDObserved: true, SandboxDisposition: "host-configured", MCPDisposition: "host-configured",
			HookDisposition: "host-configured", ApprovalDisposition: "host-configured",
		},
		Methods: []string{"config/read", "hooks/list", "skills/list"}, CodexVersion: "codex-cli/0.146.1",
	}}
	service, err := NewService(ServiceOptions{
		Observer: observer, Store: NewEvidenceStore(CacheOptions{MaximumEntries: 8}), StateRoot: t.TempDir(),
		ProjectRoot: projectRoot, UserConfigRoot: t.TempDir(), UserHome: t.TempDir(), BridgeVersion: "1.2.3",
		Authority: admission.AuthorityCeiling{Effects: []string{"read-project"}, Resources: []string{"project-worktree"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func testHookContext(sessionID, cwd string) HookContext {
	return HookContext{
		SchemaVersion: HookContextSchemaV2, BridgeProtocolVersion: BridgeProtocolVersion,
		SessionID: sessionID, TurnID: "turn-1", ToolUseID: "tool-1", CWD: cwd,
		Model: "gpt-test", PermissionMode: "workspace-write",
	}
}

func installUserProvider(t *testing.T, service *Service) {
	installUserProviderWithDelegation(t, service, catalog.DelegationRequirements{})
}

func installUserProviderWithDelegation(t *testing.T, service *Service, delegation catalog.DelegationRequirements) {
	t.Helper()
	providerInstallRoot := filepath.Join(service.userHome, ".codex", "plugins", "acme")
	skillRoot := filepath.Join(providerInstallRoot, "skills", "delivery")
	skillPath := filepath.Join(skillRoot, "SKILL.md")
	writeServiceFixtureFile(t, skillPath, "---\nname: acme:delivery\n---\n")
	bindingTree, err := integrity.DigestTree(skillRoot)
	if err != nil {
		t.Fatal(err)
	}
	distributionTree, err := integrity.DigestTree(providerInstallRoot)
	if err != nil {
		t.Fatal(err)
	}
	slotDefinitions := catalog.CanonicalSlots()
	claims := make([]catalog.ResponsibilityClaim, 0, len(slotDefinitions))
	stageSpan := make([]catalog.SlotID, 0, len(slotDefinitions))
	for _, slot := range slotDefinitions {
		claims = append(claims, catalog.ResponsibilityClaim{Namespace: catalog.OwnershipStage, Name: string(slot.ID), SlotID: slot.ID, OutcomeOwner: true})
		stageSpan = append(stageSpan, slot.ID)
	}
	descriptor := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0", ID: "acme/suite", DisplayName: "Acme Suite",
		Distributions: []catalog.DistributionRecord{{ID: "acme", SourceURI: "https://example.test/acme/suite", Revision: strings.Repeat("a", 40), TreeDigest: distributionTree.RootDigest}},
		Discovery: []catalog.DiscoveryProbe{{
			ID: "codex", Hosts: []string{"codex"}, Surface: "codex-plugin", DistributionID: "acme", Kind: "path-exists",
			Root: "user-home", CandidatePath: ".codex/plugins/acme", EvidencePath: "skills/delivery/SKILL.md",
		}},
		Bindings: []catalog.BindingRecord{{
			ID: "codex-delivery", DistributionID: "acme", ContentRoot: "skills/delivery", InstallRoot: "skills/delivery", TreeDigest: bindingTree.RootDigest,
			Host: "codex", Surface: "codex-plugin", Kind: catalog.BindingSkill, Reference: "acme:delivery", Invocation: catalog.InvocationModel,
			Responsibilities: claims, InputArtifact: "oaw.workflow-artifact/v1", OutputArtifact: "oaw.workflow-artifact/v1",
			MaximumEffects: []string{"git-local", "read-project", "run-process", "write-project"}, Resources: []string{"git-repository", "project-worktree"},
			SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Delegation: delegation, StageSpan: stageSpan,
			InternalCalls: []catalog.InternalCall{}, Alternatives: []string{}, Conflicts: []string{},
		}},
		Capabilities: []catalog.CapabilityRecord{{
			ID: "delivery", InputSchema: "oaw.capability-input/v1", OutcomeSchema: "oaw.capability-outcome/v1",
			RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, BindingRefs: []string{"codex-delivery"},
		}},
	}
	available, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	var recipe catalog.ProfileRecipeRecord
	for _, candidate := range available.Recipes() {
		if candidate.ID == "oaw/delivery" {
			recipe = candidate
			break
		}
	}
	if recipe.ID == "" {
		t.Fatal("built-in delivery Recipe missing")
	}
	recipe.ID = "acme/current-delivery"
	recipe.DisplayName = "Acme Current Delivery"
	recipe.Family = "user-defined"
	recipe.Template = ""
	recipe.AddOns = []catalog.AddOnRecord{}
	recipe.IncidentRoutes = []catalog.IncidentRoute{}
	recipe.Overlays = []catalog.OverlayRecord{}
	for index := range recipe.Slots {
		slotID := recipe.Slots[index].SlotID
		stepID := "acme-" + string(slotID)
		recipe.Slots[index].Pipeline = []catalog.PipelineStep{{
			ID: stepID, Selector: catalog.BindingSelector{ProviderID: "acme/suite", BindingID: "codex-delivery"}, StageSpan: []catalog.SlotID{slotID},
			RequiredInputArtifact: "oaw.workflow-artifact/v1", ProducedOutputArtifact: "oaw.workflow-artifact/v1",
		}}
		recipe.Slots[index].OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerProviderBinding, StepID: stepID}
		recipe.Slots[index].HostAction = nil
	}
	providerRoot := filepath.Join(service.userConfigRoot, "providers")
	recipeRoot := filepath.Join(service.userConfigRoot, "recipes")
	if err := os.MkdirAll(providerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(recipeRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(providerRoot, "acme.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	rawRecipe, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recipeRoot, "acme.json"), rawRecipe, 0o600); err != nil {
		t.Fatal(err)
	}
	contents := "schema_version = \"oaw.user-config/v3\"\n[[provider_descriptors]]\nid = \"acme/suite\"\npath = \"providers/acme.json\"\n" +
		"[[profile_recipes]]\nid = \"acme/current-delivery\"\npath = \"recipes/acme.json\"\n"
	if err := os.WriteFile(filepath.Join(service.userConfigRoot, "config.toml"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if observer, ok := service.observer.(*fakeObserver); ok {
		observer.AddSkill(appserver.SkillMetadata{Name: "acme:delivery", Enabled: true, Path: skillPath, Scope: "user"})
	}
}

func writeServiceFixtureFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func providerSummaryState(t *testing.T, summary HostSummary, providerID string) registry.ProviderState {
	t.Helper()
	for _, provider := range summary.Providers {
		if provider.ProviderID == providerID {
			return provider.State
		}
	}
	t.Fatalf("Provider %s missing from %#v", providerID, summary.Providers)
	return ""
}

func profileEligibility(t *testing.T, result *core.CompilationResult, profile string) core.ProfileEligibility {
	t.Helper()
	for _, value := range result.EligibleProfiles {
		if value.Profile == profile {
			return value
		}
	}
	t.Fatalf("Profile %s missing from %#v", profile, result.EligibleProfiles)
	return core.ProfileEligibility{}
}

func workflowProposal() classification.ClassificationProposal {
	return classification.ClassificationProposal{SchemaVersion: classification.ProposalSchemaV1}
}

func testDigest(value string) string {
	return canonicaljson.DigestBytes([]byte(value))
}

func startedWorkflow(t *testing.T) (*Service, HostEvidenceHandle, coordinator.Command) {
	t.Helper()
	service := newTestService(t)
	return startWorkflowWithService(t, service)
}

func startedWorkflowWithFeatureEvidence(t *testing.T) (*Service, HostEvidenceHandle, coordinator.Command) {
	return startedWorkflowWithDelegationRequirement(t, false)
}

func startedWorkflowWithDelegationRequirement(t *testing.T, requireChild bool) (*Service, HostEvidenceHandle, coordinator.Command) {
	t.Helper()
	service := newTestService(t)
	live, err := host.NewFeatureObservation(host.FeatureObservation{
		Feature: host.FeatureChildDelegation, State: host.AvailabilityAvailable, Source: host.SourceNativeAPI,
		EvidenceReference: "evidence://codex/subagent-start/workflow-recovery",
	})
	if err != nil {
		t.Fatal(err)
	}
	service.featureObserver = staticFeatureObserver{result: FeatureEvidenceResult{Observations: []host.FeatureObservation{live}}}
	delegation := catalog.DelegationRequirements{}
	delegation.Child = requireChild
	return startWorkflowWithServiceAndDelegation(t, service, delegation)
}

func startWorkflowWithService(t *testing.T, service *Service) (*Service, HostEvidenceHandle, coordinator.Command) {
	return startWorkflowWithServiceAndDelegation(t, service, catalog.DelegationRequirements{})
}

func startWorkflowWithServiceAndDelegation(t *testing.T, service *Service, delegation catalog.DelegationRequirements) (*Service, HostEvidenceHandle, coordinator.Command) {
	t.Helper()
	installUserProviderWithDelegation(t, service, delegation)
	observed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := service.CoreInspect(context.Background(), CoreInspectInput{HostEvidenceHandle: observed.HostEvidenceHandle,
		DeliverableID: "bridge-service-workflow", InputDigest: testDigest("workflow-input"), Proposal: workflowProposal()})
	if err != nil {
		t.Fatal(err)
	}
	selection := profileEligibility(t, inspected.Compilation, core.UserDefinedProfile).Preview.Selection
	selection.ProfileSource = core.SelectionUser
	selection.TopologySource = core.SelectionUser
	bundle, err := service.CoreCompile(context.Background(), CoreCompileInput{HostEvidenceHandle: observed.HostEvidenceHandle,
		DeliverableID: "bridge-service-workflow", InputDigest: testDigest("workflow-input"), Proposal: workflowProposal(), Selection: selection})
	if err != nil {
		t.Fatal(err)
	}
	start := coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandStart,
		MessageID: "message-start", IdempotencyKey: "bridge-service-start",
		Start: &coordinator.StartInput{
			RequestID: "request-1", DeliverableID: "bridge-service-workflow", InputDigest: testDigest("workflow-input"),
			Proposal: workflowProposal(), Selection: bundle.Selection,
		},
	}
	started, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{HostEvidenceHandle: observed.HostEvidenceHandle, Command: publicCommand(start)})
	if err != nil {
		t.Fatal(err)
	}
	return service, observed.HostEvidenceHandle, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandCancel,
		MessageID: "message-cancel", IdempotencyKey: "bridge-service-cancel", WorkflowID: started.WorkflowID,
		ExpectedRevision: started.Revision, Cancel: &coordinator.CancelInput{Reason: "test cancellation", InvocationTerminal: true},
	}
}

func publicCommand(command coordinator.Command) WorkflowCommandInput {
	result := WorkflowCommandInput{
		SchemaVersion: command.SchemaVersion, Kind: command.Kind, MessageID: command.MessageID,
		IdempotencyKey: command.IdempotencyKey, WorkflowID: command.WorkflowID,
		ExpectedRevision: command.ExpectedRevision,
	}
	if command.Start != nil {
		result.Start = &WorkflowStartInput{
			RequestID: command.Start.RequestID, DeliverableID: command.Start.DeliverableID,
			InputDigest: command.Start.InputDigest, ActiveTicket: command.Start.ActiveTicket,
			Proposal: command.Start.Proposal, Selection: command.Start.Selection,
		}
	}
	if command.Switch != nil {
		result.Switch = &WorkflowSwitchInput{Boundary: command.Switch.Boundary, Selection: command.Switch.Selection}
	}
	if command.Prepare != nil {
		result.Prepare = &WorkflowPrepareInput{
			RequestedEffects: append([]string{}, command.Prepare.RequestedEffects...), RequestedResources: append([]string{}, command.Prepare.RequestedResources...),
			TerminationCondition: command.Prepare.TerminationCondition,
			InputReferences:      make([]WorkflowArtifactReference, len(command.Prepare.InputReferences)),
			EvidenceRequirements: make([]WorkflowEvidenceRequirement, len(command.Prepare.EvidenceRequirements)),
		}
		for index, value := range command.Prepare.InputReferences {
			result.Prepare.InputReferences[index] = WorkflowArtifactReference{Kind: value.Kind, Reference: value.Reference, Digest: value.Digest}
		}
		for index, value := range command.Prepare.EvidenceRequirements {
			result.Prepare.EvidenceRequirements[index] = WorkflowEvidenceRequirement{Kind: value.Kind, Minimum: value.Minimum, Description: value.Description}
		}
	}
	if command.Receipt != nil {
		value := command.Receipt.Receipt
		result.Receipt = &WorkflowReceiptInput{
			Kind: value.Kind, Outcome: value.Outcome, FailureCode: value.FailureCode,
			Outputs: append([]host.OutputReference{}, value.Outputs...), Evidence: append([]host.EvidenceReference{}, value.Evidence...),
			Signal: command.Receipt.Signal, StableBoundary: command.Receipt.StableBoundary,
		}
	}
	if command.Cancel != nil {
		result.Cancel = &WorkflowCancelInput{Reason: command.Cancel.Reason, InvocationTerminal: command.Cancel.InvocationTerminal}
	}
	return result
}
