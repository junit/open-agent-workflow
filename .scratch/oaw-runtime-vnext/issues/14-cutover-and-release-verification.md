# 14 — Cutover and Release Verification

**What to build:**

OAW cuts over from Bash-authoritative management to the Go-backed release path
only after Runtime Host readiness, Policy vNext projection readiness, and
management parity are proven. Release verification covers existing Bash
regressions, Go tests, conformance, security, packaging, and platform smoke
tests.

**Blocked by:** 09 — First Runtime Host and oaw run; 10 — Policy vNext and Runtime Projections; 13 — Go update, Uninstall and Security Transaction Parity

**Status:** in-progress

- [x] install.sh becomes a compatibility wrapper only after command-level parity
  evidence is complete.
- [x] Release archives include a precompiled binary and do not download executable
  code at execution time.
- [x] Existing Policy-only tasks and existing install state are not silently
  imported into Runtime State.
- [ ] Cross-platform release builds and WSL smoke verification pass.
- [x] Go unit, race, coverage, vet, static analysis, vulnerability, conformance,
  eval, Bash regression, and documentation checks all pass.
- [x] Release notes state Runtime-managed and Policy-only guarantees truthfully.

**Remaining gate:** The six release archives and local non-WSL detection pass,
but this macOS host returns status 77 from `scripts/smoke-wsl.sh` because it is
not an actual Microsoft WSL kernel. Ticket completion and release publication
require a fresh status-0 WSL smoke pass against the native Linux archive.
