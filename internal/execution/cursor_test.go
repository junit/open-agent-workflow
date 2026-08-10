package execution_test

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func TestGraphCursorUsesClosedKindsAndNonZeroOrdinal(t *testing.T) {
	for _, kind := range []execution.CursorKind{execution.CursorBinding, execution.CursorHostAction, execution.CursorGate, execution.CursorTerminal} {
		value, err := execution.NewGraphCursor("implementation", kind, "unit", 1)
		if err != nil || value.Kind != kind {
			t.Fatalf("NewGraphCursor(%q) = %#v, %v", kind, value, err)
		}
	}
	for _, value := range []execution.GraphCursor{
		{SlotID: "implementation", Kind: "other", UnitID: "unit", Ordinal: 1},
		{SlotID: "", Kind: execution.CursorBinding, UnitID: "unit", Ordinal: 1},
		{SlotID: "implementation", Kind: execution.CursorBinding, UnitID: "unit", Ordinal: 0},
		{SlotID: "implementation", Kind: execution.CursorBinding, UnitID: "bad\nunit", Ordinal: 1},
	} {
		if err := execution.ValidateGraphCursor(value); err == nil {
			t.Fatalf("ValidateGraphCursor(%#v) accepted invalid cursor", value)
		}
	}
}
