# OAW Ticket 05 Drift Backups and Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make mutating operations fail closed on drift and unsafe paths, with recoverable forced mutation and no partial writes.

**Architecture:** A prepare phase validates roots, state, markers, checksums, ownership, containment, and every destination before an apply phase can run. Prepared actions are inert tab-delimited data in a private temporary directory; forced operations back up every affected existing artifact before the first mutation.

**Security ownership:** Task 3 is the canonical implementation of the hostile
symlink and TOCTOU containment deferred from Ticket 04. Do not duplicate a
temporary containment layer in the earlier ticket; release remains blocked
until this task and its adversarial tests pass.

**Tech Stack:** Bash 3.2, POSIX path checks, `cksum`, `mktemp`, atomic renames, black-box adversarial tests.

**Canonical sources:** `.scratch/open-agent-workflow/spec.md`, `.scratch/open-agent-workflow/issues/05-drift-backups-and-hardening.md`, ADR 0002.

---

### Task 1: Reject malformed or executable-looking state as inert data

**Files:**
- Modify: `lib/state.sh`
- Create: `tests/06-security-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Write failing malicious-state cases**

Create state containing unknown records, wrong field counts, duplicate metadata, duplicate target IDs, a mismatched project root, tabs/newlines in fields, and literal values such as `$(touch "$OAW_SANDBOX/pwned")`. Assert every malformed state blocks mutation, reports `invalid state`, and never creates `pwned`.

- [ ] **Step 2: Verify RED**

Run: `bash tests/06-security-test.sh`

Expected: at least one malformed state is accepted or misclassified.

- [ ] **Step 3: Implement a closed state schema**

Require one each of `format`, `version`, `scope`, and `policy`; require `project` exactly for project scope; allow only known target IDs and ownership values; reject duplicate targets and destinations with inconsistent checksums. Parse with `while IFS="$(printf '\t')" read -r kind a b c d e`, and treat every field only as a quoted argument or compared byte string.

- [ ] **Step 4: Verify inert parsing**

Run: `bash tests/06-security-test.sh`

Expected: all malformed files fail closed and no payload-named file exists.

- [ ] **Step 5: Commit state hardening**

```bash
git add lib/state.sh tests/06-security-test.sh tests/run.sh
git commit -m "fix: validate installation state as inert data"
```

### Task 2: Detect all managed-content drift before mutation

**Files:**
- Modify: `lib/managed.sh`
- Modify: `lib/operations.sh`
- Modify: `lib/commands/check.sh`
- Modify: `tests/06-security-test.sh`

- [ ] **Step 1: Add failing drift variants**

Cover changed block body, changed owned file, missing begin marker, missing end marker, reversed markers, duplicate pairs, nested markers, state checksum mismatch, missing recorded file, and an unexpected OAW marker with no state. Add a self-consistent forged project state at its correct identity-derived filename with registry-correct target paths but no target artifact. Assert update and uninstall exit non-zero without changing any file; the forged state is not synchronized and cannot produce a successful final uninstall that silently retains the policy; check reports drift or invalid state.

- [ ] **Step 2: Verify RED**

Run: `bash tests/06-security-test.sh`

Expected: one or more corruption forms mutate or receive an ambiguous result.

- [ ] **Step 3: Make marker inspection strict and checksum-aware**

Count exact full-line marker occurrences, require either zero/zero or one/one in begin-before-end order, extract the exact block bytes to a private temp file, and compare its checksum to state. Owned-file destinations compare the complete file checksum. Absence is clean only when state does not claim the artifact.

- [ ] **Step 4: Add a complete preflight loop**

`prepare_operation` must inspect every selected and shared artifact and return a finalized action manifest before `apply_operation` is callable. Delete the prepared directory on any error; no destination directory may be created during preparation.

Apply the same liveness inspection to every candidate state used by
cross-scope policy synchronization or uninstall retention. Validate its state
location, scope/root identity, registry-derived target destinations, marker or
owned-file status, and recorded target checksums. A legitimate drifted
reference aborts the operation with its target/path diagnostic; a syntactically
valid but non-live forged state is never rewritten and never turns final
uninstall into a successful no-op with a retained policy.

- [ ] **Step 5: Verify fail-closed drift behavior**

Run: `bash tests/06-security-test.sh`

Expected: every drift variant blocks all writes and produces a target/path diagnostic.

- [ ] **Step 6: Commit drift enforcement**

```bash
git add lib/managed.sh lib/operations.sh lib/commands/check.sh tests/06-security-test.sh
git commit -m "fix: fail closed on managed-content drift"
```

### Task 3: Validate containment and reject symlink redirection

**Files:**
- Modify: `lib/paths.sh`
- Modify: `lib/filesystem.sh`
- Modify: `lib/operations.sh`
- Modify: `tests/06-security-test.sh`

- [ ] **Step 1: Add failing hostile-path tests**

Cover a missing project, a project argument that resolves through `..`, tabs/newlines in roots, a final destination symlink, a symlinked intermediate directory beneath the physical project root, a symlink to an outside file, and `CODEX_HOME`/XDG roots that are relative. Place sentinels outside scope and assert they never change.

- [ ] **Step 2: Verify RED**

Run: `bash tests/06-security-test.sh`

Expected: at least one symlink or invalid root reaches an outside sentinel.

- [ ] **Step 3: Implement explicit root and component checks**

Convert the selected existing project root with `cd -P`; require HOME, XDG overrides, and CODEX_HOME to resolve to absolute roots; reject CR, LF, and TAB. Since every destination suffix comes from the registry, verify its lexical prefix, then walk each existing component from the allowed root with `-L` and reject symlinks. Reject a symlink destination even when its target is inside scope.

- [ ] **Step 4: Revalidate immediately before atomic replace**

Store the validated allowed root and relative suffix in each action. `apply_action` reconstructs and rechecks the destination immediately before creating a directory or renaming, reducing the prepare/apply race window. Refuse inconsistent reconstructed paths.

- [ ] **Step 5: Verify containment**

Run: `bash tests/06-security-test.sh`

Expected: every hostile path fails before mutation and all outside sentinels retain their original checksum.

- [ ] **Step 6: Commit filesystem containment**

```bash
git add lib/paths.sh lib/filesystem.sh lib/operations.sh tests/06-security-test.sh
git commit -m "fix: contain adapter filesystem writes"
```

### Task 4: Back up forced mutations before applying them

**Files:**
- Create: `lib/backup.sh`
- Modify: `lib/operations.sh`
- Modify: `lib/state.sh`
- Modify: `tests/06-security-test.sh`

- [ ] **Step 1: Add failing force-order and recovery tests**

Drift two target files, run update and uninstall without force, then with `--force`. Assert the forced operation creates `${XDG_STATE_HOME}/open-agent-workflow/backups/<timestamp>-<pid>/manifest.tsv`, stores an unmodified copy of each affected existing file, lists original and backup paths as inert fields, and creates every backup before any destination mtime changes.

- [ ] **Step 2: Verify RED**

Run: `bash tests/06-security-test.sh`

Expected: forced drift is rejected or mutates without a complete backup.

- [ ] **Step 3: Implement operation-scoped backups**

Create one private backup directory after preparation. Copy each unique affected existing destination to numbered files such as `001-CLAUDE.md`; write a tab-delimited manifest with operation, scope, original, backup, and pre-mutation checksum. Verify copied checksums, then and only then call `apply_operation`. Print the backup path and record it in resulting state when state remains.

- [ ] **Step 4: Define force semantics precisely**

Force permits replacement/removal of drifted OAW destinations after backup; it
does not permit invalid CLI, unsupported scope, invalid state identity, path
escape, symlinks, or internal destination collisions. For a marker-corrupt
shared file, repair automatically only when the recorded checksum plus the
deterministic prior renderer identify one unique old managed body adjacent to
the surviving marker structure. If ownership cannot be identified uniquely,
retain the completed backup, refuse mutation, and print the exact manual
recovery path; never replace the whole shared file or append a second semantic
copy merely to make `--force` succeed.

- [ ] **Step 5: Verify backup-before-write evidence**

Run: `bash tests/06-security-test.sh`

Expected: clean operations make no backup; forced drift creates verified recoverable copies before changes.

- [ ] **Step 6: Commit recoverable force behavior**

```bash
git add lib/backup.sh lib/operations.sh lib/state.sh tests/06-security-test.sh
git commit -m "feat: back up forced workflow mutations"
```

### Task 5: Prove all-or-nothing preparation and exact uninstall

**Files:**
- Modify: `lib/operations.sh`
- Modify: `lib/filesystem.sh`
- Modify: `tests/06-security-test.sh`

- [ ] **Step 1: Add failing multi-target partial-write tests**

Select one valid early target and one later target with drift, an invalid parent, or a symlink. Snapshot the full scope before the command and assert no valid earlier target changes. Add hostile filenames containing spaces, glob characters, leading dashes, and shell metacharacters; assert exact quoted handling and clean removal.

- [ ] **Step 2: Verify RED**

Run: `bash tests/06-security-test.sh`

Expected: a partial-write or hostile-name case fails.

- [ ] **Step 3: Separate preparation from application completely**

Ensure directory creation, target writes, policy writes, state writes, and removals occur only inside `apply_operation` after all action records and optional backups are finalized. Use `--` where supported and quoted operands everywhere. Track created directories explicitly and remove them only when empty and OAW-owned.

- [ ] **Step 4: Run Ticket 05 verification**

Run: `bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh && bash tests/run.sh`

Expected: all Ticket 01-05 tests pass; invalid and drifted multi-target operations leave byte-identical scope snapshots; no payload file or outside sentinel changes.

- [ ] **Step 5: Commit hardened transactional behavior**

```bash
git add lib/operations.sh lib/filesystem.sh tests/06-security-test.sh
git commit -m "fix: preflight workflow mutations atomically"
```
