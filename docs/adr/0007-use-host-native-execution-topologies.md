# ADR 0007: Use Host-Native Execution Topologies

## Status

Accepted; supersedes ADR 0006 and the Codex execution decisions in ADR 0005

## Context

ADR 0005 and ADR 0006 attempted to make Codex execution safe by separating
Provider discovery from a filtered invocation environment. The resulting
`oaw run --host codex` path launches `codex exec`, creates a private HOME,
stages the selected Skill, and removes or projects Host configuration.

Controlled dogfooding showed that this is the wrong boundary. The new process
does not inherit the interactive Host's complete MCP, Hook, Skill, Plugin,
model-provider, authentication, project-rule, sandbox, and approval
environment. Restoring selected pieces would require OAW to emulate Codex or
Claude Code internals and would remain incomplete.

The design also makes OAW the parent of the Agent Host. OAW is a workflow Policy
and Runtime service; Codex, Claude Code, and similar tools are the actual Hosts
that own Agent execution.

Users need context isolation for complex work, but context isolation does not
require capability-environment isolation. Hosts without a native Subagent API
must still be able to execute a Workflow in the current session.

## Decision Drivers

- Preserve the complete capability environment selected by the user.
- Isolate conversational context without replacing Host capabilities.
- Support Hosts with and without a native Subagent API.
- Keep topology under explicit user control when alternatives exist.
- Maintain Provider-neutral Profile compilation and Runtime state guarantees.
- Avoid Host-specific process emulation and configuration projection.
- Make security and authority claims match the actual enforcement boundary.

## Considered Options

### Option 1: Continue the filtered Runner model

OAW would keep launching model CLIs and expand the whitelist until more Host
configuration works.

This was rejected because every new MCP server, Hook, Plugin, authentication
method, model route, or Host feature requires more emulation. A filtered child
is not the Host environment the user selected.

### Option 2: Require native Subagents for every Workflow

OAW would eliminate the clean Runner but reject Workflow execution on Hosts
without a native API.

This was rejected because native Subagent availability is a Host capability,
not a prerequisite for lifecycle governance. It would block valid inline work
and remove the user's topology choice.

### Option 3: Host-native INLINE and NATIVE_SUBAGENT topologies

The active Host calls OAW for classification, compilation, Grants, state, and
evidence contracts. The Host executes either in the current Agent context or
through its native Subagent API. A native child inherits the parent Host
capability environment and isolates only conversation context.

This option was selected.

## Decision

OAW supports exactly two execution topologies:

1. `INLINE`, executed by the current Agent in the active Host session;
2. `NATIVE_SUBAGENT`, created only through the active Host's native Subagent
   API.

When both are eligible, OAW recommends a topology and the user selects it. When
the Host lacks a conforming native API, `INLINE` is the only eligible topology
and work continues. OAW never starts a clean Codex, Claude Code, or other model
process to simulate a child Agent.

The control direction is Host to OAW to Host-native execution. OAW returns a
Host-neutral Dispatch Packet and accepts normalized receipts. It does not own
model process invocation.

A native child must demonstrate:

- separate conversational context;
- semantic inheritance of the parent's model route, opaque authentication
  context, MCP servers, Hooks, Skills, Plugins, Host/project configuration,
  sandbox, and approval policy.

OAW Grants continue to constrain lifecycle ownership, effects, resources,
delegation, termination, transitions, and evidence. They are logical workflow
authority and are not described as an operating-system sandbox.

The `runner-managed`, `native-managed`, `isolated-executor`,
`native-invocation`, `main-agent-allowed`, and `isolated-required` contracts are
removed. Capability descriptors instead declare supported topologies and
topology-compatible Host bindings.

This is a hard cutover. `oaw run --host codex`, the Codex Runner, private HOME,
Skill staging, Host configuration filtering, model-provider projection, and
legacy schema readers are deleted without compatibility aliases or migration
shims. Pre-release Runtime state using the old contracts is reset.

## Consequences

### Positive

- MCP, Hooks, Skills, Plugins, third-party API routing, authentication, and
  Host policy remain available during execution.
- Context isolation uses the Host mechanism designed for it.
- Hosts without native Subagents remain fully usable through inline Workflow
  execution.
- OAW returns to its intended role as a Provider-neutral Policy and Runtime
  plane.
- Grant and sandbox responsibilities become explicit and testable.
- New engineering Providers and user-defined Profiles do not require Runner
  changes.

### Negative

- OAW cannot offer native context isolation on a Host that lacks a native
  Subagent API or inheritance evidence.
- Inline review cannot claim fresh independent context.
- Host integrations must expose dynamic session and capability inheritance
  evidence.
- Existing Runner code, conformance assets, tests, documentation, and Runtime
  state must be replaced in one coordinated cutover.

### Risks and Mitigations

- A Host may claim inheritance that it does not provide. Mitigation: separate
  static adapter conformance from dynamic parent/child session attestation.
- The inherited Host environment may be broader than a Capability Grant.
  Mitigation: describe the Grant as workflow authority, retain Host sandbox and
  approvals as the physical boundary, and never overstate containment.
- A Host API may change. Mitigation: version Host-native adapters, invalidate
  native eligibility, and offer a user-approved stable switch to inline rather
  than launching a replacement process.

## Related Decisions

- ADR 0001 established Provider-neutral arbitration.
- ADR 0003 introduced the optional Runtime Plane.
- ADR 0004 selected Go for Runtime Core implementation.
- ADR 0005 documented the failed process-containment premise for Codex MCP.
- ADR 0006 is superseded by this Host-native control model.
- The detailed contract is defined in
  `docs/superpowers/specs/2026-08-04-oaw-host-native-execution-topology-design.md`.
