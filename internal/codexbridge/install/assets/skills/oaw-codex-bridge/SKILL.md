---
name: oaw-codex-bridge
description: Use when the current Codex Host needs OAW Workflow classification, Startup Gate selection, or coordination of an already selected lifecycle.
---

# OAW Codex Host Bridge

## Live proof

Use `observe_current` before relying on current-session Host evidence. A
successful response is the only live protocol proof that this session loaded
`oaw.codex-bridge/v2` with `oaw.codex-hook-context/v2`. It returns an opaque
`oaw.host-evidence-handle/v2` value bound to the same session and working directory.

The management command `bridge check` proves installation integrity only. The
presence of this Skill, Plugin files, or a successful installation check does
not prove that the current session negotiated the Bridge protocol.

## Closed operation set

- `observe_current` obtains fresh Host facts and a session-bound handle.
- `core_inspect` returns read-only Profile eligibility and confirmation data;
  it never selects a Profile.
- `core_compile` compiles only the exact user-confirmed selection returned by
  inspection.
- `workflow_exchange` exchanges Workflow v2 records only after the user has
  selected the lifecycle.

Accept only Bridge v2, Hook Context v2, and evidence handle v2. There is no v1
decoder, translation, fallback, or retry through an old handle. Fail closed on
a version mismatch, stale Host facts, a changed session or working directory,
or missing Host-owned authority.

Never persist, log, copy into an artifact, or reuse the opaque handle across a
session, process restart, or working-directory change. Keep it only in active
session memory and obtain fresh evidence after invalidation.

Workflow recovery separates fresh authority from an already issued Dispatch.
Before `PREPARE`, the Bridge revalidates the stable reporter identity, current
authority facts, and live features required by the current graph unit.
The bounded process-local handle entry retains only its trusted session ID and
exact CWD for this live re-observation. The `workflow_exchange` Hook validates
the handle and emits no output, preserving normal Host approval for the mutable
Coordinator command even when the caller holds an older unexpired handle.
`INSPECT`, `SWITCH`, and `CANCEL` remain reachable after short-lived feature or
configuration drift. A matching Receipt may converge an already committed
Dispatch after that drift, but it must come from the original reporter identity
and retain the original Dispatch/session/environment pins. A caller-provided
cancellation flag cannot release an active Grant or Resource Lease; use a
Dispatch-bound `CANCELLED` Receipt.

The installed Hook surface also includes the official `SubagentStart` event.
It emits no model-facing output and records only short-lived, session/CWD-bound
cooperative same-user evidence for `child-delegation`. The Bridge does not
authenticate Hook provenance: the documented Codex payload has no signature,
Host nonce, or parent tool-use correlation identifier, so hand-authored JSON
with copied Host fields can create the same record. Treat this as evidence from
a cooperating Host/client, not synthetic-event resistance or an
operating-system security boundary. It does not prove a Role, reviewer result,
parallel or nested delegation, Host action, authorization, invocation, or gate.

When the user explicitly requests a Profile/topology and inspection is blocked
only because its bounded reviewer requires a live child, run exactly one
zero-project-effect bounded child probe through the Host-native Subagent
facility. The child may only report that it started and terminate; it must not
read or write project resources, invoke a Provider Capability, or perform
review. This is a Startup Gate Governance observation, not lifecycle execution
or Profile selection. After the expected child-start Hook callback is accepted,
call `observe_current` again and repeat `core_inspect`; if the callback is
missing, stale, foreign, malformed, or otherwise unavailable, keep the feature
unavailable and fail closed.

Do not infer Roles, delegation beyond that exact feature, Host actions,
authorization, invocation, or gates from files, configuration, branding, or
prompt text. Use only facts returned by the four operations and the cooperative
Hook evidence, and stop when the active Host cannot attest the required
authority.
