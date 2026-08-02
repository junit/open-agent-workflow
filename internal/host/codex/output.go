package codex

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

type normalizedEvent struct {
	Type     string `json:"type"`
	ID       string `json:"id,omitempty"`
	Status   string `json:"status,omitempty"`
	ItemType string `json:"item_type,omitempty"`
}

func normalizeJSONL(request host.DispatchRequest, raw []byte, maxEvents int) (host.DispatchResult, error) {
	if maxEvents <= 0 {
		return host.DispatchResult{}, errors.New("CODEX_OUTPUT_INVALID: event limit is invalid")
	}
	lines := bytes.Split(raw, []byte{'\n'})
	events := make([]normalizedEvent, 0, len(lines))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if len(events) >= maxEvents {
			return host.DispatchResult{}, errors.New("CODEX_OUTPUT_INVALID: event limit exceeded")
		}
		var value struct {
			Type   string `json:"type"`
			ID     string `json:"id"`
			Status string `json:"status"`
			Item   *struct {
				Type string `json:"type"`
			} `json:"item"`
		}
		decoder := json.NewDecoder(bytes.NewReader(line))
		if err := decoder.Decode(&value); err != nil {
			return host.DispatchResult{}, fmt.Errorf("CODEX_OUTPUT_INVALID: malformed JSONL event: %w", err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err == nil {
			return host.DispatchResult{}, errors.New("CODEX_OUTPUT_INVALID: event has trailing JSON")
		} else if !errors.Is(err, io.EOF) {
			return host.DispatchResult{}, fmt.Errorf("CODEX_OUTPUT_INVALID: event has invalid trailing JSON: %w", err)
		}
		event := normalizedEvent{Type: strings.TrimSpace(value.Type), ID: strings.TrimSpace(value.ID), Status: strings.TrimSpace(value.Status)}
		if value.Item != nil {
			event.ItemType = strings.TrimSpace(value.Item.Type)
		}
		if event.Type == "" || len(event.Type) > 128 || strings.ContainsAny(event.Type, "\r\n\x00") {
			return host.DispatchResult{}, errors.New("CODEX_OUTPUT_INVALID: event type is invalid")
		}
		events = append(events, event)
	}
	if len(events) == 0 {
		return host.DispatchResult{}, errors.New("CODEX_OUTPUT_INVALID: no events returned")
	}
	status := host.DispatchFailed
	executionID := ""
	terminal := false
	for _, event := range events {
		if event.ID != "" && executionID == "" {
			executionID = event.ID
		}
		switch event.Type {
		case "error", "turn.failed", "result.failed":
			terminal = true
		case "turn.completed", "result.completed":
			terminal = true
			status = host.DispatchSucceeded
		case "result":
			if event.Status == "completed" || event.Status == "succeeded" {
				terminal = true
				status = host.DispatchSucceeded
			}
		}
	}
	if !terminal {
		return host.DispatchResult{}, errors.New("CODEX_OUTPUT_INVALID: no terminal Codex event")
	}
	digest, _, err := canonicaljson.Digest(events)
	if err != nil {
		return host.DispatchResult{}, fmt.Errorf("CODEX_OUTPUT_INVALID: digest events: %w", err)
	}
	if executionID == "" {
		executionID = "codex-exec-" + digest[:16]
	}
	return host.NormalizeDispatchResult(request, host.DispatchResult{
		GrantID: request.GrantID, InvocationID: request.InvocationID, ExecutorID: request.ExecutorID,
		ExecutionID: executionID, Outcome: status,
		Evidence: []host.DispatchEvidence{{Reference: "evidence://codex/" + request.InvocationID, Digest: digest}},
	})
}
