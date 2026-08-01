# 13 — Go update, Uninstall and Security Transaction Parity

**What to build:**

OAW ports update, uninstall, drift handling, forced recovery, and security
transaction behavior to Go in parity mode, matching Bash across success,
failure, rollback, and containment cases before any cutover.

**Blocked by:** 12 — Go install, Rendering and State Parity

**Status:** ready-for-agent

- [ ] Update and uninstall match Bash behavior for managed blocks, owned files,
  install state, backups, and dry-run output.
- [ ] Drift handling and forced recovery match Bash status codes and diagnostics.
- [ ] Transaction rollback preserves pre-existing user content on every injected
  failure.
- [ ] Security containment tests cover path traversal, symlink redirection, control
  characters, project-root validation, and credential-like output leakage.
- [ ] Go parity does not modify user configuration unless the corresponding Bash
  path would have done so.
