package management

import (
	"bytes"
	"fmt"
)

type scope string
type targetID string

const activationRouterPrefix = "Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. "

const activationRouterSuffix = " Apply the selected Policy Set only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit ends OAW governance for that deliverable.\n"

func renderTarget(id targetID, operationScope scope, policyPath string) ([]byte, error) {
	var rendered string
	switch string(operationScope) + ":" + string(id) {
	case "user:claude", "project:claude", "user:codex", "user:gemini", "project:gemini", "user:opencode":
		rendered = renderActivationRouter(operationScope, policyPath)
	case "project:codex", "project:opencode", "project:cline", "project:roo":
		rendered = renderActivationRouter(operationScope, policyPath)
	case "project:cursor":
		rendered = "---\ndescription: Open Agent Workflow lifecycle policy\nglobs: \"**/*\"\nalwaysApply: true\n---\n\n" + renderActivationRouter(operationScope, policyPath)
	case "project:windsurf":
		rendered = "---\ntrigger: always_on\n---\n\n" + renderActivationRouter(operationScope, policyPath)
	case "project:copilot":
		rendered = "---\napplyTo: \"**\"\n---\n\n" + renderActivationRouter(operationScope, policyPath)
	default:
		return nil, &Error{Status: 69, Message: fmt.Sprintf("no renderer for %s target '%s'", operationScope, id)}
	}
	return []byte(rendered), nil
}

func renderActivationRouter(operationScope scope, policyPath string) string {
	selection := fmt.Sprintf("On explicit activation, read `%s` as the Project Policy Set and do not read or merge the User Policy Set.", policyPath)
	if operationScope == "user" {
		selection = fmt.Sprintf("On explicit activation, if the current project contains `.oaw/policy/POLICY.md`, read that Project Policy Set and do not read or merge the User Policy Set; otherwise read `%s` as the User Policy Set.", policyPath)
	}
	return activationRouterPrefix + selection + activationRouterSuffix
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
