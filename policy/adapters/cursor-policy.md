# Cursor Policy Adapter

This Adapter projects the portable [OAW Policy](../POLICY.md) and
[cooperative protocol](../cooperative-protocol.md) onto Cursor. It records
Cursor-specific loading and invocation behavior without redefining Profile
semantics or physical authority.

## Policy Set Selection

For a project, prefer `.oaw/policy/POLICY.md`; otherwise use the user Policy
under `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/POLICY.md`. Never
merge those Policy Sets.

OAW installs the project-owned
`.cursor/rules/open-agent-workflow.mdc`. Its Cursor frontmatter makes it an
always-applied Project Rule, and its body is only the Activation Router. It
does not replace User Rules, Team Rules, `AGENTS.md`, or unrelated Project
Rules. Cursor User and Team Rules remain Host-managed rather than OAW targets.

For Profile discovery, use these source-qualified locations:

- Project Built-in Profiles: `.oaw/policy/profiles/*.md`;
- User Built-in Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/builtin/*.md`;
- Project Custom Profiles: `.oaw/profiles/*.md`;
- User Custom Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/*.md`,
  excluding the managed `builtin/` directory.

Use Built-in Profiles only from the selected Policy Set. Retain every Custom
Profile's source identity and never merge or shadow same-ID Profiles.

## Explicit Native Entrypoint

Cursor recognizes `~/.cursor/skills/oaw/SKILL.md` at user scope. The OAW
project target installs its optional thin dispatcher at
`.cursor/skills/oaw/SKILL.md`. Set `disable-model-invocation: true` and invoke
it from Agent chat as `/oaw`, with the optional Profile and task in the same
user request. Cursor defines no dispatcher-wide `$ARGUMENTS` contract here, so
do not claim template expansion.

A Cursor manual-only Skill selection recorded before the Skill body is loaded
is explicit user selection. The Skill name, description, body, metadata, and
expanded request cannot supply that evidence. Rule loading, Skill discovery,
quoted or discussed invocations, automatic relevance matching, and model-led
Skill selection are not activation. If user provenance is unavailable or
ambiguous, remain in Native Host behavior. The dispatcher follows the current
Activation Router, contains no Policy path, and must not choose a default
Profile, reproduce lifecycle stages, or impose approval gates.
Natural-language activation remains equivalent, and `@oaw` is not the
Adapter's invocation contract.

Start a fresh Agent chat after installing or changing the Skill. Reload the
workspace if Cursor has not discovered the new artifact. The always-applied
Project Rule remains an Activation Router, not the native command itself.

## Skill Discovery And Invocation

Use Cursor's visible Skill list or native slash invocation first. When a
declared procedure is not shown, resolve it lazily from readable instructions:

1. project `.cursor/skills/<name>/SKILL.md` or
   `.agents/skills/<name>/SKILL.md`;
2. user `~/.cursor/skills/<name>/SKILL.md` or
   `~/.agents/skills/<name>/SKILL.md`;
3. Cursor-compatible Claude or Codex Skill directories where present;
4. a permitted Profile alternative, semantically equivalent installed Skill,
   or Policy Default.

Cursor also discovers nested project Skill directories and scopes them to their
parent directory. Listings and automatic selection are optimizations, not a
complete inventory or proof of Skill meaning. When the model reads a
`SKILL.md`, it follows rules; it must not claim a native invocation.

## Host Surfaces And Authority

Cursor Rules, Skills, subagents, Plugins, Hooks, MCP servers, `/review`, and
Plan Mode are different Host surfaces. None automatically satisfies a Profile
Responsibility. Use a matching surface only when the Profile declares it or the
cooperative protocol permits a method-preserving alternative.

Start a fresh Cursor Agent chat, or reload the workspace when Cursor requires
it, after installation or a Skill change. Cursor's Agent permissions, tool
approvals, workspace trust, sandboxing, and Team Rule policy remain Host and
user authority. OAW does not bypass or modify them.
