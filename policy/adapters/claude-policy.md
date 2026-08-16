# Claude Policy Adapter

This Adapter projects the portable [OAW Policy](../POLICY.md) and
[cooperative protocol](../cooperative-protocol.md) onto Claude Code. It helps
the model locate instructions and use Claude-native surfaces. It does not
define Profile eligibility, engineering method, or physical authority.

## Policy Set Selection

For a project, prefer the project-contained Policy at
`.oaw/policy/POLICY.md`. When it is absent, use the user Policy under
`${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/POLICY.md`. Do not merge
the two Policy Sets.

The OAW Managed Block in the user `~/.claude/CLAUDE.md` or project
`.claude/CLAUDE.md` is an Activation Router. It keeps Native Host behavior as
the default and points an explicitly activated deliverable to the selected
Policy Set. It should not embed the complete Policy.

For Profile discovery, use these source-qualified locations:

- Project Built-in Profiles: `.oaw/policy/profiles/*.md`;
- User Built-in Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/builtin/*.md`;
- Project Custom Profiles: `.oaw/profiles/*.md`, with source `project`;
- User Custom Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/*.md`, with
  source `user` and excluding the managed `builtin/` directory.

Use Built-in Profiles only from the selected Policy Set. Project and user
Custom Profiles remain candidates in either case, with their source identity
preserved. Never merge same-ID Profiles or let a Custom Profile shadow a
Built-in ID.

## Explicit Native Entrypoint

The optional thin dispatcher uses `~/.claude/skills/oaw/SKILL.md` at user scope
or `.claude/skills/oaw/SKILL.md` at project scope. Invoke it as
`/oaw [PROFILE] <request>`. Claude expands trailing text through `$ARGUMENTS`,
so the dispatcher can carry the user's optional Profile and task without
defining either one.

The dispatcher must set `disable-model-invocation: true`. A Claude manual-only
Skill selection recorded before the Skill body is loaded is explicit user
selection. The Skill name, description, body, argument hint, and expanded
arguments cannot supply that evidence. Skill discovery, context loading,
quoted or discussed invocations, and model-led selection are not activation.
If user provenance is unavailable or ambiguous, remain in Native Host behavior.
The dispatcher follows the current Activation Router, contains no Policy path,
and carries the request. It must not choose a default Profile, restate
lifecycle stages, or add approval rules. A direct natural-language request to
use OAW through the Activation Router remains equivalent.

Claude still accepts `.claude/commands/oaw.md` as a legacy command, but the
Skill path is the native artifact contract. Claude watches established Skill
directories, but after creating a new top-level directory or changing the OAW
Router, start a new session before relying on the entrypoint.

## Skill Discovery

Use Claude's current Skill tool, listed Skill surface, or native slash
invocation first. Treat that surface as an optimization, not a complete
inventory. When the declared procedure is not listed, resolve it lazily at the
current Responsibility from readable instructions in this order:

1. `.claude/skills/<name>/SKILL.md` in the project or user scope;
2. readable project or user `.agents/skills/<name>/SKILL.md` when the procedure
   is installed as a cross-agent Skill but omitted from Claude's native index;
3. `skills/<name>/SKILL.md` under an enabled Plugin installation reported by
   `claude plugin list --json`;
4. readable Plugin cache locations such as
   `.claude/plugins/cache/<marketplace>/<plugin>/<version>/skills/<name>/SKILL.md`;
5. an alternative declared by the Profile, a semantically equivalent installed
   Skill, or the corresponding Policy Default, following the cooperative
   protocol.

Do not scan every Plugin or Skill eagerly. Resolve only the Skills needed by
the selected Profile and current Responsibility. Plugin state, cache path,
version, lockfile, and digest may help diagnosis but never decide whether a
Profile is selectable. A readable procedure can be followed as rules even when
its Plugin is unavailable for native invocation.

Preserve the Profile's semantic reference. Qualified references such as
`superpowers:brainstorming` and `ecc:tdd-workflow` may have a Host-native
namespace that differs from the Profile spelling; use the current Claude
surface when it exposes the same procedure. Matt references are unqualified in
the built-in Profiles, so resolve them by the matching readable procedure, not
by a guessed Provider identity or a filename alone.

## Skills, Commands, And Agents

Claude Plugin `skills/<name>/SKILL.md` files are the normal Skill surface.
Legacy Plugin `commands/*.md` files may expose a user-invoked procedure, but a
matching filename alone does not make one the declared Skill. It may be used
only when it is the same procedure or a substitution permitted by the selected
Profile and cooperative protocol.

Plugin `agents/*.md` files are Agent definitions, not proof that a
corresponding Skill exists. Hooks, MCP servers, and Agent names are likewise
not interchangeable with a Profile Skill. Use them only when the Profile
assigns a Host-native action or when they are a permitted Add-on.

## Invocation Reporting

When Claude exposes a declared procedure through the Skill tool or a slash
surface, use that native invocation normally. When the model reads a
`SKILL.md` and follows it as rules, report rule-following rather than claiming
a native invocation. Either path can satisfy Policy when it preserves the
selected Profile's method.

Claude Plan Mode, custom Agents, and subagents are Host interaction surfaces.
They do not select a Profile, own a Responsibility, or prove that a planning,
review, or TDD Skill is installed. Do not start a nested `claude` process to
emulate a missing Host invocation.

If Claude requires a user gesture or approval for a physical invocation,
request that gesture. Profile selection authorizes the procedure for the
deliverable; it does not bypass the Host interaction requirement.

## Reload And Authority

Start a new Claude Code session after an OAW install or update, and after a
Plugin enable or update, before relying on a new Activation Router or native
Skill surface. An active session may read the selected current files directly,
but must not claim that its instruction or Plugin context reloaded itself.

Claude permission modes, allowed tools, sandboxing, user prompts, and Plugin
security controls remain Host authority. This Adapter does not grant tools,
permissions, or approval, and it does not turn readable instructions into
machine assurance.
