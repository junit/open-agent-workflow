# Custom Profile Selection Evidence

## Project-scoped natural-language creation

The activated request was:

```text
Create a project Custom Profile named release-check from the readable installed
Skills. Use intent-driven framing, executable planning, ECC TDD, and a security
review; leave the other Responsibilities to Policy Defaults. Deliver a real
manifestcheck command with it.
```

The model wrote the shared Markdown Profile contract to
`.oaw/profiles/release-check.md` in the isolated project. No user Policy Set,
machine configuration, Provider registration, or Go code in the OAW repository
was edited.

## Profile inspection

`oaw profile list` reported:

```text
built-in:ECC-FULL
built-in:MATT-FULL
built-in:MATT-SP-HYBRID
built-in:SP-FULL
project:release-check
user:release-check
warning PROFILE_ID_CROSS_SCOPE: Profile ID "release-check" exists in project and user scopes; use project:release-check or user:release-check
```

`oaw profile check project:release-check` reported:

```text
responsibilities: 4/8 (Policy defaults cover omitted Responsibilities)
Skill availability: not evaluated
result: metadata-valid
warning PROFILE_ID_CROSS_SCOPE: Profile ID "release-check" exists in project and user scopes; use project:release-check or user:release-check
```

An unqualified `profile show release-check` returned
`PROFILE_SELECTION_INVALID` and required a source qualifier. Explicit
`project:release-check`, `user:release-check`, and built-in `SP-FULL` all showed
their distinct source and Markdown content. Built-in IDs were not shadowed.

## Transparent fallback

The fixture Host index omitted `ecc:security-review`. The model first checked
the index, then read the installed Skill rules at the adapter-documented
readable path. It reported: native Review route absent; readable
`ecc:security-review` rules used; Review and remediation ownership unchanged.
The security review applied input validation, duplicate-field rejection,
secret/dependency checks, and non-disclosure checks to the Go artifact.

This is a source fallback, not a silent ownership change and not a Provider or
cache-path admission decision.

## Fresh task reuse

After the first `manifestcheck` delivery was committed, a fresh task explicitly
selected `project:release-check` and requested `--require owner`. The existing
Profile was rediscovered by source-qualified ID; the task added
`ValidateRequired` and repeatable CLI flags without editing the Profile or
changing its omitted Policy Defaults.
