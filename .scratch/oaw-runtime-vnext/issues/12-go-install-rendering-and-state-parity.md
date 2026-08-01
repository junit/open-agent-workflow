# 12 — Go install, Rendering and State Parity

**What to build:**

OAW ports install rendering and install-state behavior to Go in parity mode,
matching Bash output, file ownership, checksums, state records, and recoverable
backup behavior without making Go authoritative before parity passes.

**Blocked by:** 11 — Go check Black-box Parity

**Status:** ready-for-agent

- [ ] Bash install behavior remains authoritative until parity passes.
- [ ] Go install fixtures reproduce managed block rendering, owned-file rendering,
  install state records, checksums, and dry-run output.
- [ ] Existing user content outside OAW-owned surfaces is preserved.
- [ ] Recoverable backup behavior matches Bash for every supported target and
  scope.
- [ ] Parity failures leave Bash behavior and user files untouched.
