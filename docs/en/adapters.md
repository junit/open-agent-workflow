# Host Adapters

An adapter is installation and loading guidance for one Agent Host. It is not
an engineering method and does not add Profile Responsibilities.

## Adapter Contract

Each adapter documents the Activation Router and thin native entrypoint paths,
scope and precedence, managed-block or owned-file rules, refresh behavior, and
native Skill or command surfaces. It may mention a readable cache location as
a fallback, but it must not require a particular cache, lockfile, revision, or
digest for Policy operation.

The Canonical Policy Set currently includes adapters for Claude, Codex, Gemini,
OpenCode, Cursor, Windsurf, Cline, Roo, and Copilot. Each applies the same
portable semantics through its own native instruction format. To add an
installable Host target, add its Adapter guidance under `policy/adapters/` and
its destination coordinates to `internal/management/targets.go`.

## Installed Coordinates

A user-scoped install supports the four Hosts that expose stable user
coordinates. `$XDG_CONFIG_HOME` uses the platform-resolved configuration root;
its usual Unix default is `~/.config`.

| Host | User Activation Router | User native entrypoint |
| --- | --- | --- |
| Claude | `~/.claude/CLAUDE.md` | `~/.claude/skills/oaw/SKILL.md` |
| Codex | `~/.codex/AGENTS.md` | `~/.agents/skills/oaw/SKILL.md` and `~/.agents/skills/oaw/agents/openai.yaml` |
| Gemini | `~/.gemini/GEMINI.md` | `~/.gemini/commands/oaw.toml` |
| OpenCode | `$XDG_CONFIG_HOME/opencode/AGENTS.md` | `$XDG_CONFIG_HOME/opencode/commands/oaw.md` |

A project-scoped install supports all nine Hosts and writes paths relative to
the selected project root.

| Host | Project Activation Router | Project native entrypoint |
| --- | --- | --- |
| Claude | `.claude/CLAUDE.md` | `.claude/skills/oaw/SKILL.md` |
| Codex | `AGENTS.md` | `.agents/skills/oaw/SKILL.md` and `.agents/skills/oaw/agents/openai.yaml` |
| Gemini | `GEMINI.md` | `.gemini/commands/oaw.toml` |
| OpenCode | `AGENTS.md` | `.opencode/commands/oaw.md` |
| Cursor | `.cursor/rules/open-agent-workflow.mdc` | `.cursor/skills/oaw/SKILL.md` |
| Windsurf | `.devin/rules/open-agent-workflow.md` | `.windsurf/workflows/oaw.md` |
| Cline | `.clinerules/open-agent-workflow.md` | `.cline/skills/oaw/SKILL.md` |
| Roo | `.roo/rules/open-agent-workflow.md` | `.roo/commands/oaw.md` |
| Copilot | `.github/instructions/open-agent-workflow.instructions.md` | `.github/skills/oaw/SKILL.md` |

The Copilot target is specifically a **Copilot CLI Agent Skill**. It is not a
VS Code Prompt File under `.github/prompts/`, and no cross-surface Prompt File
behavior is part of this contract.

## Invocation And Refresh

| Host | Explicit invocation | Refresh after install or change |
| --- | --- | --- |
| Claude | `/oaw [PROFILE] <request>` | Start a new session after creating a new top-level Skill directory or changing the Router. |
| Codex | `$oaw [PROFILE] <request>` | Codex normally detects Skill changes; restart the session if `oaw` is not visible. |
| Gemini | `/oaw [PROFILE] <request>` | Run `/commands reload`; start a new session for Router or broader context changes. |
| OpenCode | `/oaw [PROFILE] <request>` | Restart OpenCode. |
| Cursor | `/oaw [PROFILE] <request>` | Start a fresh Agent chat; reload the workspace if the Skill is not visible. |
| Windsurf | `/oaw [PROFILE] <request>` | Start a new Cascade task or reload the workspace. |
| Cline | `/oaw [PROFILE] <request>` | Start a new Cline task or reload the active context. |
| Roo | `/oaw [PROFILE] <request>` | Start a fresh Roo task; use the documented VS Code window reload if needed. |
| Copilot CLI | `/oaw [PROFILE] <request>` | Run `/skills reload`; other compatible Agent surfaces require a fresh chat or Host reload. |

Natural-language activation, such as “Use OAW with SP-FULL to deliver this
change,” remains valid on every Host. A native entrypoint has no higher
priority and is never required for Policy operation. It only carries explicit
user intent, an optional Profile, and the request into the selected Policy
Set. It must not choose a default Profile, duplicate Responsibilities, define
lifecycle stages, or impose approval gates.

Automatic discovery, relevance matching, or model-led loading of an
OAW-named Skill is not explicit activation. Claude, Codex, and other Hosts with
a documented explicit-only control use it. Cline does not expose a documented
per-Skill manual-only field, so its entrypoint relies on Policy self-gating: it
must observe explicit user intent before activating OAW. Physical invocation
alone is not evidence: the top-level request or reliable Host metadata must
identify user selection. Every dispatcher follows the Activation Router and
contains no Policy path, keeping Host template preprocessing away from install
coordinates.

## Target Ownership

Managed-block Routers preserve surrounding instructions. Native entrypoints,
Codex entrypoint metadata, and Routers whose Host format requires a dedicated
file are owned files with no user content. Installation refuses to overwrite
or adopt an untracked file at an owned destination, including under `--force`.

Install State tracks every artifact independently. `check` reports a missing
or edited tracked entrypoint as drift. `update` refreshes the Router and native
entrypoint from the same release. An edited tracked artifact can use the normal
force-and-backup repair path. A missing tracked file has no recoverable original
to back up, so force mode refuses it; restore the exact file from a trusted copy
before retrying. `uninstall` removes clean OAW-owned files and its empty owned
directories while preserving surrounding managed-file content and foreign
files. An adapter must declare each destination's ownership so these operations
remain conservative.

## Separation

Portable rules belong in POLICY.md, cooperative-protocol.md, and Profiles.
Host paths and invocation details belong here. Machine identity and attestation
belong in optional Machine Assurance. Keeping those layers separate prevents a
Host-specific scanner from becoming a Policy dependency.

These coordinates and management contracts do not by themselves claim live
runtime end-to-end validation in each Host. Runtime dogfood is reported only
after the corresponding real Host session has been exercised.
