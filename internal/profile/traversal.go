package profile

import (
	"fmt"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func assignTraversal(slots []CompiledSlot) {
	for slotIndex := range slots {
		slot := &slots[slotIndex]
		ordinal := uint64(1)
		slot.Traversal = []execution.GraphCursor{}
		for unitIndex := range slot.Pipeline {
			cursor, _ := execution.NewGraphCursor(string(slot.Pipeline[unitIndex].AnchorSlotID), execution.CursorBinding, slot.Pipeline[unitIndex].UnitID, ordinal)
			slot.Pipeline[unitIndex].Cursor = cursor
			ordinal++
			slot.Traversal = append(slot.Traversal, cursor)
		}
		if slot.HostAction != nil {
			cursor, _ := execution.NewGraphCursor(string(slot.SlotID), execution.CursorHostAction, slot.HostAction.ID, ordinal)
			slot.HostAction.Cursor = cursor
			slot.Traversal = append(slot.Traversal, cursor)
			ordinal++
		}
		for gateIndex := range slot.Gates {
			cursor, _ := execution.NewGraphCursor(string(slot.SlotID), execution.CursorGate, slot.Gates[gateIndex].ID, ordinal)
			slot.Gates[gateIndex].Cursor = cursor
			slot.Traversal = append(slot.Traversal, cursor)
			ordinal++
		}
		if slot.Terminal {
			cursor, _ := execution.NewGraphCursor(string(slot.SlotID), execution.CursorTerminal, "terminal:"+string(slot.SlotID), ordinal)
			slot.Traversal = append(slot.Traversal, cursor)
		}
	}
}

func materializeIncidentRoutes(slots []CompiledSlot, pending []pendingIncidentRoute) []CompiledIncidentRoute {
	cursors := make(map[string]execution.GraphCursor)
	for _, slot := range slots {
		for _, unit := range slot.Pipeline {
			cursors[unit.UnitID] = unit.Cursor
		}
	}
	result := make([]CompiledIncidentRoute, len(pending))
	for index, route := range pending {
		pipeline := make([]execution.GraphCursor, 0, len(route.unitIDs))
		for _, unitID := range route.unitIDs {
			if cursor, found := cursors[unitID]; found {
				pipeline = append(pipeline, cursor)
			}
		}
		result[index] = CompiledIncidentRoute{
			IncidentType: route.record.IncidentType, HandlerSlotID: catalog.SlotIncidentRecovery,
			HandlerPipeline: pipeline, ReturnTo: route.record.ReturnTo, IfUnavailable: route.record.IfUnavailable,
		}
	}
	return result
}

func ValidateGraphCursor(record ExecutionGraphRecord, cursor execution.GraphCursor) error {
	if err := execution.ValidateGraphCursor(cursor); err != nil {
		return fmt.Errorf("PROFILE_GRAPH_CURSOR_INVALID: %w", err)
	}
	if err := ValidateExecutionGraphRecord(record); err != nil {
		return err
	}
	matches := 0
	for _, slot := range record.Slots {
		for _, value := range slot.Traversal {
			if value == cursor {
				matches++
			}
		}
	}
	if matches != 1 {
		return fmt.Errorf("PROFILE_GRAPH_CURSOR_INVALID: cursor does not identify exactly one graph unit")
	}
	return nil
}

func FirstActionableCursor(record ExecutionGraphRecord) (execution.GraphCursor, error) {
	if err := ValidateExecutionGraphRecord(record); err != nil {
		return execution.GraphCursor{}, err
	}
	slot, found := compiledSlotByID(record.Slots, record.EntrySlotID)
	if !found {
		return execution.GraphCursor{}, fmt.Errorf("PROFILE_GRAPH_CURSOR_INVALID: graph entry has no traversal")
	}
	if cursor, found := firstActionableIn(record, slot.Traversal); found {
		return cursor, nil
	}
	result, err := traversalAtSlot(record, slot.SlotID)
	if err == nil && result.Cursor != nil {
		return *result.Cursor, nil
	}
	return execution.GraphCursor{}, fmt.Errorf("PROFILE_GRAPH_CURSOR_INVALID: graph entry has no actionable unit")
}

func UnitAtCursor(record ExecutionGraphRecord, cursor execution.GraphCursor) (TraversalUnit, error) {
	if err := ValidateGraphCursor(record, cursor); err != nil {
		return TraversalUnit{}, err
	}
	for _, slot := range record.Slots {
		for _, binding := range slot.Pipeline {
			if binding.Cursor == cursor {
				value := cloneResolvedBinding(binding)
				return TraversalUnit{Cursor: cursor, ProviderBinding: &value}, nil
			}
		}
		if slot.HostAction != nil && slot.HostAction.Cursor == cursor {
			value := cloneCompiledHostAction(*slot.HostAction)
			return TraversalUnit{Cursor: cursor, HostAction: &value}, nil
		}
		for _, gate := range slot.Gates {
			if gate.Cursor == cursor {
				value := cloneCompiledGate(gate)
				return TraversalUnit{Cursor: cursor, Gate: &value}, nil
			}
		}
		if cursor.Kind == execution.CursorTerminal && cursor.SlotID == string(slot.SlotID) {
			return TraversalUnit{Cursor: cursor, Terminal: true}, nil
		}
	}
	return TraversalUnit{}, fmt.Errorf("PROFILE_GRAPH_CURSOR_INVALID: graph unit was not found")
}

func NextActionableCursor(record ExecutionGraphRecord, cursor execution.GraphCursor, signal, incidentType string) (TraversalResult, error) {
	if signal != "" && incidentType != "" {
		return TraversalResult{}, fmt.Errorf("PROFILE_GRAPH_TRANSITION_INVALID: signal and incident cannot be supplied together")
	}
	unit, err := UnitAtCursor(record, cursor)
	if err != nil {
		return TraversalResult{}, err
	}
	if unit.Terminal {
		return TraversalResult{Disposition: TraversalTerminal}, nil
	}
	if incidentType != "" {
		for _, route := range record.IncidentRoutes {
			if route.IncidentType != incidentType {
				continue
			}
			if next, found := firstActionableIn(record, route.HandlerPipeline); found {
				return TraversalResult{Disposition: TraversalNext, Cursor: &next}, nil
			}
			if route.IfUnavailable == catalog.IncidentStop {
				return TraversalResult{Disposition: TraversalStop}, nil
			}
			if route.IfUnavailable == catalog.IncidentReplan {
				return TraversalResult{Disposition: TraversalReplan}, nil
			}
		}
		return TraversalResult{}, fmt.Errorf("PROFILE_GRAPH_TRANSITION_INVALID: incident route is not declared")
	}
	for _, route := range record.IncidentRoutes {
		for index, handlerCursor := range route.HandlerPipeline {
			if handlerCursor != cursor {
				continue
			}
			if signal != "" && index+1 < len(route.HandlerPipeline) {
				return TraversalResult{}, fmt.Errorf("PROFILE_GRAPH_TRANSITION_INVALID: signal supplied before incident handler completion")
			}
			if next, found := nextActionableIn(record, route.HandlerPipeline, index+1); found {
				return TraversalResult{Disposition: TraversalNext, Cursor: &next}, nil
			}
			if signal != "" && signal != "succeeded" {
				return TraversalResult{}, fmt.Errorf("PROFILE_GRAPH_TRANSITION_INVALID: incident handler signal is not declared")
			}
			return traversalAtSlot(record, route.ReturnTo)
		}
	}
	slot, found := compiledSlotByID(record.Slots, catalog.SlotID(cursor.SlotID))
	if !found {
		return TraversalResult{}, fmt.Errorf("PROFILE_GRAPH_TRANSITION_INVALID: cursor slot is unavailable")
	}
	for index, value := range slot.Traversal {
		if value == cursor {
			if signal != "" && index+1 < len(slot.Traversal) {
				return TraversalResult{}, fmt.Errorf("PROFILE_GRAPH_TRANSITION_INVALID: signal supplied before slot completion")
			}
			if next, found := nextActionableIn(record, slot.Traversal, index+1); found {
				return TraversalResult{Disposition: TraversalNext, Cursor: &next}, nil
			}
			break
		}
	}
	if slot.Terminal {
		return TraversalResult{Disposition: TraversalTerminal}, nil
	}
	for _, transition := range slot.Transitions {
		if transition.Signal == signal {
			return traversalAtSlot(record, transition.Target)
		}
	}
	return TraversalResult{}, fmt.Errorf("PROFILE_GRAPH_TRANSITION_INVALID: signal is not declared")
}

func firstActionableIn(record ExecutionGraphRecord, cursors []execution.GraphCursor) (execution.GraphCursor, bool) {
	return nextActionableIn(record, cursors, 0)
}

func nextActionableIn(record ExecutionGraphRecord, cursors []execution.GraphCursor, start int) (execution.GraphCursor, bool) {
	for index := start; index < len(cursors); index++ {
		cursor := cursors[index]
		if cursor.Kind != execution.CursorBinding {
			return cursor, true
		}
		for _, slot := range record.Slots {
			for _, binding := range slot.Pipeline {
				if binding.Cursor == cursor && binding.Disposition == DispatchByCoordinator {
					return cursor, true
				}
			}
		}
	}
	return execution.GraphCursor{}, false
}

func traversalAtSlot(record ExecutionGraphRecord, slotID catalog.SlotID) (TraversalResult, error) {
	visited := make(map[catalog.SlotID]struct{})
	for {
		if _, duplicate := visited[slotID]; duplicate {
			return TraversalResult{}, fmt.Errorf("PROFILE_GRAPH_TRANSITION_INVALID: target traversal cycles")
		}
		visited[slotID] = struct{}{}
		slot, found := compiledSlotByID(record.Slots, slotID)
		if !found || !slot.Active {
			return TraversalResult{}, fmt.Errorf("PROFILE_GRAPH_TRANSITION_INVALID: target slot is unavailable")
		}
		if next, found := firstActionableIn(record, slot.Traversal); found {
			return TraversalResult{Disposition: TraversalNext, Cursor: &next}, nil
		}
		if len(slot.Transitions) != 1 || slot.Transitions[0].Signal != "succeeded" {
			return TraversalResult{}, fmt.Errorf("PROFILE_GRAPH_TRANSITION_INVALID: target slot has no actionable unit")
		}
		slotID = slot.Transitions[0].Target
	}
}

func compiledSlotByID(values []CompiledSlot, id catalog.SlotID) (CompiledSlot, bool) {
	for _, value := range values {
		if value.SlotID == id {
			return cloneCompiledSlot(value), true
		}
	}
	return CompiledSlot{}, false
}
