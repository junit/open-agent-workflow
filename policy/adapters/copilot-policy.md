# Copilot Policy Adapter

This Adapter projects the portable [OAW Policy](../POLICY.md) and
[cooperative protocol](../cooperative-protocol.md) onto GitHub Copilot CLI and
its compatible Agent surfaces. It documents Host loading and discovery without
selecting Profiles or expanding Copilot permissions.

## Policy Set Selection

For a project, prefer `.oaw/policy/POLICY.md`; otherwise use the user Policy
under `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/POLICY.md`. Do not
merge the two Policy Sets.

OAW installs the project-owned
`.github/instructions/open-agent-workflow.instructions.md`. Its `applyTo:
"**"` frontmatter makes it a modular Copilot instruction for every matching
project file; its body is only the Activation Router. Copilot may also combine
`AGENTS.md`, `CLAUDE.md`, `GEMINI.md`,
`.github/copilot-instructions.md`, user `$HOME/.copilot` instructions, and
other applicable instruction files. OAW does not replace or reorder them.

For Profile discovery, use these source-qualified locations:

- Project Built-in Profiles: `.oaw/policy/profiles/*.md`;
- User Built-in Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/builtin/*.md`;
- Project Custom Profiles: `.oaw/profiles/*.md`;
- User Custom Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/*.md`,
  excluding the managed `builtin/` directory.

Use Built-in Profiles only from the selected Policy Set. Retain Custom Profile
source qualification and never merge or shadow same-ID Profiles.

## Skill Discovery And Invocation

Use `/skills list`, `/skills info <name>`, `copilot skill list`, or an exposed
native invocation first. When a declared procedure is not listed, resolve it
lazily from readable instructions:

1. project `.github/skills/<name>/SKILL.md`,
   `.claude/skills/<name>/SKILL.md`, or `.agents/skills/<name>/SKILL.md`;
2. user `~/.copilot/skills/<name>/SKILL.md` or
   `~/.agents/skills/<name>/SKILL.md`;
3. Skills supplied by a Plugin reported through `copilot plugin list`;
4. a permitted Profile alternative, semantically equivalent installed Skill,
   or Policy Default.

Use `/skills reload` after changing Skills in an active CLI session. Plugin
state, Skill listings, and locations assist diagnosis but cannot decide Profile
eligibility. If the model reads `SKILL.md` and follows it as rules, report that
accurately instead of claiming native Skill invocation.

## Host Surfaces And Authority

Copilot Skills, custom Agents, Plugins, Hooks, MCP servers, `/plan`, `/review`,
and `/fleet` are separate Host surfaces. None proves a Profile Responsibility.
Use an Agent or command only when the Profile declares a Host-native action or
the cooperative protocol permits a method-preserving alternative.

Exit and resume, or start a new Copilot CLI session after changing custom
instructions; active sessions do not reload them immediately. Copilot trusted
directories, sandboxing, tool, path, URL, and MCP approvals remain Host and
user authority. A selected Profile does not bypass them.
