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

## Ticket 05 - Drift and Hardening (Tasks 1-2 Checkpoint)

- Task 1 reviewed range: `aea6788..fa6182c`
- Task 2 reviewed range: `fa6182c..c5763b5`
- Canonical ticket: `.scratch/open-agent-workflow/issues/05-drift-backups-and-hardening.md`
- Execution plan: `docs/superpowers/plans/2026-07-30-open-agent-workflow-05-drift-backups-and-hardening.md`
- Result through Task 2: approved with no open Critical, Important, or security findings

Review coverage:

- Closed, inert state parsing rejects unknown records, malformed field counts,
  duplicate metadata/targets, scope/root mismatches, unsafe field separators,
  executable-looking payload text, and inconsistent shared destinations.
- Managed block bodies, marker loss/reversal/duplication/nesting, owned files,
  state checksums, missing artifacts, and untracked OAW markers all fail closed
  before update or uninstall can change a managed fingerprint.
- Cross-scope policy synchronization and uninstall retention validate canonical
  state identity, registry-derived target paths, ownership, marker structure,
  and recorded artifact checksums before any apply step.
- Retention scans every matching candidate rather than returning after the
  first healthy reference; a later drifted candidate aborts the operation.
- A syntactically valid forged project state at its correct identity filename
  cannot be synchronized or retain policy when its target artifact is absent.
- The legacy Claude retention fixture was corrected to use a canonical project
  install instead of a forged second user state; the valid cross-scope behavior
  remains covered and passes.

Plan note:

- Task 2 listed `lib/managed.sh` and `lib/commands/check.sh`, but review found
  no missing change: strict marker status and read-only drift reporting already
  satisfied the new black-box cases.

Remaining release blocker:

- Ticket 05 Task 3 still owns project symlink/component confinement and
  prepare/apply TOCTOU revalidation. No Task 1-2 finding reclassifies or closes
  that boundary.

## Ticket 05 - Drift and Hardening (Task 3 Re-review)

- Initial reviewed range: `18d54b2..fe38ec2`
- Remediated range: `18d54b2..74ff3ec`
- Canonical ticket: `.scratch/open-agent-workflow/issues/05-drift-backups-and-hardening.md`
- Execution plan: `docs/superpowers/plans/2026-07-30-open-agent-workflow-05-drift-backups-and-hardening.md`
- Result through Task 3: approved with no open Critical or Important findings

Resolved finding:

- Important: the apply-time race test pre-created the outside `rules/`
  directory, so it proved that the final target file was not written but
  missed that `mkdir -p` had already followed a swapped parent symlink and
  created an outside directory. Commit `74ff3ec` makes directory creation
  component-relative from a physically entered allowed root, verifies each
  component against its prepared coordinate, and strengthens the black-box
  race assertion to require the outside directory to remain absent.

Review coverage:

- Absolute and control-character root rejection, physical project-root
  resolution, registry-derived relative destinations, and ignored
  `CODEX_HOME`.
- Intermediate and final symlink rejection for user, project, policy, state,
  and cross-scope candidate-state paths.
- Prepared allowed-root/relative-suffix action coordinates and apply-boundary
  revalidation for replace and remove operations.
- Directory-creation race injection verifies that a swapped project component
  cannot create either an outside directory or an outside target file.
