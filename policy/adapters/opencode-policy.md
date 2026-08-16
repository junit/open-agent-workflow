# OpenCode Policy Adapter

This Adapter projects the portable [OAW Policy](../POLICY.md) and
[cooperative protocol](../cooperative-protocol.md) onto OpenCode. It documents
OpenCode's loading and invocation surfaces only; portable Policy and Profiles
remain the engineering-method authority.

## Policy Set Selection

For a project, prefer `.oaw/policy/POLICY.md`. Otherwise use
`${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/POLICY.md`. Do not merge
the two Policy Sets.

The OAW Managed Block in user `$XDG_CONFIG_HOME/opencode/AGENTS.md` or project
`AGENTS.md` is an Activation Router. It keeps native OpenCode work as the
default and redirects only an explicitly activated deliverable to the selected
Policy Set.

For Profile discovery, use these source-qualified locations:

- Project Built-in Profiles: `.oaw/policy/profiles/*.md`;
- User Built-in Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/builtin/*.md`;
- Project Custom Profiles: `.oaw/profiles/*.md`;
- User Custom Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/*.md`,
  excluding the managed `builtin/` directory.

Use Built-in Profiles only from the selected Policy Set. Preserve Custom
Profile source identity and never merge same-ID Profiles.

## Skill Discovery And Invocation

Start with `opencode debug skill`; `opencode debug paths` and
`opencode debug config` can explain the active configuration. These are
inventory and diagnostic aids, not eligibility checks. Resolve only the
declared Skill needed by the current Responsibility from:

1. `.opencode/skill/<name>/SKILL.md` or `.opencode/skills/<name>/SKILL.md`;
2. `$XDG_CONFIG_HOME/opencode/skill/<name>/SKILL.md` or
   `$XDG_CONFIG_HOME/opencode/skills/<name>/SKILL.md`;
3. automatically visible `~/.claude/skills/<name>/SKILL.md` and
   `~/.agents/skills/<name>/SKILL.md`, plus compatible project locations
   exposed by the current installation;
4. additional locations configured through `skills.paths` or `skills.urls`;
5. a permitted Profile alternative, semantically equivalent installed Skill,
   or Policy Default.

Project configuration overrides global configuration. Do not infer a Provider
from a path, and do not use config identity, cache state, or a listing omission
to reject MATT, ECC, Superpowers, or a Custom Profile. A readable procedure
can still be followed as rules when no native invocation exists.

## Commands, Agents, And Authority

OpenCode Agents (`.opencode/agent(s)`), Commands
(`.opencode/command(s)`), Plugins, MCP servers, and Skills are distinct
surfaces. A Command or Agent with the same name as a Profile reference is not
proof of that Skill. Use it only as a Profile-declared Host-native action or a
permitted substitution.

An OpenCode plan Agent or read-only Agent is a Host execution mode, not a
Profile or Responsibility owner. OpenCode configuration, agent files, plugins,
and Skill inventory load at startup; restart OpenCode after changing them.
OpenCode's permission rules and user approval remain physical authority and are
never widened by selecting an OAW Profile.
