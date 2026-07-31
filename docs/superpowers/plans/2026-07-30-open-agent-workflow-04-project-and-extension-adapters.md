# OAW Ticket 04 Project and Extension Adapters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Install project-scoped OAW entrypoints for all nine supported agent tools without modifying their user instruction files.

**Architecture:** Project destinations are fixed relative paths beneath a
physically resolved project root. The existing grouped lifecycle engine takes
scope and state-root inputs instead of assuming user scope. The canonical
policy remains in OAW's XDG config namespace, while each project's installation
state is isolated by a deterministic project identity and validates the stored
physical root before use. Codex and OpenCode share one project `AGENTS.md`
bootstrap; extension targets use whole-file ownership. Because every scope
references one stable canonical policy path, an operation that changes that
policy prepares metadata replacements for every valid referencing state while
leaving other scopes' adapter files untouched. Hostile symlink and TOCTOU
containment remains the release-blocking responsibility of Ticket 05 Task 3.

**Tech Stack:** Bash 3.2, Markdown/MDC target formats, `cksum` project identity, existing lifecycle engine and black-box harness.

**Canonical sources:** `.scratch/open-agent-workflow/spec.md`, `.scratch/open-agent-workflow/issues/04-project-and-extension-adapters.md`, core and extension adapter research.

---

### Task 1: Resolve project identities and core destinations

**Files:**
- Modify: `lib/paths.sh`
- Modify: `lib/targets.sh`
- Modify: `lib/state.sh`
- Modify: `lib/render.sh`
- Modify: `lib/operations.sh`
- Create: `tests/05-project-adapters-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Write failing project identity and destination tests**

Create a project named `project with spaces`. In separate tracer bullets, install
each core target with `--project` and assert these destinations:
`.claude/CLAUDE.md`, shared `AGENTS.md` for Codex and OpenCode, and `GEMINI.md`.
Assert the project state filename equals the `cksum` identity of the physical
root, the state contains the exact root, repeat install is unchanged, and no
target file beneath isolated user configuration changes.

- [ ] **Step 2: Verify RED**

Run: `bash tests/05-project-adapters-test.sh`

Expected: FAIL because the lifecycle engine still writes user-scoped state and
project destinations are undefined.

- [ ] **Step 3: Implement deterministic project state identity**

Compute `<crc>-<bytes>` from the physical project-root bytes with `cksum` and
store state at
`${XDG_STATE_HOME}/open-agent-workflow/installations/projects/<identity>.state`.
Extend state read/write and normalized-record validation to accept explicit
`user` or `project` scope. Project state contains one `project` record with the
exact physical root; user state contains none. Operation verification rejects
a stored root mismatch before rendering or mutation, so a checksum collision
cannot attach state to another project.

- [ ] **Step 4: Register fixed project-relative core paths**

Expose `target_project_relative_path` with an exact `case` table. `target_destination` joins only registry-owned relative paths to `OAW_PROJECT_ROOT`; no user-provided target path reaches the join.

Generalize install orchestration helpers to pass `OAW_SCOPE`,
`OAW_PROJECT_ROOT`, and the scope-specific state path through state merge and
verification. Preserve the current user lifecycle unchanged.

- [ ] **Step 5: Add the shared project bootstrap renderer**

At project scope, Codex and OpenCode must render this identical managed body in
their shared `AGENTS.md` destination:

```text
Before engineering lifecycle work, read `<absolute-policy-path>`, follow its blocking selection gate, and preserve the selected lifecycle bundle for the task.
```

This is a model-visible instruction, not a Markdown import claim. Claude and
Gemini retain their documented import forms.

- [ ] **Step 6: Verify project paths and user isolation**

Run: `bash tests/05-project-adapters-test.sh`

Expected: project identity, stored-root validation, core destinations, shared
bootstrap bytes, repeat install, and user-target isolation pass. Extension
targets remain unsupported.

- [ ] **Step 7: Commit project scope foundations**

```bash
git add lib/paths.sh lib/targets.sh lib/state.sh lib/render.sh lib/operations.sh tests/05-project-adapters-test.sh tests/run.sh
git commit -m "feat: isolate project adapter state"
```

### Task 2: Render all extension adapter formats

**Files:**
- Modify: `lib/targets.sh`
- Modify: `lib/render.sh`
- Modify: `lib/operations.sh`
- Modify: `tests/05-project-adapters-test.sh`

- [ ] **Step 1: Add failing exact-format tests**

Assert fixed destinations and format requirements:

```text
cursor    .cursor/rules/open-agent-workflow.mdc
windsurf .devin/rules/open-agent-workflow.md
cline     .clinerules/open-agent-workflow.md
roo       .roo/rules/open-agent-workflow.md
copilot   .github/instructions/open-agent-workflow.instructions.md
```

Cursor must start with valid `description`, `globs: "**/*"`, and
`alwaysApply: true` frontmatter. Windsurf must use `trigger: always_on`.
Copilot must use `applyTo: "**"`. Every body uses the shared project bootstrap
sentence from Task 1. Pre-existing sibling files remain byte-identical; a
pre-existing owned destination is rejected before mutation.

- [ ] **Step 2: Verify RED**

Run: `bash tests/05-project-adapters-test.sh`

Expected: FAIL because extension renderers are missing.

- [ ] **Step 3: Implement owned-file renderers**

Add one pure renderer per extension target. Each renderer prints its complete
file, including frontmatter where documented. Set `target_ownership` to
`owned-file` for the five unique extension paths and `managed-block` for core
instruction files.

Generalize artifact rendering, installed checksum verification, update, and
uninstall by ownership mode. A managed-block checksum covers the extracted OAW
block; an owned-file checksum covers the complete file. Fresh owned-file
install requires an absent destination, update replaces only a clean recorded
file, and uninstall removes only a clean recorded owned file.

- [ ] **Step 4: Verify exact adapter bytes**

Run: `bash tests/05-project-adapters-test.sh`

Expected: all path, frontmatter, canonical-path, lifecycle-lock, owned-file
conflict, and sibling-preservation assertions pass.

- [ ] **Step 5: Commit extension rendering**

```bash
git add lib/targets.sh lib/render.sh lib/operations.sh tests/05-project-adapters-test.sh
git commit -m "feat: render project extension adapters"
```

### Task 3: Deduplicate shared project instruction surfaces

**Files:**
- Modify: `lib/operations.sh`
- Modify: `lib/state.sh`
- Modify: `tests/05-project-adapters-test.sh`

- [ ] **Step 1: Add failing shared-AGENTS tests**

Install `codex,opencode` together and assert `AGENTS.md` has one OAW block,
state has both target rows, and the destination checksum is identical in both
rows. Uninstall Codex only and assert the block remains for OpenCode; uninstall
OpenCode and assert only the block is removed. With both user and project state
installed, uninstall either scope and assert the shared canonical policy
survives until the final state reference is removed.

- [ ] **Step 2: Verify RED**

Run: `bash tests/05-project-adapters-test.sh`

Expected: FAIL from duplicate writes or premature removal.

- [ ] **Step 3: Group operation actions by physical destination**

Reuse normalized action records keyed by exact destination. If two targets
share a path, require identical ownership and rendered content; otherwise stop
before mutation with an internal contract error. Apply the grouped action once
and attach every target reference to its resulting managed checksum. Extend
policy-reference discovery across flat user states and nested project states.

- [ ] **Step 4: Verify shared ownership lifecycle**

Run: `bash tests/05-project-adapters-test.sh`

Expected: one block survives until the final target reference is uninstalled,
and the canonical policy survives until the final cross-scope state reference
is removed.

- [ ] **Step 5: Commit destination deduplication**

```bash
git add lib/operations.sh lib/state.sh tests/05-project-adapters-test.sh
git commit -m "feat: share project AGENTS ownership"
```

### Task 4: Complete the nine-target project lifecycle matrix

**Files:**
- Modify: `lib/operations.sh`
- Modify: `lib/commands/check.sh`
- Modify: `tests/05-project-adapters-test.sh`

- [ ] **Step 1: Add failing project matrix tests**

For every target and the default nine-target set, exercise install, repeated
install, copied-checkout selected update, install/update/uninstall dry runs,
selected uninstall, and final uninstall. Seed user content in managed-block
files and unrelated sibling files beside owned extension destinations. Assert
preserved bytes, scope-specific state, and no user-target changes.

- [ ] **Step 2: Verify RED**

Run: `bash tests/05-project-adapters-test.sh`

Expected: one or more lifecycle matrix cases fail.

- [ ] **Step 3: Generalize project operation and diagnostics**

Complete project update and uninstall through the scope-generic grouped engine.
`check --project` validates the stored physical root and reports selected
adapter support plus `not-installed`, `clean`, `drift`, or `invalid-state`
without writing. Project install may ensure the OAW canonical policy exists in
OAW's XDG config, but it must never edit a tool's user-level instruction
destination.

- [ ] **Step 4: Verify paths containing spaces and deterministic defaults**

Run: `bash tests/05-project-adapters-test.sh`

Expected: all nine individual adapters and the registry-ordered default set pass in a spaced project path.

- [ ] **Step 5: Run Ticket 04 verification**

Run: `bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh && bash tests/run.sh`

Expected: all Ticket 01-04 tests pass; user and project repeated installs are byte-for-byte and mtime unchanged.

- [ ] **Step 6: Commit project lifecycle support**

```bash
git add lib/operations.sh lib/commands/check.sh tests/05-project-adapters-test.sh
git commit -m "feat: manage project workflow adapters"
```

### Task 5: Coordinate the shared canonical policy across scopes

**Files:**
- Modify: `lib/state.sh`
- Modify: `lib/operations.sh`
- Create: `tests/05-policy-coordination-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Write the failing cross-scope policy tests**

Use a new test file because `tests/05-project-adapters-test.sh` is already 794
lines. Source `tests/test-helper.sh`, use `setup_sandbox`/`run_oaw`, and register
the new script immediately after the project-adapter suite in `tests/run.sh`.
Install one user target and one project target from the original checkout, copy
`install.sh`, `lib/`, `policy/`, and `VERSION` into a sandbox update checkout,
then change both copied `VERSION` and copied `policy/ENGINEERING.md`. Cover these
tracer bullets:

```text
project update -> user state receives the new version/checksum -> both checks clean
user update -> project state receives the new version/checksum -> both checks clean
new-scope install from a changed checkout -> older scope state is synchronized
dry run -> policy, every state, and every adapter fingerprint remain unchanged
updated scope final uninstall -> policy survives for the other clean scope
final remaining-scope uninstall -> policy and final state are removed
```

For every coordinated operation, snapshot the non-selected adapter's path,
checksum, inode, mtime, and size and require an identical fingerprint afterward.
Require every referencing state to contain the source checkout's literal version
and the exact checksum of the installed canonical policy.

- [ ] **Step 2: Verify RED**

Run: `bash tests/05-policy-coordination-test.sh`

Expected: FAIL after the first changed-checkout update because the other scope
reports `drift`; uninstalling the updated scope may also remove the still-needed
canonical policy.

- [ ] **Step 3: Prepare replacement metadata for every valid policy reference**

Keep the stable policy path from ADR 0002. In `lib/state.sh`, make policy
reference discovery path-based after full state parsing:

```bash
state_file_references_policy() (
  local input_file=$1
  local policy_path=$2
  local target_records=

  target_records=$(mktemp "${TMPDIR:-/tmp}/oaw-state-records.XXXXXX") ||
    die "cannot create state validation workspace" 73
  trap 'rm -f -- "$target_records"' EXIT HUP INT TERM
  load_state_file "$input_file" "$target_records"
  [ "$STATE_POLICY_PATH" = "$policy_path" ]
)
```

Change `other_state_references_policy` and its uninstall caller to use the
validated path reference without requiring the old checksum. This prevents an
installed scope from losing the policy merely because another state records an
older checksum.

In `lib/operations.sh`, add this Bash 3.2-compatible preparation helper:

```bash
prepare_policy_state_actions() {
  local source_version=$1
  local new_policy_checksum=$2
  local excluded_state=$3
  local actions_file=$4
  local installed_policy_checksum=
  local candidate_state=
  local candidate_records=
  local candidate_output=
  local candidate_index=0

  if [ -f "$OAW_POLICY_DESTINATION" ]; then
    installed_policy_checksum=$(checksum_file "$OAW_POLICY_DESTINATION")
  fi

  for candidate_state in \
    "$OAW_INSTALLATIONS_DIR"/*.state \
    "$OAW_INSTALLATIONS_DIR"/projects/*.state; do
    [ -e "$candidate_state" ] || continue
    [ "$candidate_state" = "$excluded_state" ] && continue
    candidate_index=$((candidate_index + 1))
    candidate_records="$OAW_OPERATION_TEMP/policy-records-$candidate_index"
    candidate_output="$OAW_OPERATION_TEMP/policy-state-$candidate_index"
    load_state_file "$candidate_state" "$candidate_records"
    [ "$STATE_POLICY_PATH" = "$OAW_POLICY_DESTINATION" ] || continue
    [ -n "$installed_policy_checksum" ] &&
      [ "$STATE_POLICY_CHECKSUM" = "$installed_policy_checksum" ] ||
      die "managed policy has drifted" 65
    write_state_file "$candidate_output" "$source_version" "$STATE_SCOPE" \
      "$STATE_PROJECT_ROOT" "$STATE_POLICY_PATH" "$new_policy_checksum" \
      "$candidate_records"
    add_target_action "$actions_file" replace "state-reference-$candidate_index" \
      "$candidate_output" "$candidate_state" 600
  done
}
```

The helper enumerates `installations/*.state` and
`installations/projects/*.state`, fully parses every candidate, skips the
current scope state and states for another policy path, and requires each
referencing state's recorded checksum to match the currently installed policy
before preparing any mutation. For each clean reference it writes a new state
file in `OAW_OPERATION_TEMP` with the new version/checksum and its original
scope, project root, and target records, then adds a mode-600 replace action.
Any invalid or stale referencing state aborts before policy, target, or state
writes.

After rendering the current scope state, install and update use this exact
prepare/apply order:

```bash
: >"$OAW_OPERATION_TEMP/state-actions"
write_operation_state "$source_version" "$OAW_OPERATION_TEMP/final-records"
new_policy_checksum=$(checksum_file "$OAW_OPERATION_TEMP/policy")
add_target_action "$OAW_OPERATION_TEMP/state-actions" replace state \
  "$OAW_OPERATION_TEMP/state" "$OAW_STATE_FILE" 600
prepare_policy_state_actions "$source_version" "$new_policy_checksum" \
  "$OAW_STATE_FILE" "$OAW_OPERATION_TEMP/state-actions"

apply_replace policy "$OAW_OPERATION_TEMP/policy" "$OAW_POLICY_DESTINATION" 600
apply_target_actions "$OAW_OPERATION_TEMP/target-actions"
apply_target_actions "$OAW_OPERATION_TEMP/state-actions"
```

Declare `new_policy_checksum` locally in both operations. Remove their old
direct `apply_replace state ...` calls. Dry run traverses the same action
manifests but writes nothing. Other scopes' target action manifests remain
empty.

- [ ] **Step 4: Verify coordinated lifecycle behavior**

Run: `bash tests/05-policy-coordination-test.sh`

Expected: all user-to-project, project-to-user, new-scope install, dry-run,
clean-check, fingerprint-preservation, and reference-aware uninstall cases pass.

- [ ] **Step 5: Run Ticket 04 verification**

Run: `bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh && bash tests/run.sh`

Expected: all Ticket 01-04 cases pass, both scope checks remain clean after a
policy change, non-selected adapter fingerprints are unchanged, and the policy
survives until the final validated path reference is removed.

- [ ] **Step 6: Commit policy-state coordination**

```bash
git add lib/state.sh lib/operations.sh tests/05-policy-coordination-test.sh tests/run.sh
git commit -m "fix: synchronize canonical policy state"
```
