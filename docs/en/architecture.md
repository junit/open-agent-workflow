# Architecture

[简体中文](../zh/architecture.md) | [README](../../README.md)

Open Agent Workflow (OAW) distributes one canonical engineering policy,
compiles conflict-free lifecycle contracts, and optionally coordinates durable
Workflow State. It never owns Agent execution. Agent tools and engineering
Providers remain independently installed and versioned.

## Components and Boundaries

The product has four modules with separate authority:

1. **Distribution** installs `policy/ENGINEERING.md` through target-native
   instruction surfaces and manages checksummed Install State and backups.
2. **OAW Core** is required and stateless. It classifies requests, resolves
   verified Provider Instances, compiles Profile Recipes, and creates immutable
   Lifecycle Bundles.
3. **Workflow Coordinator** is optional and Workflow-only. It records revisions,
   idempotency, cooperative Resource Leases, evidence, cancellation, switching,
   and recovery for cooperating clients.
4. **Agent Host** is external to OAW. It owns Agents, model calls, MCP, Hooks,
   Skills, Plugins, authentication, tools, sandbox, approvals, and every
   physical effect.

The primary control flow is:

```text
Request -> OAW Core -> Lifecycle Bundle -> Agent Host -> Receipt
                          |
                          +-> optional Workflow Coordinator
```

Distribution does not begin an engineering lifecycle. OAW Core does not retain
Workflow State. The Workflow Coordinator does not execute work. The Agent Host
does not gain authority to rewrite a Bundle.

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

## Execution Topologies and Host Integration

OAW recognizes only two execution topologies:

- `CURRENT` uses the active Agent session and its environment unchanged.
- `SUBAGENT` asks the active Agent Host to create a child through its native
  Subagent facility.

There is no process or container fallback for an unavailable Subagent. The
eligible set is the intersection of the selected Profile, every active
Capability binding, integration metadata, and current Host session facts. The
user selects a topology during the Workflow Startup Gate.

All nine built-in integrations currently have the `policy` control surface and
support `CURRENT`. A future `host-native` integration must support `CURRENT`;
its `SUBAGENT` availability remains session-dependent. It reports facts,
Dispatch Packet outcomes, and Receipts but never transfers a model command,
credential, private Hook payload, or private MCP, Skill, or Plugin configuration
to OAW.

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
another process. Policy-only use follows the same lifecycle ownership rules
without claiming atomic revisions, idempotency, leases, or transition
enforcement.

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

- `managed-block` inserts one marker-delimited OAW block while preserving
  surrounding user content. Claude, Codex, Gemini, and OpenCode use this mode.
- `owned-file` reserves an adapter-specific file for OAW. Cursor, Windsurf,
  Cline, Roo Code, and Copilot use this mode.

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
