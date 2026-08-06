# OAW Codex Host Bridge Design

**Date:** 2026-08-07
**Status:** Approved for implementation planning
**Governance:** One-time bootstrap exception; no Lifecycle Bundle or
Coordinator guarantee is claimed for this design task
**Execution topology:** CURRENT
**Scope:** Codex host-native session evidence, Provider Skill binding
inventory, Core and Coordinator MCP access, Plugin packaging, diagnostics,
installation, and conformance

## 1. Summary

Open Agent Workflow (OAW) needs a real Codex Host Bridge before it can treat
Codex Provider candidates as verified Provider Instances. Filesystem discovery
can prove that a Provider distribution exists, but it cannot prove that an
exact Skill is enabled in the active Codex Host or bind that observation to the
current Codex session.

The Codex Host Bridge is a Codex Plugin containing a bundled MCP server and
trusted `PreToolUse` Hooks. Codex injects current-call identity into one
strictly read-only observation operation. The Bridge queries stable Codex App
Server metadata methods, constructs the existing OAW Host v2 records, and
keeps the resulting facts in an in-memory cache behind a session-bound opaque
handle. Later Core and Coordinator operations consume that handle without
allowing the Agent to author or replace Host facts.

The Bridge does not start an Agent, create or resume a Codex Thread, invoke a
model, emulate a Subagent, construct a private HOME, copy Host configuration,
or execute a lifecycle Capability. The current Codex session remains the
physical executor under `CURRENT`.

Codex Plugin installation and policy installation remain distinct. The policy
adapter distributes `ENGINEERING.md`; the explicit Bridge installation adds a
host-native integration that the user must enable and whose Hooks the user
must review and trust.

## 2. Context and Problem

The Core and Coordinator hard cutover correctly separated OAW's deterministic
policy and state responsibilities from Agent Host execution. It deleted the
old Runner, private execution HOME, staged Skills, filtered Host configuration,
and model-process launch path. Codex currently has only the built-in
`oaw/codex-policy` integration, so strict Provider inspection can discover
candidate distributions but has no Host-owned binding inventory.

The resulting diagnostics are accurate:

```text
Superpowers: candidate / HOST_BINDING_EVIDENCE_REQUIRED
Matt:        candidate / HOST_BINDING_EVIDENCE_REQUIRED
ECC:         not-found or unverified for the current Codex binding surface
Eligible Profiles: 0
```

The missing component is not another Runtime. It is a narrow Host integration
that can answer these questions without transferring Host authority to OAW:

1. Which Codex session and working directory issued this OAW call?
2. Which Skills does Codex report as enabled for that working directory?
3. Which exact discovered Provider installation owns each Skill path?
4. Which existing Profile Recipes compile from those verified bindings?
5. Can a cooperating current session exchange Core and Coordinator records
   without OAW executing the work?

Starting a separate App Server alone is insufficient session evidence. It can
report effective metadata for a working directory, but it does not prove that
an arbitrary caller belongs to the already-running conversation. A prompt or
Skill self-report is also insufficient because it is Agent-authored. The
Bridge therefore combines a Host Hook call context with stable metadata APIs.

## 3. Goals

1. Prove the current Codex session identity for each host-native OAW exchange.
2. Build exact Codex Skill binding observations from stable Host metadata.
3. Associate each observed Skill path with exactly one discovered Host
   Installation.
4. Reuse the existing Host Session, Binding Inventory, Environment Report,
   Core, and Coordinator contracts.
5. Keep `CURRENT` as the only v1 topology and preserve the active Codex
   environment unchanged.
6. Preserve Provider neutrality: Superpowers, Matt, ECC, and user Providers
   use the same Descriptor and matching algorithm.
7. Keep Host evidence out of Agent-authored request fields and durable private
   Bridge state out of Workflow State.
8. Produce precise, layered diagnostics for installation, Hook, session,
   observation, Provider, and Profile failures.
9. Install and remove the Bridge through explicit, auditable user actions.
10. Remove or reject superseded development-only Host evidence paths rather
    than maintaining compatibility aliases.

## 4. Non-Goals

Bridge v1 does not:

- support or emulate `SUBAGENT`;
- start Codex, `codex exec`, another App Agent, or any model process;
- own, create, resume, fork, or steer a Codex Thread;
- copy or reconstruct MCP, Hook, Skill, Plugin, model, authentication,
  sandbox, or approval configuration;
- use experimental `plugin/list` as a production dependency;
- treat Descriptor declarations or user configuration as Host observations;
- verify arbitrary Codex `agent` or `tool` bindings without a stable current-
  session inventory API;
- provide an operating-system authentication or isolation boundary against a
  malicious process running as the same user;
- make policy-only integrations host-native by inference;
- execute Skills or lifecycle Capabilities through the MCP server; or
- retain the old Runner, projected execution profile, private HOME, or legacy
  topology aliases.

## 5. Architectural Invariants

1. **Codex owns execution.** The active Codex session invokes every Skill and
   tool and performs every physical effect.
2. **One metadata process only.** A Bridge may start a metadata-only Codex App
   Server client process, but it never starts or controls a Thread or model.
3. **Host context is not Agent-authored.** Codex `PreToolUse` injects reserved
   context after the model produces the public tool input.
4. **Observation is read-only.** Only the strict observation operation may be
   automatically allowed to support Hook input rewriting.
5. **Approvals remain Host-owned.** Core and Coordinator operations preserve
   the normal Codex MCP approval policy.
6. **Facts remain immutable.** Agent requests carry an opaque handle; the MCP
   process resolves it to cached facts and never accepts a caller-authored
   replacement.
7. **Exact installation ownership.** A Skill binding verifies only when its
   absolute path belongs to exactly one selected-Host Candidate.
8. **No brand exceptions.** Built-in and user Providers enter the same
   matching and compilation path.
9. **No unknown-to-verified promotion.** Missing stable Host evidence remains
   `unknown`, Candidate, or ineligible.
10. **DIRECT remains lightweight.** Bridge failure does not block Direct Mode.
11. **Workflow remains strict.** Missing Bridge or binding evidence never
    silently makes a Profile eligible.
12. **Cooperative guarantee only.** Host reports and Coordinator state are
    validated cooperation records, not physical containment claims.

## 6. Component Architecture

```text
Codex current session
        |
        | OAW MCP tool call
        v
OAW Plugin PreToolUse Hook
        |
        | reserved current-call context
        v
OAW Codex Bridge MCP server
        |
        +-- Codex App Server skills/list
        +-- Codex App Server hooks/list
        +-- allowlisted config/read projection
        |
        +-- OAW discovery + registry + Core
        +-- optional Workflow Coordinator exchange
        |
        v
Lifecycle Bundle / Dispatch Packet / diagnostics
        |
        v
Codex current session executes Skills and tools
```

The implementation consists of four parts:

1. **Codex Plugin package** declares the bundled MCP server, lifecycle Hooks,
   and an optional instruction Skill.
2. **Hook adapter** parses Codex Hook input, injects or validates session-bound
   private fields, and never reads engineering project content.
3. **Bridge MCP adapter** owns the bounded in-memory fact cache and maps MCP
   operations to existing OAW Core and Coordinator calls.
4. **Metadata client** performs a version-negotiated JSON-RPC exchange with a
   metadata-only Codex App Server and projects only allowlisted results.

The instruction Skill is guidance only. Its presence cannot prove the Bridge,
the current session, or another Provider binding.

## 7. MCP and Hook Operations

The logical v1 operation surface is:

| Operation | Effect | Host context behavior |
| --- | --- | --- |
| `observe_current` | Read-only Host observation and handle creation | Hook rewrites private input and returns `allow` |
| `core.inspect` | Read-only Provider and Profile eligibility | Hook validates handle/session and otherwise returns no decision |
| `core.compile` | Read-only explicit selection compilation | Hook validates handle/session and otherwise returns no decision |
| `workflow_exchange` | Coordinator state transition | Hook validates handle/session and otherwise returns no decision |

The MCP schema exposes only the public inputs. The private
`_oaw_host_context` field is not model-fillable and the public schema rejects
unknown properties. The exact generated MCP tool names are fixed in the
Plugin Hook matcher; Hooks never use a broad Bash matcher.

`observe_current` is the only operation that uses `updatedInput`. Codex
requires `permissionDecision: "allow"` when a Hook rewrites a tool input, so
this operation must remain strictly read-only. The injected context contains:

```text
schema_version
bridge_protocol_version
session_id
turn_id
tool_use_id
cwd
model
permission_mode
```

All later operations carry a `HostEvidenceHandle`. Their Hook performs an
early session and working-directory comparison. A mismatch is denied. A match
produces no `allow` decision, so the user's configured MCP approval behavior
continues normally. The Bridge server performs the authoritative cache lookup
and full comparison before calling Core or Coordinator.

The Bridge does not expose an arbitrary command string, shell operation, model
prompt, or generic method passthrough.

## 8. Host Evidence Handle

`HostEvidenceHandle` is an opaque, high-entropy, session-bound cache reference.
It is not a Lifecycle Bundle field, credential, or durable evidence artifact.

The internal handle record contains:

```text
random identifier
host integration and protocol version
session identity digest
working-directory digest
issued and expiry times
Host Session Snapshot
Binding Inventory
Environment Report
Provider discovery and configuration snapshot digests
```

The external handle may include a non-secret version and session/cwd digest
header so the Hook can reject obvious cross-session use. The Bridge cache is
authoritative: an edited, fabricated, unknown, expired, or evicted handle is
rejected even if its external header looks valid.

Handle rules:

- generate at least 128 bits of cryptographically secure randomness;
- bind to exact `session_id`, canonical `cwd`, and Bridge protocol version;
- use a short configurable TTL with a bounded minimum and maximum;
- cap cache count and memory with deterministic LRU eviction;
- clear all handles when the Bridge process exits;
- never write handles to Workflow State, evidence files, normal logs, or the
  repository;
- permit a new observation in the same session;
- require exact fact digests to continue an existing Bundle; and
- reject cross-session and cross-directory reuse before any state mutation.

The design intentionally does not add a shared signing key, evidence file, or
long-running daemon. The existing OAW trust model assumes a cooperating Host;
it does not claim same-user process isolation.

## 9. Existing Host Protocol Mapping

Bridge v1 reuses these active schemas:

- `oaw.host-session/v2`
- `oaw.host-binding-inventory/v2`
- `oaw.host-environment-report/v2`
- `oaw.lifecycle-bundle/v3`
- `oaw.workflow-command/v1` and the active Coordinator records

The Session Snapshot is constructed as follows:

```text
host_id                   = codex
integration_id            = oaw/codex-host
integration_version       = exact Bridge release
session_id                = Hook-injected Codex session ID
supported_topologies      = [CURRENT]
provider_inventory_digest = normalized Binding Inventory digest
environment_report_digest = normalized Environment Report digest
sandbox_policy_digest     = empty unless a stable exact projection exists
approval_policy_digest    = empty unless a stable exact projection exists
```

`permission_mode` is an exact observed diagnostic field, but it is not
promoted to a complete approval-policy digest. The active model is diagnostic
metadata and does not affect Provider eligibility.

The Environment Report uses `CURRENT`, the exact current session ID, and no
parent session. It records only normalized dispositions:
`inherited`, `host-configured`, `restricted`, `unknown`, or `unavailable`.

No Bridge-private field is added to the public Host schemas. Versioned private
MCP inputs remain inside the Codex integration package.

## 10. Codex Metadata Observation

The Bridge performs an initialized JSON-RPC exchange with a Codex App Server
using the same working directory and user environment. It is a metadata
client. It must not call `thread/start`, `thread/resume`, `thread/fork`,
`turn/start`, `turn/steer`, model invocation, or process-execution methods.

Allowed v1 methods are:

- `skills/list`, scoped to the exact current `cwd`;
- `hooks/list`, scoped to the exact current `cwd`; and
- `config/read`, projected through a strict allowlist.

`skills/list` is the Provider binding authority for v1. Its stable response
includes Skill name, absolute path, scope, and enabled status. The Bridge may
refresh the list for each observation. A changed result within a pinned
Workflow produces changed Host fact digests rather than silently expanding
authority.

`hooks/list` is diagnostic environment evidence only. Raw Hook commands and
payloads are not retained. `config/read` is not a generic configuration export;
the projection excludes credentials, MCP environment variables, headers,
tokens, authentication material, arbitrary Plugin settings, and unrelated
user preferences.

`plugin/list` is explicitly excluded because Codex documents it as under
development. Other Plugin state remains `unknown` except that successful
current invocation proves the OAW Plugin's Hook and MCP surfaces are active.

Failure of a non-required environment observation may produce
`HOST_OBSERVATION_PARTIAL`. Failure of the Skill inventory prevents affected
Provider verification.

## 11. Provider Skill Binding Algorithm

Provider discovery remains Host-scoped and Descriptor-driven. Metadata
observation does not replace candidate discovery; it supplies the missing
Host-owned binding evidence.

For each enabled Codex Skill entry:

1. Require `enabled == true`.
2. Require a normalized absolute path to an existing `SKILL.md`.
3. Canonicalize the path without following an unsafe project-controlled
   indirection outside the reported installation.
4. Find discovered Codex Provider Candidates whose physical installation root
   contains the Skill path.
5. Require exactly one matching Candidate. Zero matches creates an orphan
   diagnostic; multiple matches are ambiguous and create no authority.
6. Match the Skill name exactly against a Descriptor Capability's Codex
   `HostBinding.reference` with `kind = "skill"`.
7. Require the Binding Host, Inventory Host, Candidate Host, and selected Host
   all equal `codex`.
8. Intersect declared binding topologies with the Bridge's observed
   topologies. The v1 result is only `CURRENT`.
9. Hash a canonical projection containing the Skill name, scope, enabled
   state, canonical path identity, relevant Skill content, Candidate
   Installation Key, and Bridge observation source.
10. Emit a `BindingObservation` with `source = "native-probe"` and an
    `evidence://codex/skills-list/<digest>` reference.

An observation has this semantic shape:

```text
host_id            = codex
installation_key   = exact matched Candidate
binding.host       = codex
binding.kind       = skill
binding.reference  = exact Skill name
binding.topologies = Descriptor-declared topologies
topologies         = [CURRENT]
source             = native-probe
evidence_reference = evidence://codex/skills-list/<digest>
digest             = canonical observation digest
```

Provider Registry resolution then uses the existing order: validate Host,
apply denials and Host-scoped pins, select one Candidate, intersect its exact
binding observations, apply limits and preferences, and construct a verified
Provider Instance.

Exact failure behavior:

- disabled Skill: no observation;
- no Skill for a declared binding: `HOST_BINDING_EVIDENCE_REQUIRED`;
- valid Skill outside all Candidates: orphan diagnostic only;
- several Candidates for the Provider: existing
  `PROVIDER_CANDIDATE_AMBIGUOUS`;
- one Skill path attributable to several installations: Host installation
  ambiguity, no observation;
- pin mismatch: existing `PROVIDER_PIN_INCOMPATIBLE`;
- malformed or duplicate inventory: existing
  `HOST_BINDING_INVENTORY_INVALID`; and
- exact Candidate with no usable Capability Binding: existing
  `PROVIDER_BINDING_UNAVAILABLE`.

User configuration can provide a trusted Descriptor, Recipe, pin, denial, or
discovery location. It cannot manufacture an enabled Skill or Binding
Observation.

## 12. Binding Scope and ECC

Bridge v1 authoritatively supports `skill` bindings only.

An `agent` declaration in an OAW Descriptor is not verified merely because
`config/read` contains an Agent entry or an instruction file names an Agent.
OAW needs a stable, current-session Codex Agent inventory and topology contract
before it can upgrade that declaration. Similarly, a configured MCP server or
tool name is not a verified `tool` binding without a stable current-session
tool inventory.

This limitation does not demote ECC as a Provider or remove `ECC-FULL`.
`ECC-FULL` remains a complete built-in Profile Recipe. Its Codex eligibility
depends on ECC exposing the required Capabilities through exact Codex Skill
bindings that Bridge v1 can observe, or on a future version adding conformant
Agent binding evidence. OAW must not invent ECC authority to make the built-in
Profile eligible.

Superpowers, Matt, ECC, and user Providers all follow this rule. A Provider may
be fully eligible, partially eligible, Candidate-only, or absent depending on
the exact current Host installation.

## 13. Request and Workflow Flows

### 13.1 DIRECT

```text
classify DIRECT -> current Codex session executes -> focused verification
```

Direct Mode has no Bridge, Provider Capability, Profile, Startup Gate,
Lifecycle Bundle, or Coordinator state. Bridge failure never blocks Direct
work.

### 13.2 BOUNDED

```text
observe_current
  -> resolve exactly one verified Capability
  -> validate CURRENT and declared effects
  -> current Codex session executes
  -> one terminal outcome
```

Missing exact binding evidence prevents an OAW-verified Bounded Capability. A
second Capability or remediation loop requires reclassification.

### 13.3 WORKFLOW

```text
observe_current
  -> Core inspect verified Providers and eligible Profiles
  -> Main Agent presents Startup Gate
  -> user selects Profile, CURRENT, and add-ons
  -> Core compile OR Coordinator START invokes Core atomically
  -> return Lifecycle Bundle and active Dispatch Packet
  -> current Codex session invokes the selected Skill and tools
  -> submit normalized Receipt and evidence references
  -> Coordinator commits the next legal revision
```

The Main Agent owns user communication and physical execution. The Bridge and
Coordinator never auto-select a Profile and never invoke the selected Skill.

`core.inspect` returns verified, Candidate, and excluded Provider state; every
Profile and add-on eligibility result; all exclusion reasons; eligible
topologies; a non-binding recommendation; and a secret-free Host evidence
summary. The Startup Gate waits indefinitely for an explicit user selection.

`core.compile` supports policy-only state when the caller does not request
durable coordination. `workflow_exchange` exposes the existing Coordinator
protocol for coordinated Workflow state. The Coordinator still calls Core
inside `START`; callers cannot submit their own Bundle.

## 14. Recovery and Host Fact Changes

Handle lifetime and Workflow lifetime are independent.

- An expired handle requires `observe_current` again.
- If the new Session, Inventory, Environment, Configuration, Resolution, and
  Registry digests equal the pinned facts, the Workflow may continue.
- A Bridge process restart invalidates handles but does not delete Coordinator
  state.
- Context compaction with the same Codex session ID may continue after a fresh
  observation.
- A changed session ID, working directory, Inventory, or required environment
  fact produces `HOST_SESSION_CHANGED`.
- A changed Host fact never silently replaces a Bundle pin.
- Rebinding a new session requires the existing legal recovery or switch path,
  an allowed stable boundary, explicit selection when required, and a new
  Bundle generation.
- Missing native Subagent support leaves `CURRENT`; there is no process
  fallback.

Coordinator state remains authoritative for cooperating coordinated clients.
The Host handle itself is never journaled.

## 15. Trust and Security Model

The user explicitly trusts the installed OAW Codex Plugin and its exact Hook
definition through Codex. A changed Hook hash must be reviewed again. OAW does
not use `--dangerously-bypass-hook-trust`.

Trust sources are deliberately separated:

| Source | Authority |
| --- | --- |
| Codex Hook input | Current call/session identity and permission mode |
| Codex stable metadata API | Enabled Skill metadata for the scoped working directory |
| OAW discovery evidence | Provider Distribution and Host Installation Candidate |
| OAW Descriptor | Allowed Capability and Binding contract only |
| User configuration | Trusted policy constraints and explicit locations, never Host observations |
| Agent request | Requested action and explicit user selection, never Host facts |

The Bridge validates input size, UTF-8, control characters, canonical paths,
schema versions, bounded arrays, digests, TTL, and cache limits. It uses no
shell interpolation for Hook payloads or MCP arguments. JSON-RPC traffic is
strictly decoded with bounded messages and timeouts.

Data minimization rules:

- persist Skill identity, scope, enabled state, installation association, and
  digest; do not expose full absolute paths to the model by default;
- retain Hook availability and digest, not Hook command text or payloads;
- project allowlisted config facts only;
- never store MCP environment values, headers, tokens, credentials, private
  Plugin configuration, or transcript contents;
- use the session ID as an opaque Host identity and show a short digest in
  normal diagnostics;
- never persist or log a HostEvidenceHandle;
- record `permission_mode` as a scoped observation, not a complete approval
  policy; and
- record unavailable Sandbox, MCP, Hook, or Plugin facts as `unknown` rather
  than guessing.

The Bridge is not an operating-system security boundary. A process with the
same user's full authority can interfere with local programs. OAW's claim is
limited to validated cooperation among the trusted Codex integration, OAW
Core, and Coordinator.

## 16. Diagnostics

Bridge diagnostics are distinct from Provider and Profile diagnostics:

| Code | Meaning and recovery |
| --- | --- |
| `HOST_BRIDGE_UNAVAILABLE` | Plugin or MCP Bridge is not available in this session; install/enable it and start a new session |
| `HOST_BRIDGE_CONTEXT_REQUIRED` | MCP is available but the trusted Hook did not inject context; review and trust the Hook |
| `HOST_BRIDGE_PROTOCOL_MISMATCH` | Plugin, Hook, Bridge, or Core protocol versions are incompatible; update the complete Bridge bundle |
| `HOST_EVIDENCE_HANDLE_REQUIRED` | A host-native operation omitted its current handle; observe again |
| `HOST_EVIDENCE_HANDLE_INVALID` | Handle is malformed, unknown, edited, evicted, or from a restarted Bridge; observe again |
| `HOST_EVIDENCE_EXPIRED` | Handle exceeded its TTL; observe again |
| `HOST_EVIDENCE_SESSION_MISMATCH` | Handle belongs to another session or working directory; reject before mutation |
| `HOST_OBSERVATION_FAILED` | Required stable metadata observation failed; affected Providers cannot verify |
| `HOST_OBSERVATION_PARTIAL` | Optional environment metadata is incomplete; retain explicit unknown fields |
| `HOST_SESSION_CHANGED` | Facts pinned by the active Bundle changed; pause and use legal recovery or switching |

Existing codes remain authoritative at their own layer, including
`HOST_BINDING_EVIDENCE_REQUIRED`, `HOST_BINDING_INVENTORY_INVALID`,
`PROVIDER_CANDIDATE_AMBIGUOUS`, `PROVIDER_PIN_INCOMPATIBLE`,
`PROVIDER_BINDING_UNAVAILABLE`, and `PROFILE_TOPOLOGY_UNAVAILABLE`.

Every diagnostic response includes the failing layer, affected Providers and
Profiles, whether Direct Mode remains available, whether a new observation can
recover, a concrete next action, and a secret-free evidence digest.

## 17. Plugin and Installation Design

Policy installation and Host Bridge installation remain separate commands:

```text
oaw install --target codex
oaw bridge install codex
```

The first installs the existing instruction adapter. The second is an explicit
opt-in to executable Host integration and Hook review.

The Plugin package contains:

```text
.codex-plugin/plugin.json
.mcp.json
hooks/hooks.json
skills/<bridge-instruction-skill>/SKILL.md
```

The OAW release embeds the Plugin templates. `oaw bridge install codex`
installs a checksum-pinned copy of the running OAW binary under an OAW-owned
user data directory, renders the Plugin with exact absolute helper paths, and
uses the official `codex plugin marketplace` and `codex plugin add` commands
against an OAW-owned local marketplace. It does not depend on the caller's
future `PATH`, download executable code, or directly edit Codex's Plugin cache.

Codex owns its Plugin cache and enablement configuration. OAW owns only its
rendered local marketplace source, managed binary copy, and Bridge install
state. Installation failure must roll back OAW-owned artifacts and use the
official Codex removal operation for any Plugin registration completed during
the failed transaction.

The production command surface is:

```text
oaw bridge serve codex
oaw bridge hook codex
oaw bridge check codex
oaw bridge install codex
oaw bridge update codex
oaw bridge uninstall codex
```

`bridge check` is read-only and reports installation files, versions, Codex
capability availability, and configured Plugin/Hook status where observable.
It cannot claim that the current session loaded the Bridge. Only successful
`observe_current` provides that evidence.

After install or update, the user must review the Hook in `/hooks` and start a
new Codex session. Hook re-review after an updated definition is expected and
must not be bypassed.

Uninstall uses the official Codex Plugin removal command, removes the OAW local
marketplace registration only when OAW owns it, and removes only clean,
recorded OAW Bridge artifacts. It preserves unrelated Codex configuration and
user data.

## 18. Host Integration and Version Negotiation

OAW adds a separate built-in `oaw/codex-host` integration. The existing
`oaw/codex-policy` record remains the honest policy surface; it is not upgraded
in place or treated as host-native without evidence.

The host-native integration declares:

```text
control_surface       = host-native
protocols             = [oaw.workflow/v1]
binding_kinds         = [skill]
supported_topologies  = [CURRENT]
features              = provider-binding-inventory,
                        normalized-receipts,
                        environment-reporting
```

The exact feature set is included only after conformance proves it. Audit and
Conformance records must pass before the integration enters production
configuration.

Version negotiation covers:

- Plugin manifest version;
- private Bridge MCP protocol version;
- private Hook context schema version;
- Host integration version;
- active public Host/Core/Coordinator schema versions; and
- observed Codex metadata capabilities.

The Bridge probes required methods instead of inferring them only from a Codex
version string. Codex `0.146.1` is the initial development baseline; the
minimum supported version is set from compatibility evidence.

An incompatible component returns `HOST_BRIDGE_PROTOCOL_MISMATCH`. There is no
legacy Runner fallback, old Host inventory decoder, topology alias, or silent
policy-to-host-native promotion.

## 19. Verification Strategy

### 19.1 Unit tests

- Hook input parsing, exact matcher behavior, private context construction,
  and mismatch denial;
- Handle generation, entropy, TTL boundaries, LRU eviction, process-reset
  behavior, cross-session and cross-directory rejection;
- metadata JSON-RPC framing, initialization, bounded decoding, method
  allowlist, timeouts, and failure normalization;
- Skill path canonicalization, name matching, disabled Skills, orphan paths,
  duplicate names, overlapping Candidates, content digest changes, and
  installation-key association;
- redaction and allowlist projections for Hooks and config;
- Bridge diagnostic code and recovery mapping; and
- immutable cloning of all returned records.

### 19.2 Integration tests

- stdio MCP initialize, tool listing, observe, Core inspect/compile, and
  Coordinator exchange;
- simulated Codex Hook rewrite followed by Bridge cache lookup;
- fake App Server transcripts for complete, partial, malformed, slow, and
  unsupported metadata responses;
- real Core compilation for Superpowers, Matt, ECC, and a user Provider using
  the same binding algorithm;
- policy-only and host-native Integration selection without authority mixing;
- Workflow START, Dispatch, Receipt, recovery, and changed Host facts; and
- install, update, uninstall, rollback, drift, and Hook re-trust reporting in
  isolated temporary Codex homes.

### 19.3 Negative and security tests

- MCP available with Hook missing or untrusted;
- caller-authored or missing private Host context;
- edited, guessed, expired, evicted, and cross-session handles;
- Agent attempts to replace Inventory, Session, or Environment records;
- Descriptor or user config attempts to forge a binding;
- path traversal, symlink escape, control characters, oversized payloads, and
  duplicate JSON identities;
- secret-bearing config fixtures proving redaction;
- production code guard proving `plugin/list` is not called;
- process-spawn guard proving no Thread, Agent, model, private HOME, or Runner
  process is created; and
- approval tests proving only observation receives Hook `allow`.

### 19.4 Platform and dogfood tests

- macOS: install the real Plugin, review the Hook, start a new Codex session,
  observe exact Skills, run the normal Startup Gate, and complete a controlled
  `CURRENT` dogfood;
- Linux: run Go, MCP protocol, filesystem, management, black-box, and smoke
  tests in Docker;
- unavailable Windows or Host-native environments: record explicit `SKIP`
  without blocking supported environments; and
- run `go test ./...`, `go test -race ./...`, documentation gates, the full
  black-box suite, and Docker Linux smoke before completion.

## 20. Acceptance Criteria

1. A trusted Codex Hook supplies the current session context to a read-only
   Bridge observation without Agent-authored Host facts.
2. The Bridge constructs canonical Host Session, Binding Inventory, and
   Environment Report v2 records for `CURRENT`.
3. At least one actually enabled Codex Skill verifies against its exact
   discovered Provider installation in a new real Codex session.
4. Disabled, missing, ambiguous, orphaned, and foreign-Host Skills do not
   create Provider authority.
5. `oaw providers inspect --strict` no longer leaves an actually proved
   Provider in Candidate state.
6. Uninstalled or unsupported Matt and ECC bindings remain honestly
   ineligible; no built-in exception fabricates them.
7. The Startup Gate presents Profile eligibility from the real Host inventory
   and waits for explicit user selection.
8. Core compilation and Coordinator START pin the same immutable Host facts.
9. The current Codex session, not OAW, invokes all Skills and tools.
10. Core and Coordinator operations retain Codex's configured approval
    behavior.
11. Bridge absence does not block Direct Mode and cannot produce a
    host-native claim.
12. Installation, update, Hook trust, new-session activation, diagnostics, and
    uninstall are documented in English and Chinese.
13. No Runner, model-process launcher, private HOME, projected Host config,
    legacy topology alias, or compatibility decoder is reintroduced.
14. Native macOS verification and Docker Linux verification pass; unavailable
    environments are explicitly skipped.

## 21. Rollout Order

Implementation planning should split the work along these dependency edges:

1. private Bridge contracts, Hook parser, handle cache, and diagnostics;
2. metadata-only Codex App Server client and redacted observations;
3. exact Skill-to-installation Binding Inventory assembly;
4. Bridge MCP operations over existing Core and Coordinator;
5. host-native Integration, audit, and Conformance assets;
6. explicit Plugin packaging and Bridge management commands;
7. bilingual product documentation and troubleshooting;
8. isolated integration, security, black-box, Docker, and real-session
   dogfooding; and
9. final release engineering for the remaining original release ticket.

Each phase must remain independently testable. The implementation plan may
refine file boundaries and test commands, but it must not change the approved
authority, trust, topology, evidence, or installation contracts without a new
design decision.

## 22. Approved Decisions

The following decisions are closed for implementation planning:

- v1 uses a Codex Plugin with a bundled MCP server and exact `PreToolUse`
  Hooks.
- v1 proves current-session identity and exact enabled Skill bindings.
- v1 supports `CURRENT` only and defers `SUBAGENT`.
- MCP, Hooks, Plugins, Sandbox, and approvals are observations only where
  stable evidence exists; they are never reconstructed.
- `plugin/list` is not a production dependency.
- Host facts stay in a Bridge cache behind a session-bound handle.
- only the read-only observation operation may be automatically allowed for
  Hook input rewriting.
- existing Host v2, Core, Bundle, and Coordinator records are reused.
- Skill paths and exact installation identity, not Skill names alone, prove a
  binding.
- Agent and tool bindings remain unverified until stable Host inventory APIs
  exist.
- ECC-FULL remains a complete Profile whose eligibility is evidence-driven.
- Policy installation and Bridge installation are separate explicit actions.
- a new Codex session is required after Plugin installation or update.
- the hard cutover has no Runner or compatibility fallback.

