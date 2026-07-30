# 03 - Remaining core user adapters

**What to build:** Complete user-scope lifecycle support for Codex CLI, Gemini
CLI, and OpenCode through each tool's documented instruction surface, sharing
the same canonical OAW policy and lifecycle state as Claude Code.

**Blocked by:** 02 - Claude user-scope lifecycle.

**Status:** ready-for-agent

- [ ] Codex installs a model-visible managed bootstrap in
      `~/.codex/AGENTS.md` without claiming undocumented Markdown import
      behavior or overwriting unrelated instructions.
- [ ] Gemini installs a managed entrypoint in `~/.gemini/GEMINI.md` using its
      documented import behavior.
- [ ] OpenCode installs a model-visible managed bootstrap in
      `~/.config/opencode/AGENTS.md` without relying on automatic Markdown
      reference parsing.
- [ ] `--target` accepts one target or a deterministic comma-separated target
      set, and the default user install selects the four core adapters only.
- [ ] Install, repeat install, local update, dry run, and clean uninstall work
      for each core adapter and for a multi-target installation.
- [ ] Shared destination files are mutated once per operation and uninstalling
      one target never removes content still owned by another installed target.
- [ ] Black-box tests assert the exact target-native entrypoints and preserved
      user content for all four core targets.

