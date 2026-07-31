# Open Agent Workflow Review Evidence

## Ticket 03 - Core User Adapters

- Reviewed range: `af125b0..596bc56`
- Canonical ticket: `.scratch/open-agent-workflow/issues/03-core-user-adapters.md`
- Execution plan: `docs/superpowers/plans/2026-07-30-open-agent-workflow-03-core-user-adapters.md`
- Result: approved with no open Critical or Important findings

Resolved finding:

- Important: duplicate comma selections were normalized before the temporary
  single-target guard, allowing `codex,codex` to mutate files during Task 1.
  Commit `aff4e12` added the failing no-mutation contract and raw-selection
  guard. Task 2 then replaced that temporary boundary with deterministic
  multi-target support.

Review coverage:

- Exact target-native renderers and destinations for Codex, Gemini, and
  OpenCode, including import/non-import semantics.
- Inert state parsing, target validation, duplicate rejection, registry order,
  and deterministic merging.
- Pre-rendered, destination-grouped install/update/uninstall actions with
  selected-target and reference-aware removal behavior.
- User-content preservation, dry-run behavior, local-checkout updates, and
  byte-stable repeat installs.
- Read-only `check` health reporting kept separate from provider/tool
  readiness.

Residual note:

- Core user destinations are distinct. The grouped action engine fails closed
  on conflicting renders and is ready for real shared project destinations,
  which receive black-box coverage in Ticket 04.

## Ticket 04 - Project and Extension Adapters

- Reviewed range: `b2a0a1d..e1ea03e`
- Canonical ticket: `.scratch/open-agent-workflow/issues/04-project-and-extension-adapters.md`
- Execution plan: `docs/superpowers/plans/2026-07-30-open-agent-workflow-04-project-and-extension-adapters.md`
- Result: approved for Ticket 04 with no open Critical or Important findings

Resolved findings:

- Important: adding OpenCode after Codex, or Codex after OpenCode, treated the
  already-managed shared `AGENTS.md` block as untracked. Commit `75ee8e1`
  reuses the shared destination origin/checksum, rejects conflicting renders,
  and covers both sequential orders plus partial-to-default installation.
- Plan gap: updating one scope changed the shared canonical policy but left
  every other scope state on the old version/checksum; later uninstall could
  remove the still-referenced policy. The user approved a canonical Ticket 04
  Task 5, and commit `4b2cf6b` prepares metadata replacements for every clean
  referencing state while leaving non-selected adapters byte- and
  inode-stable. Policy retention is now based on validated path references.
- Important: cross-scope preparation accepted a project state whose stored
  root no longer matched its identity-derived filename. Commit `e1ea03e`
  validates candidate state location, scope/root identity, and every
  registry-derived target destination before any action is queued.

Review coverage:

- Physical project identity, exact nine adapter formats, owned-file versus
  managed-block lifecycle, paths containing spaces, and user-target isolation.
- Shared Codex/OpenCode destination grouping, sequential joins, selected
  uninstall, checksum/origin consistency, and deterministic default targets.
- Bidirectional user/project policy updates, changed-checkout new-scope
  install, dry-run fingerprints, path-aware retention, final cleanup, stale
  reference preflight, and invalid cross-scope binding rejection.
- Bounded ECC security review of state enumeration, inert parsing, checksum
  trust, dry-run writes, failure ordering, and policy deletion behavior.

Tracked Ticket 05 release work:

- High: project destination components and final targets can still follow
  symlinks outside the project. Ticket 05 Task 3 is the canonical owner of
  symlink and TOCTOU containment; the branch is not release-ready before it
  passes.
- Low: a forged but self-consistent state at the correct identity path with no
  live target artifact can still be synchronized or over-retain the policy.
  Ticket 05 Task 2 now requires liveness/drift preflight for every candidate
  synchronization and retention state. Requiring clean targets directly in
  Ticket 04 was rejected because it could delete policy needed to recover a
  legitimate drifted installation.
