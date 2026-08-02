package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/host/codex"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
	oawruntime "github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

type runtimeExchangeCommand struct {
	stateRoot string
}

type runCommandOptions struct {
	hostID      string
	stateRoot   string
	projectRoot string
}

type runtimeDenial struct {
	SchemaVersion string `json:"schema_version"`
	Kind          string `json:"kind"`
	Reason        string `json:"reason"`
}

func runRuntimeExchange(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	parsed, err := parseRuntimeExchange(args)
	if err != nil {
		return writeRuntimeDenial("INVALID_ARGUMENT", err, 64, stdout, stderr)
	}
	engine, err := oawruntime.NewEngine(oawruntime.Options{StateRoot: parsed.stateRoot})
	if err != nil {
		return writeRuntimeDenial(runtimeReason(err), err, 65, stdout, stderr)
	}
	if err := oawruntime.ExchangeJSON(stdin, stdout, engine); err != nil {
		return writeRuntimeDenial(runtimeReason(err), err, 65, stdout, stderr)
	}
	return 0
}

func runCodex(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	parsed, err := parseRunCommand(args)
	if err != nil {
		return writeRuntimeDenial("INVALID_ARGUMENT", err, 64, stdout, stderr)
	}
	if parsed.hostID != "codex" {
		return writeRuntimeDenial("HOST_RUNTIME_UNSUPPORTED", fmt.Errorf("only selected Codex Host is Runtime-managed"), 69, stdout, stderr)
	}
	integrations, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil {
		return writeRuntimeDenial("HOST_RUNTIME_UNSUPPORTED", err, 69, stdout, stderr)
	}
	if err := host.RuntimeEntrypointAllowed(integrations, parsed.hostID); err != nil {
		return writeRuntimeDenial(runtimeReasonHost(err), err, 69, stdout, stderr)
	}
	raw, err := io.ReadAll(io.LimitReader(stdin, oawruntime.MaximumProtocolFrameBytes+1))
	if err != nil {
		return writeRuntimeDenial("RUNTIME_FRAME_READ_FAILED", err, 65, stdout, stderr)
	}
	frame, err := oawruntime.DecodeFrame(raw)
	if err != nil {
		return writeRuntimeDenial(runtimeReason(err), err, 65, stdout, stderr)
	}
	engine, err := newCLIEngine(parsed.stateRoot, parsed.projectRoot, frame, parsed.hostID)
	if err != nil {
		return writeRuntimeDenial(runtimeReason(err), err, 65, stdout, stderr)
	}
	driver := codex.New(codex.Options{Diagnostics: stderr})
	if err := runHostLoop(bytes.NewReader(raw), engine, driver, stdout); err != nil {
		return writeRuntimeDenial(runtimeReason(err), err, 65, stdout, stderr)
	}
	return 0
}

func newCLIEngine(stateRoot, configuredProjectRoot string, frame oawruntime.RunFrame, hostID string) (*oawruntime.Engine, error) {
	projectRoot := configuredProjectRoot
	if frame.Start != nil {
		frameProjectRoot := frame.Start.Project.Root
		if projectRoot != "" && projectRoot != frameProjectRoot {
			return nil, fmt.Errorf("RUNTIME_PROJECT_ROOT_MISMATCH: --project-root does not match START project root")
		}
		projectRoot = frameProjectRoot
	}
	if frame.Start != nil && frame.Start.Proposal != nil {
		decision, err := classification.Classify(frame.Start.Proposal, classification.ClassificationRules{})
		if err == nil && decision.RequestMode == classification.RequestModeDirect {
			return oawruntime.NewEngine(oawruntime.Options{StateRoot: stateRoot})
		}
	}
	userConfigRoot := defaultConfigRoot()
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userConfigRoot, ProjectRoot: projectRoot})
	if err != nil {
		return nil, fmt.Errorf("RUNTIME_CONFIGURATION_REQUIRED: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("RUNTIME_DISCOVERY_REQUIRED: %w", err)
	}
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{UserHome: home})
	if err != nil {
		return nil, fmt.Errorf("RUNTIME_DISCOVERY_REQUIRED: %w", err)
	}
	resolution, effective, err := registry.Resolve(snapshot, evidence, &registry.BindingInventory{Host: hostID, Bindings: catalogHostBindings(snapshot.Catalog(), hostID)})
	if err != nil {
		return nil, fmt.Errorf("RUNTIME_REGISTRY_REQUIRED: %w", err)
	}
	_ = resolution
	authority := admission.AuthorityCeiling{
		Effects:   []string{"git-local", "network-read", "read-project", "run-process", "write-project"},
		Resources: []string{"git-repository", "project", "project-worktree"}, ResourceLeases: true, AllowDelegation: true,
	}
	executors := []oawruntime.WorkflowExecutorRegistration{
		{Registration: admission.ExecutorRegistration{ID: "oaw-codex-write", Kind: admission.ExecutorIsolated}},
		{Registration: admission.ExecutorRegistration{ID: "oaw-codex-review", Kind: admission.ExecutorIsolated}, ReadOnly: true, Fresh: true},
	}
	return oawruntime.NewEngine(oawruntime.Options{
		StateRoot: stateRoot,
		Bounded:   oawruntime.BoundedOptions{Configuration: snapshot, Registry: effective, Authority: authority, Executors: []admission.ExecutorRegistration{{ID: "oaw-codex-write", Kind: admission.ExecutorIsolated}, {ID: "oaw-codex-review", Kind: admission.ExecutorIsolated}}},
		Workflow:  oawruntime.WorkflowOptions{Configuration: snapshot, Registry: effective, Authority: authority, Host: host.RuntimeFrame{IntegrationID: host.SelectedRuntimeIntegrationID}, Executors: executors},
	})
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

func catalogHostBindings(value catalog.Catalog, hostID string) []catalog.HostBinding {
	bindings := make([]catalog.HostBinding, 0)
	seen := make(map[string]struct{})
	for _, provider := range value.Providers() {
		for _, capability := range provider.Capabilities {
			for _, binding := range capability.HostBindings {
				if binding.Host != hostID {
					continue
				}
				key := binding.Host + "\x00" + binding.Kind + "\x00" + binding.Reference
				if _, found := seen[key]; found {
					continue
				}
				seen[key] = struct{}{}
				bindings = append(bindings, binding)
			}
		}
	}
	sort.Slice(bindings, func(left, right int) bool {
		return bindings[left].Host+"\x00"+bindings[left].Kind+"\x00"+bindings[left].Reference < bindings[right].Host+"\x00"+bindings[right].Kind+"\x00"+bindings[right].Reference
	})
	return bindings
}

func runtimeReasonHost(err error) string {
	if code := host.ErrorCode(err); code != "" {
		return code
	}
	return "HOST_RUNTIME_UNSUPPORTED"
}

func parseRunCommand(args []string) (runCommandOptions, error) {
	result := runCommandOptions{stateRoot: defaultRuntimeStateRoot()}
	if len(args) == 0 {
		return runCommandOptions{}, fmt.Errorf("run requires --host codex")
	}
	hostSeen, stateSeen, projectSeen := false, false, false
	for index := 0; index < len(args); {
		argument := args[index]
		switch {
		case argument == "--host":
			if hostSeen || index+1 >= len(args) {
				return runCommandOptions{}, fmt.Errorf("--host requires one value")
			}
			hostSeen = true
			result.hostID = args[index+1]
			index += 2
		case strings.HasPrefix(argument, "--host="):
			if hostSeen {
				return runCommandOptions{}, fmt.Errorf("--host may be specified only once")
			}
			hostSeen = true
			result.hostID = strings.TrimPrefix(argument, "--host=")
			index++
		case argument == "--state-root":
			if stateSeen || index+1 >= len(args) {
				return runCommandOptions{}, fmt.Errorf("--state-root requires one value")
			}
			stateSeen = true
			result.stateRoot = args[index+1]
			index += 2
		case strings.HasPrefix(argument, "--state-root="):
			if stateSeen {
				return runCommandOptions{}, fmt.Errorf("--state-root may be specified only once")
			}
			stateSeen = true
			result.stateRoot = strings.TrimPrefix(argument, "--state-root=")
			index++
		case argument == "--project-root":
			if projectSeen || index+1 >= len(args) {
				return runCommandOptions{}, fmt.Errorf("--project-root requires one value")
			}
			projectSeen = true
			result.projectRoot = args[index+1]
			index += 2
		case strings.HasPrefix(argument, "--project-root="):
			if projectSeen {
				return runCommandOptions{}, fmt.Errorf("--project-root may be specified only once")
			}
			projectSeen = true
			result.projectRoot = strings.TrimPrefix(argument, "--project-root=")
			index++
		default:
			return runCommandOptions{}, fmt.Errorf("unknown run argument %q", argument)
		}
	}
	if result.hostID == "" || strings.IndexFunc(result.hostID, unicode.IsControl) >= 0 {
		return runCommandOptions{}, fmt.Errorf("--host is required")
	}
	if result.stateRoot == "" || !filepath.IsAbs(result.stateRoot) || filepath.Clean(result.stateRoot) != result.stateRoot || strings.IndexFunc(result.stateRoot, unicode.IsControl) >= 0 {
		return runCommandOptions{}, fmt.Errorf("state root must be a clean absolute path")
	}
	if result.projectRoot != "" && (!filepath.IsAbs(result.projectRoot) || filepath.Clean(result.projectRoot) != result.projectRoot || strings.IndexFunc(result.projectRoot, unicode.IsControl) >= 0) {
		return runCommandOptions{}, fmt.Errorf("project root must be a clean absolute path")
	}
	return result, nil
}

type runtimeExchanger interface {
	Exchange(oawruntime.RunFrame) (oawruntime.RunReply, error)
}

func runHostLoop(input io.Reader, exchanger runtimeExchanger, driver host.Driver, output io.Writer) error {
	if input == nil || exchanger == nil || driver == nil || output == nil {
		return fmt.Errorf("RUNTIME_RUN_INVALID: runner dependencies are required")
	}
	raw, err := io.ReadAll(io.LimitReader(input, oawruntime.MaximumProtocolFrameBytes+1))
	if err != nil {
		return fmt.Errorf("RUNTIME_FRAME_READ_FAILED: %w", err)
	}
	frame, err := oawruntime.DecodeFrame(raw)
	if err != nil {
		return err
	}
	reply, err := exchanger.Exchange(frame)
	if err != nil {
		return err
	}
	if err := writeReplyLine(output, reply); err != nil {
		return err
	}
	if reply.Kind != oawruntime.ReplyGrantIssued {
		return nil
	}
	grant, err := latestGrant(reply.Snapshot.Grants)
	if err != nil {
		return err
	}
	current, err := exchanger.Exchange(dispatchInspectFrame(reply, grant))
	if err != nil {
		return err
	}
	if current.RunID != reply.RunID || current.Revision < reply.Revision {
		return fmt.Errorf("RUNTIME_RUN_INVALID: dispatch inspection returned inconsistent state")
	}
	if current.Revision > reply.Revision {
		return resumeCommittedDispatch(exchanger, output, current, grant)
	}
	bundleDigest, err := grantDispatchDigest(reply.Snapshot, grant)
	if err != nil {
		return err
	}
	request := host.DispatchRequest{GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID, BundleDigest: bundleDigest, Binding: grant.Binding}
	if err := driver.Prepare(request); err != nil {
		return err
	}
	preparedFrame := oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: derivedRunnerID("dispatch-prepared", grant), IdempotencyKey: derivedRunnerID("dispatch-prepared", grant),
		RunID: reply.RunID, ExpectedRevision: reply.Revision,
		Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalDispatchPrepared, DispatchPreparation: &oawruntime.DispatchPreparation{GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID}},
	}
	authorized, err := exchanger.Exchange(preparedFrame)
	if err != nil {
		return err
	}
	if err := writeReplyLine(output, authorized); err != nil {
		return err
	}
	if authorized.Kind != oawruntime.ReplyDispatchAuthorized {
		return fmt.Errorf("RUNTIME_RUN_INVALID: Runtime did not authorize dispatch")
	}
	result, invokeErr := driver.Invoke(request)
	if invokeErr != nil {
		uncertain := oawruntime.RunFrame{
			SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
			MessageID: derivedRunnerID("execution-uncertain", grant), IdempotencyKey: derivedRunnerID("execution-uncertain", grant),
			RunID: authorized.RunID, ExpectedRevision: authorized.Revision,
			Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalExecutionUncertain},
		}
		paused, pauseErr := exchanger.Exchange(uncertain)
		if pauseErr != nil {
			return pauseErr
		}
		if writeErr := writeReplyLine(output, paused); writeErr != nil {
			return writeErr
		}
		return nil
	}
	continueFrame := observationFrame(frame, authorized, grant, result)
	observed, err := exchanger.Exchange(continueFrame)
	if err != nil {
		return err
	}
	return writeReplyLine(output, observed)
}

func dispatchInspectFrame(reply oawruntime.RunReply, grant admission.CapabilityGrant) oawruntime.RunFrame {
	return oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameInspect,
		MessageID: derivedRunnerID("dispatch-state", grant), IdempotencyKey: derivedRunnerID("dispatch-state", grant),
		RunID: reply.RunID,
	}
}

func resumeCommittedDispatch(exchanger runtimeExchanger, output io.Writer, current oawruntime.RunReply, grant admission.CapabilityGrant) error {
	if grantObserved(current.Snapshot, grant) || current.Snapshot.Status == oawruntime.RunPaused || current.Snapshot.Status == oawruntime.RunFinished {
		return writeReplyLine(output, current)
	}
	if current.Snapshot.Status != oawruntime.RunInFlight {
		return fmt.Errorf("RUNTIME_RUN_INVALID: newer Runtime state lacks the Grant observation")
	}
	uncertain := oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: derivedRunnerID("execution-uncertain", grant), IdempotencyKey: derivedRunnerID("execution-uncertain", grant),
		RunID: current.RunID, ExpectedRevision: current.Revision,
		Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalExecutionUncertain},
	}
	paused, err := exchanger.Exchange(uncertain)
	if err != nil {
		return err
	}
	return writeReplyLine(output, paused)
}

func grantObserved(snapshot oawruntime.RunSnapshot, grant admission.CapabilityGrant) bool {
	for _, observation := range snapshot.Observations {
		if observation.GrantID == grant.ID && observation.InvocationID == grant.InvocationID && observation.ExecutorID == grant.Executor.ID {
			return true
		}
	}
	if snapshot.Workflow != nil {
		for _, observation := range snapshot.Workflow.Observations {
			if observation.GrantID == grant.ID && observation.InvocationID == grant.InvocationID && observation.ExecutorID == grant.Executor.ID {
				return true
			}
		}
	}
	return false
}

func derivedRunnerID(kind string, grant admission.CapabilityGrant) string {
	return "oaw-" + grant.ID + "-" + kind
}

func latestGrant(grants []admission.CapabilityGrant) (admission.CapabilityGrant, error) {
	if len(grants) == 0 {
		return admission.CapabilityGrant{}, fmt.Errorf("RUNTIME_RUN_INVALID: GRANT_ISSUED reply has no Grant")
	}
	return admission.CloneGrant(grants[len(grants)-1]), nil
}

func grantDispatchDigest(snapshot oawruntime.RunSnapshot, grant admission.CapabilityGrant) (string, error) {
	if grant.BundleID == "" {
		if grant.RegistryDigest == "" {
			return "", fmt.Errorf("RUNTIME_RUN_INVALID: Grant has no bounded dispatch digest")
		}
		return grant.RegistryDigest, nil
	}
	if snapshot.Workflow == nil {
		return "", fmt.Errorf("RUNTIME_RUN_INVALID: Workflow Grant has no Workflow snapshot")
	}
	for _, bundle := range snapshot.Workflow.Bundles {
		if bundle.ID == grant.BundleID {
			if bundle.Digest == "" {
				return "", fmt.Errorf("RUNTIME_RUN_INVALID: Lifecycle Bundle has no digest")
			}
			return bundle.Digest, nil
		}
	}
	return "", fmt.Errorf("RUNTIME_RUN_INVALID: Grant Bundle is absent from the committed snapshot")
}

func observationFrame(original oawruntime.RunFrame, authorized oawruntime.RunReply, grant admission.CapabilityGrant, result host.DispatchResult) oawruntime.RunFrame {
	outcome := oawruntime.ObservationFailed
	signal := "functional-failure"
	if result.Outcome == host.DispatchSucceeded {
		outcome = oawruntime.ObservationSucceeded
		signal = "succeeded"
	}
	evidence := make([]oawruntime.EvidenceReference, 0, len(result.Evidence))
	for _, value := range result.Evidence {
		evidence = append(evidence, oawruntime.EvidenceReference{Reference: value.Reference, Digest: value.Digest})
	}
	base := oawruntime.CapabilityObservation{GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID, Outcome: outcome, EvidenceReferences: evidence}
	continueInput := &oawruntime.ContinueInput{Signal: oawruntime.SignalCapabilityObserved}
	if authorized.Snapshot.RequestMode == classification.RequestModeWorkflow {
		continueInput.StageObservation = &oawruntime.StageObservation{CapabilityObservation: base, Signal: signal}
	} else {
		continueInput.Observation = &base
	}
	return oawruntime.RunFrame{SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue, MessageID: derivedRunnerID("capability-observed", grant), IdempotencyKey: derivedRunnerID("capability-observed", grant), RunID: authorized.RunID, ExpectedRevision: authorized.Revision, Continue: continueInput}
}

func writeReplyLine(output io.Writer, reply oawruntime.RunReply) error {
	encoded, err := oawruntime.EncodeReply(reply)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if _, err := output.Write(encoded); err != nil {
		return fmt.Errorf("RUNTIME_REPLY_WRITE_FAILED: %w", err)
	}
	return nil
}

func parseRuntimeExchange(args []string) (runtimeExchangeCommand, error) {
	if len(args) == 0 || args[0] != "exchange" {
		return runtimeExchangeCommand{}, fmt.Errorf("expected runtime exchange command")
	}
	result := runtimeExchangeCommand{stateRoot: defaultRuntimeStateRoot()}
	seen := false
	for index := 1; index < len(args); {
		argument := args[index]
		switch {
		case argument == "--state-root":
			if seen || index+1 >= len(args) {
				return runtimeExchangeCommand{}, fmt.Errorf("--state-root requires one value")
			}
			seen = true
			result.stateRoot = args[index+1]
			index += 2
		case strings.HasPrefix(argument, "--state-root="):
			if seen {
				return runtimeExchangeCommand{}, fmt.Errorf("--state-root may be specified only once")
			}
			seen = true
			result.stateRoot = strings.TrimPrefix(argument, "--state-root=")
			index++
		default:
			return runtimeExchangeCommand{}, fmt.Errorf("unknown runtime exchange argument %q", argument)
		}
	}
	if result.stateRoot == "" || !filepath.IsAbs(result.stateRoot) || filepath.Clean(result.stateRoot) != result.stateRoot || strings.IndexFunc(result.stateRoot, unicode.IsControl) >= 0 {
		return runtimeExchangeCommand{}, fmt.Errorf("state root must be a clean absolute path")
	}
	return result, nil
}

func defaultRuntimeStateRoot() string {
	root := os.Getenv("XDG_STATE_HOME")
	if root == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return ""
		}
		root = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(root, "open-agent-workflow", "runtime")
}

func runtimeReason(err error) string {
	if code := oawruntime.ErrorCode(err); code != "" {
		return code
	}
	return "RUNTIME_INTERNAL"
}

func writeRuntimeDenial(reason string, err error, status int, stdout, stderr io.Writer) int {
	encoded, encodeErr := canonicaljson.Marshal(runtimeDenial{
		SchemaVersion: oawruntime.RuntimeSchemaV1,
		Kind:          "DENIED",
		Reason:        reason,
	})
	if encodeErr != nil {
		fmt.Fprintf(stderr, "oaw: RUNTIME_REPLY_ENCODE_FAILED: %v\n", encodeErr)
		return 70
	}
	if _, writeErr := stdout.Write(encoded); writeErr != nil {
		fmt.Fprintf(stderr, "oaw: RUNTIME_REPLY_WRITE_FAILED: %v\n", writeErr)
		return 74
	}
	fmt.Fprintf(stderr, "oaw: %s: %v\n", reason, err)
	return status
}
