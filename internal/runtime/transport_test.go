package runtime_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func TestDecodeFrameRejectsUnknownTrailingDuplicateAndOversizedJSON(t *testing.T) {
	valid := `{"schema_version":"oaw.runtime/v1","kind":"INSPECT","message_id":"m","idempotency_key":"k","run_id":"run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	for _, test := range []struct {
		name string
		raw  string
	}{
		{name: "unknown field", raw: strings.Replace(valid, "}", ",\"unknown\":true}", 1)},
		{name: "trailing value", raw: valid + " {}"},
		{name: "duplicate field", raw: strings.Replace(valid, `"kind":"INSPECT"`, `"kind":"INSPECT","kind":"START"`, 1)},
		{name: "invalid utf8", raw: string([]byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'})},
		{name: "oversized", raw: `{"schema_version":"oaw.runtime/v1","kind":"INSPECT","message_id":"m","idempotency_key":"k","run_id":"run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","padding":"` + strings.Repeat("x", 1<<20) + `"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := runtime.DecodeFrame([]byte(test.raw)); err == nil {
				t.Fatalf("DecodeFrame(%s) unexpectedly succeeded", test.name)
			}
		})
	}
}

func TestEncodeReplyIsCanonicalJSON(t *testing.T) {
	reply := runtime.RunReply{
		SchemaVersion:   runtime.RuntimeSchemaV1,
		Kind:            runtime.ReplyStateSnapshot,
		RunID:           "run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Diagnostics:     []runtime.Diagnostic{},
		RecoveryActions: []string{},
	}
	encoded, err := runtime.EncodeReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := canonicaljson.Marshal(reply)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, expected) || !json.Valid(encoded) {
		t.Fatalf("encoded reply = %s, want canonical %s", encoded, expected)
	}
}

func TestExchangeJSONUsesEngineAndEmitsOnlyCanonicalReply(t *testing.T) {
	stateRoot, engine, started := startDirectRun(t)
	_ = stateRoot
	frame := runtime.RunFrame{
		SchemaVersion:  runtime.RuntimeSchemaV1,
		Kind:           runtime.FrameInspect,
		MessageID:      "transport-inspect",
		IdempotencyKey: "transport-inspect",
		RunID:          started.RunID,
	}
	input, err := canonicaljson.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := runtime.ExchangeJSON(strings.NewReader(string(input)), &stdout, engine); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("transport wrote diagnostics to stdout-equivalent sink: %q", stderr.String())
	}
	decoded := runtime.RunReply{}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("reply JSON = %s: %v", stdout.Bytes(), err)
	}
	canonical, err := runtime.EncodeReply(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), canonical) || decoded.Kind != runtime.ReplyStateSnapshot {
		t.Fatalf("transport reply = %s", stdout.Bytes())
	}
}
