# 14 — Cutover and Release Verification

**What to build:**

OAW cuts over from Bash-authoritative management to the Go-backed release path
only after Runtime Host readiness, Policy vNext projection readiness, and
management parity are proven. Release verification covers existing Bash
regressions, Go tests, conformance, security, packaging, and platform smoke
tests.

**Blocked by:** 09 — First Runtime Host and oaw run; 10 — Policy vNext and Runtime Projections; 13 — Go update, Uninstall and Security Transaction Parity

**Status:** ready-for-agent

- [ ] install.sh becomes a compatibility wrapper only after command-level parity
  evidence is complete.
- [ ] Release archives include a precompiled binary and do not download executable
  code at execution time.
- [ ] Existing Policy-only tasks and existing install state are not silently
  imported into Runtime State.
- [ ] Cross-platform release builds and WSL smoke verification pass.
- [ ] Go unit, race, coverage, vet, static analysis, vulnerability, conformance,
  eval, Bash regression, and documentation checks all pass.
- [ ] Release notes state Runtime-managed and Policy-only guarantees truthfully.
