# Host Adapters

An adapter is installation and loading guidance for one Agent Host. It is not
an engineering method and does not add Profile Responsibilities.

## Adapter Contract

Each adapter documents the instruction file paths, scope and precedence,
managed-block or owned-file rules, reload behavior, and native Skill
invocation surfaces. It may mention a readable cache location as a fallback,
but it must not require a particular cache, lockfile, revision, or digest for
Policy operation.

The Canonical Policy Set currently includes adapters for Claude, Codex, Gemini,
OpenCode, Cursor, Windsurf, Cline, Roo, and Copilot. Each applies the same
portable semantics through its own native instruction format. To add an
installable Host target, add its Adapter guidance under `policy/adapters/` and
its destination coordinates to `internal/management/targets.go`.

| Host | OAW-managed instruction target |
| --- | --- |
| Claude | `~/.claude/CLAUDE.md` or `.claude/CLAUDE.md` |
| Codex | `~/.codex/AGENTS.md` or `AGENTS.md` |
| Gemini | `~/.gemini/GEMINI.md` or `GEMINI.md` |
| OpenCode | `$XDG_CONFIG_HOME/opencode/AGENTS.md` or `AGENTS.md` |
| Cursor | `.cursor/rules/open-agent-workflow.mdc` |
| Windsurf | `.devin/rules/open-agent-workflow.md` |
| Cline | `.clinerules/open-agent-workflow.md` |
| Roo | `.roo/rules/open-agent-workflow.md` |
| Copilot | `.github/instructions/open-agent-workflow.instructions.md` |

## Target Ownership

Managed-block targets preserve surrounding instructions. Owned-file targets
are OAW-created files with no user content. An adapter must declare which model
owns each destination so update and uninstall can be conservative.

## Separation

Portable rules belong in POLICY.md, cooperative-protocol.md, and Profiles.
Host paths and invocation details belong here. Machine identity and attestation
belong in optional Machine Assurance. Keeping those layers separate prevents a
Host-specific scanner from becoming a Policy dependency.
