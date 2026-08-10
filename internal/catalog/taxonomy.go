package catalog

type SlotID string

const (
	SlotProblemFraming        SlotID = "problem-framing"
	SlotSolutionSpecification SlotID = "solution-specification"
	SlotDeliveryPlanning      SlotID = "delivery-planning"
	SlotWorkspacePreparation  SlotID = "workspace-preparation"
	SlotImplementation        SlotID = "implementation"
	SlotImplementationTDD     SlotID = "implementation-tdd"
	SlotIncidentRecovery      SlotID = "incident-recovery"
	SlotReviewRemediation     SlotID = "review-remediation"
	SlotFreshVerification     SlotID = "fresh-verification"
	SlotCloseout              SlotID = "closeout"
)

type MachineKind string

const (
	MachineStage            MachineKind = "stage"
	MachineHostActionGate   MachineKind = "host-action+neutral-gate"
	MachineProcedure        MachineKind = "procedure"
	MachineIncidentHandler  MachineKind = "incident-handler"
	MachineAssuranceLoop    MachineKind = "assurance-loop"
	MachineHostProviderGate MachineKind = "host-or-provider-procedure+neutral-gate"
	MachineTerminalSequence MachineKind = "terminal-sequence+user-gate"
)

type SlotDefinition struct {
	ID              SlotID      `json:"id"`
	DisplayName     string      `json:"display_name"`
	MachineKind     MachineKind `json:"machine_kind"`
	RequiredOutcome string      `json:"required_outcome"`
}

var canonicalSlots = [...]SlotDefinition{
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

func CanonicalSlots() []SlotDefinition {
	return append([]SlotDefinition(nil), canonicalSlots[:]...)
}
