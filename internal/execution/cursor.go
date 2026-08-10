package execution

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type CursorKind string

const (
	CursorBinding    CursorKind = "binding"
	CursorHostAction CursorKind = "host-action"
	CursorGate       CursorKind = "gate"
	CursorTerminal   CursorKind = "terminal"
)

type GraphCursor struct {
	SlotID  string     `json:"slot_id"`
	Kind    CursorKind `json:"kind"`
	UnitID  string     `json:"unit_id"`
	Ordinal uint64     `json:"ordinal"`
}

func NewGraphCursor(slotID string, kind CursorKind, unitID string, ordinal uint64) (GraphCursor, error) {
	value := GraphCursor{SlotID: slotID, Kind: kind, UnitID: unitID, Ordinal: ordinal}
	if err := ValidateGraphCursor(value); err != nil {
		return GraphCursor{}, err
	}
	return value, nil
}

func ValidateGraphCursor(value GraphCursor) error {
	if value.Kind != CursorBinding && value.Kind != CursorHostAction && value.Kind != CursorGate && value.Kind != CursorTerminal {
		return fmt.Errorf("GRAPH_CURSOR_INVALID: unsupported kind %q", value.Kind)
	}
	if !validCursorText(value.SlotID) || !validCursorText(value.UnitID) || value.Ordinal == 0 {
		return fmt.Errorf("GRAPH_CURSOR_INVALID: invalid cursor identity")
	}
	return nil
}

func validCursorText(value string) bool {
	return value != "" && utf8.ValidString(value) && strings.TrimSpace(value) == value &&
		utf8.RuneCountInString(value) <= 512 && strings.IndexFunc(value, unicode.IsControl) < 0
}
