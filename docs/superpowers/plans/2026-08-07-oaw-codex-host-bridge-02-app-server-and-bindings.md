# OAW Codex Host Bridge 02: App Server Metadata and Skill Bindings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans in the current session to implement this plan task-by-task. This plan is locked to `CURRENT`; do not dispatch subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Observe the active Codex working directory through the stable App Server metadata API and convert enabled Skills into exact Host Binding Inventory evidence owned by discovered Provider installations.

**Architecture:** `internal/codexbridge/appserver` is a metadata-only JSON-RPC client. It launches `codex app-server --listen stdio://` with the caller's existing environment and working directory, initializes once, and calls only `skills/list`, `hooks/list`, and an allowlisted `config/read`. `internal/codexbridge` then builds existing Host v2 records and matches each Skill path to exactly one discovery Candidate and one Descriptor Binding. No thread, turn, model, process-execution, or experimental `plugin/list` method is reachable through the client.

**Tech Stack:** Go 1.26, `os/exec`, bounded JSONL framing, `context` deadlines, existing discovery/catalog/host/canonical JSON packages, table-driven transcript tests.

---

**Selected execution:** `CURRENT`. Do not dispatch subagents. Do not publish an intermediate phase commit.

**Depends on:** Plan 01 contracts and cache; approved design Sections 9-12.

**Produces:** `MetadataObservation`, `Facts`, and `BuildBindingInventory` used by Plan 03.

## File Map

| Path | Responsibility |
| --- | --- |
| `internal/codexbridge/appserver/records.go` | Versioned metadata request/response projections and allowlisted capability facts. |
| `internal/codexbridge/appserver/errors.go` | App Server-local coded errors that cross the parent Bridge boundary without an import cycle. |
| `internal/codexbridge/appserver/jsonrpc.go` | Bounded JSON-RPC-over-stdio framing, request IDs, initialization, timeouts, and method allowlist. |
| `internal/codexbridge/appserver/client.go` | Metadata client and process launcher abstraction. |
| `internal/codexbridge/appserver/client_test.go` | Complete, partial, malformed, slow, and unsupported transcript tests. |
| `internal/codexbridge/appserver/security_test.go` | Forbidden-method and process/environment inheritance tests. |
| `internal/codexbridge/bindings.go` | Skill-to-Candidate matching, content digests, and Host Binding Inventory construction. |
| `internal/codexbridge/facts.go` | Host Session, Environment Report, Inventory, and OAW resolution fact assembly. |
| `internal/codexbridge/integration.go` | Canonical v1 Codex host-native Manifest constructor; registration and audit remain Plan 04. |
| `internal/codexbridge/bindings_test.go` | Exact path, name, host, ambiguity, and mutation tests. |
| `internal/codexbridge/facts_test.go` | Host v2 pinning and disposition projection tests. |
| `internal/codexbridge/testdata/appserver/*.jsonl` | Checked-in fake App Server request/response transcripts. |

## Locked App Server Projections

Use the Codex 0.146.1 schema observed at `/tmp/oaw-codex-app-schema-20260807`
as the initial fixture baseline. The client must decode only these fields:

```go
type MetadataError struct {
	Message string `json:"message"`
	Path    string `json:"path"`
}

type ObservationDiagnostic struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

func observationDiagnostic(code, detail string) ObservationDiagnostic {
	return ObservationDiagnostic{Code: code, Detail: detail}
}

type SkillMetadata struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
	Scope   string `json:"scope"`
}
type SkillsEntry struct {
	CWD    string          `json:"cwd"`
	Errors []MetadataError `json:"errors"`
	Skills []SkillMetadata `json:"skills"`
}
type HookMetadata struct {
	CurrentHash string `json:"currentHash"`
	Enabled     bool   `json:"enabled"`
	EventName   string `json:"eventName"`
	PluginID    string `json:"pluginId"`
	Source      string `json:"source"`
	TrustStatus string `json:"trustStatus"`
}
type HooksEntry struct {
	CWD      string          `json:"cwd"`
	Errors   []MetadataError `json:"errors"`
	Warnings []string        `json:"warnings"`
	Hooks    []HookMetadata  `json:"hooks"`
}
type MetadataObservation struct {
	Skills       SkillsEntry
	Hooks        HooksEntry
	Config       ConfigProjection
	Methods      []string
	Diagnostics  []ObservationDiagnostic
	CodexVersion string
}
```

`ConfigProjection` contains only booleans and normalized dispositions. It must
not contain the raw effective TOML object, MCP env values, headers, tokens,
credentials, or arbitrary Plugin settings.

## Task 1: Add the metadata client contract and JSON-RPC framing

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `internal/codexbridge/appserver/errors.go`
- Create: `internal/codexbridge/appserver/records.go`
- Create: `internal/codexbridge/appserver/jsonrpc.go`
- Create: `internal/codexbridge/appserver/client.go`
- Create: `internal/codexbridge/appserver/client_test.go`

- [ ] **Step 1: Add the pinned MCP dependency and write RED transcript tests**

Add the official Go MCP SDK only for the later MCP server; the metadata client
itself uses the standard library. The dependency line is:

```text
require github.com/modelcontextprotocol/go-sdk v1.7.0
```

Create a fake duplex transport and assert the exact request sequence:

```go
func TestClientUsesOnlyAllowlistedMetadataMethods(t *testing.T) {
	transport := newTranscriptTransport(t, "initialize", "initialized", "skills/list", "hooks/list", "config/read")
	client := NewClient(ClientOptions{Transport: transport, MaximumMessageBytes: 8 << 20, RequestTimeout: time.Second})
	got, err := client.Observe(context.Background(), "/repo")
	if err != nil { t.Fatal(err) }
	if !slices.Equal(got.Methods, []string{"config/read", "hooks/list", "skills/list"}) { t.Fatalf("methods = %#v", got.Methods) }
}

func TestClientRejectsForbiddenMethodBeforeWriting(t *testing.T) {
	client := NewClient(ClientOptions{Transport: forbiddenRecordingTransport{}})
	if _, err := client.Call(context.Background(), "thread/start", nil); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" { t.Fatalf("error = %v", err) }
}
```

- [ ] **Step 2: Run RED**

```bash
rtk go test ./internal/codexbridge/appserver
```

Expected: FAIL because the package, dependency, and client do not exist.

- [ ] **Step 3: Implement bounded JSON-RPC and initialization**

```go
type Error struct { Code, Detail string; Cause error }

func (err *Error) Error() string { return err.Code + ": " + err.Detail }
func (err *Error) Unwrap() error { return err.Cause }
func (err *Error) ErrorCode() string { return err.Code }
func NewError(code, detail string, cause error) error { return &Error{Code: code, Detail: detail, Cause: cause} }
func Code(err error) string {
	var value interface{ ErrorCode() string }
	if errors.As(err, &value) { return value.ErrorCode() }
	return ""
}

type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RemoteError    `json:"error,omitempty"`
}

type RemoteError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type Transport interface {
	Exchange(context.Context, []byte, int) ([]byte, error)
	Notify(context.Context, []byte) error
	Close() error
}

var allowedMethods = map[string]struct{}{"skills/list": {}, "hooks/list": {}, "config/read": {}}

func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if _, ok := allowedMethods[method]; !ok {
		return nil, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "App Server method is not allowlisted", nil)
	}
	return c.exchange(ctx, method, params)
}

// exchange is the bounded JSON-RPC implementation shared by public metadata
// calls and the private initialize handshake. Only Call performs the public
// method allowlist check; initialize is its sole private caller for v1.
func (c *Client) exchange(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if c.transport == nil {
		return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "App Server transport is not configured", nil)
	}
	id := c.nextID.Add(1)
	encoded, err := json.Marshal(Request{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil || len(encoded) > c.maximumMessageBytes {
		return nil, NewError("HOST_OBSERVATION_FAILED", "App Server request is invalid or oversized", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	raw, err := c.transport.Exchange(requestCtx, encoded, c.maximumMessageBytes)
	if err != nil { return nil, NewError("HOST_OBSERVATION_FAILED", "App Server exchange failed", err) }
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

func normalizeRemoteError(method string, value RemoteError) error {
	if method == "skills/list" && value.Code == -32601 { return NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "required skills/list method is unavailable", nil) }
	return NewError("HOST_OBSERVATION_FAILED", "Codex App Server returned a metadata error", nil)
}
```

Define the client construction and initialization seam explicitly; `initialize`
uses the same bounded exchange implementation as `Call`, but is private and is
never admitted by the public method allowlist:

```go
type ClientOptions struct {
	Transport             Transport
	Launcher              Launcher
	MaximumMessageBytes   int
	RequestTimeout        time.Duration
}

type Client struct {
	transport             Transport
	launcher              Launcher
	nextID                atomic.Uint64
	maximumMessageBytes   int
	requestTimeout        time.Duration
	initialized           bool
	initializedCWD        string
	mu                    sync.Mutex
	userAgent             string
}

func NewClient(options ClientOptions) *Client {
	return &Client{
		transport: options.Transport, launcher: options.Launcher,
		maximumMessageBytes: clampMessageBytes(options.MaximumMessageBytes),
		requestTimeout: clampRequestTimeout(options.RequestTimeout),
	}
}

func (c *Client) initialize(ctx context.Context, cwd string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.initialized {
		if c.initializedCWD != cwd { return NewError("HOST_OBSERVATION_FAILED", "App Server client is bound to another cwd", nil) }
		return nil
	}
	if c.transport == nil {
		if c.launcher == nil { return NewError("HOST_BRIDGE_UNAVAILABLE", "Codex App Server launcher is not configured", nil) }
		transport, err := c.launcher.Open(ctx, cwd)
		if err != nil { return NewError("HOST_OBSERVATION_FAILED", "open Codex App Server", err) }
		c.transport = transport
	}
	result, err := c.exchange(ctx, "initialize", map[string]any{
		"clientInfo": map[string]string{"name": "oaw-codex-bridge", "version": "1.0.0"},
		"capabilities": map[string]bool{"experimentalApi": false},
	})
	if err != nil { return err }
	c.userAgent = projectServerVersion(result)
	if err := c.transport.Notify(ctx, []byte(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)); err != nil {
		return NewError("HOST_OBSERVATION_FAILED", "send App Server initialized notification", err)
	}
	c.initializedCWD = cwd
	c.initialized = true
	return nil
}
```

`exchange` contains the current `Call` body after the method allowlist check;
`Call` invokes it only for `skills/list`, `hooks/list`, or `config/read`, while
`initialize` is the sole private caller allowed to exchange the initialization
request. `clampMessageBytes`, `clampRequestTimeout`, and
`projectServerVersion` are package helpers covered by client tests; malformed
server metadata yields an empty diagnostic version rather than Provider
authority.

`Client` owns `transport Transport`, `nextID atomic.Uint64`,
`maximumMessageBytes int`, and `requestTimeout time.Duration`; construction
clamps the size to 64 KiB through 8 MiB and the timeout to 100 milliseconds
through 30 seconds. `Transport.Exchange` serializes one request under a mutex,
writes exactly one newline-terminated JSON value, and reads exactly one
newline-terminated response with a `bufio.Reader`. It stops as soon as the
configured byte limit is exceeded. A process transport cancels and waits for
the App Server child on context expiry so a blocked pipe goroutine cannot
survive the failed observation. `Notify` writes one bounded notification and
does not read a response.

Use a monotonically increasing request ID and `context.WithTimeout` around
every exchange. `normalizeRemoteError` maps an unsupported allowlisted method
to `HOST_BRIDGE_PROTOCOL_MISMATCH` and every other remote error to
`HOST_OBSERVATION_FAILED`, retaining only the numeric JSON-RPC code and a
bounded, secret-free message. The
initialization request is:

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"oaw-codex-bridge","version":"1.0.0"},"capabilities":{"experimentalApi":false}}}
```

Follow it with the JSON-RPC `initialized` notification. Do not set a private
`HOME`, `CODEX_HOME`, auth token, sandbox override, or approval override on the
child command. `ClientOptions.Launcher` must be injectable for tests.

- [ ] **Step 4: Implement the real launcher without execution fallback**

```go
type Launcher interface { Open(context.Context, string) (Transport, error) }
type CodexLauncher struct { Binary string; Environment []string }

func (l CodexLauncher) Open(ctx context.Context, cwd string) (Transport, error) {
	processCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(processCtx, l.Binary, "app-server", "--listen", "stdio://")
	cmd.Dir = cwd
	if l.Environment == nil { cmd.Env = os.Environ() } else { cmd.Env = append([]string{}, l.Environment...) }
	stdin, err := cmd.StdinPipe(); if err != nil { cancel(); return nil, err }
	stdout, err := cmd.StdoutPipe(); if err != nil { cancel(); return nil, err }
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil { cancel(); return nil, err }
	return newProcessTransport(stdin, stdout, cmd, cancel), nil
}
```

`newProcessTransport` returns the `Transport` implementation described in
Step 3; its `Close` cancels once, closes stdin, waits exactly once, and reports
a non-zero exit as
`HOST_OBSERVATION_FAILED`. Tests must assert the argument list never contains
`thread/start`, `turn/start`, `--dangerously-bypass`, or a private config path.
When `Launcher.Environment` is nil, the production launcher copies
`os.Environ()` exactly; a non-nil test environment is copied defensively. It
never synthesizes `HOME`, `CODEX_HOME`, MCP, Hook, Skill, Plugin, auth, sandbox,
or approval settings.

- [ ] **Step 5: Run client GREEN and race checks**

```bash
rtk gofmt -w internal/codexbridge/appserver
rtk go mod tidy
rtk go test ./internal/codexbridge/appserver
rtk go test -race ./internal/codexbridge/appserver
```

Expected: PASS; `go.mod` contains only the official SDK addition required by
the Bridge and no experimental Codex package.

- [ ] **Step 6: Commit the metadata client**

```bash
rtk git add go.mod go.sum internal/codexbridge/appserver
rtk git commit -m "feat: observe Codex metadata through App Server"
```

## Task 2: Add method projections, redaction, and failure normalization

**Files:**
- Modify: `internal/codexbridge/appserver/client.go`
- Modify: `internal/codexbridge/appserver/records.go`
- Create: `internal/codexbridge/appserver/redact.go`
- Modify: `internal/codexbridge/appserver/client_test.go`
- Create: `internal/codexbridge/appserver/security_test.go`
- Create: `internal/codexbridge/appserver/testdata/appserver/complete.jsonl`
- Create: `internal/codexbridge/appserver/testdata/appserver/partial.jsonl`
- Create: `internal/codexbridge/appserver/testdata/appserver/malformed.jsonl`

- [ ] **Step 1: Write transcript and redaction tests**

```go
func TestObserveSendsExactCWDAndForceReload(t *testing.T) {
	transport := newTranscriptTransport(t, "complete")
	_, err := NewClient(ClientOptions{Transport: transport}).Observe(context.Background(), "/repo")
	if err != nil { t.Fatal(err) }
	request := transport.Request("skills/list")
	if !bytes.Contains(request.Params, []byte(`"cwds":["/repo"]`)) || !bytes.Contains(request.Params, []byte(`"forceReload":true`)) { t.Fatalf("skills request = %s", request.Params) }
}

func TestConfigProjectionDropsSecrets(t *testing.T) {
	projection, err := ProjectConfig(map[string]any{"mcp_servers": map[string]any{"token":"secret"}, "approval_policy":"ask"})
	if err != nil { t.Fatal(err) }
	raw, _ := json.Marshal(projection)
	if bytes.Contains(raw, []byte("secret")) || bytes.Contains(raw, []byte("mcp_servers")) { t.Fatalf("secret leaked: %s", raw) }
}

func TestObserveKeepsOptionalFailuresPartial(t *testing.T) {
	transport := newTranscriptTransport(t, "partial")
	observation, err := NewClient(ClientOptions{Transport: transport}).Observe(context.Background(), "/repo")
	if err != nil { t.Fatal(err) }
	if !slices.Equal(observation.Methods, []string{"skills/list"}) ||
		!hasObservationDiagnostic(observation.Diagnostics, "HOST_OBSERVATION_PARTIAL") {
		t.Fatalf("observation=%#v", observation)
	}
}
```

`hasObservationDiagnostic` is a test-only exact-code matcher defined in
`client_test.go`; the partial transcript returns valid `skills/list` data and
unsupported `hooks/list`/`config/read` responses.

- [ ] **Step 2: Run RED**

```bash
rtk go test ./internal/codexbridge/appserver -run 'CWD|Projection|Transcript'
```

Expected: FAIL until the three projections and fixture loader exist.

- [ ] **Step 3: Implement the three allowlisted calls**

```go
func (c *Client) Observe(ctx context.Context, cwd string) (MetadataObservation, error) {
	if !filepath.IsAbs(cwd) || filepath.Clean(cwd) != cwd || hasControl(cwd) { return MetadataObservation{}, NewError("HOST_OBSERVATION_FAILED", "cwd is not canonical", nil) }
	if err := c.initialize(ctx, cwd); err != nil { return MetadataObservation{}, err }
	skills, err := c.callSkills(ctx, cwd); if err != nil { return MetadataObservation{}, requiredObservationError(err) }
	methods := []string{"skills/list"}
	diagnostics := make([]ObservationDiagnostic, 0, 2)
	hooks, hookErr := c.callHooks(ctx, cwd)
	if hookErr == nil { methods = append(methods, "hooks/list") } else { diagnostics = append(diagnostics, observationDiagnostic("HOST_OBSERVATION_PARTIAL", "hooks/list unavailable")) }
	config, configErr := c.callConfig(ctx, cwd)
	if configErr == nil { methods = append(methods, "config/read") } else { diagnostics = append(diagnostics, observationDiagnostic("HOST_OBSERVATION_PARTIAL", "config/read unavailable")) }
	slices.Sort(methods)
	return MetadataObservation{Skills: skills, Hooks: normalizeHooks(hooks, hookErr), Config: normalizeConfig(config, configErr), Methods: methods, Diagnostics: diagnostics, CodexVersion: c.userAgent}, nil
}
```

`skills/list` failure is required and maps to `HOST_OBSERVATION_FAILED`.
`hooks/list` and `config/read` failure maps to explicit unknown dispositions and
`HOST_OBSERVATION_PARTIAL`; a failed optional call is absent from `Methods` and
has one bounded `MetadataError` entry. Neither failure fabricates a binding. Reject any
response whose `cwd` is not the requested canonical cwd. Never call
`plugin/list`, even when a plugin appears in config.

Define `requiredObservationError`, `normalizeHooks`, `normalizeConfig`, and
`hasControl` in the App Server package. `requiredObservationError` preserves
`HOST_BRIDGE_PROTOCOL_MISMATCH` for a required-method/schema mismatch and maps
all other required failures to `HOST_OBSERVATION_FAILED`. The two normalizers
set every unavailable optional surface to its explicit `unknown` disposition,
copy only bounded arrays, and call `normalizeMetadataErrors`; that helper keeps
the redacted message but drops every raw error path (or replaces it with a
short digest) so an absolute Skill/config path cannot escape this package.

Keep the JSON-RPC envelope closed, but decode method results through the
allowlisted projection records above. Codex may add descriptive Skill fields
such as `description`, `interface`, or `dependencies`; ignore those fields
without retaining them. Authority still derives only from validated `name`,
`enabled`, `path`, and `scope`. Missing or mistyped required projection fields
fail the Skills observation instead of receiving zero values.

- [ ] **Step 4: Implement the redacted configuration/environment projection**

```go
type ConfigProjection struct {
	CWDObserved       bool   `json:"cwd_observed"`
	SandboxDisposition string `json:"sandbox_disposition"`
	MCPDisposition     string `json:"mcp_disposition"`
	HookDisposition    string `json:"hook_disposition"`
	ApprovalDisposition string `json:"approval_disposition"`
}
```

Only the disposition enum values from `internal/execution` may be emitted.
`permission_mode` remains a diagnostic field in the Hook context and is never
copied into `ApprovalPolicyDigest`. `hooks/list` may prove that the OAW plugin
Hook is enabled/trusted by matching its plugin ID, event, and current hash, but
raw commands, source paths, and payloads are discarded.

- [ ] **Step 5: Run GREEN and security checks**

```bash
rtk gofmt -w internal/codexbridge/appserver
rtk go test ./internal/codexbridge/appserver
rtk go test -race ./internal/codexbridge/appserver
rtk go vet ./internal/codexbridge/appserver
```

Expected: PASS; fixtures prove malformed JSON, slow response timeout,
unsupported method, and secret redaction.

- [ ] **Step 6: Commit projections**

```bash
rtk git add internal/codexbridge/appserver
rtk git commit -m "feat: redact Codex metadata observations"
```

## Task 3: Build exact Skill-to-installation Binding Inventory

**Files:**
- Create: `internal/codexbridge/bindings.go`
- Create: `internal/codexbridge/bindings_test.go`
- Modify: `internal/codexbridge/evidence.go`

- [ ] **Step 1: Write failing binding tests**

```go
func TestBuildBindingInventoryMatchesPathAndName(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	metadata := MetadataObservation{Skills: SkillsEntry{CWD: fixture.Home, Skills: []SkillMetadata{{Name: "acme:review", Enabled: true, Path: filepath.Join(fixture.Candidate.Location, "skills/review/SKILL.md"), Scope: "user"}}}}
	inventory, diagnostics, err := BuildBindingInventory(fixture.Catalog, fixture.Discovery, metadata, fixture.Home)
	if err != nil || len(diagnostics) != 0 || len(inventory.Observations) != 1 { t.Fatalf("inventory = %#v diagnostics=%v err=%v", inventory, diagnostics, err) }
}

func TestBuildBindingInventoryRejectsOrphanAmbiguousDisabledAndForeignSkills(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	second := hosttest.CloneProviderCandidate(t, fixture, "second")
	metadata := MetadataObservation{Skills: SkillsEntry{CWD: fixture.Home, Skills: []SkillMetadata{
		{Name: "disabled", Enabled: false, Path: filepath.Join(fixture.Candidate.Location, "skills/review/SKILL.md"), Scope: "user"},
		{Name: "orphan", Enabled: true, Path: filepath.Join(fixture.Home, "unowned", "SKILL.md"), Scope: "user"},
		{Name: "ambiguous", Enabled: true, Path: filepath.Join(fixture.Candidate.Location, "skills/review", "SKILL.md"), Scope: "user"},
		{Name: "foreign", Enabled: true, Path: filepath.Join(second.Location, "skills/other", "SKILL.md"), Scope: "user"},
	}}}
	writeSkillFixture(t, metadata.Skills.Skills[1].Path)
	inventory, diagnostics, err := BuildBindingInventory(fixture.Catalog, fixture.Discovery, metadata, fixture.Home)
	if err != nil { t.Fatal(err) }
	if len(inventory.Observations) != 0 { t.Fatalf("rejected Skills produced observations: %#v", inventory.Observations) }
	if !hasDiagnostic(diagnostics, "HOST_SKILL_ORPHAN") ||
		!hasDiagnostic(diagnostics, "HOST_SKILL_INSTALLATION_AMBIGUOUS") ||
		!hasDiagnostic(diagnostics, "HOST_BINDING_EVIDENCE_REQUIRED") {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestBuildBindingInventoryChangesWhenSkillContentChanges(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	path := filepath.Join(fixture.Candidate.Location, "skills/review/SKILL.md")
	metadata := MetadataObservation{Skills: SkillsEntry{CWD: fixture.Home, Skills: []SkillMetadata{{Name: "acme:review", Enabled: true, Path: path, Scope: "user"}}}}
	first, diagnostics, err := BuildBindingInventory(fixture.Catalog, fixture.Discovery, metadata, fixture.Home)
	if err != nil || len(diagnostics) != 0 || len(first.Observations) != 1 { t.Fatalf("first=%#v diagnostics=%v err=%v", first, diagnostics, err) }
	appendSkillFixture(t, path, "\nchanged")
	second, diagnostics, err := BuildBindingInventory(fixture.Catalog, fixture.Discovery, metadata, fixture.Home)
	if err != nil || len(diagnostics) != 0 || len(second.Observations) != 1 { t.Fatalf("second=%#v diagnostics=%v err=%v", second, diagnostics, err) }
	if first.Observations[0].Digest == second.Observations[0].Digest || first.Digest == second.Digest { t.Fatal("Skill content change did not change evidence digest") }
}

func TestValidateSkillIdentityRejectsMalformedNameAndUnknownScope(t *testing.T) {
	for _, value := range []SkillMetadata{
		{Name: "", Scope: "user"},
		{Name: " leading", Scope: "user"},
		{Name: "bad\nname", Scope: "repo"},
		{Name: "acme:review", Scope: "workspace"},
	} {
		if err := validateSkillIdentity(value.Name, value.Scope); err == nil { t.Fatalf("accepted %#v", value) }
	}
	for _, scope := range []string{"user", "repo", "system", "admin"} {
		if err := validateSkillIdentity("acme:review", scope); err != nil { t.Fatalf("scope %q: %v", scope, err) }
	}
}

func TestCandidatePathRejectsForeignHost(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	candidate := fixture.Candidate
	candidate.HostID = "claude"
	path := filepath.Join(fixture.Candidate.Location, "skills/review/SKILL.md")
	if candidateContainsPath(candidate, "codex", path) { t.Fatal("foreign Host Candidate matched Codex Skill") }
}
```

The test-only helpers `CloneProviderCandidate`, `writeSkillFixture`,
`appendSkillFixture`, and `hasDiagnostic` create isolated temporary files and
are defined in `internal/codexbridge/bindings_test.go`; the clone helper adds a
second Candidate with the same physical root and a distinct installation key
so the containment test is genuinely ambiguous. They never alter the real Host
installation.

- [ ] **Step 2: Run RED**

```bash
rtk go test ./internal/codexbridge -run 'BindingInventory'
```

Expected: FAIL because the builder is absent.

- [ ] **Step 3: Implement the exact matching algorithm**

```go
func BuildBindingInventory(value catalog.Catalog, report discovery.Report, metadata MetadataObservation, cwd string) (host.BindingInventory, []Diagnostic, error) {
	diagnostics := make([]Diagnostic, 0)
	add := func(code, detail string) { diagnostics = append(diagnostics, NewDiagnostic(code, "binding", detail, true)) }
	if report.HostID() != "codex" || metadata.Skills.CWD != cwd {
		inventory, err := host.NewBindingInventory("codex", nil)
		return inventory, append(diagnostics, NewDiagnostic("HOST_OBSERVATION_FAILED", "binding", "Skill metadata CWD does not match the requested canonical CWD", true)), err
	}
	observations := make([]host.BindingObservation, 0)
	for _, entry := range metadata.Skills.Skills {
		if !entry.Enabled { continue }
		if err := validateSkillIdentity(entry.Name, entry.Scope); err != nil { add("HOST_OBSERVATION_FAILED", "enabled Skill has an invalid name or scope"); continue }
		path, err := canonicalSkillPath(entry.Path)
		if err != nil { add("HOST_OBSERVATION_FAILED", "Skill path is not a canonical regular SKILL.md"); continue }
		content, err := os.ReadFile(path)
		if err != nil { add("HOST_OBSERVATION_FAILED", "enabled Skill content could not be read"); continue }
		candidates := candidatesContaining(value, report, path)
		if len(candidates) == 0 { add("HOST_SKILL_ORPHAN", "enabled Skill is outside every discovered Candidate"); continue }
		if len(candidates) != 1 { add("HOST_SKILL_INSTALLATION_AMBIGUOUS", "Skill path belongs to more than one Candidate"); continue }
		candidate := candidates[0]
		bindings := skillBindings(value, candidate.ProviderID, entry.Name)
		if len(bindings) == 0 { add("HOST_BINDING_EVIDENCE_REQUIRED", "no declared Skill binding matches the enabled Skill"); continue }
		for _, binding := range bindings {
			topologies := intersectTopologies(binding.Topologies, []execution.Topology{execution.TopologyCurrent})
			if len(topologies) == 0 { add("HOST_BINDING_TOPOLOGY_UNAVAILABLE", "declared Skill binding does not support CURRENT"); continue }
			record := struct {
				Name, Scope, Path, ContentDigest, InstallationKey, Source string
				Enabled bool
			}{entry.Name, entry.Scope, path, canonicaljson.DigestBytes(content), candidate.InstallationKey, "native-probe", true}
			digest, _, err := canonicaljson.Digest(record)
			if err != nil { return host.BindingInventory{}, diagnostics, NewError("HOST_OBSERVATION_FAILED", "Skill evidence cannot be canonicalized", err) }
			observations = append(observations, host.BindingObservation{
				HostID: candidate.HostID, InstallationKey: candidate.InstallationKey, Binding: binding,
				Topologies: topologies, Source: "native-probe",
				EvidenceReference: "evidence://codex/skills-list/" + digest, Digest: digest,
			})
		}
	}
	inventory, err := host.NewBindingInventory(report.HostID(), observations)
	return inventory, diagnostics, err
}

func canonicalSkillPath(value string) (string, error) {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 4096 ||
		strings.IndexFunc(value, unicode.IsControl) >= 0 || !filepath.IsAbs(value) {
		return "", NewError("HOST_OBSERVATION_FAILED", "invalid absolute Skill path", nil)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(value))
	if err != nil { return "", NewError("HOST_OBSERVATION_FAILED", "resolve Skill path", err) }
	resolved, err = filepath.Abs(resolved)
	if err != nil { return "", NewError("HOST_OBSERVATION_FAILED", "canonicalize Skill path", err) }
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() || filepath.Base(resolved) != "SKILL.md" {
		return "", NewError("HOST_OBSERVATION_FAILED", "Skill path is not a regular SKILL.md", err)
	}
	return filepath.Clean(resolved), nil
}

func candidatesContaining(value catalog.Catalog, report discovery.Report, path string) []discovery.Candidate {
	result := make([]discovery.Candidate, 0)
	for _, provider := range value.Providers() {
		for _, candidate := range report.Candidates(provider.ID) {
			if candidateContainsPath(candidate, report.HostID(), path) { result = append(result, candidate) }
		}
	}
	return result
}

func candidateContainsPath(candidate discovery.Candidate, reportHost, path string) bool {
	if reportHost != "codex" || candidate.HostID != "codex" || candidate.HostID != reportHost { return false }
	root, err := filepath.EvalSymlinks(candidate.Location)
	if err != nil { return false }
	root, err = filepath.Abs(filepath.Clean(root))
	if err != nil { return false }
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

func validateSkillIdentity(name, scope string) error {
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) == 0 || utf8.RuneCountInString(name) > 512 ||
		name != strings.TrimSpace(name) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return NewError("HOST_OBSERVATION_FAILED", "invalid Skill name", nil)
	}
	switch scope {
	case "user", "repo", "system", "admin":
		return nil
	default:
		return NewError("HOST_OBSERVATION_FAILED", "invalid Skill scope", nil)
	}
}

func skillBindings(value catalog.Catalog, providerID, skillName string) []catalog.HostBinding {
	result := make([]catalog.HostBinding, 0)
	for _, provider := range value.Providers() {
		if provider.ID != providerID { continue }
		for _, capability := range provider.Capabilities {
			for _, binding := range capability.HostBindings {
				if binding.Host == "codex" && binding.Kind == "skill" && binding.Reference == skillName {
					if !slices.ContainsFunc(result, func(existing catalog.HostBinding) bool { return existing.Host == binding.Host && existing.Kind == binding.Kind && existing.Reference == binding.Reference }) {
						result = append(result, binding)
					}
				}
			}
		}
	}
	return result
}

func intersectTopologies(left, right []execution.Topology) []execution.Topology {
	result := make([]execution.Topology, 0, len(left))
	for _, topology := range left {
		if slices.Contains(right, topology) && !slices.Contains(result, topology) { result = append(result, topology) }
	}
	return result
}
```

The production implementation must not use the `name`, `scope`, or `path`
strings as authority before validation. Preserve the Skill name byte-for-byte
for exact Descriptor matching, but require valid UTF-8, 1-512 runes, no
leading/trailing whitespace, and no control characters. Accept only the four
Codex 0.146.1 `SkillScope` values `user`, `repo`, `system`, and `admin`; never
coerce an unknown scope. The Candidate filter must require the report,
Candidate, Binding, Inventory, and selected Host to all equal `codex` before
physical containment can contribute authority.

The implementation must not use the `path` string as authority
until it has `filepath.EvalSymlinks`, verifies a regular file named `SKILL.md`,
and proves the physical path is contained by exactly one Candidate `Location`.
Require `utf8.ValidString`, no controls, and a bounded 512-rune Skill name.
Hash a canonical record containing name, scope, enabled state, canonical path
identity, `canonicaljson.DigestBytes(content)`, Candidate Installation Key,
and source `native-probe`. Set `Topologies` to the intersection of the
Descriptor Binding's declared set and `[CURRENT]`; an empty intersection is a
diagnostic, not an observation. Use
`evidence://codex/skills-list/<observation-digest>` as the reference.

- [ ] **Step 4: Add immutable fact assembly and Host v2 validation**

Use the `FactDigests` record defined in Plan 01; do not redefine it in this
package or change its field names. Add `integration.go` with the one canonical
Manifest constructor used by fact assembly and Plan 04 conformance:

```go
const (
	BridgeIntegrationID      = "oaw/codex-host"
	BridgeIntegrationVersion = "1.0.0"
)

func CodexHostManifest() (host.Manifest, error) {
	return host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "1.0.0", HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1},
		BindingKinds: []string{"skill"}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features: []host.Feature{host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
	})
}

func AssembleFacts(context HookContext, metadata MetadataObservation, snapshot config.Snapshot, report discovery.Report, inventory host.BindingInventory, resolution core.ResolutionResult) (Facts, error) {
	rebuilt, err := host.NewBindingInventory(inventory.HostID, inventory.Observations)
	if err != nil || rebuilt.Digest != inventory.Digest { return Facts{}, NewError("HOST_OBSERVATION_FAILED", "Skill inventory is not canonical", err) }
	environment, err := buildCurrentEnvironment(context, metadata)
	if err != nil { return Facts{}, err }
	manifest, err := CodexHostManifest()
	if err != nil { return Facts{}, err }
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{SchemaVersion: host.HostSessionSchemaV2, HostID: "codex", IntegrationID: BridgeIntegrationID, IntegrationVersion: BridgeIntegrationVersion, SessionID: context.SessionID, SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, ProviderInventoryDigest: inventory.Digest, EnvironmentReportDigest: environment.Digest})
	if err != nil { return Facts{}, err }
	return Facts{Session: session, Inventory: inventory, Environment: environment, Configuration: snapshot, Discovery: report, Resolutions: resolution.Report, Registry: resolution.Registry, FactDigests: FactDigests{Session: session.Digest, Inventory: inventory.Digest, Environment: environment.Digest, Configuration: snapshot.Digest(), Discovery: report.Digest(), Resolution: resolution.Report.Digest(), Registry: resolution.Registry.Digest()}}, nil
}
```

`buildCurrentEnvironment` emits one observation per known surface (`skills`,
`hooks`, `mcp`, `sandbox`, `approvals`) with `inherited`, `host-configured`,
or `unknown`; it never guesses from a missing response. Build the resolution
only after passing the inventory pointer to `core.Resolve`.

- [ ] **Step 5: Run all binding and Host leaf tests**

```bash
rtk gofmt -w internal/codexbridge/bindings.go internal/codexbridge/facts.go internal/codexbridge/integration.go internal/codexbridge/*_test.go
rtk go test ./internal/codexbridge ./internal/host ./internal/registry ./internal/discovery
rtk go test -race ./internal/codexbridge -run 'Binding|Facts'
```

Expected: PASS; disabled, missing, ambiguous, orphan, foreign-Host, and
unbound Skills never produce a verified Provider capability.

- [ ] **Step 6: Commit exact binding evidence**

```bash
rtk git add internal/codexbridge/bindings.go internal/codexbridge/facts.go internal/codexbridge/integration.go internal/codexbridge/*_test.go
rtk git commit -m "feat: map Codex Skills to Host binding evidence"
```

## Task 4: Self-review against the approved design

- [ ] **Step 1: Prove forbidden calls and data are absent**

```bash
rtk rg -n 'plugin/list|thread/start|thread/resume|thread/fork|turn/start|turn/steer|dangerously-bypass|MCP.*env|Authorization|token' internal/codexbridge/appserver internal/codexbridge/bindings.go
```

Expected: forbidden method names may appear only in negative tests and must
never appear in the allowlist or launcher arguments; secrets appear only as
test fixture assertions, never in production projections.

- [ ] **Step 2: Run the phase gate**

```bash
rtk go test ./internal/codexbridge/...
rtk go test -race ./internal/codexbridge/...
rtk git diff --check
```

Expected: PASS. Plan 03 may now use `Facts` as the sole source of Host facts.
