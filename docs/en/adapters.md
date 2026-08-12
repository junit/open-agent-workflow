# Adapter Evidence

[简体中文](../zh/adapters.md) | [README](../../README.md)

This guide documents how Open Agent Workflow (OAW) projects its one canonical
policy into each supported agent tool. OAW behavior is defined by the local
adapter registry and renderers in [lib/targets.sh](../../lib/targets.sh) and
[lib/render.sh](../../lib/render.sh). Provider behavior is documented only from
the official sources listed below.

Official sources **Retrieved: 2026-07-30**.

## Support Levels

| Target ID | Tool | OAW scopes | Control surface |
| --- | --- | --- | --- |
| `claude` | Claude Code | user + project | `policy` |
| `codex` | Codex CLI | user + project | `policy` |
| `gemini` | Gemini CLI | user + project | `policy` |
| `opencode` | OpenCode | user + project | `policy` |
| `cursor` | Cursor | project only | `policy` |
| `windsurf` | Windsurf / Devin rules | project only | `policy` |
| `cline` | Cline | project only | `policy` |
| `roo` | Roo Code | project only | `policy` |
| `copilot` | GitHub Copilot | project only | `policy` |

Core adapters are installed by default for user scope:
`claude,codex,gemini,opencode`. Project scope defaults to all nine targets in
the registry order shown above. Unsupported user-scope extension adapters are
 an OAW support decision, not a claim that the provider has no global settings.

## Host Integration Surfaces

Adapter installation and Host execution are separate concerns. Codex exposes
`oaw/codex-policy` by default and the separate audited `oaw/codex-host`
host-native Bridge as an explicit opt-in. The Bridge supports `CURRENT` and
`skill` bindings only. All other built-in targets remain policy surfaces unless
their own Host-native integration is explicitly installed and verified.

The Codex Bridge is not inferred from the `codex` target adapter. It must be
installed and trusted separately, and it reports current-session facts only
after a trusted Hook observation. A host-native integration is an explicit Host
feature, not a property inferred from a target name.

`CURRENT` means the active Agent session remains unchanged. `SUBAGENT` means the
active Agent Host creates a child through its native Subagent facility. Its
availability is session-dependent; a missing facility returns
`SUBAGENT_UNAVAILABLE` and has no process fallback.

A host-native adapter may report secret-free session facts, Provider Binding
Evidence, topology availability, Dispatch Packet status, and normalized
Receipts. The Agent Host owns physical execution authority. OAW never starts a
model process and never asks an adapter to reconstruct MCP, Hooks, Skills,
Plugins, authentication, sandbox, approvals, or private configuration.

### Provider Surface v4 evidence

Provider Descriptor `oaw.provider-descriptor/v4` separates Distribution
`ContentRoot` from Host Binding `InstallRoot`. The Host must observe the exact
enabled surface, resolve it beneath one exact installation, digest the complete
Binding tree, and match the source-pinned Distribution tree. Same-name paths,
shared ancestors, disabled skills, symlinks, or partial file hashes fail
closed.

Binding kinds are `skill`, `agent`, `role`, and `instruction`. Skills, Claude
custom Agents, Codex Roles, Instructions, Hooks, and tools never substitute for
one another. Static adapter or multi-agent metadata can be `host-configured`;
it cannot report live delegation as available. Current Codex observation proves
only `skill`/`CURRENT`; other kinds and actions require a stable Host API.

The Host may report `inherited`, `host-configured`, `restricted`, `unknown`, or
`unavailable` environment observations. A Receipt is evidence of what the Host
attested; it is not a claim that OAW physically contained the Host.

## OAW Paths

The canonical OAW policy is installed at
`$XDG_CONFIG_HOME/open-agent-workflow/ENGINEERING.md`, or
`~/.config/open-agent-workflow/ENGINEERING.md` when `XDG_CONFIG_HOME` is unset.
User-scope state is stored under
`$XDG_STATE_HOME/open-agent-workflow/installations/user.state`, or
`~/.local/state/open-agent-workflow/installations/user.state` when
`XDG_STATE_HOME` is unset. Project-scope state is stored under
`$XDG_STATE_HOME/open-agent-workflow/installations/projects/<project-id>.state`.

| Target ID | OAW user path | OAW project path | OAW ownership |
| --- | --- | --- | --- |
| `claude` | `$HOME/.claude/CLAUDE.md` | `.claude/CLAUDE.md` | Managed block |
| `codex` | `$HOME/.codex/AGENTS.md` | `AGENTS.md` | Managed block |
| `gemini` | `$HOME/.gemini/GEMINI.md` | `GEMINI.md` | Managed block |
| `opencode` | `$XDG_CONFIG_HOME/opencode/AGENTS.md` | `AGENTS.md` | Managed block |
| `cursor` | Not supported | `.cursor/rules/open-agent-workflow.mdc` | Owned file |
| `windsurf` | Not supported | `.devin/rules/open-agent-workflow.md` | Owned file |
| `cline` | Not supported | `.clinerules/open-agent-workflow.md` | Owned file |
| `roo` | Not supported | `.roo/rules/open-agent-workflow.md` | Owned file |
| `copilot` | Not supported | `.github/instructions/open-agent-workflow.instructions.md` | Owned file |

Managed-block adapters preserve unrelated content in a shared destination and
replace only the marked OAW block. Owned-file adapters require their exact
destination to be absent or already OAW-owned; OAW does not merge inside those
files. Those are OAW mechanical choices from [lib/targets.sh](../../lib/targets.sh)
and [lib/render.sh](../../lib/render.sh), not provider-level precedence rules.
Every adapter renders the same lazy **Activation Router** body. It is always
visible where a Host requires an always-applied rule, but that visibility is not
activation: without an explicit user instruction, the Router preserves Native
Host behavior and does not load the full Policy. In the matrix below,
**documented import** means a provider-defined file-import feature; OAW does not
use it for eager Policy loading.

## Adapter Matrix

| Target | Official loading behavior | OAW rendering choice | Precedence and reload caveats | Primary source |
| --- | --- | --- | --- | --- |
| Claude Code | Claude Code loads `CLAUDE.md` memory files and documents `@path` imports. It supports user memory under `~/.claude/CLAUDE.md` and project memory from `CLAUDE.md` or `.claude/CLAUDE.md`. Imports may be recursive, and Markdown comments are hidden from injected model context. | OAW writes a managed Activation Router that names the canonical policy path but does not import it eagerly. The ownership markers remain useful to the installer, but HTML comments are stripped from injected context. | Claude loads memory at session start. Its docs describe broad-to-specific memory loading and an `@` import limit; restart or refresh the Claude Code session when changed files are not reflected. | <https://code.claude.com/docs/en/memory> |
| Codex CLI | Codex documents `AGENTS.md` discovery, including user instructions under `~/.codex/AGENTS.md` and project instructions from `AGENTS.md` files. It documents fallback filenames and closer instructions overriding broader instructions by appearing later in the combined prompt. | OAW writes model-visible bootstrap text telling Codex to read the canonical policy. Codex has no official documented Markdown `@path` import behavior, so OAW does not rely on an import marker. | Codex rebuilds instructions for a new run or TUI session. A changed `AGENTS.md` should be picked up by the next Codex invocation; OAW does not claim live hot reload inside an already-running exchange. | <https://developers.openai.com/codex/guides/agents-md> |
| Gemini CLI | Gemini CLI uses hierarchical `GEMINI.md` context files. It documents global `~/.gemini/GEMINI.md`, project and subdirectory context, `@file.md` imports, nested imports, circular detection, and `/memory refresh`. | OAW writes a managed Activation Router that names the canonical policy path but does not import it eagerly. | Gemini context is hierarchical. More specific context can supplement or override broader guidance. Use `/memory show` to inspect loaded context and `/memory refresh` after edits. | <https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/reference/configuration.md>, <https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/reference/memport.md> |
| OpenCode | OpenCode documents `AGENTS.md` instruction files at the project root and at `~/.config/opencode/AGENTS.md`. It also documents Claude fallback files when OpenCode files are absent. Its rules page says OpenCode does not automatically parse file references in `AGENTS.md`. | OAW writes model-visible bootstrap text telling OpenCode to read the canonical policy. OpenCode has no documented automatic Markdown import in `AGENTS.md`, so OAW does not rely on one. | OpenCode searches local instructions and global instructions with its documented precedence. It also has documented `opencode.json` instruction entries, including globs and URLs, but OAW v0.1 intentionally uses `AGENTS.md` bootstrap text instead of provider config mutation. | <https://opencode.ai/docs/rules/> |
| Cursor | Cursor Project Rules are `.mdc` files under `.cursor/rules`. Frontmatter controls `description`, `globs`, and `alwaysApply`; `alwaysApply: true` means the rule is always included. Cursor also documents project-root and nested `AGENTS.md`, but Project Rules require `.mdc`. | OAW creates one owned `.mdc` file with `alwaysApply: true`, `globs: "**/*"`, and bootstrap text that tells the agent to read the canonical policy. | Cursor merges Team, Project, and User Rules with documented precedence of Team, then Project, then User. Cursor's nested `AGENTS.md` support is official, but OAW does not use it because the `.mdc` rule surface is a stable project-rule target with explicit frontmatter. | <https://cursor.com/docs/rules> |
| Windsurf / Devin rules | Devin Desktop / Cascade documents workspace rules under `.devin/rules/*.md`, with `.windsurf/rules/*.md` as a fallback and legacy `.windsurfrules` still read. `.devin/` is preferred and takes precedence. Rule frontmatter can set `trigger: always_on`. | OAW creates one owned `.devin/rules/open-agent-workflow.md` file with `trigger: always_on` and bootstrap text. | Workspace rules are local to the project. Because `.devin/rules` is the preferred current surface, OAW does not write `.windsurf/rules` or legacy `.windsurfrules`. Restart or refresh the target app if it has already cached workspace rules. | <https://docs.devin.ai/desktop/cascade/memories> |
| Cline | Cline's primary project rules format is `.clinerules/`; Cline processes `.md` and `.txt` files in that directory. It also detects several compatibility files, including `AGENTS.md`. Workspace rules take precedence over global rules when conflicts exist. | OAW creates one owned `.clinerules/open-agent-workflow.md` file with the Activation Router. | Cline supports conditional rules with YAML `paths` frontmatter, but no frontmatter means the Router is always visible. It remains a lazy Router: ordinary engineering work stays Native Host behavior until explicit activation. | <https://docs.cline.bot/customization/cline-rules> |
| Roo Code | Roo Code's preferred workspace rule directory is `.roo/rules/`, with `.roorules` as fallback. Preferred mode-specific rules use `.roo/rules-<modeSlug>/`. Roo also documents workspace-root `AGENTS.md` / `AGENT.md`, but `.roo/rules/` is the preferred workspace rule surface. | OAW creates one owned `.roo/rules/open-agent-workflow.md` file with bootstrap text. | Roo loads global rules before workspace rules; workspace rules take precedence on conflicts. Rule directories are read recursively in alphabetical order, and symlinks have a documented depth limit. OAW writes a normal file and does not use Roo's `AGENTS.md` fallback. | <https://docs.roocode.com/features/custom-instructions> |
| GitHub Copilot | GitHub Copilot repository custom instructions support repository-wide `.github/copilot-instructions.md` and path-specific `.github/instructions/<name>.instructions.md` files with `applyTo` frontmatter. When both repository-wide and path-specific instructions match, both are used. GitHub and VS Code also document `AGENTS.md`, and VS Code marks nested `AGENTS.md` support as experimental. | OAW creates one owned `.github/instructions/open-agent-workflow.instructions.md` file with `applyTo: "**"` and bootstrap text. OAW does not use Copilot `AGENTS.md` behavior because nested Copilot AGENTS behavior is experimental and unused in OAW v0.1. | GitHub documents personal, repository, and organization instruction priority. OAW uses path-specific repository instructions so it can avoid mutating user or organization settings and avoid experimental nested `AGENTS.md` behavior. | <https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions>, <https://code.visualstudio.com/docs/agent-customization/custom-instructions> |

## Shared Destination Caveat

Codex and OpenCode both use project-root `AGENTS.md` in OAW project scope. OAW
therefore treats `AGENTS.md` as a shared managed-block destination and renders
one coherent Activation Router rather than two independent blocks. This
preserves unrelated project instructions while avoiding duplicate router text.

## Official Behavior vs OAW Choices

- Official provider behavior defines where each tool looks for instructions,
  whether a provider supports imports, and how its own precedence works.
- OAW choices define which supported provider surface is used, whether the
  destination is managed as a block or as an owned file, and how the canonical
  policy path is made visible to the model.
- Claude and Gemini do not emit `@` for the canonical policy. Their Router reads
  it only after explicit activation.
- Codex and OpenCode do not have documented automatic Markdown imports in their
  instruction Markdown files; all four use the same model-visible Activation
  Router semantics.
- Cursor requires `.mdc` for Project Rules. Although Cursor documents
  `AGENTS.md`, OAW uses `.cursor/rules/open-agent-workflow.mdc`.
- Windsurf / Devin documentation prefers `.devin/rules` as its current surface, so OAW uses that path rather
  than `.windsurf/rules` or `.windsurfrules`.
- Cline uses `.clinerules`, and Roo Code uses `.roo/rules`.
- GitHub Copilot path-specific instructions use `.github/instructions` with
  `applyTo` frontmatter. The experimental nested `AGENTS.md` behavior is not used
  by OAW.

## Sources and Uncertainty

Validated official sources:

- Claude Code memory: <https://code.claude.com/docs/en/memory>
- Codex CLI `AGENTS.md`: <https://developers.openai.com/codex/guides/agents-md>
- Gemini CLI configuration: <https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/reference/configuration.md>
- Gemini CLI memory import processor: <https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/reference/memport.md>
- OpenCode rules: <https://opencode.ai/docs/rules/>
- Cursor rules: <https://cursor.com/docs/rules>
- Devin Desktop / Cascade memories and rules: <https://docs.devin.ai/desktop/cascade/memories>
- Cline rules: <https://docs.cline.bot/customization/cline-rules>
- Roo Code custom instructions: <https://docs.roocode.com/features/custom-instructions>
- GitHub Copilot repository instructions: <https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions>
- VS Code custom instructions: <https://code.visualstudio.com/docs/agent-customization/custom-instructions>

Known uncertainty:

- Provider documentation changes frequently. The retrieval date above records
  the evidence snapshot used for OAW v0.1 documentation.
- OpenCode documents `opencode.json` `instructions`, including globs and remote
  URLs. This guide's "no documented Markdown import" statement is limited to
  automatic reference parsing inside `AGENTS.md`.
- Cursor and Copilot both document `AGENTS.md` behavior. OAW intentionally does
  not use those surfaces for these project-extension adapters in v0.1.
- Hot-reload behavior is not uniformly documented across providers. When a
  provider does not document live reload for rule files, assume a new session or
  application refresh is required after OAW changes adapter files.
