# Windsurf Policy Adapter

This Adapter projects the portable [OAW Policy](../POLICY.md) and
[cooperative protocol](../cooperative-protocol.md) onto Windsurf, whose current
Desktop documentation also uses the Devin Desktop name. It documents Host
surfaces only and does not add Profile or execution authority.

## Policy Set Selection

For a project, prefer `.oaw/policy/POLICY.md`; otherwise use the user Policy
under `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/POLICY.md`. Do not
merge the two Policy Sets.

OAW installs the project-owned
`.devin/rules/open-agent-workflow.md` with `trigger: always_on`. Current
Windsurf/Devin Desktop prefers `.devin/rules`; legacy `.windsurf/rules` is a
fallback location. The installed file is an Activation Router only. Global
rules at `~/.codeium/windsurf/memories/global_rules.md`, workspace
`AGENTS.md`, and unrelated Rules remain Host-owned.

For Profile discovery, use these source-qualified locations:

- Project Built-in Profiles: `.oaw/policy/profiles/*.md`;
- User Built-in Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/builtin/*.md`;
- Project Custom Profiles: `.oaw/profiles/*.md`;
- User Custom Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/*.md`,
  excluding the managed `builtin/` directory.

Use Built-in Profiles only from the selected Policy Set. Keep Custom Profile
source identity and do not merge or shadow IDs.

## Explicit Native Entrypoint

Windsurf recognizes user Workflows at
`~/.codeium/windsurf/global_workflows/oaw.md`. The OAW project target installs
its optional thin Workflow at `.windsurf/workflows/oaw.md`. Invoke it from
Cascade as `/oaw` and place any optional Profile and task in the same user
request. Windsurf documents Workflows as manual slash entrypoints but does not
define a portable argument-substitution macro for this file, so the dispatcher
must not claim one.

A user-entered `/oaw` is explicit OAW activation. Loading the always-on Router,
discovering Skills, or model-led selection of an `oaw`-named Skill is not. The
Workflow treats invocation alone as insufficient proof of user selection. It
follows the current Activation Router, contains no Policy path, and must not
choose a default Profile, restate lifecycle stages, or add approval gates. A
natural-language activation request remains valid without a Workflow.

Start a new Cascade task or reload the workspace after installing or changing
the Workflow. The `.devin/rules/open-agent-workflow.md` Router and the
`.windsurf/workflows/oaw.md` entrypoint are distinct Host surfaces.

## Skill Discovery And Invocation

Use Cascade's current Skill surface first. If a declared procedure is absent
there, resolve only that Skill from readable instructions in this order:

1. `.windsurf/skills/<name>/SKILL.md` or `.agents/skills/<name>/SKILL.md`;
2. `~/.codeium/windsurf/skills/<name>/SKILL.md` or
   `~/.agents/skills/<name>/SKILL.md`;
3. Claude-compatible Skill locations when the Host has enabled their reading;
4. a permitted Profile alternative, semantically equivalent installed Skill,
   or Policy Default.

Cascade uses Skill descriptions for automatic selection and loads full Skill
instructions on demand. A missing display entry, cache detail, or Provider
identity does not decide Profile eligibility. Distinguish native Skill use from
reading a `SKILL.md` and following its rules.

## Host Surfaces And Authority

Rules, `AGENTS.md`, Workflows, Skills, Cascade modes, Devin Local agents,
Hooks, MCP servers, and Quick Review are distinct Host features. A Workflow
slash command or Plan-like mode can help work but does not itself own a Profile
Responsibility. Start a new Cascade task or reload the workspace after changing
rules or Skills before assuming a session has reloaded them.

Windsurf/Devin Desktop controls all edits, commands, approvals, workspace
trust, extension policy, credentials, and sandboxing. OAW selection does not
expand any of those permissions.
