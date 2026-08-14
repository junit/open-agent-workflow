package policyflow

import "strings"

const StateSchemaV4 = "oaw.policy-flow-state/v4"

type NextState struct {
	Kind         string          `json:"kind"`
	Ref          string          `json:"ref,omitempty"`
	Name         string          `json:"name,omitempty"`
	Covers       []LifecycleSlot `json:"covers,omitempty"`
	Code         string          `json:"code,omitempty"`
	Reason       string          `json:"reason,omitempty"`
	Incident     IncidentType    `json:"incident,omitempty"`
	RetryAllowed bool            `json:"retry_allowed,omitempty"`
	Review       bool            `json:"review,omitempty"`
}

type recordedEvent struct {
	Kind      string         `json:"kind"`
	Ref       string         `json:"ref"`
	Outcome   ReviewOutcome  `json:"outcome,omitempty"`
	Incident  IncidentType   `json:"incident,omitempty"`
	Reason    string         `json:"reason,omitempty"`
	Profile   ProfileID      `json:"profile,omitempty"`
	OfferRef  string         `json:"offer_ref,omitempty"`
	Inventory RouteInventory `json:"route_inventory"`
}

type State struct {
	SchemaVersion      string          `json:"schema_version"`
	FlowID             string          `json:"flow_id"`
	Profile            ProfileID       `json:"profile"`
	InitialProfile     ProfileID       `json:"initial_profile"`
	InitialInventory   RouteInventory  `json:"initial_route_inventory"`
	Topology           string          `json:"topology"`
	AddOn              string          `json:"add_on"`
	StepIndex          int             `json:"step_index"`
	InventoryDigest    string          `json:"inventory_digest"`
	SemanticsDigest    string          `json:"semantics_digest"`
	IntentKind         uint8           `json:"intent_kind"`
	IntentRef          string          `json:"intent_ref,omitempty"`
	ReturnIndex        int             `json:"return_index"`
	InIncident         bool            `json:"in_incident"`
	ActiveIncident     IncidentType    `json:"active_incident,omitempty"`
	IncidentHandler    string          `json:"incident_handler,omitempty"`
	IncidentReturnSlot LifecycleSlot   `json:"incident_return_slot,omitempty"`
	Remediating        bool            `json:"remediating,omitempty"`
	ReviewIndex        int             `json:"review_index"`
	NextIntent         uint64          `json:"next_intent"`
	CompletedSlots     []LifecycleSlot `json:"completed_slots"`
	CompletedGates     []string        `json:"completed_gates"`
	StableBoundaries   []string        `json:"stable_boundaries"`
	SwitchBoundary     string          `json:"switch_boundary,omitempty"`
	History            []recordedEvent `json:"history"`
	Next               NextState       `json:"next"`
}

func (saved State) Progress() (Progress, error) {
	if saved.SchemaVersion != StateSchemaV4 {
		return Progress{}, fail(FailureSemanticsInvalid, "saved Policy state is invalid")
	}
	next, err := restoreNext(saved.Next)
	if err != nil {
		return Progress{}, err
	}
	return Progress{
		Plan:             Plan{Profile: saved.Profile, Topology: saved.Topology, AddOn: saved.AddOn},
		CompletedSlots:   append([]LifecycleSlot(nil), saved.CompletedSlots...),
		CompletedGates:   append([]string(nil), saved.CompletedGates...),
		StableBoundaries: append([]string(nil), saved.StableBoundaries...),
		Next:             next,
	}, nil
}

func OfferReference(value string) (OfferRef, error) {
	if !strings.HasPrefix(value, "policy-offer-") || strings.IndexAny(value, "\x00\r\n") >= 0 {
		return OfferRef{}, fail(FailureOfferStale, "Policy Offer reference is invalid")
	}
	return OfferRef{value: value}, nil
}

func WorkReference(value string) (WorkRef, error) {
	if _, found := intentRunID(value); !found || strings.IndexAny(value, "\x00\r\n") >= 0 {
		return WorkRef{}, fail(FailureEventOutOfOrder, "Policy work reference is invalid")
	}
	return WorkRef{value: value}, nil
}

func GateReference(value string) (GateRef, error) {
	if _, found := intentRunID(value); !found || strings.IndexAny(value, "\x00\r\n") >= 0 {
		return GateRef{}, fail(FailureEventOutOfOrder, "Policy gate reference is invalid")
	}
	return GateRef{value: value}, nil
}

func (module *Module) Export(progress Progress) (State, error) {
	if module == nil || module.initErr != nil {
		if module == nil {
			return State{}, fail(FailureSemanticsInvalid, "Policy Workflow Module is nil")
		}
		return State{}, module.initErr
	}
	module.mu.Lock()
	defer module.mu.Unlock()

	ref := nextReference(progress.Next)
	flowID, found := intentRunID(ref)
	if !found {
		matched := ""
		for id, state := range module.runs {
			if state.terminal != nil && sameTerminal(state.terminal, progress.Next) {
				if matched != "" {
					return State{}, fail(FailureEventOutOfOrder, "terminal progress is ambiguous across Policy flows")
				}
				matched = id
			}
		}
		flowID, found = matched, matched != ""
	}
	if !found {
		return State{}, fail(FailureEventOutOfOrder, "progress is not owned by this Policy Workflow Module")
	}
	state, found := module.runs[flowID]
	if !found || state.intentRef != ref && state.terminal == nil {
		return State{}, fail(FailureEventOutOfOrder, "progress is not owned by this Policy Workflow Module")
	}
	return exportState(state, progress.Next), nil
}

func (module *Module) Restore(inventory RouteInventory, saved State) (Progress, error) {
	if module == nil || module.initErr != nil {
		if module == nil {
			return Progress{}, fail(FailureSemanticsInvalid, "Policy Workflow Module is nil")
		}
		return Progress{}, module.initErr
	}
	if saved.SchemaVersion != StateSchemaV4 || saved.FlowID == "" || saved.InventoryDigest == "" ||
		saved.SemanticsDigest != module.semantics || saved.Topology != TopologyCurrent || saved.AddOn != AddOnNone ||
		saved.InitialProfile == "" || strings.IndexAny(saved.FlowID, "\x00\r\n") >= 0 {
		return Progress{}, fail(FailureSemanticsInvalid, "saved Policy state is invalid")
	}
	normalized, err := normalizeInventory(inventory)
	if err != nil {
		return Progress{}, err
	}
	initialInventory, err := normalizeInventory(saved.InitialInventory)
	if err != nil {
		return Progress{}, fail(FailureSemanticsInvalid, "saved initial route inventory is invalid")
	}
	program, found := module.findProgram(saved.InitialProfile)
	if !found || !projectProfile(program, initialInventory).HostRoutable {
		return Progress{}, fail(FailureSemanticsInvalid, "saved initial Policy Profile is invalid")
	}
	state := &runState{
		id:             saved.FlowID,
		plan:           Plan{Profile: saved.InitialProfile, Topology: TopologyCurrent, AddOn: AddOnNone},
		initialProfile: saved.InitialProfile, initialInventory: canonicalInventory(initialInventory),
		program: program, inventoryDigest: profileInventoryDigest(program, initialInventory), semanticsDigest: saved.SemanticsDigest,
		returnIndex: -1, reviewIndex: -1,
	}
	progress := module.progress(state, initialInventory)
	for _, record := range saved.History {
		recordInventory, inventoryErr := normalizeInventory(record.Inventory)
		if inventoryErr != nil {
			return Progress{}, fail(FailureSemanticsInvalid, "saved Policy transition inventory is invalid")
		}
		event, err := restoreEvent(record)
		if err != nil || !eventMatchesCurrent(event, state) {
			return Progress{}, fail(FailureSemanticsInvalid, "saved Policy event history is invalid")
		}
		currentDigest := profileInventoryDigest(state.program, recordInventory)
		if state.inventoryDigest != currentDigest {
			switch event.(type) {
			case ProfileSwitchRequested, StopRequested, ExecutionUncertain:
			default:
				return Progress{}, fail(FailureSemanticsInvalid, "saved Policy transition has invalid route drift")
			}
		}
		progress, err = module.reduce(state, recordInventory, event)
		if err != nil {
			return Progress{}, fail(FailureSemanticsInvalid, "saved Policy event history is unreachable")
		}
		state.history = append(state.history, record)
	}
	derived := exportState(state, progress.Next)
	if !sameStateProjection(saved, derived) {
		return Progress{}, fail(FailureSemanticsInvalid, "saved Policy state does not match replayed lifecycle history")
	}
	if state.terminal == nil && !projectProfile(state.program, normalized).HostRoutable {
		return Progress{}, fail(FailureInventoryDrift, "active Policy Profile is no longer Host-routable")
	}
	if state.terminal == nil && profileInventoryDigest(state.program, normalized) != saved.InventoryDigest {
		return Progress{}, fail(FailureInventoryDrift, "Host routes required by the active Policy Profile changed since Policy state was saved")
	}

	module.mu.Lock()
	defer module.mu.Unlock()
	if _, exists := module.runs[state.id]; exists {
		return Progress{}, fail(FailureSemanticsInvalid, "saved Policy flow already exists in this Module")
	}
	module.runs[state.id] = state
	return progress, nil
}

func exportState(state *runState, next NextWork) State {
	return State{
		SchemaVersion: StateSchemaV4, FlowID: state.id, Profile: state.plan.Profile,
		InitialProfile: state.initialProfile, InitialInventory: append(RouteInventory(nil), state.initialInventory...),
		Topology: state.plan.Topology, AddOn: state.plan.AddOn,
		StepIndex: state.index, InventoryDigest: state.inventoryDigest, SemanticsDigest: state.semanticsDigest,
		IntentKind: uint8(state.intentKind), IntentRef: state.intentRef, ReturnIndex: state.returnIndex,
		InIncident: state.inIncident, ActiveIncident: state.activeIncident,
		IncidentHandler: state.incidentHandler, IncidentReturnSlot: state.incidentReturn,
		Remediating: state.remediating, ReviewIndex: state.reviewIndex, NextIntent: state.nextIntent,
		CompletedSlots:   append([]LifecycleSlot(nil), state.completedSlots...),
		CompletedGates:   append([]string(nil), state.completedGates...),
		StableBoundaries: append([]string(nil), state.stable...), SwitchBoundary: state.switchBoundary,
		History: cloneHistory(state.history), Next: exportNext(next),
	}
}

func recordEvent(event Event, inventory map[string]Route) recordedEvent {
	ref, _, _ := eventReference(event)
	routes := canonicalInventory(inventory)
	switch value := event.(type) {
	case SkillCompleted:
		return recordedEvent{Kind: "skill-completed", Ref: ref, Inventory: routes}
	case ReviewCompleted:
		return recordedEvent{Kind: "review-completed", Ref: ref, Outcome: value.Outcome, Inventory: routes}
	case HostActionCompleted:
		return recordedEvent{Kind: "host-action-completed", Ref: ref, Inventory: routes}
	case UserGateApproved:
		return recordedEvent{Kind: "user-gate-approved", Ref: ref, Inventory: routes}
	case HostGateSatisfied:
		return recordedEvent{Kind: "host-gate-satisfied", Ref: ref, Inventory: routes}
	case IncidentReported:
		return recordedEvent{Kind: "incident", Ref: ref, Incident: value.Incident, Reason: value.Reason, Inventory: routes}
	case ProfileSwitchRequested:
		return recordedEvent{Kind: "switch", Ref: ref, Profile: value.Profile, OfferRef: value.OfferRef.value, Inventory: routes}
	case StopRequested:
		return recordedEvent{Kind: "stop", Ref: ref, Reason: value.Reason, Inventory: routes}
	case ExecutionUncertain:
		return recordedEvent{Kind: "uncertain", Ref: ref, Reason: value.Reason, Inventory: routes}
	default:
		return recordedEvent{}
	}
}

func restoreEvent(saved recordedEvent) (Event, error) {
	work := WorkRef{value: saved.Ref}
	gate := GateRef{value: saved.Ref}
	switch saved.Kind {
	case "skill-completed":
		return SkillCompleted{WorkRef: work}, nil
	case "review-completed":
		return ReviewCompleted{WorkRef: work, Outcome: saved.Outcome}, nil
	case "host-action-completed":
		return HostActionCompleted{WorkRef: work}, nil
	case "user-gate-approved":
		return UserGateApproved{GateRef: gate}, nil
	case "host-gate-satisfied":
		return HostGateSatisfied{GateRef: gate}, nil
	case "incident":
		return IncidentReported{WorkRef: work, Incident: saved.Incident, Reason: saved.Reason}, nil
	case "switch":
		return ProfileSwitchRequested{Current: currentRefForKind(saved.Ref), OfferRef: OfferRef{value: saved.OfferRef}, Profile: saved.Profile}, nil
	case "stop":
		return StopRequested{Current: currentRefForKind(saved.Ref), Reason: saved.Reason}, nil
	case "uncertain":
		return ExecutionUncertain{WorkRef: work, Reason: saved.Reason}, nil
	default:
		return nil, fail(FailureSemanticsInvalid, "saved Policy event is invalid")
	}
}

func currentRefForKind(value string) CurrentRef { return WorkRef{value: value} }

func eventMatchesCurrent(event Event, state *runState) bool {
	ref, kind, valid := eventReference(event)
	if !valid || ref != state.intentRef || state.terminal != nil || !eventKindMatches(kind, state.intentKind) {
		return false
	}
	if untypedCompletion(event) && state.activeStep.reviewOutcome && !state.inIncident {
		return false
	}
	return true
}

func sameStateProjection(left, right State) bool {
	return left.SchemaVersion == right.SchemaVersion && left.FlowID == right.FlowID &&
		left.Profile == right.Profile && left.InitialProfile == right.InitialProfile &&
		equalInventory(left.InitialInventory, right.InitialInventory) &&
		left.Topology == right.Topology && left.AddOn == right.AddOn && left.StepIndex == right.StepIndex &&
		left.InventoryDigest == right.InventoryDigest && left.SemanticsDigest == right.SemanticsDigest &&
		left.IntentKind == right.IntentKind && left.IntentRef == right.IntentRef && left.ReturnIndex == right.ReturnIndex &&
		left.InIncident == right.InIncident && left.ActiveIncident == right.ActiveIncident &&
		left.IncidentHandler == right.IncidentHandler && left.IncidentReturnSlot == right.IncidentReturnSlot &&
		left.Remediating == right.Remediating && left.ReviewIndex == right.ReviewIndex && left.NextIntent == right.NextIntent &&
		equalSlots(left.CompletedSlots, right.CompletedSlots) && equalStrings(left.CompletedGates, right.CompletedGates) &&
		equalStrings(left.StableBoundaries, right.StableBoundaries) && left.SwitchBoundary == right.SwitchBoundary &&
		equalHistory(left.History, right.History) && sameNextState(left.Next, right.Next)
}

func exportNext(next NextWork) NextState {
	switch value := next.(type) {
	case InvokeSkill:
		return NextState{Kind: "invoke-skill", Ref: value.WorkRef.value, Name: value.Skill, Covers: append([]LifecycleSlot(nil), value.Covers...), Review: value.Review}
	case AwaitUserSkill:
		return NextState{Kind: "await-user-skill", Ref: value.WorkRef.value, Name: value.Skill, Covers: append([]LifecycleSlot(nil), value.Covers...), Review: value.Review}
	case HostAction:
		return NextState{Kind: "host-action", Ref: value.WorkRef.value, Name: value.Action, Covers: append([]LifecycleSlot(nil), value.Covers...), Review: value.Review}
	case UserGate:
		return NextState{Kind: "user-gate", Ref: value.GateRef.value, Name: value.Gate}
	case HostGate:
		return NextState{Kind: "host-gate", Ref: value.GateRef.value, Name: value.Gate}
	case Done:
		return NextState{Kind: "done"}
	case Stopped:
		return NextState{Kind: "stopped", Code: string(value.Code), Reason: value.Reason, Incident: value.Incident}
	case Blocked:
		return NextState{Kind: "blocked", Code: string(value.Code), Reason: value.Reason, RetryAllowed: value.RetryAllowed}
	default:
		return NextState{}
	}
}

func restoreNext(saved NextState) (NextWork, error) {
	switch saved.Kind {
	case "invoke-skill":
		return InvokeSkill{WorkRef: WorkRef{value: saved.Ref}, Skill: saved.Name, Covers: append([]LifecycleSlot(nil), saved.Covers...), Review: saved.Review}, nil
	case "await-user-skill":
		return AwaitUserSkill{WorkRef: WorkRef{value: saved.Ref}, Skill: saved.Name, Covers: append([]LifecycleSlot(nil), saved.Covers...), Review: saved.Review}, nil
	case "host-action":
		return HostAction{WorkRef: WorkRef{value: saved.Ref}, Action: saved.Name, Covers: append([]LifecycleSlot(nil), saved.Covers...), Review: saved.Review}, nil
	case "user-gate":
		return UserGate{GateRef: GateRef{value: saved.Ref}, Gate: saved.Name}, nil
	case "host-gate":
		return HostGate{GateRef: GateRef{value: saved.Ref}, Gate: saved.Name}, nil
	case "done":
		return Done{}, nil
	case "stopped":
		return Stopped{Code: StopCode(saved.Code), Reason: saved.Reason, Incident: saved.Incident}, nil
	case "blocked":
		return Blocked{Code: BlockCode(saved.Code), Reason: saved.Reason, RetryAllowed: saved.RetryAllowed}, nil
	default:
		return nil, fail(FailureSemanticsInvalid, "saved next Policy work is invalid")
	}
}

func nextReference(next NextWork) string {
	switch value := next.(type) {
	case InvokeSkill:
		return value.WorkRef.value
	case AwaitUserSkill:
		return value.WorkRef.value
	case HostAction:
		return value.WorkRef.value
	case UserGate:
		return value.GateRef.value
	case HostGate:
		return value.GateRef.value
	default:
		return ""
	}
}

func sameTerminal(left, right NextWork) bool {
	return sameNextState(exportNext(left), exportNext(right))
}

func sameNextState(left, right NextState) bool {
	return left.Kind == right.Kind && left.Ref == right.Ref && left.Name == right.Name &&
		equalSlots(left.Covers, right.Covers) && left.Code == right.Code && left.Reason == right.Reason &&
		left.Incident == right.Incident && left.RetryAllowed == right.RetryAllowed && left.Review == right.Review
}

func equalSlots(left, right []LifecycleSlot) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalHistory(left, right []recordedEvent) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Kind != right[index].Kind || left[index].Ref != right[index].Ref ||
			left[index].Outcome != right[index].Outcome || left[index].Incident != right[index].Incident ||
			left[index].Reason != right[index].Reason || left[index].Profile != right[index].Profile ||
			left[index].OfferRef != right[index].OfferRef || !equalInventory(left[index].Inventory, right[index].Inventory) {
			return false
		}
	}
	return true
}

func cloneHistory(values []recordedEvent) []recordedEvent {
	result := make([]recordedEvent, len(values))
	for index, value := range values {
		result[index] = value
		result[index].Inventory = append(RouteInventory(nil), value.Inventory...)
	}
	return result
}

func equalInventory(left, right RouteInventory) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
