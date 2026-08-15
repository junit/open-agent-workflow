package codexbridge

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/wifibaby4u/open-agent-workflow/internal/assurance"
)

type ObserveProfileInput struct {
	Profile string `json:"profile"`
}

type ObserveProfileOutput struct {
	Overlay     assurance.Overlay `json:"overlay"`
	Diagnostics []Diagnostic      `json:"diagnostics"`
}

func DecodeObserveProfileInput(raw []byte) (ObserveProfileInput, error) {
	return decodePublicInput[ObserveProfileInput](raw)
}

func decodePublicInput[T any](raw []byte) (T, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var input T
	if err := decoder.Decode(&input); err != nil {
		return input, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "unknown or malformed public field", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return input, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "trailing JSON value", err)
	}
	return input, nil
}
