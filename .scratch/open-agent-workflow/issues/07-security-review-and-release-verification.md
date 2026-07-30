# 07 - Security review and release verification

**What to build:** Produce a locally releasable OAW v0.1 candidate backed by an
independent bounded security review, remediated code-quality review, and fresh
cross-platform-oriented verification evidence.

**Blocked by:** 01 - Installer foundation and check command; 02 - Claude
user-scope lifecycle; 03 - Remaining core user adapters; 04 - Project-scope
core and extension adapters; 05 - Drift, backups, force, and filesystem
hardening; 06 - Bilingual documentation and adapter extension contract.

**Status:** ready-for-agent

- [ ] The approved ECC `security-review` add-on inspects installer input
      handling, path validation, symlink policy, marker parsing, state parsing,
      backup ordering, and command execution boundaries without taking over the
      lifecycle.
- [ ] Every critical or high security finding is fixed and re-reviewed; lower
      findings are fixed or explicitly documented with rationale.
- [ ] Superpowers spec-compliance and code-quality reviews cover all seven
      tickets and all 40 specification stories, with remediation and re-review
      recorded.
- [ ] Fresh verification runs shell syntax checks, static analysis when
      available, the full black-box suite, dry-run mutation checks, and a scan
      for secrets and prohibited remote execution behavior.
- [ ] Verification evidence records exact commands, exit statuses, relevant
      output, environment assumptions, and any unavailable optional checks.
- [ ] The final local Git worktree is clean, contains no generated fixtures or
      private configuration, and has an Apache-2.0-licensed, coherent v0.1
      project history.
- [ ] No GitHub repository, push, release, package publication, or other remote
      mutation is performed.

