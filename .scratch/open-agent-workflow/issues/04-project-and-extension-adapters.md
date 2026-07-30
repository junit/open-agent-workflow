# 04 - Project-scope core and extension adapters

**What to build:** Allow a user to apply the same OAW governance to one project
across all core tools and the five extension tools, using supported project
rule locations and a single project operation that never changes global
configuration.

**Blocked by:** 02 - Claude user-scope lifecycle.

**Status:** ready-for-agent

- [ ] `--project <path>` resolves an existing project directory, confines all
      adapter writes to it, and binds its XDG-hosted state to that exact
      physical project root.
- [ ] Project adapters work for Claude Code, Codex CLI, Gemini CLI, and
      OpenCode without changing their user-level instruction files.
- [ ] Cursor receives a valid `.mdc` rule with required frontmatter; Windsurf
      uses the current `.devin/rules` surface; Cline uses `.clinerules`; Roo
      Code uses `.roo/rules`; and Copilot uses a path-specific
      `.github/instructions` file.
- [ ] The default project target set and explicit mixed target sets are
      deterministic and documented by command output.
- [ ] Where tools share `AGENTS.md`, OAW installs one coherent managed block
      rather than duplicate or nested blocks.
- [ ] Install, repeat install, local update, dry run, and clean uninstall work
      for every project adapter while preserving unrelated project rules.
- [ ] Black-box tests exercise all nine project destinations from paths that
      contain spaces and prove that user-scope files remain untouched.
