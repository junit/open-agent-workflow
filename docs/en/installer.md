# Installer

The installer manages one Canonical Policy Set at user or project scope. It
does not install a Profile's engineering Skills, inspect their contents,
invoke a model, or create workflow execution state. It does install the small
OAW dispatcher Skill, command, or Workflow that belongs to each selected Host
target.

## Commands

~~~text
oaw check [--project PATH] [--target IDS]
oaw install [--project PATH] [--target IDS] [--dry-run] [--force]
oaw update [--project PATH] [--target IDS] [--dry-run] [--force]
oaw uninstall [--project PATH] [--target IDS] [--dry-run] [--force]
oaw profile list
oaw profile show SOURCE:ID
oaw profile check SOURCE:ID
~~~

User installation defaults to the user Host instruction files. Project
installation writes a self-contained set under PATH/.oaw/policy and project
adapter files. A project set takes precedence over a user set without merging.

## Artifacts By Scope

Every selected target is one logical unit containing an Activation Router and
a thin native entrypoint. Codex also contains the metadata that disables
implicit Skill invocation. A user install supports these exact four targets:

| Host | User Router | User native entrypoint |
| --- | --- | --- |
| Claude | `~/.claude/CLAUDE.md` | `~/.claude/skills/oaw/SKILL.md` |
| Codex | `~/.codex/AGENTS.md` | `~/.agents/skills/oaw/SKILL.md`; `~/.agents/skills/oaw/agents/openai.yaml` |
| Gemini | `~/.gemini/GEMINI.md` | `~/.gemini/commands/oaw.toml` |
| OpenCode | `$XDG_CONFIG_HOME/opencode/AGENTS.md` | `$XDG_CONFIG_HOME/opencode/commands/oaw.md` |

A project install supports these exact nine targets, relative to the selected
project root passed through `--project PATH`:

| Host | Project Router | Project native entrypoint |
| --- | --- | --- |
| Claude | `.claude/CLAUDE.md` | `.claude/skills/oaw/SKILL.md` |
| Codex | `AGENTS.md` | `.agents/skills/oaw/SKILL.md`; `.agents/skills/oaw/agents/openai.yaml` |
| Gemini | `GEMINI.md` | `.gemini/commands/oaw.toml` |
| OpenCode | `AGENTS.md` | `.opencode/commands/oaw.md` |
| Cursor | `.cursor/rules/open-agent-workflow.mdc` | `.cursor/skills/oaw/SKILL.md` |
| Windsurf | `.devin/rules/open-agent-workflow.md` | `.windsurf/workflows/oaw.md` |
| Cline | `.clinerules/open-agent-workflow.md` | `.cline/skills/oaw/SKILL.md` |
| Roo | `.roo/rules/open-agent-workflow.md` | `.roo/commands/oaw.md` |
| Copilot | `.github/instructions/open-agent-workflow.instructions.md` | `.github/skills/oaw/SKILL.md` |

The Copilot artifact is a Copilot CLI Agent Skill, not a VS Code Prompt File.
Codex uses `$oaw [PROFILE] <request>`; every other target uses
`/oaw [PROFILE] <request>`. Natural-language requests to use OAW remain valid
and require no native entrypoint. Dispatchers contain no Policy path; they use
the Activation Router already installed for that Host, so Host template
preprocessing cannot reinterpret an installation coordinate.

## Ownership

Managed blocks preserve surrounding Host instructions. Owned files are created
only when the destination is absent. All native entrypoints and Codex's native
policy metadata are owned files; several project Router formats are owned
files as well. A foreign file at any owned destination is a conflict. Neither
ordinary install nor `--force` overwrites or adopts it.

Install State records the exact Policy Set files, every target artifact and
checksum, scope, and directories owned by that installation. A selected target
is complete only when all of its declared artifacts are recorded. Preparation
checks the whole request before applying changes, so one conflicting
entrypoint prevents a partial target install. Install State is private
bookkeeping for safe update and uninstall; it is not workflow progress.

Install State format 1 from releases 0.1.0 and 0.1.1 is accepted only as a
legacy upgrade input. `check` reports each clean legacy target as
`upgrade-required`. A clean `install` from the same release content, or an
`update` to current content, adds the native artifacts for every target already
recorded by that installation and atomically writes format 2. An occupied new
owned-file destination remains a foreign-file conflict, including under
`--force`. Partial uninstall may preserve format 1 for the remaining legacy
targets; fresh and migrated installations use format 2.

Install, update, and uninstall validate every path and source before writing.
`check` aggregates Router and entrypoint health: a missing, replaced, or edited
tracked owned file is drift. `update` refreshes all artifacts of the selected
target from one release. Force mode may create a private backup before repairing
an edited or replaced tracked file. It refuses a missing tracked file because no
original remains to back up; restore the exact file from a trusted copy before
retrying. Force never adopts untracked files or changes another scope's
installation.

If an install or legacy migration fails after writing begins, OAW restores
changed files in reverse order and removes only the still-empty directories it
created whose filesystem identity still matches creation time. Concurrent
replacement directories and foreign content are preserved rather than deleted.

`uninstall` removes clean tracked owned files, removes the OAW managed block
from shared instruction files, and removes only empty directories owned by the
installation. Foreign content is preserved. Tracked drift must be resolved or
handled through the same force-and-backup rules before destructive cleanup.

After installation or update, apply the Host refresh action documented in
[Host Adapters](adapters.md). Gemini uses `/commands reload`; Copilot CLI uses
`/skills reload`; Hosts without a reload command require the documented new
session, task, chat, workspace reload, or process restart. A clean `check`
result validates installed bytes and ownership, not live Host runtime E2E.

## Wrapper and Releases

install.sh is an offline wrapper that resolves only a sibling oaw or oaw.exe.
It never searches PATH for a different executable, downloads a release, or
builds code. Release archives contain the precompiled binary, wrapper, Policy
documentation, and checksums.

Build from source with go build -o ./oaw ./cmd/oaw. Verify a release with the
published SHA256SUMS before executing it.
