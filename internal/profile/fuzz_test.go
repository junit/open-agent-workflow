package profile_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func FuzzExecutionGraphV4FailsClosed(f *testing.F) {
	seed, err := json.Marshal(compiledGraph(f))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Fuzz(func(t *testing.T, raw []byte) {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		var record profile.ExecutionGraphRecord
		if err := decoder.Decode(&record); err != nil {
			return
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			return
		}
		err := profile.ValidateExecutionGraphRecord(record)
		if bytes.Equal(raw, seed) {
			if err != nil {
				t.Fatalf("valid seed rejected: %v", err)
			}
			return
		}
		if err != nil && !strings.Contains(err.Error(), "PROFILE_GRAPH_RECORD_INVALID") {
			t.Fatalf("unexpected validation error: %v", err)
		}
	})
}
