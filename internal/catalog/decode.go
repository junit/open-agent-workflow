package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

var (
	contentDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	revisionPattern      = regexp.MustCompile(`^(?:[0-9a-f]{40}|sha256:[0-9a-f]{64})$`)
)

func strictDecode(data []byte, destination any) error {
	if err := rejectDuplicateJSONFields(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func rejectDuplicateJSONFields(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := consumeUniqueJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON token")
		}
		return err
	}
	return nil
}

func consumeUniqueJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate object field %q", key)
			}
			seen[key] = struct{}{}
			if err := consumeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("object is not closed")
		}
	case '[':
		for decoder.More() {
			if err := consumeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("array is not closed")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
	return nil
}

func DecodeProvider(data []byte) (ProviderDescriptorRecord, error) {
	var record ProviderDescriptorRecord
	if err := strictDecode(data, &record); err != nil {
		return ProviderDescriptorRecord{}, fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
	}
	if record.SchemaVersion != ProviderDescriptorSchemaV4 {
		return ProviderDescriptorRecord{}, fmt.Errorf("UNSUPPORTED_PROVIDER_SCHEMA: %q", record.SchemaVersion)
	}
	if err := validateProviderRecord(&record); err != nil {
		return ProviderDescriptorRecord{}, err
	}
	normalizeProvider(&record)
	return cloneProvider(record), nil
}

func DecodeRecipe(data []byte) (ProfileRecipeRecord, error) {
	var record ProfileRecipeRecord
	if err := strictDecode(data, &record); err != nil {
		return ProfileRecipeRecord{}, fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
	}
	if record.SchemaVersion != ProfileRecipeSchemaV3 {
		return ProfileRecipeRecord{}, fmt.Errorf("UNSUPPORTED_RECIPE_SCHEMA: %q", record.SchemaVersion)
	}
	if err := validateRecipeRecord(&record); err != nil {
		return ProfileRecipeRecord{}, err
	}
	if err := normalizeRecipe(&record); err != nil {
		return ProfileRecipeRecord{}, err
	}
	return cloneRecipe(record), nil
}

func DecodeAliasSet(data []byte) (ProfileAliasSetRecord, error) {
	var record ProfileAliasSetRecord
	if err := strictDecode(data, &record); err != nil {
		return ProfileAliasSetRecord{}, fmt.Errorf("INVALID_PROFILE_ALIAS_SET: %w", err)
	}
	if record.SchemaVersion != ProfileAliasSetSchemaV1 {
		return ProfileAliasSetRecord{}, fmt.Errorf("UNSUPPORTED_ALIAS_SET_SCHEMA: %q", record.SchemaVersion)
	}
	if record.Aliases == nil || len(record.Aliases) == 0 {
		return ProfileAliasSetRecord{}, errors.New("INVALID_PROFILE_ALIAS_SET: aliases must not be empty")
	}
	if err := validateAliases(record.Aliases, nil); err != nil {
		return ProfileAliasSetRecord{}, err
	}
	normalizeAliases(record.Aliases)
	return cloneAliases(record), nil
}

func validateProviderRecord(record *ProviderDescriptorRecord) error {
	if record.SchemaVersion != ProviderDescriptorSchemaV4 {
		return fmt.Errorf("UNSUPPORTED_PROVIDER_SCHEMA: %q", record.SchemaVersion)
	}
	if record.DescriptorVersion == "" || record.ID == "" || record.DisplayName == "" || record.Distributions == nil || record.Discovery == nil || record.Bindings == nil || record.Capabilities == nil {
		return errors.New("INVALID_PROVIDER_DESCRIPTOR: required field missing")
	}
	if _, err := ParseContentVersion(record.DescriptorVersion); err != nil {
		return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
	}
	if _, err := ParseQualifiedID(record.ID); err != nil {
		return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
	}
	if strings.TrimSpace(record.DisplayName) != record.DisplayName {
		return errors.New("INVALID_PROVIDER_DESCRIPTOR: invalid display name")
	}
	if len(record.Distributions) == 0 || len(record.Discovery) == 0 || len(record.Bindings) == 0 || len(record.Capabilities) == 0 {
		return errors.New("INVALID_PROVIDER_DESCRIPTOR: required collection is empty")
	}
	if err := validateProviderMembers(record); err != nil {
		return err
	}
	return validateProviderReferences(record)
}

func validateProviderMembers(record *ProviderDescriptorRecord) error {
	distributionIDs := make(map[string]struct{}, len(record.Distributions))
	for _, distribution := range record.Distributions {
		if _, err := ParseLocalID(distribution.ID); err != nil || distribution.SourceURI == "" || strings.TrimSpace(distribution.SourceURI) != distribution.SourceURI {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: invalid Distribution")
		}
		if _, exists := distributionIDs[distribution.ID]; exists {
			return errors.New("DUPLICATE_DISTRIBUTION_ID: duplicate Distribution id")
		}
		distributionIDs[distribution.ID] = struct{}{}
		if !revisionPattern.MatchString(distribution.Revision) {
			return errors.New("PROVIDER_DISTRIBUTION_REVISION_INVALID: immutable revision is required")
		}
		if !contentDigestPattern.MatchString(distribution.TreeDigest) {
			return errors.New("PROVIDER_DISTRIBUTION_DIGEST_INVALID: invalid tree digest")
		}
	}

	probeIDs := make(map[string]struct{}, len(record.Discovery))
	for i := range record.Discovery {
		probe := &record.Discovery[i]
		if _, err := ParseLocalID(probe.ID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if _, exists := probeIDs[probe.ID]; exists {
			return errors.New("DUPLICATE_DISCOVERY_PROBE_ID: duplicate probe id")
		}
		probeIDs[probe.ID] = struct{}{}
		if probe.Hosts == nil || len(probe.Hosts) == 0 {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: discovery hosts are required")
		}
		if err := uniqueStrings(probe.Hosts, "DUPLICATE_DISCOVERY_HOST"); err != nil {
			return err
		}
		for _, host := range probe.Hosts {
			if _, err := ParseLocalID(host); err != nil {
				return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
			}
		}
		if _, err := ParseLocalID(probe.Surface); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if _, err := ParseLocalID(probe.DistributionID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if err := validateProbe(probe); err != nil {
			return err
		}
	}

	bindingIDs := make(map[string]struct{}, len(record.Bindings))
	for i := range record.Bindings {
		binding := &record.Bindings[i]
		if _, err := ParseLocalID(binding.ID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if _, exists := bindingIDs[binding.ID]; exists {
			return errors.New("DUPLICATE_BINDING_ID: duplicate Binding id")
		}
		bindingIDs[binding.ID] = struct{}{}
		if _, err := ParseLocalID(binding.DistributionID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if !safeRelativePath(binding.ContentRoot) || !safeRelativePath(binding.InstallRoot) {
			return errors.New("PROVIDER_BINDING_PATH_INVALID: clean relative ContentRoot and InstallRoot are required")
		}
		if !contentDigestPattern.MatchString(binding.TreeDigest) {
			return errors.New("PROVIDER_BINDING_DIGEST_INVALID: invalid tree digest")
		}
		if _, err := ParseLocalID(binding.Host); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if _, err := ParseLocalID(binding.Surface); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if !validBindingKind(binding.Kind) || binding.Reference == "" || strings.TrimSpace(binding.Reference) != binding.Reference || !validInvocation(binding.Invocation) {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: invalid Binding surface")
		}
		if binding.Responsibilities == nil || binding.MaximumEffects == nil || binding.Resources == nil || binding.SupportedTopologies == nil || binding.StageSpan == nil || binding.InternalCalls == nil || binding.Alternatives == nil || binding.Conflicts == nil || binding.InputArtifact == "" || binding.OutputArtifact == "" {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: incomplete Binding contract")
		}
		if len(binding.Responsibilities) == 0 || len(binding.MaximumEffects) == 0 || len(binding.Resources) == 0 {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: empty Binding contract")
		}
		if err := validateResponsibilities(binding.Responsibilities); err != nil {
			return err
		}
		if err := validateEffects(binding.MaximumEffects); err != nil {
			return err
		}
		if err := validateResources(binding.Resources); err != nil {
			return err
		}
		topologies, err := execution.NormalizeTopologies(binding.SupportedTopologies)
		if err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		binding.SupportedTopologies = topologies
		if !validStageSpan(binding.StageSpan) {
			return errors.New("STAGE_SPAN_INVALID: Binding span is not a contiguous canonical span")
		}
		if err := uniqueStrings(binding.Alternatives, "CAPABILITY_BINDING_AMBIGUOUS"); err != nil {
			return err
		}
		if err := uniqueStrings(binding.Conflicts, "MACRO_INTERNAL_CONFLICT"); err != nil {
			return err
		}
		for _, target := range append(cloneSlice(binding.Alternatives), binding.Conflicts...) {
			if _, err := ParseLocalID(target); err != nil {
				return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
			}
		}
		for _, call := range binding.InternalCalls {
			if _, err := ParseLocalID(call.BindingID); err != nil {
				return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
			}
			if !validInternalCallMode(call.Mode) {
				return fmt.Errorf("INTERNAL_CALL_MODE_INVALID: %q", call.Mode)
			}
			if !validStageSpan(call.StageSpan) || !spanContains(binding.StageSpan, call.StageSpan) {
				return errors.New("STAGE_SPAN_INVALID: internal-call span is outside its parent")
			}
		}
	}

	capabilityIDs := make(map[string]struct{}, len(record.Capabilities))
	for _, capability := range record.Capabilities {
		if _, err := ParseLocalID(capability.ID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if _, exists := capabilityIDs[capability.ID]; exists {
			return errors.New("DUPLICATE_CAPABILITY_ID: duplicate Capability id")
		}
		capabilityIDs[capability.ID] = struct{}{}
		if capability.InputSchema == "" || capability.OutcomeSchema == "" || capability.RequestModes == nil || len(capability.RequestModes) == 0 || capability.BindingRefs == nil || len(capability.BindingRefs) == 0 {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: incomplete Capability contract")
		}
		seenModes := make(map[RequestMode]struct{}, len(capability.RequestModes))
		for _, mode := range capability.RequestModes {
			if mode != RequestModeBounded && mode != RequestModeWorkflow {
				return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: invalid request mode %q", mode)
			}
			if _, exists := seenModes[mode]; exists {
				return errors.New("DUPLICATE_REQUEST_MODE: duplicate request mode")
			}
			seenModes[mode] = struct{}{}
		}
		if err := uniqueStrings(capability.BindingRefs, "CAPABILITY_BINDING_AMBIGUOUS"); err != nil {
			return err
		}
		for _, bindingID := range capability.BindingRefs {
			if _, err := ParseLocalID(bindingID); err != nil {
				return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
			}
		}
	}
	return nil
}

func validateRecipeRecord(record *ProfileRecipeRecord) error {
	if record.SchemaVersion != ProfileRecipeSchemaV3 {
		return fmt.Errorf("UNSUPPORTED_RECIPE_SCHEMA: %q", record.SchemaVersion)
	}
	if record.TaxonomyVersion == "" || record.RecipeVersion == "" || record.ID == "" || record.DisplayName == "" || record.Family == "" || record.Slots == nil || record.AddOns == nil || record.IncidentRoutes == nil || record.Overlays == nil || record.StableBoundaries == nil || record.EnvironmentRequirements == nil {
		return errors.New("INVALID_PROFILE_RECIPE: required field missing")
	}
	if record.TaxonomyVersion != TaxonomyVersionV1 {
		return fmt.Errorf("RECIPE_TAXONOMY_UNSUPPORTED: %q", record.TaxonomyVersion)
	}
	if _, err := ParseContentVersion(record.RecipeVersion); err != nil {
		return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
	}
	if _, err := ParseQualifiedID(record.ID); err != nil {
		return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
	}
	if _, err := ParseLocalID(record.Family); err != nil {
		return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
	}
	if record.Template != "" {
		if _, err := ParseLocalID(record.Template); err != nil {
			return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
		}
	}
	if strings.TrimSpace(record.DisplayName) != record.DisplayName || record.DisplayName == "" {
		return errors.New("INVALID_PROFILE_RECIPE: invalid display name")
	}
	if err := validateRecipeMembers(record); err != nil {
		return err
	}
	return nil
}

func validateRecipeMembers(record *ProfileRecipeRecord) error {
	canonical := CanonicalSlots()
	if len(record.Slots) != len(canonical) {
		return errors.New("RECIPE_SLOT_COVERAGE_INVALID: Recipe must contain all canonical slots")
	}
	gateIDs := make(map[string]struct{})
	for index := range record.Slots {
		slot := &record.Slots[index]
		if slot.SlotID != canonical[index].ID {
			return errors.New("RECIPE_SLOT_COVERAGE_INVALID: slots must be unique and canonical")
		}
		if slot.Applicability != SlotMandatory && slot.Applicability != SlotConditional {
			return errors.New("INVALID_PROFILE_RECIPE: invalid slot applicability")
		}
		if slot.Pipeline == nil || slot.Gates == nil || slot.Transitions == nil {
			return errors.New("INVALID_PROFILE_RECIPE: slot collections are required")
		}
		if err := validateOwnerShape(slot); err != nil {
			return err
		}
		stepIDs := make(map[string]struct{}, len(slot.Pipeline))
		for _, step := range slot.Pipeline {
			if _, err := ParseLocalID(step.ID); err != nil {
				return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
			}
			if _, exists := stepIDs[step.ID]; exists {
				return errors.New("INVALID_PROFILE_RECIPE: duplicate pipeline step id")
			}
			stepIDs[step.ID] = struct{}{}
			if _, err := ParseQualifiedID(step.Selector.ProviderID); err != nil {
				return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
			}
			if _, err := ParseLocalID(step.Selector.BindingID); err != nil {
				return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
			}
			if !validStageSpan(step.StageSpan) || !spanContains(step.StageSpan, []SlotID{slot.SlotID}) || step.RequiredInputArtifact == "" || step.ProducedOutputArtifact == "" {
				return errors.New("STAGE_SPAN_INVALID: pipeline step must cover its slot with a contiguous span")
			}
		}
		seenSignals := make(map[string]struct{}, len(slot.Transitions))
		for _, transition := range slot.Transitions {
			if transition.Signal == "" || strings.TrimSpace(transition.Signal) != transition.Signal || !validSlotID(transition.Target) {
				return errors.New("INVALID_PROFILE_RECIPE: invalid transition")
			}
			if _, exists := seenSignals[transition.Signal]; exists {
				return errors.New("INVALID_PROFILE_RECIPE: duplicate transition signal")
			}
			seenSignals[transition.Signal] = struct{}{}
		}
		for _, gate := range slot.Gates {
			if _, err := ParseLocalID(gate.ID); err != nil || gate.Predicate == "" || strings.TrimSpace(gate.Predicate) != gate.Predicate || !validGateAuthority(gate.Authority) || gate.EvidenceRequirements == nil {
				return errors.New("INVALID_PROFILE_RECIPE: invalid neutral gate")
			}
			if _, exists := gateIDs[gate.ID]; exists {
				return errors.New("INVALID_PROFILE_RECIPE: duplicate gate id")
			}
			gateIDs[gate.ID] = struct{}{}
			if err := validateEvidenceRequirements(gate.EvidenceRequirements); err != nil {
				return err
			}
		}
	}

	incidentTypes := make(map[string]struct{}, len(record.IncidentRoutes))
	for _, route := range record.IncidentRoutes {
		if route.IncidentType == "" || strings.TrimSpace(route.IncidentType) != route.IncidentType || !validSlotID(route.ReturnTo) || (route.IfUnavailable != IncidentStop && route.IfUnavailable != IncidentReplan) {
			return errors.New("INVALID_PROFILE_RECIPE: invalid incident route")
		}
		if _, err := ParseQualifiedID(route.Handler.ProviderID); err != nil {
			return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
		}
		if _, err := ParseLocalID(route.Handler.BindingID); err != nil {
			return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
		}
		if _, exists := incidentTypes[route.IncidentType]; exists {
			return errors.New("INVALID_PROFILE_RECIPE: duplicate incident route")
		}
		incidentTypes[route.IncidentType] = struct{}{}
	}

	addOnIDs := make(map[string]struct{}, len(record.AddOns))
	for _, addOn := range record.AddOns {
		if _, err := ParseLocalID(addOn.ID); err != nil || (addOn.Kind != AddOnIncidentHandler && addOn.Kind != AddOnSpecialistCheck) || !validSlotID(addOn.SlotID) || addOn.IncidentTypes == nil || addOn.EvidenceRequirements == nil {
			return errors.New("INVALID_PROFILE_RECIPE: invalid Add-on")
		}
		if _, exists := addOnIDs[addOn.ID]; exists {
			return errors.New("INVALID_PROFILE_RECIPE: duplicate Add-on id")
		}
		addOnIDs[addOn.ID] = struct{}{}
		if _, err := ParseQualifiedID(addOn.Selector.ProviderID); err != nil {
			return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
		}
		if _, err := ParseLocalID(addOn.Selector.BindingID); err != nil {
			return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
		}
		if addOn.Kind == AddOnIncidentHandler && len(addOn.IncidentTypes) == 0 {
			return errors.New("INVALID_PROFILE_RECIPE: incident-handler Add-on has no incident types")
		}
		if err := uniqueStrings(addOn.IncidentTypes, "INVALID_PROFILE_RECIPE"); err != nil {
			return err
		}
		if err := validateEvidenceRequirements(addOn.EvidenceRequirements); err != nil {
			return err
		}
	}

	overlayIDs := make(map[string]struct{}, len(record.Overlays))
	for _, overlay := range record.Overlays {
		if _, err := ParseLocalID(overlay.ID); err != nil || overlay.Precedence == nil || len(overlay.Precedence) == 0 || overlay.PausedBindings == nil || overlay.Rationale == "" {
			return errors.New("OVERLAY_INVALID: incomplete overlay")
		}
		if _, exists := overlayIDs[overlay.ID]; exists {
			return errors.New("OVERLAY_INVALID: duplicate overlay id")
		}
		overlayIDs[overlay.ID] = struct{}{}
		if overlay.SelectedAlternative != "" {
			if _, err := ParseLocalID(overlay.SelectedAlternative); err != nil {
				return errors.New("OVERLAY_INVALID: invalid selected alternative")
			}
		}
		if err := uniqueStrings(overlay.Precedence, "OVERLAY_INVALID"); err != nil {
			return err
		}
		for _, precedence := range overlay.Precedence {
			if _, err := ParseLocalID(precedence); err != nil {
				return errors.New("OVERLAY_INVALID: invalid precedence entry")
			}
		}
		seenPaused := make(map[string]struct{}, len(overlay.PausedBindings))
		for _, selector := range overlay.PausedBindings {
			if _, err := ParseQualifiedID(selector.ProviderID); err != nil {
				return errors.New("OVERLAY_INVALID: invalid paused Provider")
			}
			if _, err := ParseLocalID(selector.BindingID); err != nil {
				return errors.New("OVERLAY_INVALID: invalid paused Binding")
			}
			key := selector.ProviderID + "\x00" + selector.BindingID
			if _, exists := seenPaused[key]; exists {
				return errors.New("OVERLAY_INVALID: duplicate paused Binding")
			}
			seenPaused[key] = struct{}{}
		}
	}
	if err := uniqueStrings(record.StableBoundaries, "INVALID_PROFILE_RECIPE"); err != nil {
		return err
	}
	for _, boundary := range record.StableBoundaries {
		if boundary == "" || strings.TrimSpace(boundary) != boundary {
			return errors.New("INVALID_PROFILE_RECIPE: invalid stable boundary")
		}
	}
	if _, err := execution.NormalizeRequirements(record.EnvironmentRequirements); err != nil {
		return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
	}
	return nil
}

func validateOwnerShape(slot *SlotRecipe) error {
	providerRequired := slot.SlotID == SlotProblemFraming || slot.SlotID == SlotSolutionSpecification || slot.SlotID == SlotDeliveryPlanning || slot.SlotID == SlotImplementation || slot.SlotID == SlotImplementationTDD || slot.SlotID == SlotReviewRemediation
	switch slot.OutcomeOwner.Kind {
	case OwnerProviderBinding:
		if slot.OutcomeOwner.StepID == "" || slot.OutcomeOwner.HostAction != "" || slot.HostAction != nil || len(slot.Pipeline) == 0 {
			return errors.New("OUTCOME_OWNER_MISSING: invalid Provider owner")
		}
	case OwnerHostAction:
		expected := expectedHostAction(slot.SlotID)
		if providerRequired || expected == "" || slot.OutcomeOwner.StepID != "" || slot.OutcomeOwner.HostAction != expected || slot.HostAction == nil || slot.HostAction.ID != expected || slot.HostAction.InputArtifact == "" || slot.HostAction.OutputArtifact == "" {
			return errors.New("OUTCOME_OWNER_MISSING: invalid Host action owner")
		}
	case OwnerNone:
		if slot.SlotID != SlotIncidentRecovery || slot.Applicability != SlotConditional || slot.OutcomeOwner.StepID != "" || slot.OutcomeOwner.HostAction != "" || slot.HostAction != nil || len(slot.Pipeline) != 0 {
			return errors.New("OUTCOME_OWNER_MISSING: none owner is not allowed")
		}
	default:
		return errors.New("OUTCOME_OWNER_MISSING: unknown owner kind")
	}
	return nil
}

func expectedHostAction(slotID SlotID) string {
	switch slotID {
	case SlotWorkspacePreparation:
		return "workspace.prepare-or-confirm"
	case SlotFreshVerification:
		return "verification.execute"
	case SlotCloseout:
		return "closeout.execute"
	default:
		return ""
	}
}

func validateProbe(probe *DiscoveryProbe) error {
	if probe.Kind != "path-exists" && probe.Kind != "one-level-version-path-exists" {
		return fmt.Errorf("DISCOVERY_PROBE_SHAPE_INVALID: invalid discovery kind %q", probe.Kind)
	}
	if probe.Root != "user-home" && probe.Root != "xdg-config-home" && probe.Root != "project-root" {
		return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: invalid discovery root %q", probe.Root)
	}
	paths := make([]string, 0, 2)
	switch probe.Kind {
	case "path-exists":
		if probe.CandidatePath == "" || probe.EvidencePath == "" || probe.Prefix != "" {
			return errors.New("DISCOVERY_PROBE_SHAPE_INVALID: path-exists payload mismatch")
		}
		paths = append(paths, probe.CandidatePath, probe.EvidencePath)
	case "one-level-version-path-exists":
		if probe.Prefix == "" || probe.EvidencePath == "" || probe.CandidatePath != "" {
			return errors.New("DISCOVERY_PROBE_SHAPE_INVALID: version probe payload mismatch")
		}
		paths = append(paths, probe.Prefix, probe.EvidencePath)
	}
	for _, value := range paths {
		if !safeRelativePath(value) {
			return fmt.Errorf("DISCOVERY_PATH_INVALID: %q", value)
		}
	}
	return nil
}

func validateResponsibilities(values []ResponsibilityClaim) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validOwnershipNamespace(value.Namespace) || value.Name == "" || strings.TrimSpace(value.Name) != value.Name || !validSlotID(value.SlotID) {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: invalid responsibility claim")
		}
		key := string(value.Namespace) + "\x00" + value.Name + "\x00" + string(value.SlotID)
		if _, exists := seen[key]; exists {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: duplicate responsibility claim")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateEffects(values []string) error {
	if err := uniqueStrings(values, "INVALID_PROVIDER_DESCRIPTOR"); err != nil {
		return err
	}
	for _, value := range values {
		switch value {
		case "read-project", "write-project", "run-process", "git-local", "network-read", "network-write":
		default:
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: invalid effect %q", value)
		}
	}
	return nil
}

func validateResources(values []string) error {
	if err := uniqueStrings(values, "INVALID_PROVIDER_DESCRIPTOR"); err != nil {
		return err
	}
	for _, value := range values {
		switch value {
		case "project", "project-worktree", "git-repository":
		default:
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: invalid resource %q", value)
		}
	}
	return nil
}

func validateEvidenceRequirements(values []EvidenceRequirementRecord) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value.Kind == "" || strings.TrimSpace(value.Kind) != value.Kind || value.Minimum == 0 || value.Description == "" || strings.TrimSpace(value.Description) != value.Description {
			return errors.New("INVALID_PROFILE_RECIPE: invalid evidence requirement")
		}
		if _, exists := seen[value.Kind]; exists {
			return errors.New("INVALID_PROFILE_RECIPE: duplicate evidence requirement")
		}
		seen[value.Kind] = struct{}{}
	}
	return nil
}

func validBindingKind(value BindingKind) bool {
	return value == BindingSkill || value == BindingAgent || value == BindingRole || value == BindingInstruction || value == BindingTool
}

func validInvocation(value InvocationDisposition) bool {
	return value == InvocationHumanExplicit || value == InvocationModel || value == InvocationHost || value == InvocationInternal
}

func validInternalCallMode(value InternalCallMode) bool {
	return value == InternalCreditOnly || value == InternalDispatchBefore || value == InternalDispatchAfter
}

func validOwnershipNamespace(value OwnershipNamespace) bool {
	return value == OwnershipStage || value == OwnershipProcedure || value == OwnershipIncident || value == OwnershipAssurance || value == OwnershipHostAction || value == OwnershipGate
}

func validGateAuthority(value GateAuthority) bool {
	return value == GateOAWCore || value == GateHost || value == GateUser
}

func validSlotID(value SlotID) bool {
	_, exists := canonicalSlotPosition(value)
	return exists
}

func validStageSpan(values []SlotID) bool {
	if len(values) == 0 {
		return false
	}
	start, exists := canonicalSlotPosition(values[0])
	if !exists {
		return false
	}
	for index, value := range values {
		position, exists := canonicalSlotPosition(value)
		if !exists || position != start+index {
			return false
		}
	}
	return true
}

func spanContains(parent, child []SlotID) bool {
	if len(parent) == 0 || len(child) == 0 {
		return false
	}
	parentStart, parentOK := canonicalSlotPosition(parent[0])
	childStart, childOK := canonicalSlotPosition(child[0])
	return parentOK && childOK && childStart >= parentStart && childStart+len(child) <= parentStart+len(parent)
}

func canonicalSlotPosition(value SlotID) (int, bool) {
	for index, definition := range CanonicalSlots() {
		if definition.ID == value {
			return index, true
		}
	}
	return 0, false
}

func safeRelativePath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") || strings.Contains(value, ":") {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." || strings.ContainsAny(component, "*?[]{}()") {
			return false
		}
	}
	return true
}

func uniqueStrings(values []string, code string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			return errors.New(code + ": duplicate value")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func cloneProvider(record ProviderDescriptorRecord) ProviderDescriptorRecord {
	record.Distributions = cloneSlice(record.Distributions)
	record.Discovery = cloneSlice(record.Discovery)
	for index := range record.Discovery {
		record.Discovery[index].Hosts = cloneSlice(record.Discovery[index].Hosts)
	}
	record.Bindings = cloneSlice(record.Bindings)
	for index := range record.Bindings {
		binding := &record.Bindings[index]
		binding.Responsibilities = cloneSlice(binding.Responsibilities)
		binding.MaximumEffects = cloneSlice(binding.MaximumEffects)
		binding.Resources = cloneSlice(binding.Resources)
		binding.SupportedTopologies = cloneSlice(binding.SupportedTopologies)
		binding.StageSpan = cloneSlice(binding.StageSpan)
		binding.InternalCalls = cloneSlice(binding.InternalCalls)
		for callIndex := range binding.InternalCalls {
			binding.InternalCalls[callIndex].StageSpan = cloneSlice(binding.InternalCalls[callIndex].StageSpan)
		}
		binding.Alternatives = cloneSlice(binding.Alternatives)
		binding.Conflicts = cloneSlice(binding.Conflicts)
	}
	record.Capabilities = cloneSlice(record.Capabilities)
	for index := range record.Capabilities {
		record.Capabilities[index].RequestModes = cloneSlice(record.Capabilities[index].RequestModes)
		record.Capabilities[index].BindingRefs = cloneSlice(record.Capabilities[index].BindingRefs)
	}
	return record
}

func cloneRecipe(record ProfileRecipeRecord) ProfileRecipeRecord {
	record.Slots = cloneSlice(record.Slots)
	for slotIndex := range record.Slots {
		slot := &record.Slots[slotIndex]
		slot.Pipeline = cloneSlice(slot.Pipeline)
		for stepIndex := range slot.Pipeline {
			slot.Pipeline[stepIndex].StageSpan = cloneSlice(slot.Pipeline[stepIndex].StageSpan)
		}
		if slot.HostAction != nil {
			hostAction := *slot.HostAction
			slot.HostAction = &hostAction
		}
		slot.Gates = cloneSlice(slot.Gates)
		for gateIndex := range slot.Gates {
			slot.Gates[gateIndex].EvidenceRequirements = cloneSlice(slot.Gates[gateIndex].EvidenceRequirements)
		}
		slot.Transitions = cloneSlice(slot.Transitions)
	}
	record.AddOns = cloneSlice(record.AddOns)
	for index := range record.AddOns {
		record.AddOns[index].IncidentTypes = cloneSlice(record.AddOns[index].IncidentTypes)
		record.AddOns[index].EvidenceRequirements = cloneSlice(record.AddOns[index].EvidenceRequirements)
	}
	record.IncidentRoutes = cloneSlice(record.IncidentRoutes)
	record.Overlays = cloneSlice(record.Overlays)
	for index := range record.Overlays {
		record.Overlays[index].Precedence = cloneSlice(record.Overlays[index].Precedence)
		record.Overlays[index].PausedBindings = cloneSlice(record.Overlays[index].PausedBindings)
	}
	record.StableBoundaries = cloneSlice(record.StableBoundaries)
	record.EnvironmentRequirements = cloneSlice(record.EnvironmentRequirements)
	for index := range record.EnvironmentRequirements {
		record.EnvironmentRequirements[index].AcceptedDispositions = cloneSlice(record.EnvironmentRequirements[index].AcceptedDispositions)
	}
	return record
}

func cloneAliases(record ProfileAliasSetRecord) ProfileAliasSetRecord {
	record.Aliases = cloneSlice(record.Aliases)
	return record
}
