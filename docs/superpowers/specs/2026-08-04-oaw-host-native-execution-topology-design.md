# OAW Host-Native Execution Topology Design

**Date:** 2026-08-04
**Status:** Written design pending user review
**Lifecycle:** MATT-SP-HYBRID
**Execution:** INLINE
**Scope:** Host control inversion, execution topology, capability inheritance,
Provider discovery, Runtime dispatch, and hard cutover from the Codex Runner

## 1. Summary

OAW must coordinate engineering work inside the Agent Host that the user is
already using. It must not launch a second Codex, Claude Code, or equivalent
process and attempt to reconstruct that Host from a filtered environment.

The execution model has exactly two topologies:

- `INLINE`: the current Host Agent executes the selected lifecycle in the
  current conversation.
- `NATIVE_SUBAGENT`: the current Host uses its native Subagent API to create a
  child context inside the same Host environment.

Both topologies use the complete capability environment of the active Host.
`NATIVE_SUBAGENT` isolates conversational context only. It does not replace or
filter the Host's model route, authentication, MCP servers, Hooks, Skills,
Plugins, project configuration, sandbox, or approval policy.

```text
User
  |
  v
Active Host Session (Codex, Claude Code, or another Host)
  |
  +--> OAW classification, Profile compilation, Bundle, Grant, state
  |
  +--> INLINE -----------------------------------+
  |     current Agent executes                  |
  |                                             v
  +--> NATIVE_SUBAGENT --> Host-native child --> Evidence
                              context only          |
                                                     v
                                              OAW state transition
```

The control direction is therefore:

```text
Host -> OAW policy and Runtime services -> Host-native execution
```

The following direction is forbidden:

```text
OAW -> launch a clean Host process -> simulate the user's Host environment
```

This is a hard architectural cutover. The existing `runner-managed` Codex
execution path, private HOME, Skill staging, configuration projection, and
`isolated-required` Workflow rule are removed. No compatibility shim or legacy
fallback is retained.

## 2. Why the Current Model Is Incorrect

The current implementation treats OAW as an Agent process supervisor:

1. `oaw run --host codex` constructs a Codex Runner and starts `codex exec`.
2. The Runner creates an invocation-local execution profile and private HOME.
3. It disables or filters Host configuration to approximate a narrow Grant.
4. Workflow admission requires an `isolated` Executor.
5. Host conformance describes the resulting subprocess as an
   `isolated-executor`.

That model confuses three different concerns:

- conversational context isolation;
- engineering workflow ownership;
- operating-system capability containment.

The subprocess can isolate conversation history, but it cannot reproduce the
interactive Host that the user selected. Filtering the environment removes or
changes MCP servers, Hooks, Skills, Plugins, third-party model routing,
authentication behavior, Host rules, and project configuration. Reintroducing
those capabilities one by one creates an incomplete Host emulator and makes
OAW responsible for private Host implementation details.

The model also inverts authority. Codex and Claude Code are independent Agent
Hosts. OAW is a workflow Policy and Runtime service used by those Hosts; it is
not a replacement Host and must not bootstrap one.

## 3. Architectural Invariants

The redesign is governed by these invariants:

1. **Host control:** The active Host owns Agent creation, model invocation,
   authentication, tools, sandboxing, approval, and extension loading.
2. **Two topologies only:** Execution is either `INLINE` or
   `NATIVE_SUBAGENT`.
3. **No process emulation:** OAW never launches a model CLI to simulate a
   missing Subagent API.
4. **Context-only isolation:** A native child receives a new conversational
   context, not a filtered capability environment.
5. **Complete capability inheritance:** A native child sees the same
   user-visible Host capabilities and policy baseline as its parent.
6. **User choice:** When both topologies are eligible, OAW recommends one and
   the user selects one.
7. **Inline continuity:** When the Host has no conforming native Subagent API,
   OAW records `INLINE` as the only eligible topology and work continues.
8. **Provider neutrality:** Superpowers, Matt, ECC, and third-party engineering
   Providers use the same discovery, Capability, and Profile contracts.
9. **Logical authority:** A Grant controls lifecycle ownership, effects,
   resources, transitions, and evidence. It is not an OS sandbox.
10. **No false guarantees:** OAW reports only Host capabilities that the
    current Host session can demonstrate.
11. **No legacy compatibility:** Changed schemas and state are replaced, not
    translated or dual-read.

## 4. Request Mode and Execution Topology

Request Mode and Execution Topology answer different questions:

| Axis | Question | Values |
| --- | --- | --- |
| Request Mode | How much workflow governance is required? | `DIRECT`, `BOUNDED`, `WORKFLOW` |
| Execution Topology | Where does the admitted work execute? | `INLINE`, `NATIVE_SUBAGENT` |

The fixed behavior is:

| Request Mode | Topology behavior |
| --- | --- |
| `DIRECT` | Always `INLINE`; no Lifecycle Bundle or Runtime Grant. |
| `BOUNDED` | `INLINE` unless the user explicitly requests an eligible native Capability invocation. It never starts the Workflow Startup Gate. |
| `WORKFLOW` | The Lifecycle Bundle records one topology. When both are eligible, topology selection is part of the Startup Gate. |

Selecting a Profile and selecting a topology remain independent. A user can
run the same eligible Profile inline or in a native child. A Provider brand
does not force a topology.

## 5. Workflow Startup Gate

For `WORKFLOW`, the Startup Gate presents two related decisions in one
interaction:

1. every eligible built-in and user-defined Profile Recipe;
2. every topology eligible for the current Host and selected Recipe;
3. one Profile recommendation and one topology recommendation;
4. the concrete reason for each recommendation;
5. every bounded add-on.

Rules:

- The user must select the Profile.
- If `INLINE` and `NATIVE_SUBAGENT` are both eligible, the user must select the
  topology. There is no silent default.
- If only `INLINE` is eligible, OAW reports why, records
  `selection_source = "host-only-option"`, and continues after the Profile is
  selected. It does not ask the user to configure a separate process.
- An explicit request for `NATIVE_SUBAGENT` cannot be satisfied by `codex
  exec`, `claude` CLI, a shell subprocess, a container, or another clean
  process. The result is `NATIVE_SUBAGENT_UNAVAILABLE`, followed by the valid
  inline path.
- Topology is locked to the Bundle generation. Only the user may switch it,
  and only at an existing stable lifecycle boundary.

Recommendation policy is advisory:

- recommend `INLINE` for interactive, short, or context-dependent work;
- recommend `NATIVE_SUBAGENT` for long-lived, multi-ticket, context-heavy, or
  fresh-review work when inheritance conformance passes;
- never convert a recommendation into authority.

## 6. Execution Topologies

### 6.1 INLINE

`INLINE` means the current Agent performs admitted work in the active Host
conversation.

Properties:

- the current model route, authentication, MCP servers, Hooks, Skills,
  Plugins, Host rules, project rules, sandbox, and approval policy remain in
  force;
- OAW does not spawn a process or create another HOME;
- the active Lifecycle Bundle and Grant constrain workflow ownership and
  allowed Runtime transitions;
- Resource Leases still prevent conflicting project mutations;
- evidence is returned by the current Agent through the Runtime receipt
  contract;
- the receipt records `context_freshness = "shared"` and must not claim an
  independent review context.

Inline execution is a first-class Workflow topology, not a degraded error
mode.

### 6.2 NATIVE_SUBAGENT

`NATIVE_SUBAGENT` means the Host creates a child through its own Subagent API.
Examples include a Codex native child Agent or a Claude Code Task Agent. The
exact API belongs to the Host adapter.

The child receives:

- a new conversational history and working context;
- the exact Dispatch Packet for the active Bundle node;
- references to canonical artifacts and prior evidence required by that node;
- the same Host capability environment as the parent.

The child does not receive unrelated parent conversation history through OAW.
It may receive Host-level rules, project instructions, Provider installations,
and tool configuration because those belong to the inherited Host environment,
not to conversational state.

OAW neither starts nor configures the child process. The Host returns an opaque
native invocation handle used for pause, cancellation, follow-up, and evidence
correlation.

### 6.3 Native Executor Lifetime

The Lifecycle Bundle owns one logical Workflow Executor session. The Host may
map that session to one resumable child or to node-scoped native invocations,
depending on its native API. That mapping does not change topology.

- A resumable Host keeps the same native handle across compatible lifecycle
  nodes and follow-ups.
- A task-scoped Host may return a new native handle for a later node, but the
  Dispatch Packet must carry the same Bundle generation and the canonical
  artifact and evidence references required by that node.
- A fresh review child is a node-scoped native invocation recorded under the
  same logical Workflow Executor session.
- OAW Runtime State and canonical artifacts are authoritative. Correctness
  must not depend on hidden conversational memory in a particular child.

Under native execution, the Main Agent retains user communication, selection,
and lifecycle coordination. It receives normalized receipts and artifact
references rather than accumulating the child's private working conversation.

## 7. Capability Environment Inheritance

Native topology is eligible only when the current Host session can attest that
the child inherits the parent's user-visible capability environment.

The inheritance comparison covers:

| Capability surface | Required relationship |
| --- | --- |
| Model route | Same model and model-provider route selected by the Host. |
| Authentication | Same opaque Host authentication context; credentials never enter OAW records. |
| MCP | Same enabled server identities and effective configuration digests. |
| Hooks | Same registered Hook identities and event behavior exposed by the Host. |
| Skills | Same visible Skill inventory, including plugin-provided Skills. |
| Plugins | Same enabled Plugin identities and versions. |
| Host configuration | Same effective Host and project configuration snapshot. |
| Project context | Same project root and instructions, except an explicit Resource Lease may select a worktree. |
| Sandbox | Same Host sandbox policy. |
| Approval | Same approval policy and user-mediated approval channel. |

"Same" means semantic equality of the Host-visible identity and configuration,
not copying secret values into OAW. A Host may expose opaque identifiers or
digests. OAW stores no tokens, API keys, credential material, private Hook
payloads, or full MCP configuration.

A missing capability is still equal when both parent and child lack it. For
example, a Host without Hooks can conform if neither context exposes Hooks.

An integration that can create a child but cannot demonstrate inherited
capabilities is not eligible for `NATIVE_SUBAGENT`. It remains eligible for
`INLINE`.

## 8. Host Integration Contract

The current `instruction-only`, `runner-managed`, and `native-managed`
hierarchy is replaced. A Host integration has one of two control surfaces:

| Control surface | Contract |
| --- | --- |
| `policy` | The Host consumes OAW instructions and executes inline without Runtime enforcement claims. |
| `host-native` | The active Host calls OAW Runtime services and supplies verified inventory, dispatch, and receipt operations. |

A Host-native manifest declares:

- Host identity and integration version;
- supported execution topologies;
- exact Provider binding inventory support;
- Dispatch Packet and receipt protocol versions;
- pause, cancellation, deduplication, and evidence capabilities;
- native context isolation and capability inheritance support when
  `NATIVE_SUBAGENT` is declared.

`INLINE` is mandatory for a Host-native integration. `NATIVE_SUBAGENT` is
optional and session-dependent. A static manifest states what the adapter can
do; a session attestation states what is available now.

The obsolete features `runner-managed`, `isolated-executor`, and
`native-invocation` are removed. Native child creation is represented directly
by the `NATIVE_SUBAGENT` topology and its two conformance properties:
`context-isolation` and `capability-environment-inheritance`.

## 9. Host Session Snapshot and Conformance

At Workflow compilation, the Host adapter supplies a bounded, secret-free
session snapshot:

```go
type HostSessionSnapshot struct {
    HostID                    string
    IntegrationID             string
    SessionID                 string
    SupportedTopologies       []ExecutionTopology
    ProviderInventoryDigest   string
    CapabilityEnvironmentHash string
    SandboxPolicyDigest       string
    ApprovalPolicyDigest      string
    Digest                    string
}
```

Before the Startup Gate, the Host session attests native API availability and
its inheritance contract without performing engineering work. For an actual
native dispatch, the Host returns a child attestation containing the parent
session digest, native invocation handle, child capability environment hash,
and context isolation evidence. OAW compares the child attestation before
marking dispatch started or releasing task input to the child.

Static adapter conformance and dynamic session conformance are separate:

- static conformance proves protocol behavior with deterministic fixtures;
- dynamic conformance proves that this session exposes the required native API
  and inherited environment;
- documentation or a CLI's general feature list is not dynamic evidence.

If the Host capability environment changes after Bundle compilation, new
dispatch is blocked with `HOST_SESSION_CHANGED`. The user may refresh discovery
and compile a new Bundle generation at a stable boundary.

## 10. Dispatch and Receipt Flow

OAW remains the authoritative state machine, but the Host owns execution.

```text
Host Agent
  -> OAW: classify / compile / issue stage Grant
  <- OAW: Dispatch Packet
  -> Host: execute inline OR invoke native Subagent API
  -> OAW: started receipt
  -> OAW: evidence and terminal receipt
  <- OAW: committed next state
```

The new Dispatch Packet is Host-neutral:

```go
type DispatchPacket struct {
    SchemaVersion       string
    RunID               string
    BundleID            string
    BundleGeneration    uint64
    BundleDigest        string
    NodeID              string
    Topology            ExecutionTopology
    HostSessionDigest   string
    Grant               CapabilityGrant
    ProviderID          string
    ProviderInstanceID  string
    CapabilityID        string
    Binding             HostBinding
    InputReferences     []ArtifactReference
    TerminationCondition string
    EvidenceRequirements []EvidenceRequirement
    Digest              string
}
```

The packet contains references and digests, not credentials or a reconstructed
Host configuration.

The Host returns normalized receipts:

- `DISPATCH_STARTED`: logical dispatch ID, topology, Host invocation handle,
  session digest, and context freshness;
- `DISPATCH_PAUSED`: pause reason and resumable Host handle;
- `DISPATCH_COMPLETED`: outcome and digest-pinned evidence references;
- `DISPATCH_FAILED`: typed failure and bounded diagnostic evidence;
- `DISPATCH_CANCELLED`: cancellation acknowledgement.

The Runtime continues to provide idempotency, state revision checks, Resource
Leases, evidence closure, and stable transitions. It no longer calls a Host
Driver that launches a model process.

The `oaw runtime` exchange may remain as a transport for state operations.
`oaw run --host codex` and all equivalent Host-launching entrypoints are
removed. There is no compatibility alias.

## 11. Provider and Capability Model

Superpowers, Matt, ECC, and future engineering systems remain equal Providers.
OAW ships contracts and built-in Profile Recipes, not assumed installations.

Provider resolution follows this chain:

```text
Provider Descriptor
  -> Host-scoped installation discovery
  -> Host-native binding inventory
  -> verified Provider Instance
  -> verified Capability binding for selected topology
  -> Profile compilation
```

Built-in Provider handling is dynamic:

1. OAW ships descriptors for `oaw/superpowers`, `oaw/matt`, and `oaw/ecc`.
2. Each descriptor declares known Host installation surfaces and Capability
   contracts.
3. The current Host adapter observes its real Skill, Agent, Tool, and Plugin
   inventories.
4. A built-in Provider becomes available only when its current Host
   installation and exact Capability binding verify.
5. Codex and Claude Code installations are independent Provider Instances even
   when they reference the same physical distribution.
6. The verified inventory digest is pinned in the Lifecycle Bundle.

OAW does not copy a selected Skill into a private HOME. Inline and native child
execution resolve the Skill through the active Host's own inventory.

### 11.1 Third-Party Providers

Users can register additional Provider descriptors and installation hints in
the OAW configuration. A registration supplies trusted metadata and discovery
instructions; it cannot forge an installed Capability.

```toml
schema_version = "oaw.user-config/v3"

[[provider_sources]]
descriptor = "/absolute/path/to/provider.toml"

[[provider_installations]]
provider_id = "acme/engineering"
host_id = "codex"
surface_id = "codex-user-skill"
location = "/absolute/path/to/installation"
```

The Host must still observe the declared installation and binding. Missing,
ambiguous, or changed evidence fails closed.

### 11.2 Capability Topology Contract

The singular `executor_topology = "main-agent-allowed" | "isolated-required"`
field is removed. A Capability declares supported topologies, and each Host
binding declares where it is valid:

```json
{
  "id": "implementation",
  "supported_topologies": ["INLINE", "NATIVE_SUBAGENT"],
  "host_bindings": [
    {
      "host": "codex",
      "kind": "skill",
      "reference": "acme:implementation",
      "topologies": ["INLINE", "NATIVE_SUBAGENT"]
    }
  ]
}
```

A Host-native `agent` binding may support only `NATIVE_SUBAGENT`. Built-in
workflow Providers must expose an inline-capable binding so a missing native
API never blocks ordinary Workflow execution.

## 12. Profiles and Built-In Orchestration

Profile Recipes remain declarative and Provider-neutral. Users can define a
Profile from any verified engineering Providers through configuration.

A configured Profile must specify:

- one exact Provider Capability owner for every required responsibility;
- transitions and terminal gates;
- incident routes and bounded add-ons;
- maximum effects and resources;
- any topology limitations derived from its selected Capability bindings.

The compiler intersects:

```text
Host session topologies
  AND Profile node Capability topologies
  AND verified Host binding topologies
```

The result is the Profile's eligible topology set for this session.

OAW ships these canonical built-in Recipes:

| Selection | Meaning |
| --- | --- |
| `SP-FULL` | Superpowers owns a complete delivery lifecycle. |
| `MATT-FULL` | Matt owns a complete domain-engineering lifecycle. |
| `ECC-FULL` | ECC owns a complete engineering lifecycle. |
| `MATT-SP-HYBRID` | Matt owns specification, domain analysis, TDD, and functional debugging; Superpowers owns implementation planning, execution orchestration, review, verification, and completion. |

`ECC-FULL` remains a complete independent Profile. ECC Capabilities may also
be used as bounded build, type, dependency, or security add-ons in other
Profiles.

Built-in Recipes use the same compiler and schema as user-defined Recipes.
There is no hard-coded Provider whitelist in admission or dispatch. Built-in
names are canonical selections, not compatibility aliases.

## 13. Lifecycle Bundle, Lock, and Grants

The Lifecycle Bundle records:

- Request Mode, Complexity, and Risk Class;
- selected Profile and selection source;
- selected topology and selection source;
- Host session and Provider inventory digests;
- exact Provider Instances and Capability bindings;
- stage ownership, transitions, add-ons, and stable boundaries;
- topology eligibility evidence;
- canonical artifacts and evidence requirements.

The Lifecycle Lock additionally records the active node, active ticket,
allowed and blocked actions, current Grant, Host invocation handle when one
exists, Resource Leases, context freshness, and evidence status.

A Capability Grant authorizes workflow behavior:

- the selected owner and binding;
- permitted effects and resources;
- allowed delegation targets;
- termination and evidence conditions;
- the selected topology and Host session.

It does not create filesystem, network, MCP, Hook, Plugin, or process
containment. The Host sandbox and approval system remain the physical authority
boundary. Documentation and errors must not describe a Grant as stronger than
the Host boundary.

## 14. Review and Delegation Semantics

The selected topology applies to the logical Workflow executor. Profile-owned
delegation remains explicit and Grant-controlled.

- Under `INLINE`, review can run in the current context and must record shared
  context. It cannot claim fresh independent review.
- Under `NATIVE_SUBAGENT`, a Profile can request a fresh native child for review
  when the Host supports it. The receipt records the fresh context evidence.
- A specialist Agent invoked by a stage is allowed only when the active
  Capability's delegation allow-list and effects permit it.
- OAW never implements delegation by starting another model CLI.

Fresh review improves evidence quality but is not a hidden requirement that
makes inline execution impossible.

## 15. Failure and Recovery Rules

The topology redesign uses explicit failures:

| Code | Meaning | Recovery |
| --- | --- | --- |
| `TOPOLOGY_SELECTION_REQUIRED` | Both topologies are eligible and no user choice exists. | Ask the user once at the Startup Gate. |
| `NATIVE_SUBAGENT_UNAVAILABLE` | Before selection, the Host has no conforming native API in this session. | Remove native eligibility and continue with inline as the only valid topology. |
| `HOST_CAPABILITY_INHERITANCE_UNVERIFIED` | An actual child does not satisfy the selected topology's inheritance contract. | Cancel before task input, keep the Run at the pre-dispatch stable boundary, and ask the user to switch topology. |
| `HOST_SESSION_CHANGED` | The Host capability snapshot changed after Bundle compilation. | Refresh and compile a new generation at a stable boundary. |
| `TOPOLOGY_BINDING_UNAVAILABLE` | A required Capability has no verified binding for the selected topology. | Select another topology or Profile. |
| `PROVIDER_BINDING_CHANGED` | An installed Provider or exact binding no longer matches the Bundle. | Refresh Provider discovery and recompile. |

`HOST_ISOLATION_UNAVAILABLE` is removed because physical isolation is no longer
the Workflow admission criterion.

Failures before a Host invocation create no started receipt. Failures after a
native invocation begins use its opaque handle for cancellation and require a
terminal receipt before the Resource Lease is released.

There is no silent topology fallback after Bundle selection. Automatic inline
continuity applies only when native execution was excluded before the Startup
Gate; an active Bundle changes topology only through the normal user-approved
stable-switch procedure.

## 16. Security and Trust Boundaries

This redesign intentionally preserves the user's selected Host environment.
It does not narrow extensions to the selected Provider.

The trust boundaries are:

1. the Host owns process, credential, sandbox, approval, MCP, Hook, Plugin, and
   model security;
2. OAW owns workflow selection, Provider binding identity, lifecycle
   ownership, state transitions, leases, and evidence integrity;
3. Provider descriptors and user configuration are declarations that require
   Host evidence;
4. Dispatch Packets contain only bounded references and digests;
5. no credential or raw private Host configuration is copied into Runtime
   State, project projections, logs, or evidence.

An OAW Grant may be narrower than the Host's physical authority, but OAW must
describe that difference honestly. Instruction-level conformance is policy
coordination, not a security sandbox.

Security review remains a lifecycle constraint or bounded add-on, not an
execution topology.

## 17. Hard Cutover and Deletion Scope

The implementation performs a clean replacement. It does not maintain a dual
path.

Remove:

- the `oaw run --host codex` command and Codex process-launch loop;
- the `oaw/codex-runner` built-in integration and pinned Codex Runtime
  entrypoint;
- private HOME and neutral workspace creation used to simulate isolation;
- Skill staging and exact-single-Skill projection into a private HOME;
- `--ignore-user-config`, `--ignore-rules`, Hook disabling, Plugin/MCP
  filtering, and model-provider route projection performed by OAW;
- Codex JSONL process-output normalization that exists only for the Runner;
- `runner-managed`, `native-managed`, `isolated-executor`,
  `native-invocation`, `main-agent-allowed`, and `isolated-required` contracts;
- Workflow admission that rejects inline Executors;
- compatibility aliases, legacy schema decoders, and old Runtime state readers
  for the replaced contracts;
- documentation and tests that claim OAW provides physical executor isolation.

Retain and adapt:

- `DIRECT`, `BOUNDED`, and `WORKFLOW` classification;
- Startup Gate and user-selected Profiles;
- Provider-neutral Catalog, host-scoped dynamic discovery, Binding Inventory,
  Registry, and user configuration;
- Profile compilation, Lifecycle Bundle, single-owner graph, stable switching,
  and bounded add-ons;
- Runtime revisions, idempotency, Resource Leases, pause, cancellation,
  evidence closure, and project projections;
- the `oaw runtime` state exchange as an optional Host-to-OAW transport.

All changed contracts receive new schema versions. Old Host manifests,
Provider descriptors containing `executor_topology`, Capability Grants,
Dispatch records, and Runtime state are rejected as unsupported. Because the
project is pre-release, users reset development Runtime state rather than run a
migration. Historical ADRs remain as decision history and are marked
superseded where appropriate.

## 18. Verification Strategy

### 18.1 Contract Tests

Tests must prove:

- only `INLINE` and `NATIVE_SUBAGENT` decode;
- the Startup Gate requires a topology choice only when both are eligible;
- a Host without a native API continues inline without a process fallback;
- Profile compilation intersects Host, Capability, and Binding topology sets;
- built-in and user-defined Providers resolve through the same dynamic path;
- ECC-FULL compiles as a complete Profile when its Capabilities verify;
- Grants contain topology and Host session identity but no sandbox claim;
- old schemas and old Runtime state fail explicitly;
- no command path invokes `codex exec`, `claude`, or another model CLI.

### 18.2 Host Conformance Tests

For each Host-native adapter, verify:

- exact Host-scoped Provider inventory;
- inline execution uses the active Host session;
- native child context is separate from parent conversation history;
- parent and child capability environment digests match for model route,
  authentication context, MCP, Hooks, Skills, Plugins, configuration,
  sandbox, and approvals;
- Dispatch Packet identity and Bundle inheritance;
- pause, cancellation, idempotency, evidence return, and normalized receipts;
- no private HOME, staged Skill, or filtered Host configuration is created.

### 18.3 Platform Tests

Development occurs on macOS. Tests for other operating systems run through
Docker when the relevant behavior can be represented there. Host-native APIs
that cannot run in Docker are reported as unavailable and skipped; their
absence does not block unrelated implementation work or inline verification.

### 18.4 Dogfooding

Controlled dogfooding must cover:

1. Codex `INLINE` with the current third-party API route and installed MCP,
   Skills, and Plugins still visible;
2. Codex `NATIVE_SUBAGENT` only when its native API and inheritance attestation
   are available;
3. a Host without a native API continuing inline;
4. dynamic discovery of Superpowers, Matt, ECC, and one user-registered
   Provider;
5. a user-defined Profile compiled from verified installed Capabilities;
6. evidence return and stable topology switching.

## 19. Acceptance Criteria

The redesign is complete only when all of the following are true:

1. OAW has no code path that starts a Codex, Claude Code, or other model CLI.
2. `INLINE` is a fully admitted Workflow topology.
3. `NATIVE_SUBAGENT` is available only through a conforming Host-native API.
4. A native child inherits the complete parent Host capability environment and
   isolates conversational context only.
5. Users choose topology whenever both valid options exist.
6. Hosts without native Subagents continue inline without separate setup.
7. Built-in and third-party Providers are dynamically discovered per Host.
8. Users can register installed Provider sources and define custom Profiles in
   configuration.
9. SP-FULL, MATT-FULL, ECC-FULL, and MATT-SP-HYBRID use the same generic
   Profile compiler.
10. Lifecycle Grants constrain ownership and workflow actions without claiming
    OS isolation.
11. Old Runner, isolation, staging, projection, schemas, tests, and docs are
    removed rather than retained behind compatibility switches.
12. Runtime state, evidence, idempotency, and Resource Lease behavior remain
    deterministic under both topologies.

## 20. Implementation Boundary

This design is one architectural cutover, but implementation should be planned
in dependency order:

1. contracts and schemas;
2. topology selection and Bundle/Grant state;
3. Host session and conformance interfaces;
4. Host-owned Dispatch Packet flow;
5. dynamic Provider and Profile integration;
6. Runner deletion and CLI cutover;
7. policy, documentation, conformance, and dogfooding.

The implementation plan may split these into reviewable tickets. It must not
introduce an intermediate compatibility layer that allows old and new
execution models to coexist.
