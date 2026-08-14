package policyflow_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
)

func TestStateRoundTripPreservesCurrentReferenceAndProgress(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileMattFull)
	wantRef := currentRef(t, progress.Next).String()

	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	var decoded policyflow.State
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}

	restarted := policyflow.New()
	restored, err := restarted.Restore(inventory, decoded)
	if err != nil {
		t.Fatal(err)
	}
	if got := currentRef(t, restored.Next).String(); got != wantRef {
		t.Fatalf("current ref changed across restore: want %q got %q", wantRef, got)
	}
	if !reflect.DeepEqual(restored, progress) {
		t.Fatalf("restored progress differs:\nwant %#v\ngot  %#v", progress, restored)
	}

	next := restored.Next.(policyflow.AwaitUserSkill)
	if _, err := restarted.Apply(inventory, policyflow.SkillCompleted{WorkRef: next.WorkRef}); err != nil {
		t.Fatal(err)
	}
}

func TestStateRoundTripPreservesAwaitUserSkillDiscriminator(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileMattFull)
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Next.Kind != "await-user-skill" {
		t.Fatalf("next state = %#v", saved.Next)
	}

	restarted := policyflow.New()
	restored, err := restarted.Restore(inventory, saved)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := restored.Next.(policyflow.AwaitUserSkill); !ok {
		t.Fatalf("restored next = %T", restored.Next)
	}
}

func TestStateRoundTripPreservesTerminalState(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	progress, err := module.Apply(inventory, policyflow.StopRequested{
		Current: currentRef(t, progress.Next), Reason: "cancelled",
	})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}

	restored, err := policyflow.New().Restore(inventory, saved)
	if err != nil {
		t.Fatal(err)
	}
	stopped, ok := restored.Next.(policyflow.Stopped)
	if !ok || stopped.Code != policyflow.StopExplicit || stopped.Reason != "cancelled" {
		t.Fatalf("restored terminal = %#v", restored.Next)
	}
}

func TestStateRoundTripAfterProfileSwitchReplaysFromInitialProfile(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	progress = advancePastGate(t, module, inventory, progress, "specification-approved")
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	progress, err = module.Apply(inventory, policyflow.ProfileSwitchRequested{
		Current: currentRef(t, progress.Next), OfferRef: offer.Ref, Profile: policyflow.ProfileMattFull,
	})
	if err != nil {
		t.Fatal(err)
	}

	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	restarted := policyflow.New()
	restored, err := restarted.Restore(inventory, saved)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored, progress) {
		t.Fatalf("restored switched progress differs:\nwant %#v\ngot  %#v", progress, restored)
	}
	if restored.Plan.Profile != policyflow.ProfileMattFull {
		t.Fatalf("restored profile = %q", restored.Plan.Profile)
	}
	next, ok := restored.Next.(policyflow.AwaitUserSkill)
	if !ok || next.Skill != "to-tickets" {
		t.Fatalf("restored next after switch = %#v", restored.Next)
	}
	if _, err := restarted.Apply(inventory, policyflow.SkillCompleted{WorkRef: next.WorkRef}); err != nil {
		t.Fatal(err)
	}
}

func TestStateRoundTripPreservesReviewRemediationAndFreshReview(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	progress = advanceToSkill(t, module, inventory, progress, "superpowers:requesting-code-review")
	review := progress.Next.(policyflow.InvokeSkill)
	progress = applyOK(t, module, inventory, policyflow.ReviewCompleted{
		WorkRef: review.WorkRef, Outcome: policyflow.ReviewFindings,
	})

	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	restarted := policyflow.New()
	progress, err = restarted.Restore(inventory, saved)
	if err != nil {
		t.Fatal(err)
	}
	remediation, ok := progress.Next.(policyflow.InvokeSkill)
	if !ok || remediation.Skill != "superpowers:executing-plans" {
		t.Fatalf("restored remediation work = %#v", progress.Next)
	}
	progress = applyOK(t, restarted, inventory, policyflow.SkillCompleted{WorkRef: remediation.WorkRef})
	reviewAgain, ok := progress.Next.(policyflow.InvokeSkill)
	if !ok || reviewAgain.Skill != "superpowers:requesting-code-review" || !reviewAgain.Review {
		t.Fatalf("fresh review work = %#v", progress.Next)
	}

	saved, err = restarted.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	restartedAgain := policyflow.New()
	progress, err = restartedAgain.Restore(inventory, saved)
	if err != nil {
		t.Fatal(err)
	}
	reviewAgain = progress.Next.(policyflow.InvokeSkill)
	progress = applyOK(t, restartedAgain, inventory, policyflow.ReviewCompleted{
		WorkRef: reviewAgain.WorkRef, Outcome: policyflow.ReviewClean,
	})
	if next, ok := progress.Next.(policyflow.InvokeSkill); !ok || next.Skill != "superpowers:receiving-code-review" {
		t.Fatalf("clean restored review did not continue pipeline: %#v", progress.Next)
	}
}

func TestStateRoundTripPreservesECCHostReviewTypedOutcomeContract(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileECCFull)
	progress = advanceToHostAction(t, module, inventory, progress, "review.execute")

	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	var decoded policyflow.State
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}

	restarted := policyflow.New()
	progress, err = restarted.Restore(inventory, decoded)
	if err != nil {
		t.Fatal(err)
	}
	review, ok := progress.Next.(policyflow.HostAction)
	if !ok || review.Action != "review.execute" || !review.Review {
		t.Fatalf("restored ECC review Host action = %#v", progress.Next)
	}
	if _, err := restarted.Apply(inventory, policyflow.HostActionCompleted{WorkRef: review.WorkRef}); err == nil {
		t.Fatal("restored ECC review accepted untyped Host action completion")
	} else {
		requireFailureCode(t, err, policyflow.FailureEventOutOfOrder)
	}

	progress = applyOK(t, restarted, inventory, policyflow.ReviewCompleted{
		WorkRef: review.WorkRef, Outcome: policyflow.ReviewFindings,
	})
	remediation, ok := progress.Next.(policyflow.InvokeSkill)
	if !ok || remediation.Skill != "ecc:tdd-workflow" {
		t.Fatalf("ECC review remediation work = %#v", progress.Next)
	}
	progress = applyOK(t, restarted, inventory, policyflow.SkillCompleted{WorkRef: remediation.WorkRef})
	reviewAgain, ok := progress.Next.(policyflow.HostAction)
	if !ok || reviewAgain.Action != "review.execute" || !reviewAgain.Review {
		t.Fatalf("fresh ECC review Host action = %#v", progress.Next)
	}
	if reviewAgain.WorkRef.String() == review.WorkRef.String() {
		t.Fatalf("fresh ECC review reused work reference %q", reviewAgain.WorkRef.String())
	}
	if _, err := restarted.Apply(inventory, policyflow.HostActionCompleted{WorkRef: reviewAgain.WorkRef}); err == nil {
		t.Fatal("fresh ECC review accepted untyped Host action completion")
	} else {
		requireFailureCode(t, err, policyflow.FailureEventOutOfOrder)
	}

	progress = applyOK(t, restarted, inventory, policyflow.ReviewCompleted{
		WorkRef: reviewAgain.WorkRef, Outcome: policyflow.ReviewClean,
	})
	verification, ok := progress.Next.(policyflow.InvokeSkill)
	if !ok || verification.Skill != "ecc:verification-loop" {
		t.Fatalf("clean ECC review did not continue to verification: %#v", progress.Next)
	}
}

func TestRestoreRejectsTamperedCurrentWork(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileMattFull)
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	saved.Next.Name = "implement"

	_, err = policyflow.New().Restore(inventory, saved)
	requireFailureCode(t, err, policyflow.FailureSemanticsInvalid)
}

func TestRestoreRejectsTamperedInvocationMode(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileMattFull)
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	saved.Next.Kind = "invoke-skill"

	_, err = policyflow.New().Restore(inventory, saved)
	requireFailureCode(t, err, policyflow.FailureSemanticsInvalid)
}

func TestStateRoundTripPreservesAwaitUserIncidentSkill(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileMattFull)
	progress = advanceToSkill(t, module, inventory, progress, "implement")
	current := progress.Next.(policyflow.AwaitUserSkill)
	progress, err := module.Apply(inventory, policyflow.IncidentReported{
		WorkRef: current.WorkRef, Incident: policyflow.IncidentFunctionalFailure, Reason: "failed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := progress.Next.(policyflow.AwaitUserSkill); !ok {
		t.Fatalf("incident next = %T", progress.Next)
	}
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}

	restored, err := policyflow.New().Restore(inventory, saved)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := restored.Next.(policyflow.AwaitUserSkill); !ok {
		t.Fatalf("restored incident next = %T", restored.Next)
	}
}

func TestRestoreRejectsInventoryDrift(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileMattFull)
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}

	_, err = policyflow.New().Restore(removeRoute(inventory, "implement"), saved)
	requireFailureCode(t, err, policyflow.FailureInventoryDrift)
}

func TestRestoreIgnoresUnrelatedProfileRouteDrift(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := policyflow.New().Restore(removeRoute(inventory, "implement"), saved); err != nil {
		t.Fatalf("unrelated Matt route prevented SP-FULL restore: %v", err)
	}
}

func TestRestoreRejectsProfileSemanticsDrift(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileMattFull)
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	saved.SemanticsDigest = "different-semantics"

	_, err = policyflow.New().Restore(inventory, saved)
	requireFailureCode(t, err, policyflow.FailureSemanticsInvalid)
}

func TestRestoreRejectsTerminalStateWithActiveIncidentMetadata(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	progress, err := module.Apply(inventory, policyflow.StopRequested{
		Current: currentRef(t, progress.Next), Reason: "cancelled",
	})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	saved.InIncident = true
	saved.ReturnIndex = 2

	_, err = policyflow.New().Restore(inventory, saved)
	requireFailureCode(t, err, policyflow.FailureSemanticsInvalid)
}

func TestRestoreRejectsActiveStateWithTerminalNextKind(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileMattFull)
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	saved.Next = policyflow.NextState{Kind: "done", Ref: saved.IntentRef}

	_, err = policyflow.New().Restore(inventory, saved)
	requireFailureCode(t, err, policyflow.FailureSemanticsInvalid)
}

func TestRestoreRejectsForgedDerivedProgress(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*policyflow.State)
	}{
		{"completed slots", func(state *policyflow.State) {
			state.CompletedSlots = []policyflow.LifecycleSlot{policyflow.SlotCloseout}
		}},
		{"completed gates", func(state *policyflow.State) {
			state.CompletedGates = []string{"user-closeout"}
		}},
		{"stable boundaries", func(state *policyflow.State) {
			state.StableBoundaries = []string{"verification-complete"}
		}},
		{"switch boundary", func(state *policyflow.State) {
			state.SwitchBoundary = "verification-complete"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tampered := saved
			test.mutate(&tampered)
			_, err := policyflow.New().Restore(inventory, tampered)
			requireFailureCode(t, err, policyflow.FailureSemanticsInvalid)
		})
	}
}

func TestRestoreRejectsForgedTerminalCompletion(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	saved.StepIndex = 14
	saved.IntentRef = ""
	saved.Next = policyflow.NextState{Kind: "done"}
	saved.CompletedSlots = []policyflow.LifecycleSlot{
		policyflow.SlotProblemFraming, policyflow.SlotSolutionSpecification,
		policyflow.SlotDeliveryPlanning, policyflow.SlotWorkspacePreparation,
		policyflow.SlotImplementation, policyflow.SlotImplementationTDD,
		policyflow.SlotReviewRemediation, policyflow.SlotFreshVerification,
		policyflow.SlotCloseout,
	}
	saved.CompletedGates = []string{
		"shared-understanding", "specification-approved", "delivery-plan-approved",
		"fresh-evidence", "user-closeout",
	}

	_, err = policyflow.New().Restore(inventory, saved)
	requireFailureCode(t, err, policyflow.FailureSemanticsInvalid)
}

func TestRestoreRejectsTamperedIncidentIdentityAndReturn(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileMattFull)
	progress = advanceToSkill(t, module, inventory, progress, "implement")
	current := progress.Next.(policyflow.AwaitUserSkill)
	progress = applyOK(t, module, inventory, policyflow.IncidentReported{
		WorkRef: current.WorkRef, Incident: policyflow.IncidentFunctionalFailure,
	})
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*policyflow.State)
	}{
		{"return index", func(state *policyflow.State) { state.ReturnIndex = -1 }},
		{"incident type", func(state *policyflow.State) { state.ActiveIncident = policyflow.IncidentHardBug }},
		{"handler", func(state *policyflow.State) { state.IncidentHandler = "implement" }},
		{"return slot", func(state *policyflow.State) { state.IncidentReturnSlot = policyflow.SlotFreshVerification }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tampered := saved
			test.mutate(&tampered)
			_, err := policyflow.New().Restore(inventory, tampered)
			requireFailureCode(t, err, policyflow.FailureSemanticsInvalid)
		})
	}
}

func TestRestoreRejectsTamperedTransitionRouteInventory(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	current := progress.Next.(policyflow.InvokeSkill)
	progress = applyOK(t, module, inventory, policyflow.SkillCompleted{WorkRef: current.WorkRef})
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.History) != 1 {
		t.Fatalf("history length = %d", len(saved.History))
	}
	saved.History[0].Inventory = removeRoute(saved.History[0].Inventory, "superpowers:brainstorming")

	_, err = policyflow.New().Restore(inventory, saved)
	requireFailureCode(t, err, policyflow.FailureSemanticsInvalid)
}

func TestStateSchemaRejectsPreviousReducerSnapshots(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	if saved.SchemaVersion != policyflow.StateSchemaV4 {
		t.Fatalf("schema version = %q", saved.SchemaVersion)
	}
	saved.SchemaVersion = "oaw.policy-flow-state/v3"

	_, err = policyflow.New().Restore(inventory, saved)
	requireFailureCode(t, err, policyflow.FailureSemanticsInvalid)
}

func TestStateProgressRejectsPreviousReducerSnapshots(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	saved, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	saved.SchemaVersion = "oaw.policy-flow-state/v3"

	_, err = saved.Progress()
	requireFailureCode(t, err, policyflow.FailureSemanticsInvalid)
}

func TestExportRejectsAmbiguousTerminalProgress(t *testing.T) {
	inventory := completeInventory()
	module := policyflow.New()
	for range 2 {
		progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
		if _, err := module.Apply(inventory, policyflow.StopRequested{
			Current: currentRef(t, progress.Next), Reason: "same reason",
		}); err != nil {
			t.Fatal(err)
		}
	}

	_, err := module.Export(policyflow.Progress{Next: policyflow.Stopped{
		Code: policyflow.StopExplicit, Reason: "same reason",
	}})
	requireFailureCode(t, err, policyflow.FailureEventOutOfOrder)
}
