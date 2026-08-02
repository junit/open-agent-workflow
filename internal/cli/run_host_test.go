package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	oawruntime "github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

type fakeHostExchange struct {
	frames []oawruntime.RunFrame
	step   int
	grant  admission.CapabilityGrant
}

func (fake *fakeHostExchange) Exchange(frame oawruntime.RunFrame) (oawruntime.RunReply, error) {
	fake.frames = append(fake.frames, frame)
	fake.step++
	snapshot := oawruntime.RunSnapshot{SchemaVersion: "oaw.runtime-snapshot/v1", RunID: "run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Revision: uint64(fake.step), RequestMode: classification.RequestModeBounded, Status: oawruntime.RunGranted, Grants: []admission.CapabilityGrant{fake.grant}}
	switch fake.step {
	case 1:
		return oawruntime.RunReply{SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.ReplyGrantIssued, RunID: snapshot.RunID, Revision: 1, Snapshot: snapshot, Diagnostics: []oawruntime.Diagnostic{}, RecoveryActions: []string{}}, nil
	case 2:
		snapshot.Status = oawruntime.RunInFlight
		return oawruntime.RunReply{SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.ReplyDispatchAuthorized, RunID: snapshot.RunID, Revision: 2, Snapshot: snapshot, Diagnostics: []oawruntime.Diagnostic{}, RecoveryActions: []string{}}, nil
	default:
		snapshot.Status = oawruntime.RunFinished
		return oawruntime.RunReply{SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.ReplyFinished, RunID: snapshot.RunID, Revision: 3, Snapshot: snapshot, Diagnostics: []oawruntime.Diagnostic{}, RecoveryActions: []string{}}, nil
	}
}

type fakeHostDriver struct {
	prepared host.DispatchRequest
	invoked  int
	result   host.DispatchResult
	err      error
}

func (fake *fakeHostDriver) Prepare(request host.DispatchRequest) error {
	fake.prepared = request
	return nil
}

func (fake *fakeHostDriver) Invoke(request host.DispatchRequest) (host.DispatchResult, error) {
	fake.invoked++
	if fake.err != nil {
		return host.DispatchResult{}, fake.err
	}
	return fake.result, nil
}

func (fake *fakeHostDriver) Cancel(string) error { return nil }

func TestRunHostLoopUsesRuntimeExchangeForOrderedDispatch(t *testing.T) {
	grant := admission.CapabilityGrant{ID: "grant", InvocationID: "invocation", RegistryDigest: strings.Repeat("a", 64), Executor: admission.ExecutorRegistration{ID: "executor", Kind: admission.ExecutorIsolated}, Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "to-spec"}}
	engine := &fakeHostExchange{grant: grant}
	driver := &fakeHostDriver{result: host.DispatchResult{GrantID: "grant", InvocationID: "invocation", ExecutorID: "executor", ExecutionID: "execution", Outcome: host.DispatchSucceeded, Evidence: []host.DispatchEvidence{{Reference: "evidence://codex/invocation", Digest: strings.Repeat("a", 64)}}}}
	input := `{"schema_version":"oaw.runtime/v1","kind":"CONTINUE","message_id":"m","idempotency_key":"k","run_id":"run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_revision":1,"continue":{"signal":"REQUEST_STAGE_GRANT"}}`
	var output bytes.Buffer
	if err := runHostLoop(strings.NewReader(input), engine, driver, &output); err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'})
	if len(lines) != 3 || driver.invoked != 1 || len(engine.frames) != 3 {
		t.Fatalf("output lines=%d invoked=%d frames=%d output=%q", len(lines), driver.invoked, len(engine.frames), output.String())
	}
	for _, line := range lines {
		var reply oawruntime.RunReply
		if err := json.Unmarshal(line, &reply); err != nil {
			t.Fatal(err)
		}
		canonical, err := oawruntime.EncodeReply(reply)
		if err != nil || !bytes.Equal(line, canonical) {
			t.Fatalf("non-canonical reply %q", line)
		}
	}
	if engine.frames[1].Continue == nil || engine.frames[1].Continue.Signal != oawruntime.SignalDispatchPrepared || engine.frames[2].Continue == nil || engine.frames[2].Continue.Signal != oawruntime.SignalCapabilityObserved {
		t.Fatalf("runtime handshake frames = %#v", engine.frames)
	}
	if driver.prepared.Binding != grant.Binding || driver.prepared.InvocationID != grant.InvocationID {
		t.Fatalf("driver request = %#v", driver.prepared)
	}
}

func TestRunRejectsUnsupportedHostWithoutInvokingAnything(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := RunWithInput([]string{"run", "--host", "gemini", "--state-root", t.TempDir()}, strings.NewReader(`{}`), &stdout, &stderr)
	if status != 69 || !json.Valid(stdout.Bytes()) || !strings.Contains(stderr.String(), "HOST_RUNTIME_UNSUPPORTED") {
		t.Fatalf("unsupported Host status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
}

func TestGrantDispatchDigestUsesLifecycleBundleDigest(t *testing.T) {
	grant := admission.CapabilityGrant{BundleID: "bundle-1", RegistryDigest: strings.Repeat("a", 64), GraphDigest: strings.Repeat("b", 64)}
	snapshot := oawruntime.RunSnapshot{Workflow: &oawruntime.WorkflowState{Bundles: []oawruntime.LifecycleBundle{{ID: "bundle-1", Digest: strings.Repeat("c", 64)}}}}
	digest, err := grantDispatchDigest(snapshot, grant)
	if err != nil {
		t.Fatal(err)
	}
	if digest != strings.Repeat("c", 64) {
		t.Fatalf("dispatch digest = %q, want Lifecycle Bundle digest", digest)
	}
}

func TestGrantDispatchDigestRejectsMissingLifecycleBundle(t *testing.T) {
	grant := admission.CapabilityGrant{BundleID: "bundle-1", RegistryDigest: strings.Repeat("a", 64), GraphDigest: strings.Repeat("b", 64)}
	if _, err := grantDispatchDigest(oawruntime.RunSnapshot{Workflow: &oawruntime.WorkflowState{}}, grant); err == nil {
		t.Fatal("dispatch digest accepted a Grant without its committed Lifecycle Bundle")
	}
}
