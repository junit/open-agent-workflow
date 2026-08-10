package host

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

const BindingInventorySchemaV3 = "oaw.host-binding-inventory/v3"

type BindingObservation struct {
	HostID            string                        `json:"host_id"`
	ProviderID        string                        `json:"provider_id"`
	InstallationKey   string                        `json:"installation_key"`
	DistributionID    string                        `json:"distribution_id"`
	BindingID         string                        `json:"binding_id"`
	Surface           string                        `json:"surface"`
	Kind              catalog.BindingKind           `json:"kind"`
	Reference         string                        `json:"reference"`
	Invocation        catalog.InvocationDisposition `json:"invocation"`
	BindingTreeDigest string                        `json:"binding_tree_digest"`
	Topologies        []execution.Topology          `json:"topologies"`
	Source            ObservationSource             `json:"source"`
	EvidenceReference string                        `json:"evidence_reference"`
	Digest            string                        `json:"digest"`
}

type BindingInventory struct {
	SchemaVersion string               `json:"schema_version"`
	HostID        string               `json:"host_id"`
	Observations  []BindingObservation `json:"observations"`
	Digest        string               `json:"digest"`
}

func NewBindingObservation(input BindingObservation) (BindingObservation, error) {
	providedDigest := input.Digest
	input.Digest = ""
	topologies, err := execution.NormalizeTopologies(input.Topologies)
	if err != nil {
		return BindingObservation{}, hostError("HOST_BINDING_OBSERVATION_INVALID", "invalid observed Binding topologies", err)
	}
	input.Topologies = topologies
	if err := validateBindingObservation(input); err != nil {
		return BindingObservation{}, err
	}
	digest, _, err := canonicaljson.Digest(input)
	if err != nil {
		return BindingObservation{}, hostError("HOST_BINDING_OBSERVATION_INVALID", "Binding observation cannot be canonicalized", err)
	}
	if providedDigest != "" && providedDigest != digest {
		return BindingObservation{}, hostError("HOST_BINDING_OBSERVATION_INVALID", "Binding observation digest mismatch", nil)
	}
	input.Digest = digest
	return input, nil
}

func BuildBindingInventoryV3(hostID string, observations []BindingObservation) (BindingInventory, error) {
	if _, err := catalog.ParseLocalID(hostID); err != nil {
		return BindingInventory{}, hostError("HOST_BINDING_INVENTORY_INVALID", "invalid Host ID", err)
	}
	values := make([]BindingObservation, len(observations))
	for index, observation := range observations {
		normalized, err := NewBindingObservation(observation)
		if err != nil {
			return BindingInventory{}, hostError("HOST_BINDING_INVENTORY_INVALID", "invalid Binding observation", err)
		}
		if normalized.HostID != hostID {
			return BindingInventory{}, hostError("HOST_BINDING_INVENTORY_INVALID", "observation Host does not match inventory Host", nil)
		}
		values[index] = normalized
	}
	sort.Slice(values, func(left, right int) bool {
		return bindingObservationSortKey(values[left]) < bindingObservationSortKey(values[right])
	})
	for index := 1; index < len(values); index++ {
		if bindingObservationIdentity(values[index-1]) == bindingObservationIdentity(values[index]) {
			return BindingInventory{}, hostError("HOST_BINDING_INVENTORY_INVALID", "duplicate Host Binding observation", nil)
		}
	}
	record := BindingInventory{SchemaVersion: BindingInventorySchemaV3, HostID: hostID, Observations: values}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return BindingInventory{}, hostError("HOST_BINDING_INVENTORY_INVALID", "inventory cannot be canonicalized", err)
	}
	record.Digest = digest
	return record, nil
}

func ValidateBindingInventory(record BindingInventory) (BindingInventory, error) {
	if record.SchemaVersion != BindingInventorySchemaV3 {
		return BindingInventory{}, hostError("HOST_SCHEMA_UNSUPPORTED", "unsupported Host Binding Inventory schema", nil)
	}
	provided := CloneBindingInventory(record)
	normalized, err := BuildBindingInventoryV3(record.HostID, record.Observations)
	if err != nil {
		return BindingInventory{}, err
	}
	if !reflect.DeepEqual(provided, normalized) {
		return BindingInventory{}, hostError("HOST_BINDING_INVENTORY_INVALID", "Binding Inventory is not canonical", nil)
	}
	return normalized, nil
}

func CloneBindingInventory(value BindingInventory) BindingInventory {
	value.Observations = cloneBindingObservations(value.Observations)
	return value
}

func cloneBindingObservations(values []BindingObservation) []BindingObservation {
	result := make([]BindingObservation, len(values))
	for index, value := range values {
		result[index] = value
		result[index].Topologies = append([]execution.Topology{}, value.Topologies...)
	}
	return result
}

func validateBindingObservation(value BindingObservation) error {
	if _, err := catalog.ParseLocalID(value.HostID); err != nil {
		return hostError("HOST_BINDING_OBSERVATION_INVALID", "invalid Binding Host", err)
	}
	if _, err := catalog.ParseQualifiedID(value.ProviderID); err != nil {
		return hostError("HOST_BINDING_OBSERVATION_INVALID", "invalid Provider ID", err)
	}
	if _, err := catalog.ParseLocalID(value.DistributionID); err != nil {
		return hostError("HOST_BINDING_OBSERVATION_INVALID", "invalid Distribution ID", err)
	}
	if _, err := catalog.ParseLocalID(value.BindingID); err != nil {
		return hostError("HOST_BINDING_OBSERVATION_INVALID", "invalid Binding ID", err)
	}
	if !validHostText(value.InstallationKey, 512) || !validHostText(value.Surface, 128) || strings.ContainsAny(value.Surface, `/\\`) {
		return hostError("HOST_BINDING_OBSERVATION_INVALID", "invalid installation or surface identity", nil)
	}
	if !validBindingKind(value.Kind) || !validBindingInvocation(value.Invocation) || !validHostText(value.Reference, 2048) || filepath.IsAbs(value.Reference) {
		return hostError("HOST_BINDING_OBSERVATION_INVALID", "invalid Host Binding identity", nil)
	}
	if !treeDigestPattern.MatchString(value.BindingTreeDigest) || len(value.Topologies) == 0 {
		return hostError("HOST_BINDING_OBSERVATION_INVALID", "invalid Binding content or topology evidence", nil)
	}
	if value.Source != SourceNativeAPI && value.Source != SourceLiveHostIndex && value.Source != SourceLiveFilesystem {
		return hostError("HOST_BINDING_OBSERVATION_INVALID", "Binding evidence is not live", nil)
	}
	if !validOpaqueEvidenceReference(value.EvidenceReference) {
		return hostError("HOST_BINDING_OBSERVATION_INVALID", "invalid Binding evidence reference", nil)
	}
	return nil
}

func validBindingKind(value catalog.BindingKind) bool {
	return value == catalog.BindingSkill || value == catalog.BindingAgent || value == catalog.BindingRole || value == catalog.BindingInstruction || value == catalog.BindingTool
}

func validBindingInvocation(value catalog.InvocationDisposition) bool {
	return value == catalog.InvocationHumanExplicit || value == catalog.InvocationModel || value == catalog.InvocationHost || value == catalog.InvocationInternal
}

func bindingObservationIdentity(value BindingObservation) string {
	return strings.Join([]string{
		value.HostID, value.ProviderID, value.InstallationKey, value.DistributionID, value.BindingID,
		value.Surface, string(value.Kind), value.Reference, string(value.Invocation),
	}, "\x00")
}

func bindingObservationSortKey(value BindingObservation) string {
	return bindingObservationIdentity(value) + "\x00" + value.BindingTreeDigest + "\x00" + string(value.Source) + "\x00" + value.EvidenceReference + "\x00" + value.Digest
}
