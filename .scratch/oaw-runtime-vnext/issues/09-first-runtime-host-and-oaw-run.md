# 09 — First Runtime Host and oaw run

**What to build:**

After a Host passes official capability audit and the user explicitly selects
it, OAW exposes oaw run as the reference Runtime entrypoint for that Host and
drives Runtime Protocol exchanges without changing unsupported Hosts from
Policy-only behavior.

**Blocked by:** 08 — Host Conformance and Capability Audit; explicit user Host selection

**Status:** completed

**Selection evidence:** `.scratch/oaw-runtime-vnext/evidence/host-selection.md`

- [x] The selected Host has a pinned conforming Manifest and integration record.
- [x] oaw run uses the same Runtime Protocol as native Host integrations.
- [x] Machine-facing exchange output is canonical JSON on stdout with diagnostics
  on stderr.
- [x] Unsupported Hosts remain Policy-only and do not claim Runtime guarantees.
- [x] Runtime-aware entrypoints are exposed only where exact Host capability
  permits them.
- [x] User selection of the first Runtime Host is recorded as migration evidence.

## Explicit Host Selection

The user selected **Codex CLI** on 2026-08-03 as the first production Runtime
Host. Ticket 09 implements the pinned `runner-managed` integration
`oaw/codex-runner`; every other built-in Host remains `instruction-only`.

## Completion Record

Ticket 09 passed its selection, protocol, runner, capability, isolation, and
evidence gates at the implementation fixed point `0cb396d`. The pinned Codex
identities are:

| Asset | Digest |
| --- | --- |
| Manifest | `1cdbf2d09620d585486000418f1770fc604ae323a9cd8c27bd3e0bdef5c30d5d` |
| Audit evidence | `2e0e3c9e74bae4a8d249507ac1573596f5ad06964c19b8c739b2ef1e093052ec` |
| Conformance report | `7ea7026fda4146cbd6a19db8ea25168a9c02bf2f81f67a29ba37ab3fac419e4b` |
| Integration record | `bea2b3a7caee2062e7b058a8fbfe1187adfc5c60ac7c033275d02e251393d303` |

The implementation commits are `a15c019`, `ab1dc6e`, `84dccc4`, `fb82114`,
`82896b8`, dispatch identity/project-context remediation `8d3ef17`,
transport/deduplication hardening `3e23771`, cross-process replay remediation
`5bb0380`, and dispatch orchestration refactor `0cb396d`.
`oaw run --host codex` is the only runtime-managed entrypoint; it rejects every
other Host and does not modify the existing Bash installer authority. Resumed
non-START frames may supply `--project-root` so project configuration remains
explicit and pinned.

## Selection Audit

Official automation surfaces and locally installed versions for Codex CLI,
Claude Code, and Gemini CLI were checked on 2026-08-03. Codex CLI was the
evidence-backed recommendation, and the user explicitly selected it as the
first `runner-managed` integration. That selection unblocked implementation;
promotion completed only after the exact Manifest, audit, Integration record,
and Conformance report passed the Ticket 09 gates.
