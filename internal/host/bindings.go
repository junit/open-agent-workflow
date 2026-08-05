package host

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

const BindingInventorySchemaV2 = "oaw.host-binding-inventory/v2"

type BindingObservation struct {
	HostID            string               `json:"host_id"`
	InstallationKey   string               `json:"installation_key"`
	Binding           catalog.HostBinding  `json:"binding"`
	Topologies        []execution.Topology `json:"topologies"`
	Source            string               `json:"source"`
	EvidenceReference string               `json:"evidence_reference"`
	Digest            string               `json:"digest"`
}

type BindingInventory struct {
	SchemaVersion string               `json:"schema_version"`
	HostID        string               `json:"host_id"`
	Observations  []BindingObservation `json:"observations"`
	Digest        string               `json:"digest"`
}

func NewBindingInventory(hostID string, observations []BindingObservation) (BindingInventory, error) {
	if _, err := catalog.ParseLocalID(hostID); err != nil {
		return BindingInventory{}, hostError("HOST_BINDING_INVENTORY_INVALID", "invalid Host ID", err)
	}
	values := make([]BindingObservation, len(observations))
	for index, observation := range observations {
		normalized, err := normalizeBindingObservation(hostID, observation)
		if err != nil {
			return BindingInventory{}, err
		}
		values[index] = normalized
	}
	sort.Slice(values, func(i, j int) bool {
		return bindingObservationSortKey(values[i]) < bindingObservationSortKey(values[j])
	})
	for i := 1; i < len(values); i++ {
		if bindingObservationIdentity(values[i-1]) == bindingObservationIdentity(values[i]) {
			return BindingInventory{}, hostError("HOST_BINDING_INVENTORY_INVALID", "duplicate Host Installation Binding observation", nil)
		}
	}
	record := struct {
		SchemaVersion string               `json:"schema_version"`
		HostID        string               `json:"host_id"`
		Observations  []BindingObservation `json:"observations"`
	}{BindingInventorySchemaV2, hostID, values}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return BindingInventory{}, hostError("HOST_BINDING_INVENTORY_INVALID", "inventory cannot be canonicalized", err)
	}
	return BindingInventory{SchemaVersion: BindingInventorySchemaV2, HostID: hostID, Observations: values, Digest: digest}, nil
}

func CloneBindingInventory(value BindingInventory) BindingInventory {
	value.Observations = cloneBindingObservations(value.Observations)
	return value
}

func cloneBindingObservations(values []BindingObservation) []BindingObservation {
	result := make([]BindingObservation, len(values))
	for index, value := range values {
		result[index] = value
		result[index].Binding.Topologies = append([]execution.Topology{}, value.Binding.Topologies...)
		result[index].Topologies = append([]execution.Topology{}, value.Topologies...)
	}
	return result
}

func normalizeBindingObservation(hostID string, value BindingObservation) (BindingObservation, error) {
	declared, err := execution.NormalizeTopologies(value.Binding.Topologies)
	if err != nil {
		return BindingObservation{}, hostError("HOST_BINDING_INVENTORY_INVALID", "invalid declared Binding topologies", err)
	}
	observed, err := execution.NormalizeTopologies(value.Topologies)
	if err != nil {
		return BindingObservation{}, hostError("HOST_BINDING_INVENTORY_INVALID", "invalid observed Binding topologies", err)
	}
	for _, topology := range observed {
		if !slices.Contains(declared, topology) {
			return BindingObservation{}, hostError("HOST_BINDING_INVENTORY_INVALID", "observed topology is not declared by the Binding", nil)
		}
	}
	value.Binding.Topologies = declared
	value.Topologies = observed
	if err := validateBindingObservation(hostID, value); err != nil {
		return BindingObservation{}, err
	}
	return value, nil
}

func validateBindingObservation(hostID string, value BindingObservation) error {
	if value.HostID != hostID || value.Binding.Host != hostID {
		return hostError("HOST_BINDING_INVENTORY_INVALID", "observation Host does not match inventory Host", nil)
	}
	if _, err := catalog.ParseLocalID(value.Binding.Host); err != nil {
		return hostError("HOST_BINDING_INVENTORY_INVALID", "invalid Binding Host", err)
	}
	if value.InstallationKey == "" || hasBindingControl(value.InstallationKey) {
		return hostError("HOST_BINDING_INVENTORY_INVALID", "invalid Installation Key", nil)
	}
	if value.Binding.Reference == "" || hasBindingControl(value.Binding.Reference) || (value.Binding.Kind != "skill" && value.Binding.Kind != "agent" && value.Binding.Kind != "tool") {
		return hostError("HOST_BINDING_INVENTORY_INVALID", "invalid Host Binding", nil)
	}
	if value.Source != "host-index" && value.Source != "host-filesystem" && value.Source != "native-probe" {
		return hostError("HOST_BINDING_INVENTORY_INVALID", fmt.Sprintf("unsupported observation source %q", value.Source), nil)
	}
	if !validEvidenceReference(value.Source, value.EvidenceReference) {
		return hostError("HOST_BINDING_INVENTORY_INVALID", "invalid evidence reference", nil)
	}
	if !validBindingDigest(value.Digest) {
		return hostError("HOST_BINDING_INVENTORY_INVALID", "invalid evidence digest", nil)
	}
	return nil
}

func validEvidenceReference(source, value string) bool {
	if value == "" || hasBindingControl(value) {
		return false
	}
	if source == "native-probe" {
		return strings.HasPrefix(value, "evidence://")
	}
	return filepath.IsAbs(value) && filepath.Clean(value) == value
}

func validBindingDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func bindingObservationIdentity(value BindingObservation) string {
	return value.HostID + "\x00" + value.InstallationKey + "\x00" + value.Binding.Host + "\x00" + value.Binding.Kind + "\x00" + value.Binding.Reference
}

func bindingObservationSortKey(value BindingObservation) string {
	return bindingObservationIdentity(value) + "\x00" + value.Source + "\x00" + value.EvidenceReference + "\x00" + value.Digest
}

func hasBindingControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}
