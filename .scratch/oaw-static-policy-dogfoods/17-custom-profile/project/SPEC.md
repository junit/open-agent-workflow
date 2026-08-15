# Release Manifest Check Specification

## Problem Statement

Release operators need a deterministic check for the small text manifest that
travels with a release handoff.

## Solution

Create `manifestcheck`, a dependency-free CLI that validates `key=value` lines,
requires `version` and `commit` by default, and later allows a fresh task to
add extra Required Fields with `--require`.

## User Stories

1. As a release operator, I want malformed lines rejected, so that a handoff
   cannot silently omit a value.
2. As a release operator, I want duplicate fields rejected, so that the result
   does not depend on parser order.
3. As a release operator, I want the default `version` and `commit` fields
   enforced, so that every handoff has its identity and source revision.
4. As a release operator, I want to add a Required Field in a fresh task, so
   that teams can reuse the same project Profile for local release rules.
5. As a release operator, I want validation to read only the named file and
   emit no secrets, so that the check is safe in an isolated project.

## Implementation Decisions

- The public manifest validator is the highest test seam. It returns a stable
  success summary containing only the version and returns actionable errors.
- The first delivery requires `version` and `commit`. The fresh follow-up adds
  `ValidateRequired` and a repeatable `--require key` CLI option without
  changing default behavior.
- The CLI owns file I/O, argument parsing, and exit statuses. The domain
  validator owns field parsing and duplicate/required-field rules.
- No machine configuration, persistence, network, credentials, or OAW runtime
  is part of the capability.

## Testing Decisions

Tests use literal manifest bytes at the public validator seam and CLI output
tests for user-visible statuses. They cover malformed input, duplicate keys,
missing defaults, extra Required Fields, and file-read errors.

## Out of Scope

Cryptographic signature verification, semantic version ordering, YAML/JSON,
remote registries, secrets management, and publishing releases.
