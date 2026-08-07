package appserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestClientUsesOnlyAllowlistedMetadataMethods(t *testing.T) {
	transport := newTranscriptTransport(t, "complete.jsonl")
	client := NewClient(ClientOptions{Transport: transport, MaximumMessageBytes: 8 << 20, RequestTimeout: time.Second})
	got, err := client.Observe(context.Background(), "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Methods, []string{"config/read", "hooks/list", "skills/list"}) {
		t.Fatalf("methods = %#v", got.Methods)
	}
	if requested := requestMethods(transport.Requests()); !slices.Equal(requested, []string{"initialize", "skills/list", "hooks/list", "config/read"}) {
		t.Fatalf("requested methods = %#v", requested)
	}
	if got.Config.SandboxDisposition != "host-configured" || got.Config.MCPDisposition != "host-configured" ||
		got.Config.HookDisposition != "host-configured" || got.Config.ApprovalDisposition != "host-configured" || !got.Config.CWDObserved {
		t.Fatalf("config projection = %#v", got.Config)
	}
	raw, err := json.Marshal(got.Hooks)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{[]byte("discarded-command"), []byte("private-hook"), []byte("fixture-private")} {
		if bytes.Contains(raw, forbidden) {
			t.Fatalf("Hook projection retained private metadata: %s", raw)
		}
	}
}

func TestObserveSendsExactCWDAndForceReload(t *testing.T) {
	transport := newMetadataTransport()
	_, err := NewClient(ClientOptions{Transport: transport}).Observe(context.Background(), "/repo")
	if err != nil {
		t.Fatal(err)
	}
	request := findRequest(t, transport.Requests(), "skills/list")
	raw, err := json.Marshal(request.Params)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"cwds":["/repo"]`)) || !bytes.Contains(raw, []byte(`"forceReload":true`)) {
		t.Fatalf("skills request = %s", raw)
	}
}

func TestConfigProjectionDropsSecrets(t *testing.T) {
	projection, err := ProjectConfig(map[string]any{
		"mcp_servers":     map[string]any{"private_setting": "fixture-value"},
		"approval_policy": "ask",
		"sandbox_mode":    "workspace-write",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("fixture-value")) || bytes.Contains(raw, []byte("mcp_servers")) || bytes.Contains(raw, []byte("workspace-write")) {
		t.Fatalf("secret or raw config leaked: %s", raw)
	}
}

func TestObserveKeepsOptionalFailuresPartial(t *testing.T) {
	transport := newTranscriptTransport(t, "partial.jsonl")
	observation, err := NewClient(ClientOptions{Transport: transport}).Observe(context.Background(), "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(observation.Methods, []string{"skills/list"}) || !hasObservationDiagnostic(observation.Diagnostics, "HOST_OBSERVATION_PARTIAL") {
		t.Fatalf("observation=%#v", observation)
	}
	if observation.Config.MCPDisposition != "unknown" || observation.Hooks.CWD != "/repo" {
		t.Fatalf("optional projections were not normalized: %#v", observation)
	}
}

func TestObserveRejectsMalformedTranscript(t *testing.T) {
	transport := newTranscriptTransport(t, "malformed.jsonl")
	if _, err := NewClient(ClientOptions{Transport: transport}).Observe(context.Background(), "/repo"); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("error=%v", err)
	}
}

func TestObserveRejectsMalformedRequiredSkillProjection(t *testing.T) {
	transport := newMetadataTransport()
	transport.response = func(request Request) []byte {
		if request.Method == "skills/list" {
			return mustResponse(Response{JSONRPC: "2.0", ID: request.ID, Result: json.RawMessage(`{"data":[{"cwd":"/repo","errors":[],"skills":[{"name":"acme:review","enabled":true,"path":"/plugin/SKILL.md"}]}]}`)})
		}
		return metadataResponse(request)
	}
	if _, err := NewClient(ClientOptions{Transport: transport}).Observe(context.Background(), "/repo"); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("error=%v", err)
	}
}

func TestObserveNormalizesRequiredSkillFailure(t *testing.T) {
	transport := newMetadataTransport()
	transport.response = func(request Request) []byte {
		if request.Method == "skills/list" {
			return mustResponse(Response{JSONRPC: "2.0", ID: request.ID, Error: &RemoteError{Code: -32000, Message: "failed"}})
		}
		return metadataResponse(request)
	}
	if _, err := NewClient(ClientOptions{Transport: transport}).Observe(context.Background(), "/repo"); Code(err) != "HOST_OBSERVATION_FAILED" {
		t.Fatalf("error=%v", err)
	}
}

func TestObserveRedactsMetadataErrorPaths(t *testing.T) {
	transport := newMetadataTransport()
	transport.response = func(request Request) []byte {
		if request.Method == "skills/list" {
			return mustResponse(Response{JSONRPC: "2.0", ID: request.ID, Result: json.RawMessage(`{"data":[{"cwd":"/repo","errors":[{"message":"failed at /Users/example/private","path":"/Users/example/private/SKILL.md"}],"skills":[]}]}`)})
		}
		return metadataResponse(request)
	}
	observation, err := NewClient(ClientOptions{Transport: transport}).Observe(context.Background(), "/repo")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(observation.Skills.Errors)
	if bytes.Contains(raw, []byte("/Users/")) || bytes.Contains(raw, []byte("private")) {
		t.Fatalf("metadata error leaked a path: %s", raw)
	}
}

func TestObserveRejectsNonCanonicalCWDAndCWDReuse(t *testing.T) {
	client := NewClient(ClientOptions{Transport: newMetadataTransport()})
	if _, err := client.Observe(context.Background(), "relative"); Code(err) != "HOST_OBSERVATION_FAILED" {
		t.Fatalf("relative cwd error=%v", err)
	}
	if _, err := client.Observe(context.Background(), "/repo"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Observe(context.Background(), "/other"); Code(err) != "HOST_OBSERVATION_FAILED" {
		t.Fatalf("reused client error=%v", err)
	}
}

func newMetadataTransport() *recordingTransport {
	transport := newRecordingTransport()
	transport.response = metadataResponse
	return transport
}

type transcriptRecord struct {
	Method string          `json:"method"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RemoteError    `json:"error,omitempty"`
}

func newTranscriptTransport(t *testing.T, name string) *recordingTransport {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", "appserver", name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	records := make(map[string]transcriptRecord)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record transcriptRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		if record.Method == "" || records[record.Method].Method != "" {
			t.Fatalf("invalid or duplicate transcript method %q", record.Method)
		}
		records[record.Method] = record
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	transport := newRecordingTransport()
	transport.response = func(request Request) []byte {
		record, found := records[request.Method]
		if !found {
			t.Fatalf("unexpected transcript method %q", request.Method)
		}
		return mustResponse(Response{JSONRPC: "2.0", ID: request.ID, Result: record.Result, Error: record.Error})
	}
	return transport
}

func metadataResponse(request Request) []byte {
	var result json.RawMessage
	switch request.Method {
	case "initialize":
		result = json.RawMessage(`{"userAgent":"codex-cli/0.146.1","codexHome":"/host/.codex","platformFamily":"unix","platformOs":"macos"}`)
	case "skills/list":
		result = json.RawMessage(`{"data":[{"cwd":"/repo","errors":[],"skills":[{"name":"acme:review","enabled":true,"path":"/plugins/acme/skills/review/SKILL.md","scope":"user","description":"ignored"}]}]}`)
	case "hooks/list":
		result = json.RawMessage(`{"data":[{"cwd":"/repo","errors":[],"warnings":[],"hooks":[{"currentHash":"abc","enabled":true,"eventName":"preToolUse","pluginId":"oaw-codex-host","source":"plugin","trustStatus":"trusted","command":"discarded","displayOrder":0,"handlerType":"command","isManaged":false,"key":"oaw","sourcePath":"/private/hook.json","timeoutSec":10}]}]}`)
	case "config/read":
		result = json.RawMessage(`{"config":{"mcp_servers":{"private_setting":"fixture-value"},"features":{"hooks":true},"approval_policy":"ask","sandbox_mode":"workspace-write"},"origins":{}}`)
	default:
		panic("unexpected method " + request.Method)
	}
	return mustResponse(Response{JSONRPC: "2.0", ID: request.ID, Result: result})
}

func findRequest(t *testing.T, requests []Request, method string) Request {
	t.Helper()
	for _, request := range requests {
		if request.Method == method {
			return request
		}
	}
	t.Fatalf("request %q not found", method)
	return Request{}
}

func hasObservationDiagnostic(values []ObservationDiagnostic, code string) bool {
	return slices.ContainsFunc(values, func(value ObservationDiagnostic) bool { return value.Code == code })
}
