# 04 - Project-scope core and extension adapters

**What to build:** Allow a user to apply the same OAW governance to one project
across all core tools and the five extension tools, using supported project
rule locations and a single project operation that never changes an agent
tool's user-level configuration. OAW-owned canonical configuration and
installation-state metadata remain coordinated across scopes.

**Blocked by:** 02 - Claude user-scope lifecycle.

**Status:** completed

- [x] `--project <path>` resolves an existing project directory, constructs
      adapter destinations only from registry-owned relative paths beneath it,
      and binds its XDG-hosted state to that exact physical project root.
- [x] Project adapters work for Claude Code, Codex CLI, Gemini CLI, and
      OpenCode without changing their user-level instruction files.
- [x] Cursor receives a valid `.mdc` rule with required frontmatter; Windsurf
      uses the current `.devin/rules` surface; Cline uses `.clinerules`; Roo
      Code uses `.roo/rules`; and Copilot uses a path-specific
      `.github/instructions` file.
- [x] The default project target set and explicit mixed target sets are
      deterministic and documented by command output.
- [x] Where tools share `AGENTS.md`, OAW installs one coherent managed block
      rather than duplicate or nested blocks.
- [x] Install, repeat install, local update, dry run, and clean uninstall work
      for every project adapter while preserving unrelated project rules.
- [x] When one scope changes the shared canonical policy, every valid user and
      project state that references its stable path receives the same version
      and policy checksum without rewriting another scope's adapter files;
      uninstall retains the policy until the final path reference is removed.
- [x] Black-box tests exercise all nine project destinations from paths that
      contain spaces and prove that user-scope files remain untouched.

Release boundary: hostile symlink components, destination revalidation, and
TOCTOU-resistant containment are release-blocking hardening owned by Ticket 05
Task 3. Ticket 04 owns registry-derived lexical destinations and physical-root
state binding, not that later filesystem-hardening implementation.
