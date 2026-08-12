package profile

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

type compilerContext struct {
	registry    EffectiveRegistry
	descriptors map[string]catalog.ProviderDescriptorRecord
	host        HostEvidenceRecord
	topology    execution.Topology
	decisions   *[]CompileDecision
}

func newCompilerContext(providers []catalog.ProviderDescriptorRecord, verified EffectiveRegistry, host HostEvidenceRecord, topology execution.Topology) compilerContext {
	descriptors := make(map[string]catalog.ProviderDescriptorRecord)
	for _, provider := range providers {
		descriptors[provider.ID] = provider
	}
	return compilerContext{registry: verified, descriptors: descriptors, host: host, topology: topology}
}

func (context compilerContext) resolveStep(slotID catalog.SlotID, step catalog.PipelineStep) ([]ResolvedBinding, []CompileDiagnostic, error) {
	unitID := string(slotID) + "/" + step.ID
	stack := make(map[string]bool)
	values, diagnostics, err := context.expandBinding(step.Selector, step.ID, unitID, "", step.StageSpan, "", stack)
	if err != nil || len(diagnostics) != 0 {
		return values, diagnostics, err
	}
	rootIndex := -1
	for index := range values {
		if values[index].UnitID == unitID {
			rootIndex = index
			break
		}
	}
	if rootIndex < 0 {
		return nil, nil, fmt.Errorf("PROFILE_COMPILER_INTERNAL: root unit was not expanded")
	}
	root := values[rootIndex]
	if root.InputArtifact != step.RequiredInputArtifact || root.OutputArtifact != step.ProducedOutputArtifact {
		return nil, []CompileDiagnostic{{
			Code: "PROFILE_ARTIFACT_MISMATCH", SlotID: slotID, StepID: step.ID, ProviderID: root.ProviderID, BindingID: root.BindingID,
			Detail: "pipeline artifacts do not match the selected Binding contract",
		}}, nil
	}
	if !slotSpanContains(root.SlotIDs, step.StageSpan) {
		return nil, []CompileDiagnostic{{
			Code: "STAGE_SPAN_INVALID", SlotID: slotID, StepID: step.ID, ProviderID: root.ProviderID, BindingID: root.BindingID,
			Detail: "pipeline span is outside the selected Binding contract",
		}}, nil
	}
	return values, nil, nil
}

func (context compilerContext) expandBinding(selector catalog.BindingSelector, stepID, unitID, parentUnitID string, span []catalog.SlotID, mode catalog.InternalCallMode, stack map[string]bool) ([]ResolvedBinding, []CompileDiagnostic, error) {
	key := selector.ProviderID + "\x00" + selector.BindingID
	if stack[key] {
		return nil, []CompileDiagnostic{{
			Code: "MACRO_INTERNAL_CONFLICT", StepID: stepID, ProviderID: selector.ProviderID, BindingID: selector.BindingID,
			Detail: "Binding macro contains a recursive call",
		}}, nil
	}
	stack[key] = true
	defer delete(stack, key)

	resolved, descriptorBinding, diagnostics, err := context.resolveExactBinding(selector, stepID, unitID, parentUnitID, span, mode)
	if err != nil || len(diagnostics) != 0 {
		return nil, diagnostics, err
	}
	before := []ResolvedBinding{}
	credited := []ResolvedBinding{}
	after := []ResolvedBinding{}
	for index, call := range descriptorBinding.InternalCalls {
		childID := fmt.Sprintf("%s/internal-%03d-%s", unitID, index+1, call.BindingID)
		children, childDiagnostics, childErr := context.expandBinding(
			catalog.BindingSelector{ProviderID: selector.ProviderID, BindingID: call.BindingID}, stepID, childID, unitID, call.StageSpan, call.Mode, stack,
		)
		if childErr != nil {
			return nil, nil, childErr
		}
		if len(childDiagnostics) != 0 {
			if call.Required {
				return nil, childDiagnostics, nil
			}
			if context.decisions != nil {
				detail := "optional internal Binding is unavailable"
				if childDiagnostics[0].Code != "" {
					detail = childDiagnostics[0].Code + ": " + childDiagnostics[0].Detail
				}
				*context.decisions = append(*context.decisions, CompileDecision{SlotID: call.StageSpan[0], StepID: stepID, UnitID: childID, Disposition: OmittedBySelection, ReasonCode: "OPTIONAL_INTERNAL_UNAVAILABLE", Detail: detail})
			}
			continue
		}
		switch call.Mode {
		case catalog.InternalDispatchBefore:
			before = append(before, children...)
		case catalog.InternalCreditOnly:
			for childIndex := range children {
				children[childIndex].Disposition = CreditInternalOnly
				children[childIndex].MacroMode = catalog.InternalCreditOnly
			}
			credited = append(credited, children...)
		case catalog.InternalDispatchAfter:
			after = append(after, children...)
		default:
			return nil, nil, fmt.Errorf("PROFILE_COMPILER_INTERNAL: invalid macro mode %q", call.Mode)
		}
	}
	result := make([]ResolvedBinding, 0, len(before)+1+len(credited)+len(after))
	result = append(result, before...)
	result = append(result, resolved)
	result = append(result, credited...)
	result = append(result, after...)
	return result, nil, nil
}

func (context compilerContext) resolveExactBinding(selector catalog.BindingSelector, stepID, unitID, parentUnitID string, span []catalog.SlotID, mode catalog.InternalCallMode) (ResolvedBinding, catalog.BindingRecord, []CompileDiagnostic, error) {
	descriptor, found := context.descriptors[selector.ProviderID]
	if !found {
		return ResolvedBinding{}, catalog.BindingRecord{}, unavailableBindingDiagnostic(selector, stepID, "Provider Descriptor is unavailable"), nil
	}
	declared, found := descriptorBinding(descriptor, selector.BindingID)
	if !found {
		return ResolvedBinding{}, catalog.BindingRecord{}, unavailableBindingDiagnostic(selector, stepID, "Binding is not declared by the Provider Descriptor"), nil
	}
	instance, found := context.registry.Provider(selector.ProviderID)
	if !found || instance.ProviderID != selector.ProviderID || instance.HostID != context.host.HostID || !recordDigestPattern.MatchString(instance.Digest) {
		return ResolvedBinding{}, catalog.BindingRecord{}, unavailableBindingDiagnostic(selector, stepID, "verified Provider Instance is unavailable"), nil
	}
	verified, found := context.registry.Binding(selector.ProviderID, selector.BindingID)
	if !found {
		return ResolvedBinding{}, catalog.BindingRecord{}, unavailableBindingDiagnostic(selector, stepID, "verified Binding is unavailable"), nil
	}
	if !bindingAdmittedByCapability(context.registry, descriptor, selector) {
		return ResolvedBinding{}, catalog.BindingRecord{}, unavailableBindingDiagnostic(selector, stepID, "Binding is not admitted by a verified WORKFLOW Capability"), nil
	}
	if !verifiedBindingMatches(instance, declared, verified) {
		return ResolvedBinding{}, catalog.BindingRecord{}, nil, fmt.Errorf("PROFILE_TRUSTED_BINDING_MISMATCH: %s/%s", selector.ProviderID, selector.BindingID)
	}
	if !slices.Contains(verified.SupportedTopologies, context.topology) {
		return ResolvedBinding{}, catalog.BindingRecord{}, []CompileDiagnostic{{
			Code: "PROFILE_TOPOLOGY_UNAVAILABLE", StepID: stepID, ProviderID: selector.ProviderID, BindingID: selector.BindingID, Topology: context.topology,
			Detail: "Binding does not support the selected topology",
		}}, nil
	}
	required, evidence, diagnostics := context.bindDelegation(selector, stepID, declared.Delegation)
	if len(diagnostics) != 0 {
		return ResolvedBinding{}, catalog.BindingRecord{}, diagnostics, nil
	}
	resolved := ResolvedBinding{
		UnitID: unitID, StepID: stepID, AnchorSlotID: bindingAnchorSlot(span, declared.Responsibilities), SlotIDs: append([]catalog.SlotID{}, span...),
		ProviderID: selector.ProviderID, ProviderInstanceDigest: instance.Digest, BindingID: declared.ID,
		DistributionID: verified.DistributionID, DistributionRevision: verified.DistributionRevision, DistributionTreeDigest: verified.DistributionTreeDigest,
		Surface: verified.Surface, Kind: verified.Kind, Reference: verified.Reference, Invocation: verified.Invocation, BindingTreeDigest: verified.BindingTreeDigest,
		InputArtifact: declared.InputArtifact, OutputArtifact: declared.OutputArtifact, Responsibilities: append([]catalog.ResponsibilityClaim{}, declared.Responsibilities...),
		MaximumEffects: append([]string{}, declared.MaximumEffects...), Resources: append([]string{}, declared.Resources...),
		SupportedTopologies: append([]execution.Topology{}, verified.SupportedTopologies...), Delegation: declared.Delegation,
		RequiredFeatures: required, FeatureEvidenceDigests: evidence, Disposition: DispatchByCoordinator, MacroMode: mode, ParentUnitID: parentUnitID,
		RequiresExplicitInvocation: declared.Invocation == catalog.InvocationHumanExplicit, BindingEvidenceDigest: verified.BindingEvidenceDigest,
	}
	return resolved, declared, nil, nil
}

func bindingAnchorSlot(span []catalog.SlotID, responsibilities []catalog.ResponsibilityClaim) catalog.SlotID {
	positions := make(map[catalog.SlotID]int, len(catalog.CanonicalSlots()))
	for index, slot := range catalog.CanonicalSlots() {
		positions[slot.ID] = index
	}
	anchor := span[0]
	anchorPosition := len(positions)
	for _, claim := range responsibilities {
		position, found := positions[claim.SlotID]
		if !claim.OutcomeOwner || !found || !slices.Contains(span, claim.SlotID) || position >= anchorPosition {
			continue
		}
		anchor = claim.SlotID
		anchorPosition = position
	}
	return anchor
}

func (context compilerContext) bindDelegation(selector catalog.BindingSelector, stepID string, requirements catalog.DelegationRequirements) ([]host.FeatureID, []string, []CompileDiagnostic) {
	features := make([]host.FeatureID, 0, 2)
	if context.topology == execution.TopologyCurrent {
		if requirements.Child {
			features = append(features, host.FeatureChildDelegation)
		}
		if requirements.ParallelChild {
			features = append(features, host.FeatureParallelChildDelegation)
		}
	} else {
		if requirements.NestedChild {
			features = append(features, host.FeatureNestedChildDelegation)
		}
		if requirements.NestedParallel {
			features = append(features, host.FeatureNestedParallelDelegation)
		}
	}
	evidence := make([]string, 0, len(features))
	for _, feature := range features {
		observation, found := hostFeature(context.host, feature)
		if !found || observation.State != host.AvailabilityAvailable || !liveSource(observation.Source) {
			return nil, nil, []CompileDiagnostic{{
				Code: "HOST_FEATURE_UNATTESTED", StepID: stepID, ProviderID: selector.ProviderID, BindingID: selector.BindingID, Topology: context.topology,
				Detail: delegationFeatureUnattestedDetail(feature),
			}}
		}
		evidence = append(evidence, observation.Digest)
	}
	return features, evidence, nil
}

func delegationFeatureUnattestedDetail(feature host.FeatureID) string {
	detail := fmt.Sprintf("required delegation feature %q is not live and available", feature)
	if feature == host.FeatureChildDelegation {
		return detail + "; starting a new session alone does not attest it; when an explicit Profile/topology request is blocked only by this feature, the Startup Gate may run one bounded native child capability probe as a governance probe, observe again, and retry inspection"
	}
	return detail
}

func unavailableBindingDiagnostic(selector catalog.BindingSelector, stepID, detail string) []CompileDiagnostic {
	return []CompileDiagnostic{{Code: "PROFILE_BINDING_UNAVAILABLE", StepID: stepID, ProviderID: selector.ProviderID, BindingID: selector.BindingID, Detail: detail}}
}

func descriptorBinding(provider catalog.ProviderDescriptorRecord, bindingID string) (catalog.BindingRecord, bool) {
	for _, binding := range provider.Bindings {
		if binding.ID == bindingID {
			return binding, true
		}
	}
	return catalog.BindingRecord{}, false
}

func bindingAdmittedByCapability(verified EffectiveRegistry, descriptor catalog.ProviderDescriptorRecord, selector catalog.BindingSelector) bool {
	for _, capability := range descriptor.Capabilities {
		if !slices.Contains(capability.BindingRefs, selector.BindingID) || !slices.Contains(capability.RequestModes, catalog.RequestModeWorkflow) {
			continue
		}
		observed, found := verified.Capability(selector.ProviderID, capability.ID)
		if found && observed.ID == capability.ID && slices.Contains(observed.BindingIDs, selector.BindingID) {
			return true
		}
	}
	return false
}

func verifiedBindingMatches(instance registry.ProviderInstance, declared catalog.BindingRecord, verified registry.VerifiedBinding) bool {
	return verified.BindingID == declared.ID && verified.DistributionID == declared.DistributionID &&
		verified.DistributionID == instance.DistributionID && verified.DistributionRevision == instance.DistributionRevision &&
		verified.DistributionTreeDigest == instance.DistributionTreeDigest && verified.Surface == declared.Surface && verified.Kind == declared.Kind &&
		verified.Reference == declared.Reference && verified.Invocation == declared.Invocation && verified.BindingTreeDigest == declared.TreeDigest &&
		verified.Delegation == declared.Delegation && verified.BindingEvidenceDigest != ""
}

func slotSpanContains(parent, child []catalog.SlotID) bool {
	if len(parent) == 0 || len(child) == 0 {
		return false
	}
	positions := make(map[catalog.SlotID]int)
	for index, slot := range catalog.CanonicalSlots() {
		positions[slot.ID] = index
	}
	parentStart, parentFound := positions[parent[0]]
	childStart, childFound := positions[child[0]]
	return parentFound && childFound && childStart >= parentStart && childStart+len(child) <= parentStart+len(parent)
}

func selectorIdentity(value catalog.BindingSelector) string {
	return value.ProviderID + "\x00" + value.BindingID
}

func sortedUniqueStrings(values []string) ([]string, bool) {
	result := append([]string{}, values...)
	sort.Strings(result)
	for index := range result {
		if strings.TrimSpace(result[index]) != result[index] || result[index] == "" || index > 0 && result[index-1] == result[index] {
			return nil, false
		}
	}
	return result, true
}
