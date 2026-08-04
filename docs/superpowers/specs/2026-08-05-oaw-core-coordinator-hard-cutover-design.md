# OAW Core and Workflow Coordinator Hard-Cutover Design

**Date:** 2026-08-05
**Status:** Written design pending user review
**Lifecycle:** MATT-SP-HYBRID
**Execution topology:** CURRENT
**Scope:** Product boundaries, execution topology, Host integration, workflow
coordination, contract replacement, deletion scope, and migration-free cutover

## 1. Summary

Open Agent Workflow (OAW) is not an Agent process supervisor. Its primary
product is a Provider-neutral engineering policy and compiler that turns a
request, a user selection, Host-observed Provider Capabilities, and a Profile
Recipe into one conflict-free Lifecycle Bundle.

OAW is split into three explicit ownership areas:

1. **OAW Core** is required. It owns policy distribution, request
   classification, Provider and Capability resolution, Profile compilation,
   Startup Gate decisions, and Lifecycle Bundle construction.
2. **Workflow Coordinator** is optional. It owns durable Workflow revisions,
   idempotency, Resource Leases, evidence indexing, pause and cancellation,
   and legal lifecycle transitions for cooperating clients.
3. **Agent Host** is external to OAW. Codex, Claude Code, or another Host owns
   the current Agent, Subagent creation, model execution, authentication, MCP,
   Hooks, Skills, Plugins, tools, sandboxing, and approvals.

The old execution Runtime is removed. OAW never starts a model CLI, constructs
a private HOME, stages a Skill, filters Host configuration, or emulates a
Subagent. The only execution topologies are:

- `CURRENT`: execute in the current Agent session;
- `SUBAGENT`: ask the active Host to create a child through its native
  Subagent facility.

`SUBAGENT` is Host-native by definition. There is no process fallback and no
compatibility alias for `INLINE`, `NATIVE_SUBAGENT`, or older executor models.

## 2. Product Definition

OAW's core capability is lifecycle contract compilation:

```text
Request evidence
  + user selection
  + Host session facts
  + verified Provider Capabilities
  + Profile Recipe
  -> conflict-free Lifecycle Bundle
```

The Bundle answers these questions:

- Which Request Mode applies?
- Is a lifecycle required?
- Which Profile did the user select?
- Who owns each engineering responsibility?
- Which bounded add-ons are admitted?
- Where does work execute?
- What evidence closes each stage?
- Where may the user switch the Profile or topology?

OAW does not answer how a model is invoked or how physical authority is
enforced. Those concerns remain with the Host.

## 3. Architectural Invariants

1. **Host control:** The active Host owns every Agent and tool invocation.
2. **No model process launch:** OAW never starts Codex, Claude Code, or another
   model CLI.
3. **Two topologies only:** `CURRENT` and `SUBAGENT` are the complete topology
   set.
4. **No fallback emulation:** Missing Subagent support leaves `CURRENT` as the
   valid path.
5. **Explicit user selection:** When both topologies are eligible for a
   Workflow, the user selects one in the Startup Gate.
6. **Host-owned child environment:** OAW records Host reports but never
   reconstructs or guarantees unreported MCP, Hook, Skill, Plugin, model,
   authentication, sandbox, or approval behavior.
7. **Provider neutrality:** Built-in and user-registered Providers use the same
   descriptor, discovery, verification, Capability, and Recipe contracts.
8. **One owner per responsibility:** Compilation rejects missing, ambiguous,
   or competing owners.
9. **Logical coordination only:** Grants and Resource Leases coordinate
   cooperating clients; they are not an operating-system security boundary.
10. **Optional persistence:** Policy-only use remains complete and honest
    without the Coordinator.
11. **No legacy contracts:** Replaced schemas, commands, state, and enum values
    are rejected rather than translated.

## 4. Module Architecture

```text
Independent engineering Providers
  Superpowers / Matt / ECC / user Providers
                     |
                     v
+--------------------------------------------------+
| OAW Distribution                                |
| installer + target adapters + canonical policy |
+--------------------------------------------------+
                     |
                     v
+--------------------------------------------------+
| OAW Core                                         |
| classification + discovery + registry + compiler|
| output: decision, choices, Lifecycle Bundle      |
+--------------------------------------------------+
                     |
          optional Workflow State commands
                     v
+--------------------------------------------------+
| Workflow Coordinator                             |
| revisions + leases + evidence + transitions      |
+--------------------------------------------------+
                     |
                Dispatch Packet
                     v
+--------------------------------------------------+
| Active Agent Host                                |
| CURRENT or native SUBAGENT execution             |
| tools + environment + sandbox + approvals        |
+--------------------------------------------------+
                     |
                   Receipt
                     +----------> Coordinator
```

### 4.1 OAW Distribution

This module retains the existing management behavior:

- install one canonical policy through target-native instruction surfaces;
- preserve user-owned content;
- detect drift and back up forced mutations;
- keep installation state separate from Workflow State.

Distribution does not discover execution authority and does not start an
engineering lifecycle.

### 4.2 OAW Core

OAW Core is a required, stateless module with a small interface. Its
implementation contains the complex policy decisions:

```go
type Core interface {
    Classify(ClassificationProposal) (ClassificationDecision, error)
    Resolve(HostInventorySnapshot) (ResolutionReport, error)
    Compile(CompilationRequest) (CompilationResult, error)
}
```

The records above are illustrative interface shapes, not a requirement to use
a Go interface type. Callers must not assemble Bundles or infer eligibility
themselves.

`Compile` returns:

- every eligible Profile and bounded add-on;
- topology eligibility and reason-coded exclusions;
- the recommendation without converting it into a default;
- the selected immutable Lifecycle Bundle only after explicit user choice.

Core accepts secret-free Host facts. It does not retain Host credentials,
private extension configuration, or conversational history.

Callers never construct a Lifecycle Bundle. In Policy-only use, the caller
receives the Bundle directly from Core. In coordinated use, a `START` command
carries request evidence, immutable Host snapshots, and the explicit user
selection; the Coordinator invokes Core and commits that exact compilation
result in one transaction. A caller-supplied Bundle is invalid input.

### 4.3 Workflow Coordinator

The Coordinator is an optional state module. It is not an execution Runtime.
Its one external seam is a deterministic exchange:

```go
type Coordinator interface {
    Exchange(WorkflowCommand) (WorkflowResult, error)
}
```

It owns only:

- immutable Workflow revisions and `HEAD`;
- idempotency-key replay and conflict rejection;
- active Lifecycle Bundle generation and digest;
- current graph node, ticket, Grant, and stable boundary;
- cooperative Resource Leases for conflicting project mutations;
- normalized receipt and evidence references;
- pause, cancellation, uncertain execution, and recovery state;
- validation of legal state transitions.

It does not own:

- classification, Provider discovery, or Profile compilation algorithms;
- Agent or Subagent creation;
- Skill invocation, tool use, filesystem calls, network access, or Git;
- physical sandboxing, authentication, approval, or credential storage;
- prevention of Host actions taken outside the protocol.

`DIRECT` and `BOUNDED` do not require Coordinator state. Version 1 of the
Coordinator accepts only `WORKFLOW` Bundles. A Bounded Capability remains one
atomic Host operation with no lifecycle lock or durable transition graph.

The Coordinator depends on Core rather than duplicating its decisions. `START`
is the only command that may create a Bundle, and it must compile the Bundle
inside the locked state transition. Later commands reference the committed
Bundle generation and digest; they cannot replace its ownership graph.

### 4.4 Host Integration

A Host integration has one of two surfaces:

| Surface | Meaning |
| --- | --- |
| `policy` | The Host consumes OAW instructions and cooperates without machine-enforced Workflow State. |
| `host-native` | The Host reports session facts, calls OAW Core or Coordinator, executes Dispatch Packets, and returns normalized Receipts. |

A host-native integration must support `CURRENT`. `SUBAGENT` is optional and
session-dependent. Static integration metadata cannot claim that a native
child exists in the current session.

The Host supplies a secret-free session snapshot containing:

- Host and integration identity and versions;
- current session identity;
- supported topologies;
- Provider binding inventory digest;
- supported receipt and evidence behavior;
- optional environment-surface observations;
- sandbox and approval policy digests when observable.

The Host creates a Subagent only after receiving a `SUBAGENT` Dispatch Packet.
OAW never calls a Host Driver that starts a model process.

## 5. Execution Topology Contract

The protocol enum is:

```text
ExecutionTopology = CURRENT | SUBAGENT
```

The user-facing labels are:

| Enum | English | Chinese |
| --- | --- | --- |
| `CURRENT` | Current session | 当前会话 |
| `SUBAGENT` | Subagent | 子 Agent |

### 5.1 CURRENT

`CURRENT` means the active Agent session performs the admitted work.

- The exact current Host environment remains in force.
- OAW creates no process, HOME, workspace, or projected configuration.
- Context freshness is `shared`.
- A review performed in this topology cannot claim an independent fresh
  context unless the Host separately creates one and records it.

### 5.2 SUBAGENT

`SUBAGENT` means the active Host creates a child using its native Subagent
facility.

- The child receives a new conversational context.
- OAW supplies only the Dispatch Packet and referenced artifacts.
- The Host decides model, authentication, tools, extensions, sandbox, and
  approvals.
- The Host reports required environment surfaces as `inherited`,
  `host-configured`, `restricted`, `unknown`, or `unavailable`.
- A Profile may reject dispositions that do not satisfy a hard requirement.
- A missing native facility returns `SUBAGENT_UNAVAILABLE`; work may continue
  under `CURRENT` only through the normal user selection rules.

`SUBAGENT` never means a shell child process, container, `codex exec`, Claude
CLI invocation, remote job, or OAW-managed clean environment.

## 6. Request Flows

### 6.1 DIRECT

```text
Request -> classify DIRECT -> CURRENT execution -> focused verification
```

There is no Capability selection, Profile, Bundle, Coordinator state, or
Startup Gate.

### 6.2 BOUNDED

```text
Request -> classify BOUNDED -> select one exact Capability
        -> validate Host binding -> CURRENT by default
        -> optional explicit SUBAGENT when eligible -> one terminal outcome
```

There is no Profile, lifecycle ownership, durable graph, or Startup Gate. A
second Capability or remediation loop escalates the request.

### 6.3 WORKFLOW

```text
Request
  -> classify WORKFLOW
  -> resolve current-Host Provider Instances
  -> compile eligible Profiles and topologies
  -> Startup Gate
  -> user selects Profile, topology, and add-ons
  -> Policy-only: Core compiles Lifecycle Bundle
     OR coordinated: Coordinator invokes Core and atomically commits Bundle
  -> issue Dispatch Packet for active node
  -> Host executes CURRENT or creates SUBAGENT
  -> Host returns Receipt and evidence references
  -> Coordinator commits the next legal revision, or Policy lock is updated
  -> repeat until the terminal gate closes
```

Policy-only execution follows the same ownership model but provides no claim
of atomic revisions, leases, idempotency, or transition enforcement.

## 7. Startup Gate

For `WORKFLOW`, one Gate presents:

1. eligible built-in and user-defined Profiles;
2. exact missing or ambiguous Capability diagnostics;
3. eligible topologies for the selected Profile and current Host session;
4. one Profile recommendation and one topology recommendation;
5. every proposed bounded add-on;
6. environment warnings relevant to `SUBAGENT`.

The user must select a Profile. When both topologies are eligible, the user
must also select a topology. When only `CURRENT` is eligible, OAW states why
and records `host-only-option`; it never proposes a clean-process workaround.

## 8. Provider and Profile Model

Superpowers, Matt, ECC, and user Providers remain independently installed.
OAW ships declarative descriptors and built-in Recipes, not Provider code.

Provider resolution is Host-scoped:

```text
Provider Descriptor
  -> physical installation evidence
  -> Host-observed binding inventory
  -> verified Provider Instance
  -> topology-compatible Capability binding
  -> Profile compilation
```

Each Capability declares `supported_topologies`. Each Host binding declares
the subset it supports. The compiler intersects:

```text
Host session topologies
  AND Capability supported topologies
  AND verified Host binding topologies
  AND Profile environment requirements
```

Built-in and user-defined Recipes use one compiler. `SP-FULL`, `MATT-FULL`,
`ECC-FULL`, and `MATT-SP-HYBRID` are canonical built-in selections.
`ECC-FULL` remains a complete engineering lifecycle.

## 9. Lifecycle Bundle and Dispatch

The Lifecycle Bundle records:

- classification, complexity, and risk;
- selected Profile and selection source;
- selected `CURRENT` or `SUBAGENT` topology and selection source;
- Host session and Provider inventory digests;
- exact Provider Instances and Capability bindings;
- single-owner graph, transitions, incident routes, and stable boundaries;
- bounded add-ons;
- environment requirements;
- artifact and evidence requirements.

The Host-neutral Dispatch Packet records:

- Run, Bundle generation, graph node, and ticket identity;
- selected topology and Host session digest;
- exact Provider and Capability binding;
- logical Grant, effects, resources, and termination condition;
- input artifact references and required evidence;
- environment requirements without credentials or private configuration.

The Host returns one normalized Receipt kind:

- `STARTED`;
- `PAUSED`;
- `COMPLETED`;
- `FAILED`;
- `CANCELLED`.

The Coordinator validates identity, revision, idempotency, evidence closure,
and transition legality. It does not validate or contain unreported physical
Host actions.

## 10. Public Command and Storage Cutover

Retain management, catalog, and Provider diagnostics commands. Replace the
old execution surfaces as follows:

| Remove | Replacement |
| --- | --- |
| `oaw run --host codex` | No replacement; the active Host executes work. |
| `oaw runtime exchange` | `oaw workflow exchange` for optional Workflow State. |
| Runtime State directory | Workflow State directory under the OAW XDG state root. |
| `oaw.runtime/*` schemas | New `oaw.workflow-*` command, result, revision, and snapshot schemas. |

The Go package `internal/runtime` is removed. Reusable state logic moves into
`internal/coordinator`; classification, resolution, and compilation stay in
their existing Core packages. There is no forwarding package or command alias.

## 11. Hard Deletion Scope

### 11.1 Delete Completely

- `internal/host/codex/`: Runner, private execution profile, process output
  parser, MCP filtering, and Runner-only inventory behavior;
- `internal/host/driver.go` and its tests: OAW-owned model dispatch;
- `internal/host/entrypoint.go` and its tests: pinned Codex Runtime entrypoint;
- `internal/cli/run_host_test.go` and every `runHostLoop` or `runCodexContext`
  path;
- the built-in `oaw/codex-runner` integration;
- Runner-only conformance fixtures and integration tests;
- Bounded and Direct persistence paths inside `internal/runtime`;
- old Runtime command parsing and Host-launch handshake;
- private HOME, neutral workspace, Skill staging, configuration filtering,
  model-route projection, and Codex JSONL normalization code;
- old schema readers, state readers, aliases, and migration code for replaced
  contracts.

### 11.2 Replace, Do Not Layer

- `internal/catalog.ExecutorTopology` and `executor_topology` become topology
  sets containing only `CURRENT` and `SUBAGENT`;
- Host `instruction-only`, `runner-managed`, and `native-managed` levels become
  `policy` and `host-native` surfaces;
- Host manifest, integration, conformance, Provider descriptor, user config,
  Profile Recipe, Grant, Dispatch, and Workflow State schemas receive new
  versions;
- `internal/runtime` Workflow state logic is extracted into
  `internal/coordinator` with Workflow-only inputs;
- `oaw runtime exchange` becomes `oaw workflow exchange` without an alias;
- canonical policy, README, architecture, lifecycle, adapter, security,
  troubleshooting, comparison, changelog, and English/Chinese documentation
  are rewritten to describe Core, Coordinator, and Host execution accurately.

### 11.3 Retain and Adapt

- installation management, transaction safety, adapters, and Install State;
- canonical JSON and strict schema validation;
- `DIRECT`, `BOUNDED`, and `WORKFLOW` classification;
- configuration trust, discovery, Registry resolution, and Host scoping;
- Profile Recipe compilation and built-in aliases;
- Workflow revisions, journal integrity, idempotency, Resource Leases,
  evidence references, projections, pause, cancellation, and recovery;
- Host conformance as protocol and session-report validation, not process
  isolation evidence;
- macOS native tests and Docker-based tests for representable non-macOS
  behavior.

### 11.4 Historical Material

Historical ADRs, completed implementation plans, and lifecycle evidence remain
as history. Superseded ADRs are marked explicitly. They are never parsed as
active configuration or compatibility contracts. Canonical product docs and
active code must contain no obsolete execution claim.

## 12. State and Schema Reset

This is a pre-release hard cutover:

- no old Runtime state migration;
- no dual-read or dual-write period;
- no legacy enum aliases;
- no compatibility command shims;
- no fallback to a model subprocess;
- no automatic adoption of Policy-only Markdown locks.

Pre-cutover Runtime state is deleted by the developer explicitly when testing
the new Coordinator. OAW never deletes an unknown state root automatically.
Old documents fail with stable `SCHEMA_UNSUPPORTED`; old state fails with
`WORKFLOW_STATE_UNSUPPORTED`.

## 13. Failure Semantics

- Missing or ambiguous Provider binding: fail compilation, show exact
  diagnostics, and do not substitute a Provider.
- Missing Subagent facility: mark `SUBAGENT` ineligible; keep `CURRENT`.
- Required child surface is `unknown`, `restricted`, or `unavailable`: reject
  `SUBAGENT` when the Profile does not accept that disposition.
- Host session digest changes: reject new dispatch and require recompilation at
  a stable boundary.
- Stale Workflow revision: reject without changing state.
- Duplicate idempotency key with identical content: return the committed
  result; different content: reject.
- Conflicting Resource Lease: reject the Workflow mutation before dispatch.
- Uncertain Host execution: pause and require reconciliation; never retry a
  mutation blindly.
- Invalid or incomplete evidence: keep the active node open.

## 14. Security and Trust

OAW owns declarative workflow authority and state integrity. The Host owns
physical authority. A Grant can be narrower than the Host sandbox, but this is
a policy violation surface rather than proof of containment.

The Coordinator accepts facts and selections, not caller-authored authority.
It invokes Core to compile the initial Bundle and verifies the committed
generation and digest on every later command. This protects OAW state from a
cooperating but malformed client; it still cannot stop a Host that ignores OAW
and acts outside the protocol.

OAW stores only opaque identities, digests, references, dispositions, and
bounded diagnostics. It stores no API keys, tokens, credential material,
private Hook payloads, or complete MCP and Plugin configuration.

Networked or destructive external actions continue to require Host approval
and user authority independently of the selected Profile.

## 15. Verification Strategy

### Contract Tests

- only `CURRENT` and `SUBAGENT` decode;
- old topology values and schemas fail explicitly;
- `SUBAGENT` always requires a Host-native session capability;
- Direct and Bounded requests create no Workflow Coordinator state;
- built-in and user Providers follow the same resolution and compiler path;
- each compiled responsibility has exactly one owner;
- Profile topology eligibility is the exact intersection of Host, Capability,
  binding, and environment requirements.

### Coordinator Tests

- deterministic commands and results;
- immutable revisions and atomic `HEAD` replacement;
- idempotent replay and conflicting-key rejection;
- Resource Lease acquisition, conflict, release, and recovery;
- pause, cancellation, evidence closure, and stable switching;
- crash-safe committed revision reads;
- no Agent, tool, or process invocation from Coordinator code.

### Host Conformance Tests

- `CURRENT` uses the active session;
- `SUBAGENT` uses only the Host-native facility;
- Host session and child environment reports are secret-free and validated;
- required Provider bindings are visible in the selected topology;
- Dispatch Packet and Receipt identity remain pinned to the Bundle;
- unavailable Host-native behavior is reported as skipped, never passed.

### Repository Gates

- no production source invokes `codex exec`, Claude CLI, or another model CLI;
- no canonical source or product documentation contains old topology or
  integration-level contracts;
- Go unit, integration, race-relevant, schema, management, and docs gates pass;
- Linux behavior runs in Docker on macOS when representable;
- unavailable WSL or Host-native tests return a documented skip and do not
  block unrelated progress.

## 16. Implementation Phases

The cutover is implemented in dependency order. No phase introduces a
user-visible dual path.

1. **Core contracts:** replace topology, Provider, Profile, Host session, and
   environment-report schemas and update compiler inputs.
2. **Host-native seam:** replace integration levels and process Driver records
   with session snapshots, Dispatch Packets, Receipts, and conformance rules.
3. **Coordinator extraction:** create `internal/coordinator`, move only
   Workflow state behavior, and expose `oaw workflow exchange`.
4. **Runner deletion:** remove Codex Runner, process profiles, output parser,
   pinned entrypoint, old CLI commands, and Runner-only tests.
5. **Policy and documentation cutover:** rewrite canonical policy and all
   product-facing English and Chinese docs; mark superseded ADRs.
6. **Conformance and dogfooding:** verify policy-only, `CURRENT`, optional
   `SUBAGENT`, third-party Providers, user Profiles, state recovery, macOS, and
   Docker-representable platform behavior.

Detailed implementation plans must split these phases into reviewable,
independently testable tickets while preserving one hard public cutover.

## 17. Acceptance Criteria

The redesign is complete only when:

1. OAW has no execution Runtime and no code path that starts a model CLI.
2. OAW Core performs classification, Provider resolution, Profile compilation,
   and Lifecycle Bundle construction without durable state.
3. Workflow Coordinator is optional and has no execution authority.
4. `CURRENT` and `SUBAGENT` are the only topology values everywhere.
5. `SUBAGENT` is created only by the active Host's native facility.
6. Hosts without Subagent support continue through `CURRENT`.
7. Policy-only Hosts remain supported without Coordinator guarantees.
8. Direct and Bounded work never create Workflow State.
9. Built-in and user-defined Providers and Profiles use the same generic path.
10. `SP-FULL`, `MATT-FULL`, `ECC-FULL`, and `MATT-SP-HYBRID` compile when their
    current-Host Capability requirements verify.
11. Coordinator revisions, leases, evidence, pause, cancellation, and recovery
    are deterministic for cooperating clients.
12. Old commands, schemas, state readers, Runner code, and compatibility aliases
    are absent.
13. Canonical policy and product docs make no physical execution or environment
    inheritance claim that OAW cannot enforce.
14. Native macOS gates pass; Docker-representable platform gates pass; truly
    unavailable Host-native or WSL checks are explicitly skipped.

## 18. Non-Goals

- building a universal Agent Host;
- wrapping model CLIs as Subagents;
- copying or merging user Host configuration;
- credential brokering;
- operating-system sandbox enforcement;
- remote Agent scheduling;
- automatic publication, push, or merge;
- preserving pre-release Runtime contracts or state.
