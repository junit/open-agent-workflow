package runtime_test

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func FuzzDecodeFrameFailsClosed(f *testing.F) {
	f.Add([]byte(`{"schema_version":"oaw.runtime/v1","kind":"INSPECT","message_id":"inspect","idempotency_key":"inspect","run_id":"run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`))
	f.Add([]byte(`{"schema_version":"oaw.runtime/v1","kind":"INSPECT","unknown":true}`))
	f.Add([]byte{0xff, 0xfe})

	f.Fuzz(func(t *testing.T, raw []byte) {
		frame, err := runtime.DecodeFrame(raw)
		if err != nil {
			return
		}
		if frame.SchemaVersion != runtime.RuntimeSchemaV1 {
			t.Fatalf("accepted frame with schema version %q", frame.SchemaVersion)
		}
		switch frame.Kind {
		case runtime.FrameStart, runtime.FrameContinue, runtime.FrameInspect:
		default:
			t.Fatalf("accepted frame kind %q", frame.Kind)
		}
	})
}
