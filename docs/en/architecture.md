# Architecture

[简体中文](../zh/architecture.md) | [README](../../README.md)

Open Agent Workflow (OAW) distributes one canonical engineering policy,
compiles conflict-free lifecycle contracts, and optionally coordinates durable
Workflow State. It never owns Agent execution. Agent tools and engineering
Providers remain independently installed and versioned.

## Components and Boundaries

The product has four modules with separate authority:

1. **Distribution** installs `policy/ENGINEERING.md` plus a lazy target-native
   Activation Router and manages checksummed Install State and backups.
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

## Canonical Storage

OAW follows the XDG base-directory convention while retaining explicit
defaults:

| Artifact | Canonical path |
| --- | --- |
| Installed policy | `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/ENGINEERING.md` |
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

Codex has a policy integration by default and a separate audited host-native
Bridge at `oaw/codex-host`. The Bridge must be explicitly installed and trusted;
Current Codex proves only `skill` bindings and `CURRENT` topology. The policy
surface remains `oaw/codex-policy` and is never promoted by filesystem
detection. Role, instruction, agent, tool, delegation, invocation,
`workspace.prepare-or-confirm`, `verification.execute`, and `closeout.execute`
remain unknown or unavailable unless a stable live API attests them.

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

Codex exposes `oaw/codex-policy` by default and the opt-in `oaw/codex-host`
host-native surface. The Bridge supports `CURRENT` and `skill` bindings only;
its session-dependent evidence comes from the trusted Hook and allowlisted Host
metadata. Other built-in integrations remain policy surfaces unless their own
Host-native integration is explicitly installed and verified. No integration
transfers a model command, credential, private Hook payload, or private MCP,
Skill, or Plugin configuration to OAW.

The Codex Bridge path is:

```text
observe_current -> Core inspect -> explicit Startup Gate
                -> Core compile / Coordinator START
                -> current Codex session executes Skills and tools
```

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
| `policy` | one | Installed policy path and checksum metadata. |
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
