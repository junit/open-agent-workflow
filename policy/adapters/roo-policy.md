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

## Explicit Native Entrypoint

Roo recognizes `~/.roo/commands/oaw.md` at user scope. The OAW project target
installs its optional thin Custom Command at `.roo/commands/oaw.md`. Invoke it
as `/oaw [PROFILE] <request>`. The command may provide `description` and
`argument-hint` metadata for the picker, but the hint is not an
argument-substitution macro; the dispatcher must use the current user request
without inventing expanded arguments.

The original pre-expansion Roo user input must independently show that the user
selected this Custom Command, or reliable Roo metadata must identify that user
selection. The command name, description, body, argument hint, and expanded
request cannot supply the evidence. If an experimental Host setting lets the
model invoke slash commands, that invocation and any quoted or discussed form
are not activation and remain subject to Roo's command approval. If Roo does
not expose the original input or reliable user-selection metadata, the command
remains inert and the user can activate OAW in natural language through the
Router. It contains no Policy path and must not choose a default Profile,
duplicate lifecycle stages, or add approval rules.

Start a fresh Roo task after installing or changing the command. If it is not
visible, use Roo's documented VS Code window reload before relying on it.

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
