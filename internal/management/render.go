package management

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

type scope string
type targetID string

const activationRouterPrefix = "Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. "

const activationRouterSuffix = " Apply the selected Policy Set only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit ends OAW governance for that deliverable.\n"

func renderArtifact(id targetID, artifactID string, operationScope scope, policyPath string) ([]byte, error) {
	candidate, targetFound := findTarget(string(id))
	artifact, found := findTargetArtifact(string(id), artifactID)
	if !targetFound || !found {
		return nil, &Error{Status: 69, Message: fmt.Sprintf("no renderer for %s target '%s' artifact '%s'", operationScope, id, artifactID)}
	}
	if !targetSupportsScope(candidate, string(operationScope)) {
		return nil, &Error{Status: 69, Message: fmt.Sprintf("no renderer for %s target '%s' artifact '%s'", operationScope, id, artifactID)}
	}
	if policyPath == "" || hasControl(policyPath) {
		return nil, integrityError("policy path cannot be rendered")
	}
	if artifact.ID == routerArtifactID {
		return renderRouterTarget(id, operationScope, policyPath)
	}
	switch artifact.Kind {
	case skillArtifactKind:
		return renderSkillDispatcher(string(id))
	case geminiCommandArtifactKind:
		return renderGeminiCommand()
	case commandArtifactKind:
		return renderCommandDispatcher(string(id))
	case workflowArtifactKind:
		return renderWorkflowDispatcher()
	case codexPolicyArtifactKind:
		if string(id) != "codex" || artifact.ID != nativePolicyArtifactID {
			return nil, &Error{Status: 69, Message: fmt.Sprintf("no renderer for %s target '%s' artifact '%s'", operationScope, id, artifactID)}
		}
		return renderCodexPolicyMetadata()
	default:
		return nil, &Error{Status: 69, Message: fmt.Sprintf("no renderer for %s target '%s' artifact '%s'", operationScope, id, artifactID)}
	}
}

func renderRouterTarget(id targetID, operationScope scope, policyPath string) ([]byte, error) {
	var rendered string
	switch string(operationScope) + ":" + string(id) {
	case "user:claude", "project:claude", "user:codex", "user:gemini", "project:gemini", "user:opencode":
		rendered = renderActivationRouter(operationScope, policyPath)
	case "project:codex", "project:opencode", "project:cline", "project:roo":
		rendered = renderActivationRouter(operationScope, policyPath)
	case "project:cursor":
		rendered = "---\ndescription: Open Agent Workflow activation router\nglobs: \"**/*\"\nalwaysApply: true\n---\n\n" + renderActivationRouter(operationScope, policyPath)
	case "project:windsurf":
		rendered = "---\ntrigger: always_on\n---\n\n" + renderActivationRouter(operationScope, policyPath)
	case "project:copilot":
		rendered = "---\napplyTo: \"**\"\n---\n\n" + renderActivationRouter(operationScope, policyPath)
	default:
		return nil, &Error{Status: 69, Message: fmt.Sprintf("no renderer for %s target '%s'", operationScope, id)}
	}
	return []byte(rendered), nil
}

func renderSkillDispatcher(id string) ([]byte, error) {
	header := "---\nname: oaw\ndescription: Explicitly activate the current OAW Policy for a user-requested deliverable.\n"
	switch id {
	case "claude":
		header += "argument-hint: \"[PROFILE] <task>\"\nuser-invocable: true\ndisable-model-invocation: true\n"
	case "codex":
		// Codex uses agents/openai.yaml for implicit-invocation policy.
	case "cursor":
		header += "disable-model-invocation: true\n"
	case "copilot":
		header += "argument-hint: \"[PROFILE] <task>\"\ndisable-model-invocation: true\n"
	case "cline":
		// Cline's current Skill format has no disable-model-invocation field.
	default:
		return nil, &Error{Status: 69, Message: "no Skill renderer for target '" + id + "'"}
	}
	header += "---\n\n"
	body := renderNativeDispatcher("$ARGUMENTS")
	if id == "codex" || id == "cursor" || id == "cline" || id == "copilot" {
		body = renderNativeDispatcher("the remainder of this user request")
	}
	return []byte(header + body), nil
}

func renderGeminiCommand() ([]byte, error) {
	body := renderNativeDispatcher("{{args}}")
	return []byte("description = \"Explicitly activate the current OAW Policy for a user-requested deliverable.\"\n" +
		"prompt = " + strconv.Quote(body) + "\n"), nil
}

func renderCommandDispatcher(id string) ([]byte, error) {
	arguments := "the remainder of this user request"
	metadata := "description: Explicitly activate the current OAW Policy for a user-requested deliverable.\n"
	if id == "opencode" {
		arguments = "$ARGUMENTS"
	} else if id == "roo" {
		metadata += "argument-hint: \"[PROFILE] <task>\"\n"
	} else {
		return nil, &Error{Status: 69, Message: "no Command renderer for target '" + id + "'"}
	}
	return []byte("---\n" + metadata + "---\n\n" +
		renderNativeDispatcher(arguments)), nil
}

func renderWorkflowDispatcher() ([]byte, error) {
	return []byte(renderNativeDispatcher("the remainder of this user request")), nil
}

func renderCodexPolicyMetadata() ([]byte, error) {
	return []byte("interface:\n  display_name: \"Open Agent Workflow\"\n  short_description: \"Explicitly activate the current OAW Policy\"\npolicy:\n  allow_implicit_invocation: false\n"), nil
}

func renderNativeDispatcher(arguments string) string {
	return "Activate OAW only when the current top-level user request itself contains a literal `/oaw` or `$oaw`, explicitly asks to use OAW in natural language, or provides reliable Host metadata that the user, not the model, selected this entrypoint. Invocation or loading of this entrypoint alone is not evidence of user intent. If none of those conditions holds, do not activate OAW and continue as the native Host.\n\n" +
		"Follow the current OAW Activation Router to select and read one Policy Set. Do not embed or infer a Policy path here.\n\n" +
		"Pass the optional Profile and task from the invocation to the current Policy and Router. Do not choose a default Profile, lifecycle stages, approval gates, Skills, tools, or permissions here.\n\n" +
		"Invocation arguments: " + arguments + "\n"
}

func renderActivationRouter(operationScope scope, policyPath string) string {
	policyReference := markdownCodeSpan(policyPath)
	selection := fmt.Sprintf("On explicit activation, read %s as the Project Policy Set and do not read or merge the User Policy Set.", policyReference)
	if operationScope == "user" {
		selection = fmt.Sprintf("On explicit activation, if the current project contains `.oaw/policy/POLICY.md`, read that Project Policy Set and do not read or merge the User Policy Set; otherwise read %s as the User Policy Set.", policyReference)
	}
	return activationRouterPrefix + selection + activationRouterSuffix
}

func markdownCodeSpan(value string) string {
	longestRun := 0
	currentRun := 0
	for _, character := range value {
		if character == '`' {
			currentRun++
			if currentRun > longestRun {
				longestRun = currentRun
			}
			continue
		}
		currentRun = 0
	}
	delimiter := strings.Repeat("`", longestRun+1)
	return delimiter + value + delimiter
}

func renderManagedBlock(id targetID, operationScope scope, policyPath string) ([]byte, error) {
	body, err := renderArtifact(id, routerArtifactID, operationScope, policyPath)
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
		return nil, integrityError("managed markers are invalid")
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
