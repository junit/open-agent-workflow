# OAW Codex Host Bridge 01: Contracts and Handle Cache Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans in the current session to implement this plan task-by-task. This plan is locked to `CURRENT`; do not dispatch subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Define the private Codex Bridge protocol, strict Hook context exchange, immutable Host evidence container, and short-lived session-bound handle cache without starting or emulating an Agent.

**Architecture:** Keep private Bridge records under `internal/codexbridge` and the Hook wire adapter under `internal/codexbridge/hook`. The cache owns Host facts in memory and returns only an opaque handle; all records are validated through the existing `internal/host` constructors before they enter the cache. No file, daemon, shared key, private HOME, or process fallback is introduced.

**Tech Stack:** Go 1.26, standard `crypto/rand`, `container/list`, `encoding/json`, existing canonical JSON and Host v2 records, table-driven tests, race detector.

---

**Selected execution:** `CURRENT`. Do not dispatch subagents. Do not publish an intermediate phase commit.

**Depends on:** Approved design `docs/superpowers/specs/2026-08-07-oaw-codex-host-bridge-design.md`; existing `internal/host`, `internal/execution`, and `internal/canonicaljson` contracts.

**Produces:** The `HostEvidenceHandle` and `EvidenceStore` APIs consumed by Plans 02 and 03.

## File Map

| Path | Responsibility |
| --- | --- |
| `internal/codexbridge/protocol.go` | Private protocol versions, operation names, Hook context, public request envelopes, and stable diagnostic codes. |
| `internal/codexbridge/evidence.go` | Immutable cached Host facts and defensive cloning. |
| `internal/codexbridge/cache.go` | Cryptographically random handle issuance, session/cwd binding, TTL, bounded LRU, and reset behavior. |
| `internal/codexbridge/diagnostics.go` | Layered Bridge error type and secret-free diagnostic projection. |
| `internal/codexbridge/hook/input.go` | Strict Codex `PreToolUse` input decoder and reserved context extraction. |
| `internal/codexbridge/hook/output.go` | Exact nested Hook decision/output encoder; observation returns `allow` plus `updatedInput`, a later-operation mismatch returns `deny`, and a valid later operation returns no output. |
| `internal/codexbridge/protocol_test.go` | Protocol version, operation, and input-boundary tests. |
| `internal/codexbridge/cache_test.go` | Entropy, TTL, LRU, reset, and cross-context tests. |
| `internal/codexbridge/hook/input_test.go` | Hook parsing, tool matcher, context injection, and denial tests. |
| `internal/codexbridge/diagnostics_test.go` | Stable error code and redaction tests. |

## Locked Records

Create these records exactly; later plans must import them rather than redefine them:

```go
package codexbridge

const (
	BridgeProtocolVersion = "oaw.codex-bridge/v1"
	HookContextSchemaV1   = "oaw.codex-hook-context/v1"
	EvidenceHandleVersion = "oaw.host-evidence-handle/v1"
)

type Operation string

const (
	OperationObserveCurrent   Operation = "observe_current"
	OperationCoreInspect      Operation = "core.inspect"
	OperationCoreCompile      Operation = "core.compile"
	OperationWorkflowExchange Operation = "workflow_exchange"
)

type HookContext struct {
	SchemaVersion         string `json:"schema_version"`
	BridgeProtocolVersion string `json:"bridge_protocol_version"`
	SessionID             string `json:"session_id"`
	TurnID                string `json:"turn_id"`
	ToolUseID             string `json:"tool_use_id"`
	CWD                   string `json:"cwd"`
	Model                 string `json:"model"`
	PermissionMode        string `json:"permission_mode"`
}

type HostEvidenceHandle struct {
	Version       string `json:"version"`
	SessionDigest string `json:"session_digest"`
	CWDDigest     string `json:"cwd_digest"`
	Token         string `json:"token"`
}

type FactDigests struct {
	Session       string `json:"session"`
	Inventory     string `json:"inventory"`
	Environment   string `json:"environment"`
	Configuration string `json:"configuration"`
	Discovery     string `json:"discovery"`
	Resolution    string `json:"resolution"`
	Registry      string `json:"registry"`
}

type OperationRequest struct {
	Handle HostEvidenceHandle `json:"host_evidence_handle"`
}
```

`Token` contains at least 16 random bytes encoded with unpadded base64url. The
external JSON never contains a Session Snapshot, Binding Inventory,
Environment Report, absolute Skill path, credential, or model transcript.

## Task 1: Establish protocol records and error codes

**Files:**
- Create: `internal/codexbridge/protocol.go`
- Create: `internal/codexbridge/diagnostics.go`
- Create: `internal/codexbridge/protocol_test.go`
- Create: `internal/codexbridge/diagnostics_test.go`

- [ ] **Step 1: Write the failing protocol tests**

```go
func TestProtocolRejectsUnknownOperation(t *testing.T) {
	if _, err := ParseOperation("plugin/list"); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("ParseOperation() error = %v", err)
	}
}

func TestDiagnosticProjectionNeverIncludesHandleOrAbsolutePath(t *testing.T) {
	err := NewError("HOST_EVIDENCE_HANDLE_INVALID", "edited handle", nil)
	value := ProjectDiagnostic(err, "codex", true)
	if strings.Contains(value.Detail, "token") || strings.Contains(value.Detail, "/Users/") {
		t.Fatalf("diagnostic leaked private data: %#v", value)
	}
}
```

- [ ] **Step 2: Run the tests to verify RED**

```bash
rtk go test ./internal/codexbridge -run 'Protocol|Diagnostic'
```

Expected: FAIL because the package and constructors do not exist.

- [ ] **Step 3: Implement the exact closed operation set and layered error**

```go
type Error struct {
	Code   string
	Layer  string
	Detail string
	Cause  error
}

type Diagnostic struct {
	Code                     string   `json:"code"`
	Layer                    string   `json:"layer"`
	Detail                   string   `json:"detail"`
	AffectedProviders        []string `json:"affected_providers"`
	AffectedProfiles         []string `json:"affected_profiles"`
	DirectAvailable          bool     `json:"direct_available"`
	RecoverableByObservation bool     `json:"recoverable_by_observation"`
	RecoveryAction           string   `json:"recovery_action"`
	EvidenceDigest           string   `json:"evidence_digest"`
}

func (e *Error) Error() string {
	if e.Detail == "" { return e.Code }
	return e.Code + ": " + e.Detail
}
func (e *Error) Unwrap() error { return e.Cause }
func NewError(code, detail string, cause error) error {
	return &Error{Code: code, Layer: layerForCode(code), Detail: detail, Cause: cause}
}
func Code(err error) string {
	var value *Error
	if errors.As(err, &value) { return value.Code }
	var external interface{ ErrorCode() string }
	if errors.As(err, &external) { return external.ErrorCode() }
	return ""
}
func ParseOperation(value string) (Operation, error) {
	operation := Operation(value)
	switch operation {
	case OperationObserveCurrent, OperationCoreInspect, OperationCoreCompile, OperationWorkflowExchange:
		return operation, nil
	default:
		return "", NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "operation is not in the v1 allowlist", nil)
	}
}
```

Implement the remaining diagnostics helpers in `diagnostics.go`:

The file imports `errors` and `strings` from the standard library in addition
to the imports required by the protocol records.

```go
func layerForCode(code string) string {
	switch {
	case strings.HasPrefix(code, "HOST_BRIDGE_"):
		return "bridge"
	case strings.HasPrefix(code, "HOST_EVIDENCE_"):
		return "evidence"
	default:
		return "downstream"
	}
}

func ProjectDiagnostic(err error, _ string, directAvailable bool) Diagnostic {
	code := Code(err)
	layer := layerForCode(code)
	detail := "Bridge operation failed"
	var value *Error
	if errors.As(err, &value) {
		layer, detail = value.Layer, value.Detail
	}
	return NewDiagnostic(code, layer, detail, directAvailable)
}

func NewDiagnostic(code, layer, detail string, directAvailable bool) Diagnostic {
	return Diagnostic{
		Code: code, Layer: layer, Detail: redactDiagnosticDetail(detail),
		AffectedProviders: []string{}, AffectedProfiles: []string{},
		DirectAvailable: directAvailable, RecoverableByObservation: recoverableByObservation(code),
		RecoveryAction: recoveryAction(code), EvidenceDigest: "",
	}
}

func redactDiagnosticDetail(value string) string {
	for _, marker := range []string{"oawh1.", "/Users/", "/home/", "token", "credential", "Authorization"} {
		if strings.Contains(value, marker) {
			return "Bridge operation failed; inspect the stable diagnostic code"
		}
	}
	return value
}

func recoverableByObservation(code string) bool {
	switch code {
	case "HOST_BRIDGE_CONTEXT_REQUIRED", "HOST_EVIDENCE_HANDLE_REQUIRED", "HOST_EVIDENCE_HANDLE_INVALID", "HOST_EVIDENCE_EXPIRED", "HOST_EVIDENCE_SESSION_MISMATCH", "HOST_OBSERVATION_PARTIAL", "HOST_SESSION_CHANGED":
		return true
	default:
		return false
	}
}

func recoveryAction(code string) string {
	switch code {
	case "HOST_BRIDGE_CONTEXT_REQUIRED":
		return "review and trust the OAW PreToolUse Hook, then observe again"
	case "HOST_EVIDENCE_HANDLE_REQUIRED", "HOST_EVIDENCE_HANDLE_INVALID", "HOST_EVIDENCE_EXPIRED", "HOST_EVIDENCE_SESSION_MISMATCH":
		return "call observe_current in the active Codex session"
	case "HOST_OBSERVATION_PARTIAL":
		return "retain unknown environment dispositions and continue only within verified scope"
	case "HOST_SESSION_CHANGED":
		return "pause the Workflow and perform the legal recovery or switching transition"
	default:
		return "inspect the stable diagnostic code and update the Bridge bundle if required"
	}
}
```

`ProjectDiagnostic` and `NewDiagnostic` always return the complete secret-free
diagnostic shape required by the design; packages must not construct a partial
public diagnostic literal. `EvidenceDigest` is filled by the service when a
current fact set exists; it remains empty for pre-observation errors. Do not
add a generic passthrough operation or a compatibility decoder.

- [ ] **Step 4: Run formatting and GREEN checks**

```bash
rtk gofmt -w internal/codexbridge/protocol.go internal/codexbridge/diagnostics.go internal/codexbridge/*_test.go
rtk go test ./internal/codexbridge -run 'Protocol|Diagnostic'
rtk go vet ./internal/codexbridge
```

Expected: PASS.

- [ ] **Step 5: Commit the protocol seam**

```bash
rtk git add internal/codexbridge/protocol.go internal/codexbridge/diagnostics.go internal/codexbridge/protocol_test.go internal/codexbridge/diagnostics_test.go
rtk git commit -m "feat: define Codex Bridge protocol records"
```

## Task 2: Add immutable Host evidence and the bounded LRU cache

**Files:**
- Create: `internal/codexbridge/evidence.go`
- Create: `internal/codexbridge/cache.go`
- Create: `internal/codexbridge/cache_test.go`

- [ ] **Step 1: Write failing cache tests with an injected clock and random source**

```go
func TestEvidenceStoreBindsHandleToSessionAndCWD(t *testing.T) {
	store := NewEvidenceStore(CacheOptions{Now: fixedTime, TTL: time.Minute, MaximumEntries: 2, Random: bytes.NewReader(bytes.Repeat([]byte{0x42}, 64))})
	facts := testFacts(t, "session-a", "/repo/a")
	handle, err := store.Put(testContext("session-a", "/repo/a"), facts)
	if err != nil { t.Fatal(err) }
	foreignSession := handle
	foreignSession.SessionDigest = digestSessionID("session-b")
	if _, err := store.Get(foreignSession); Code(err) != "HOST_EVIDENCE_SESSION_MISMATCH" { t.Fatalf("cross-session header error = %v", err) }
	foreignCWD := handle
	foreignCWD.CWDDigest = digestCWD("/repo/b")
	if _, err := store.Get(foreignCWD); Code(err) != "HOST_EVIDENCE_SESSION_MISMATCH" { t.Fatalf("cross-cwd header error = %v", err) }
}

func TestEvidenceStoreExpiresAndEvictsDeterministically(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	store := NewEvidenceStore(CacheOptions{
		Now: func() time.Time { return now }, TTL: time.Minute, MaximumEntries: 2,
		Random: bytes.NewReader(bytes.Repeat([]byte{0x41}, 128)),
	})
	context := testContext("session-a", "/repo/a")
	first := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	second := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	if _, err := store.Get(first); err != nil { t.Fatal(err) } // first is now MRU
	third := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	if _, err := store.Get(second); Code(err) != "HOST_EVIDENCE_HANDLE_INVALID" {
		t.Fatalf("evicted handle error = %v", err)
	}
	if _, err := store.Get(first); err != nil { t.Fatalf("first handle was evicted: %v", err) }
	if _, err := store.Get(third); err != nil { t.Fatalf("third handle unavailable: %v", err) }
	now = now.Add(time.Minute)
	if _, err := store.Get(first); Code(err) != "HOST_EVIDENCE_EXPIRED" {
		t.Fatalf("expired handle error = %v", err)
	}
}

func TestEvidenceStoreResetInvalidatesAllHandles(t *testing.T) {
	store := NewEvidenceStore(CacheOptions{
		Now: fixedTime, TTL: time.Minute, MaximumEntries: 2,
		Random: bytes.NewReader(bytes.Repeat([]byte{0x42}, 64)),
	})
	context := testContext("session-a", "/repo/a")
	handle := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	store.Reset()
	if _, err := store.Get(handle); Code(err) != "HOST_EVIDENCE_HANDLE_INVALID" {
		t.Fatalf("reset handle error = %v", err)
	}
}

func TestEvidenceStoreIssuesDistinctRandomTokens(t *testing.T) {
	store := NewEvidenceStore(CacheOptions{Now: fixedTime, TTL: time.Minute, MaximumEntries: 2, Random: rand.Reader})
	context := testContext("session-a", "/repo/a")
	first := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	second := mustPut(t, store, context, testFacts(t, "session-a", "/repo/a"))
	if first.Token == second.Token { t.Fatal("two handles reused the same random token") }
}

func mustPut(t *testing.T, store EvidenceStore, context HookContext, facts Facts) HostEvidenceHandle {
	t.Helper()
	handle, err := store.Put(context, facts)
	if err != nil { t.Fatal(err) }
	return handle
}

func digestSessionID(value string) string {
	digest, _, err := ContextDigestHeaders(HookContext{SessionID: value, CWD: "/repo/a"})
	if err != nil { panic(err) }
	return digest
}

func digestCWD(value string) string {
	_, digest, err := ContextDigestHeaders(HookContext{SessionID: "session-a", CWD: value})
	if err != nil { panic(err) }
	return digest
}

var fixedTime = func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) }
```

`testContext` fills both Bridge version fields, a stable session/turn/tool ID,
and the supplied canonical cwd. `testFacts` builds an immutable
`host.SessionSnapshot`, `host.BindingInventory`, `host.EnvironmentReport`, and
empty validated configuration/discovery/registry records through the existing
`internal/hosttest` fixture constructors; both helpers are defined in
`cache_test.go` and never write outside `t.TempDir()`.

The tests must also assert two issued tokens differ with a real
`crypto/rand.Reader`, and run under `go test -race`.

- [ ] **Step 2: Run RED**

```bash
rtk go test ./internal/codexbridge -run 'EvidenceStore'
```

Expected: FAIL because the cache API is absent.

- [ ] **Step 3: Implement the immutable fact set and cache**

```go
type Facts struct {
	Session       host.SessionSnapshot
	Inventory     host.BindingInventory
	Environment   host.EnvironmentReport
	Configuration config.Snapshot
	Discovery     discovery.Report
	Resolutions   registry.ResolutionReport
	Registry      registry.Registry
	FactDigests   FactDigests
}

type EvidenceStore interface {
	Put(HookContext, Facts) (HostEvidenceHandle, error)
	Get(HostEvidenceHandle) (Facts, error)
	Reset()
}

type CacheOptions struct {
	Now             func() time.Time
	TTL             time.Duration
	MaximumEntries  int
	Random          io.Reader
}

func ContextDigestHeaders(context HookContext) (string, string, error) {
	if !utf8.ValidString(context.SessionID) || context.SessionID == "" || len(context.SessionID) > 512 ||
		strings.IndexFunc(context.SessionID, unicode.IsControl) >= 0 {
		return "", "", NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "invalid Codex session identity", nil)
	}
	cwd, err := canonicalCWD(context.CWD)
	if err != nil { return "", "", err }
	return digestHeader("session", context.SessionID), digestHeader("cwd", cwd), nil
}

func ValidateHandleContext(handle HostEvidenceHandle, context HookContext) error {
	if context.SchemaVersion != HookContextSchemaV1 || context.BridgeProtocolVersion != BridgeProtocolVersion {
		return NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook context schema or Bridge protocol is unsupported", nil)
	}
	sessionDigest, cwdDigest, err := ContextDigestHeaders(context)
	if err != nil { return err }
	if handle.Version != EvidenceHandleVersion ||
		subtle.ConstantTimeCompare([]byte(handle.SessionDigest), []byte(sessionDigest)) != 1 ||
		subtle.ConstantTimeCompare([]byte(handle.CWDDigest), []byte(cwdDigest)) != 1 {
		return NewError("HOST_EVIDENCE_SESSION_MISMATCH", "handle headers do not match the current Host context", nil)
	}
	return nil
}

func digestHeader(kind, value string) string {
	digest := sha256.Sum256([]byte(EvidenceHandleVersion + "\x00" + kind + "\x00" + value))
	return hex.EncodeToString(digest[:])
}

func canonicalCWD(value string) (string, error) {
	if !utf8.ValidString(value) || value == "" || strings.IndexFunc(value, unicode.IsControl) >= 0 || !filepath.IsAbs(value) {
		return "", NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "cwd must be an absolute path without controls", nil)
	}
	absolute, err := filepath.Abs(value)
	if err != nil { return "", NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "canonicalize cwd", err) }
	clean := filepath.Clean(absolute)
	if !filepath.IsAbs(clean) { return "", NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "canonical cwd is not absolute", nil) }
	return clean, nil
}
```

Validate every fact with `host.NewBindingInventory`, `host.NewEnvironmentReport`,
`host.NewSessionSnapshot` and the existing immutable Snapshot/Registry digest
accessors before `Put`. `canonicalCWD` requires valid UTF-8 with no controls,
calls `filepath.Abs` followed by `filepath.Clean`, and rejects a non-absolute
result. The token
format is `oawh1.<base64url(random-16-or-more-bytes)>.<hex(session+cwd+protocol)[:16]>`.
The Hook compares the header to the current call before the MCP operation.
The header is still only an early rejection hint: `Get` must locate the exact
token, recompute both header digests from the stored full Session ID and
canonical cwd, check `issued <= now < expires`, and then clone all nested
slices before returning. This two-layer check does not require later Hooks to
rewrite the public MCP input.

Use `container/list` under one mutex. On `Get` move the entry to the front;
on capacity overflow remove the back entry. Clamp TTL to 30 seconds through
15 minutes, capacity to 1 through 256, and clear the list on `Reset`. Never
serialize the entry or log the token.

- [ ] **Step 4: Run GREEN and race checks**

```bash
rtk gofmt -w internal/codexbridge/evidence.go internal/codexbridge/cache.go internal/codexbridge/cache_test.go
rtk go test ./internal/codexbridge -run 'EvidenceStore'
rtk go test -race ./internal/codexbridge -run 'EvidenceStore'
```

Expected: PASS with no race reports.

- [ ] **Step 5: Commit the cache**

```bash
rtk git add internal/codexbridge/evidence.go internal/codexbridge/cache.go internal/codexbridge/cache_test.go
rtk git commit -m "feat: add session-bound Host evidence cache"
```

## Task 3: Implement the exact PreToolUse adapter

**Files:**
- Create: `internal/codexbridge/hook/input.go`
- Create: `internal/codexbridge/hook/output.go`
- Create: `internal/codexbridge/hook/input_test.go`
- Create: `internal/codexbridge/hook/output_test.go`

- [ ] **Step 1: Write failing Hook tests**

```go
func TestParsePreToolUseRequiresExactEventAndIdentity(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{"session_id":"s","turn_id":"t","tool_use_id":"u","cwd":"/repo","tool_name":"mcp__oaw_codex_bridge__observe_current","tool_input":{}}`),
		[]byte(`{"session_id":"s","turn_id":"t","tool_use_id":"u","cwd":"/repo","hook_event_name":"PostToolUse","tool_name":"mcp__oaw_codex_bridge__observe_current","tool_input":{}}`),
		[]byte(`{"session_id":"s","tool_use_id":"u","cwd":"/repo","hook_event_name":"PreToolUse","tool_name":"mcp__oaw_codex_bridge__observe_current","tool_input":{}}`),
	} {
		if _, err := ParsePreToolUse(raw); codexbridge.Code(err) != "HOST_BRIDGE_CONTEXT_REQUIRED" { t.Fatalf("error = %v", err) }
	}
}

func TestObserveRewriteIsTheOnlyAutomaticAllow(t *testing.T) {
	ctx := validInput("mcp__oaw_codex_bridge__observe_current")
	result, err := RewriteObserveInput(ctx)
	decision := result.HookSpecificOutput
	if err != nil || decision == nil || decision.HookEventName != "PreToolUse" || decision.PermissionDecision != "allow" || len(decision.UpdatedInput) == 0 { t.Fatalf("result = %#v, %v", result, err) }
	encoded, err := json.Marshal(result)
	if err != nil || bytes.Contains(encoded, []byte(`{"permissionDecision":`)) || !bytes.Contains(encoded, []byte(`{"hookSpecificOutput":`)) { t.Fatalf("wire output = %s", encoded) }
	for _, name := range []string{"mcp__oaw_codex_bridge__core_inspect", "mcp__oaw_codex_bridge__core_compile", "mcp__oaw_codex_bridge__workflow_exchange"} {
		result, err := ValidateHandleInput(validHandleInput(t, name))
		if err != nil || result.HookSpecificOutput != nil { t.Fatalf("%s changed approval: %#v, %v", name, result, err) }
	}
}

func TestLaterOperationContextMismatchReturnsWrappedDeny(t *testing.T) {
	input := validHandleInput(t, "mcp__oaw_codex_bridge__core_inspect")
	input.SessionID = "foreign-session"
	result, err := ValidateHandleInput(input)
	decision := result.HookSpecificOutput
	if err != nil || decision == nil || decision.HookEventName != "PreToolUse" ||
		decision.PermissionDecision != "deny" || decision.PermissionDecisionReason == "" || len(decision.UpdatedInput) != 0 {
		t.Fatalf("result = %#v, %v", result, err)
	}
}

func validInput(toolName string) PreToolUseInput {
	return PreToolUseInput{SessionID: "s", TurnID: "t", ToolUseID: "u", CWD: "/repo",
		HookEventName: "PreToolUse", Model: "gpt-test", PermissionMode: "default",
		ToolName: toolName, ToolInput: json.RawMessage(`{}`)}
}

func validHandleInput(t *testing.T, toolName string) PreToolUseInput {
	t.Helper()
	input := validInput(toolName)
	sessionDigest, cwdDigest, err := codexbridge.ContextDigestHeaders(codexbridge.HookContext{SessionID: input.SessionID, CWD: input.CWD})
	if err != nil { t.Fatal(err) }
	handle := codexbridge.HostEvidenceHandle{Version: codexbridge.EvidenceHandleVersion, SessionDigest: sessionDigest, CWDDigest: cwdDigest, Token: strings.Repeat("h", 22)}
	encoded, err := json.Marshal(struct{ HostEvidenceHandle codexbridge.HostEvidenceHandle `json:"host_evidence_handle"` }{handle})
	if err != nil { t.Fatal(err) }
	input.ToolInput = encoded
	return input
}
```

Add cases for a wrong tool name, a non-object `tool_input`, oversized input,
control characters, missing `turn_id`, missing or non-`PreToolUse`
`hook_event_name`, and a handle whose session/cwd differs. Define `validInput`
and `validHandleInput` in `input_test.go`; the latter obtains the expected
headers through `codexbridge.ContextDigestHeaders` and encodes a complete
`host_evidence_handle` object rather than hard-coding a digest.

- [ ] **Step 2: Run RED**

```bash
rtk go test ./internal/codexbridge/hook
```

Expected: FAIL because the Hook package is absent.

- [ ] **Step 3: Implement strict decoding and context injection**

```go
type PreToolUseInput struct {
	SessionID      string          `json:"session_id"`
	TranscriptPath *string         `json:"transcript_path"`
	TurnID         string          `json:"turn_id"`
	ToolUseID      string          `json:"tool_use_id"`
	CWD            string          `json:"cwd"`
	HookEventName  string          `json:"hook_event_name"`
	Model          string          `json:"model"`
	PermissionMode string          `json:"permission_mode"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
}

type PreToolUseDecision struct {
	HookEventName            string                     `json:"hookEventName"`
	PermissionDecision       string                     `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string                     `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             map[string]json.RawMessage `json:"updatedInput,omitempty"`
}

type HookOutput struct {
	HookSpecificOutput *PreToolUseDecision `json:"hookSpecificOutput,omitempty"`
}

func RewriteObserveInput(input PreToolUseInput) (HookOutput, error) {
	if input.ToolName != "mcp__oaw_codex_bridge__observe_current" {
		return HookOutput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "unexpected observation tool", nil)
	}
	public := make(map[string]json.RawMessage)
	if err := json.Unmarshal(input.ToolInput, &public); err != nil {
		return HookOutput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "tool input must be an object", err)
	}
	if _, exists := public["_oaw_host_context"]; exists {
		return HookOutput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "reserved context was caller supplied", nil)
	}
	context := codexbridge.HookContext{SchemaVersion: codexbridge.HookContextSchemaV1, BridgeProtocolVersion: codexbridge.BridgeProtocolVersion, SessionID: input.SessionID, TurnID: input.TurnID, ToolUseID: input.ToolUseID, CWD: input.CWD, Model: input.Model, PermissionMode: input.PermissionMode}
	private, err := json.Marshal(context)
	if err != nil { return HookOutput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "encode reserved context", err) }
	public["_oaw_host_context"] = private
	return HookOutput{HookSpecificOutput: &PreToolUseDecision{
		HookEventName: "PreToolUse", PermissionDecision: "allow", UpdatedInput: public,
	}}, nil
}

func denyContextMismatch() HookOutput {
	return HookOutput{HookSpecificOutput: &PreToolUseDecision{
		HookEventName: "PreToolUse", PermissionDecision: "deny",
		PermissionDecisionReason: "OAW Host evidence does not match the current Codex session and working directory.",
	}}
}

func ValidateHandleInput(input PreToolUseInput) (HookOutput, error) {
	switch input.ToolName {
	case "mcp__oaw_codex_bridge__core_inspect", "mcp__oaw_codex_bridge__core_compile", "mcp__oaw_codex_bridge__workflow_exchange":
	default:
		return denyContextMismatch(), nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(input.ToolInput, &object); err != nil { return denyContextMismatch(), nil }
	encoded, ok := object["host_evidence_handle"]
	if !ok { return denyContextMismatch(), nil }
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var handle codexbridge.HostEvidenceHandle
	if err := decoder.Decode(&handle); err != nil { return denyContextMismatch(), nil }
	if err := codexbridge.ValidateHandleContext(handle, codexbridge.HookContext{
		SchemaVersion: codexbridge.HookContextSchemaV1, BridgeProtocolVersion: codexbridge.BridgeProtocolVersion,
		SessionID: input.SessionID, TurnID: input.TurnID, ToolUseID: input.ToolUseID, CWD: input.CWD,
		Model: input.Model, PermissionMode: input.PermissionMode,
	}); err != nil { return denyContextMismatch(), nil }
	return HookOutput{}, nil
}

func ProcessPreToolUse(raw []byte) (HookOutput, error) {
	input, err := ParsePreToolUse(raw)
	if err != nil { return denyContextMismatch(), nil }
	if input.ToolName == "mcp__oaw_codex_bridge__observe_current" {
		output, rewriteErr := RewriteObserveInput(input)
		if rewriteErr != nil { return denyContextMismatch(), nil }
		return output, nil
	}
	return ValidateHandleInput(input)
}
```

`ParsePreToolUse` uses a size-bounded decoder with `DisallowUnknownFields`,
requires `hook_event_name == "PreToolUse"`, validates every required identity
field, and first proves that `tool_input` is a non-null JSON object, so the map
merge cannot reinterpret arrays or scalars. The struct includes every official
common and event-specific Codex 0.146.1 field, including nullable
`transcript_path`; a schema change therefore fails closed and is covered by a
fixture update rather than being silently ignored.

Add `ContextDigestHeaders(HookContext) (sessionDigest, cwdDigest string, err
error)` and `ValidateHandleContext(HostEvidenceHandle, HookContext) error` to
`protocol.go`, and use those same functions from both the cache and Hook.
`ValidateHandleInput` extracts the public `host_evidence_handle` from the
otherwise operation-specific input object. For an exact later-operation tool
and matching handle headers it returns `HookOutput{}`; the CLI writes no stdout,
so normal Codex MCP approval behavior remains active. A malformed handle,
unexpected matched tool, wrong version, foreign session, or foreign canonical
cwd returns `denyContextMismatch()` with exit status 0 and no `updatedInput`.
The Bridge server still performs the authoritative token/cache/fact lookup.
The matcher is an exact set of the four generated MCP tool names; it must not
match Bash, shell, or a prefix.

- [ ] **Step 4: Run Hook GREEN, fuzz, and vet checks**

```bash
rtk gofmt -w internal/codexbridge/hook
rtk go test ./internal/codexbridge/hook
rtk go test -fuzz=FuzzParsePreToolUse -fuzztime=10s ./internal/codexbridge/hook
rtk go vet ./internal/codexbridge/hook
```

Expected: PASS; fuzzing must never panic or emit an `allow` for a non-observation tool.

- [ ] **Step 5: Commit the Hook adapter**

```bash
rtk git add internal/codexbridge/hook
rtk git commit -m "feat: validate Codex PreToolUse context"
```

## Task 4: Self-review the phase boundary

- [ ] **Step 1: Verify no private fact is durable or executable**

```bash
rtk rg -n 'os\.WriteFile|json\.Encoder|exec\.Command|HOME|plugin/list|thread/start|turn/start' internal/codexbridge
```

Expected: no file writes, process launch, model/thread method, or credential
handling in this phase. JSON encoding is allowed only for Hook/MCP wire values.

- [ ] **Step 2: Run the complete leaf gate**

```bash
rtk go test ./internal/codexbridge/...
rtk go test -race ./internal/codexbridge/...
rtk git diff --check
```

Expected: PASS. The next phase may now consume `Facts`, `EvidenceStore`, and
the Hook records without changing their names or semantics.
