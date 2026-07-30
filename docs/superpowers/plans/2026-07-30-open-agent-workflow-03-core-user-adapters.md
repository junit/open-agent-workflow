# OAW Ticket 03 Core User Adapters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the complete user lifecycle to Codex CLI, Gemini CLI, and OpenCode while sharing one canonical policy.

**Architecture:** The target registry owns destination paths and ownership modes; pure renderers own target-native content. The existing operation planner consumes normalized target records, deduplicates shared destinations, and persists one merged scope state so partial target changes remain reference-aware.

**Tech Stack:** Bash 3.2, target-native Markdown entrypoints, existing black-box harness.

**Canonical sources:** `.scratch/open-agent-workflow/spec.md`, `.scratch/open-agent-workflow/issues/03-core-user-adapters.md`, core adapter research cited in `docs/` during Ticket 06.

---

### Task 1: Render the three remaining user entrypoints

**Files:**
- Modify: `lib/targets.sh`
- Modify: `lib/paths.sh`
- Modify: `lib/render.sh`
- Create: `tests/04-core-adapters-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Write failing exact-content tests**

For isolated user roots, assert destinations are `~/.codex/AGENTS.md`, `~/.gemini/GEMINI.md`, and `${XDG_CONFIG_HOME}/opencode/AGENTS.md`. Codex and OpenCode content must tell the model to read the absolute canonical file before lifecycle selection and must not contain a standalone `@/path` import. Gemini must contain the documented `@/absolute/path` import.

- [ ] **Step 2: Verify RED**

Run: `bash tests/04-core-adapters-test.sh`

Expected: FAIL because only Claude renders.

- [ ] **Step 3: Implement target-native renderers**

Add pure functions with these semantic forms:

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

- [ ] **Step 4: Verify exact destinations and content**

Run: `bash tests/04-core-adapters-test.sh`

Expected: renderer assertions pass; lifecycle assertions fail until integration.

- [ ] **Step 5: Commit core renderer support**

```bash
git add lib/targets.sh lib/paths.sh lib/render.sh tests/04-core-adapters-test.sh tests/run.sh
git commit -m "feat: render core agent entrypoints"
```

### Task 2: Merge selected targets into one scope lifecycle

**Files:**
- Modify: `lib/state.sh`
- Modify: `lib/operations.sh`
- Modify: `tests/04-core-adapters-test.sh`

- [ ] **Step 1: Add failing selection and merge tests**

Install `claude,codex`, then install `gemini`, then repeat with `codex,claude,claude`. Assert state target rows are uniquely ordered by registry, existing clean targets are retained, and only newly selected files change. Assert `--target all` is rejected because the public contract uses explicit defaults rather than a magic target.

- [ ] **Step 2: Verify RED**

Run: `bash tests/04-core-adapters-test.sh`

Expected: FAIL because state replacement drops or duplicates target rows.

- [ ] **Step 3: Implement target-state merging**

Parse existing state into a validated normalized record file, merge selected targets by exact ID, and regenerate state in registry order. Treat a target already present with the same destination/content as unchanged. `update --target` updates selected installed targets only and rejects a selected target absent from state; `install --target` adds missing targets.

- [ ] **Step 4: Verify deterministic merged state**

Run: `bash tests/04-core-adapters-test.sh`

Expected: selection, deduplication, add-target, and selected-update cases pass.

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

For each core target and the default four-target set, exercise fresh install, repeat install, copied-checkout update, dry run, selected uninstall, and final uninstall. Seed every destination with target-specific user content and assert it survives. After removing one target, assert remaining state and policy references survive.

- [ ] **Step 2: Verify RED**

Run: `bash tests/04-core-adapters-test.sh`

Expected: at least one add/update/uninstall matrix row fails.

- [ ] **Step 3: Generalize operation records**

Build target actions from registry functions `target_destination`, `target_ownership`, and `render_target_content`. Group actions by destination before mutation. A destination receives at most one prospective write and one state checksum per operation, even if multiple target rows reference it.

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

