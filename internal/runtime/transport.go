package runtime

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

func DecodeFrame(raw []byte) (RunFrame, error) {
	if len(raw) == 0 || len(raw) > MaximumProtocolFrameBytes {
		return RunFrame{}, runtimeError("RUNTIME_FRAME_DECODE_INVALID", "Runtime frame size is invalid", nil)
	}
	if !utf8.Valid(raw) {
		return RunFrame{}, runtimeError("RUNTIME_FRAME_DECODE_INVALID", "Runtime frame is not valid UTF-8", nil)
	}
	if err := rejectDuplicateJSONFields(raw); err != nil {
		return RunFrame{}, runtimeError("RUNTIME_FRAME_DECODE_INVALID", "Runtime frame contains duplicate or invalid JSON fields", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var frame RunFrame
	if err := decoder.Decode(&frame); err != nil {
		return RunFrame{}, runtimeError("RUNTIME_FRAME_DECODE_INVALID", "Runtime frame JSON is invalid", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("trailing JSON value")
		}
		return RunFrame{}, runtimeError("RUNTIME_FRAME_DECODE_INVALID", "Runtime frame has trailing JSON", err)
	}
	if err := validateCommonFrame(frame); err != nil {
		return RunFrame{}, err
	}
	return cloneRunFrame(frame), nil
}

func EncodeReply(reply RunReply) ([]byte, error) {
	encoded, err := canonicaljson.Marshal(reply)
	if err != nil {
		return nil, runtimeError("RUNTIME_REPLY_ENCODE_FAILED", "Runtime reply cannot be canonicalized", err)
	}
	return encoded, nil
}

func ExchangeJSON(input io.Reader, output io.Writer, engine *Engine) error {
	if input == nil || output == nil || engine == nil {
		return runtimeError("RUNTIME_TRANSPORT_INVALID", "Runtime transport requires input, output, and Engine", nil)
	}
	raw, err := io.ReadAll(io.LimitReader(input, MaximumProtocolFrameBytes+1))
	if err != nil {
		return runtimeError("RUNTIME_FRAME_READ_FAILED", "read Runtime frame", err)
	}
	frame, err := DecodeFrame(raw)
	if err != nil {
		return err
	}
	reply, err := engine.Exchange(frame)
	if err != nil {
		return err
	}
	encoded, err := EncodeReply(reply)
	if err != nil {
		return err
	}
	if _, err := output.Write(encoded); err != nil {
		return runtimeError("RUNTIME_REPLY_WRITE_FAILED", "write Runtime reply", err)
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

func cloneRunFrame(value RunFrame) RunFrame {
	if value.Start != nil {
		start := *value.Start
		start.Proposal = cloneProposal(value.Start.Proposal)
		start.Bounded = cloneBoundedInput(value.Start.Bounded)
		start.Workflow = cloneWorkflowInput(value.Start.Workflow)
		value.Start = &start
	}
	return value
}
