package management

import (
	"bytes"
	"fmt"
)

type scope string
type targetID string

func renderTarget(id targetID, operationScope scope, policyPath string) ([]byte, error) {
	var rendered string
	switch string(operationScope) + ":" + string(id) {
	case "user:claude", "project:claude":
		rendered = "Before any new top-level engineering task that may use workflow skills, read and follow the Open Agent Workflow policy:\n@" + policyPath + "\n"
	case "user:codex":
		rendered = fmt.Sprintf("For every new top-level engineering request, first read `%s`, classify it as DIRECT, BOUNDED, or WORKFLOW, and run its blocking selection gate only for WORKFLOW. Preserve the selected Lifecycle Bundle for Workflow work.\n", policyPath)
	case "user:gemini", "project:gemini":
		rendered = "Follow the Open Agent Workflow policy before engineering lifecycle work:\n@" + policyPath + "\n"
	case "user:opencode":
		rendered = fmt.Sprintf("Before engineering lifecycle work, use the Read tool to read `%s`, then follow its blocking selection gate and lifecycle lock.\n", policyPath)
	case "project:codex", "project:opencode", "project:cline", "project:roo":
		rendered = renderProjectBootstrap(policyPath)
	case "project:cursor":
		rendered = "---\ndescription: Open Agent Workflow lifecycle policy\nglobs: \"**/*\"\nalwaysApply: true\n---\n\n" + renderProjectBootstrap(policyPath)
	case "project:windsurf":
		rendered = "---\ntrigger: always_on\n---\n\n" + renderProjectBootstrap(policyPath)
	case "project:copilot":
		rendered = "---\napplyTo: \"**\"\n---\n\n" + renderProjectBootstrap(policyPath)
	default:
		return nil, &Error{Status: 69, Message: fmt.Sprintf("no renderer for %s target '%s'", operationScope, id)}
	}
	return []byte(rendered), nil
}

func renderProjectBootstrap(policyPath string) string {
	return fmt.Sprintf("Before engineering lifecycle work, read `%s`, follow its blocking selection gate, and preserve the selected lifecycle bundle for the task.\n", policyPath)
}

func renderManagedBlock(id targetID, operationScope scope, policyPath string) ([]byte, error) {
	body, err := renderTarget(id, operationScope, policyPath)
	if err != nil {
		return nil, err
	}
	result := make([]byte, 0, len(beginMarker)+len(body)+len(endMarker)+2)
	result = append(result, beginMarker...)
	result = append(result, '\n')
	result = append(result, body...)
	result = append(result, endMarker...)
	result = append(result, '\n')
	return result, nil
}

func renderManagedFile(current, block []byte) ([]byte, error) {
	lines := managedLineSpans(current)
	beginCount, endCount := 0, 0
	beginIndex, endIndex := -1, -1
	for index, line := range lines {
		switch string(current[line.start:line.contentEnd]) {
		case beginMarker:
			beginCount++
			if beginIndex == -1 {
				beginIndex = index
			}
		case endMarker:
			endCount++
			if endIndex == -1 {
				endIndex = index
			}
		}
	}

	if beginCount == 0 && endCount == 0 {
		if len(current) != 0 && current[len(current)-1] != '\n' {
			result := make([]byte, 0, len(block)+len(current))
			result = append(result, block...)
			result = append(result, current...)
			return result, nil
		}
		result := make([]byte, 0, len(current)+len(block))
		result = append(result, current...)
		result = append(result, block...)
		return result, nil
	}
	if beginCount != 1 || endCount != 1 || beginIndex >= endIndex {
		return nil, compatibilityError("managed markers are invalid")
	}

	prefixEnd := lines[beginIndex].start
	suffixStart := lines[endIndex].end
	result := make([]byte, 0, prefixEnd+len(block)+len(current)-suffixStart)
	result = append(result, current[:prefixEnd]...)
	result = append(result, block...)
	result = append(result, current[suffixStart:]...)
	return result, nil
}

type managedLineSpan struct {
	start      int
	contentEnd int
	end        int
}

func managedLineSpans(data []byte) []managedLineSpan {
	lines := make([]managedLineSpan, 0, bytes.Count(data, []byte{'\n'})+1)
	for start := 0; start < len(data); {
		offset := bytes.IndexByte(data[start:], '\n')
		if offset == -1 {
			lines = append(lines, managedLineSpan{start: start, contentEnd: len(data), end: len(data)})
			break
		}
		contentEnd := start + offset
		lines = append(lines, managedLineSpan{start: start, contentEnd: contentEnd, end: contentEnd + 1})
		start = contentEnd + 1
	}
	return lines
}
