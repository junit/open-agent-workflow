package appserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

func TestClientAcceptsCodexResponseWithoutJSONRPCVersion(t *testing.T) {
	transport := newRecordingTransport()
	transport.response = func(request Request) []byte {
		return []byte(`{"id":` + jsonNumber(request.ID) + `,"result":{"data":[]}}`)
	}
	client := NewClient(ClientOptions{Transport: transport})
	result, err := client.Call(context.Background(), "skills/list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != `{"data":[]}` {
		t.Fatalf("result = %s", result)
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
		{"wrong-version", func(request Request) []byte {
			return mustResponse(Response{JSONRPC: "1.0", ID: request.ID, Result: json.RawMessage(`{}`)})
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

func TestClientBoundsSlowMetadataResponse(t *testing.T) {
	client := NewClient(ClientOptions{Transport: blockingTransport{}, RequestTimeout: 100 * time.Millisecond})
	started := time.Now()
	if _, err := client.Call(context.Background(), "skills/list", nil); Code(err) != "HOST_OBSERVATION_FAILED" {
		t.Fatalf("error=%v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("slow response exceeded bounded timeout: %s", elapsed)
	}
}

func TestClientCloseReleasesTransportOnce(t *testing.T) {
	transport := newRecordingTransport()
	client := NewClient(ClientOptions{Transport: transport})
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	transport.mu.Lock()
	closed := transport.closed
	transport.mu.Unlock()
	if closed != 1 {
		t.Fatalf("transport close count = %d", closed)
	}
}

func TestProcessTransportExchangesAndNotifiesWithBoundedJSONLines(t *testing.T) {
	var exchangeInput bytes.Buffer
	exchange := newProcessTransport(nopWriteCloser{Writer: &exchangeInput}, bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"result":{}}`+"\n"), nil, func() {})
	response, err := exchange.Exchange(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"skills/list"}`), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if string(response) != `{"jsonrpc":"2.0","id":1,"result":{}}` || exchangeInput.Bytes()[len(exchangeInput.Bytes())-1] != '\n' {
		t.Fatalf("response=%s request=%q", response, exchangeInput.Bytes())
	}

	var notifyInput bytes.Buffer
	notify := newProcessTransport(nopWriteCloser{Writer: &notifyInput}, bytes.NewReader(nil), nil, func() {})
	if err := notify.Notify(context.Background(), []byte(`{"jsonrpc":"2.0","method":"initialized"}`)); err != nil {
		t.Fatal(err)
	}
	if notifyInput.Bytes()[len(notifyInput.Bytes())-1] != '\n' {
		t.Fatalf("notification=%q", notifyInput.Bytes())
	}
}

func TestProcessTransportSkipsInterleavedNotifications(t *testing.T) {
	stdout := bytes.NewBufferString(
		`{"jsonrpc":"2.0","method":"remoteControl/status/changed","params":{}}` +
			"\n" +
			`{"id":1,"result":{"ok":true}}` +
			"\n",
	)
	transport := newProcessTransport(nopWriteCloser{Writer: &bytes.Buffer{}}, stdout, nil, func() {})
	response, err := transport.Exchange(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"skills/list"}`), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if string(response) != `{"id":1,"result":{"ok":true}}` {
		t.Fatalf("response = %s", response)
	}
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

type blockingTransport struct{}

func (blockingTransport) Exchange(ctx context.Context, _ []byte, _ int) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (blockingTransport) Notify(context.Context, []byte) error { return nil }
func (blockingTransport) Close() error                         { return nil }

type recordingTransport struct {
	mu            sync.Mutex
	requests      []Request
	notifications [][]byte
	response      func(Request) []byte
	closed        int
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

func (transport *recordingTransport) Close() error {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.closed++
	return nil
}

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
