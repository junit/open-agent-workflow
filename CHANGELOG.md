# Changelog

All notable changes to Open Agent Workflow will be documented in this file.

## [Unreleased]

### 0.1.0 - Local candidate

This local candidate is not published and has no remote release.

#### Added

- Provider-neutral task classification, blocking profile selection, lifecycle
  locking, bounded specialist add-ons, and the Matt-Superpowers hybrid.
- One XDG canonical policy with user and project adapters for Claude Code,
  Codex CLI, Gemini CLI, OpenCode, Cursor, Windsurf/Devin, Cline, Roo Code,
  and GitHub Copilot.
- Local install, check, update, dry-run, forced recovery, and exact uninstall
  lifecycles.
- A public Go-authoritative management CLI for `check`, `install`, `update`, and
  `uninstall`, with `install.sh` retained only as an offline sibling-binary
  compatibility wrapper.
- Offline release tooling for precompiled Darwin, Linux, and Windows archives
  on amd64 and arm64, plus sorted `SHA256SUMS` and an actual-WSL smoke gate.
- Inert state, fail-closed drift checks, symlink containment, prepared actions,
  operation-scoped backups, backup-before-mutation enforcement, and reverse
  mutation rollback on reported apply failures.
- English/Chinese governance and complete bilingual documentation of management
  cutover and Runtime limits.

#### Boundaries

- Release archives include a precompiled binary and perform no runtime
  executable download. A source checkout must build `./oaw` before use.
- TSV Install State and revisioned Runtime State remain disjoint; no automatic
  migration imports Policy-only tasks or profile locks.
- Only the pinned `oaw/codex-runner` integration is Runtime-managed. All other
  installed adapters remain Policy-only without Runtime admission, Grants,
  leases, transition enforcement, or physical isolation guarantees.
- This remains a local candidate, not a published release. Publication requires
  a successful smoke run inside an actual Microsoft WSL kernel and separate
  owner approval.

#### Security

- Forced operations preserve recoverable bytes and a private manifest before
  applying drifted replacements or removals.
- Install and uninstall bind targets and owned directories to registry-derived
  paths and reject unsafe parent changes.
