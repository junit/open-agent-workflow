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

func TestClientObservesOnlyCurrentSkillMetadata(t *testing.T) {
	transport := newTranscriptTransport(t, "complete.jsonl")
	client := NewClient(ClientOptions{Transport: transport, MaximumMessageBytes: 8 << 20, RequestTimeout: time.Second})
	got, err := client.Observe(context.Background(), "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if requested := requestMethods(transport.Requests()); !slices.Equal(requested, []string{"initialize", "skills/list"}) {
		t.Fatalf("requested methods = %#v", requested)
	}
	if got.CodexVersion != "codex-cli/0.146.1" || got.Skills.CWD != "/repo" || len(got.Skills.Skills) != 1 {
		t.Fatalf("observation = %#v", got)
	}
	observationRaw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, inferred := range [][]byte{[]byte(`"hooks"`), []byte(`"config"`), []byte(`"roles"`), []byte(`"agents"`), []byte(`"delegation"`), []byte(`"tools"`)} {
		if bytes.Contains(observationRaw, inferred) {
			t.Fatalf("stable metadata projection inferred unsupported surface %s: %s", inferred, observationRaw)
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
