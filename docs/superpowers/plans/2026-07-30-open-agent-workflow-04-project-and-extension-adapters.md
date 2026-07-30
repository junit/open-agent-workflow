# OAW Ticket 04 Project and Extension Adapters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Install project-scoped OAW entrypoints for all nine supported agent tools without modifying their user instruction files.

**Architecture:** Project destinations are fixed relative paths beneath a
physically resolved project root. The existing grouped lifecycle engine takes
scope and state-root inputs instead of assuming user scope. The canonical
policy remains in OAW's XDG config namespace, while each project's installation
state is isolated by a deterministic project identity and validates the stored
physical root before use. Codex and OpenCode share one project `AGENTS.md`
bootstrap; extension targets use whole-file ownership.

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
