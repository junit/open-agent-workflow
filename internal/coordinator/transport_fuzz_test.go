package coordinator_test

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
)

func FuzzDecodeCommandFailsClosed(f *testing.F) {
	f.Add(mustMarshal(f, validStartCommand(f)))
	f.Add([]byte(`{"schema_version":"oaw.runtime/v1","kind":"START"}`))
	f.Add([]byte{0xff, 0xfe})
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > coordinator.MaximumProtocolFrameBytes+1 {
			t.Skip()
		}
		first, firstErr := coordinator.DecodeCommand(raw)
		second, secondErr := coordinator.DecodeCommand(raw)
		if (firstErr == nil) != (secondErr == nil) || firstErr != nil && firstErr.Error() != secondErr.Error() {
			t.Fatalf("DecodeCommand() is nondeterministic: %#v/%v %#v/%v", first, firstErr, second, secondErr)
		}
		if firstErr == nil && first.SchemaVersion != coordinator.WorkflowCommandSchemaV1 {
			t.Fatalf("accepted unexpected schema %q", first.SchemaVersion)
		}
	})
}
