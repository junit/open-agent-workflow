# Custom Profile Dogfood Specification

## Capability

Create and reuse a project-owned `release-check` Custom Profile that combines
readable installed Skills, leaves omitted Responsibilities to Policy Defaults,
and validates a small release manifest.

## Custom Profile Contract

- Project Profile ID: `release-check`; built-in IDs remain reserved.
- Declared Responsibilities: problem framing via
  `ecc:intent-driven-development`, delivery planning via
  `superpowers:writing-plans`, implementation/TDD via `ecc:tdd-workflow`, and
  review via `ecc:security-review`.
- Omitted Responsibilities intentionally use Policy Defaults, including
  specification, workspace safety, fresh verification, closeout, and safety.
- If a native Review route is absent, the model reads the readable
  `ecc:security-review` rules and records that fallback without changing the
  Review and remediation owner.

## Deliverable

`manifestcheck <path>` validates a `key=value` Release Manifest with default
Required Fields `version` and `commit`. A fresh task reuses
`project:release-check` and adds repeatable `--require key` support while
preserving the default contract.

## Acceptance Criteria

- AC-001: Profile is created in `.oaw/profiles/` and no machine configuration is
  edited.
- AC-002: Profile inspection reports omitted Responsibilities covered by Policy
  Defaults.
- AC-003: Missing native Review route falls back visibly to readable Skill rules
  with Review ownership preserved.
- AC-004: Project/user same-ID conflict requires explicit source qualification;
  built-in `SP-FULL` remains independently selectable.
- AC-005: Fresh task discovers and reuses the project Profile for `--require`.
- AC-006: Artifact remains functional after removing `oaw`, Assurance, Bridge,
  and runtime state.
