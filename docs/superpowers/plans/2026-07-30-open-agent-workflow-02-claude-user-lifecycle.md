# OAW Ticket 02 Claude User Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a complete, idempotent install/update/uninstall lifecycle for the canonical OAW policy and Claude Code user instructions.

**Architecture:** Repository policy content is copied into the OAW XDG configuration namespace, while Claude receives one mechanically owned block importing that absolute canonical path. Prospective content is rendered before writes; a tab-delimited, non-evaluated state file records the clean installation baseline.

**Tech Stack:** Bash 3.2, `cksum`, `awk`, `mktemp`, atomic same-directory file replacement, black-box shell tests.

**Canonical sources:** `.scratch/open-agent-workflow/spec.md`, `.scratch/open-agent-workflow/issues/02-claude-user-lifecycle.md`, ADR 0001, ADR 0002.

---

### Task 1: Install the canonical policy and render Claude instructions

**Files:**
- Create: `policy/ENGINEERING.md`
- Create: `lib/paths.sh`
- Create: `lib/render.sh`
- Create: `tests/03-claude-lifecycle-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Write the failing render contract**

Run a dry internal fixture through the real `install` command and assert the intended policy path is `${XDG_CONFIG_HOME}/open-agent-workflow/ENGINEERING.md`. Assert Claude's managed text contains this exact instruction and import on separate lines:

```markdown
Before any new top-level engineering task that may use workflow skills, read and follow the Open Agent Workflow policy:
@/absolute/isolated/config/open-agent-workflow/ENGINEERING.md
```

- [ ] **Step 2: Verify the test is RED**

Run: `bash tests/03-claude-lifecycle-test.sh`

Expected: exit `69` because install is not implemented.

- [ ] **Step 3: Add the canonical policy source**

Write `policy/ENGINEERING.md` from the approved specification. It must define classification, the blocking choice of `SP-FULL`, `MATT-FULL`, `ECC-FULL`, `MATT-SP-HYBRID`, and `CUSTOM-LOCKED`, exact stage ownership, bounded ECC add-ons, lifecycle persistence, subagent inheritance, stable switching, artifact locations, missing-capability behavior, and the rule that provider detection never selects a profile.

- [ ] **Step 4: Implement path and renderer APIs**

Expose exact functions:

```bash
oaw_config_dir() { printf '%s/open-agent-workflow\n' "$OAW_XDG_CONFIG_HOME"; }
oaw_state_dir() { printf '%s/open-agent-workflow\n' "$OAW_XDG_STATE_HOME"; }
canonical_policy_path() { printf '%s/ENGINEERING.md\n' "$(oaw_config_dir)"; }
render_target_content() { target=$1; policy=$2; case "$target" in claude) render_claude "$policy" ;; esac; }
```

All roots are resolved from already validated CLI environment values; renderers write to stdout and perform no mutation.

- [ ] **Step 5: Commit policy and rendering**

```bash
git add policy/ENGINEERING.md lib/paths.sh lib/render.sh tests/03-claude-lifecycle-test.sh tests/run.sh
git commit -m "feat: define canonical workflow policy"
```

### Task 2: Create and replace one valid managed block

**Files:**
- Create: `lib/managed.sh`
- Modify: `tests/03-claude-lifecycle-test.sh`

- [ ] **Step 1: Add failing managed-content cases**

Prepopulate `~/.claude/CLAUDE.md` with `personal-before` and `personal-after`. Assert install yields exactly one pair of delimiters, preserves both personal lines and their order, places one blank line around the block, and does not treat HTML delimiters as semantic precedence.

- [ ] **Step 2: Verify RED**

Run: `bash tests/03-claude-lifecycle-test.sh`

Expected: FAIL because no target file is written.

- [ ] **Step 3: Implement strict block primitives**

Use fixed markers:

```bash
OAW_BEGIN_MARKER='<!-- BEGIN OPEN AGENT WORKFLOW -->'
OAW_END_MARKER='<!-- END OPEN AGENT WORKFLOW -->'
```

Implement `inspect_managed_block`, `render_block_install`, `render_block_replace`, and `render_block_remove`. Inspection returns clean-absent, clean-present, or drift and rejects duplicate, missing, or reversed markers. Rendering always writes a complete prospective file to a caller-provided temporary path and preserves bytes outside the block.

- [ ] **Step 4: Verify preservation and marker invariants**

Run: `bash tests/03-claude-lifecycle-test.sh`

Expected: managed block assertions pass; lifecycle assertions remain RED until operations are wired.

- [ ] **Step 5: Commit managed-block behavior**

```bash
git add lib/managed.sh tests/03-claude-lifecycle-test.sh
git commit -m "feat: manage isolated instruction blocks"
```

### Task 3: Persist inert installation state

**Files:**
- Create: `lib/checksum.sh`
- Create: `lib/state.sh`
- Modify: `tests/03-claude-lifecycle-test.sh`

- [ ] **Step 1: Add a failing state contract test**

Assert a fresh install creates `${XDG_STATE_HOME}/open-agent-workflow/installations/user.state` with tab-delimited `format`, `version`, `scope`, `policy`, and `target` records. Assert no line is sourced or contains a shell assignment, and recorded checksums match `cksum` output for the managed policy and target block.

- [ ] **Step 2: Verify RED**

Run: `bash tests/03-claude-lifecycle-test.sh`

Expected: FAIL because state is absent.

- [ ] **Step 3: Implement checksums and state serialization**

`checksum_file` emits `<crc>:<bytes>`. `write_state` accepts normalized data records and uses `printf '%s\t%s\n'`; `read_state` validates field counts and known record types with `IFS` and `read`, never `source`, `eval`, command substitution from data, or executable permissions. Reject tabs, CR, and LF in serialized path fields.

- [ ] **Step 4: Verify state round trips as data**

Run: `bash tests/03-claude-lifecycle-test.sh`

Expected: state schema and checksum assertions pass.

- [ ] **Step 5: Commit state persistence**

```bash
git add lib/checksum.sh lib/state.sh tests/03-claude-lifecycle-test.sh
git commit -m "feat: persist inert installation state"
```

### Task 4: Implement fresh install and exact idempotence

**Files:**
- Create: `lib/filesystem.sh`
- Create: `lib/operations.sh`
- Create: `lib/commands/mutate.sh`
- Modify: `install.sh`
- Modify: `tests/03-claude-lifecycle-test.sh`

- [ ] **Step 1: Add failing fresh and repeat install tests**

Assert `install --target claude` creates the policy, Claude block, and state with exit `0`. Snapshot `cksum` and modification time for each file, sleep one second in the test, repeat install, and assert output includes `unchanged: claude`, checksums and mtimes are identical, and no backup exists.

- [ ] **Step 2: Verify RED**

Run: `bash tests/03-claude-lifecycle-test.sh`

Expected: exit `69` from the unimplemented dispatcher.

- [ ] **Step 3: Implement atomic file helpers and install orchestration**

`atomic_replace <prospective> <destination>` creates the destination directory, copies mode `0600` for state and ordinary inherited/readable mode for instructions, and renames a same-directory temporary file. `operation_install` renders every prospective artifact first, validates clean absence or exact installed state, then applies policy, target, and state in that order. It skips `atomic_replace` whenever the prospective checksum equals the current checksum.

- [ ] **Step 4: Wire mutating commands**

`command_mutate` normalizes scope and targets, rejects extension user targets, and calls `operation_install`, `operation_update`, or `operation_uninstall`. Replace the temporary exit `69` dispatch in `install.sh`.

- [ ] **Step 5: Verify install and idempotence**

Run: `bash tests/03-claude-lifecycle-test.sh`

Expected: fresh install and repeat install pass; update and uninstall cases remain RED until their steps.

- [ ] **Step 6: Commit the first vertical lifecycle**

```bash
git add install.sh lib/filesystem.sh lib/operations.sh lib/commands/mutate.sh tests/03-claude-lifecycle-test.sh
git commit -m "feat: install Claude workflow instructions"
```

### Task 5: Add local update and mutation-free dry run

**Files:**
- Modify: `lib/operations.sh`
- Modify: `lib/commands/mutate.sh`
- Modify: `tests/03-claude-lifecycle-test.sh`

- [ ] **Step 1: Add failing update and dry-run tests**

Install from a copied checkout, alter that checkout's `VERSION` and policy fixture, then run `update --target claude`. Assert only local checkout bytes appear in config and state. For every mutating command with `--dry-run`, snapshot the full isolated tree before and after and assert equality while output begins each proposed action with `would-create`, `would-update`, or `would-remove`.

- [ ] **Step 2: Verify RED**

Run: `bash tests/03-claude-lifecycle-test.sh`

Expected: update or dry-run mutation assertions fail.

- [ ] **Step 3: Implement update from checkout artifacts**

Require existing valid state, compare current files with recorded checksums, render the repository's current `VERSION` and `policy/ENGINEERING.md`, and replace only changed clean artifacts. Do not invoke `curl`, `wget`, Git network commands, a package manager, or an interpreter download.

- [ ] **Step 4: Implement a centralized apply/dry-run boundary**

Every planned create, update, remove, and state write passes through `apply_action`; when `OAW_DRY_RUN=1`, it prints the action and returns without `mkdir`, `cp`, `mv`, `rm`, or state changes.

- [ ] **Step 5: Verify updates and dry runs**

Run: `bash tests/03-claude-lifecycle-test.sh`

Expected: update uses only copied-checkout content and each dry run leaves an identical filesystem snapshot.

- [ ] **Step 6: Commit update and preview behavior**

```bash
git add lib/operations.sh lib/commands/mutate.sh tests/03-claude-lifecycle-test.sh
git commit -m "feat: update and preview managed workflow files"
```

### Task 6: Remove only OAW-owned Claude artifacts

**Files:**
- Modify: `lib/operations.sh`
- Modify: `lib/state.sh`
- Modify: `tests/03-claude-lifecycle-test.sh`

- [ ] **Step 1: Add failing clean uninstall cases**

Cover an OAW-created Claude file, a user-owned Claude file surrounding the block, repeated uninstall, and retained canonical policy while another synthetic state references it. Assert created-empty files are removed, user files retain exact outside content, repeated uninstall reports unchanged, and state disappears only after its managed artifacts are removed.

- [ ] **Step 2: Verify RED**

Run: `bash tests/03-claude-lifecycle-test.sh`

Expected: uninstall exits without removing the managed artifacts.

- [ ] **Step 3: Implement reference-aware uninstall**

Use recorded ownership (`created-file` or `managed-block`) and destination checksums. Remove the block from shared files; remove a created file only when its post-removal content is empty; retain the canonical policy while any valid installation state references it. Never remove parent directories unless OAW created them and they are empty.

- [ ] **Step 4: Run Ticket 02 verification**

Run: `bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh && bash tests/run.sh`

Expected: every Ticket 01 and Ticket 02 black-box case passes with no access outside isolated roots.

- [ ] **Step 5: Commit the Claude lifecycle**

```bash
git add lib/operations.sh lib/state.sh tests/03-claude-lifecycle-test.sh
git commit -m "feat: uninstall owned Claude workflow content"
```

