package appserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	minimumMessageBytes = 64 << 10
	maximumMessageBytes = 8 << 20
	minimumTimeout      = 100 * time.Millisecond
	maximumTimeout      = 30 * time.Second
)

var allowedMethods = map[string]struct{}{
	"skills/list": {},
	"hooks/list":  {},
	"config/read": {},
}

type ClientOptions struct {
	Transport           Transport
	Launcher            Launcher
	MaximumMessageBytes int
	RequestTimeout      time.Duration
}

type Client struct {
	transport           Transport
	launcher            Launcher
	nextID              atomic.Uint64
	maximumMessageBytes int
	requestTimeout      time.Duration
	initialized         bool
	initializedCWD      string
	mu                  sync.Mutex
	userAgent           string
}

func NewClient(options ClientOptions) *Client {
	return &Client{
		transport:           options.Transport,
		launcher:            options.Launcher,
		maximumMessageBytes: clampMessageBytes(options.MaximumMessageBytes),
		requestTimeout:      clampRequestTimeout(options.RequestTimeout),
	}
}

func (client *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if _, allowed := allowedMethods[method]; !allowed {
		return nil, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "App Server method is not allowlisted", nil)
	}
	return client.exchange(ctx, method, params)
}

func (client *Client) exchange(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if client.transport == nil {
		return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "App Server transport is not configured", nil)
	}
	id := client.nextID.Add(1)
	encoded, err := json.Marshal(Request{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil || len(encoded) > client.maximumMessageBytes {
		return nil, NewError("HOST_OBSERVATION_FAILED", "App Server request is invalid or oversized", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, client.requestTimeout)
	defer cancel()
	raw, err := client.transport.Exchange(requestCtx, encoded, client.maximumMessageBytes)
	if err != nil {
		return nil, NewError("HOST_OBSERVATION_FAILED", "App Server exchange failed", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var response Response
	if err := decoder.Decode(&response); err != nil {
		return nil, NewError("HOST_OBSERVATION_FAILED", "App Server response is malformed", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, NewError("HOST_OBSERVATION_FAILED", "App Server response has trailing JSON", err)
	}
	if response.JSONRPC != "2.0" || response.ID != id {
		return nil, NewError("HOST_OBSERVATION_FAILED", "App Server response ID does not match the request", nil)
	}
	if response.Error != nil {
		return nil, normalizeRemoteError(method, *response.Error)
	}
	if len(response.Result) == 0 {
		return nil, NewError("HOST_OBSERVATION_FAILED", "App Server response has no result", nil)
	}
	return append(json.RawMessage{}, response.Result...), nil
}

func (client *Client) initialize(ctx context.Context, cwd string) error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.initialized {
		if client.initializedCWD != cwd {
			return NewError("HOST_OBSERVATION_FAILED", "App Server client is bound to another cwd", nil)
		}
		return nil
	}
	if client.transport == nil {
		if client.launcher == nil {
			return NewError("HOST_BRIDGE_UNAVAILABLE", "Codex App Server launcher is not configured", nil)
		}
		transport, err := client.launcher.Open(ctx, cwd)
		if err != nil {
			return NewError("HOST_OBSERVATION_FAILED", "open Codex App Server", err)
		}
		client.transport = transport
	}
	result, err := client.exchange(ctx, "initialize", map[string]any{
		"clientInfo":   map[string]string{"name": "oaw-codex-bridge", "version": "1.0.0"},
		"capabilities": map[string]bool{"experimentalApi": false},
	})
	if err != nil {
		return err
	}
	client.userAgent = projectServerVersion(result)
	if err := client.transport.Notify(ctx, []byte(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)); err != nil {
		return NewError("HOST_OBSERVATION_FAILED", "send App Server initialized notification", err)
	}
	client.initializedCWD = cwd
	client.initialized = true
	return nil
}

func normalizeRemoteError(method string, value RemoteError) error {
	if method == "skills/list" && value.Code == -32601 {
		return NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "required skills/list method is unavailable", nil)
	}
	return NewError("HOST_OBSERVATION_FAILED", fmt.Sprintf("Codex App Server returned metadata error %d", value.Code), nil)
}

func projectServerVersion(raw json.RawMessage) string {
	var value struct {
		UserAgent string `json:"userAgent"`
	}
	if err := json.Unmarshal(raw, &value); err != nil || !validServerText(value.UserAgent, 256) {
		return ""
	}
	return value.UserAgent
}

func validServerText(value string, maximum int) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum &&
		strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}

func clampMessageBytes(value int) int {
	if value < minimumMessageBytes {
		return minimumMessageBytes
	}
	if value > maximumMessageBytes {
		return maximumMessageBytes
	}
	return value
}

func clampRequestTimeout(value time.Duration) time.Duration {
	if value < minimumTimeout {
		return minimumTimeout
	}
	if value > maximumTimeout {
		return maximumTimeout
	}
	return value
}

type Launcher interface {
	Open(context.Context, string) (Transport, error)
}

type CodexLauncher struct {
	Binary      string
	Environment []string
}

func (launcher CodexLauncher) Open(ctx context.Context, cwd string) (Transport, error) {
	processCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(processCtx, launcher.Binary, "app-server", "--listen", "stdio://")
	cmd.Dir = cwd
	if launcher.Environment == nil {
		cmd.Env = append([]string{}, os.Environ()...)
	} else {
		cmd.Env = append([]string{}, launcher.Environment...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	return newProcessTransport(stdin, stdout, cmd, cancel), nil
}

type processTransport struct {
	ioMu      sync.Mutex
	stdin     io.WriteCloser
	stdout    *bufio.Reader
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	closeOnce sync.Once
	closeErr  error
}

func newProcessTransport(stdin io.WriteCloser, stdout io.Reader, cmd *exec.Cmd, cancel context.CancelFunc) *processTransport {
	return &processTransport{stdin: stdin, stdout: bufio.NewReader(stdout), cmd: cmd, cancel: cancel}
}

func (transport *processTransport) Exchange(ctx context.Context, request []byte, maximum int) ([]byte, error) {
	type result struct {
		value []byte
		err   error
	}
	completed := make(chan result, 1)
	go func() {
		transport.ioMu.Lock()
		defer transport.ioMu.Unlock()
		if err := writeJSONLine(transport.stdin, request, maximum); err != nil {
			completed <- result{err: err}
			return
		}
		value, err := readJSONLine(transport.stdout, maximum)
		completed <- result{value: value, err: err}
	}()
	select {
	case output := <-completed:
		return output.value, output.err
	case <-ctx.Done():
		_ = transport.Close()
		<-completed
		return nil, ctx.Err()
	}
}

func (transport *processTransport) Notify(ctx context.Context, notification []byte) error {
	completed := make(chan error, 1)
	go func() {
		transport.ioMu.Lock()
		defer transport.ioMu.Unlock()
		completed <- writeJSONLine(transport.stdin, notification, maximumMessageBytes)
	}()
	select {
	case err := <-completed:
		return err
	case <-ctx.Done():
		_ = transport.Close()
		<-completed
		return ctx.Err()
	}
}

func (transport *processTransport) Close() error {
	transport.closeOnce.Do(func() {
		transport.cancel()
		_ = transport.stdin.Close()
		if err := transport.cmd.Wait(); err != nil {
			transport.closeErr = NewError("HOST_OBSERVATION_FAILED", "Codex App Server exited unsuccessfully", err)
		}
	})
	return transport.closeErr
}

func writeJSONLine(writer io.Writer, value []byte, maximum int) error {
	if len(value) == 0 || len(value)+1 > maximum {
		return NewError("HOST_OBSERVATION_FAILED", "App Server message is empty or oversized", nil)
	}
	line := make([]byte, len(value)+1)
	copy(line, value)
	line[len(value)] = '\n'
	_, err := writer.Write(line)
	return err
}

func readJSONLine(reader *bufio.Reader, maximum int) ([]byte, error) {
	line := make([]byte, 0, min(maximum, 4096))
	for {
		part, err := reader.ReadSlice('\n')
		if len(line)+len(part) > maximum {
			return nil, NewError("HOST_OBSERVATION_FAILED", "App Server response is oversized", nil)
		}
		line = append(line, part...)
		if err == nil {
			return append([]byte{}, bytes.TrimSuffix(line, []byte{'\n'})...), nil
		}
		if err != bufio.ErrBufferFull {
			return nil, err
		}
	}
}
