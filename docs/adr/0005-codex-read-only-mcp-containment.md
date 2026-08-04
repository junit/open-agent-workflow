# ADR 0005: Fail Closed When Codex Read-Only MCP Containment Is Unproven

## Status

Accepted

## Context

The Codex `--sandbox read-only` option constrains model-generated shell
commands, but it does not necessarily constrain MCP subprocesses. Controlled
dogfooding observed the Serena MCP server creating `.serena/` project metadata
inside a Grant that contained only `read-project`.

Treating the CLI sandbox flag as the complete Resource Lease boundary would
therefore grant a false safety property. Editing the user's Codex configuration
or silently disabling installed Providers would also violate Host ownership.

## Decision

For a Codex dispatch without `write-project` or `git-local`:

1. Run `codex mcp list --json` before dispatch preparation.
2. Normalize enabled server names and build invocation-local `enabled=false`
   overrides without writing user configuration.
3. Probe the effective MCP inventory with those overrides and require zero
   enabled servers.
4. Repeat the probe immediately before `codex exec` to detect configuration
   drift.

If inventory cannot be parsed, a server name cannot be represented safely, the
Codex configuration rejects an override, or any server remains enabled, return
`CODEX_MCP_INVENTORY_FAILED` or `CODEX_MCP_ISOLATION_FAILED` before
`DISPATCH_PREPARED`. After authorization, a failed pre-exec probe pauses the
Run as `EXECUTION_UNCERTAIN` and does not start the model process.

This is a Host precondition and fail-closed admission rule. It is not a claim
that OAW replaces the operating system sandbox or controls arbitrary software
outside the admitted Host invocation.

## Consequences

Read-only Runtime dispatches cannot run against a Codex installation whose
plugin-provided MCP servers lack a safe per-invocation control surface. A
future Host execution profile may provide plugin-scoped MCP controls or a
whole-process OS sandbox, but OAW must not guess or mutate the user's
interactive configuration to obtain one.

The same Runtime Grant and idempotent replay semantics remain unchanged for
write-capable dispatches and stale authorized invocations.
