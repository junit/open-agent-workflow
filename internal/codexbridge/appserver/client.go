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
	"path/filepath"
	"slices"
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

func (client *Client) Close() error {
	client.mu.Lock()
	transport := client.transport
	client.transport = nil
	client.initialized = false
	client.initializedCWD = ""
	client.mu.Unlock()
	if transport == nil {
		return nil
	}
	return transport.Close()
}

func (client *Client) Observe(ctx context.Context, cwd string) (MetadataObservation, error) {
	if !filepath.IsAbs(cwd) || filepath.Clean(cwd) != cwd || hasControl(cwd) {
		return MetadataObservation{}, NewError("HOST_OBSERVATION_FAILED", "cwd is not canonical", nil)
	}
	if err := client.initialize(ctx, cwd); err != nil {
		return MetadataObservation{}, err
	}
	skills, err := client.callSkills(ctx, cwd)
	if err != nil {
		return MetadataObservation{}, requiredObservationError(err)
	}

	methods := []string{"skills/list"}
	diagnostics := make([]ObservationDiagnostic, 0, 2)
	hooks, hookErr := client.callHooks(ctx, cwd)
	if hookErr == nil {
		methods = append(methods, "hooks/list")
	} else {
		diagnostics = append(diagnostics, observationDiagnostic("HOST_OBSERVATION_PARTIAL", "hooks/list unavailable"))
	}
	config, configErr := client.callConfig(ctx, cwd)
	if configErr == nil {
		methods = append(methods, "config/read")
	} else {
		diagnostics = append(diagnostics, observationDiagnostic("HOST_OBSERVATION_PARTIAL", "config/read unavailable"))
	}
	slices.Sort(methods)
	return MetadataObservation{
		Skills:       skills,
		Hooks:        normalizeHooks(hooks, hookErr, cwd),
		Config:       normalizeConfig(config, configErr),
		Methods:      methods,
		Diagnostics:  diagnostics,
		CodexVersion: client.userAgent,
	}, nil
}

func (client *Client) callSkills(ctx context.Context, cwd string) (SkillsEntry, error) {
	raw, err := client.Call(ctx, "skills/list", map[string]any{"cwds": []string{cwd}, "forceReload": true})
	if err != nil {
		return SkillsEntry{}, err
	}
	var response skillsResultWire
	if err := json.Unmarshal(raw, &response); err != nil || response.Data == nil || len(*response.Data) != 1 {
		return SkillsEntry{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "skills/list projection is malformed", err)
	}
	entry := (*response.Data)[0]
	if entry.CWD == nil || entry.Errors == nil || entry.Skills == nil || *entry.CWD != cwd {
		return SkillsEntry{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "skills/list projection does not match cwd", nil)
	}
	if len(*entry.Skills) > maximumMetadataEntries {
		return SkillsEntry{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "skills/list projection is oversized", nil)
	}
	errors, err := normalizeMetadataErrors(*entry.Errors)
	if err != nil {
		return SkillsEntry{}, err
	}
	skills := make([]SkillMetadata, 0, len(*entry.Skills))
	for _, value := range *entry.Skills {
		if value.Name == nil || value.Enabled == nil || value.Path == nil || value.Scope == nil ||
			!validMetadataText(*value.Name, 512) || !filepath.IsAbs(*value.Path) || filepath.Clean(*value.Path) != *value.Path ||
			hasControl(*value.Path) || !validSkillScope(*value.Scope) {
			return SkillsEntry{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "skills/list contains an invalid Skill projection", nil)
		}
		skills = append(skills, SkillMetadata{Name: *value.Name, Enabled: *value.Enabled, Path: *value.Path, Scope: *value.Scope})
	}
	return SkillsEntry{CWD: cwd, Errors: errors, Skills: skills}, nil
}

func (client *Client) callHooks(ctx context.Context, cwd string) (HooksEntry, error) {
	raw, err := client.Call(ctx, "hooks/list", map[string]any{"cwds": []string{cwd}})
	if err != nil {
		return HooksEntry{}, err
	}
	var response hooksResultWire
	if err := json.Unmarshal(raw, &response); err != nil || response.Data == nil || len(*response.Data) != 1 {
		return HooksEntry{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "hooks/list projection is malformed", err)
	}
	entry := (*response.Data)[0]
	if entry.CWD == nil || entry.Errors == nil || entry.Warnings == nil || entry.Hooks == nil || *entry.CWD != cwd {
		return HooksEntry{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "hooks/list projection does not match cwd", nil)
	}
	if len(*entry.Hooks) > maximumMetadataEntries {
		return HooksEntry{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "hooks/list projection is oversized", nil)
	}
	errors, err := normalizeMetadataErrors(*entry.Errors)
	if err != nil {
		return HooksEntry{}, err
	}
	warnings, err := normalizeWarnings(*entry.Warnings)
	if err != nil {
		return HooksEntry{}, err
	}
	hooks := make([]HookMetadata, 0, len(*entry.Hooks))
	for _, value := range *entry.Hooks {
		if value.CurrentHash == nil || value.Enabled == nil || value.EventName == nil || value.Source == nil || value.TrustStatus == nil ||
			!validOptionalMetadataText(*value.CurrentHash, 512) || !validHookEvent(*value.EventName) || !validHookSource(*value.Source) ||
			!validHookTrust(*value.TrustStatus) || value.PluginID != nil && !validOptionalMetadataText(*value.PluginID, 512) {
			return HooksEntry{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "hooks/list contains an invalid Hook projection", nil)
		}
		pluginID := ""
		if value.PluginID != nil {
			pluginID = *value.PluginID
		}
		hooks = append(hooks, HookMetadata{CurrentHash: *value.CurrentHash, Enabled: *value.Enabled, EventName: *value.EventName, PluginID: pluginID, Source: *value.Source, TrustStatus: *value.TrustStatus})
	}
	return HooksEntry{CWD: cwd, Errors: errors, Warnings: warnings, Hooks: hooks}, nil
}

func (client *Client) callConfig(ctx context.Context, cwd string) (ConfigProjection, error) {
	raw, err := client.Call(ctx, "config/read", map[string]any{"cwd": cwd, "includeLayers": false})
	if err != nil {
		return ConfigProjection{}, err
	}
	var response configResultWire
	if err := json.Unmarshal(raw, &response); err != nil || response.Config == nil || response.Origins == nil {
		return ConfigProjection{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "config/read projection is malformed", err)
	}
	var config map[string]any
	if err := json.Unmarshal(*response.Config, &config); err != nil || config == nil {
		return ConfigProjection{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "config/read config is malformed", err)
	}
	projection, err := ProjectConfig(config)
	if err != nil {
		return ConfigProjection{}, err
	}
	projection.CWDObserved = true
	return projection, nil
}

func requiredObservationError(err error) error {
	if Code(err) == "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		return err
	}
	return NewError("HOST_OBSERVATION_FAILED", "required skills/list observation failed", err)
}

func normalizeHooks(value HooksEntry, err error, cwd string) HooksEntry {
	if err == nil {
		return value
	}
	return HooksEntry{CWD: cwd, Errors: optionalMetadataFailure("hooks/list"), Warnings: []string{}, Hooks: []HookMetadata{}}
}

func normalizeConfig(value ConfigProjection, err error) ConfigProjection {
	if err == nil {
		return value
	}
	return unknownConfigProjection()
}

func validSkillScope(value string) bool {
	return slices.Contains([]string{"admin", "repo", "system", "user"}, value)
}

func validHookEvent(value string) bool {
	return slices.Contains([]string{"permissionRequest", "postCompact", "postToolUse", "preCompact", "preToolUse", "sessionEnd", "sessionStart", "stop", "subagentStart", "subagentStop", "userPromptSubmit"}, value)
}

func validHookSource(value string) bool {
	return slices.Contains([]string{"cloudManagedConfig", "cloudRequirements", "legacyManagedConfigFile", "legacyManagedConfigMdm", "mdm", "plugin", "project", "sessionFlags", "system", "unknown", "user"}, value)
}

func validHookTrust(value string) bool {
	return slices.Contains([]string{"managed", "modified", "trusted", "untrusted"}, value)
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
	// Codex 0.146.1 accepts JSON-RPC requests but omits the version field on
	// stdio responses. Preserve strict ID matching and reject conflicting
	// non-empty versions instead of accepting arbitrary response envelopes.
	if (response.JSONRPC != "" && response.JSONRPC != "2.0") || response.ID != id {
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
