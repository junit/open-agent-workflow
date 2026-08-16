# Changelog

All notable changes to Open Agent Workflow are documented in this file.

## [Unreleased]

#### Added

- A bilingual release operations manual covering validation, cross-platform
  artifacts, immutable tags, Draft verification, and GitHub publication.
- A Claude Policy Adapter for Claude-native instruction loading, lazy Skill
  discovery, invocation reporting, and Host authority boundaries.
- Complete Policy Adapters for Gemini, OpenCode, Cursor, Windsurf, Cline, Roo,
  and Copilot, with native instruction paths, lazy Skill discovery, reload
  behavior, surface separation, and physical-authority boundaries.
- Thin native OAW entrypoints for all nine project targets and the four
  user-scoped targets: `$oaw` for Codex and `/oaw` for Claude, Gemini,
  OpenCode, Cursor, Windsurf, Cline, Roo, and Copilot. The Copilot entrypoint
  is a Copilot CLI Agent Skill.

#### Changed

- Target installation now owns and tracks each Activation Router, native
  entrypoint, and Codex explicit-only metadata as a separate artifact, with
  conflict refusal, drift reporting, release-coherent update, backup-aware
  repair, and conservative uninstall.
- Natural-language activation remains equivalent to native invocation. Thin
  dispatchers carry only explicit user intent, an optional Profile, and the
  task; they do not define a default Profile, Responsibilities, lifecycle
  stages, or approval gates.
- Hosts use documented explicit-only controls when available. Cline relies on
  Policy self-gating because it has no documented per-Skill manual-only field.
- Copilot CLI entrypoints now use `disable-model-invocation` and an argument
  hint documented by the Host's Agent Skill format.
- Install State format 1 from 0.1.0 and 0.1.1 is a bounded upgrade input:
  `check` reports `upgrade-required`, while clean install or update atomically
  adds native artifacts for every previously installed target and emits format
  2. Fresh install and legacy migration now roll back changed files and empty
  created directories when a later write fails. Directory cleanup requires the
  identity captured at creation, so a concurrent replacement is preserved.

#### Fixed

- Native dispatchers no longer treat physical invocation or loading as proof of
  user intent. They require the top-level user request or reliable Host
  user-selection metadata and remain inert under model-led invocation.
- Native dispatchers follow the Activation Router without embedding a Policy
  path, so Host argument, file-inclusion, and command-expansion preprocessing
  cannot rewrite or execute path fragments.

#### Boundaries

- Static installation and management validation of the new entrypoints does
  not constitute live runtime end-to-end validation for each Host. Runtime
  dogfood is reported only after exercising the corresponding real Host
  session.

## [0.1.1] - 2026-08-16

#### Changed

- Removed development scratch data, obsolete implementation plans, and
  superseded architecture records from the active source tree.
- Rebuilt the architecture and product documentation around the current static
  Policy and optional machine-evidence boundaries.
- Removed legacy Provider cache probing from the default `oaw check` path;
  Profile Skill availability remains model-led and independent of Bridge.

## [0.1.0] - 2026-08-15

The first fixed source release establishes the static Policy product.

#### Added

- A Canonical Policy Set containing portable Policy, cooperative procedure,
  four Built-in Profiles, and Codex Adapter guidance.
- Project- and user-scoped installation with explicit activation, precedence
  without merging, managed Host instructions, drift detection, safe updates,
  backups, and exact uninstall.
- Natural-language Profile selection and Custom Profiles composed from
  currently installed Skills.
- Advisory `profile list`, `profile show`, and `profile check` commands.
- Standalone `oaw-assurance` Profile-to-Binding evidence and standalone
  `oaw-bridge` Codex observation.
- Offline cross-platform release archives, checksums, Docker smoke tests, and
  English/Chinese documentation.

#### Boundaries

- The installed Policy, selected Profile, readable Skills, and Host-native
  abilities form a complete operating path without Bridge or a running OAW
  executable.
- The default `oaw` executable manages installation and advisory inspection; it
  does not select a Profile, invoke Skills, execute models, or track delivery.
- Machine Assurance and Bridge are optional evidence components. They cannot
  change Profile semantics or veto Policy operation.
- The Agent Host and user retain physical execution and approval authority.
