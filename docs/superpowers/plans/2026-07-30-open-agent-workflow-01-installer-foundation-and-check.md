# OAW Ticket 01 Installer Foundation and Check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a portable, read-only OAW CLI foundation with deterministic target and provider detection.

**Architecture:** `install.sh` resolves its checkout and sources focused Bash modules; it never exposes sourceable installer internals as a public API. A data-oriented target registry and detector feed the `check` command, while tests invoke the real entrypoint with isolated environment roots.

**Tech Stack:** Bash 3.2, POSIX userland utilities, a dependency-free shell test harness.

**Canonical sources:** `.scratch/open-agent-workflow/spec.md`, `.scratch/open-agent-workflow/issues/01-installer-foundation-and-check.md`, `CONTEXT.md`.

---

### Task 1: Establish the black-box harness and inert entrypoint

**Files:**
- Create: `VERSION`
- Create: `.gitignore`
- Create: `install.sh`
- Create: `tests/test-helper.sh`
- Create: `tests/run.sh`
- Create: `tests/01-cli-test.sh`

- [ ] **Step 1: Write the no-argument and help tests**

Create a harness whose `run_oaw` always launches a new Bash process and injects isolated roots:

```bash
run_oaw() {
  OAW_OUTPUT_FILE="$OAW_SANDBOX/output"
  set +e
  HOME="$OAW_HOME" XDG_CONFIG_HOME="$OAW_CONFIG" XDG_STATE_HOME="$OAW_STATE" \
    bash "$OAW_INSTALLER" "$@" >"$OAW_OUTPUT_FILE" 2>&1
  OAW_STATUS=$?
  set -e
  OAW_OUTPUT=$(cat "$OAW_OUTPUT_FILE")
}
```

Assert that no arguments exits `0`, includes `Usage: ./install.sh <command> [options]`, and leaves the isolated home, config, and state directories empty. Assert the same invariants for `--help` and `help`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/01-cli-test.sh`

Expected: non-zero with an assertion showing `install.sh` is missing.

- [ ] **Step 3: Add the version and inert CLI shell**

Set `VERSION` to `0.1.0`. Ignore only OS/editor debris and test-owned temporary
output in `.gitignore`; keep `.scratch` lifecycle evidence tracked. Create an
executable `install.sh` with `set -eu`, a physical `OAW_SOURCE_DIR`, and a
`usage` function listing `check`, `install`, `update`, `uninstall`, `--target`,
`--project`, `--dry-run`, `--force`, and `--help`. For no arguments, `help`,
`-h`, or `--help`, print usage and exit without creating directories.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/01-cli-test.sh`

Expected: `PASS: no arguments are inert` and `PASS: help is inert`.

- [ ] **Step 5: Commit the harness foundation**

```bash
git add VERSION .gitignore install.sh tests/test-helper.sh tests/run.sh tests/01-cli-test.sh
git commit -m "test: establish black-box installer harness"
```

### Task 2: Parse and validate the public CLI contract

**Files:**
- Create: `lib/common.sh`
- Create: `lib/cli.sh`
- Modify: `install.sh`
- Modify: `tests/01-cli-test.sh`

- [ ] **Step 1: Add failing parser contract cases**

Add table-driven invocations for `unknown`, `check --bogus`, `check --target`, `check --project`, `check --dry-run`, and `check --force`. Assert unknown syntax exits `64`; missing values name the flag; and mutating-only flags on `check` are rejected before any isolated path changes.

- [ ] **Step 2: Run the parser tests to verify RED**

Run: `bash tests/01-cli-test.sh`

Expected: FAIL because the skeleton does not parse options or return exit `64`.

- [ ] **Step 3: Implement a Bash 3.2-compatible parser**

Define readonly-by-convention globals without associative arrays:

```bash
OAW_COMMAND=
OAW_TARGETS=
OAW_PROJECT=
OAW_DRY_RUN=0
OAW_FORCE=0

cli_error() { printf 'oaw: error: %s\n' "$*" >&2; return 64; }
```

`parse_cli` must consume arguments with a `while [ "$#" -gt 0 ]`/`case` loop, accept both `--target value` and `--target=value`, accept one `--project`, reject duplicate scalar flags, and reject `--dry-run`/`--force` for `check`. `common.sh` supplies `die`, `note`, `warn`, `command_exists`, and `require_absolute_root`; no function creates files during parsing.

- [ ] **Step 4: Route parsed commands from the entrypoint**

Source modules from the resolved checkout, parse once, and dispatch only after successful validation:

```bash
case "$OAW_COMMAND" in
  check) command_check ;;
  install|update|uninstall) die "command not implemented: $OAW_COMMAND" 69 ;;
esac
```

The temporary exit `69` is asserted only until Ticket 02 replaces it.

- [ ] **Step 5: Verify parser behavior and Bash syntax**

Run: `bash -n install.sh lib/common.sh lib/cli.sh tests/*.sh && bash tests/01-cli-test.sh`

Expected: syntax exits `0`; every parser contract case passes.

- [ ] **Step 6: Commit the CLI parser**

```bash
git add install.sh lib/common.sh lib/cli.sh tests/01-cli-test.sh
git commit -m "feat: validate installer command line"
```

### Task 3: Register targets and resolve scope without mutation

**Files:**
- Create: `lib/targets.sh`
- Modify: `tests/01-cli-test.sh`

- [ ] **Step 1: Add failing target and scope tests**

Cover all canonical IDs: `claude`, `codex`, `gemini`, `opencode`, `cursor`, `windsurf`, `cline`, `roo`, and `copilot`. Verify user defaults are exactly `claude,codex,gemini,opencode`; project defaults contain all nine in registry order; duplicates normalize once; whitespace and empty CSV members fail; and extension targets at user scope fail with exit `64`.

- [ ] **Step 2: Run the target tests to verify RED**

Run: `bash tests/01-cli-test.sh`

Expected: FAIL because target normalization is undefined.

- [ ] **Step 3: Implement the ordered registry**

Use newline records rather than Bash 4 maps:

```bash
target_ids() {
  printf '%s\n' claude codex gemini opencode cursor windsurf cline roo copilot
}

target_supports_user() {
  case "$1" in claude|codex|gemini|opencode) return 0 ;; *) return 1 ;; esac
}
```

Implement `normalize_targets <csv-or-empty> <user|project>` by iterating the registry, matching exact comma-delimited members, rejecting unknown/empty members, and emitting a normalized comma list. Resolve `--project` with `cd -P` only after confirming it is an existing directory; reject control characters in roots.

- [ ] **Step 4: Verify deterministic target results**

Run: `bash tests/01-cli-test.sh`

Expected: all target and project-resolution assertions pass, including a project path containing spaces.

- [ ] **Step 5: Commit target registration**

```bash
git add lib/targets.sh tests/01-cli-test.sh
git commit -m "feat: register supported workflow targets"
```

### Task 4: Detect providers and tools through the check command

**Files:**
- Create: `lib/detect.sh`
- Create: `lib/commands/check.sh`
- Modify: `install.sh`
- Create: `tests/02-check-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Write failing provider and tool detection tests**

Build three isolated fixtures: empty, Matt-only with `~/.agents/skills/to-spec/SKILL.md`, `to-tickets/SKILL.md`, and `tdd/SKILL.md`, and all-providers with explicit indicator files. Add fake core executables to an isolated `PATH`. Assert stable lines such as:

```text
provider superpowers: missing
provider matt: detected
provider ecc: missing
target claude: detected (user, project)
target cursor: adapter-only (project)
```

Assert detection never creates OAW config or state.

- [ ] **Step 2: Run the check tests to verify RED**

Run: `bash tests/02-check-test.sh`

Expected: FAIL because `command_check` does not exist.

- [ ] **Step 3: Implement evidence-based detection**

`detect.sh` must use fixed, documented indicators and exact capability bundles. Matt is detected only when `to-spec`, `to-tickets`, `tdd`, and `diagnosing-bugs` skill files exist. ECC detection accepts its global skill indicator. Superpowers accepts its Claude or Codex plugin indicator. Core tools are detected by command presence or their instruction root; extension tools report `adapter-only` because project rules do not prove editor installation.

- [ ] **Step 4: Render deterministic check output**

`command_check` prints version, resolved scope, selected targets, provider statuses, and selected target statuses. A missing provider is diagnostic and does not change exit status; invalid CLI or scope remains an error.

- [ ] **Step 5: Run the complete Ticket 01 verification**

Run: `bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh && bash tests/run.sh`

Expected: syntax exits `0`; the runner reports all Ticket 01 cases passed; isolated config and state roots remain empty.

- [ ] **Step 6: Commit the read-only foundation**

```bash
git add install.sh lib/detect.sh lib/commands/check.sh tests/02-check-test.sh tests/run.sh
git commit -m "feat: add workflow readiness check"
```
