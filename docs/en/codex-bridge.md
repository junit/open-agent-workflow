# Codex Host Bridge

[Chinese](../zh/codex-bridge.md) | [Installer](installer.md) | [Architecture](architecture.md) | [Security](security.md)

The Codex Host Bridge is an explicit, audited Codex Plugin integration for Open Agent Workflow (OAW). It is separate from the policy adapter. The policy adapter distributes `ENGINEERING.md`; the Bridge adds an opt-in MCP server, four narrowly scoped `PreToolUse` Hooks, and one official `SubagentStart` Hook for live child-delegation evidence.

The Bridge is a Host integration, not a second runtime. The active Codex session invokes Skills and tools and performs every physical effect. OAW observes secret-free Host metadata, compiles policy, and exchanges Coordinator records. OAW does not start a child session, execute a lifecycle Capability, invoke a model, or reconstruct the Codex environment.

Bridge installation does not activate OAW. Filesystem evidence, a registered
Plugin, and the availability of `observe_current` are readiness facts only.
Bridge and Core operations may begin only after the current top-level user
request creates an active OAW Engagement. Assurance Preflight may then use the
Bridge to establish `core-backed` or `coordinator-backed` support; before that,
ordinary requests and ordinary Skill routing remain Native Host behavior.

## Command Surface

Policy installation and Bridge installation are different actions:

```text
oaw install --target codex
oaw bridge install codex
oaw bridge check codex
oaw bridge update codex
oaw bridge uninstall codex
oaw bridge serve codex
oaw bridge hook codex
```

`oaw install --target codex` is the policy-only path. It does not install an executable Plugin or claim Host-native evidence. `oaw bridge install codex` is the explicit executable Plugin transaction and requires review and trust of the exact Hook definition.

Management commands use the current working directory as the physical project root and the current user `codex` executable. Use `--format json` with `check`, `install`, `update`, or `uninstall` for a machine-readable management projection. `install` and `update` also accept `--dry-run`.

`bridge serve codex` is started by the Plugin MCP configuration. `bridge hook codex` is called by the Plugin Hook with one JSON Hook event on stdin. Neither command is a general shell or model launcher.

## Installation And Trust

Before installing, inspect policy-only state and the current checkout:

```bash
oaw install --target codex
oaw bridge check codex --format json
```

Install only after deciding that the current Codex Host is trusted to provide the reported facts:

```bash
oaw bridge install codex --format json
oaw bridge check codex --format json
```

`bridge check` is installation management only. Text output explicitly reports:

```text
proof_scope: installation-integrity
live_protocol_proof: false
```

It proves managed files and Codex registration, not the protocol loaded by the
active session. Only a fresh `observe_current` call negotiates live protocol
evidence. Stop before Workflow START if `check` reports installation drift, a
real installed-version mismatch, or an installation-authority mismatch. A
management-only `requires_new_session` value is operator advice, not live
negative evidence, and does not block a fresh `observe_current` call in the
same active session. A successful `observe_current` response with canonical
VersionEvidence is authoritative for the active session. If live observation
instead returns `HOST_BRIDGE_PROTOCOL_MISMATCH` or another version/authority
diagnostic, stop; update and session replacement are separate operator actions.

The transaction renders a local marketplace named `oaw-local`, a Plugin named `oaw-codex-host`, and a checksum-pinned copy of the running OAW binary. Review all five rendered files before trusting the Plugin:

```text
.agents/plugins/marketplace.json
plugins/oaw-codex-host/.codex-plugin/plugin.json
plugins/oaw-codex-host/.mcp.json
plugins/oaw-codex-host/hooks/hooks.json
plugins/oaw-codex-host/skills/oaw-codex-bridge/SKILL.md
```

The Plugin manifest must point exactly to `./skills/`, `./.mcp.json`, and `./hooks/hooks.json`. The MCP map must invoke the OAW binary directly with `bridge serve codex`. The Hook file must contain exactly one `PreToolUse` matcher for each generated tool and exactly one `SubagentStart` matcher of `*`:

```text
mcp__oaw_codex_bridge__observe_current
mcp__oaw_codex_bridge__core_inspect
mcp__oaw_codex_bridge__core_compile
mcp__oaw_codex_bridge__workflow_exchange
```

The `SubagentStart` Hook is the only live callback path the Bridge currently
uses to record `child-delegation`. Its record is cooperative same-user
evidence, not authenticated proof of Hook provenance. The documented Codex
payload contains no signature, Host-issued nonce, or parent tool-use
correlation identifier, so `oaw bridge hook codex` cannot distinguish a
genuine Host callback from hand-authored JSON with copied Host fields. The
record contains no prompt, transcript, agent identifier, model output, or Host
handle, and emits no model-facing output. It proves neither parallel nor nested
delegation and does not make a Profile or topology selection.

Open Codex `/hooks` and review the exact four matchers, the `SubagentStart`
matcher, and the command path. Do not
use a trust-bypass flag. Starting a new Codex session is the normal action after
installation, but management state alone cannot determine what the active
session loaded. Only a successful fresh `observe_current` response with
canonical VersionEvidence establishes that authority.

## Current Session Contract

Bridge v2 supports only the `CURRENT` topology in the current Codex integration.
The current Codex session is the physical executor. There is no `SUBAGENT`
implementation, shell fallback, container fallback, alternate user home,
projected configuration, or copied MCP, Hook, Skill, Plugin, model,
authentication, sandbox, or approval environment. Current observation proves
`skill` Bindings and `CURRENT`. After a strictly parsed `SubagentStart` callback
for the reported current session/CWD, the next `observe_current` may also report
`child-delegation` as available within this cooperative integration. This is
not cryptographic or operating-system attestation. Role, instruction, agent,
tool, parallel/nested delegation, and Host-action facts remain unknown or
unavailable.

The canonical live negotiation tuple is:

```text
Bridge integration 2.0.0
oaw.codex-bridge/v2
oaw.codex-hook-context/v2
oaw.host-evidence-handle/v2
oaw.provider-descriptor/v4
oaw.profile-recipe/v3
oaw.host-manifest/v3
oaw.host-session/v3
oaw.host-binding-inventory/v3
oaw.host-environment-report/v2
oaw.host-invocation-receipt/v3
oaw.host-conformance-transcript/v4
oaw.host-conformance-report/v4
oaw.execution-graph/v4
oaw.lifecycle-bundle/v4
oaw.capability-grant/v3
oaw.dispatch-packet/v2
oaw.workflow-command/v2
oaw.workflow-result/v2
oaw.workflow-snapshot/v2
oaw.workflow-revision/v2
```

The VersionEvidence digest covers the full normalized tuple independently.
Any missing, duplicate, non-canonical, stale, or mismatched field returns
`HOST_BRIDGE_PROTOCOL_MISMATCH` before Core or Coordinator authority is used.
A successful `observe_current` response with canonical VersionEvidence is
authoritative for the active session; management-only `requires_new_session`
advice cannot override that live result.

Only `observe_current` creates current-session evidence and issues a handle. Its
Hook is strictly read-only and injects reserved context after Codex creates the
public tool input. The semantic output is the official nested envelope:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "updatedInput": {
      "_oaw_host_context": {
        "schema_version": "oaw.codex-hook-context/v2",
        "bridge_protocol_version": "oaw.codex-bridge/v2",
        "session_id": "<opaque Host value>",
        "turn_id": "<opaque Host value>",
        "tool_use_id": "<opaque Host value>",
        "cwd": "/absolute/project/root",
        "model": "<diagnostic metadata>",
        "permission_mode": "<diagnostic metadata>"
      }
    }
  }
}
```

The actual output preserves the public tool input and adds `_oaw_host_context`; the example omits unrelated public fields. A caller may not author or replace that reserved field.

The other three Hook matchers validate `host_evidence_handle`, exact session,
and exact working directory. Every valid later operation emits zero stdout
bytes so the normal Codex MCP approval policy remains active. The Hook never
rewrites or automatically allows the mutable `workflow_exchange` call. A
missing, edited, expired, foreign, or malformed handle returns the nested deny
form:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny"
  }
}
```

Every invocation must have `hook_event_name == PreToolUse`. A wrong event,
missing context, or foreign session fails closed. Only the strictly read-only
`observe_current` rewrite receives automatic Hook `allow`.

## Observation And Workflow

In a trusted active session, use the Bridge operations in this order:

```text
observe_current
core_inspect
core_compile
workflow_exchange
```

`observe_current` returns a short Host summary and an opaque `HostEvidenceHandle`. The handle is a cache reference, not a credential, Lifecycle Bundle field, or durable evidence artifact. Do not copy it into Workflow State, logs, tickets, or reports.

`core_inspect` resolves Host-scoped Provider Candidates and exact enabled Skill
Bindings. `skills/list` is required; `hooks/list` and `config/read` are optional,
and these three methods are the complete metadata allowlist. A Skill verifies
only when its exact enabled name/path resolves below one exact discovered
Provider installation and its complete Binding tree matches the pinned
Distribution digest. Disabled, missing, orphaned, ambiguous, same-name foreign,
shared-ancestor, symlinked, or drifting trees never create authority.

`core_compile` requires the exact confirmation digest for the user's explicit
Profile, `CURRENT`, and Add-on selection. Inspection has no implicit selection.
The Startup Gate still applies to `WORKFLOW`; Bridge observation only supplies
Host evidence. `workflow_exchange` accepts only Workflow Command v2 and returns
only Workflow Result/Snapshot/Revision v2 containing Bundle/Graph v4.

If the user explicitly requests a Profile/topology and its only inspection
blocker is the reviewer child's `child-delegation` requirement, the Startup
Gate may execute one zero-project-effect native child capability probe. The
child may only report that it started and terminate; it cannot read or write
project resources, invoke a Provider Capability, perform review, select the
Profile, or create Workflow State. Treat it only as a Governance observation,
wait for the expected `SubagentStart` Hook callback, then call
`observe_current` again and repeat `core_inspect`. A missing, foreign, stale,
or malformed callback keeps `child-delegation` unavailable. A syntactically
valid hand-authored callback with copied current session/CWD fields is
indistinguishable from the Host callback and can create the same record.
Therefore this probe is cooperative evidence only; do not describe it as
synthetic-resistant or cryptographically attested. Never replace the probe
with static `agents.enabled`, prompt text, a shell/model fallback, or
self-review.

Public callers cannot provide user authorization, explicit invocation
attestation, or gate attestation. The Bridge hydrates those Host-owned facts or
fails closed. The opaque handle's process-local entry retains only the trusted
session ID and exact CWD needed for freshness checks; it does not retain turn,
tool-use, model, or permission metadata. Before `PREPARE`, the Bridge uses those
internal coordinates to re-observe live facts and checks the stable reporter
identity plus current environment, inventory, actions, configuration,
resolution, registry, and the current graph unit's required live features
before issuing new authority.
`INSPECT`, `SWITCH`, and `CANCEL` remain reachable
for recovery when short-lived facts change. An already committed Dispatch may
accept its matching Receipt after authority drift, but only from the same
stable reporter identity and with the original Dispatch/session/environment
pins. A caller-provided cancellation flag never releases an active Grant or
Lease; a Dispatch-bound `CANCELLED` Receipt is required.

Direct Mode remains available when the Bridge is absent or unavailable. A Bridge failure must not turn a small bounded change into a Host-native claim.

## Ownership And Rollback

OAW owns only:

- Bridge install state below `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/codex-bridge`;
- the managed binary and local marketplace below `${XDG_DATA_HOME:-$HOME/.local/share}/open-agent-workflow/codex-bridge`;
- rendered Plugin source files inside that local marketplace.

Codex owns its Plugin cache, enablement configuration, approvals, and other Host state. OAW never edits Codex config or cache directly. It invokes only official `codex plugin marketplace` and `codex plugin` command surfaces through a fixed argument vector.

Installation and update are transactional. A failure rolls back OAW-owned files and uses official Codex removal for registrations created by the failed operation. Drift is preserved rather than overwritten. Update requires clean, exactly recorded OAW payload and matching Codex registration. Uninstall first performs official Plugin and marketplace removal, then deletes only clean recorded OAW files and state. Unrelated files, user edits, symlinks, and unrecorded payload entries are retained and reported.

Inspect before changing anything:

```bash
oaw bridge check codex --format json
oaw bridge update codex --dry-run --format json
oaw bridge uninstall codex --format json
```

Install or update may report that a new Codex session is required. Treat that
as management advice unless installation drift or a real version/authority
mismatch exists. A canonical successful `observe_current` result remains
authoritative for the active session. After uninstall, start a new session and
verify that only the policy-only surface remains; a management check never
claims that the active session loaded the Bridge.

## Diagnostics And Recovery

Host integration diagnostics are distinct from Provider and Profile decisions. The following codes are stable v2 reasons. Every recovery action is explicit; none silently selects a Profile or changes Host authority.

| Code | Meaning | Recovery |
| --- | --- | --- |
| `HOST_BRIDGE_UNAVAILABLE` | Plugin or MCP Bridge is unavailable in this session. | Run `oaw bridge check codex --format json`, then install or enable the Plugin and start a new session. |
| `HOST_BRIDGE_CONTEXT_REQUIRED` | The trusted Hook did not inject valid context. | Open Codex `/hooks`, review the four exact tool matchers plus `SubagentStart`, trust them, then start a new session. |
| `HOST_BRIDGE_PROTOCOL_MISMATCH` | Plugin, Hook, Bridge, Core, or Host protocol versions differ. | Run `oaw bridge update codex`, review the Hook again, and start a new session. |
| `HOST_EVIDENCE_HANDLE_REQUIRED` | A later operation omitted its current handle. | Call `observe_current` in the active session, then retry the operation. |
| `HOST_EVIDENCE_HANDLE_INVALID` | The handle is malformed, edited, unknown, evicted, or from a restarted Bridge. | Call `observe_current` and use only the returned handle. |
| `HOST_EVIDENCE_EXPIRED` | The handle exceeded its bounded TTL. | Call `observe_current` again before `core_inspect` or `core_compile`. |
| `HOST_EVIDENCE_SESSION_MISMATCH` | The handle belongs to another session or working directory. | Stop the stale exchange; call `observe_current` in the current session and recompile. |
| `HOST_OBSERVATION_FAILED` | Required stable metadata observation failed. | Run `oaw bridge check codex --format json`; repair the local Codex/App Server capability before retrying. |
| `HOST_OBSERVATION_PARTIAL` | Optional environment metadata is incomplete. | Run `core_inspect` again and keep every unavailable field explicitly `unknown`. |
| `HOST_SESSION_CHANGED` | Stable reporter identity changed, or current authority facts no longer support a new Dispatch. | Existing recovery commands remain available. Converge an already issued Dispatch with its matching Receipt from the original reporter; otherwise observe again, return to the Startup Gate, and compile a new generation before `PREPARE`. |

Existing layers retain their own reasons, including `HOST_BINDING_EVIDENCE_REQUIRED`, `HOST_BINDING_INVENTORY_INVALID`, `PROVIDER_CANDIDATE_AMBIGUOUS`, `PROVIDER_PIN_INCOMPATIBLE`, `PROVIDER_BINDING_UNAVAILABLE`, and `PROFILE_TOPOLOGY_UNAVAILABLE`.

Management diagnostics include `BRIDGE_INSTALL_NOT_INSTALLED`, `BRIDGE_INSTALL_DRIFT`, `BRIDGE_INSTALL_STATE_INVALID`, `BRIDGE_INSTALL_AUTHORITY_MISMATCH`, and `BRIDGE_INSTALL_ROLLBACK_INCOMPLETE`. Run `oaw bridge check codex --format json` first. Do not use a force override for unsafe state, symlink containment, or unrecorded files; preserve the exact files and state for manual recovery.

## Security Boundary

The Hook is the only current-session identity source. A Skill instruction,
prompt, filesystem candidate, or user-authored input cannot substitute for it.
`skills/list` is the required v2 Skill-observation authority; `plugin/list` is
not a production dependency. Raw Hook commands, credentials, MCP environment
values, headers, tokens, arbitrary Plugin settings, and App Server
configuration are not retained.

The Bridge uses the normal current user environment to query metadata; it does
not create an alternate user home or projected environment. A malicious process
running as the same user can interfere with local programs or invoke the Hook
command with copied fields. This is a cooperative Host integration, not an
operating-system authentication or isolation boundary.

Do not publish handles, absolute Skill paths, raw Hook commands, or credentials
in tickets, Workflow State, evidence, logs, or screenshots. A handle is bound
to one live Bridge process, session, and CWD; never persist or reuse it across
a restart, session, or working directory. Keep local dogfood records under
`.scratch/oaw-codex-host-bridge-dogfood/` and untracked.

## Uninstall

After a controlled pilot or when the Bridge is no longer trusted:

```bash
oaw bridge uninstall codex --format json
oaw bridge check codex --format json
```

Uninstall invokes official Codex Plugin removal before removing the OAW-owned local marketplace and clean payload. Drifted or unrecorded files remain. Start a fresh Codex session and confirm that the policy-only surface does not report current-session evidence. There is no compatibility fallback to an old Runner or a projected Host environment.
