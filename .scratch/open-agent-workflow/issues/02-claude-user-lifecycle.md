# 02 - Claude user-scope lifecycle

**What to build:** The first complete OAW install lifecycle: install the
canonical policy and a Claude Code user entrypoint, repeat and update it
idempotently, preview it safely, and uninstall only OAW-owned content.

**Blocked by:** 01 - Installer foundation and check command.

**Status:** ready-for-agent

- [ ] A fresh user-scope Claude install creates the OAW-owned canonical policy,
      state, and a valid managed entrypoint in `~/.claude/CLAUDE.md` while
      preserving pre-existing user instructions byte-for-byte outside the OAW
      block.
- [ ] The Claude entrypoint uses Claude Code's documented import mechanism and
      points at the resolved canonical policy path.
- [ ] Repeating an identical install reports unchanged and does not rewrite
      managed files.
- [ ] `update` derives content only from the current checkout and updates a
      clean OAW installation without fetching or executing remote content.
- [ ] `--dry-run` reports all intended creates and edits while leaving config,
      state, and target files unchanged.
- [ ] A clean uninstall removes only the OAW block and OAW-owned artifacts;
      an otherwise empty file created by OAW may be removed, while user content
      remains.
- [ ] Black-box tests cover fresh install, preserved content, idempotence,
      local update, dry run, and clean uninstall under an isolated home.

