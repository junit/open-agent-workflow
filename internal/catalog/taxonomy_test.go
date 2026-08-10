package catalog

import "testing"

func TestCanonicalSlotsAreStableAndDefensive(t *testing.T) {
	want := []SlotDefinition{
		{SlotProblemFraming, "Requirements and domain alignment", MachineStage, "Purpose, constraints, domain terms, decisions, and success conditions are user-aligned."},
		{SlotSolutionSpecification, "Solution specification and test boundaries", MachineStage, "A reviewable solution specification and test boundaries are approved."},
		{SlotDeliveryPlanning, "Delivery planning, decomposition, and acceptance items", MachineStage, "Work is decomposed into independently verifiable units sufficient for the selected executor."},
		{SlotWorkspacePreparation, "Workspace preparation", MachineHostActionGate, "The selected workspace is safe, initialized, and has a known baseline."},
		{SlotImplementation, "Implementation execution", MachineStage, "Approved changes are produced with bounded effects and progress evidence."},
		{SlotImplementationTDD, "TDD and implementation testing", MachineProcedure, "Expected behavior drives a witnessed RED/GREEN cycle and focused tests."},
		{SlotIncidentRecovery, "Conditional debugging and repair", MachineIncidentHandler, "A typed unexpected failure is diagnosed and returns to a declared stage, replans, or stops."},
		{SlotReviewRemediation, "Review and remediation", MachineAssuranceLoop, "Findings are reported, fixed or adjudicated, and re-reviewed."},
		{SlotFreshVerification, "Fresh final verification", MachineHostProviderGate, "Claim-relevant commands run after remediation and produce fresh evidence."},
		{SlotCloseout, "Completion and delivery", MachineTerminalSequence, "Acceptance is reconciled and the user-authorized delivery or preservation action is recorded."},
	}

	got := CanonicalSlots()
	if len(got) != len(want) {
		t.Fatalf("slot count = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("slot[%d] = %#v, want %#v", index, got[index], want[index])
		}
	}

	got[0].ID = "changed"
	if CanonicalSlots()[0].ID != SlotProblemFraming {
		t.Fatal("CanonicalSlots exposed package-owned storage")
	}
}
