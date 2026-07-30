# OAW Ticket 03 Core User Adapters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the complete user lifecycle to Codex CLI, Gemini CLI, and OpenCode while sharing one canonical policy.

**Architecture:** Task 1 replaces the Claude-only install path with a generic
single-target path while the target registry owns destinations and ownership
modes and pure renderers own target-native content. Task 2 then evolves the
single-row state into normalized, registry-ordered target records; Task 3 uses
those records for update, dry-run, and reference-aware uninstall.

**Tech Stack:** Bash 3.2, target-native Markdown entrypoints, existing black-box harness.

**Canonical sources:** `.scratch/open-agent-workflow/spec.md`, `.scratch/open-agent-workflow/issues/03-core-user-adapters.md`, core adapter research cited in `docs/` during Ticket 06.

---

### Task 1: Install one core target through target-native rendering

**Files:**
- Modify: `lib/targets.sh`
- Modify: `lib/paths.sh`
- Modify: `lib/render.sh`
- Modify: `lib/state.sh`
- Modify: `lib/operations.sh`
- Create: `tests/04-core-adapters-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Write the failing single-target install contracts**

Create `tests/04-core-adapters-test.sh` using `tests/test-helper.sh`. For each
of `codex`, `gemini`, and `opencode`, start from a fresh isolated sandbox,
preseed the destination with `personal instruction`, run the real command
`install --target <id>`, and assert exit `0`, exact destination, preserved user
content, one managed block, and a state target row containing the selected ID.
Repeat the same install and assert `unchanged: <id>` with unchanged checksums.

Use these exact destinations and managed bodies:

```bash
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
case "$target_id" in
  codex)
    target_path=$OAW_HOME/.codex/AGENTS.md
    expected_body="For every new top-level engineering task that may use workflow skills, first read \`$OAW_POLICY\`, run its blocking selection gate, and preserve the selected lifecycle bundle for the task."
    ;;
  gemini)
    target_path=$OAW_HOME/.gemini/GEMINI.md
    expected_body=$(printf 'Follow the Open Agent Workflow policy before engineering lifecycle work:\n@%s' "$OAW_POLICY")
    ;;
  opencode)
    target_path=$OAW_CONFIG/opencode/AGENTS.md
    expected_body="Before engineering lifecycle work, use the Read tool to read \`$OAW_POLICY\`, then follow its blocking selection gate and lifecycle lock."
    ;;
esac
```

Codex and OpenCode must not contain a standalone `@<policy-path>` line;
Gemini must contain it. Add `04-core-adapters-test.sh` to `tests/run.sh` only
after the focused script is green.

- [ ] **Step 2: Verify RED**

Run: `bash tests/04-core-adapters-test.sh`

Expected: FAIL on the first core target with exit `69` and
`Ticket 02 install supports only target 'claude'`.

- [ ] **Step 3: Implement target-native renderers**

Add `target_ownership` for the four core user targets, destination cases for
the three new targets, and pure renderers with these exact semantic forms:

```bash
render_codex() {
  printf 'For every new top-level engineering task that may use workflow skills, first read `%s`, run its blocking selection gate, and preserve the selected lifecycle bundle for the task.\n' "$1"
}
render_gemini() { printf 'Follow the Open Agent Workflow policy before engineering lifecycle work:\n@%s\n' "$1"; }
render_opencode() {
  printf 'Before engineering lifecycle work, use the Read tool to read `%s`, then follow its blocking selection gate and lifecycle lock.\n' "$1"
}
```

Do not claim that Codex or OpenCode interprets Markdown file references as imports.

- [ ] **Step 4: Generalize the one-row state and install path**

Keep the state format at exactly one target row in this task, but accept any
core user target instead of only Claude. Replace Claude-specific artifact and
verification functions with these interfaces:

```bash
render_target_artifacts <target-id> <target-path> <origin> <source-version>
verify_current_target_state <target-id> <target-path>
```

`operation_install` must reject comma-separated selections with an explicit
Ticket 03 not-yet-supported error, then use the selected target ID for
`target_destination`, `render_managed_block`, state serialization,
verification, and `apply_replace`. Preserve the existing Claude behavior and
do not generalize update or uninstall in this task.

- [ ] **Step 5: Verify single-target installs and regressions**

Run: `bash tests/04-core-adapters-test.sh`

Expected: all exact-content, preservation, state-ID, repeat-install, and
idempotence assertions pass.

Run: `bash tests/03-claude-lifecycle-test.sh`

Expected: all existing Claude lifecycle assertions pass.

- [ ] **Step 6: Commit single-target core installs**

```bash
git add lib/targets.sh lib/paths.sh lib/render.sh lib/state.sh lib/operations.sh tests/04-core-adapters-test.sh tests/run.sh
git commit -m "feat: install core agent entrypoints"
```

### Task 2: Merge selected installs into one scope state

**Files:**
- Modify: `lib/state.sh`
- Modify: `lib/operations.sh`
- Modify: `tests/04-core-adapters-test.sh`

- [ ] **Step 1: Add failing selection and merge tests**

Install `claude,codex`, then install `gemini`, then repeat with
`codex,claude,claude`. Assert state target rows are unique and ordered
`claude`, `codex`, `gemini`, `opencode`; existing clean targets are retained;
and only newly selected destinations and state change. Assert an omitted
`--target` selects the four core targets, while `--target all` is rejected as
an unknown target.

- [ ] **Step 2: Verify RED**

Run: `bash tests/04-core-adapters-test.sh`

Expected: FAIL because `operation_install` still accepts exactly one target
and the state parser still requires exactly one target row.

- [ ] **Step 3: Implement target-state merging**

Parse existing state into a caller-provided normalized target-record file.
Validate every target ID, destination, mode, checksum, origin, duplicate, and
registry order before any mutation. Add these interfaces:

```bash
load_state_file <state-file> <normalized-target-records>
write_state_file <output> <version> <scope> <policy-path> <policy-checksum> <normalized-target-records>
merge_install_target_records <existing-records> <selected-records> <merged-records>
```

Each normalized record remains tab-delimited as
`<id>\t<path>\tmanaged-block\t<checksum>\t<origin>`. Render selected target
artifacts first, merge by exact ID, and regenerate state in registry order.
An installed clean target remains unchanged; `install --target` adds missing
targets. Selected update behavior remains Task 3.

- [ ] **Step 4: Verify deterministic merged state**

Run: `bash tests/04-core-adapters-test.sh`

Expected: selection, deduplication, add-target, default-target, deterministic
ordering, and repeat-install cases pass. The full Ticket 01-03 suite is green.

- [ ] **Step 5: Commit multi-target state behavior**

```bash
git add lib/state.sh lib/operations.sh tests/04-core-adapters-test.sh
git commit -m "feat: merge selected user targets"
```

### Task 3: Complete lifecycle operations for every core adapter

**Files:**
- Modify: `lib/operations.sh`
- Modify: `tests/04-core-adapters-test.sh`

- [ ] **Step 1: Add failing lifecycle matrix tests**

For each core target and the default four-target set, exercise copied-checkout
update, install/update/uninstall dry runs, selected uninstall, and final
uninstall. Seed every destination with target-specific user content and assert
it survives. Assert update rejects a selected target absent from state. After
removing one target, assert remaining target rows, destinations, state, and
canonical policy survive.

- [ ] **Step 2: Verify RED**

Run: `bash tests/04-core-adapters-test.sh`

Expected: at least one add/update/uninstall matrix row fails.

- [ ] **Step 3: Generalize operation records**

Build target actions from the Task 1 registry functions
`target_destination`, `target_ownership`, and `render_target_content`. Group
actions by destination before mutation. A destination receives at most one
prospective write and one state checksum per operation, even if multiple target
rows reference it. `update --target` updates selected installed targets only
and rejects a selected target absent from state.

- [ ] **Step 4: Implement reference-aware selected uninstall**

Remove selected rows first in the prospective state. Remove a destination's block only when no remaining row references that destination. Keep policy and state directories until the final installation reference is gone.

- [ ] **Step 5: Verify all core lifecycles**

Run: `bash tests/04-core-adapters-test.sh`

Expected: all target and multi-target matrix cases pass.

- [ ] **Step 6: Commit the core lifecycle matrix**

```bash
git add lib/operations.sh tests/04-core-adapters-test.sh
git commit -m "feat: manage all core user adapters"
```

### Task 4: Regress read-only detection against installed state

**Files:**
- Modify: `lib/commands/check.sh`
- Modify: `tests/02-check-test.sh`
- Modify: `tests/04-core-adapters-test.sh`

- [ ] **Step 1: Add failing post-install check tests**

After default install, assert `check` reports each core target as tool-detected or not-detected separately from `installed: clean`. Modify one managed block and assert check reports `installed: drift` but remains read-only.

- [ ] **Step 2: Verify RED**

Run: `bash tests/02-check-test.sh && bash tests/04-core-adapters-test.sh`

Expected: FAIL because check does not inspect state cleanliness.

- [ ] **Step 3: Add non-mutating install-state diagnostics**

Reuse validation and checksum readers without calling render/apply functions. Report `not-installed`, `clean`, `drift`, or `invalid-state` for each selected target. Provider/tool detection remains orthogonal and cannot alter profile recommendations.

- [ ] **Step 4: Run Ticket 03 verification**

Run: `bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh && bash tests/run.sh`

Expected: all Ticket 01-03 tests pass; rerunning the default install produces no filesystem changes.

- [ ] **Step 5: Commit installed-state checks**

```bash
git add lib/commands/check.sh tests/02-check-test.sh tests/04-core-adapters-test.sh
git commit -m "feat: report adapter installation health"
```
