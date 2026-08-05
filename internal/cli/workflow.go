package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
)

type workflowExchangeCommand struct {
	stateRoot   string
	projectRoot string
}

func runWorkflowExchange(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	parsed, err := parseWorkflowExchange(args)
	if err != nil {
		return writeWorkflowRejection("INVALID_ARGUMENT", err, 64, stdout, stderr)
	}
	raw, err := io.ReadAll(io.LimitReader(stdin, coordinator.MaximumProtocolFrameBytes+1))
	if err != nil {
		return writeWorkflowRejection("WORKFLOW_COMMAND_READ_FAILED", err, 65, stdout, stderr)
	}
	command, err := coordinator.DecodeCommand(raw)
	if err != nil {
		return writeWorkflowRejection(workflowFailureCode(err), err, 65, stdout, stderr)
	}
	options, err := workflowCoordinatorOptions(parsed, command)
	if err != nil {
		return writeWorkflowRejection(workflowFailureCode(err), err, 65, stdout, stderr)
	}
	engine, err := coordinator.NewEngine(options)
	if err != nil {
		return writeWorkflowRejection(workflowFailureCode(err), err, 65, stdout, stderr)
	}
	result, err := engine.Exchange(command)
	if err != nil {
		return writeWorkflowRejection(workflowFailureCode(err), err, 65, stdout, stderr)
	}
	return writeWorkflowResult(result, 0, stdout, stderr)
}

func workflowCoordinatorOptions(parsed workflowExchangeCommand, command coordinator.Command) (coordinator.Options, error) {
	options := coordinator.Options{StateRoot: parsed.stateRoot}
	switch command.Kind {
	case coordinator.CommandStart:
		return withWorkflowProviderInputs(options, parsed.projectRoot, command.Start.HostSession.HostID)
	case coordinator.CommandSwitch:
		return withWorkflowProviderInputs(options, parsed.projectRoot, command.Switch.HostSession.HostID)
	case coordinator.CommandPrepare:
		options.PhysicalProjectRoot = parsed.projectRoot
		options.Authority = workflowAuthorityCeiling()
	}
	return options, nil
}

func withWorkflowProviderInputs(options coordinator.Options, projectRoot, hostID string) (coordinator.Options, error) {
	inputs, err := loadProviderInputs(providerInputOptions{
		HostID: hostID, ProjectRoot: projectRoot, UserConfigRoot: defaultConfigRoot(),
	})
	if err != nil {
		return coordinator.Options{}, fmt.Errorf("%s: run oaw providers inspect --host %s for physical evidence", providerInputReason(err), hostID)
	}
	options.Configuration = inputs.Configuration
	options.Resolutions = inputs.Resolutions
	options.Registry = inputs.Registry
	return options, nil
}

func workflowAuthorityCeiling() admission.AuthorityCeiling {
	return admission.AuthorityCeiling{
		Effects:         []string{"git-local", "network-read", "read-project", "run-process", "write-project"},
		Resources:       []string{"git-repository", "project", "project-worktree"},
		ResourceLeases:  true,
		AllowDelegation: true,
	}
}

func writeWorkflowRejection(code string, err error, status int, stdout, stderr io.Writer) int {
	fmt.Fprintf(stderr, "oaw: %s: %v\n", code, err)
	return writeWorkflowResult(coordinator.Result{
		SchemaVersion: coordinator.WorkflowResultSchemaV1,
		Kind:          coordinator.ResultRejected,
		Diagnostics:   []coordinator.Diagnostic{{Code: code, Detail: "Workflow exchange rejected"}},
	}, status, stdout, stderr)
}

func writeWorkflowResult(result coordinator.Result, status int, stdout, stderr io.Writer) int {
	raw, err := coordinator.EncodeResult(result)
	if err != nil {
		fmt.Fprintf(stderr, "oaw: WORKFLOW_RESULT_ENCODE_FAILED: %v\n", err)
		return 65
	}
	if _, err := stdout.Write(raw); err != nil {
		fmt.Fprintf(stderr, "oaw: WORKFLOW_RESULT_WRITE_FAILED: %v\n", err)
		return 65
	}
	return status
}

func workflowFailureCode(err error) string {
	if code := coordinator.ErrorCode(err); code != "" {
		return code
	}
	if reason := providerInputReason(err); strings.HasPrefix(reason, "PROVIDER_") {
		return reason
	}
	return "WORKFLOW_EXCHANGE_FAILED"
}

func parseWorkflowExchange(args []string) (workflowExchangeCommand, error) {
	if len(args) == 0 || args[0] != "exchange" {
		return workflowExchangeCommand{}, fmt.Errorf("expected workflow exchange command")
	}
	result := workflowExchangeCommand{stateRoot: defaultWorkflowStateRoot()}
	stateSeen, projectSeen := false, false
	for index := 1; index < len(args); {
		argument := args[index]
		switch {
		case argument == "--state-root" || strings.HasPrefix(argument, "--state-root="):
			value, next, err := parseWorkflowPathOption(args, index, "--state-root", &stateSeen)
			if err != nil {
				return workflowExchangeCommand{}, err
			}
			result.stateRoot, index = value, next
		case argument == "--project-root" || strings.HasPrefix(argument, "--project-root="):
			value, next, err := parseWorkflowPathOption(args, index, "--project-root", &projectSeen)
			if err != nil {
				return workflowExchangeCommand{}, err
			}
			result.projectRoot, index = value, next
		default:
			return workflowExchangeCommand{}, fmt.Errorf("unknown workflow exchange argument %q", argument)
		}
	}
	if !validWorkflowPath(result.stateRoot) {
		return workflowExchangeCommand{}, fmt.Errorf("state root must be a clean absolute path")
	}
	if result.projectRoot != "" && !validWorkflowPath(result.projectRoot) {
		return workflowExchangeCommand{}, fmt.Errorf("project root must be a clean absolute path")
	}
	return result, nil
}

func parseWorkflowPathOption(args []string, index int, name string, seen *bool) (string, int, error) {
	if *seen {
		return "", index, fmt.Errorf("%s may be specified only once", name)
	}
	*seen = true
	if args[index] == name {
		if index+1 >= len(args) || args[index+1] == "" {
			return "", index, fmt.Errorf("%s requires one value", name)
		}
		return args[index+1], index + 2, nil
	}
	value := strings.TrimPrefix(args[index], name+"=")
	if value == "" {
		return "", index, fmt.Errorf("%s requires one value", name)
	}
	return value, index + 1, nil
}

func validWorkflowPath(value string) bool {
	return value != "" && filepath.IsAbs(value) && filepath.Clean(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}

func defaultWorkflowStateRoot() string {
	root := os.Getenv("XDG_STATE_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		root = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(root, "open-agent-workflow", "workflows")
}

func defaultConfigRoot() string {
	root := os.Getenv("XDG_CONFIG_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		root = filepath.Join(home, ".config")
	}
	return filepath.Join(root, "open-agent-workflow")
}
