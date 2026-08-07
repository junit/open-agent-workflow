package appserver

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestClientRejectsForbiddenMethodBeforeWriting(t *testing.T) {
	transport := &recordingTransport{}
	client := NewClient(ClientOptions{Transport: transport})
	if _, err := client.Call(context.Background(), "thread/start", nil); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("error = %v", err)
	}
	if len(transport.Requests()) != 0 {
		t.Fatalf("forbidden call reached transport: %#v", transport.Requests())
	}
}

func TestClientInitializesOnceAndBindsCWD(t *testing.T) {
	transport := newRecordingTransport()
	client := NewClient(ClientOptions{Transport: transport, RequestTimeout: time.Second})
	if err := client.initialize(context.Background(), "/repo"); err != nil {
		t.Fatal(err)
	}
	if err := client.initialize(context.Background(), "/repo"); err != nil {
		t.Fatal(err)
	}
	if err := client.initialize(context.Background(), "/other"); Code(err) != "HOST_OBSERVATION_FAILED" {
		t.Fatalf("cwd error = %v", err)
	}
	if got := requestMethods(transport.Requests()); !slices.Equal(got, []string{"initialize"}) {
		t.Fatalf("methods = %#v", got)
	}
	if len(transport.Notifications()) != 1 || client.userAgent != "codex-cli/0.146.1" {
		t.Fatalf("notifications=%q userAgent=%q", transport.Notifications(), client.userAgent)
	}
}

func TestClientCallUsesAllowlistAndMonotonicRequestIDs(t *testing.T) {
	transport := newRecordingTransport()
	client := NewClient(ClientOptions{Transport: transport, MaximumMessageBytes: 8 << 20, RequestTimeout: time.Second})
	if err := client.initialize(context.Background(), "/repo"); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"skills/list", "hooks/list", "config/read"} {
		if _, err := client.Call(context.Background(), method, map[string]string{"cwd": "/repo"}); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
	}
	requests := transport.Requests()
	if got := requestMethods(requests); !slices.Equal(got, []string{"initialize", "skills/list", "hooks/list", "config/read"}) {
		t.Fatalf("methods = %#v", got)
	}
	for index, request := range requests {
		if request.ID != uint64(index+1) || request.JSONRPC != "2.0" {
			t.Fatalf("request[%d] = %#v", index, request)
		}
	}
}

func TestClientRejectsMalformedAndMismatchedResponses(t *testing.T) {
	tests := []struct {
		name     string
		response func(Request) []byte
	}{
		{"malformed", func(Request) []byte { return []byte(`{"jsonrpc":`) }},
		{"trailing", func(request Request) []byte {
			return []byte(`{"jsonrpc":"2.0","id":` + jsonNumber(request.ID) + `,"result":{}} {}`)
		}},
		{"wrong-id", func(request Request) []byte {
			return mustResponse(Response{JSONRPC: "2.0", ID: request.ID + 1, Result: json.RawMessage(`{}`)})
		}},
		{"missing-result", func(request Request) []byte { return mustResponse(Response{JSONRPC: "2.0", ID: request.ID}) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transport := newRecordingTransport()
			transport.response = test.response
			client := NewClient(ClientOptions{Transport: transport})
			if _, err := client.Call(context.Background(), "skills/list", nil); Code(err) != "HOST_OBSERVATION_FAILED" {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestClientNormalizesRemoteMethodErrors(t *testing.T) {
	transport := newRecordingTransport()
	transport.response = func(request Request) []byte {
		return mustResponse(Response{JSONRPC: "2.0", ID: request.ID, Error: &RemoteError{Code: -32601, Message: "method missing"}})
	}
	client := NewClient(ClientOptions{Transport: transport})
	if _, err := client.Call(context.Background(), "skills/list", nil); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("required method error = %v", err)
	}
	if _, err := client.Call(context.Background(), "hooks/list", nil); Code(err) != "HOST_OBSERVATION_FAILED" {
		t.Fatalf("optional method error = %v", err)
	}
}

type recordingTransport struct {
	mu            sync.Mutex
	requests      []Request
	notifications [][]byte
	response      func(Request) []byte
}

func newRecordingTransport() *recordingTransport {
	return &recordingTransport{response: func(request Request) []byte {
		result := json.RawMessage(`{}`)
		if request.Method == "initialize" {
			result = json.RawMessage(`{"userAgent":"codex-cli/0.146.1","codexHome":"/host/.codex","platformFamily":"unix","platformOs":"macos"}`)
		}
		return mustResponse(Response{JSONRPC: "2.0", ID: request.ID, Result: result})
	}}
}

func (transport *recordingTransport) Exchange(_ context.Context, raw []byte, _ int) ([]byte, error) {
	var request Request
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, err
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.requests = append(transport.requests, request)
	return append([]byte{}, transport.response(request)...), nil
}

func (transport *recordingTransport) Notify(_ context.Context, raw []byte) error {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.notifications = append(transport.notifications, append([]byte{}, raw...))
	return nil
}

func (transport *recordingTransport) Close() error { return nil }

func (transport *recordingTransport) Requests() []Request {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	return append([]Request{}, transport.requests...)
}

func (transport *recordingTransport) Notifications() [][]byte {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	result := make([][]byte, len(transport.notifications))
	for index := range transport.notifications {
		result[index] = append([]byte{}, transport.notifications[index]...)
	}
	return result
}

func requestMethods(requests []Request) []string {
	result := make([]string, len(requests))
	for index, request := range requests {
		result[index] = request.Method
	}
	return result
}

func mustResponse(response Response) []byte {
	encoded, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}
	return encoded
}

func jsonNumber(value uint64) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
