# Roo Policy Adapter

This Adapter projects the portable [OAW Policy](../POLICY.md) and
[cooperative protocol](../cooperative-protocol.md) onto Roo Code. It describes
Roo's host-specific instruction and Skill surfaces only.

## Policy Set Selection

For a project, prefer `.oaw/policy/POLICY.md`; otherwise use the user Policy
under `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/POLICY.md`. Do not
merge the two Policy Sets.

OAW installs the project-owned `.roo/rules/open-agent-workflow.md` Activation
Router. Roo aggregates it with global `~/.roo/rules/`, workspace `.roo/rules/`,
and, where enabled, root `AGENTS.md` instructions. OAW owns only its created
file and never modifies the surrounding rule sets or Roo modes.

For Profile discovery, use these source-qualified locations:

- Project Built-in Profiles: `.oaw/policy/profiles/*.md`;
- User Built-in Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/builtin/*.md`;
- Project Custom Profiles: `.oaw/profiles/*.md`;
- User Custom Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/*.md`,
  excluding the managed `builtin/` directory.

Use Built-in Profiles only from the selected Policy Set. Retain Custom Profile
sources and never merge or shadow IDs.

## Skill Discovery And Invocation

Use Roo's current Skill inventory or native selection first. If a declared
procedure is not visible, resolve only that Skill from readable instructions:

1. project `.roo/skills/<name>/SKILL.md` or
   `.agents/skills/<name>/SKILL.md`;
2. user `~/.roo/skills/<name>/SKILL.md` or
   `~/.agents/skills/<name>/SKILL.md`;
3. mode-specific `skills-<mode>` directories when the current Roo mode applies;
4. a permitted Profile alternative, semantically equivalent installed Skill,
   or Policy Default.

Roo gives project Skills priority over global Skills and Roo-specific paths
priority over `.agents` paths at the same scope. This is selection behavior,
not Provider attestation or Profile admission. Roo loads full `SKILL.md`
instructions on demand; reading it and following the procedure is not a native
invocation claim.

## Host Surfaces And Authority

Roo Rules, Custom Modes, Skills, Slash Commands, MCP servers, and subagents
are not interchangeable. A mode's tool restrictions or a Command with a
matching name cannot prove that a Profile Skill exists or owns a Responsibility.
Start a fresh Roo task, or reload the VS Code window when Roo requires it,
after changing rules, modes, or Skills.

Roo's mode permissions, file restrictions, command approval, MCP policy,
credentials, and user prompts remain physical authority. Profile selection
cannot bypass them.
