# OAW Ticket 07 Security Review and Release Verification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce a locally releasable OAW v0.1 candidate with closed review loops and fresh verification evidence.

**Architecture:** Review is evidence-first and non-mutating until findings are accepted into remediation. Superpowers owns spec and quality review plus completion verification; the approved ECC `security-review` add-on contributes one bounded security report, after which remediation and re-review return to Superpowers.

**Tech Stack:** Git, Bash syntax checks, optional ShellCheck, black-box installer suite, Markdown evidence records.

**Canonical sources:** `.scratch/open-agent-workflow/spec.md`, all seven ticket files, all seven Superpowers plans, locked bundle `MATT-SP-HYBRID + ECC(security-review)`.

---

### Task 1: Run Superpowers spec-compliance review

**Files:**
- Create: `.scratch/open-agent-workflow/evidence/review.md`
- Modify: implementation and tests only when a finding is confirmed

- [ ] **Step 1: Capture the review baseline**

Record the reviewed commit, `git status --short`, `git diff --check`, and a matrix mapping all 40 specification stories and every ticket acceptance criterion to implementation paths and black-box tests.

- [ ] **Step 2: Invoke the Superpowers review owner**

Use `superpowers:requesting-code-review` with the canonical spec, ticket graph, plans, baseline commit, and locked bundle. Require findings to include severity, exact path/line, violated acceptance criterion, and reproduction command; do not invoke Matt or ECC general review.

- [ ] **Step 3: Validate and remediate confirmed findings**

Use `superpowers:receiving-code-review` for feedback handling. For each confirmed behavior defect, add a failing black-box regression test, observe RED, implement the smallest fix, observe GREEN, and record the commit. Reject unsupported findings with concrete code/test evidence in `review.md`.

- [ ] **Step 4: Re-run spec review until critical and high findings are closed**

Repeat the same reviewer against the remediation commit and append disposition. The section ends with explicit counts for open critical, high, medium, and low findings.

- [ ] **Step 5: Commit review remediation and evidence**

```bash
git add .scratch/open-agent-workflow/evidence/review.md install.sh lib tests scripts policy docs README.md README-zh.md
git commit -m "fix: close specification review findings"
```

Expected: no open critical or high spec-compliance finding.

### Task 2: Run the bounded ECC security review

**Files:**
- Modify: `.scratch/open-agent-workflow/evidence/review.md`
- Modify: implementation, tests, and security docs only for confirmed findings

- [ ] **Step 1: Declare the bounded add-on scope**

Record that `ecc:security-review` may inspect CLI input, environment roots, containment, symlinks, marker/state parsing, temp files, backup ordering, command execution, remote execution, and uninstall ownership. It does not own planning, implementation orchestration, general review, Git, or completion.

- [ ] **Step 2: Invoke `ecc:security-review` with concrete evidence**

Provide `install.sh`, `lib/`, `tests/06-security-test.sh`, the security docs, `git diff`, and the exact threat scope. Require exploitability, severity, evidence, and a minimal reproduction for every finding.

- [ ] **Step 3: Return confirmed findings to Superpowers remediation**

For each valid finding, add a black-box adversarial test that fails for the reported reason, implement the minimal correction under the active Superpowers executor, and run the focused test plus full suite. Record dismissed false positives with evidence rather than changing code defensively without a demonstrated risk.

- [ ] **Step 4: Re-run the bounded security check**

Ask the same ECC add-on to verify only the finding dispositions and affected surfaces. Append the final open-severity counts and end the ECC bundle.

- [ ] **Step 5: Commit security remediation**

```bash
git add .scratch/open-agent-workflow/evidence/review.md install.sh lib tests docs/en/security.md docs/zh/security.md SECURITY.md SECURITY-zh.md
git commit -m "fix: close installer security findings"
```

Expected: no open critical or high security finding; every accepted finding has a regression test.

### Task 3: Run final Superpowers code-quality review

**Files:**
- Modify: `.scratch/open-agent-workflow/evidence/review.md`
- Modify: code and tests only for confirmed findings

- [ ] **Step 1: Request a fresh code-quality review**

Use `superpowers:requesting-code-review` on the full branch delta. Focus on Bash 3.2 compatibility, quoting, exit-status propagation, function/file cohesion, duplicated target logic, cleanup traps, idempotence, test isolation, user-content preservation, and documentation parity.

- [ ] **Step 2: Handle feedback and preserve behavior**

Use `superpowers:receiving-code-review`; reproduce each issue, add or adjust a focused test when behavior can regress, make one scoped correction, and rerun affected tests. Do not accept a refactor that weakens preflight or creates a second functional test seam.

- [ ] **Step 3: Re-review the final delta**

Record every finding as fixed, rejected with evidence, or accepted low-risk debt with rationale. Require zero unresolved critical/high and no unexplained medium finding.

- [ ] **Step 4: Commit quality remediation**

```bash
git add .scratch/open-agent-workflow/evidence/review.md install.sh lib tests scripts docs README.md README-zh.md
git commit -m "refactor: close final code review findings"
```

Expected: review evidence contains closed spec, security, and quality sections.

### Task 4: Capture fresh completion verification

**Files:**
- Create: `.scratch/open-agent-workflow/evidence/verification.md`

- [ ] **Step 1: Record environment and clean inputs**

Record date, commit, OS, `bash --version`, available `shellcheck --version`, and `git status --short`. Remove only test-owned temporary directories through their harness cleanup; do not remove user files or rewrite the branch to manufacture cleanliness.

- [ ] **Step 2: Run syntax and repository hygiene checks**

Run:

```bash
bash -n install.sh lib/*.sh lib/commands/*.sh scripts/*.sh tests/*.sh
git diff --check
git ls-files | grep -E '(^|/)(\.env|id_rsa|credentials)(\.|$)' && exit 1 || true
grep -R -nE '(curl|wget).*(\||bash|sh)|git (pull|fetch|clone)' install.sh lib scripts && exit 1 || true
```

Expected: all commands exit `0`; secret/prohibited remote-execution scans print no match.

- [ ] **Step 3: Run optional static shell analysis**

If `shellcheck` exists, run `shellcheck -s bash install.sh lib/*.sh lib/commands/*.sh scripts/*.sh tests/*.sh` and require exit `0`. If unavailable, record `SKIPPED: shellcheck command not installed`; do not claim it passed.

- [ ] **Step 4: Run the full black-box and documentation suites**

Run:

```bash
bash tests/run.sh
bash scripts/check-docs.sh
```

Expected: all installer cases and documentation checks pass with zero failures.

- [ ] **Step 5: Prove dry-run and idempotence from a fresh sandbox**

Create one test-owned temporary HOME/XDG/project, install user and project defaults, snapshot checksums and mtimes, repeat both installs, run update/uninstall dry runs, and compare the tree after each read-only/idempotent operation. Record the exact commands and comparison exit statuses.

- [ ] **Step 6: Save verification evidence**

For every command, include exit status and the relevant unedited output in `verification.md`. End with a checklist for all 40 stories, open review counts, optional skipped checks, and a statement that no remote action occurred.

### Task 5: Finish the local branch without remote mutation

**Files:**
- Modify: `.scratch/open-agent-workflow/workflow.md`
- Modify: `.scratch/open-agent-workflow/issues/01-installer-foundation-and-check.md`
- Modify: `.scratch/open-agent-workflow/issues/02-claude-user-lifecycle.md`
- Modify: `.scratch/open-agent-workflow/issues/03-core-user-adapters.md`
- Modify: `.scratch/open-agent-workflow/issues/04-project-and-extension-adapters.md`
- Modify: `.scratch/open-agent-workflow/issues/05-drift-backups-and-hardening.md`
- Modify: `.scratch/open-agent-workflow/issues/06-bilingual-docs-and-extension-contract.md`
- Modify: `.scratch/open-agent-workflow/issues/07-security-review-and-release-verification.md`

- [ ] **Step 1: Mark tickets from evidence, not assumption**

Check an acceptance criterion only when its implementation path, focused test, and fresh verification result are present. Set each completed ticket status to `done`; leave any unmet criterion unchecked and keep the lifecycle active.

- [ ] **Step 2: Update the lifecycle manifest**

Set `current_stage: complete`, `active_ticket: null`, and add the final commit, review evidence path, verification evidence path, and exact locked bundle. Do not change the canonical specification.

- [ ] **Step 3: Commit completion records**

```bash
git add .scratch/open-agent-workflow
git commit -m "docs: record OAW v0.1 verification"
```

- [ ] **Step 4: Use Superpowers branch completion**

Invoke `superpowers:verification-before-completion`, then `superpowers:finishing-a-development-branch`. Select the local keep/hand-off outcome: do not push, open a pull request, publish a release, or create a remote repository.

- [ ] **Step 5: Confirm final local state**

Run: `git status --short && git log -1 --oneline`

Expected: status prints no paths; log prints the verification-record commit. The completed project remains local for the separately authorized move to `~/LLM/open-agent-workflow`.

