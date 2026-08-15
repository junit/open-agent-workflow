# Architecture

[简体中文](../zh/architecture.md) | [README](../../README.md)

Open Agent Workflow (OAW) distributes one canonical engineering policy,
compiles conflict-free lifecycle contracts, and optionally coordinates durable
Workflow State. It never owns Agent execution. Agent tools and engineering
Providers remain independently installed and versioned.

## Components and Boundaries

The product has four modules with separate authority:

1. **Distribution** installs the user Policy or the complete project-scoped
   Canonical Policy Set plus a lazy target-native Activation Router, and manages
   checksummed Install State and backups.
2. **OAW Core** is stateless. After explicit activation and current Host-native
   evidence, it classifies requests, resolves verified Provider Instances,
   compiles Profile Recipes, and creates immutable Lifecycle Bundles.
3. **Workflow Coordinator** is optional and Workflow-only. It records revisions,
   idempotency, cooperative Resource Leases, evidence, cancellation, switching,
   and recovery for cooperating clients.
4. **Agent Host** is external to OAW. It owns Agents, model calls, MCP, Hooks,
   Skills, Plugins, authentication, tools, sandbox, approvals, and every
   physical effect.

The primary control flow is:

```text
Top-level user request
    -> Activation Router
       -> Native Host
       -> Activated OAW
          -> Assurance Preflight
             -> policy-cooperative -> Agent Host
             -> core-backed -> OAW Core -> Lifecycle Bundle -> Agent Host
             -> coordinator-backed -> OAW Core -> Workflow Coordinator -> Agent Host
```

Always-applied frontmatter is Host metadata, not activation. Distribution and
Bridge installation do not begin an engineering lifecycle. OAW Core does not
retain Workflow State. The Workflow Coordinator does not execute work. The
Agent Host does not gain authority to rewrite a Bundle.

## Policy and Machine Profile Projections

OAW keeps one authority-neutral lifecycle meaning and projects it onto two
independent implementations:

```text
Shared Profile semantics
  aliases, responsibilities, gates, macro credit, incidents, switching
        |
        +-- Policy Projection
        |     Host-visible and user-explicit routes
        |     neutral Host actions and cooperative records
        |
        +-- Machine Projection
              verified Provider Instances and Bindings
              Core compilation and optional coordination records
```

The Policy Projection is implemented by the Policy catalog, route inspector,
lifecycle reducer, Engagement, and project-bound persistence modules. Its
route input contains only the route name and invocation mode. It reports
`policy_selectable` separately from `host_routable`, derives every next action
from typed events, and never imports discovery, integrity, Registry, Core,
Coordinator, or Bridge authority.

The Machine Projection retains Provider descriptors, source audits, complete
Binding-tree verification, Core compilation, and optional Coordinator state.
Machine attestation may increase assurance, but it cannot veto a Policy Offer.
Lockfiles, distribution identity, installation paths, revisions, tree digests,
and Bridge state therefore never decide whether an otherwise routable Policy
Profile can be selected.

All four built-in Profiles remain in both projections. With the required Codex
routes present, `SP-FULL`, `MATT-FULL`, `ECC-FULL`, and `MATT-SP-HYBRID` can
traverse the cooperative `CURRENT` lifecycle without Bridge. This proves route
availability only; it does not verify Skill provenance, physical execution, or
effect containment.

## Canonical Storage

OAW follows the XDG base-directory convention while retaining explicit
defaults:

| Artifact | Canonical path |
| --- | --- |
| User Policy Set | `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/` |
| User Built-in Profiles (managed) | `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/builtin/` |
| User Custom Profiles (user-owned) | `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/` excluding `builtin/` |
| Project Policy Set | `<project>/.oaw/policy/` |
| Project Custom Profiles (user-owned) | `<project>/.oaw/profiles/` |
| User installation state | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/installations/user.state` |
| Project installation state | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/installations/projects/<crc>-<bytes>.state` |
| Operation backups | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/backups` |
| Workflow State | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/workflows` |

Install State and Workflow State use disjoint namespaces. There is no migration
or implicit adoption between them. `<crc>-<bytes>` is the `cksum` result for
the physical project-root path bytes; it isolates installer metadata without
placing that metadata inside the repository.

Project Workflow documents are one-way, non-authoritative projections of
committed Workflow State. They are never parsed back into authority.

When `<project>/.oaw/policy/POLICY.md` exists, the Project Policy Set is selected
as a whole. Otherwise the User Policy Set is selected. Core Policy files are
never merged; project and user Custom Profiles remain separately discoverable
with their source identity.

## Distribution Data Flow

The management mutation pipeline is:

```text
embedded checkout policy -> pure renderer -> preflight/prepare -> required backup -> apply -> Install State/targets
```

The Go binary embeds the Policy, Version, registry, and renderer behavior from
the checkout used to build it. Release archives already contain that binary;
a source checkout must build `./oaw` before execution. The arrows describe data
and control flow, not simultaneous operation-wide atomicity. A **pure renderer**
writes prospective content only to a caller-provided temporary path and never
inspects or mutates the eventual destination.

## Host-Scoped Provider Authority

Provider identity and authority flow through one chain:

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

Codex and Claude Code are independent Hosts. A physical Provider directory
shared by both still yields different Host Installation keys because Host and
surface identities participate in the digest. Descriptor bindings and
configured installation hints are declarations only. A Host must report the
exact binding and associate it with the exact installation before the registry
can emit a Verified Provider Instance.

A `policy` integration can distribute instructions and report Candidates, but
static detection alone never verifies a Provider Instance. A `host-native`
integration can report secret-free session facts and Host Binding Evidence.
Foreign-Host diagnostics never enter pin matching, Profile compilation, or a
Lifecycle Bundle. Active schemas reject `oaw.provider-descriptor/v1` and
`oaw.user-config/v1` instead of silently upgrading them.

Codex has only `oaw/codex-policy` in the default Integration set. The separately
built `oaw-bridge` is an optional Assurance adapter, not a host-native workflow
surface. Its `observe_profile` operation can attach exact current `skill`
Binding identities to one source-qualified Markdown Profile. It does not attest
topology, delegation, invocation, Host actions, Receipts, or completion, and it
cannot promote the Policy surface through filesystem detection.

The v4 Provider model keeps Skills, Claude custom Agents, Codex Roles,
Instructions, Hooks, and tools distinct. Binding kinds are `skill`, `agent`,
`role`, and `instruction`; complete Binding-tree evidence is checked below the
exact Distribution and Host Installation. A shared ancestor, same name,
filesystem role file, or static multi-agent configuration cannot create a
verified Binding or prove live delegation.

## Core Compilation

OAW Core accepts request evidence, trusted configuration, Host session facts,
verified Provider Capabilities, and an explicit user selection. It returns
eligible Profiles and topologies, reason-coded exclusions, recommendations,
and the immutable Lifecycle Bundle. Callers never construct a Bundle.

Every built-in and user Provider follows the same descriptor, binding, and
compiler path. A Provider brand does not fix its role; the selected Recipe
assigns each responsibility exactly once. Built-in selections are:

| Selection | Recipe |
| --- | --- |
| `SP-FULL` | `oaw/delivery` |
| `MATT-FULL` | `oaw/domain-engineering` |
| `ECC-FULL` | `oaw/ecc-engineering` |
| `MATT-SP-HYBRID` | `oaw/reliable-feature` |
| `USER-DEFINED` | A configured, versioned user Recipe |

`ECC-FULL` remains a complete lifecycle. The same ECC Provider may instead own
one bounded specialist Capability in another Recipe.

The compiler hard-cut is Provider Descriptor `oaw.provider-descriptor/v4`,
Profile Recipe `oaw.profile-recipe/v3`, Execution Graph
`oaw.execution-graph/v4`, Lifecycle Bundle `oaw.lifecycle-bundle/v4`, and
Capability Grant `oaw.capability-grant/v3`. Workflow command/result/snapshot/
revision records are v2. No reader converts an older authority record.

## Execution Topologies and Host Integration

OAW recognizes only two execution topologies:

- `CURRENT` uses the active Agent session and its environment unchanged.
- `SUBAGENT` asks the active Agent Host to create a child through its native
  Subagent facility.

There is no process or container fallback for an unavailable Subagent. The
eligible set is the intersection of the selected Profile, every active
Capability binding, integration metadata, and current Host session facts. The
user selects a topology during the Workflow Startup Gate.

Codex exposes `oaw/codex-policy` by default. `oaw-bridge` is separately built,
installed, and tested; the default `oaw` installer neither installs nor manages
it. Bridge reads only current `skills/list` metadata and returns an optional
Assurance Overlay. Other built-in integrations remain Policy surfaces unless a
separate Host-native workflow contract is explicitly installed and verified.
No integration transfers a model command, credential, private Hook payload, or
private MCP, Skill, or Plugin configuration to OAW.

The optional Codex Assurance path is:

```text
Markdown Profile -> Policy selection and rule-driven execution
       |
       +-> observe_profile -> current Skill Binding evidence
                           -> Assurance Overlay bound to the same Profile
```

Bridge failure removes the lower optional branch only. It never changes the
Policy Profile or controls the rule-driven branch.

The Agent Host owns physical execution authority. Lifecycle Bundles,
Capability Grants, and Resource Leases express logical workflow authority for
cooperating clients. A Grant may be narrower than the Host sandbox and
approvals, but it cannot physically stop out-of-protocol Host actions.

## Workflow Coordination

The optional Workflow Coordinator accepts only `WORKFLOW`. `DIRECT` and
`BOUNDED` create no Workflow State. It commits immutable revisions, the Bundle
generation and digest, current graph node, ticket, stable boundary, logical
Capability Grant, Resource Lease, Receipt, and digest-pinned evidence
references.

Resource Leases coordinate cooperating Workflows that declare conflicting
project resources. They do not lock the operating system, filesystem, Git, or
another process. At `policy-cooperative`, the Host may create a Policy Workflow
Plan, Progress Tracker, Execution Notes, and Conflict Warnings. Those terms
cannot create a verified Provider Instance, eligible Profile, Lifecycle Bundle,
Grant, Lease, Receipt, atomic revision, idempotency, transition enforcement, or
recovery guarantee.

## Management Transaction

During the **prepare phase**, OAW resolves source and destinations, renders
prospective files, checks containment and symlink rules, parses prior state,
detects drift, and verifies every planned action before any managed destination
is written. Target selection and shared-destination collisions are resolved at
this point.

If forced mutation would replace drifted or foreign content, OAW creates and
verifies an **operation-scoped backup** of every affected artifact before
applying any prepared action. The backup reference can be recorded in Install
State.

During the **apply phase**, paths are validated again immediately before use.
Each file is written beside its destination as a temporary file and moved into
place, providing **atomic replacement per destination**. After every effect,
the Go manager records an inverse in its mutation journal. A reported apply
failure attempts reverse-order rollback; rollback failure is surfaced as an
error. OAW does not claim simultaneous atomic replacement across destinations
or automatic recovery from a process or machine crash.

## Ownership Modes

Every target declares one ownership mode:

- `managed-block` inserts one marker-delimited Activation Router while preserving
  surrounding user content. Claude, Codex, Gemini, and OpenCode use this mode.
- `owned-file` reserves an adapter-specific Activation Router file for OAW.
  Cursor, Windsurf, Cline, Roo Code, and Copilot use this mode. Their
  always-applied metadata does not activate OAW.

Markers are an installer ownership boundary. **Marker comments do not establish model precedence**,
override a tool's documented instruction hierarchy, or force a running Agent
session to reload.

Drift means recorded OAW ownership no longer matches the current file. Without
`--force`, mutation fails closed. Uninstall removes a clean managed block or a
clean owned file and prunes only empty directories that state proves OAW
created.

## Install State Schema

Install State is tab-separated inert text and is never sourced or evaluated by
the shell. It is not Workflow State and cannot grant workflow authority.

| Record | Cardinality | Meaning |
| --- | --- | --- |
| `format` | one | State serialization format. |
| `version` | one | Installed OAW version. |
| `scope` | one | `user` or `project`. |
| `project` | project only | Physical project root. |
| `policy` | one | Primary installed Policy path and checksum metadata. |
| `policy-file` | zero or more | Each project Policy Set file path and checksum metadata. |
| `backup` | optional | Backup manifest associated with the last relevant operation. |
| `directory` | zero or more | Directory created and therefore potentially removable by OAW. |
| `target` | one or more | Target ID, absolute path, ownership mode, checksum, and origin. |

Before update or uninstall, OAW validates the record format, scope, project
identity, target registry membership, paths, ownership modes, and checksums.
Malformed or unsafe state is rejected rather than interpreted leniently.

## Trust Model

`HOME`, `XDG_CONFIG_HOME`, `XDG_STATE_HOME`, project roots, checkout artifacts,
existing target files, and state are trust-boundary inputs. OAW requires
absolute safe roots, resolves project scope physically, rejects unsafe symlink
or containment paths, and rechecks destinations during apply. The
[installer guide](installer.md) documents commands and failures; the
[adapter guide](adapters.md) documents each client-facing surface.
