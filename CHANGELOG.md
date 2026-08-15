# Changelog

All notable changes to Open Agent Workflow will be documented in this file.

## [0.1.0] - 2026-08-15

This is the fixed static Policy Profile release baseline. The Policy Set,
built-in and Custom Profile contract, optional Assurance components, and
installation behavior described below are the current product behavior.

#### Added

- A separately built `oaw-bridge` executable with its own Codex Plugin
  installation lifecycle, diagnostics, tests, and v3 `observe_profile` MCP
  protocol.
- Content-addressed `oaw.assurance-overlay/v1` issuance and verification for
  exact Skill Bindings declared by one source-qualified Markdown Profile.

#### Changed

- The default `oaw` executable is now the static product surface: installation
  management plus advisory `profile list`, `profile show`, and `profile check`.
  Profile selection and Skill use happen through natural language without a
  topology, Add-on sentinel, Complexity, Risk, or limitation form.
- Codex Bridge is now an optional Assurance integration. The default `oaw`
  executable, installer, Policy integrations, Core, and Coordinator neither
  install nor depend on it.
- Bridge observation is read-only and limited to Codex `skills/list`; a
  missing, revoked, failed, or incomplete Bridge cannot veto the no-Bridge
  rule-driven Policy path.
- Optional Assurance now reads Binding occurrences from the selected Markdown
  Profile and matches them against a minimal Provider Descriptor v5 identity
  catalog. Provider discovery and Codex observation carry no Profile,
  lifecycle, classification, Capability, topology, or progress semantics.
- Codex Binding identity now matches the Profile and Host surfaces exactly:
  ECC Skills use `ecc:<skill>` references and Matt Skills resolve beneath
  `.agents/skills/<skill>`.

#### Removed

- The cooperative Policy route admission, lifecycle reducer, Engagement store,
  policy-run persistence, local lifecycle locks, replay state, and typed
  transition machinery. Model-owned progress and optional Markdown Progress
  Notes now remain outside the static CLI.
- Provider/catalog inspection, Policy lifecycle transitions, Workflow exchange,
  Bridge routing, status, and machine-run commands from the default `oaw`
  executable. Removed commands are absent rather than compatibility-wrapped.
- The legacy Bridge Core/Coordinator proxy operations, evidence handles,
  lifecycle Bundles, Receipts, Leases, delegation observations, and default
  `oaw/codex-host` host-native integration.
- Fixed Go Profile semantics, Profile Recipes and aliases, request
  classification, USER-DEFINED machine configuration, Capability compilation,
  execution graphs, Core, Coordinator, Registry, Host workflow records, and
  their pre-release schemas and compatibility surfaces.
- Provider source-audit manifests, generators, fixtures, and the standalone
  audit command. Provider JSON identity catalogs remain available to optional
  Assurance and Bridge paths.

## [0.1.0-pre-refactor] - 2026-08-14

This section records the superseded pre-release runtime architecture. It was
never published and is retained only to explain the hard-cut removal below.

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
- This pre-release architecture was not a published release and has no
  compatibility or migration obligation. Its runtime and machine-backed
  contracts were removed before the static Policy baseline was fixed.
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
