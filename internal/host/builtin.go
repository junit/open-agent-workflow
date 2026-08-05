package host

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
)

const (
	HostIntegrationSetSchemaV2 = "oaw.host-integration-set/v2"
)

type IntegrationSetRecord struct {
	SchemaVersion string              `json:"schema_version"`
	Integrations  []IntegrationRecord `json:"integrations"`
}

func LoadBuiltinIntegrations(files fs.FS) ([]IntegrationRecord, error) {
	raw, err := fs.ReadFile(files, "host-integrations.json")
	if err != nil {
		return nil, fmt.Errorf("BUILTIN_HOST_INTEGRATIONS_READ_FAILED: %w", err)
	}
	set, err := DecodeIntegrationSetJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("BUILTIN_HOST_INTEGRATIONS_INVALID: %w", err)
	}
	return cloneIntegrations(set.Integrations), nil
}

func DecodeIntegrationSetJSON(raw []byte) (IntegrationSetRecord, error) {
	if err := validateEncodedIntegration(raw); err != nil {
		return IntegrationSetRecord{}, err
	}
	var set IntegrationSetRecord
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&set); err != nil {
		return IntegrationSetRecord{}, hostError("HOST_INTEGRATION_SET_INVALID", "invalid Integration Set JSON", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return IntegrationSetRecord{}, hostError("HOST_INTEGRATION_SET_INVALID", "trailing Integration Set JSON", err)
	}
	if set.SchemaVersion != HostIntegrationSetSchemaV2 {
		return IntegrationSetRecord{}, hostError("HOST_SCHEMA_UNSUPPORTED", "unsupported Host Integration Set schema", nil)
	}
	if len(set.Integrations) == 0 {
		return IntegrationSetRecord{}, hostError("HOST_INTEGRATION_SET_INVALID", "invalid Integration Set identity", nil)
	}
	set.Integrations = cloneIntegrations(set.Integrations)
	sort.Slice(set.Integrations, func(left, right int) bool { return set.Integrations[left].ID < set.Integrations[right].ID })
	for index, record := range set.Integrations {
		if index > 0 && set.Integrations[index-1].ID == record.ID {
			return IntegrationSetRecord{}, hostError("HOST_INTEGRATION_SET_INVALID", "duplicate Integration ID", nil)
		}
		if err := ValidateIntegrationRecord(record); err != nil {
			return IntegrationSetRecord{}, hostError("HOST_INTEGRATION_SET_INVALID", "invalid Integration record", err)
		}
	}
	return set, nil
}

func cloneIntegrations(values []IntegrationRecord) []IntegrationRecord {
	result := make([]IntegrationRecord, len(values))
	for index, value := range values {
		result[index] = CloneIntegration(value)
	}
	return result
}
