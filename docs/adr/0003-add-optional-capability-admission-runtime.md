# ADR 0003: Add an Optional Capability-Admission Runtime

## Status

Superseded by ADR 0009

ADR 0009 is the active decision; this record remains historical context.

## Context

OAW v0.1 distributes a provider-neutral Markdown policy through supported agent
instruction surfaces. The policy prevents competing engineering methodologies
only when an agent follows it correctly. It cannot persist an authoritative
task lock, admit exact Provider Capabilities, coordinate concurrent Executors,
or recover an uncertain external invocation.

Making every supported Host depend on a Runtime would discard the portability
that makes the Policy Plane useful. Keeping only the policy would leave OAW
unable to provide the stronger execution guarantees required for a dependable
multi-agent engineering control plane.

## Decision Drivers

- Preserve instruction-only support for Hosts with limited integration points.
- Provide truthful, testable enforcement where Host integration permits it.
- Keep workflow Providers independently installed and versioned.
- Avoid duplicating control semantics across CLI, Hook, Plugin, and MCP entrypoints.
- Do not replace Host sandbox, filesystem, Git, network, or credential authority.

## Considered Options

### Policy v2 Only

This preserves maximum portability and minimum implementation cost, but all
classification, locking, isolation, and ownership constraints remain soft.

### Separate Sidecar Runtime

This enables early experimentation, but creates two long-lived configuration,
state, versioning, and diagnostic models.

### Unified Optional Runtime

This preserves Policy-only operation while adding one shared Runtime Core for
Capability admission, execution-graph transitions, task state, and Host
integration.

### Mandatory Runtime Platform

This offers the strongest control but excludes instruction-only Hosts and
requires a much larger plugin, daemon, authentication, and deployment surface.

## Decision

OAW will have two execution planes backed by one product model:

- The **Policy Plane** remains the portable governance source for every Host.
- The optional **Runtime Plane** manages Engineering Runs, Provider resolution,
  Capability Grants, control transitions, authoritative XDG state, and recovery.

`oaw run` is the reference Runtime entrypoint. Native Host integrations call
the same versioned Runtime Protocol. A Host that cannot satisfy Runtime
requirements remains `instruction-only` and must not claim Runtime enforcement.
No Host is promoted beyond `instruction-only` until its current official
capabilities are audited, its Manifest is admitted through a built-in or
user-trusted integration record, and it passes the Adapter Conformance Suite.
Per-run Host frames may narrow the pinned Manifest but cannot self-enable
features.

Runtime admission controls Provider Capability invocation and lifecycle
transitions. Effective authority remains the intersection of user authority,
Host permissions, OAW limits, Provider limits, Profile limits, and current
control state.

## Consequences

### Positive

- OAW can provide durable and testable workflow coordination without losing its
  current cross-Host policy surface.
- CLI and native integrations share one state machine and protocol.
- Host capability gaps become explicit diagnostics rather than silent fallbacks.
- Providers remain external dependencies rather than OAW-loaded executable plugins.

### Negative

- OAW becomes responsible for a versioned Runtime Protocol, task-state format,
  recovery behavior, and Host Adapter conformance.
- Policy-only and Runtime-managed guarantees must be documented separately.
- A cooperating Host Adapter is required; OAW cannot stop a Host from bypassing
  the Runtime outside the protocol.

### Risks

- Users may assume Capability admission is an operating-system sandbox.
- Host integrations may overstate isolation or invocation guarantees.
- Policy and Runtime semantics may drift.

Mitigations include explicit Integration Levels, shared conformance fixtures,
reason-coded diagnostics, and documentation that lists non-guarantees.

## Related Decisions

- Extends ADR 0001 without changing provider neutrality.
- Extends ADR 0002 by adding authoritative XDG Runtime State.
- ADR 0004 selects the Runtime Core implementation language.
