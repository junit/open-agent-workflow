// Package policyflow projects built-in workflow semantics onto Host-observed
// routes. It is cooperative policy state, not machine execution authority.
package policyflow

import "fmt"

type InvocationMode string

const (
	HostVisible    InvocationMode = "host-visible"
	UserExplicit   InvocationMode = "user-explicit"
	HostControlled InvocationMode = "host-controlled"
)

type RouteKind string

const (
	RouteSkill      RouteKind = "skill"
	RouteHostAction RouteKind = "host-action"
	RouteUserGate   RouteKind = "user-gate"
	RouteHostGate   RouteKind = "host-gate"
)

type Route struct {
	Name string         `json:"name"`
	Mode InvocationMode `json:"mode"`
}

type RouteInventory []Route

type ProfileID string

const (
	ProfileSPFull       ProfileID = "SP-FULL"
	ProfileMattFull     ProfileID = "MATT-FULL"
	ProfileECCFull      ProfileID = "ECC-FULL"
	ProfileMattSPHybrid ProfileID = "MATT-SP-HYBRID"
)

type LifecycleSlot string

const (
	SlotProblemFraming        LifecycleSlot = "problem-framing"
	SlotSolutionSpecification LifecycleSlot = "solution-specification"
	SlotDeliveryPlanning      LifecycleSlot = "delivery-planning"
	SlotWorkspacePreparation  LifecycleSlot = "workspace-preparation"
	SlotImplementation        LifecycleSlot = "implementation"
	SlotImplementationTDD     LifecycleSlot = "implementation-tdd"
	SlotIncidentRecovery      LifecycleSlot = "incident-recovery"
	SlotReviewRemediation     LifecycleSlot = "review-remediation"
	SlotFreshVerification     LifecycleSlot = "fresh-verification"
	SlotCloseout              LifecycleSlot = "closeout"
)

type OfferRef struct{ value string }
type WorkRef struct{ value string }
type GateRef struct{ value string }

// CurrentRef is the opaque reference attached to the current work or gate.
// Callers can pass a reference back to Apply but cannot construct one.
type CurrentRef interface {
	String() string
	policyCurrentRef()
}

func (ref OfferRef) String() string { return ref.value }
func (ref WorkRef) String() string  { return ref.value }
func (ref GateRef) String() string  { return ref.value }

func (ref OfferRef) MarshalText() ([]byte, error) { return []byte(ref.value), nil }
func (ref WorkRef) MarshalText() ([]byte, error)  { return []byte(ref.value), nil }
func (ref GateRef) MarshalText() ([]byte, error)  { return []byte(ref.value), nil }

func (ref *OfferRef) UnmarshalText(value []byte) error {
	parsed, err := OfferReference(string(value))
	if err != nil {
		return err
	}
	*ref = parsed
	return nil
}

func (ref *WorkRef) UnmarshalText(value []byte) error {
	parsed, err := WorkReference(string(value))
	if err != nil {
		return err
	}
	*ref = parsed
	return nil
}

func (ref *GateRef) UnmarshalText(value []byte) error {
	parsed, err := GateReference(string(value))
	if err != nil {
		return err
	}
	*ref = parsed
	return nil
}

func (WorkRef) policyCurrentRef() {}
func (GateRef) policyCurrentRef() {}

type RouteStatus struct {
	Name      string          `json:"name"`
	Kind      RouteKind       `json:"kind"`
	Mode      InvocationMode  `json:"mode,omitempty"`
	Available bool            `json:"available"`
	Covers    []LifecycleSlot `json:"covers"`
	Credited  bool            `json:"credited,omitempty"`
}

type ProfileOffer struct {
	ID               ProfileID             `json:"id"`
	PolicySelectable bool                  `json:"policy_selectable"`
	HostRoutable     bool                  `json:"host_routable"`
	Routes           []RouteStatus         `json:"routes"`
	Missing          []RouteStatus         `json:"missing"`
	IncidentRoutes   []IncidentRouteStatus `json:"incident_routes"`
}

type Offer struct {
	Ref      OfferRef       `json:"ref"`
	Profiles []ProfileOffer `json:"profiles"`
}

const (
	TopologyCurrent = "CURRENT"
	AddOnNone       = "NONE"
)

type Selection struct {
	OfferRef OfferRef
	Profile  ProfileID
}

type Plan struct {
	Profile  ProfileID `json:"profile"`
	Topology string    `json:"topology"`
	AddOn    string    `json:"add_on"`
}

type NextWork interface{ policyNextWork() }

type InvokeSkill struct {
	WorkRef WorkRef         `json:"work_ref"`
	Skill   string          `json:"skill"`
	Covers  []LifecycleSlot `json:"covers"`
	Review  bool            `json:"review,omitempty"`
}

func (InvokeSkill) policyNextWork() {}

type AwaitUserSkill struct {
	WorkRef WorkRef         `json:"work_ref"`
	Skill   string          `json:"skill"`
	Covers  []LifecycleSlot `json:"covers"`
	Review  bool            `json:"review,omitempty"`
}

func (AwaitUserSkill) policyNextWork() {}

type HostAction struct {
	WorkRef WorkRef         `json:"work_ref"`
	Action  string          `json:"action"`
	Covers  []LifecycleSlot `json:"covers"`
	Review  bool            `json:"review,omitempty"`
}

func (HostAction) policyNextWork() {}

type UserGate struct {
	GateRef GateRef `json:"gate_ref"`
	Gate    string  `json:"gate"`
}

func (UserGate) policyNextWork() {}

type HostGate struct {
	GateRef GateRef `json:"gate_ref"`
	Gate    string  `json:"gate"`
}

func (HostGate) policyNextWork() {}

type Done struct{}

func (Done) policyNextWork() {}

type StopCode string

const (
	StopExplicit                   StopCode = "EXPLICIT_STOP"
	StopIncidentHandlerUnavailable StopCode = "INCIDENT_HANDLER_UNAVAILABLE"
	StopIncidentUnhandled          StopCode = "INCIDENT_UNHANDLED"
)

type Stopped struct {
	Code     StopCode     `json:"code"`
	Reason   string       `json:"reason"`
	Incident IncidentType `json:"incident,omitempty"`
}

func (Stopped) policyNextWork() {}

type BlockCode string

const BlockExecutionUncertain BlockCode = "EXECUTION_UNCERTAIN"

type Blocked struct {
	Code         BlockCode `json:"code"`
	Reason       string    `json:"reason"`
	RetryAllowed bool      `json:"retry_allowed"`
}

func (Blocked) policyNextWork() {}

type Progress struct {
	Plan             Plan            `json:"plan"`
	CompletedSlots   []LifecycleSlot `json:"completed_slots"`
	CompletedGates   []string        `json:"completed_gates"`
	StableBoundaries []string        `json:"stable_boundaries"`
	Next             NextWork        `json:"next"`
}

type Event interface{ policyEvent() }

type SkillCompleted struct{ WorkRef WorkRef }

func (SkillCompleted) policyEvent() {}

type ReviewOutcome string

const (
	ReviewClean    ReviewOutcome = "clean"
	ReviewFindings ReviewOutcome = "findings"
)

// ReviewCompleted is required for review-producing work. Findings send the
// reducer back through implementation and a fresh review; only a clean outcome
// may advance the review pipeline.
type ReviewCompleted struct {
	WorkRef WorkRef
	Outcome ReviewOutcome
}

func (ReviewCompleted) policyEvent() {}

type HostActionCompleted struct{ WorkRef WorkRef }

func (HostActionCompleted) policyEvent() {}

type UserGateApproved struct{ GateRef GateRef }

func (UserGateApproved) policyEvent() {}

type HostGateSatisfied struct{ GateRef GateRef }

func (HostGateSatisfied) policyEvent() {}

type IncidentType string

const (
	IncidentBuildFailure          IncidentType = "build-failure"
	IncidentDependencyFailure     IncidentType = "dependency-failure"
	IncidentFunctionalFailure     IncidentType = "functional-failure"
	IncidentHardBug               IncidentType = "hard-bug"
	IncidentPerformanceRegression IncidentType = "performance-regression"
	IncidentTypeFailure           IncidentType = "type-failure"
)

type IncidentRouteStatus struct {
	Incident  IncidentType   `json:"incident"`
	Skill     string         `json:"skill,omitempty"`
	Mode      InvocationMode `json:"mode,omitempty"`
	Available bool           `json:"available"`
}

// IncidentReported consumes the current work reference and enters the
// Recipe-declared recovery route. The Recipe, not the caller, selects where
// successful recovery returns.
type IncidentReported struct {
	WorkRef  WorkRef
	Incident IncidentType
	Reason   string
}

func (IncidentReported) policyEvent() {}

// ProfileSwitchRequested replaces the remaining Profile program at a stable
// lifecycle point. Completed slots and gates stay credited.
type ProfileSwitchRequested struct {
	Current  CurrentRef
	OfferRef OfferRef
	Profile  ProfileID
}

func (ProfileSwitchRequested) policyEvent() {}

type StopRequested struct {
	Current CurrentRef
	Reason  string
}

func (StopRequested) policyEvent() {}

// ExecutionUncertain records that work may have produced an external effect
// but its outcome is unknown. It is terminal and cannot be retried in place.
type ExecutionUncertain struct {
	WorkRef WorkRef
	Reason  string
}

func (ExecutionUncertain) policyEvent() {}

type FailureCode string

const (
	FailureInventoryInvalid  FailureCode = "ROUTE_INVENTORY_INVALID"
	FailureSemanticsInvalid  FailureCode = "PROFILE_SEMANTICS_INVALID"
	FailureOfferStale        FailureCode = "OFFER_STALE"
	FailureProfileUnknown    FailureCode = "PROFILE_UNKNOWN"
	FailureProfileIncomplete FailureCode = "PROFILE_INCOMPLETE"
	FailureEventOutOfOrder   FailureCode = "EVENT_OUT_OF_ORDER"
	FailureInventoryDrift    FailureCode = "ROUTE_INVENTORY_DRIFT"
	FailureSwitchNotStable   FailureCode = "PROFILE_SWITCH_NOT_STABLE"
)

type Failure struct {
	Code   FailureCode
	Detail string
}

func (failure *Failure) Error() string {
	if failure == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", failure.Code, failure.Detail)
}
