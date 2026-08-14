package policyflow

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
)

type Module struct {
	mu        sync.Mutex
	programs  []profileProgram
	semantics string
	initErr   error
	runs      map[string]*runState
}

type runState struct {
	id               string
	plan             Plan
	initialProfile   ProfileID
	initialInventory RouteInventory
	program          profileProgram
	index            int
	inventoryDigest  string
	semanticsDigest  string
	intentKind       stepKind
	intentRef        string
	activeStep       programStep
	returnIndex      int
	inIncident       bool
	activeIncident   IncidentType
	incidentHandler  string
	incidentReturn   LifecycleSlot
	remediating      bool
	reviewIndex      int
	nextIntent       uint64
	history          []recordedEvent
	completedSlots   []LifecycleSlot
	completedGates   []string
	stable           []string
	switchBoundary   string
	terminal         NextWork
}

func New() *Module {
	programs, err := loadBuiltInPrograms()
	semantics := ""
	if err == nil {
		semantics = programDigest(programs)
	}
	return &Module{programs: programs, semantics: semantics, initErr: err, runs: map[string]*runState{}}
}

func (module *Module) Offer(inventory RouteInventory) (Offer, error) {
	if module == nil || module.initErr != nil {
		if module == nil {
			return Offer{}, fail(FailureSemanticsInvalid, "Policy Workflow Module is nil")
		}
		return Offer{}, module.initErr
	}
	normalized, err := normalizeInventory(inventory)
	if err != nil {
		return Offer{}, err
	}
	digest := inventoryDigest(normalized)
	profiles := make([]ProfileOffer, 0, len(module.programs))
	for _, program := range module.programs {
		profiles = append(profiles, projectProfile(program, normalized))
	}
	return Offer{Ref: OfferRef{value: module.offerReference(digest)}, Profiles: profiles}, nil
}

func (module *Module) Start(inventory RouteInventory, selection Selection) (Progress, error) {
	if module == nil || module.initErr != nil {
		if module == nil {
			return Progress{}, fail(FailureSemanticsInvalid, "Policy Workflow Module is nil")
		}
		return Progress{}, module.initErr
	}
	normalized, err := normalizeInventory(inventory)
	if err != nil {
		return Progress{}, err
	}
	digest := inventoryDigest(normalized)
	if selection.OfferRef.value == "" || selection.OfferRef.value != module.offerReference(digest) {
		return Progress{}, fail(FailureOfferStale, "selection does not match the current Policy semantics and Host route inventory")
	}
	program, found := module.findProgram(selection.Profile)
	if !found {
		return Progress{}, fail(FailureProfileUnknown, "selected Profile is not built in")
	}
	profile := projectProfile(program, normalized)
	if !profile.HostRoutable {
		return Progress{}, fail(FailureProfileIncomplete, "selected Profile has missing Host routes")
	}

	module.mu.Lock()
	defer module.mu.Unlock()
	flowID, err := module.allocateFlowID()
	if err != nil {
		return Progress{}, err
	}
	state := &runState{
		id:             flowID,
		plan:           Plan{Profile: selection.Profile, Topology: TopologyCurrent, AddOn: AddOnNone},
		initialProfile: selection.Profile, initialInventory: canonicalInventory(normalized),
		program: program, inventoryDigest: profileInventoryDigest(program, normalized),
		semanticsDigest: module.semantics, returnIndex: -1, reviewIndex: -1,
	}
	module.runs[state.id] = state
	return module.progress(state, normalized), nil
}

func (module *Module) allocateFlowID() (string, error) {
	for range 4 {
		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			return "", fail(FailureSemanticsInvalid, "could not allocate Policy flow reference")
		}
		id := "policy-flow-" + hex.EncodeToString(raw)
		if _, exists := module.runs[id]; !exists {
			return id, nil
		}
	}
	return "", fail(FailureSemanticsInvalid, "could not allocate unique Policy flow reference")
}

func (module *Module) Apply(inventory RouteInventory, event Event) (Progress, error) {
	if module == nil || module.initErr != nil {
		if module == nil {
			return Progress{}, fail(FailureSemanticsInvalid, "Policy Workflow Module is nil")
		}
		return Progress{}, module.initErr
	}
	normalized, err := normalizeInventory(inventory)
	if err != nil {
		return Progress{}, err
	}
	ref, kind, valid := eventReference(event)
	if !valid || ref == "" {
		return Progress{}, fail(FailureEventOutOfOrder, "event has no current policy work reference")
	}
	runID, found := intentRunID(ref)
	if !found {
		return Progress{}, fail(FailureEventOutOfOrder, "event reference is not a policy work reference")
	}

	module.mu.Lock()
	defer module.mu.Unlock()
	state, found := module.runs[runID]
	if !found || state.terminal != nil || state.intentRef != ref || !eventKindMatches(kind, state.intentKind) {
		return Progress{}, fail(FailureEventOutOfOrder, "event does not match the current policy work")
	}
	if untypedCompletion(event) && state.activeStep.reviewOutcome && !state.inIncident {
		return Progress{}, fail(FailureEventOutOfOrder, "active review work requires a typed review outcome")
	}
	currentDigest := profileInventoryDigest(state.program, normalized)
	if state.inventoryDigest != currentDigest {
		switch event.(type) {
		case ProfileSwitchRequested, StopRequested, ExecutionUncertain:
		default:
			return Progress{}, fail(FailureInventoryDrift, "Host route inventory changed during policy execution")
		}
	}

	progress, err := module.reduce(state, normalized, event)
	if err != nil {
		return Progress{}, err
	}
	state.history = append(state.history, recordEvent(event, normalized))
	return progress, nil
}

func (module *Module) reduce(state *runState, normalized map[string]Route, event Event) (Progress, error) {
	currentDigest := profileInventoryDigest(state.program, normalized)
	switch value := event.(type) {
	case IncidentReported:
		return module.applyIncident(state, normalized, value)
	case ReviewCompleted:
		return module.applyReview(state, normalized, value)
	case ProfileSwitchRequested:
		return module.applySwitch(state, normalized, value)
	case StopRequested:
		state.inventoryDigest = currentDigest
		state.intentRef = ""
		clearIncident(state)
		state.terminal = Stopped{Code: StopExplicit, Reason: value.Reason}
		return module.snapshot(state, state.terminal), nil
	case ExecutionUncertain:
		state.inventoryDigest = currentDigest
		state.intentRef = ""
		clearIncident(state)
		state.terminal = Blocked{Code: BlockExecutionUncertain, Reason: value.Reason, RetryAllowed: false}
		return module.snapshot(state, state.terminal), nil
	default:
		module.completeCurrent(state)
		return module.progress(state, normalized), nil
	}
}

func eventKindMatches(eventKind, currentKind stepKind) bool {
	switch eventKind {
	case stepAny:
		return true
	case stepExecutable:
		return currentKind == stepSkill || currentKind == stepHostAction
	default:
		return eventKind == currentKind
	}
}

func (module *Module) progress(state *runState, inventory map[string]Route) Progress {
	if state.terminal != nil {
		return module.snapshot(state, state.terminal)
	}
	if state.index >= len(state.program.steps) {
		state.terminal = Done{}
		return module.snapshot(state, state.terminal)
	}
	step := state.program.steps[state.index]
	return module.progressStep(state, inventory, step)
}

func (module *Module) progressStep(state *runState, inventory map[string]Route, step programStep) Progress {
	state.nextIntent++
	ref := state.id + "/intent-" + uintString(state.nextIntent)
	state.intentKind = step.kind
	state.intentRef = ref
	state.activeStep = step
	covers := append([]LifecycleSlot(nil), step.covers...)
	switch step.kind {
	case stepUserGate:
		return module.snapshot(state, UserGate{GateRef: GateRef{value: ref}, Gate: step.name})
	case stepHostGate:
		return module.snapshot(state, HostGate{GateRef: GateRef{value: ref}, Gate: step.name})
	case stepHostAction:
		return module.snapshot(state, HostAction{WorkRef: WorkRef{value: ref}, Action: step.name, Covers: covers, Review: step.reviewOutcome})
	default:
		route := inventory[step.name]
		if route.Mode == UserExplicit {
			return module.snapshot(state, AwaitUserSkill{
				WorkRef: WorkRef{value: ref}, Skill: step.name, Covers: covers, Review: step.reviewOutcome,
			})
		}
		return module.snapshot(state, InvokeSkill{
			WorkRef: WorkRef{value: ref}, Skill: step.name, Covers: covers, Review: step.reviewOutcome,
		})
	}
}

func (module *Module) completeCurrent(state *runState) {
	state.switchBoundary = ""
	for _, slot := range state.activeStep.completes {
		state.completedSlots = appendUniqueSlot(state.completedSlots, slot)
	}
	if state.activeStep.kind == stepUserGate || state.activeStep.kind == stepHostGate {
		state.completedGates = appendUniqueString(state.completedGates, state.activeStep.name)
		if containsStringValue(state.program.stableBoundaries, state.activeStep.name) {
			state.stable = appendUniqueString(state.stable, state.activeStep.name)
			state.switchBoundary = state.activeStep.name
		}
	}
	state.intentRef = ""
	if state.inIncident {
		if containsStringValue(state.program.stableBoundaries, "debugging-cycle-complete") {
			state.stable = appendUniqueString(state.stable, "debugging-cycle-complete")
			state.switchBoundary = "debugging-cycle-complete"
		}
		state.index = state.returnIndex
		clearIncident(state)
		return
	}
	if state.remediating {
		state.index = state.reviewIndex
		state.remediating = false
		state.reviewIndex = -1
		return
	}
	state.index++
	module.recordDerivedStableBoundary(state)
}

func (module *Module) recordDerivedStableBoundary(state *runState) {
	derived := []struct {
		slot     LifecycleSlot
		boundary string
	}{
		{SlotImplementationTDD, "tdd-cycle-complete"},
		{SlotReviewRemediation, "review-complete"},
		{SlotFreshVerification, "verification-complete"},
	}
	for _, candidate := range derived {
		if !containsSlot(state.activeStep.completes, candidate.slot) ||
			!containsStringValue(state.program.stableBoundaries, candidate.boundary) {
			continue
		}
		state.stable = appendUniqueString(state.stable, candidate.boundary)
		state.switchBoundary = candidate.boundary
	}
}

func (module *Module) applyIncident(state *runState, inventory map[string]Route, event IncidentReported) (Progress, error) {
	if state.intentKind != stepSkill && state.intentKind != stepHostAction {
		return Progress{}, fail(FailureEventOutOfOrder, "incident does not match active executable work")
	}
	incident, found := findIncidentRoute(state.program, event.Incident)
	returnIndex := -1
	if found {
		returnIndex, found = programIndexForSlot(state.program, incident.returnTo)
	}
	if found && state.index < returnIndex {
		return Progress{}, fail(FailureEventOutOfOrder, "incident is not valid before its declared return stage")
	}
	if !found {
		implementationIndex, hasImplementation := programIndexForSlot(state.program, SlotImplementation)
		if !hasImplementation || state.index < implementationIndex {
			return Progress{}, fail(FailureEventOutOfOrder, "incident is not valid before its declared return stage")
		}
		state.intentRef = ""
		clearIncident(state)
		state.terminal = Stopped{Code: StopIncidentUnhandled, Reason: event.Reason, Incident: event.Incident}
		return module.snapshot(state, state.terminal), nil
	}
	state.switchBoundary = ""
	if incident.route == "" || !routeAvailable(inventory, incident.route, []InvocationMode{HostVisible, UserExplicit}) {
		state.intentRef = ""
		clearIncident(state)
		state.terminal = Stopped{Code: StopIncidentHandlerUnavailable, Reason: event.Reason, Incident: event.Incident}
		return module.snapshot(state, state.terminal), nil
	}
	state.intentRef = ""
	state.inIncident = true
	state.returnIndex = returnIndex
	state.activeIncident = event.Incident
	state.incidentHandler = incident.route
	state.incidentReturn = incident.returnTo
	return module.progressStep(state, inventory, programStep{
		kind: stepSkill, name: incident.route, slot: SlotIncidentRecovery,
		covers: []LifecycleSlot{SlotIncidentRecovery}, completes: []LifecycleSlot{SlotIncidentRecovery},
	}), nil
}

func (module *Module) applyReview(state *runState, inventory map[string]Route, event ReviewCompleted) (Progress, error) {
	if state.inIncident || state.intentKind != stepSkill && state.intentKind != stepHostAction || !state.activeStep.reviewOutcome {
		return Progress{}, fail(FailureEventOutOfOrder, "review outcome does not match active review work")
	}
	switch event.Outcome {
	case ReviewClean:
		if state.remediating && state.reviewIndex == state.index {
			state.remediating = false
			state.reviewIndex = -1
		}
		module.completeCurrent(state)
		return module.progress(state, inventory), nil
	case ReviewFindings:
		implementationIndex, found := programIndexForSlot(state.program, SlotImplementation)
		if !found {
			return Progress{}, fail(FailureSemanticsInvalid, "review findings have no implementation remediation route")
		}
		state.switchBoundary = ""
		state.intentRef = ""
		state.remediating = true
		state.reviewIndex = state.index
		state.index = implementationIndex
		return module.progress(state, inventory), nil
	default:
		return Progress{}, fail(FailureEventOutOfOrder, "review outcome is invalid")
	}
}

func clearIncident(state *runState) {
	state.inIncident = false
	state.returnIndex = -1
	state.activeIncident = ""
	state.incidentHandler = ""
	state.incidentReturn = ""
}

func (module *Module) applySwitch(state *runState, inventory map[string]Route, event ProfileSwitchRequested) (Progress, error) {
	if state.switchBoundary == "" {
		return Progress{}, fail(FailureSwitchNotStable, "Profile can only switch after a declared stable boundary")
	}
	if event.OfferRef.value == "" || event.OfferRef.value != module.offerReference(inventoryDigest(inventory)) {
		return Progress{}, fail(FailureOfferStale, "switch selection does not match the current Policy semantics and Host route inventory")
	}
	program, found := module.findProgram(event.Profile)
	if !found {
		return Progress{}, fail(FailureProfileUnknown, "target Profile is not built in")
	}
	if !projectProfile(program, inventory).HostRoutable {
		return Progress{}, fail(FailureProfileIncomplete, "target Profile has missing Host routes")
	}
	index := nextUncreditedProgramIndex(program, state.completedSlots, state.completedGates)
	state.plan.Profile = event.Profile
	state.program = program
	state.index = index
	state.inventoryDigest = profileInventoryDigest(program, inventory)
	state.semanticsDigest = module.semantics
	state.intentRef = ""
	state.inIncident = false
	state.returnIndex = -1
	state.activeIncident = ""
	state.incidentHandler = ""
	state.incidentReturn = ""
	state.remediating = false
	state.reviewIndex = -1
	state.switchBoundary = ""
	return module.progress(state, inventory), nil
}

func (module *Module) snapshot(state *runState, next NextWork) Progress {
	return Progress{
		Plan:             state.plan,
		CompletedSlots:   append([]LifecycleSlot(nil), state.completedSlots...),
		CompletedGates:   append([]string(nil), state.completedGates...),
		StableBoundaries: append([]string(nil), state.stable...),
		Next:             next,
	}
}

func projectProfile(program profileProgram, inventory map[string]Route) ProfileOffer {
	result := ProfileOffer{
		ID: program.id, PolicySelectable: true, HostRoutable: true,
		Routes: []RouteStatus{}, Missing: []RouteStatus{}, IncidentRoutes: []IncidentRouteStatus{},
	}
	byName := map[string]int{}
	for _, step := range program.steps {
		covers := step.covers
		if len(covers) == 0 {
			covers = []LifecycleSlot{step.slot}
		}
		kind := RouteSkill
		wantMode := []InvocationMode{HostVisible, UserExplicit}
		switch step.kind {
		case stepHostAction:
			kind = RouteHostAction
			wantMode = []InvocationMode{HostControlled}
		case stepUserGate:
			kind = RouteUserGate
			wantMode = []InvocationMode{HostControlled}
		case stepHostGate:
			kind = RouteHostGate
			wantMode = []InvocationMode{HostControlled}
		}
		if index, seen := byName[step.name]; seen {
			for _, slot := range covers {
				result.Routes[index].Covers = appendUniqueSlot(result.Routes[index].Covers, slot)
			}
			continue
		}
		route, found := inventory[step.name]
		if step.kind == stepUserGate || step.kind == stepHostGate {
			route, found = Route{Name: step.name, Mode: HostControlled}, true
		}
		available := found && containsMode(wantMode, route.Mode)
		status := RouteStatus{
			Name: step.name, Kind: kind, Available: available,
			Covers: append([]LifecycleSlot(nil), covers...),
		}
		if found {
			status.Mode = route.Mode
		}
		byName[step.name] = len(result.Routes)
		result.Routes = append(result.Routes, status)
	}
	for _, required := range program.requires {
		if index, seen := byName[required.route]; seen {
			result.Routes[index].Covers = appendUniqueSlot(result.Routes[index].Covers, required.slot)
			continue
		}
		route, found := inventory[required.route]
		available := found && containsMode([]InvocationMode{HostVisible, UserExplicit}, route.Mode)
		status := RouteStatus{
			Name: required.route, Kind: RouteSkill, Available: available,
			Covers: []LifecycleSlot{required.slot}, Credited: true,
		}
		if found {
			status.Mode = route.Mode
		}
		byName[required.route] = len(result.Routes)
		result.Routes = append(result.Routes, status)
	}
	for _, status := range result.Routes {
		if status.Available {
			continue
		}
		result.HostRoutable = false
		result.Missing = append(result.Missing, status)
	}
	for _, incident := range program.incidents {
		status := IncidentRouteStatus{Incident: incident.incident, Skill: incident.route}
		if incident.route != "" {
			route, found := inventory[incident.route]
			status.Available = found && containsMode([]InvocationMode{HostVisible, UserExplicit}, route.Mode)
			if found {
				status.Mode = route.Mode
			}
		}
		result.IncidentRoutes = append(result.IncidentRoutes, status)
	}
	return result
}

func normalizeInventory(inventory RouteInventory) (map[string]Route, error) {
	result := make(map[string]Route, len(inventory))
	for _, route := range inventory {
		if route.Name == "" || strings.TrimSpace(route.Name) != route.Name || strings.IndexAny(route.Name, "\x00\r\n") >= 0 {
			return nil, fail(FailureInventoryInvalid, "route name is invalid")
		}
		if route.Mode != HostVisible && route.Mode != UserExplicit && route.Mode != HostControlled {
			return nil, fail(FailureInventoryInvalid, "route invocation mode is invalid")
		}
		if existing, duplicate := result[route.Name]; duplicate && existing.Mode != route.Mode {
			return nil, fail(FailureInventoryInvalid, "route has conflicting invocation modes")
		}
		result[route.Name] = route
	}
	return result, nil
}

func inventoryDigest(inventory map[string]Route) string {
	keys := make([]string, 0, len(inventory))
	for name := range inventory {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	hash := sha256.New()
	for _, name := range keys {
		hash.Write([]byte(name))
		hash.Write([]byte{0})
		hash.Write([]byte(inventory[name].Mode))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))[:24]
}

func profileInventoryDigest(program profileProgram, inventory map[string]Route) string {
	relevant := map[string]Route{}
	for _, step := range program.steps {
		if step.kind != stepSkill && step.kind != stepHostAction {
			continue
		}
		if route, found := inventory[step.name]; found {
			relevant[step.name] = route
		}
	}
	for _, requirement := range program.requires {
		if route, found := inventory[requirement.route]; found {
			relevant[requirement.route] = route
		}
	}
	for _, incident := range program.incidents {
		if route, found := inventory[incident.route]; found {
			relevant[incident.route] = route
		}
	}
	return inventoryDigest(relevant)
}

func canonicalInventory(inventory map[string]Route) RouteInventory {
	names := make([]string, 0, len(inventory))
	for name := range inventory {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make(RouteInventory, 0, len(names))
	for _, name := range names {
		result = append(result, inventory[name])
	}
	return result
}

func programDigest(programs []profileProgram) string {
	hash := sha256.New()
	for _, program := range programs {
		hash.Write([]byte(program.id))
		hash.Write([]byte{0})
		// Composite Skills (for example Matt's implement macro) depend on
		// credited internal routes. Keep those requirements in the semantic
		// fingerprint so persisted Offers cannot survive a contract change.
		for _, requirement := range program.requires {
			hash.Write([]byte(requirement.route))
			hash.Write([]byte{0})
			hash.Write([]byte(requirement.slot))
			hash.Write([]byte{0})
		}
		hash.Write([]byte{0xfe})
		for _, step := range program.steps {
			hash.Write([]byte{byte(step.kind)})
			if step.reviewOutcome {
				hash.Write([]byte{1})
			} else {
				hash.Write([]byte{0})
			}
			hash.Write([]byte(step.name))
			hash.Write([]byte{0})
			hash.Write([]byte(step.slot))
			hash.Write([]byte{0})
			writeSlotDigest(hash, step.covers)
			writeSlotDigest(hash, step.completes)
		}
		for _, incident := range program.incidents {
			hash.Write([]byte(incident.incident))
			hash.Write([]byte{0})
			hash.Write([]byte(incident.route))
			hash.Write([]byte{0})
			hash.Write([]byte(incident.returnTo))
			hash.Write([]byte{0})
		}
		for _, boundary := range program.stableBoundaries {
			hash.Write([]byte(boundary))
			hash.Write([]byte{0})
		}
	}
	return hex.EncodeToString(hash.Sum(nil))[:24]
}

func writeSlotDigest(hash interface{ Write([]byte) (int, error) }, slots []LifecycleSlot) {
	for _, slot := range slots {
		_, _ = hash.Write([]byte(slot))
		_, _ = hash.Write([]byte{0})
	}
	_, _ = hash.Write([]byte{0xff})
}

func (module *Module) offerReference(inventoryDigest string) string {
	return "policy-offer-" + module.semantics + "-" + inventoryDigest
}

func eventReference(event Event) (string, stepKind, bool) {
	switch value := event.(type) {
	case SkillCompleted:
		return value.WorkRef.value, stepSkill, true
	case ReviewCompleted:
		return value.WorkRef.value, stepExecutable, true
	case HostActionCompleted:
		return value.WorkRef.value, stepHostAction, true
	case UserGateApproved:
		return value.GateRef.value, stepUserGate, true
	case HostGateSatisfied:
		return value.GateRef.value, stepHostGate, true
	case IncidentReported:
		return value.WorkRef.value, stepExecutable, true
	case ProfileSwitchRequested:
		ref, valid := currentReferenceValue(value.Current)
		return ref, stepAny, valid
	case StopRequested:
		ref, valid := currentReferenceValue(value.Current)
		return ref, stepAny, valid
	case ExecutionUncertain:
		return value.WorkRef.value, stepExecutable, true
	default:
		return "", 0, false
	}
}

func currentReferenceValue(ref CurrentRef) (string, bool) {
	switch value := ref.(type) {
	case WorkRef:
		return value.value, true
	case GateRef:
		return value.value, true
	default:
		return "", false
	}
}

func untypedCompletion(event Event) bool {
	switch event.(type) {
	case SkillCompleted, HostActionCompleted:
		return true
	default:
		return false
	}
}

func intentRunID(ref string) (string, bool) {
	index := strings.LastIndex(ref, "/intent-")
	return ref[:max(index, 0)], index > 0
}

func containsMode(values []InvocationMode, wanted InvocationMode) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func routeAvailable(inventory map[string]Route, name string, modes []InvocationMode) bool {
	route, found := inventory[name]
	return found && containsMode(modes, route.Mode)
}

func findIncidentRoute(program profileProgram, incident IncidentType) (programIncidentRoute, bool) {
	for _, route := range program.incidents {
		if route.incident == incident {
			return route, true
		}
	}
	return programIncidentRoute{}, false
}

func programIndexForSlot(program profileProgram, slot LifecycleSlot) (int, bool) {
	for index, step := range program.steps {
		if step.slot == slot || containsSlot(step.covers, slot) {
			return index, true
		}
	}
	return 0, false
}

func nextUncreditedProgramIndex(program profileProgram, completedSlots []LifecycleSlot, completedGates []string) int {
	for index, step := range program.steps {
		switch step.kind {
		case stepUserGate, stepHostGate:
			if !containsStringValue(completedGates, step.name) {
				return index
			}
		default:
			slots := step.covers
			if len(slots) == 0 && step.slot != "" {
				slots = []LifecycleSlot{step.slot}
			}
			if len(slots) == 0 || !allSlotsCompleted(slots, completedSlots) {
				return index
			}
		}
	}
	return len(program.steps)
}

func allSlotsCompleted(slots []LifecycleSlot, completed []LifecycleSlot) bool {
	for _, slot := range slots {
		if !containsSlot(completed, slot) {
			return false
		}
	}
	return true
}

func containsSlot(slots []LifecycleSlot, wanted LifecycleSlot) bool {
	for _, slot := range slots {
		if slot == wanted {
			return true
		}
	}
	return false
}

func appendUniqueString(values []string, value string) []string {
	if containsStringValue(values, value) {
		return values
	}
	return append(values, value)
}

func appendUniqueSlot(values []LifecycleSlot, value LifecycleSlot) []LifecycleSlot {
	if containsSlot(values, value) {
		return values
	}
	return append(values, value)
}

func containsStringValue(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func fail(code FailureCode, detail string) error { return &Failure{Code: code, Detail: detail} }

func uintString(value uint64) string {
	if value == 0 {
		return "0"
	}
	buffer := [20]byte{}
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[index:])
}

func (module *Module) findProgram(id ProfileID) (profileProgram, bool) {
	for _, program := range module.programs {
		if program.id == id {
			return program, true
		}
	}
	return profileProgram{}, false
}
