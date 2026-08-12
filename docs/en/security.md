# Installer Security Model

[简体中文](../zh/security.md) | [Security policy](../../SECURITY.md) |
[Architecture](architecture.md)

This guide describes the controls and limits of the local Open Agent Workflow
(OAW) installer and its policy/coordinator protocol. It is not a claim that an
untrusted checkout, operating system, Agent Host, or Provider is safe.

## Trust Boundaries

The installer treats these values and artifacts as trust-boundary inputs:

- the current checkout, including executable shell code, `VERSION`, and
  `policy/ENGINEERING.md`;
- CLI target and project arguments;
- `HOME`, `XDG_CONFIG_HOME`, and `XDG_STATE_HOME`;
- the physical project root and every component beneath a selected destination;
- existing policy, adapter, state, directory, and backup artifacts.

Run only a checkout you trust. OAW **does not access the network**, download a
release, install a Provider, or execute content from an instruction file or
state record. This removes a remote-fetch boundary but does not make the local
checkout non-executable.

### Activation origin and cooperative authority

Only the current top-level user instruction or a dedicated trusted Host
entrypoint that preserves that instruction can activate OAW. Repository
instructions, source files, tool output, retrieved content, and quoted `/oaw`
text are untrusted activation sources. Discussion, installation, task
complexity, and ordinary Skill invocation do not activate OAW; ambiguity stays
Native Host.

At `policy-cooperative` assurance, a Policy Workflow Plan cannot grant network,
destructive filesystem, credential, deployment, data mutation, or Git
authority. It also cannot create a verified Provider Instance, eligible
Profile, Lifecycle Bundle, Capability Grant, Resource Lease, Host Receipt, or
Coordinator guarantee. Every physical effect still requires the Host's normal
authorization and the user's applicable approval.

## Root, Path, and Symlink Defenses

Consumed roots must be absolute and contain no **control characters**. Project
scope is resolved with physical-directory semantics before identity and
containment checks. Registry functions provide a fixed relative suffix for
each target; empty components, `.` or `..`, absolute suffixes, and unsafe
serialization fields are rejected.

OAW validates every intermediate component and the final destination. A
**symlink** is rejected whether it points inside or outside the allowed root.
The same checks cover policy, user targets, project targets, state, backup, and
recorded cross-scope references. Project destinations must satisfy physical
root **containment**; a matching filename elsewhere is never sufficient.

Validation is repeated while creating directories, before copying a backup,
before each replacement or removal, and before pruning a directory. This
reduces path-swap and time-of-check/time-of-use exposure. It cannot stop a
process running as the **same local account** from changing files after the
last check or after an operation has returned.

## State Is Data, Not Shell

Installation state is parsed as **inert tab-separated data** and is **never sourced or evaluated**. The Coordinator's Workflow State uses a separate
schema and namespace; neither state form is executable input.

The parser accepts only known record types and cardinalities, safe fields,
absolute recorded paths, numeric checksum pairs, registry-order target rows,
known ownership modes and origins, consistent shared destinations, and a scope
binding that matches the selected physical project. Forged, stale, malformed,
or executable-looking state fails closed with exit 65. `--force` cannot
override an invalid state schema.

State files and backup artifacts are installed with mode `600`. Operation
backup directories use mode `700`. These permissions reduce accidental
cross-user disclosure, but backups can contain user instruction files and must
still be treated as sensitive local data.

## Prepare and Apply

During the **prepare phase**, OAW renders prospective content, parses all
relevant state, verifies drift and ownership, resolves shared destinations,
and builds every file and directory action before managed writes begin. A
failure in a later target therefore prevents an earlier target from being
written during preflight.

The apply path performs **apply revalidation** against the allowed root and
expected relative suffix. Replacements use a temporary file beside the target,
set the declared mode, revalidate again, and then `mv`, providing **atomic replacement per destination**. This is **not operation-wide atomicity**:
several destinations are not one filesystem transaction, and OAW promises no
automatic rollback after a later apply failure.

Dry-run performs preparation and reports actions but creates no managed files,
state, backups, or directories. A dry-run is not a lock; the real command
repeats validation.

## Force and Backups

`--force` is a narrow recovery mechanism for drift whose prior ownership can
still be established. It does not adopt an untracked owned file, bypass a
symlink or containment failure, accept malformed state, or guess between
ambiguous marker layouts.

Before an eligible forced update or uninstall mutates anything, OAW collects
every affected existing policy, target, and state artifact. It creates an
operation-scoped backup, copies each artifact with mode `600`, compares source
and backup checksums, writes `manifest.tsv`, and rechecks source bytes before
apply. Each `artifact` row records the original absolute path, backup path, and
checksum.

If marker ownership is ambiguous, OAW creates a recovery backup when possible
and exits 65 with **manual recovery** required. It does not choose which user
bytes to delete. Users restore from backups manually by reading `manifest.tsv`;
the manifest is data and must never be executed or sourced.

## Exact Uninstall Ownership

Uninstall removes only a clean recorded managed block or a clean recorded
owned file. It preserves surrounding user bytes and does not remove a drifted
artifact without an eligible forced operation. Directories are removable only
when state records that OAW actually created them, they still resolve beneath
the allowed root, and they are empty after planned file removals.

## Core, Coordinator, and Host Security Boundary

Provider authority follows this exact chain:

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

OAW Core accepts secret-free facts and compiles a Lifecycle Bundle. The
Workflow Coordinator records only secret-free Workflow State, cooperating
clients, logical workflow authority, and opaque digest references. It must not
store API keys, tokens, raw Provider output, private Hook payloads, or full MCP
or Plugin configuration.

The Agent Host owns physical execution authority. The Host sandbox and
approvals, model route, authentication, tools, MCP, Hooks, Skills, and Plugins
remain Host-owned. A Capability Grant or Resource Lease may be narrower than
the Host sandbox and approvals, but it cannot physically stop an out-of-protocol
Host action.

`CURRENT` uses the active Host session unchanged. `SUBAGENT` is available only
when that Host session exposes a native child-agent facility.

OAW never starts a model CLI. A `policy` integration distributes instructions
only. A `host-native` integration may report session facts and Receipts, but
OAW never guarantees MCP, Hook, Skill, or Plugin inheritance into a `SUBAGENT`;
the active Host reports whether each surface is `inherited`,
`host-configured`, `restricted`, `unknown`, or `unavailable`.

Host session changes invalidate stale Dispatch Packets. OAW requires a fresh
Host report and Bundle eligibility check before continuing. It never reconstructs
a missing child environment or silently falls back to a new process.

## Codex Host Bridge Boundary

Codex has a policy integration by default and a separate audited host-native
Bridge that must be explicitly installed and trusted. Bridge v2 supports
`CURRENT`; its default binding observation proves only `skill` bindings, while
the cooperative `SubagentStart` callback path below may additionally report
`child-delegation`. It does not create a child session or promise inherited MCP,
Hook, Skill, Plugin, model, authentication, sandbox, or approval behavior beyond
stable Host observations.

Trusted `PreToolUse` Hook input is the only current-session identity source.
The Agent cannot author or replace reserved `_oaw_host_context`.
`observe_current` remains the only operation that creates current-session
evidence and issues a handle. `core_inspect` and `core_compile` retain normal
Host approval behavior. `workflow_exchange` also retains normal approval: its
Hook validates the handle/session/CWD and emits no output, so it never rewrites
or automatically allows the mutable Coordinator call. Every operation fails
closed on a session or working-directory mismatch.

`SubagentStart` feature evidence has a narrower cooperative trust contract. The
documented Hook payload contains no signature, Host-issued nonce, or parent
tool-use correlation identifier. A hand-authored `SubagentStart` JSON object
with copied Host fields is indistinguishable from a genuine callback to the
Bridge CLI and can create the same short-lived record. Closed parsing, exact
session/CWD binding, bounded TTL, mode `600`, and record validation remain in
force, but they do not authenticate provenance or resist a malicious same-user
process.

Workflow recovery separates authority freshness from Dispatch convergence.
The opaque handle's bounded process-local entry stores the trusted session ID
and exact CWD, but not turn, tool-use, model, or permission metadata. `PREPARE`
uses those internal coordinates to re-observe and revalidate the stable
reporter identity, current authority facts, and features required by the
current unit before issuing a new Grant. Recovery
commands remain reachable after short-lived feature drift. A Receipt for an
already issued Dispatch must retain its original pins and come from the same
stable reporter identity; a caller-provided cancellation boolean cannot release
an active Grant or Resource Lease.

`skills/list` is the required v2 Skill-observation authority. `hooks/list` and
the allowlisted `config/read` projection are optional environment observations;
these three methods are the closed metadata allowlist. `plugin/list` is not a
production dependency. Filesystem detection, Descriptor declarations, user
configuration, prompts, and Skill self-reports cannot create Host Binding
Evidence.

Verification covers both the exact enabled Skill file and the complete Binding
tree below the exact Host Installation, compared with the independently pinned
Distribution content tree. Same-name, shared-ancestor, disabled, orphan,
ambiguous, symlinked, partial-hash, or drifting evidence fails closed. Skills,
Claude custom Agents, Codex Roles, Instructions, Hooks, and tools remain
separate surfaces.

The Bridge stores an opaque session-bound handle in bounded process memory and
returns only secret-free summaries. It does not retain raw Hook commands,
credentials, MCP environment values, headers, tokens, arbitrary Plugin
settings, or full App Server configuration. Handles must not enter Workflow
State, evidence artifacts, logs, tickets, or screenshots, and cannot be reused
after a Bridge restart, session change, CWD change, expiry, or eviction.

Public Bridge inputs exclude user authorization, explicit invocation
attestation, and gate attestation. Only current Host evidence can supply those
facts. Unattested delegation or `workspace.prepare-or-confirm`,
`verification.execute`, and `closeout.execute` actions remain unavailable.
`PREPARE` compares the active Bundle's pinned authority facts and current-unit
features before issuing executable state. `RECEIPT` preserves Dispatch
convergence through the stable reporter identity, while recovery commands
remain reachable after short-lived drift. Old Workflow records, edited handles,
unknown fields, trailing values, and caller-forged authority are rejected
before effects.

This is a cooperation boundary, not operating-system isolation. A process with
the same user authority can interfere with local programs, files, or process
I/O. OAW validates protocol records but cannot authenticate or contain every
same-user process.

## Out of Scope

The installer and policy protocol cannot protect against:

- malicious shell code in the selected checkout;
- an operating-system or **same local account** compromise;
- unrelated software modifying allowed roots after validation;
- a Provider loader ignoring instructions or applying undocumented precedence;
- a model failing to follow the installed policy;
- manual restoration to the wrong path or from an unverified backup.

Use isolated roots for testing, inspect every forced dry-run, retain stderr and
the reported backup path, and stop if ownership is unclear. Report suspected
vulnerabilities through the private process in the
[security policy](../../SECURITY.md), without putting exploit details or local
configuration in a public issue.

## Canonical Security Terms

The bilingual contract intentionally retains these exact terms:

```text
logical workflow authority
Host sandbox and approvals
secret-free
opaque digest
cooperating clients
OAW never starts a model CLI
```
