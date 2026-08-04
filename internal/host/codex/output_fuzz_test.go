package codex

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func FuzzNormalizeJSONLFailsClosed(f *testing.F) {
	f.Add([]byte(`{"type":"turn.completed","id":"turn-1"}` + "\n"))
	f.Add([]byte(`{"type":"turn.started"}` + "\n"))
	f.Add([]byte(`not-json` + "\n"))

	request := testDispatchRequest()
	f.Fuzz(func(t *testing.T, raw []byte) {
		result, err := normalizeJSONL(request, raw, 16)
		if err != nil {
			return
		}
		if _, err := host.NormalizeDispatchResult(request, result); err != nil {
			t.Fatalf("normalizer returned an invalid result: %v", err)
		}
	})
}
