# ADR 0008: Treat the Subagent Environment as Host-Owned

## Status

Accepted; amends the capability-environment decision in ADR 0007

## Context

ADR 0007 correctly moved execution control from an OAW-launched model process
to the active Agent Host and introduced `INLINE` and `NATIVE_SUBAGENT`.
However, it also required a native child to inherit the parent's complete MCP,
Hook, Skill, Plugin, model-route, authentication, project configuration,
sandbox, and approval environment.

That requirement assigns OAW authority it does not have. Codex and Claude Code
own child creation and child configuration. OAW cannot force a Host to inherit
a setting, inspect private credentials, or prove behavior that the Host does
not expose.

Current Host contracts also differ:

- Codex custom agents inherit some omitted session settings, including MCP and
  Skills configuration, while custom agent files and spawn settings can
  override the model and other supported values. Parent live sandbox and
  approval overrides are reapplied by Codex.
- Claude Code subagents can independently configure tools, permission mode,
  Skills, MCP servers, Hooks, model, and effort. Plugin subagents ignore some
  fields, and built-in Explore and Plan agents omit some parent context.

The fact that a native child exists therefore proves context separation, not
complete capability-environment equality.

## Decision Drivers

- Keep Agent execution under the active Host.
- Avoid false claims about capabilities OAW cannot control or observe.
- Preserve explicit user topology selection.
- Let Profiles require the Host surfaces they actually need.
- Keep `NATIVE_SUBAGENT` usable when optional Host surfaces are unknown.
- Allow a user to demand strict full-parent behavior when a Host can prove it.
- Never rebuild the Host environment inside OAW.

## Considered Options

### Option 1: Keep complete inheritance as a universal requirement

This was rejected because OAW cannot enforce it, current Hosts allow per-child
overrides, and several surfaces have no portable attestation API. It would
either create a false guarantee or make native execution unavailable on nearly
every Host.

### Option 2: Ignore the child environment entirely

This was rejected because a Profile may genuinely require a Provider Skill,
MCP server, project instruction, or security policy. Starting a child without
checking known requirements can produce incorrect execution.

### Option 3: Use Host reports and Profile-specific requirements

OAW declares the environment surfaces required by the selected Profile. The
Host reports each requested surface as inherited, host-configured, restricted,
unknown, or unavailable. OAW validates hard requirements and records every
unknown rather than converting it into a guarantee.

This option was selected.

## Decision

`INLINE` and `NATIVE_SUBAGENT` remain the only execution topologies.

`INLINE` necessarily uses the current Agent environment. `NATIVE_SUBAGENT`
uses the active Host's native child-environment semantics. OAW does not inject,
stage, filter, or reconstruct model routes, credentials, MCP servers, Hooks,
Skills, Plugins, sandbox rules, or approval settings.

Profiles declare required environment surfaces and accepted dispositions. A
Host-native adapter may report a surface as:

- `inherited`;
- `host-configured`;
- `restricted`;
- `unknown`;
- `unavailable`.

Hard Profile requirements must have an accepted Host report. Optional unknown
surfaces remain visible in the Lifecycle Bundle and execution receipts but do
not block native execution.

A user may select a strict `full-parent` requirement. Under that requirement,
any unknown, restricted, or unavailable surface makes `NATIVE_SUBAGENT`
ineligible for the selected Profile. This is a requirement on the Host, not a
capability created by OAW.

OAW may compare opaque Host IDs and digests, but it never stores credentials or
private extension configuration. A successful child launch is not evidence of
unreported inheritance.

## Consequences

### Positive

- OAW's guarantees now match its actual authority.
- Native Subagents remain available under Host-native behavior.
- Profiles can block execution when a genuinely required capability is absent.
- Users can request strict full-parent semantics without making them a false
  global default.
- Inline execution remains the exact-current-environment option.

### Negative

- Native child environments can differ across Hosts and agent roles.
- Some surfaces will remain `unknown` until Hosts expose better reporting.
- A strict `full-parent` Profile may be inline-only on current Hosts.
- OAW cannot promise that optional MCP, Hooks, Plugins, or credentials are
  present in a child.

### Risks and Mitigations

- A Host report may be incomplete. Mitigation: treat absent evidence as
  `unknown`, not inherited.
- A child may receive broader physical authority than the Grant. Mitigation:
  keep the Host sandbox and approvals as the physical boundary and the Grant as
  workflow authority.
- Host behavior may change by version. Mitigation: include Host and integration
  version in reports and invalidate stale reports at Bundle boundaries.

## References

- [Codex subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents)
- [Claude Code custom subagents](https://code.claude.com/docs/en/sub-agents)
- `docs/superpowers/specs/2026-08-04-oaw-host-native-execution-topology-design.md`
