# OAW Codex Host Bridge 05: Plugin Packaging and Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans in the current session to implement this plan task-by-task. This plan is locked to `CURRENT`; do not dispatch subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Package the Bridge as an explicit Codex Plugin and provide auditable install, update, check, serve, hook, and uninstall commands with transactional ownership of only OAW-created artifacts.

**Architecture:** Bridge management is a separate package from policy management. OAW renders a local marketplace and Plugin from embedded templates, copies the running binary into an OAW-owned data directory, and invokes only official `codex plugin marketplace` / `codex plugin` commands through an argument-vector runner. It never edits Codex config/cache, downloads executable code, relies on future `PATH`, creates a private HOME, or silently enables the Plugin.

**Tech Stack:** Go 1.26, `go:embed`, `os.Root`/atomic rename, `crypto/sha256`, `os/exec`, strict JSON, existing management environment conventions, black-box shell tests.

---

**Selected execution:** `CURRENT`. Do not dispatch subagents. Do not publish an intermediate phase commit.

**Depends on:** Plans 01-04.

**Produces:** The production `oaw bridge ...` command surface and install state consumed by Plan 06.

## File Map

| Path | Responsibility |
| --- | --- |
| `internal/codexbridge/install/records.go` | Install state, ownership records, drift status, and command results. |
| `internal/codexbridge/install/paths.go` | OAW-owned data coordinates and safe relative path validation. |
| `internal/codexbridge/install/templates.go` | Embedded Plugin/marketplace templates and deterministic rendering. |
| `internal/codexbridge/install/runner.go` | Official Codex CLI argument-vector abstraction and JSON output parsing. |
| `internal/codexbridge/install/transaction.go` | Prepare/apply/rollback install, update, and uninstall operations. |
| `internal/codexbridge/install/check.go` | Read-only file, version, Codex capability, marketplace, and Plugin status projection. |
| `internal/codexbridge/install/*_test.go` | Transaction, drift, rollback, redaction, and runner tests. |
| `internal/codexbridge/install/assets/.codex-plugin/plugin.json` | Plugin manifest template. |
| `internal/codexbridge/install/assets/.mcp.json` | MCP server template with binary placeholder. |
| `internal/codexbridge/install/assets/hooks/hooks.json` | Exact four-tool PreToolUse matcher template. |
| `internal/codexbridge/install/assets/skills/oaw-codex-bridge/SKILL.md` | Instruction-only guidance; never evidence. |
| `internal/codexbridge/install/assets/.agents/plugins/marketplace.json` | OAW-owned local marketplace template at the Codex 0.146.1 discovery path. |
| `internal/cli/bridge.go` | CLI parsing and command dispatch. |
| `internal/cli/run.go` | Root command dispatch and usage text. |
| `internal/cli/bridge_test.go` | Argument validation and command exit status tests. |
| `tests/17-codex-bridge-management-test.sh` | Black-box CLI and isolation checks. |

## Locked Rendered Package

The renderer recognizes only `{{OAW_BINARY}}`, `{{OAW_HOOK_COMMAND}}`,
`{{BRIDGE_VERSION}}`, `{{MARKETPLACE_NAME}}`, and `{{PLUGIN_NAME}}`; any other
or remaining placeholder fails the render step. It JSON-escapes every value
before inserting it into a quoted JSON field. The Hook command uses a dedicated
POSIX argument encoder for the absolute binary path; it never reuses an MCP
command string or interpolates unquoted path bytes. The five rendered files
use this marketplace-relative layout:

```text
.agents/plugins/marketplace.json
plugins/oaw-codex-host/.codex-plugin/plugin.json
plugins/oaw-codex-host/.mcp.json
plugins/oaw-codex-host/hooks/hooks.json
plugins/oaw-codex-host/skills/oaw-codex-bridge/SKILL.md
```

Codex 0.146.1 accepts a local marketplace directory only when its manifest is
at `.agents/plugins/marketplace.json`; it rejects both a root
`marketplace.json` and a JSON file passed directly as the marketplace source.
The marketplace entry therefore uses the official object-valued local source
`{"source":"local","path":"./plugins/oaw-codex-host"}`. OAW supports only
that current format and does not render the rejected shapes.

The MCP file has this semantic content:

```json
{
  "oaw_codex_bridge": {
    "command": "{{OAW_BINARY}}",
    "args": ["bridge", "serve", "codex"],
    "cwd": ".",
    "env_vars": []
  }
}
```

This is the official direct server-map form. Do not wrap the file in
`mcpServers`; that name belongs to the Plugin manifest. The only accepted
alternative wrapper would be `mcp_servers`, which this package deliberately
does not use.

The Hook template contains one exact `PreToolUse` matcher for each generated
tool name and the same command with `bridge hook codex`. The four matchers are
`mcp__oaw_codex_bridge__observe_current`, `mcp__oaw_codex_bridge__core_inspect`,
`mcp__oaw_codex_bridge__core_compile`, and
`mcp__oaw_codex_bridge__workflow_exchange`. The observation matcher is the
only one allowed to return an automatic decision because it rewrites a
strictly read-only input. Its stdout is the official nested
`hookSpecificOutput` object with `hookEventName = PreToolUse`,
`permissionDecision = allow`, and object-valued `updatedInput`. The other
three Hook entries emit no stdout when the handle header matches the current
session/cwd, and emit the same nested shape with `permissionDecision = deny`
and no `updatedInput` when it does not. Every invocation requires input
`hook_event_name == PreToolUse`.

Each Hook handler's `command` field contains the `{{OAW_HOOK_COMMAND}}`
placeholder; the renderer replaces it with the shell-quoted absolute binary
followed by `bridge hook codex`. The `.mcp.json` server's `command` field uses
the separate `{{OAW_BINARY}}` placeholder and passes `bridge serve codex` as
argv, so the two execution surfaces cannot accidentally share quoting rules.

## Task 1: Add embedded Plugin and marketplace templates

**Files:**
- Create: `internal/codexbridge/install/templates.go`
- Create: `internal/codexbridge/install/assets/.codex-plugin/plugin.json`
- Create: `internal/codexbridge/install/assets/.mcp.json`
- Create: `internal/codexbridge/install/assets/hooks/hooks.json`
- Create: `internal/codexbridge/install/assets/skills/oaw-codex-bridge/SKILL.md`
- Create: `internal/codexbridge/install/assets/.agents/plugins/marketplace.json`
- Create: `internal/codexbridge/install/templates_test.go`

- [x] **Step 1: Write failing template tests**

```go
func TestRenderPluginHasExactSurface(t *testing.T) {
	files, err := Render(RenderOptions{Binary: "/state/bin/oaw", Version: "1.0.0", Marketplace: "oaw-local", Plugin: "oaw-codex-host"})
	if err != nil { t.Fatal(err) }
	if len(files) != 5 { t.Fatalf("file count=%d", len(files)) }
	if strings.Contains(string(files["plugins/oaw-codex-host/hooks/hooks.json"]), "{{") || strings.Contains(string(files["plugins/oaw-codex-host/.mcp.json"]), "{{") { t.Fatal("unresolved placeholder") }
	assertPluginManifest(t, files["plugins/oaw-codex-host/.codex-plugin/plugin.json"], "./skills/", "./.mcp.json", "./hooks/hooks.json")
	assertExactMatchers(t, files["plugins/oaw-codex-host/hooks/hooks.json"])
	assertMarketplaceSource(t, files[".agents/plugins/marketplace.json"], "./plugins/oaw-codex-host")
}

func TestRenderRejectsUnsafeAbsoluteBinary(t *testing.T) {
	if _, err := Render(RenderOptions{Binary: "relative/oaw", Version: "1.0.0", Marketplace: "oaw-local", Plugin: "oaw-codex-host"}); Code(err) != "BRIDGE_INSTALL_INPUT_INVALID" { t.Fatalf("error=%v", err) }
}

func TestRenderRejectsAlternateBridgeIdentities(t *testing.T) {
	for _, options := range []RenderOptions{
		{Binary: "/state/bin/oaw", Version: "1.0.0", Marketplace: "other", Plugin: "oaw-codex-host"},
		{Binary: "/state/bin/oaw", Version: "1.0.0", Marketplace: "oaw-local", Plugin: "other"},
	} {
		if _, err := Render(options); Code(err) != "BRIDGE_INSTALL_INPUT_INVALID" { t.Fatalf("options=%#v error=%v", options, err) }
	}
}

func TestRenderEscapesBinaryPathForJSONAndHookShell(t *testing.T) {
	binary := "/state/space $dir/it's/oaw"
	files, err := Render(RenderOptions{Binary: binary, Version: "1.0.0", Marketplace: "oaw-local", Plugin: "oaw-codex-host"})
	if err != nil { t.Fatal(err) }
	assertMCPCommand(t, files["plugins/oaw-codex-host/.mcp.json"], binary)
	assertHookCommand(t, files["plugins/oaw-codex-host/hooks/hooks.json"], quotePOSIX(binary)+" bridge hook codex")
}
```

- [x] **Step 2: Run RED**

```bash
rtk go test ./internal/codexbridge/install -run 'Render'
```

Expected: FAIL because templates and renderer are absent.

- [x] **Step 3: Add the exact plugin manifest and renderer**

Before implementing the renderer, add the literal placeholder to the embedded
assets: every entry in `assets/hooks/hooks.json` must be a `PreToolUse` matcher
for exactly one of the four generated MCP tool names and must use
`"command": "{{OAW_HOOK_COMMAND}}"`. The asset must contain all four matchers
and no shell command, binary path, or second placeholder. The direct server map
in `assets/.mcp.json` uses only `{{OAW_BINARY}}`; the Plugin manifest uses
`{{BRIDGE_VERSION}}`, while `assets/.agents/plugins/marketplace.json` uses
`{{MARKETPLACE_NAME}}` and `{{PLUGIN_NAME}}` for its exact identities.

The Hook asset must use the official Codex lifecycle-hook envelope, with one
matcher object per generated MCP tool under `hooks.PreToolUse`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "mcp__oaw_codex_bridge__observe_current",
        "hooks": [
          {"type": "command", "command": "{{OAW_HOOK_COMMAND}}"}
        ]
      }
    ]
  }
}
```

Repeat the matcher object for `core_inspect`, `core_compile`, and
`workflow_exchange`. Do not flatten the handler, use a top-level `command`, or
place the entries under a non-official event key. The renderer must decode the
closed Hook projection and verify exactly four `PreToolUse` matcher objects,
one command handler per matcher, and no additional event or handler fields.

```json
{
  "name": "oaw-codex-host",
  "version": "{{BRIDGE_VERSION}}",
  "description": "Open Agent Workflow Host Bridge for Codex",
  "skills": "./skills/",
  "mcpServers": "./.mcp.json",
  "hooks": "./hooks/hooks.json",
  "interface": {"displayName":"OAW Codex Host Bridge","shortDescription":"Current-session Host evidence and Workflow coordination","category":"Developer Tools"}
}
```

```go
//go:embed assets/.agents/plugins/marketplace.json assets/.codex-plugin/plugin.json assets/.mcp.json assets/hooks/hooks.json assets/skills/oaw-codex-bridge/SKILL.md
var templateFS embed.FS

func Render(options RenderOptions) (map[string][]byte, error) {
	if err := validateRenderOptions(options); err != nil { return nil, err }
	if runtime.GOOS == "windows" { return nil, installError("BRIDGE_INSTALL_UNSUPPORTED", "Codex Hook command rendering is not verified on Windows", nil) }
	hookCommand := quotePOSIX(options.Binary) + " bridge hook codex"
	replacements := map[string]string{
		"{{OAW_BINARY}}": options.Binary,
		"{{OAW_HOOK_COMMAND}}": hookCommand,
		"{{BRIDGE_VERSION}}": options.Version,
		"{{MARKETPLACE_NAME}}": options.Marketplace,
		"{{PLUGIN_NAME}}": options.Plugin,
	}
	result := make(map[string][]byte, 5)
	templates := []struct{ source, target string }{
		{".agents/plugins/marketplace.json", ".agents/plugins/marketplace.json"},
		{".codex-plugin/plugin.json", "plugins/oaw-codex-host/.codex-plugin/plugin.json"},
		{".mcp.json", "plugins/oaw-codex-host/.mcp.json"},
		{"hooks/hooks.json", "plugins/oaw-codex-host/hooks/hooks.json"},
		{"skills/oaw-codex-bridge/SKILL.md", "plugins/oaw-codex-host/skills/oaw-codex-bridge/SKILL.md"},
	}
	for _, file := range templates {
		raw, err := templateFS.ReadFile("assets/" + file.source); if err != nil { return nil, err }
		rendered := append([]byte{}, raw...)
		for token, value := range replacements {
			encoded, err := json.Marshal(value)
			if err != nil { return nil, installError("BRIDGE_INSTALL_INPUT_INVALID", "encode template value", err) }
			rendered = bytes.ReplaceAll(rendered, []byte(token), encoded[1:len(encoded)-1])
		}
		if bytes.Contains(rendered, []byte("{{")) { return nil, installError("BRIDGE_INSTALL_INPUT_INVALID", "unresolved template placeholder", nil) }
		result[file.target] = rendered
	}
	return result, nil
}

func quotePOSIX(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
```

Validate UTF-8, control characters, an absolute clean binary path, the exact
v1 identities `oaw-local` and `oaw-codex-host`, and a semver-like Bridge
version. The names remain explicit inputs for state/CLI comparison but cannot
change the embedded marketplace layout in v1. Parse every
rendered JSON file with a closed plan-owned projection and
`DisallowUnknownFields`; ensure the marketplace path is relative to its own
root, the Plugin source is the local-source object pointing to
`./plugins/oaw-codex-host`, and the manifest
points exactly to `./skills/`, `./.mcp.json`, and
`./hooks/hooks.json`. `assertPluginManifest` decodes that same projection and
compares all three component paths; a root `.mcp.json` without the explicit
`mcpServers` pointer in the Plugin manifest is a failing fixture. Decode
`.mcp.json` as a direct map and
reject a top-level `mcpServers` key. Decode every Hook entry and prove the
rendered command is exactly the quoted binary plus the three fixed arguments.
Windows installation remains explicitly unsupported in Bridge v1 and returns
`BRIDGE_INSTALL_UNSUPPORTED`; deterministic Core/Bridge tests still run in
Docker Linux, while real Host installation is verified on macOS.

- [x] **Step 4: Run GREEN**

```bash
rtk gofmt -w internal/codexbridge/install/templates.go internal/codexbridge/install/templates_test.go
rtk go test ./internal/codexbridge/install -run 'Render'
rtk go vet ./internal/codexbridge/install
```

Expected: PASS.

- [x] **Step 5: Commit templates**

```bash
rtk git add internal/codexbridge/install/templates.go internal/codexbridge/install/assets internal/codexbridge/install/templates_test.go
rtk git commit -m "feat: package Codex Host Bridge plugin"
```

## Task 2: Implement independent install state and official CLI runner

**Files:**
- Create: `internal/codexbridge/install/records.go`
- Create: `internal/codexbridge/install/paths.go`
- Create: `internal/codexbridge/install/runner.go`
- Create: `internal/codexbridge/install/state_test.go`
- Create: `internal/codexbridge/install/runner_test.go`

- [x] **Step 1: Write failing state and argument-vector tests**

```go
func TestRunnerNeverUsesShell(t *testing.T) {
	fake := &recordingRunner{}
	_, err := fake.Run(context.Background(), "plugin", "list", "--json")
	if err != nil { t.Fatal(err) }
	if fake.Shell || len(fake.Commands) != 1 || !slices.Equal(fake.Commands[0], []string{"plugin", "list", "--json"}) { t.Fatalf("runner=%#v", fake) }
}

func TestInstallStateIsClosedAndDigestPinned(t *testing.T) {
	state := InstallState{SchemaVersion: InstallStateSchemaV1, BridgeVersion: "1.0.0", BinaryPath: "/state/bin/oaw", BinaryDigest: strings.Repeat("a",64), MarketplacePath: "/state/marketplace", MarketplaceName: "oaw-local", PluginName: "oaw-codex-host", Files: []OwnedFile{{Path:"plugins/oaw-codex-host/.mcp.json", Digest: strings.Repeat("b",64)}}}
	encoded, err := EncodeState(state); if err != nil { t.Fatal(err) }
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil { t.Fatal(err) }
	object["unknown"] = true
	mutated, err := json.Marshal(object); if err != nil { t.Fatal(err) }
	if _, err := DecodeState(mutated); err == nil { t.Fatal("unknown state field accepted") }
}
```

- [x] **Step 2: Run RED**

```bash
rtk go test ./internal/codexbridge/install -run 'Runner|InstallState'
```

Expected: FAIL because state and runner types are absent.

- [x] **Step 3: Add state records and OAW-owned coordinates**

```go
const InstallStateSchemaV1 = "oaw.codex-bridge-install/v1"
type OwnedFile struct { Path string `json:"path"`; Digest string `json:"digest"`; Mode uint32 `json:"mode"` }
type InstallState struct {
	SchemaVersion string `json:"schema_version"`
	BridgeVersion string `json:"bridge_version"`
	BinaryPath string `json:"binary_path"`
	BinaryDigest string `json:"binary_digest"`
	MarketplacePath string `json:"marketplace_path"`
	MarketplaceName string `json:"marketplace_name"`
	PluginName string `json:"plugin_name"`
	Files []OwnedFile `json:"files"`
	CodexPluginID string `json:"codex_plugin_id"`
	InstalledAt string `json:"installed_at"`
	Digest string `json:"digest"`
}

type Environment struct {
	StateRoot    string
	DataRoot     string
	StateFile    string
	CodexBinary  string
	ProjectRoot  string
}

type InstallRequest struct {
	Binary  string
	Version string
	DryRun  bool
}

type UninstallRequest struct{}
```

Store state at `$XDG_STATE_HOME/open-agent-workflow/codex-bridge/install.json`.
Store the checksum-pinned binary and rendered local marketplace below
`$XDG_DATA_HOME/open-agent-workflow/codex-bridge/`; `OwnedFile.Path` is
relative to that data root. Do not store OAW state or source artifacts under
`~/.codex`. Use separate `os.OpenRoot` handles for the state and data roots,
reject symlinks in all managed directories, write a 0600 temporary state file,
fsync, and atomically rename. State validation checks every recorded file's
current checksum before update or uninstall. Drift yields
`BRIDGE_INSTALL_DRIFT` and preserves the file.

- [x] **Step 4: Add the official Codex CLI runner**

```go
type CLIResult struct { Stdout, Stderr []byte; ExitCode int }
type CodexRunner interface { Run(context.Context, ...string) (CLIResult, error) }
type ExecRunner struct { Binary string; Environment []string; Dir string }

func (r ExecRunner) Run(ctx context.Context, args ...string) (CLIResult, error) {
	cmd := exec.CommandContext(ctx, r.Binary, args...)
	cmd.Dir = r.Dir
	if r.Environment == nil { cmd.Env = os.Environ() } else { cmd.Env = append([]string{}, r.Environment...) }
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	return CLIResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: exitCode(err)}, err
}
```

Provide wrappers with exact argument vectors:

```text
codex plugin marketplace add <absolute-marketplace-path> --json
codex plugin add oaw-codex-host@oaw-local --json
codex plugin list --json
codex plugin remove oaw-codex-host@oaw-local --json
codex plugin marketplace remove oaw-local --json
```

These vectors are pinned to and verified against the Codex 0.146.1 CLI help:
add,
remove, list, marketplace add/list/remove, and marketplace upgrade all expose
`--json`. The local Bridge never calls marketplace upgrade because Codex
defines that operation only for Git marketplaces.

Parse only the fields needed to identify the exact marketplace/plugin; retain
raw stdout only in a bounded, redacted test result. Never pass a shell string.
`ExecRunner.Environment == nil` means inherit `os.Environ()` so Codex can load
the user's normal authentication and config surface; a non-nil environment is
copied without mutation. The runner never creates a private HOME/CODEX_HOME or
projects replacement configuration.

- [x] **Step 5: Run GREEN and security checks**

```bash
rtk gofmt -w internal/codexbridge/install/records.go internal/codexbridge/install/paths.go internal/codexbridge/install/runner.go internal/codexbridge/install/*_test.go
rtk go test ./internal/codexbridge/install -run 'Runner|InstallState'
rtk go test -race ./internal/codexbridge/install -run 'Runner|InstallState'
rtk go vet ./internal/codexbridge/install
```

Expected: PASS; tests prove no shell, no Codex config edit, and no secret in
state or runner diagnostics.

- [x] **Step 6: Commit state and runner**

```bash
rtk git add internal/codexbridge/install/records.go internal/codexbridge/install/paths.go internal/codexbridge/install/runner.go internal/codexbridge/install/*_test.go
rtk git commit -m "feat: add transactional Codex Bridge install state"
```

## Task 3: Implement install, update, check, and uninstall transactions

**Files:**
- Create: `internal/codexbridge/install/transaction.go`
- Create: `internal/codexbridge/install/check.go`
- Create: `internal/codexbridge/install/transaction_test.go`
- Create: `internal/codexbridge/install/check_test.go`

- [x] **Step 1: Write failing transaction tests**

```go
func TestInstallRollsBackMarketplaceWhenPluginAddFails(t *testing.T) {
	env := tempInstallEnvironment(t)
	runner := &recordingRunner{Failures: map[string]error{"plugin add": errors.New("denied")}}
	_, err := Install(context.Background(), env, runner, InstallRequest{Binary: "/usr/local/bin/oaw"})
	if Code(err) != "BRIDGE_INSTALL_ROLLBACK" || runner.Saw("plugin remove") == false || runner.Saw("marketplace remove") == false { t.Fatalf("err=%v calls=%v", err, runner.Commands) }
	if _, statErr := os.Stat(env.StateFile); !errors.Is(statErr, fs.ErrNotExist) { t.Fatalf("state remains: %v", statErr) }
}

func TestUninstallPreservesDriftedUserFile(t *testing.T) {
	env, runner, state := installedFixture(t)
	owned := filepath.Join(env.DataRoot, state.Files[0].Path)
	if err := os.WriteFile(owned, []byte("user edit"), 0o600); err != nil { t.Fatal(err) }
	result, err := Uninstall(context.Background(), env, runner, UninstallRequest{})
	if err != nil { t.Fatal(err) }
	if !hasDiagnostic(result.Diagnostics, "BRIDGE_INSTALL_DRIFT") { t.Fatalf("diagnostics=%#v", result.Diagnostics) }
	if content, readErr := os.ReadFile(owned); readErr != nil || string(content) != "user edit" { t.Fatalf("drifted file was removed or changed: %q %v", content, readErr) }
	if runner.Saw("plugin remove") == false || runner.Saw("marketplace remove") == false { t.Fatalf("official cleanup missing: %v", runner.Commands) }
}

func TestCheckNeverClaimsCurrentSessionLoaded(t *testing.T) {
	env, runner := tempInstallEnvironment(t), &recordingRunner{}
	result, err := Check(context.Background(), env, runner)
	if err != nil { t.Fatal(err) }
	if result.CurrentSessionLoaded { t.Fatal("installation check inferred an active Codex session") }
}
```

`installedFixture` performs a successful install into a temporary `Environment`
using the recording runner, returns the decoded `InstallState`, and is defined
in `transaction_test.go`. `hasDiagnostic` matches the stable code without
printing paths or command output. `Check` is deliberately unable to observe a
live MCP process; only a successful `observe_current` response can establish
current-session evidence.

- [x] **Step 2: Run RED**

```bash
rtk go test ./internal/codexbridge/install -run 'Install|Uninstall|Check'
```

Expected: FAIL because transaction functions are absent.

- [x] **Step 3: Implement `Install` and `Update` in ordered phases**

```go
func Install(ctx context.Context, env Environment, runner CodexRunner, request InstallRequest) (Result, error) {
	prepared, err := prepare(ctx, env, request, false); if err != nil { return Result{}, err }
	if request.DryRun { return prepared.Predicted, nil }
	if err := applyOAWFiles(prepared); err != nil { return Result{}, err }
	marketplaceAdded := false
	if _, err := runner.Run(ctx, "plugin", "marketplace", "add", prepared.MarketplacePath, "--json"); err != nil { return rollbackInstall(prepared, runner, ctx, err) }
	marketplaceAdded = true
	if _, err := runner.Run(ctx, "plugin", "add", prepared.PluginName+"@"+prepared.MarketplaceName, "--json"); err != nil {
		return rollbackRegisteredInstall(prepared, runner, ctx, marketplaceAdded, err)
	}
	if err := writeState(prepared.State); err != nil { return rollbackRegisteredInstall(prepared, runner, ctx, true, err) }
	return prepared.Result, nil
}
```

`Update` first validates recorded checksums and the exact installed Plugin
identity, renders a new versioned staging tree, and keeps the previous clean
tree until the transaction commits. It atomically replaces the OAW-owned
local marketplace contents at the already registered path, runs official
Plugin remove and Plugin add for exactly
`oaw-codex-host@oaw-local`, and records `RequiresNewSession: true`. It does not
call `marketplace upgrade`, because that command refreshes Git marketplaces,
not local ones, and it never removes an unrelated Plugin. On failure it
restores the previous marketplace tree and re-adds the previous exact Plugin;
only after that recovery succeeds may it delete the staging tree. Install
rollback removes a registered Plugin before removing the marketplace, then
removes only staged/recorded artifacts. If any rollback command fails, return
`BRIDGE_INSTALL_ROLLBACK_INCOMPLETE` and preserve the state file and recovery
coordinates for manual recovery; do not delete broadly.

- [x] **Step 4: Implement read-only `Check`**

```go
type CheckResult struct {
	SchemaVersion string `json:"schema_version"`
	Files []FileStatus `json:"files"`
	CodexMarketplace MarketplaceStatus `json:"codex_marketplace"`
	CodexPlugin PluginStatus `json:"codex_plugin"`
	CurrentSessionLoaded bool `json:"current_session_loaded"`
	RequiresNewSession bool `json:"requires_new_session"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
```

The concrete entry points are `Check(context.Context, Environment,
CodexRunner) (CheckResult, error)` and `Uninstall(context.Context,
Environment, CodexRunner, UninstallRequest) (Result, error)`.

`Check` reads OAW-owned files and invokes `codex plugin marketplace list
--json` / `codex plugin list --json` only. It reports
`current_session_loaded=false` unconditionally because this install-management
process has no access to the MCP service's in-memory evidence. A successful
`observe_current` response is the separate current-session proof. Redact
absolute paths to short digests in the default text projection while retaining
exact paths in the machine-readable local result.

- [x] **Step 5: Implement safe uninstall**

Uninstall reads and validates state, invokes official Plugin remove and
marketplace remove only for the recorded names, removes only files whose
current digest equals the recorded digest, and leaves drifted files plus a
`BRIDGE_INSTALL_DRIFT` diagnostic. Remove empty OAW-owned directories only
after all recorded descendants are gone. `--force` is not accepted by Bridge
uninstall; a drifted artifact requires explicit manual resolution.

- [x] **Step 6: Run GREEN, fault injection, and race checks**

```bash
rtk gofmt -w internal/codexbridge/install/transaction.go internal/codexbridge/install/check.go internal/codexbridge/install/*_test.go
rtk go test ./internal/codexbridge/install
rtk go test -race ./internal/codexbridge/install
rtk go vet ./internal/codexbridge/install
```

Expected: PASS; injected failures prove rollback, drift preservation, and
idempotent replay without deleting unrelated files.

- [x] **Step 7: Commit management transactions**

```bash
rtk git add internal/codexbridge/install/transaction.go internal/codexbridge/install/check.go internal/codexbridge/install/*_test.go
rtk git commit -m "feat: manage Codex Bridge installation transactionally"
```

## Task 4: Add the production CLI surface and Bridge runners

**Files:**
- Create: `internal/cli/bridge.go`
- Create: `internal/cli/bridge_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/management.go`
- Modify: `internal/cli/management_test.go`

- [x] **Step 1: Write failing parser and dispatch tests**

```go
func TestParseBridgeCommands(t *testing.T) {
	cases := []struct{ args []string; operation string }{
		{[]string{"bridge","serve","codex"}, "serve"},
		{[]string{"bridge","hook","codex"}, "hook"},
		{[]string{"bridge","install","codex"}, "install"},
		{[]string{"bridge","update","codex"}, "update"},
		{[]string{"bridge","check","codex"}, "check"},
		{[]string{"bridge","uninstall","codex"}, "uninstall"},
	}
	for _, tc := range cases { parsed, err := parseBridge(tc.args[1:]); if err != nil || parsed.Operation != tc.operation { t.Fatalf("%v -> %#v %v", tc.args, parsed, err) } }
}

func TestBridgeRejectsUnknownHostAndLegacyRunner(t *testing.T) {	for _, args := range [][]string{{"bridge","serve","claude"}, {"runtime"}, {"run"}} { if status := RunWithInput(args, strings.NewReader(""), io.Discard, io.Discard); status != 64 { t.Fatalf("%v status=%d", args, status) } }
}
```

- [x] **Step 2: Run RED**

```bash
rtk go test ./internal/cli -run 'Bridge|LegacyRunner'
```

Expected: FAIL because the Bridge parser and dispatch are absent.

- [x] **Step 3: Add exact dispatch and usage**

```go
switch args[0] {
case "bridge":
	return runBridge(args[1:], stdin, stdout, stderr)
case "runtime", "run":
	fmt.Fprintf(stderr, "oaw: INVALID_ARGUMENT: command %q has been removed\n", args[0])
	return 64
}
```

`runBridge` accepts only the six operations and literal host `codex`. `serve`
constructs a `CodexLauncher` with the current binary's inherited environment,
the App Server metadata client, Evidence Store, and Service, then runs MCP on
stdio. `hook` reads at most 1 MiB from stdin, calls the exact Hook adapter, and
writes the wrapped JSON result only for observation `allow` or a rejected
later-operation `deny`; it writes no stdout for a valid later operation.
Malformed matched Hook input is converted to a secret-free wrapped `deny`
rather than a Hook process error that Codex would treat as advisory.
`check/install/update/uninstall` use the separate install package and never
instantiate a Service.

- [x] **Step 4: Run CLI GREEN and black-box parser checks**

```bash
rtk gofmt -w internal/cli/bridge.go internal/cli/bridge_test.go internal/cli/run.go internal/cli/management.go
rtk go test ./internal/cli
rtk go test ./cmd/oaw ./...
```

Expected: PASS; the command help lists `oaw bridge ...` and no old Runner,
`INLINE`, or `NATIVE_SUBAGENT` alias.

- [x] **Step 5: Commit the CLI surface**

```bash
rtk git add internal/cli/bridge.go internal/cli/bridge_test.go internal/cli/run.go internal/cli/management.go internal/cli/management_test.go
rtk git commit -m "feat: add Codex Bridge CLI commands"
```

## Task 5: Run isolated management black-box tests

**Files:**
- Create: `tests/17-codex-bridge-management-test.sh`
- Modify: `tests/run.sh`

- [x] **Step 1: Add executable black-box assertions**

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/test-helper.sh"
assert_contains "$(run_oaw bridge --help)" "serve codex"
assert_exit 64 run_oaw bridge serve claude
assert_exit 64 run_oaw runtime
```

Use `mktemp -d` to construct explicit `Environment{StateRoot, DataRoot}`
coordinates and a fake Codex runner that records argv. Assert no file is
written below the fake Codex-owned fixture root except changes made by the
official-runner fixture, and assert rollback removes only the recorded local
marketplace. Do not mutate process-global `HOME`, `CODEX_HOME`, or another
ambient configuration variable, and do not call the real Codex Plugin
installer in this test.

- [x] **Step 2: Run the black-box test**

```bash
rtk bash tests/17-codex-bridge-management-test.sh
```

Expected: PASS.

- [x] **Step 3: Commit black-box coverage**

```bash
rtk git add tests/17-codex-bridge-management-test.sh tests/run.sh
rtk git commit -m "test: cover Codex Bridge command boundaries"
```

## Task 6: Self-review installation authority

- [x] **Step 1: Scan for forbidden mutations**

```bash
rtk rg -n 'os\.WriteFile.*\.codex|\.codex/config|\.codex/plugins/cache|HOME=|CODEX_HOME=|sh -c|bash -c|plugin/list' --glob '*.go' --glob '!**/*_test.go' internal/codexbridge/install internal/cli
```

Expected: no direct Codex config/cache writes, shell interpolation, private
HOME, or production `plugin/list` dependency. Negative-test literals remain
covered by the tests and are deliberately excluded from this production scan.

- [x] **Step 2: Run the phase gate**

```bash
rtk go test ./internal/codexbridge/... ./internal/cli
rtk go test -race ./internal/codexbridge/... ./internal/cli
rtk bash tests/17-codex-bridge-management-test.sh
rtk git diff --check
```

Expected: PASS.
