# Changelog

All notable changes to Open Agent Workflow will be documented in this file.

## [Unreleased]

### 0.1.0 - Local candidate

This local candidate is not published and has no remote release.

#### Added

- Provider-neutral task classification, blocking Profile selection, lifecycle
  locking, bounded specialist add-ons, and the Matt-Superpowers hybrid.
- OAW Core, the optional Workflow Coordinator, `CURRENT`/`SUBAGENT` topology,
  and Host session reports.
- One XDG canonical policy with user and project adapters for Claude Code,
  Codex CLI, Gemini CLI, OpenCode, Cursor, Windsurf/Devin, Cline, Roo Code,
  and GitHub Copilot.
- Local install, check, update, dry-run, forced recovery, and exact uninstall
  lifecycles.
- A public Go-authoritative management CLI for `check`, `install`, `update`, and
  `uninstall`, with `install.sh` retained only as an offline sibling-binary
  compatibility wrapper.
- Offline release tooling for precompiled Darwin, Linux, and Windows archives
  on amd64 and arm64, plus sorted `SHA256SUMS` and shared Linux smoke through
  Docker or optional WSL execution.
- Inert state, fail-closed drift checks, symlink containment, prepared actions,
  operation-scoped backups, backup-before-mutation enforcement, and reverse
  mutation rollback on reported apply failures.
- English/Chinese governance and complete bilingual documentation of Core,
  Coordinator, and Host boundaries.

#### Changed

- OAW is now explicitly activated per deliverable. Ordinary Host requests and
  ordinary Skill invocations remain Native Host behavior until the user asks
  OAW to govern the task.
- `update` replaces OAW-owned eager managed instructions with a lazy Activation Router
  while preserving non-OAW instruction content.
- Existing policy-only Markdown lifecycle locks are not converted. Complete
  them under their old contract or explicitly reactivate and reselect the
  deliverable.
- Provider descriptor v3, Profile Recipe v2, user configuration v3, Host integration v2, Capability Grant v2, and Workflow State v1 are the active
  contracts. There is no migration reader or compatibility alias.

#### Removed

The following old execution contracts are removed; this is the only current
product section that names them:

- `oaw run --host codex` and `oaw runtime exchange`.
- Codex Runner and `oaw/codex-runner` process execution.
- private HOME/Skill staging, Host configuration filtering, and old execution
  schemas, state readers, topology aliases, and compatibility aliases.

#### Boundaries

- Release archives include a precompiled binary and perform no runtime
  executable download. A source checkout must build `./oaw` before use.
- TSV Install State and revisioned Workflow State remain disjoint; no automatic
  migration imports policy-only tasks or Profile locks.
- The Agent Host retains physical execution authority. The Workflow Coordinator
  records no credentials or private extension configuration.
- This remains a local candidate, not a published release. Available native and
  Docker smoke must pass; unavailable platform checks return 77 and do not block
  release readiness. Remote publication still requires separate owner approval.

#### Security

- Forced operations preserve recoverable bytes and a private manifest before
  applying drifted replacements or removals.
- Install and uninstall bind targets and owned directories to registry-derived
  paths and reject unsafe parent changes.
- Core and Coordinator state is secret-free and contains only opaque digests;
  Host sandbox and approvals remain the physical boundary for cooperating
  clients.
