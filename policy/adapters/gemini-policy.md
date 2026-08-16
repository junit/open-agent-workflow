# Gemini Policy Adapter

This Adapter projects the portable [OAW Policy](../POLICY.md) and
[cooperative protocol](../cooperative-protocol.md) onto Gemini CLI. It
describes Gemini-native instruction and Skill surfaces; it does not select a
Profile, define an engineering method, or grant physical authority.

## Policy Set Selection

For a project, prefer `.oaw/policy/POLICY.md`. When it is absent, use the user
Policy under `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/POLICY.md`.
Do not merge the two Policy Sets.

The OAW Managed Block in user `~/.gemini/GEMINI.md` or project `GEMINI.md` is
an Activation Router. It preserves native Gemini behavior by default and
points only an explicitly activated deliverable to the selected Policy Set.

For Profile discovery, use these source-qualified locations:

- Project Built-in Profiles: `.oaw/policy/profiles/*.md`;
- User Built-in Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/builtin/*.md`;
- Project Custom Profiles: `.oaw/profiles/*.md`;
- User Custom Profiles:
  `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/*.md`,
  excluding the managed `builtin/` directory.

Preserve each Custom Profile's source. Do not merge or shadow IDs, and use
Built-in Profiles only from the selected Policy Set.

## Explicit Native Entrypoint

The optional thin dispatcher is a Gemini Custom Command at
`~/.gemini/commands/oaw.toml` for user scope or
`.gemini/commands/oaw.toml` for project scope. Invoke it as
`/oaw [PROFILE] <request>`. Its `prompt` may use Gemini's `{{args}}` expansion
to carry the optional Profile and task into the current user request.

A user-entered `/oaw` is explicit OAW activation. Command discovery, Skill
listing, extension loading, or model-led selection of an `oaw` procedure is
not. Invocation alone is not proof of user selection. The command follows the
current Activation Router, contains no Policy path, and must not set a default
Profile, duplicate lifecycle stages, or impose an approval gate.
Natural-language activation remains equivalent.

Run `/commands reload` after adding or changing the Custom Command in an active
Gemini session. A Router or broader session-context change still requires a
new session as described below.

## Skill Discovery And Invocation

Use Gemini's current Skill surface first: `/skills list`, `gemini skills list`,
or `gemini skills list --all`. These listings are advisory. Resolve a declared
procedure lazily from readable Skill documents when it is not listed:

1. project `.gemini/skills/<name>/SKILL.md` or
   `.agents/skills/<name>/SKILL.md`;
2. user `~/.gemini/skills/<name>/SKILL.md` or
   `~/.agents/skills/<name>/SKILL.md`;
3. `skills/<name>/SKILL.md` in an installed Gemini extension reported by
   `gemini extensions list`;
4. a permitted Profile alternative, semantically equivalent installed Skill,
   or Policy Default.

Gemini gives workspace Skills precedence over user Skills, and the `.agents`
alias wins within the same scope. Skill indexes, extension metadata, locations,
versions, and digests help diagnosis only; none decides Profile eligibility.
Use `/skills reload` after adding or changing a Skill in an active session.

An enabled Skill can be invoked through Gemini's native Skill surface. If the
model instead reads `SKILL.md` and follows it as rules, report rule-following,
not a native invocation. Extensions, Commands, MCP servers, and Agents are not
interchangeable with a declared Skill merely because their names match.

## Host Surfaces And Authority

Gemini Plan mode, slash commands, extensions, and subagents are Host surfaces.
They may help complete a selected Profile but do not themselves own planning,
review, TDD, verification, or Profile selection. Do not start a nested Gemini
process to imitate a missing invocation.

Start a new Gemini session after an OAW install or Router update before relying
on it. Gemini approval mode, tool permissions, sandboxing, extension consent,
credentials, and user prompts remain Host authority. An OAW Profile never
bypasses them.
