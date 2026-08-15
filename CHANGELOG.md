# Changelog

All notable changes to Open Agent Workflow are documented in this file.

## [Unreleased]

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
