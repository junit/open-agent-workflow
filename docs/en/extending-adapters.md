# Extending OAW

OAW has two extension points: Markdown Custom Profiles and Host Adapters. A
Custom Profile needs no Bridge, Provider registration, or Go change. Adding an
installable Host target also requires a small installer target definition.

## Host Adapter

An adapter documents how one host loads the Canonical Policy Set:

- the user and project instruction paths;
- managed-block or owned-file installation behavior;
- Skill index and native invocation surfaces;
- reload timing and readable fallback paths.

Adapter discovery hints must not decide that a Profile is available, attest
Skill content, select a method, or own physical execution. Keep Host-specific
paths in `policy/adapters/`, not in `POLICY.md`. Add install coordinates to
`internal/management/targets.go` only when OAW should manage that Host's
instruction file.

## Custom Profile

Create `.oaw/profiles/<id>.md` in a project or
`$XDG_CONFIG_HOME/open-agent-workflow/profiles/<id>.md` for a user Profile.
Use `id` and `name` front matter and a Responsibilities table. Refer to Skills
by their Host-visible name and state a neutral Host-native action when no Skill
is required.

Custom Profiles are partial by design. The Policy fills unspecified
Responsibilities with defaults. A project Profile wins only when explicitly
selected from the project source; same IDs are never silently merged.

Example:

~~~markdown
---
id: release-readiness
name: Release Readiness
---

## Responsibilities

| Responsibility | Skill or action |
| --- | --- |
| planning | ecc:blueprint |
| implementation | Policy default |
| review | ecc:security-review |
| verification | ecc:delivery-gate |
| closeout | Host-native closeout |
~~~

Use `oaw profile check` to validate metadata and inspect warnings. Validation
does not read or verify Skill content and does not select the Profile.

## Design Rules

Keep Profile semantics portable. Do not put Codex cache paths, CLI command
syntax, provider revisions, or machine digests in a Profile. Put those details
in the relevant Host Adapter or optional Machine Assurance component.
