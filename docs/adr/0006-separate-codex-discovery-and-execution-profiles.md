# ADR 0006: Separate Codex Host Discovery from Invocation Execution Profiles

## Status

Superseded by ADR 0007

## Context

ADR 0005 established that Codex `--sandbox read-only` does not contain MCP
subprocesses and that OAW must fail closed when containment is unproven.
Controlled dogfooding then showed that plugin-provided MCP configuration cannot
always be disabled with invocation-local `-c` overrides. In particular, a
read-only Grant still allowed the Serena MCP server to create project metadata.

OAW must continue to discover Provider installations from the user's real Host
environment. Using that same interactive environment for execution, however,
also imports unrelated plugins, skills, rules, hooks, and MCP servers that are
outside the selected Capability Grant.

## Decision

Separate the Codex integration into two authority surfaces:

1. The **discovery surface** reads the real Host installation and produces a
   Host-scoped Registry and Binding Inventory.
2. The **execution surface** revalidates the granted Provider Instance,
   Capability, Installation, Binding, inventory digest, and physical evidence
   digest during `Prepare`.
3. Each invocation receives a private `0700` HOME and neutral workspace under
   the Runtime state root. Only the exact verified skill Binding is mapped into
   that HOME.
4. `codex exec` uses `--ignore-user-config`, `--ignore-rules`, and
   `--disable hooks`. The original `CODEX_HOME` remains available only for
   Codex authentication, and the physical project root is exposed with
   `--add-dir` under the Grant-derived sandbox mode.
5. Agent and tool Bindings fail with `CODEX_BINDING_KIND_UNSUPPORTED` until the
   execution profile can map their semantics exactly. Changed or incomplete
   evidence fails before the model process starts.

The invocation profile, rather than the interactive MCP inventory, is the
containment boundary used by `oaw run --host codex`. This supersedes ADR 0005's
MCP override procedure for the CLI execution path while preserving its
fail-closed security decision.

## Consequences

Provider discovery remains dynamic and Host-scoped, but discovery does not
implicitly authorize every installed Host extension during execution. User
configuration, project rules, hooks, ungranted skills, plugins, and MCP servers
are excluded from the invocation profile without mutating the user's Codex
installation.

The Runtime currently executes only physical skill Bindings through this
profile. Supporting agent or tool Bindings requires a later design that can
reproduce their exact Host registration and containment semantics. The Codex
shell sandbox remains a defense-in-depth control; it is not treated as an MCP
process sandbox.
