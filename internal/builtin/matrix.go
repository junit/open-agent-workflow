package builtin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/provideraudit"
)

const ProfileMatrixSchemaV1 = "oaw.profile-matrix/v1"

var matrixDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type MatrixBinding struct {
	ProviderID           string                        `json:"provider_id"`
	BindingID            string                        `json:"binding_id"`
	Host                 string                        `json:"host"`
	Surface              string                        `json:"surface"`
	Kind                 catalog.BindingKind           `json:"kind"`
	Reference            string                        `json:"reference"`
	Invocation           catalog.InvocationDisposition `json:"invocation"`
	StageSpan            []catalog.SlotID              `json:"stage_span"`
	MacroMode            catalog.InternalCallMode      `json:"macro_mode,omitempty"`
	DistributionRevision string                        `json:"distribution_revision"`
	BindingTreeDigest    string                        `json:"binding_tree_digest"`
	RequiredFeatures     []host.FeatureID              `json:"required_features"`
	Topologies           []execution.Topology          `json:"topologies"`
	Paused               bool                          `json:"paused"`
}

type MatrixSlot struct {
	SlotID        catalog.SlotID            `json:"slot_id"`
	Applicability catalog.SlotApplicability `json:"applicability"`
	OutcomeOwner  string                    `json:"outcome_owner"`
	Pipeline      []MatrixBinding           `json:"pipeline"`
	HostActionID  string                    `json:"host_action_id,omitempty"`
	GateIDs       []string                  `json:"gate_ids"`
	IncidentTypes []string                  `json:"incident_types"`
}

type MatrixProfile struct {
	Alias        string       `json:"alias"`
	RecipeID     string       `json:"recipe_id"`
	Family       string       `json:"family"`
	Template     string       `json:"template,omitempty"`
	RecipeDigest string       `json:"recipe_digest"`
	Slots        []MatrixSlot `json:"slots"`
}

type ProfileMatrixRecord struct {
	SchemaVersion         string          `json:"schema_version"`
	CanonicalMatrixDigest string          `json:"canonical_matrix_digest"`
	SourceAuditDigest     string          `json:"source_audit_digest"`
	Profiles              []MatrixProfile `json:"profiles"`
	Digest                string          `json:"digest"`
}

func BuildProfileMatrix(value catalog.Catalog, audit provideraudit.Manifest) (ProfileMatrixRecord, error) {
	if err := provideraudit.Validate(audit); err != nil {
		return ProfileMatrixRecord{}, matrixError("source audit is invalid", err)
	}
	providers := providerRecordIndex(value.Providers())
	recipes := recipeRecordIndex(value.Recipes())
	aliases := value.Aliases()
	if len(providers) != 3 || len(recipes) != 4 || len(aliases) != 4 {
		return ProfileMatrixRecord{}, matrixError("built-in inventory is not exact", nil)
	}
	sort.Slice(aliases, func(left, right int) bool { return aliases[left].Alias < aliases[right].Alias })

	profiles := make([]MatrixProfile, 0, len(aliases))
	for _, alias := range aliases {
		recipe, found := recipes[alias.RecipeID]
		if !found {
			return ProfileMatrixRecord{}, matrixError("alias Recipe is missing", nil)
		}
		_, recipeDigest, err := catalog.NormalizeAndDigestRecipe(value.Providers(), recipe)
		if err != nil {
			return ProfileMatrixRecord{}, matrixError("Recipe cannot be normalized", err)
		}
		profile, err := projectRecipe(alias.Alias, recipe, recipeDigest, providers, audit)
		if err != nil {
			return ProfileMatrixRecord{}, err
		}
		profiles = append(profiles, profile)
	}
	result := ProfileMatrixRecord{
		SchemaVersion: ProfileMatrixSchemaV1, CanonicalMatrixDigest: provideraudit.CanonicalMatrixDigest,
		SourceAuditDigest: audit.Digest, Profiles: profiles,
	}
	result.Digest = result.ContentDigest()
	if err := validateMatrixRecord(result); err != nil {
		return ProfileMatrixRecord{}, err
	}
	return cloneProfileMatrix(result), nil
}

func DecodeProfileMatrix(raw []byte) (ProfileMatrixRecord, error) {
	var value ProfileMatrixRecord
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return ProfileMatrixRecord{}, matrixError("decode projection", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return ProfileMatrixRecord{}, matrixError("trailing JSON value", nil)
		}
		return ProfileMatrixRecord{}, matrixError("decode trailing JSON", err)
	}
	if err := validateMatrixRecord(value); err != nil {
		return ProfileMatrixRecord{}, err
	}
	return cloneProfileMatrix(value), nil
}

func ValidateProfileMatrix(value catalog.Catalog, audit provideraudit.Manifest, matrix ProfileMatrixRecord) error {
	if err := validateMatrixRecord(matrix); err != nil {
		return err
	}
	expected, err := BuildProfileMatrix(value, audit)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(expected, matrix) {
		return matrixError("projection differs from the built-in Catalog or source audit", nil)
	}
	return nil
}

func LoadProfileMatrix() (ProfileMatrixRecord, error) {
	return loadProfileMatrixFromFS(assets.FS())
}

func loadProfileMatrixFromFS(files fs.FS) (ProfileMatrixRecord, error) {
	raw, err := fs.ReadFile(files, "profile-matrix.json")
	if err != nil {
		return ProfileMatrixRecord{}, matrixError("read projection", err)
	}
	return DecodeProfileMatrix(raw)
}

func (value ProfileMatrixRecord) ContentDigest() string {
	clone := cloneProfileMatrix(value)
	clone.Digest = ""
	digest, _, err := canonicaljson.Digest(clone)
	if err != nil {
		return ""
	}
	return digest
}

func projectRecipe(alias string, recipe catalog.ProfileRecipeRecord, recipeDigest string, providers map[string]catalog.ProviderDescriptorRecord, audit provideraudit.Manifest) (MatrixProfile, error) {
	profile := MatrixProfile{Alias: alias, RecipeID: recipe.ID, Family: recipe.Family, Template: recipe.Template, RecipeDigest: recipeDigest, Slots: make([]MatrixSlot, len(recipe.Slots))}
	for index, slot := range recipe.Slots {
		gateIDs := make([]string, len(slot.Gates))
		for gateIndex, gate := range slot.Gates {
			gateIDs[gateIndex] = gate.ID
		}
		sort.Strings(gateIDs)
		hostActionID := ""
		if slot.HostAction != nil {
			hostActionID = slot.HostAction.ID
		}
		profile.Slots[index] = MatrixSlot{
			SlotID: slot.SlotID, Applicability: slot.Applicability, Pipeline: []MatrixBinding{}, HostActionID: hostActionID,
			GateIDs: gateIDs, IncidentTypes: []string{},
		}
	}
	for _, route := range recipe.IncidentRoutes {
		slot := matrixSlotPointer(profile.Slots, catalog.SlotIncidentRecovery)
		slot.IncidentTypes = append(slot.IncidentTypes, route.IncidentType)
	}
	sort.Strings(matrixSlotPointer(profile.Slots, catalog.SlotIncidentRecovery).IncidentTypes)

	seenMulti := map[string]struct{}{}
	for _, slot := range recipe.Slots {
		for _, step := range slot.Pipeline {
			multiKey := step.Selector.ProviderID + "\x00" + step.Selector.BindingID + "\x00" + slotIdentity(step.StageSpan)
			if len(step.StageSpan) > 1 {
				if _, found := seenMulti[multiKey]; found {
					continue
				}
				seenMulti[multiKey] = struct{}{}
			}
			if err := appendSelectorProjection(&profile, step.Selector, step.StageSpan, "", false, providers, audit, map[string]bool{}); err != nil {
				return MatrixProfile{}, err
			}
			provider := providers[step.Selector.ProviderID]
			binding, _ := descriptorMatrixBinding(provider, step.Selector.BindingID)
			for _, alternativeID := range binding.Alternatives {
				if err := appendSelectorProjection(&profile, catalog.BindingSelector{ProviderID: step.Selector.ProviderID, BindingID: alternativeID}, step.StageSpan, "", false, providers, audit, map[string]bool{}); err != nil {
					return MatrixProfile{}, err
				}
			}
		}
	}
	for _, overlay := range recipe.Overlays {
		for _, paused := range overlay.PausedBindings {
			provider, found := providers[paused.ProviderID]
			if !found {
				return MatrixProfile{}, matrixError("paused Provider is missing", nil)
			}
			binding, found := descriptorMatrixBinding(provider, paused.BindingID)
			if !found {
				return MatrixProfile{}, matrixError("paused Binding is missing", nil)
			}
			row, err := projectBinding(paused.ProviderID, provider, binding, binding.StageSpan, "", true, audit)
			if err != nil {
				return MatrixProfile{}, err
			}
			anchor := bindingAnchor(binding.StageSpan, binding.Responsibilities)
			appendUniqueMatrixBinding(matrixSlotPointer(profile.Slots, anchor), row)
		}
	}
	for index, slot := range recipe.Slots {
		switch slot.OutcomeOwner.Kind {
		case catalog.OwnerHostAction:
			profile.Slots[index].OutcomeOwner = "host-action:" + slot.OutcomeOwner.HostAction
		case catalog.OwnerNone:
			profile.Slots[index].OutcomeOwner = "none"
		case catalog.OwnerProviderBinding:
			owner, err := matrixOutcomeOwner(slot, providers)
			if err != nil {
				return MatrixProfile{}, err
			}
			profile.Slots[index].OutcomeOwner = owner
		}
	}
	return profile, nil
}

func matrixOutcomeOwner(slot catalog.SlotRecipe, providers map[string]catalog.ProviderDescriptorRecord) (string, error) {
	var ownerStep catalog.PipelineStep
	found := false
	for _, step := range slot.Pipeline {
		if step.ID == slot.OutcomeOwner.StepID {
			ownerStep = step
			found = true
			break
		}
	}
	if !found {
		return "", matrixError("Provider outcome step is missing", nil)
	}
	provider, found := providers[ownerStep.Selector.ProviderID]
	if !found {
		return "", matrixError("Provider outcome owner is missing", nil)
	}
	binding, found := descriptorMatrixBinding(provider, ownerStep.Selector.BindingID)
	if !found {
		return "", matrixError("Provider outcome Binding is missing", nil)
	}
	owners, err := expandedMatrixOutcomeOwners(provider, binding, slot.SlotID, map[string]bool{})
	if err != nil {
		return "", err
	}
	if len(owners) != 1 {
		return "", matrixError("Provider outcome owner is not unique", nil)
	}
	return ownerStep.Selector.ProviderID + "/" + owners[0], nil
}

func expandedMatrixOutcomeOwners(provider catalog.ProviderDescriptorRecord, binding catalog.BindingRecord, slot catalog.SlotID, stack map[string]bool) ([]string, error) {
	if stack[binding.ID] {
		return nil, matrixError("recursive outcome owner projection", nil)
	}
	stack[binding.ID] = true
	defer delete(stack, binding.ID)

	owners := []string{}
	if claimsOutcome(binding.Responsibilities, slot) {
		owners = append(owners, binding.ID)
	}
	for _, call := range binding.InternalCalls {
		child, found := descriptorMatrixBinding(provider, call.BindingID)
		if !found {
			return nil, matrixError("internal outcome Binding is missing", nil)
		}
		children, err := expandedMatrixOutcomeOwners(provider, child, slot, stack)
		if err != nil {
			return nil, err
		}
		owners = append(owners, children...)
	}
	return owners, nil
}

func appendSelectorProjection(profile *MatrixProfile, selector catalog.BindingSelector, span []catalog.SlotID, mode catalog.InternalCallMode, paused bool, providers map[string]catalog.ProviderDescriptorRecord, audit provideraudit.Manifest, stack map[string]bool) error {
	key := selector.ProviderID + "\x00" + selector.BindingID
	if stack[key] {
		return matrixError("recursive macro projection", nil)
	}
	stack[key] = true
	defer delete(stack, key)
	provider, found := providers[selector.ProviderID]
	if !found {
		return matrixError("pipeline Provider is missing", nil)
	}
	binding, found := descriptorMatrixBinding(provider, selector.BindingID)
	if !found {
		return matrixError("pipeline Binding is missing", nil)
	}
	for _, call := range binding.InternalCalls {
		if call.Mode != catalog.InternalDispatchBefore {
			continue
		}
		if err := appendSelectorProjection(profile, catalog.BindingSelector{ProviderID: selector.ProviderID, BindingID: call.BindingID}, call.StageSpan, call.Mode, paused, providers, audit, stack); err != nil {
			return err
		}
	}
	row, err := projectBinding(selector.ProviderID, provider, binding, span, mode, paused, audit)
	if err != nil {
		return err
	}
	appendUniqueMatrixBinding(matrixSlotPointer(profile.Slots, bindingAnchor(span, binding.Responsibilities)), row)
	for _, call := range binding.InternalCalls {
		if call.Mode == catalog.InternalDispatchBefore {
			continue
		}
		if err := appendSelectorProjection(profile, catalog.BindingSelector{ProviderID: selector.ProviderID, BindingID: call.BindingID}, call.StageSpan, call.Mode, paused, providers, audit, stack); err != nil {
			return err
		}
	}
	return nil
}

func projectBinding(providerID string, provider catalog.ProviderDescriptorRecord, binding catalog.BindingRecord, span []catalog.SlotID, mode catalog.InternalCallMode, paused bool, audit provideraudit.Manifest) (MatrixBinding, error) {
	audited, found := audit.Binding(providerID, binding.ID)
	if !found || len(provider.Distributions) != 1 || binding.TreeDigest != audited.TreeDigest {
		return MatrixBinding{}, matrixError("Binding source provenance is missing", nil)
	}
	return MatrixBinding{
		ProviderID: providerID, BindingID: binding.ID, Host: binding.Host, Surface: binding.Surface, Kind: binding.Kind,
		Reference: binding.Reference, Invocation: binding.Invocation, StageSpan: append([]catalog.SlotID{}, span...), MacroMode: mode,
		DistributionRevision: provider.Distributions[0].Revision, BindingTreeDigest: audited.TreeDigest,
		RequiredFeatures: delegationFeatures(binding.Delegation), Topologies: append([]execution.Topology{}, binding.SupportedTopologies...), Paused: paused,
	}, nil
}

func validateMatrixRecord(value ProfileMatrixRecord) error {
	if value.SchemaVersion != ProfileMatrixSchemaV1 || value.CanonicalMatrixDigest != provideraudit.CanonicalMatrixDigest || !matrixDigestPattern.MatchString(value.SourceAuditDigest) || !matrixDigestPattern.MatchString(value.Digest) || value.ContentDigest() != value.Digest || len(value.Profiles) != 4 {
		return matrixError("projection header or digest mismatch", nil)
	}
	aliases := map[string]struct{}{}
	canonical := catalog.CanonicalSlots()
	for _, profile := range value.Profiles {
		if profile.Alias == "" || profile.RecipeID == "" || profile.Family == "" || !matrixDigestPattern.MatchString(profile.RecipeDigest) || len(profile.Slots) != len(canonical) {
			return matrixError("profile shape is invalid", nil)
		}
		if _, duplicate := aliases[profile.Alias]; duplicate {
			return matrixError("profile alias is duplicated", nil)
		}
		aliases[profile.Alias] = struct{}{}
		for index, slot := range profile.Slots {
			if slot.SlotID != canonical[index].ID || slot.OutcomeOwner == "" || slot.Pipeline == nil || slot.GateIDs == nil || slot.IncidentTypes == nil {
				return matrixError("slot shape is invalid", nil)
			}
			for _, binding := range slot.Pipeline {
				if binding.ProviderID == "" || binding.BindingID == "" || binding.Host == "" || binding.Surface == "" || binding.Reference == "" || binding.DistributionRevision == "" || binding.BindingTreeDigest == "" || len(binding.StageSpan) == 0 || binding.RequiredFeatures == nil || binding.Topologies == nil {
					return matrixError("Binding row is incomplete", nil)
				}
				if binding.Reference == "requirements" || binding.Reference == "delivery-gate" || (binding.Host == "codex" && binding.Kind == catalog.BindingAgent) {
					return matrixError("fictional or Hook Binding is forbidden", nil)
				}
			}
		}
	}
	return nil
}

func providerRecordIndex(values []catalog.ProviderDescriptorRecord) map[string]catalog.ProviderDescriptorRecord {
	result := make(map[string]catalog.ProviderDescriptorRecord, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result
}

func recipeRecordIndex(values []catalog.ProfileRecipeRecord) map[string]catalog.ProfileRecipeRecord {
	result := make(map[string]catalog.ProfileRecipeRecord, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result
}

func descriptorMatrixBinding(provider catalog.ProviderDescriptorRecord, id string) (catalog.BindingRecord, bool) {
	for _, binding := range provider.Bindings {
		if binding.ID == id {
			return binding, true
		}
	}
	return catalog.BindingRecord{}, false
}

func matrixSlotPointer(slots []MatrixSlot, id catalog.SlotID) *MatrixSlot {
	for index := range slots {
		if slots[index].SlotID == id {
			return &slots[index]
		}
	}
	panic("matrix slot is missing: " + id)
}

func appendUniqueMatrixBinding(slot *MatrixSlot, row MatrixBinding) {
	for _, current := range slot.Pipeline {
		if current.ProviderID == row.ProviderID && current.BindingID == row.BindingID && current.MacroMode == row.MacroMode && current.Paused == row.Paused && slices.Equal(current.StageSpan, row.StageSpan) {
			return
		}
	}
	slot.Pipeline = append(slot.Pipeline, row)
}

func bindingAnchor(span []catalog.SlotID, claims []catalog.ResponsibilityClaim) catalog.SlotID {
	positions := map[catalog.SlotID]int{}
	for index, slot := range catalog.CanonicalSlots() {
		positions[slot.ID] = index
	}
	anchor := span[0]
	anchorPosition := len(positions)
	for _, claim := range claims {
		position, found := positions[claim.SlotID]
		if !claim.OutcomeOwner || !found || !slices.Contains(span, claim.SlotID) || position >= anchorPosition {
			continue
		}
		anchor = claim.SlotID
		anchorPosition = position
	}
	return anchor
}

func claimsOutcome(values []catalog.ResponsibilityClaim, slot catalog.SlotID) bool {
	return slices.ContainsFunc(values, func(value catalog.ResponsibilityClaim) bool { return value.SlotID == slot && value.OutcomeOwner })
}

func delegationFeatures(value catalog.DelegationRequirements) []host.FeatureID {
	result := []host.FeatureID{}
	if value.Child {
		result = append(result, host.FeatureChildDelegation)
	}
	if value.ParallelChild {
		result = append(result, host.FeatureParallelChildDelegation)
	}
	if value.NestedChild {
		result = append(result, host.FeatureNestedChildDelegation)
	}
	if value.NestedParallel {
		result = append(result, host.FeatureNestedParallelDelegation)
	}
	return result
}

func slotIdentity(values []catalog.SlotID) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = string(value)
	}
	return strings.Join(parts, "\x00")
}

func cloneProfileMatrix(value ProfileMatrixRecord) ProfileMatrixRecord {
	profiles := value.Profiles
	value.Profiles = make([]MatrixProfile, len(profiles))
	for profileIndex, profile := range profiles {
		value.Profiles[profileIndex] = profile
		slots := profile.Slots
		value.Profiles[profileIndex].Slots = make([]MatrixSlot, len(slots))
		for slotIndex, slot := range slots {
			value.Profiles[profileIndex].Slots[slotIndex] = slot
			value.Profiles[profileIndex].Slots[slotIndex].GateIDs = append([]string{}, slot.GateIDs...)
			value.Profiles[profileIndex].Slots[slotIndex].IncidentTypes = append([]string{}, slot.IncidentTypes...)
			value.Profiles[profileIndex].Slots[slotIndex].Pipeline = make([]MatrixBinding, len(slot.Pipeline))
			for bindingIndex, binding := range slot.Pipeline {
				value.Profiles[profileIndex].Slots[slotIndex].Pipeline[bindingIndex] = binding
				value.Profiles[profileIndex].Slots[slotIndex].Pipeline[bindingIndex].StageSpan = append([]catalog.SlotID{}, binding.StageSpan...)
				value.Profiles[profileIndex].Slots[slotIndex].Pipeline[bindingIndex].RequiredFeatures = append([]host.FeatureID{}, binding.RequiredFeatures...)
				value.Profiles[profileIndex].Slots[slotIndex].Pipeline[bindingIndex].Topologies = append([]execution.Topology{}, binding.Topologies...)
			}
		}
	}
	return value
}

func matrixError(detail string, err error) error {
	if err == nil {
		return fmt.Errorf("BUILTIN_PROFILE_MATRIX_INVALID: %s", detail)
	}
	return fmt.Errorf("BUILTIN_PROFILE_MATRIX_INVALID: %s: %w", detail, err)
}
