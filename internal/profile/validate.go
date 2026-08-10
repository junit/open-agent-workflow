package profile

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

var (
	treeDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	revisionPattern   = regexp.MustCompile(`^(?:[0-9a-f]{40}|sha256:[0-9a-f]{64})$`)
)

func ValidateExecutionGraphRecord(record ExecutionGraphRecord) error {
	if record.SchemaVersion != ExecutionGraphSchemaV4 || record.HostID == "" || record.TaxonomyVersion != catalog.TaxonomyVersionV1 ||
		record.RecipeID == "" || record.RecipeVersion == "" || record.EntrySlotID == "" || record.ProviderInstances == nil || record.Slots == nil ||
		record.IncidentRoutes == nil || record.StableBoundaries == nil || record.EnvironmentRequirements == nil || record.Decisions == nil ||
		!recordDigestPattern.MatchString(record.HostEvidenceDigest) || !recordDigestPattern.MatchString(record.RegistryDigest) ||
		!recordDigestPattern.MatchString(record.RecipeDigest) || !recordDigestPattern.MatchString(record.Selection.Digest) ||
		!recordDigestPattern.MatchString(record.Digest) || record.ContentDigest() != record.Digest || selectionContentDigest(record.Selection) != record.Selection.Digest {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	if _, err := catalog.ParseLocalID(record.HostID); err != nil || record.Selection.RecipeID != record.RecipeID || record.Selection.RecipeDigest != record.RecipeDigest || record.Selection.Topology != record.Topology {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	if _, err := catalog.ParseQualifiedID(record.RecipeID); err != nil || func() error { _, err := catalog.ParseContentVersion(record.RecipeVersion); return err }() != nil || !validSelection(record.Selection) {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	if normalized, err := execution.NormalizeTopologies([]execution.Topology{record.Topology}); err != nil || len(normalized) != 1 {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	if normalized, err := execution.NormalizeRequirements(record.EnvironmentRequirements); err != nil || !reflect.DeepEqual(normalized, record.EnvironmentRequirements) {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	if !canonicalStrings(record.StableBoundaries) {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	providers := make(map[string]string, len(record.ProviderInstances))
	for index, provider := range record.ProviderInstances {
		if provider.HostID != record.HostID || !recordDigestPattern.MatchString(provider.InstanceDigest) || index > 0 && record.ProviderInstances[index-1].ProviderID >= provider.ProviderID {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		if _, err := catalog.ParseQualifiedID(provider.ProviderID); err != nil {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		providers[provider.ProviderID] = provider.InstanceDigest
	}
	canonical := catalog.CanonicalSlots()
	if len(record.Slots) != len(canonical) {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	active := make(map[catalog.SlotID]CompiledSlot)
	for index, slot := range record.Slots {
		if slot.SlotID != canonical[index].ID || slot.Pipeline == nil || slot.Gates == nil || slot.Transitions == nil || slot.Traversal == nil ||
			slot.Applicability != catalog.SlotMandatory && slot.Applicability != catalog.SlotConditional {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		if err := validateCompiledSlot(slot, providers, record.Topology); err != nil {
			return err
		}
		if slot.Active {
			active[slot.SlotID] = slot
		}
	}
	if err := validateGraphOwners(record.Slots); err != nil {
		return err
	}
	if err := validateGlobalCursorIdentity(record.Slots); err != nil {
		return err
	}
	usedProviders := make(map[string]struct{})
	for _, slot := range record.Slots {
		for _, unit := range slot.Pipeline {
			usedProviders[unit.ProviderID] = struct{}{}
		}
	}
	if len(usedProviders) != len(providers) {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	if _, found := active[record.EntrySlotID]; !found {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	if record.EntrySlotID != firstActiveSlot(record.Slots) {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	for _, route := range record.IncidentRoutes {
		if route.IncidentType == "" || route.ReturnTo == "" || route.HandlerSlotID != catalog.SlotIncidentRecovery ||
			route.IfUnavailable != catalog.IncidentStop && route.IfUnavailable != catalog.IncidentReplan {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		if _, found := active[route.ReturnTo]; !found {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		for _, cursor := range route.HandlerPipeline {
			if cursor.SlotID != string(catalog.SlotIncidentRecovery) || !cursorInSlots(record.Slots, cursor) {
				return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
			}
		}
	}
	for _, decision := range record.Decisions {
		if decision.Disposition != DispatchByCoordinator && decision.Disposition != CreditInternalOnly && decision.Disposition != OmittedBySelection ||
			decision.ReasonCode == "" || strings.TrimSpace(decision.ReasonCode) != decision.ReasonCode || decision.Detail == "" || strings.TrimSpace(decision.Detail) != decision.Detail {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
	}
	if diagnostics := validateCompiledControlGraph(record.Slots, record.IncidentRoutes); len(diagnostics) != 0 {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	return nil
}

func validateCompiledSlot(slot CompiledSlot, providers map[string]string, topology execution.Topology) error {
	seenUnits := make(map[string]struct{})
	for _, unit := range slot.Pipeline {
		if err := execution.ValidateGraphCursor(unit.Cursor); err != nil || unit.AnchorSlotID != slot.SlotID || unit.Cursor.SlotID != string(unit.AnchorSlotID) || unit.Cursor.Kind != execution.CursorBinding || unit.Cursor.UnitID != unit.UnitID ||
			unit.UnitID == "" || unit.StepID == "" || unit.AnchorSlotID == "" || unit.ProviderID == "" || unit.BindingID == "" ||
			providers[unit.ProviderID] != unit.ProviderInstanceDigest || !treeDigestPattern.MatchString(unit.DistributionTreeDigest) || !treeDigestPattern.MatchString(unit.BindingTreeDigest) ||
			!revisionPattern.MatchString(unit.DistributionRevision) || !recordDigestPattern.MatchString(unit.BindingEvidenceDigest) || unit.SlotIDs == nil || unit.Responsibilities == nil ||
			unit.MaximumEffects == nil || unit.Resources == nil || unit.SupportedTopologies == nil || unit.RequiredFeatures == nil || unit.FeatureEvidenceDigests == nil ||
			!slices.Contains(unit.SupportedTopologies, topology) || len(unit.RequiredFeatures) != len(unit.FeatureEvidenceDigests) ||
			unit.Disposition != DispatchByCoordinator && unit.Disposition != CreditInternalOnly && unit.Disposition != OmittedBySelection {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		if len(unit.SlotIDs) == 0 || !slices.Contains(unit.SlotIDs, unit.AnchorSlotID) || !contiguousSlotIDs(unit.SlotIDs) {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		if _, err := catalog.ParseQualifiedID(unit.ProviderID); err != nil {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		if _, err := catalog.ParseLocalID(unit.BindingID); err != nil {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		if unit.Kind != catalog.BindingSkill && unit.Kind != catalog.BindingAgent && unit.Kind != catalog.BindingRole && unit.Kind != catalog.BindingInstruction && unit.Kind != catalog.BindingTool ||
			unit.Invocation != catalog.InvocationHumanExplicit && unit.Invocation != catalog.InvocationModel && unit.Invocation != catalog.InvocationHost && unit.Invocation != catalog.InvocationInternal ||
			unit.MacroMode != "" && unit.MacroMode != catalog.InternalCreditOnly && unit.MacroMode != catalog.InternalDispatchBefore && unit.MacroMode != catalog.InternalDispatchAfter ||
			unit.RequiresExplicitInvocation != (unit.Invocation == catalog.InvocationHumanExplicit) || !canonicalStrings(unit.MaximumEffects) || !canonicalStrings(unit.Resources) {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		normalizedTopologies, err := execution.NormalizeTopologies(unit.SupportedTopologies)
		if err != nil || !reflect.DeepEqual(normalizedTopologies, unit.SupportedTopologies) {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		if _, duplicate := seenUnits[unit.UnitID]; duplicate {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		seenUnits[unit.UnitID] = struct{}{}
		for _, digest := range unit.FeatureEvidenceDigests {
			if !recordDigestPattern.MatchString(digest) {
				return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
			}
		}
	}
	if slot.HostAction != nil {
		if err := execution.ValidateGraphCursor(slot.HostAction.Cursor); err != nil || slot.HostAction.Cursor.Kind != execution.CursorHostAction ||
			slot.HostAction.Cursor.SlotID != string(slot.SlotID) || slot.HostAction.Cursor.UnitID != slot.HostAction.ID || slot.HostAction.ID == "" || !recordDigestPattern.MatchString(slot.HostAction.ObservationDigest) ||
			slot.HostAction.MaximumEffects == nil || slot.HostAction.Resources == nil {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
	}
	for _, gate := range slot.Gates {
		if err := execution.ValidateGraphCursor(gate.Cursor); err != nil || gate.Cursor.Kind != execution.CursorGate || gate.Cursor.SlotID != string(slot.SlotID) || gate.Cursor.UnitID != gate.ID || gate.ID == "" || gate.EvidenceRequirements == nil {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
	}
	expectedTraversal := make([]execution.GraphCursor, 0, len(slot.Pipeline)+len(slot.Gates)+2)
	for _, unit := range slot.Pipeline {
		expectedTraversal = append(expectedTraversal, unit.Cursor)
	}
	if slot.HostAction != nil {
		expectedTraversal = append(expectedTraversal, slot.HostAction.Cursor)
	}
	for _, gate := range slot.Gates {
		expectedTraversal = append(expectedTraversal, gate.Cursor)
	}
	if slot.Terminal {
		terminal, err := execution.NewGraphCursor(string(slot.SlotID), execution.CursorTerminal, "terminal:"+string(slot.SlotID), uint64(len(expectedTraversal)+1))
		if err != nil {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		expectedTraversal = append(expectedTraversal, terminal)
	}
	for _, cursor := range slot.Traversal {
		if err := execution.ValidateGraphCursor(cursor); err != nil || cursor.SlotID != string(slot.SlotID) {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
	}
	if !slices.Equal(expectedTraversal, slot.Traversal) {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	for index, cursor := range slot.Traversal {
		if cursor.Ordinal != uint64(index+1) {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
	}
	if slot.Active && len(slot.Traversal) == 0 && (len(slot.Pipeline) != 0 || slot.HostAction != nil || len(slot.Gates) != 0 || slot.Terminal) ||
		!slot.Active && (len(slot.Pipeline) != 0 || slot.HostAction != nil || len(slot.Traversal) != 0) ||
		slot.Terminal && (!slot.Active || len(slot.Transitions) != 0) ||
		slot.SlotID != catalog.SlotIncidentRecovery && slot.Active && (len(slot.Transitions) == 0) != slot.Terminal {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	seenSignals := make(map[string]struct{}, len(slot.Transitions))
	for _, transition := range slot.Transitions {
		if transition.Signal == "" || transition.Target == "" {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		if _, duplicate := seenSignals[transition.Signal]; duplicate {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
		seenSignals[transition.Signal] = struct{}{}
	}
	return nil
}

func validateGraphOwners(slots []CompiledSlot) error {
	units := make(map[string][]ResolvedBinding)
	for _, slot := range slots {
		for _, unit := range slot.Pipeline {
			units[unit.UnitID] = append(units[unit.UnitID], unit)
		}
	}
	for _, slot := range slots {
		if err := validateOwner(slot, units); err != nil {
			return err
		}
	}
	return nil
}

func validateOwner(slot CompiledSlot, units map[string][]ResolvedBinding) error {
	switch slot.OutcomeOwner.Kind {
	case catalog.OwnerProviderBinding:
		matches := 0
		for _, unit := range units[slot.OutcomeOwner.UnitID] {
			claimsOutcome := slices.ContainsFunc(unit.Responsibilities, func(claim catalog.ResponsibilityClaim) bool {
				return claim.SlotID == slot.SlotID && claim.OutcomeOwner
			})
			if unit.ProviderID == slot.OutcomeOwner.ProviderID && unit.BindingID == slot.OutcomeOwner.BindingID && unit.Disposition != OmittedBySelection && slices.Contains(unit.SlotIDs, slot.SlotID) && claimsOutcome {
				matches++
			}
		}
		if matches != 1 || slot.OutcomeOwner.HostActionID != "" {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
	case catalog.OwnerHostAction:
		if slot.HostAction == nil || slot.OutcomeOwner.HostActionID != slot.HostAction.ID || slot.OutcomeOwner.UnitID != slot.HostAction.ID || slot.OutcomeOwner.ProviderID != "" || slot.OutcomeOwner.BindingID != "" {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
	case catalog.OwnerNone:
		if slot.SlotID != catalog.SlotIncidentRecovery || slot.OutcomeOwner.UnitID != "" || slot.OutcomeOwner.ProviderID != "" || slot.OutcomeOwner.BindingID != "" || slot.OutcomeOwner.HostActionID != "" {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
		}
	default:
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	return nil
}

func validSelection(value Selection) bool {
	if value.Profile == "" || strings.TrimSpace(value.Profile) != value.Profile || value.AddOns == nil || value.Alternatives == nil || value.Overlays == nil || !canonicalStrings(value.AddOns) {
		return false
	}
	seenOverlays := make(map[string]struct{}, len(value.Overlays))
	for _, overlay := range value.Overlays {
		if _, err := catalog.ParseLocalID(overlay); err != nil {
			return false
		}
		if _, duplicate := seenOverlays[overlay]; duplicate {
			return false
		}
		seenOverlays[overlay] = struct{}{}
	}
	seenAlternatives := make(map[string]struct{}, len(value.Alternatives))
	for _, alternative := range value.Alternatives {
		key := string(alternative.SlotID) + "\x00" + alternative.StepID
		if alternative.SlotID == "" || alternative.StepID == "" || alternative.AlternativeID == "" || alternative.AlternativeID != alternative.Selector.BindingID || alternative.Selector.ProviderID == "" {
			return false
		}
		if _, duplicate := seenAlternatives[key]; duplicate {
			return false
		}
		seenAlternatives[key] = struct{}{}
	}
	return true
}

func canonicalStrings(values []string) bool {
	if values == nil {
		return false
	}
	copy := append([]string{}, values...)
	sort.Strings(copy)
	if !slices.Equal(copy, values) {
		return false
	}
	for index, value := range values {
		if value == "" || strings.TrimSpace(value) != value || index > 0 && values[index-1] == value {
			return false
		}
	}
	return true
}

func validateGlobalCursorIdentity(slots []CompiledSlot) error {
	seenUnits := make(map[string]struct{})
	seenCursors := make(map[execution.GraphCursor]struct{})
	for _, slot := range slots {
		for _, unit := range slot.Pipeline {
			if _, duplicate := seenUnits[unit.UnitID]; duplicate {
				return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
			}
			seenUnits[unit.UnitID] = struct{}{}
		}
		for _, cursor := range slot.Traversal {
			if _, duplicate := seenCursors[cursor]; duplicate {
				return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
			}
			seenCursors[cursor] = struct{}{}
		}
	}
	return nil
}

func contiguousSlotIDs(values []catalog.SlotID) bool {
	positions := make(map[catalog.SlotID]int, len(catalog.CanonicalSlots()))
	for index, slot := range catalog.CanonicalSlots() {
		positions[slot.ID] = index
	}
	for index, value := range values {
		position, found := positions[value]
		if !found || index > 0 && position != positions[values[index-1]]+1 {
			return false
		}
	}
	return true
}

func validateCompiledControlGraph(slots []CompiledSlot, routes []CompiledIncidentRoute) []CompileDiagnostic {
	active := make(map[catalog.SlotID]CompiledSlot)
	for _, slot := range slots {
		if slot.Active {
			active[slot.SlotID] = slot
		}
	}
	entry := firstActiveSlot(slots)
	if entry == "" {
		return []CompileDiagnostic{{Code: "PROFILE_GRAPH_UNREACHABLE", Detail: "graph has no active entry slot"}}
	}
	visited := make(map[catalog.SlotID]bool)
	queue := []catalog.SlotID{entry}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] {
			continue
		}
		visited[current] = true
		for _, transition := range active[current].Transitions {
			target, found := active[transition.Target]
			if !found {
				return []CompileDiagnostic{{Code: "PROFILE_GRAPH_UNREACHABLE", SlotID: current, Detail: "transition targets an inactive or missing slot"}}
			}
			if active[current].OutcomeArtifact != "" && target.EntryArtifact != "" && active[current].OutcomeArtifact != target.EntryArtifact {
				return []CompileDiagnostic{{Code: "PIPELINE_ARTIFACT_INCOMPATIBLE", SlotID: current, Detail: "cross-slot artifact edge is incompatible"}}
			}
			queue = append(queue, transition.Target)
		}
	}
	for id := range active {
		if id == catalog.SlotIncidentRecovery {
			continue
		}
		if !visited[id] {
			return []CompileDiagnostic{{Code: "PROFILE_GRAPH_UNREACHABLE", SlotID: id, Detail: "active slot is unreachable"}}
		}
	}
	terminals := 0
	terminalSlots := make(map[catalog.SlotID]struct{})
	for _, slot := range active {
		if slot.Terminal {
			terminals++
			terminalSlots[slot.SlotID] = struct{}{}
			if slot.SlotID == catalog.SlotCloseout && !slices.ContainsFunc(slot.Gates, func(gate CompiledGate) bool { return gate.Authority == catalog.GateUser }) {
				return []CompileDiagnostic{{Code: "PROFILE_TERMINAL_INVALID", SlotID: slot.SlotID, Detail: "closeout terminal requires a user gate"}}
			}
		}
	}
	if terminals == 0 {
		return []CompileDiagnostic{{Code: "PROFILE_TERMINAL_INVALID", Detail: "graph has no terminal slot"}}
	}
	canReachTerminal := make(map[catalog.SlotID]bool, len(active))
	for terminal := range terminalSlots {
		canReachTerminal[terminal] = true
	}
	changed := true
	for changed {
		changed = false
		for id, slot := range active {
			if canReachTerminal[id] || id == catalog.SlotIncidentRecovery {
				continue
			}
			for _, transition := range slot.Transitions {
				if canReachTerminal[transition.Target] {
					canReachTerminal[id] = true
					changed = true
					break
				}
			}
		}
	}
	for id := range active {
		if id != catalog.SlotIncidentRecovery && !canReachTerminal[id] {
			return []CompileDiagnostic{{Code: "PROFILE_TERMINAL_INVALID", SlotID: id, Detail: "active slot cannot reach a terminal"}}
		}
	}
	seenIncidents := make(map[string]struct{})
	for _, route := range routes {
		if _, duplicate := seenIncidents[route.IncidentType]; duplicate {
			return []CompileDiagnostic{{Code: "PROFILE_INCIDENT_INVALID", IncidentType: route.IncidentType, Detail: "incident route is duplicated"}}
		}
		seenIncidents[route.IncidentType] = struct{}{}
	}
	return nil
}

func cursorInSlots(slots []CompiledSlot, cursor execution.GraphCursor) bool {
	for _, slot := range slots {
		if slices.Contains(slot.Traversal, cursor) {
			return true
		}
	}
	return false
}
