package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/host/codex"
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
	return runCodexContext(context.Background(), args, stdin, stdout, stderr)
}

func runCodexContext(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if ctx == nil {
		return writeRuntimeDenial("RUNTIME_RUN_INVALID", fmt.Errorf("run context is required"), 65, stdout, stderr)
	}
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
	engine, inputs, err := newCLIEngineContext(parsed.stateRoot, parsed.projectRoot, frame, parsed.hostID)
	if err != nil {
		return writeRuntimeDenial(runtimeReason(err), err, 65, stdout, stderr)
	}
	executionProjectRoot := parsed.projectRoot
	if frame.Start != nil {
		executionProjectRoot = frame.Start.Project.Root
	}
	driver := codex.New(codex.Options{Diagnostics: stderr, ExecutionRoot: parsed.stateRoot, ProjectRoot: executionProjectRoot, BindingResolver: codexBindingResolver(inputs)})
	if err := runHostLoop(ctx, bytes.NewReader(raw), engine, driver, stdout); err != nil {
		return writeRuntimeDenial(runtimeReason(err), err, 65, stdout, stderr)
	}
	return 0
}

func newCLIEngine(stateRoot, configuredProjectRoot string, frame oawruntime.RunFrame, hostID string) (*oawruntime.Engine, error) {
	engine, _, err := newCLIEngineContext(stateRoot, configuredProjectRoot, frame, hostID)
	return engine, err
}

func newCLIEngineContext(stateRoot, configuredProjectRoot string, frame oawruntime.RunFrame, hostID string) (*oawruntime.Engine, *providerInputs, error) {
	projectRoot := configuredProjectRoot
	if frame.Start != nil {
		frameProjectRoot := frame.Start.Project.Root
		if projectRoot != "" && projectRoot != frameProjectRoot {
			return nil, nil, fmt.Errorf("RUNTIME_PROJECT_ROOT_MISMATCH: --project-root does not match START project root")
		}
		projectRoot = frameProjectRoot
	}
	if frame.Start != nil && frame.Start.Proposal != nil {
		decision, err := classification.Classify(frame.Start.Proposal, classification.ClassificationRules{})
		if err == nil && decision.RequestMode == classification.RequestModeDirect {
			engine, engineErr := oawruntime.NewEngine(oawruntime.Options{StateRoot: stateRoot})
			return engine, nil, engineErr
		}
	}
	inputs, err := loadProviderInputs(providerInputOptions{HostID: hostID, ProjectRoot: projectRoot, UserConfigRoot: defaultConfigRoot()})
	if err != nil {
		return nil, nil, runtimeProviderInputsError(hostID, err)
	}
	authority := admission.AuthorityCeiling{
		Effects:   []string{"git-local", "network-read", "read-project", "run-process", "write-project"},
		Resources: []string{"git-repository", "project", "project-worktree"}, ResourceLeases: true, AllowDelegation: true,
	}
	executors := []oawruntime.WorkflowExecutorRegistration{
		{Registration: admission.ExecutorRegistration{ID: "oaw-codex-write", Kind: admission.ExecutorIsolated}},
		{Registration: admission.ExecutorRegistration{ID: "oaw-codex-review", Kind: admission.ExecutorIsolated}, ReadOnly: true, Fresh: true},
	}
	engine, engineErr := oawruntime.NewEngine(oawruntime.Options{
		StateRoot: stateRoot,
		Bounded:   oawruntime.BoundedOptions{Configuration: inputs.Configuration, Resolutions: inputs.Resolutions, Registry: inputs.Registry, HostID: hostID, Authority: authority, Executors: []admission.ExecutorRegistration{{ID: "oaw-codex-write", Kind: admission.ExecutorIsolated}, {ID: "oaw-codex-review", Kind: admission.ExecutorIsolated}}},
		Workflow:  oawruntime.WorkflowOptions{Configuration: inputs.Configuration, Resolutions: inputs.Resolutions, Registry: inputs.Registry, Authority: authority, Host: host.RuntimeFrame{HostID: hostID, IntegrationID: host.SelectedRuntimeIntegrationID}, Executors: executors},
	})
	return engine, &inputs, engineErr
}

func codexBindingResolver(inputs *providerInputs) codex.BindingResolver {
	if inputs == nil || inputs.Inventory == nil {
		return nil
	}
	return func(request host.DispatchRequest) (host.BindingObservation, error) {
		if inputs.HostID != "codex" || inputs.Registry.HostID() != inputs.HostID || inputs.Inventory.HostID != inputs.HostID || request.Binding.Host != inputs.HostID {
			return host.BindingObservation{}, fmt.Errorf("CODEX_BINDING_EVIDENCE_REQUIRED: Host authority is not the verified Codex inventory")
		}
		instance, found := inputs.Registry.Provider(request.ProviderID)
		if !found || instance.HostID != inputs.HostID || instance.Digest != request.ProviderInstanceDigest || instance.BindingInventoryDigest != inputs.Inventory.Digest {
			return host.BindingObservation{}, fmt.Errorf("CODEX_BINDING_EVIDENCE_REQUIRED: Provider Instance is not the verified dispatch instance")
		}
		capability, found := inputs.Registry.Capability(request.ProviderID, request.CapabilityID)
		if !found || capability.Binding != request.Binding {
			return host.BindingObservation{}, fmt.Errorf("CODEX_BINDING_EVIDENCE_REQUIRED: Capability Binding is not verified")
		}
		for _, observation := range inputs.Inventory.Observations {
			if observation.HostID == instance.HostID && observation.InstallationKey == instance.InstallationKey && observation.Binding == request.Binding && observation.Digest == capability.BindingEvidenceDigest {
				return observation, nil
			}
		}
		return host.BindingObservation{}, fmt.Errorf("CODEX_BINDING_EVIDENCE_REQUIRED: exact Host Binding evidence is absent")
	}
}

func runtimeProviderInputsError(hostID string, inputErr error) error {
	return fmt.Errorf("%s: Run oaw providers inspect --host %s for physical evidence.", providerInputReason(inputErr), hostID)
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

func runHostLoop(ctx context.Context, input io.Reader, exchanger runtimeExchanger, driver host.Driver, output io.Writer) error {
	if ctx == nil || input == nil || exchanger == nil || driver == nil || output == nil {
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
	return dispatchGranted(ctx, frame, reply, exchanger, driver, output)
}

func dispatchGranted(ctx context.Context, frame oawruntime.RunFrame, reply oawruntime.RunReply, exchanger runtimeExchanger, driver host.Driver, output io.Writer) error {
	grant, err := latestGrant(reply.Snapshot.Grants)
	if err != nil {
		return err
	}
	handled, err := inspectCommittedDispatch(exchanger, output, reply, grant)
	if err != nil || handled {
		return err
	}
	bundleDigest, err := grantDispatchDigest(reply.Snapshot, grant)
	if err != nil {
		return err
	}
	request := host.DispatchRequest{GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID, BundleDigest: bundleDigest, ProviderID: grant.ProviderID, CapabilityID: grant.CapabilityID, ProviderInstanceDigest: grant.ProviderInstanceDigest, Binding: grant.Binding, Effects: append([]string{}, grant.Effects...), Resources: append([]string{}, grant.Resources...)}
	if err := driver.Prepare(ctx, request); err != nil {
		return err
	}
	authorized, err := authorizeDispatch(exchanger, output, reply, grant)
	if err != nil {
		return err
	}
	return invokeAuthorizedDispatch(ctx, authorized, grant, request, exchanger, driver, output)
}

func inspectCommittedDispatch(exchanger runtimeExchanger, output io.Writer, reply oawruntime.RunReply, grant admission.CapabilityGrant) (bool, error) {
	current, err := exchanger.Exchange(dispatchInspectFrame(reply, grant))
	if err != nil {
		return false, err
	}
	if current.RunID != reply.RunID || current.Revision < reply.Revision {
		return false, fmt.Errorf("RUNTIME_RUN_INVALID: dispatch inspection returned inconsistent state")
	}
	if current.Revision == reply.Revision {
		return false, nil
	}
	return true, resumeCommittedDispatch(exchanger, output, current, grant)
}

func authorizeDispatch(exchanger runtimeExchanger, output io.Writer, reply oawruntime.RunReply, grant admission.CapabilityGrant) (oawruntime.RunReply, error) {
	preparedFrame := oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: derivedRunnerID("dispatch-prepared", grant), IdempotencyKey: derivedRunnerID("dispatch-prepared", grant),
		RunID: reply.RunID, ExpectedRevision: reply.Revision,
		Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalDispatchPrepared, DispatchPreparation: &oawruntime.DispatchPreparation{GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID}},
	}
	authorized, err := exchanger.Exchange(preparedFrame)
	if err != nil {
		return oawruntime.RunReply{}, err
	}
	if err := writeReplyLine(output, authorized); err != nil {
		return oawruntime.RunReply{}, err
	}
	if authorized.Kind != oawruntime.ReplyDispatchAuthorized {
		return oawruntime.RunReply{}, fmt.Errorf("RUNTIME_RUN_INVALID: Runtime did not authorize dispatch")
	}
	return authorized, nil
}

func invokeAuthorizedDispatch(ctx context.Context, authorized oawruntime.RunReply, grant admission.CapabilityGrant, request host.DispatchRequest, exchanger runtimeExchanger, driver host.Driver, output io.Writer) error {
	if ctx.Err() != nil {
		return cancelAndPauseUncertainExecution(ctx, authorized, grant, request.InvocationID, exchanger, driver, output)
	}
	type invocationResult struct {
		result host.DispatchResult
		err    error
	}
	done := make(chan invocationResult, 1)
	go func() {
		result, err := driver.Invoke(ctx, request)
		done <- invocationResult{result: result, err: err}
	}()
	select {
	case invocation := <-done:
		if ctx.Err() != nil {
			return cancelAndPauseUncertainExecution(ctx, authorized, grant, request.InvocationID, exchanger, driver, output)
		}
		if invocation.err != nil {
			return pauseUncertainExecution(authorized, grant, exchanger, output)
		}
		continueFrame := observationFrame(authorized, grant, invocation.result)
		observed, err := exchanger.Exchange(continueFrame)
		if err != nil {
			return err
		}
		return writeReplyLine(output, observed)
	case <-ctx.Done():
		return cancelAndPauseUncertainExecution(ctx, authorized, grant, request.InvocationID, exchanger, driver, output)
	}
}

func cancelAndPauseUncertainExecution(ctx context.Context, authorized oawruntime.RunReply, grant admission.CapabilityGrant, invocationID string, exchanger runtimeExchanger, driver host.Driver, output io.Writer) error {
	cancelErr := driver.Cancel(context.WithoutCancel(ctx), invocationID)
	pauseErr := pauseUncertainExecution(authorized, grant, exchanger, output)
	return errors.Join(cancelErr, pauseErr)
}

func pauseUncertainExecution(authorized oawruntime.RunReply, grant admission.CapabilityGrant, exchanger runtimeExchanger, output io.Writer) error {
	uncertain := oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: derivedRunnerID("execution-uncertain", grant), IdempotencyKey: derivedRunnerID("execution-uncertain", grant),
		RunID: authorized.RunID, ExpectedRevision: authorized.Revision,
		Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalExecutionUncertain},
	}
	paused, err := exchanger.Exchange(uncertain)
	if err != nil {
		return err
	}
	return writeReplyLine(output, paused)
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

func observationFrame(authorized oawruntime.RunReply, grant admission.CapabilityGrant, result host.DispatchResult) oawruntime.RunFrame {
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
	if reason := providerInputReason(err); strings.HasPrefix(reason, "PROVIDER_") {
		return reason
	}
	message := err.Error()
	for _, reason := range []string{
		"CODEX_BINDING_EVIDENCE_REQUIRED",
		"CODEX_BINDING_EVIDENCE_CHANGED",
		"CODEX_BINDING_KIND_UNSUPPORTED",
		"CODEX_EXECUTION_PROFILE_INVALID",
		"CODEX_MCP_INVENTORY_FAILED",
		"CODEX_MCP_ISOLATION_FAILED",
	} {
		if message == reason || strings.HasPrefix(message, reason+":") {
			return reason
		}
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
