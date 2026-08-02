package host

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
)

const maximumIntegrationBytes = 1 << 20

func DecodeIntegrationJSON(raw []byte) (IntegrationRecord, error) {
	if err := validateEncodedIntegration(raw); err != nil {
		return IntegrationRecord{}, err
	}
	var record IntegrationRecord
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return IntegrationRecord{}, hostError("HOST_INTEGRATION_DECODE_INVALID", "invalid Integration JSON", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("trailing JSON value")
		}
		return IntegrationRecord{}, hostError("HOST_INTEGRATION_DECODE_INVALID", "invalid trailing Integration JSON", err)
	}
	validated, err := NewIntegration(record)
	if err != nil {
		return IntegrationRecord{}, hostError("HOST_INTEGRATION_DECODE_INVALID", "invalid Integration record", err)
	}
	return validated, nil
}

func DecodeIntegrationTOML(raw []byte) (IntegrationRecord, error) {
	if err := validateEncodedIntegration(raw); err != nil {
		return IntegrationRecord{}, err
	}
	var record IntegrationRecord
	metadata, err := toml.Decode(string(raw), &record)
	if err != nil {
		return IntegrationRecord{}, hostError("HOST_INTEGRATION_DECODE_INVALID", "invalid Integration TOML", err)
	}
	if unknown := metadata.Undecoded(); len(unknown) != 0 {
		return IntegrationRecord{}, hostError("HOST_INTEGRATION_DECODE_INVALID", fmt.Sprintf("unknown Integration field %s", unknown[0]), nil)
	}
	validated, err := NewIntegration(record)
	if err != nil {
		return IntegrationRecord{}, hostError("HOST_INTEGRATION_DECODE_INVALID", "invalid Integration record", err)
	}
	return validated, nil
}

func validateEncodedIntegration(raw []byte) error {
	if len(raw) > maximumIntegrationBytes {
		return hostError("HOST_INTEGRATION_DECODE_INVALID", "Integration record is too large", nil)
	}
	if !utf8.Valid(raw) {
		return hostError("HOST_INTEGRATION_DECODE_INVALID", "Integration record is not valid UTF-8", nil)
	}
	return nil
}
