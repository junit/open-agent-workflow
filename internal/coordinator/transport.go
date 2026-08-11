package coordinator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

const MaximumProtocolFrameBytes = 1 << 20

func DecodeCommand(raw []byte) (Command, error) {
	if len(raw) == 0 || len(raw) > MaximumProtocolFrameBytes {
		return Command{}, coordinatorError("WORKFLOW_COMMAND_DECODE_INVALID", "Workflow Command size is invalid", nil)
	}
	if !utf8.Valid(raw) {
		return Command{}, coordinatorError("WORKFLOW_COMMAND_DECODE_INVALID", "Workflow Command is not valid UTF-8", nil)
	}
	if err := rejectDuplicateJSONFields(raw); err != nil {
		return Command{}, coordinatorError("WORKFLOW_COMMAND_DECODE_INVALID", "Workflow Command contains duplicate or invalid JSON fields", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var command Command
	if err := decoder.Decode(&command); err != nil {
		return Command{}, coordinatorError("WORKFLOW_COMMAND_DECODE_INVALID", "Workflow Command JSON is invalid", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("trailing JSON value")
		}
		return Command{}, coordinatorError("WORKFLOW_COMMAND_DECODE_INVALID", "Workflow Command has trailing JSON", err)
	}
	if command.SchemaVersion != WorkflowCommandSchemaV2 {
		return Command{}, coordinatorError("SCHEMA_UNSUPPORTED", "unsupported Workflow Command schema", nil)
	}
	command, err := normalizeCommand(command)
	if err != nil {
		return Command{}, err
	}
	if err := validateCommand(command); err != nil {
		return Command{}, err
	}
	return command, nil
}

func EncodeResult(result Result) ([]byte, error) {
	normalized, err := normalizeResult(result)
	if err != nil {
		return nil, err
	}
	encoded, err := canonicaljson.Marshal(normalized)
	if err != nil {
		return nil, coordinatorError("WORKFLOW_RESULT_ENCODE_FAILED", "Workflow Result cannot be encoded", err)
	}
	return encoded, nil
}

func ExchangeJSON(input io.Reader, output io.Writer, engine *Engine) error {
	if input == nil || output == nil || engine == nil {
		return coordinatorError("WORKFLOW_TRANSPORT_INVALID", "Workflow transport requires input, output, and Engine", nil)
	}
	raw, err := io.ReadAll(io.LimitReader(input, MaximumProtocolFrameBytes+1))
	if err != nil {
		return coordinatorError("WORKFLOW_COMMAND_READ_FAILED", "read Workflow Command", err)
	}
	command, err := DecodeCommand(raw)
	if err != nil {
		return err
	}
	result, err := engine.Exchange(command)
	if err != nil {
		return err
	}
	encoded, err := EncodeResult(result)
	if err != nil {
		return err
	}
	if _, err := output.Write(encoded); err != nil {
		return coordinatorError("WORKFLOW_RESULT_WRITE_FAILED", "write Workflow Result", err)
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
