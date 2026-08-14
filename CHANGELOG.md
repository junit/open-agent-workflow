# Changelog

All notable changes to Open Agent Workflow will be documented in this file.

## [Unreleased]

No changes have been recorded after the fixed `0.1.0` source baseline.

## [0.1.0] - 2026-08-14

The `0.1.0` source baseline is fixed at this repository state. It is prepared
for a local version commit but is not published and has no remote release or
tag created by this change.

#### Added

- Provider-neutral task classification, blocking Profile selection, lifecycle
  locking, bounded specialist add-ons, and the Matt-Superpowers hybrid.
- A no-Bridge `policy-cooperative` CLI for `profiles`, `use`, `status`, typed
  completion and review events, gates, incidents, stable switching, explicit
  stop, and uncertain execution.
- Separate Policy catalog, route inspection, lifecycle reducer, Engagement,
  and persistence modules. Policy state is bound to the physical project and
  replayed before derived progress is trusted.
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
  Coordinator, Host, Policy Projection, and Machine Projection boundaries.
- Recognition of both legacy Everything Claude Code cache layouts and the
  current `.codex/plugins/cache/ecc/ecc/<version>` Codex plugin layout.

#### Changed

- OAW is now explicitly activated per deliverable. Ordinary Host requests and
  ordinary Skill invocations remain Native Host behavior until the user asks
  OAW to govern the task.
- Policy availability now reports `policy_selectable` and `host_routable`
  independently. Host-visible Skills, user-explicit Matt Skills, and neutral
  Host actions can route cooperative work without machine Provider
  attestation.
- Policy and machine execution share canonical responsibilities, gates,
  incidents, and lifecycle order while using physically separate types and
  authority records. Machine attestation can increase assurance but cannot
  veto a Policy Offer.
- `update` replaces OAW-owned eager managed instructions with a lazy Activation Router
  while preserving non-OAW instruction content.
- Existing policy-only Markdown lifecycle locks are not converted. Complete
  them under their old contract or explicitly reactivate and reselect the
  deliverable.
- Provider Descriptor v4, Profile Recipe v3, and Execution Graph v4 are active,
  alongside Lifecycle Bundle v4, user configuration v3, Host integration v3,
  Capability Grant v3, and Workflow State v2. There is no migration reader or
  compatibility alias for older pre-release authority records.
- Management verification now exercises the authoritative Go implementation
  directly through package tests and the numbered CLI black-box suite; the
  pre-cutover Bash implementation is no longer a parity oracle.

#### Removed

The following old execution contracts are removed; this is the only current
product section that names them:

- `oaw run --host codex` and `oaw runtime exchange`.
- Codex Runner and `oaw/codex-runner` process execution.
- private HOME/Skill staging, Host configuration filtering, and old execution
  schemas, state readers, topology aliases, and compatibility aliases.
- Retired authority schema files that were rejected by the active Registry but
  still embedded in the production binary.
- The test-only pre-cutover Bash management implementation and its Bash/Go
  parity harnesses. `install.sh` remains the supported offline wrapper for the
  colocated Go binary.
- Caller-selected Policy slots, free-form next actions, caller-managed Policy
  state roots, and manual completed-state closure. The reducer derives current
  work and accepts only typed events.

#### Boundaries

- Release archives include a precompiled binary and perform no runtime
  executable download. A source checkout must build `./oaw` before use.
- TSV Install State and revisioned Workflow State remain disjoint; no automatic
  migration imports policy-only tasks or Profile locks.
- The Agent Host retains physical execution authority. The Workflow Coordinator
  records no credentials or private extension configuration.
- The source version is fixed at `0.1.0`, but this change does not publish a
  release. Available native and Docker smoke must pass; unavailable platform
  checks return 77 and do not block release readiness. Remote publication and
  tag creation still require separate owner approval.
- Without Bridge, Policy execution supports only the current Host session and
  cannot claim a verified Provider Instance, Lifecycle Bundle, Capability
  Grant, Resource Lease, Host Receipt, atomic revision, idempotency, or
  enforced recovery.

#### Security

- Forced operations preserve recoverable bytes and a private manifest before
  applying drifted replacements or removals.
- Install and uninstall bind targets and owned directories to registry-derived
  paths and reject unsafe parent changes.
- Core and Coordinator state is secret-free and contains only opaque digests;
  Host sandbox and approvals remain the physical boundary for cooperating
  clients.
