package host

import (
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

type FeatureID string

const (
	FeatureChildDelegation          FeatureID = "child-delegation"
	FeatureParallelChildDelegation  FeatureID = "parallel-child-delegation"
	FeatureNestedChildDelegation    FeatureID = "nested-child-delegation"
	FeatureNestedParallelDelegation FeatureID = "nested-parallel-child-delegation"
)

type Availability string
type ObservationSource string

const (
	AvailabilityAvailable   Availability = "available"
	AvailabilityUnavailable Availability = "unavailable"
	AvailabilityUnknown     Availability = "unknown"
	AvailabilityConfigured  Availability = "host-configured"

	SourceNativeAPI      ObservationSource = "native-api"
	SourceLiveHostIndex  ObservationSource = "live-host-index"
	SourceLiveFilesystem ObservationSource = "live-host-filesystem"
	SourceStaticConfig   ObservationSource = "static-configuration"
)

type FeatureObservation struct {
	Feature           FeatureID         `json:"feature"`
	State             Availability      `json:"state"`
	Source            ObservationSource `json:"source"`
	EvidenceReference string            `json:"evidence_reference"`
	Digest            string            `json:"digest"`
}

type HostActionContract struct {
	ID             string   `json:"id"`
	InputSchema    string   `json:"input_schema"`
	OutcomeSchema  string   `json:"outcome_schema"`
	MaximumEffects []string `json:"maximum_effects"`
	Resources      []string `json:"resources"`
}

type HostActionObservation struct {
	Action            HostActionContract `json:"action"`
	State             Availability       `json:"state"`
	Source            ObservationSource  `json:"source"`
	EvidenceReference string             `json:"evidence_reference"`
	Digest            string             `json:"digest"`
}

var knownDelegationFeatures = []FeatureID{
	FeatureChildDelegation,
	FeatureNestedChildDelegation,
	FeatureNestedParallelDelegation,
	FeatureParallelChildDelegation,
}

var canonicalHostActions = []HostActionContract{
	{
		ID:             "closeout.execute",
		InputSchema:    "oaw.host-action.closeout-input/v1",
		OutcomeSchema:  "oaw.host-action.closeout-outcome/v1",
		MaximumEffects: []string{"git-local", "network-mutation", "read-project", "run-process"},
		Resources:      []string{"git-repository", "network", "project-worktree"},
	},
	{
		ID:             "verification.execute",
		InputSchema:    "oaw.host-action.verification-input/v1",
		OutcomeSchema:  "oaw.host-action.verification-outcome/v1",
		MaximumEffects: []string{"read-project", "run-process"},
		Resources:      []string{"project"},
	},
	{
		ID:             "workspace.prepare-or-confirm",
		InputSchema:    "oaw.host-action.workspace-input/v1",
		OutcomeSchema:  "oaw.host-action.workspace-outcome/v1",
		MaximumEffects: []string{"git-local", "read-project", "run-process", "write-project"},
		Resources:      []string{"git-repository", "project-worktree"},
	},
}

func NewFeatureObservation(input FeatureObservation) (FeatureObservation, error) {
	providedDigest := input.Digest
	input.Digest = ""
	if !slices.Contains(knownDelegationFeatures, input.Feature) || !validAvailabilitySource(input.State, input.Source) || !validOpaqueEvidenceReference(input.EvidenceReference) {
		return FeatureObservation{}, hostError("HOST_FEATURE_OBSERVATION_INVALID", "invalid delegation feature observation", nil)
	}
	digest, _, err := canonicaljson.Digest(input)
	if err != nil {
		return FeatureObservation{}, hostError("HOST_FEATURE_OBSERVATION_INVALID", "feature observation cannot be canonicalized", err)
	}
	if providedDigest != "" && providedDigest != digest {
		return FeatureObservation{}, hostError("HOST_FEATURE_OBSERVATION_INVALID", "feature observation digest mismatch", nil)
	}
	input.Digest = digest
	return input, nil
}

func NewHostActionObservation(input HostActionObservation) (HostActionObservation, error) {
	providedDigest := input.Digest
	input.Digest = ""
	action, ok := normalizeHostActionContract(input.Action)
	if !ok || !validAvailabilitySource(input.State, input.Source) || !validOpaqueEvidenceReference(input.EvidenceReference) {
		return HostActionObservation{}, hostError("HOST_ACTION_OBSERVATION_INVALID", "invalid Host action observation", nil)
	}
	input.Action = action
	digest, _, err := canonicaljson.Digest(input)
	if err != nil {
		return HostActionObservation{}, hostError("HOST_ACTION_OBSERVATION_INVALID", "Host action observation cannot be canonicalized", err)
	}
	if providedDigest != "" && providedDigest != digest {
		return HostActionObservation{}, hostError("HOST_ACTION_OBSERVATION_INVALID", "Host action observation digest mismatch", nil)
	}
	input.Digest = digest
	return input, nil
}

func normalizeFeatureObservations(manifest Manifest, values []FeatureObservation) ([]FeatureObservation, string, error) {
	result := make([]FeatureObservation, len(values))
	for index, value := range values {
		normalized, err := NewFeatureObservation(value)
		if err != nil {
			return nil, "", hostError("HOST_SESSION_INVALID", "invalid feature observation", err)
		}
		if !slices.Contains(manifest.DelegationFeatures, normalized.Feature) {
			return nil, "", hostError("HOST_SESSION_INVALID", "feature observation is not declared by Manifest", nil)
		}
		result[index] = normalized
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Feature < result[right].Feature })
	for index := 1; index < len(result); index++ {
		if result[index-1].Feature == result[index].Feature {
			return nil, "", hostError("HOST_SESSION_INVALID", "duplicate feature observation", nil)
		}
	}
	digest, _, err := canonicaljson.Digest(struct {
		SchemaVersion string               `json:"schema_version"`
		Observations  []FeatureObservation `json:"observations"`
	}{"oaw.host-feature-observations/v1", result})
	if err != nil {
		return nil, "", hostError("HOST_SESSION_INVALID", "feature observations cannot be canonicalized", err)
	}
	return result, digest, nil
}

func normalizeHostActionObservations(manifest Manifest, values []HostActionObservation) ([]HostActionObservation, string, error) {
	result := make([]HostActionObservation, len(values))
	for index, value := range values {
		normalized, err := NewHostActionObservation(value)
		if err != nil {
			return nil, "", hostError("HOST_SESSION_INVALID", "invalid Host action observation", err)
		}
		declared, ok := hostActionByID(manifest.HostActions, normalized.Action.ID)
		if !ok || !reflect.DeepEqual(declared, normalized.Action) {
			return nil, "", hostError("HOST_SESSION_INVALID", "Host action observation is not declared by Manifest", nil)
		}
		result[index] = normalized
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Action.ID < result[right].Action.ID })
	for index := 1; index < len(result); index++ {
		if result[index-1].Action.ID == result[index].Action.ID {
			return nil, "", hostError("HOST_SESSION_INVALID", "duplicate Host action observation", nil)
		}
	}
	digest, _, err := canonicaljson.Digest(struct {
		SchemaVersion string                  `json:"schema_version"`
		Observations  []HostActionObservation `json:"observations"`
	}{"oaw.host-action-observations/v1", result})
	if err != nil {
		return nil, "", hostError("HOST_SESSION_INVALID", "Host action observations cannot be canonicalized", err)
	}
	return result, digest, nil
}

func validAvailabilitySource(state Availability, source ObservationSource) bool {
	switch source {
	case SourceNativeAPI, SourceLiveHostIndex, SourceLiveFilesystem:
		return state == AvailabilityAvailable || state == AvailabilityUnavailable || state == AvailabilityUnknown
	case SourceStaticConfig:
		return state == AvailabilityConfigured || state == AvailabilityUnknown
	default:
		return false
	}
}

func validOpaqueEvidenceReference(value string) bool {
	return validHostText(value, 2048) && strings.HasPrefix(value, "evidence://") && len(value) > len("evidence://")
}

func normalizeHostActionContract(input HostActionContract) (HostActionContract, bool) {
	input = cloneHostActionContract(input)
	sort.Strings(input.MaximumEffects)
	sort.Strings(input.Resources)
	wanted, ok := hostActionByID(canonicalHostActions, input.ID)
	if !ok || !reflect.DeepEqual(input, wanted) {
		return HostActionContract{}, false
	}
	return input, true
}

func hostActionByID(values []HostActionContract, id string) (HostActionContract, bool) {
	for _, value := range values {
		if value.ID == id {
			return cloneHostActionContract(value), true
		}
	}
	return HostActionContract{}, false
}

func cloneHostActionContract(value HostActionContract) HostActionContract {
	value.MaximumEffects = append([]string{}, value.MaximumEffects...)
	value.Resources = append([]string{}, value.Resources...)
	return value
}

func cloneHostActionContracts(values []HostActionContract) []HostActionContract {
	result := make([]HostActionContract, len(values))
	for index, value := range values {
		result[index] = cloneHostActionContract(value)
	}
	return result
}

func cloneFeatureObservations(values []FeatureObservation) []FeatureObservation {
	return append([]FeatureObservation{}, values...)
}

func cloneHostActionObservations(values []HostActionObservation) []HostActionObservation {
	result := make([]HostActionObservation, len(values))
	for index, value := range values {
		result[index] = value
		result[index].Action = cloneHostActionContract(value.Action)
	}
	return result
}
