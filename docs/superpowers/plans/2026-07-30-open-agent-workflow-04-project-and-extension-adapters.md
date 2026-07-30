# OAW Ticket 04 Project and Extension Adapters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Install project-scoped OAW entrypoints for all nine supported agent tools without modifying their user instruction files.

**Architecture:** Project destinations are fixed relative paths beneath a physically resolved project root. The canonical policy remains in OAW's XDG config namespace, while each project's installation state is isolated under the XDG state namespace by a deterministic project identity and validates the stored canonical root before use.

**Tech Stack:** Bash 3.2, Markdown/MDC target formats, `cksum` project identity, existing lifecycle engine and black-box harness.

**Canonical sources:** `.scratch/open-agent-workflow/spec.md`, `.scratch/open-agent-workflow/issues/04-project-and-extension-adapters.md`, core and extension adapter research.

---

### Task 1: Resolve project identities and core destinations

**Files:**
- Modify: `lib/paths.sh`
- Modify: `lib/targets.sh`
- Modify: `lib/state.sh`
- Create: `tests/05-project-adapters-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Write failing project identity and destination tests**

Create a project named `project with spaces`, invoke each core target with `--project`, and assert these destinations: `.claude/CLAUDE.md`, `AGENTS.md` for Codex and OpenCode, and `GEMINI.md`. Assert no file beneath isolated `~/.claude`, `~/.codex`, `~/.gemini`, or user OpenCode config changes.

- [ ] **Step 2: Verify RED**

Run: `bash tests/05-project-adapters-test.sh`

Expected: FAIL because project lifecycle destinations are undefined.

- [ ] **Step 3: Implement deterministic project state identity**

Compute `<crc>-<bytes>` from the physical project-root bytes with `cksum` and store state at `${XDG_STATE_HOME}/open-agent-workflow/installations/projects/<identity>.state`. Include a `project` record containing the exact physical root. On read, reject a state file whose stored root differs, so a checksum collision cannot attach state to another project.

- [ ] **Step 4: Register fixed project-relative core paths**

Expose `target_project_relative_path` with an exact `case` table. `target_destination` joins only registry-owned relative paths to `OAW_PROJECT_ROOT`; no user-provided target path reaches the join.

- [ ] **Step 5: Verify project paths and user isolation**

Run: `bash tests/05-project-adapters-test.sh`

Expected: project identity and core destination cases pass; extension cases remain RED.

- [ ] **Step 6: Commit project scope foundations**

```bash
git add lib/paths.sh lib/targets.sh lib/state.sh tests/05-project-adapters-test.sh tests/run.sh
git commit -m "feat: isolate project adapter state"
```

### Task 2: Render all extension adapter formats

**Files:**
- Modify: `lib/targets.sh`
- Modify: `lib/render.sh`
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

Cursor must start with valid `description`, `globs`, and `alwaysApply: true` frontmatter. Windsurf must use `trigger: always_on`. Copilot must use `applyTo: "**"`. Every body directs the agent to read the absolute canonical policy before lifecycle work and says the selected bundle persists for the task.

- [ ] **Step 2: Verify RED**

Run: `bash tests/05-project-adapters-test.sh`

Expected: FAIL because extension renderers are missing.

- [ ] **Step 3: Implement owned-file renderers**

Add one pure renderer per extension target. Each renderer prints its complete file, including frontmatter where documented. Set `target_ownership` to `owned-file` for the five unique extension paths and `managed-block` for core shared instruction files.

- [ ] **Step 4: Verify exact adapter bytes**

Run: `bash tests/05-project-adapters-test.sh`

Expected: all path, frontmatter, canonical-path, and lifecycle-lock assertions pass.

- [ ] **Step 5: Commit extension rendering**

```bash
git add lib/targets.sh lib/render.sh tests/05-project-adapters-test.sh
git commit -m "feat: render project extension adapters"
```

### Task 3: Deduplicate shared project instruction surfaces

**Files:**
- Modify: `lib/operations.sh`
- Modify: `lib/state.sh`
- Modify: `tests/05-project-adapters-test.sh`

- [ ] **Step 1: Add failing shared-AGENTS tests**

Install `codex,opencode` together and assert `AGENTS.md` has one OAW block, state has both target rows, and the destination checksum is identical in both rows. Uninstall Codex only and assert the block remains for OpenCode; uninstall OpenCode and assert only the block is removed.

- [ ] **Step 2: Verify RED**

Run: `bash tests/05-project-adapters-test.sh`

Expected: FAIL from duplicate writes or premature removal.

- [ ] **Step 3: Group operation actions by physical destination**

Create normalized action records keyed by exact destination. If two targets share a path, require identical ownership and rendered content; otherwise stop before mutation with an internal contract error. Apply the grouped action once and attach every target reference to its resulting managed checksum.

- [ ] **Step 4: Verify shared ownership lifecycle**

Run: `bash tests/05-project-adapters-test.sh`

Expected: one block survives until the final referencing target is uninstalled.

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

For every target and the default nine-target set, exercise install, repeated install, selected update, dry run, selected uninstall, and final uninstall. Seed user content in managed-block files and unrelated sibling files beside owned extension destinations. Assert preserved bytes and no user-scope changes.

- [ ] **Step 2: Verify RED**

Run: `bash tests/05-project-adapters-test.sh`

Expected: one or more lifecycle matrix cases fail.

- [ ] **Step 3: Generalize project operation and diagnostics**

Feed the physical project root and project state path into the same grouped lifecycle engine used by user scope. `check --project` reports selected adapter support and `not-installed`, `clean`, `drift`, or `invalid-state` without writing. Project install may ensure the OAW canonical policy exists in OAW's XDG config, but it must never edit a tool's user-level instruction destination.

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

