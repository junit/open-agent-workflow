package codex

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestNormalizeCodexJSONLSuccess(t *testing.T) {
	request := testDispatchRequest()
	result, err := normalizeJSONL(request, []byte(`{"type":"thread.started","id":"thread-1"}
{"type":"turn.started"}
{"type":"turn.completed","id":"turn-1"}
`), 16)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != host.DispatchSucceeded || result.ExecutionID == "" || len(result.Evidence) != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestNormalizeCodexJSONLRejectsMalformedUnknownAndOversizedEventStreams(t *testing.T) {
	request := testDispatchRequest()
	for _, test := range []struct {
		name string
		raw  []byte
		max  int
	}{
		{name: "malformed", raw: []byte(`{"type":`), max: 16},
		{name: "unknown only", raw: []byte(`{"type":"thread.started"}`), max: 16},
		{name: "too many", raw: []byte("{\"type\":\"turn.started\"}\n{\"type\":\"turn.started\"}\n"), max: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := normalizeJSONL(request, test.raw, test.max); err == nil {
				t.Fatal("normalizeJSONL unexpectedly succeeded")
			}
		})
	}
}
