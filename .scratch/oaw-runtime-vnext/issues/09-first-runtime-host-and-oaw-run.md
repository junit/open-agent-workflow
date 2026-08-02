# 09 — First Runtime Host and oaw run

**What to build:**

After a Host passes official capability audit and the user explicitly selects
it, OAW exposes oaw run as the reference Runtime entrypoint for that Host and
drives Runtime Protocol exchanges without changing unsupported Hosts from
Policy-only behavior.

**Blocked by:** 08 — Host Conformance and Capability Audit; explicit user Host selection

**Status:** ready-for-agent

**Selection evidence:** `.scratch/oaw-runtime-vnext/evidence/host-selection.md`

- [ ] The selected Host has a pinned conforming Manifest and integration record.
- [ ] oaw run uses the same Runtime Protocol as native Host integrations.
- [ ] Machine-facing exchange output is canonical JSON on stdout with diagnostics
  on stderr.
- [ ] Unsupported Hosts remain Policy-only and do not claim Runtime guarantees.
- [ ] Runtime-aware entrypoints are installed only where exact Host capability
  permits them.
- [ ] User selection of the first Runtime Host is recorded as migration evidence.

## Pre-selection Audit

Official automation surfaces and locally installed versions for Codex CLI,
Claude Code, and Gemini CLI were checked on 2026-08-03. Codex CLI is the
evidence-backed recommendation for the first `runner-managed` integration, but
no Host is selected or promoted by this recommendation. Implementation planning
remains blocked until the user explicitly selects one candidate.
