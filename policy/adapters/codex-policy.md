# Codex Policy Adapter

This Adapter projects the portable [OAW Policy](../POLICY.md) and
[cooperative protocol](../cooperative-protocol.md) onto Codex. It helps the
model locate instructions and use Host-native surfaces. It does not define
Profile eligibility, engineering method, or physical authority.

## Policy Set Selection

For a project, prefer the project-contained Policy at
`.oaw/policy/POLICY.md`. When it is absent, use the user Policy under
`${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/POLICY.md`. Do not merge
the two Policy Sets.

The OAW Managed Block in `AGENTS.md` is an Activation Router. It keeps Native
Host behavior as the default and points an explicitly activated deliverable to
the selected Policy Set. It should not embed the complete Policy.

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

The optional thin dispatcher uses `~/.agents/skills/oaw/SKILL.md` at user scope
or `.agents/skills/oaw/SKILL.md` at project scope. Invoke it as
`$oaw [PROFILE] <request>`, or select `oaw` from `/skills` and keep the optional
Profile and task in the same user prompt. Codex does not define a portable
`$ARGUMENTS` expansion for a Skill, so the dispatcher must use the actual user
request rather than inventing one.

The Skill's `agents/openai.yaml` metadata should set
`policy.allow_implicit_invocation: false`. A user-entered `$oaw` or explicit
selection from `/skills` activates OAW; indexing, preloading, or model-led
selection of the same Skill does not. Invocation alone is not proof of user
selection. The dispatcher follows the current Activation Router, contains no
Policy path, and must not select a default Profile, reproduce Profile
Responsibilities, lifecycle stages, or approval gates. Natural-language
activation remains valid without the Skill.

Codex normally detects Skill changes automatically. If a new or changed
dispatcher is not visible, restart the Codex session before relying on it.

## Skill Discovery

Use Codex's current Skill index first, but treat it as an optimization rather
than a complete inventory. When a declared Skill is not listed, inspect readable
instructions lazily at relevant Codex locations, including:

- `.agents/skills/<name>/SKILL.md` in project or user scope;
- `.codex/skills/<name>/SKILL.md` where present;
- `.codex/plugins/cache/ecc/ecc/<version>/skills/<name>/SKILL.md`;
- `.codex/plugins/cache/openai-api-curated/superpowers/<version>/skills/<name>/SKILL.md`;
- other `.codex/plugins/cache/<plugin>/<version>/.../SKILL.md` locations exposed
  by the current installation.

Do not scan every Skill eagerly. Resolve only the Skills needed by the selected
Profile and current Responsibility. Version, source, lockfile, cache path, and
tree digest may help diagnostics but are never Policy eligibility conditions.

Matt Skills are ordinary readable Skill documents even when Codex omits them
from its generated index. ECC Skills use their public Codex Skill routes when
those instructions match the declared Responsibility. A filename on a Claude
Agent, Codex Role, command, Hook, or tool surface does not prove that a Skill
surface exists, and those surfaces are not interchangeable.

## Invocation Reporting

When Codex exposes a Skill as a native invocation, use that surface normally.
When the model reads a `SKILL.md` and follows it as rules, report that behavior
as rule-following rather than claiming a native invocation. Either path can
satisfy Policy when it preserves the selected Profile's method.

If Codex requires a user gesture for a physical invocation, request that gesture.
Profile selection already authorizes the Skill as an engineering procedure; it
does not bypass the Host interaction requirement.

## Planning and Review Surfaces

Codex /plan is a Host interaction surface, not a Profile, Responsibility owner,
or proof that a particular planning Skill is installed. Use it when it helps the
selected Profile's planning method.

The ECC `review-pr` command is specific to pull-request review. It does not own
generic review and remediation. For a working-tree or local delivery review,
use the selected Profile's declared review Skill or Codex's Host-native review
behavior.

## Skill Listings

Codex Skill indexes and Adapter-discovered paths are advisory. They cannot
prove Skill content, make a Profile available or unavailable, or veto
model-led Skill resolution. Re-read a relevant Skill when its instructions may
have changed, and respond to the actual semantic difference rather than cache
or revision metadata alone.

## Host-Native Tools and Effects

Codex owns shell execution, file edits, approvals, network access, browser or
MCP tools, subagents, sandboxing, and credentials. Apply their current
permissions exactly as for Native Host work. OAW Profile selection does not
expand them.

Use repository-native test and formatting commands. Preserve unrelated changes,
avoid destructive Git operations without approval, and report fresh command
output before completion.
