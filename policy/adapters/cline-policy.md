# Cline Policy Adapter

This Adapter projects the portable [OAW Policy](../POLICY.md) and
[cooperative protocol](../cooperative-protocol.md) onto Cline. It is a loading
and discovery guide, not a parallel workflow or permission system.

## Policy Set Selection

For a project, prefer `.oaw/policy/POLICY.md`; otherwise use the user Policy
under `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/POLICY.md`. Do not
merge the two Policy Sets.

OAW installs the project-owned
`.clinerules/open-agent-workflow.md` as an Activation Router. It coexists with
other Cline Rules and does not take over their content.

For Profile discovery, use these source-qualified locations:

- Project Built-in Profiles: `.oaw/policy/profiles/*.md`;
- User Built-in Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/builtin/*.md`;
- Project Custom Profiles: `.oaw/profiles/*.md`;
- User Custom Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/*.md`,
  excluding the managed `builtin/` directory.

Use Built-in Profiles only from the selected Policy Set. Keep every Custom
Profile's source qualifier and never merge same-ID Profiles.

## Explicit Native Entrypoint

Cline recognizes `~/.cline/skills/oaw/SKILL.md` at user scope. The OAW project
target installs its optional thin dispatcher at `.cline/skills/oaw/SKILL.md`.
Invoke it as `/oaw` and keep the optional Profile and task in the same user
request. Cline does not define a portable dispatcher argument-expansion macro.

The original pre-expansion Cline user input must independently show that the
user selected this Skill, or reliable Cline metadata must identify that user
selection. The Skill name, description, body, and expanded request cannot
supply the evidence. Cline may match Skills from their descriptions and does
not expose a documented per-Skill manual-only field, so automatic matching,
quoted or discussed invocations, `use_skill` selection by the model, listing,
or loading of `oaw` must not activate OAW. If Cline does not expose the original
input or reliable user-selection metadata, the dispatcher remains inert and
the user can activate OAW in natural language through the Router. It contains
no Policy path and must not supply a default Profile, lifecycle stages, or
approval rules.

Use a new Cline task or reload the active context after installing or changing
the dispatcher before relying on fresh discovery.

## Skill Discovery And Invocation

Use Cline's Skills panel, `use_skill` tool, or `/skill-name` invocation first.
When a declared Skill is not listed, resolve it lazily from readable
instructions:

1. project `.cline/skills/<name>/SKILL.md`,
   `.clinerules/skills/<name>/SKILL.md`, or `.claude/skills/<name>/SKILL.md`;
2. user `~/.cline/skills/<name>/SKILL.md`;
3. readable project or user `.agents/skills/<name>/SKILL.md` when the
   cross-agent procedure is not in Cline's native inventory;
4. a permitted Profile alternative, semantically equivalent installed Skill,
   or Policy Default.

Cline's enabled-state toggle, listing, and descriptive matching are advisory.
They cannot prove the semantic content of a Skill or reject a Profile. A Skill
may be manually triggered through `/name`; when the procedure is read directly
from `SKILL.md`, report rule-following rather than native invocation. When a
global and project Skill share a name, Cline's current native loader gives the
global Skill precedence; inspect the displayed location instead of guessing.

## Host Surfaces And Authority

Cline Rules, Skills, Commands, Plugins, MCP servers, subagents, Checkpoints,
and Plan and Act modes are distinct features. Plan Mode can explore and plan
without editing but is not a Profile, planning Skill, review, or verification
claim. Switch to Act Mode only when Cline and the user authorize physical
changes.

Use a new task or reload the active Cline context after installing or changing
rules and Skills before relying on fresh discovery. Cline approvals, Auto
Approve or YOLO settings, provider credentials, and tool permissions remain
Host authority. OAW never overrides them.
