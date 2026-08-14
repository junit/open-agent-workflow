package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/policyengagement"
	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
)

// runPolicy is intentionally retired. The old interface exposed reducer
// references, state roots, and machine-shaped options to callers.
func runPolicy(args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "oaw: POLICY_INTERFACE_REMOVED: use oaw profiles|use|status|complete|review|approve|satisfy|incident|switch|stop|uncertain")
	return 64
}

func runPolicySimple(args []string, stdout, stderr io.Writer) int {
	command, err := parsePolicySimple(args)
	if err != nil {
		fmt.Fprintf(stderr, "oaw: INVALID_ARGUMENT: %v\n", err)
		return 64
	}
	module, err := policyengagement.NewCurrent()
	if err != nil {
		return policyError(err, stderr)
	}
	result, err := module.Execute(command)
	if err != nil {
		if result.Profiles != nil {
			if status := writePolicyResult(result, stdout, stderr); status != 0 {
				return status
			}
		}
		return policyError(err, stderr)
	}
	return writePolicyResult(result, stdout, stderr)
}

func parsePolicySimple(args []string) (policyengagement.Command, error) {
	if len(args) == 0 {
		return policyengagement.Command{}, errors.New("expected profiles|use|status|complete|review|approve|satisfy|incident|switch|stop|uncertain")
	}
	command := policyengagement.Command{Action: policyengagement.Action(args[0])}
	args = args[1:]
	var err error
	switch command.Action {
	case policyengagement.ActionProfiles, policyengagement.ActionStatus,
		policyengagement.ActionComplete, policyengagement.ActionApprove,
		policyengagement.ActionSatisfy:
		if len(args) != 0 {
			return policyengagement.Command{}, fmt.Errorf("%s takes no arguments", command.Action)
		}
	case policyengagement.ActionUse:
		return parseUse(args, command)
	case policyengagement.ActionReview:
		if len(args) != 1 || args[0] != string(policyflow.ReviewClean) && args[0] != string(policyflow.ReviewFindings) {
			return policyengagement.Command{}, errors.New("review requires exactly one outcome: clean or findings")
		}
		command.Review = policyflow.ReviewOutcome(args[0])
	case policyengagement.ActionSwitch:
		if len(args) != 1 || strings.TrimSpace(args[0]) == "" || strings.HasPrefix(args[0], "-") {
			return policyengagement.Command{}, errors.New("switch requires exactly one Profile")
		}
		command.Profile = policyflow.ProfileID(args[0])
	case policyengagement.ActionStop:
		command.Reason, args, err = parseOptionalReason(args)
		if err != nil || len(args) != 0 {
			if err == nil {
				err = errors.New("stop accepts only --reason text")
			}
			return policyengagement.Command{}, err
		}
	case policyengagement.ActionUncertain:
		command.Reason, args, err = parseRequiredReason(args)
		if err != nil || len(args) != 0 {
			if err == nil {
				err = errors.New("uncertain accepts only --reason text")
			}
			return policyengagement.Command{}, err
		}
	case policyengagement.ActionIncident:
		if len(args) == 0 || strings.HasPrefix(args[0], "-") {
			return policyengagement.Command{}, errors.New("incident requires an incident type")
		}
		command.Incident = policyflow.IncidentType(args[0])
		command.Reason, args, err = parseOptionalReason(args[1:])
		if err != nil || len(args) != 0 {
			if err == nil {
				err = errors.New("incident accepts TYPE [--reason text]")
			}
			return policyengagement.Command{}, err
		}
	default:
		return policyengagement.Command{}, fmt.Errorf("unknown action %q", command.Action)
	}
	return command, nil
}

func parseUse(args []string, command policyengagement.Command) (policyengagement.Command, error) {
	for len(args) > 0 && args[0] != "--" {
		name, value, rest, err := parseUseOption(args)
		if err != nil {
			return policyengagement.Command{}, err
		}
		switch name {
		case "profile":
			if command.Profile != "" {
				return policyengagement.Command{}, errors.New("--profile may be specified only once")
			}
			command.Profile = policyflow.ProfileID(value)
		case "complexity":
			if command.Complexity != "" {
				return policyengagement.Command{}, errors.New("--complexity may be specified only once")
			}
			command.Complexity = value
		case "risk":
			if command.Risk != "" {
				return policyengagement.Command{}, errors.New("--risk may be specified only once")
			}
			command.Risk = value
		}
		args = rest
	}
	if len(args) < 2 || args[0] != "--" {
		return policyengagement.Command{}, errors.New("use requires [--profile PROFILE] --complexity ordinary|complex --risk normal|elevated|critical -- intent")
	}
	command.Intent = strings.TrimSpace(strings.Join(args[1:], " "))
	if command.Intent == "" {
		return policyengagement.Command{}, errors.New("use requires a non-empty intent after --")
	}
	return command, nil
}

func parseUseOption(args []string) (string, string, []string, error) {
	for _, name := range []string{"profile", "complexity", "risk"} {
		option := "--" + name
		if args[0] == option {
			if len(args) < 2 || args[1] == "" || strings.HasPrefix(args[1], "-") {
				return "", "", nil, fmt.Errorf("%s requires a value", option)
			}
			return name, args[1], args[2:], nil
		}
		if strings.HasPrefix(args[0], option+"=") {
			value := strings.TrimPrefix(args[0], option+"=")
			if value == "" {
				return "", "", nil, fmt.Errorf("%s requires a value", option)
			}
			return name, value, args[1:], nil
		}
	}
	return "", "", nil, fmt.Errorf("unknown use option %q", args[0])
}

func parseOptionalReason(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", args, nil
	}
	if args[0] != "--reason" && !strings.HasPrefix(args[0], "--reason=") {
		return "", args, errors.New("expected --reason text")
	}
	if strings.HasPrefix(args[0], "--reason=") {
		value := strings.TrimPrefix(args[0], "--reason=")
		if value == "" {
			return "", nil, errors.New("--reason requires a value")
		}
		return value, args[1:], nil
	}
	if len(args) < 2 || args[1] == "" {
		return "", nil, errors.New("--reason requires a value")
	}
	return args[1], args[2:], nil
}

func parseRequiredReason(args []string) (string, []string, error) {
	value, rest, err := parseOptionalReason(args)
	if err != nil {
		return "", rest, err
	}
	if value == "" {
		return "", rest, errors.New("--reason is required")
	}
	return value, rest, nil
}

func writePolicyResult(result policyengagement.Result, stdout, stderr io.Writer) int {
	if result.Profiles != nil {
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			return policyError(err, stderr)
		}
		return 0
	}
	if result.Status == nil {
		return policyError(errors.New("POLICY_OUTPUT_INVALID: empty result"), stderr)
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return policyError(err, stderr)
	}
	return 0
}

func policyError(err error, stderr io.Writer) int {
	fmt.Fprintf(stderr, "oaw: %v\n", err)
	return 65
}
