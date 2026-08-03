# 14 — Cutover and Release Verification

**What to build:**

OAW cuts over from Bash-authoritative management to the Go-backed release path
only after Runtime Host readiness, Policy vNext projection readiness, and
management parity are proven. Release verification covers existing Bash
regressions, Go tests, conformance, security, packaging, and platform smoke
tests.

**Blocked by:** 09 — First Runtime Host and oaw run; 10 — Policy vNext and Runtime Projections; 13 — Go update, Uninstall and Security Transaction Parity

**Status:** completed

- [x] install.sh becomes a compatibility wrapper only after command-level parity
  evidence is complete.
- [x] Release archives include a precompiled binary and do not download executable
  code at execution time.
- [x] Existing Policy-only tasks and existing install state are not silently
  imported into Runtime State.
- [x] Cross-platform release builds and available platform smoke verification
  pass; unavailable platform checks return 77 and do not block completion.
- [x] Go unit, race, coverage, vet, static analysis, vulnerability, conformance,
  eval, Bash regression, and documentation checks all pass.
- [x] Release notes state Runtime-managed and Policy-only guarantees truthfully.

**Completion record:** The six release archives and checksums passed. Docker
Linux/arm64 smoke returned status 0 using the shared Linux assertions. The
macOS WSL probe returned status 77 because no Microsoft WSL kernel is available;
this optional unavailable-platform result is recorded and does not block
completion. Native Windows and WSL execution are not claimed.
