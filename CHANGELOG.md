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
- Inert state, fail-closed drift checks, symlink containment, prepared actions,
  operation-scoped backups, and backup-before-mutation enforcement.
- English/Chinese governance and the foundation for complete
  bilingual documentation.

#### Security

- Forced operations preserve recoverable bytes and a private manifest before
  applying drifted replacements or removals.
- Install and uninstall bind targets and owned directories to registry-derived
  paths and reject unsafe parent changes.
