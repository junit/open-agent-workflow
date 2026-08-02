package cli

import (
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
	hostID    string
	stateRoot string
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
	engine, err := oawruntime.NewEngine(oawruntime.Options{StateRoot: parsed.stateRoot})
	if err != nil {
		return writeRuntimeDenial(runtimeReason(err), err, 65, stdout, stderr)
	}
	driver := codex.New(codex.Options{Diagnostics: stderr})
	if err := runHostLoop(stdin, engine, driver, stdout); err != nil {
		return writeRuntimeDenial(runtimeReason(err), err, 65, stdout, stderr)
	}
	return 0
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
	hostSeen, stateSeen := false, false
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
	request := host.DispatchRequest{GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID, BundleDigest: grantBundleDigest(grant), Binding: grant.Binding}
	if err := driver.Prepare(request); err != nil {
		return err
	}
	preparedFrame := oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: frame.MessageID + "-dispatch-prepared", IdempotencyKey: frame.IdempotencyKey + "-dispatch-prepared",
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
			MessageID: frame.MessageID + "-execution-uncertain", IdempotencyKey: frame.IdempotencyKey + "-execution-uncertain",
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

func latestGrant(grants []admission.CapabilityGrant) (admission.CapabilityGrant, error) {
	if len(grants) == 0 {
		return admission.CapabilityGrant{}, fmt.Errorf("RUNTIME_RUN_INVALID: GRANT_ISSUED reply has no Grant")
	}
	return admission.CloneGrant(grants[len(grants)-1]), nil
}

func grantBundleDigest(grant admission.CapabilityGrant) string {
	if grant.BundleID != "" {
		return grant.GraphDigest
	}
	return grant.RegistryDigest
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
	return oawruntime.RunFrame{SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue, MessageID: original.MessageID + "-capability-observed", IdempotencyKey: original.IdempotencyKey + "-capability-observed", RunID: authorized.RunID, ExpectedRevision: authorized.Revision, Continue: continueInput}
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
