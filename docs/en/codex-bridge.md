# Codex Assurance Bridge

[简体中文](../zh/codex-bridge.md) | [README](../../README.md)

`oaw-bridge` is an optional, separately built Codex integration. It observes
current Codex Skill Bindings and asks the standalone Assurance module to issue
an Assurance Overlay for one source-qualified Markdown Profile.

The default `oaw` executable and installer do not import, build, install,
start, update, check, or uninstall Bridge. A missing, revoked, failed, or
incomplete Bridge removes only the optional machine claim. It does not change
whether an Agent can select and follow a Markdown Profile through the normal
rule-driven Policy path.

Bridge installation does not activate OAW. Use it only for an active OAW
Engagement or an explicit audit request that needs current Binding evidence.

## Build and Commands

Build the standalone executable from a trusted checkout:

```bash
go build -o ./oaw-bridge ./cmd/oaw-bridge
```

Its complete command surface is:

```text
oaw-bridge serve codex
oaw-bridge hook codex
oaw-bridge install codex [--dry-run] [--format text|json]
oaw-bridge update codex [--dry-run] [--format text|json]
oaw-bridge check codex [--format text|json]
oaw-bridge uninstall codex [--format text|json]
```

`oaw bridge ...` is intentionally unavailable. The default CLI directs an
explicit Bridge user to `oaw-bridge` and performs no Bridge management.

## Installation Lifecycle

Preview before mutation:

```bash
./oaw-bridge install codex --dry-run --format json
./oaw-bridge install codex --format json
./oaw-bridge check codex --format json
```

The installation owns only these roots:

```text
${XDG_DATA_HOME:-$HOME/.local/share}/open-agent-workflow/codex-bridge/
${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/codex-bridge/
```

The data payload contains `bin/oaw-bridge`, a local marketplace named
`oaw-local`, and one Plugin named `oaw-codex-assurance`:

```text
marketplace/.agents/plugins/marketplace.json
marketplace/plugins/oaw-codex-assurance/.codex-plugin/plugin.json
marketplace/plugins/oaw-codex-assurance/.mcp.json
marketplace/plugins/oaw-codex-assurance/hooks/hooks.json
marketplace/plugins/oaw-codex-assurance/skills/oaw-codex-bridge/SKILL.md
```

Installation and update use exact Codex Plugin commands, stage files before
publication, and record owned file digests and modes. Dry-run is non-mutating
and does not invoke Codex. Uninstall removes only recorded clean files; drifted
files are preserved and reported.

`check` proves installation integrity only. Its text output includes:

```text
proof_scope: installation-integrity
live_protocol_proof: false
current_session_loaded: false
```

`current_session_loaded: false` means the management command did not inspect a
live Agent session. A new session may be required after install, update, or
uninstall so Codex can reload Plugin configuration.

## Bridge v3 Protocol

Bridge v3 exposes exactly one MCP operation:

```text
observe_profile
```

The public input contains one source-qualified selector such as
`project:team-delivery`. The exact PreToolUse matcher is:

```text
mcp__oaw_codex_bridge__observe_profile
```

The Hook accepts only that matcher, preserves the selector, and injects a
private `_oaw_host_context` using `oaw.codex-hook-context/v3`. The MCP service
accepts only `oaw.codex-bridge/v3`. Caller-supplied reserved context, another
Hook event, another tool, malformed input, or a mismatched working directory
fails closed.

The service reads the same source-qualified Profile through
`internal/profileinspect`, observes current Codex `skills/list` metadata,
matches exact Provider installation and Binding content, and calls
`internal/assurance`. The result contains an `oaw.assurance-overlay/v1`
artifact bound to the full Markdown content digest and its declared Binding
occurrences.

The dependency direction is one-way:

```text
oaw-bridge -> assurance -> profileinspect -> Markdown Profile
```

Bridge does not own Profile Responsibilities, Rules, ordering, selection,
Request Mode, Risk, topology, progress, review, verification, or completion.
It does not call OAW Core or the Workflow Coordinator and does not produce a
Lifecycle Bundle, Capability Grant, Resource Lease, Dispatch Packet, Receipt,
or Workflow State.

## Failure and Recovery

| Reason | Recovery |
| --- | --- |
| `HOST_BRIDGE_UNAVAILABLE` | Check the standalone installation and Codex Plugin status. Continue without machine assurance when the Overlay is optional. |
| `HOST_BRIDGE_CONTEXT_REQUIRED` | Verify the exact PreToolUse matcher and start a session that loaded the installed Plugin. Do not hand-author reserved context. |
| `HOST_BRIDGE_PROTOCOL_MISMATCH` | Update the standalone component and reload Codex. Do not translate v1 or v2 records. |
| `HOST_OBSERVATION_FAILED` | Repair current `skills/list` access or continue without the optional claim. |
| `HOST_OBSERVATION_PARTIAL` | Treat only the affected Binding claims as unavailable; do not infer missing evidence. |
| `PROFILE_NOT_FOUND` or `PROFILE_AMBIGUOUS` | Use an existing source-qualified selector such as `project:<id>` or `user:<id>`. |
| `ASSURANCE_BINDING_UNAVAILABLE` | Install or enable the exact declared Skill Binding, or use the Profile without the optional Overlay. |

Never edit an Overlay or installation state to bypass a diagnostic. Re-observe
the current Profile and Host after repairing the underlying installation.

## Security Boundary

Bridge is not a sandbox, process supervisor, workflow runtime, permission
grant, or proof that a Skill was invoked correctly. It launches only the
official local Codex App Server needed for read-only `skills/list` metadata and
never starts an Agent or model process. Codex and the Agent Host retain all
physical execution authority, sandbox, approval, authentication, and tool
decisions.

Hook context is cooperative Host input, not a cryptographic signature. The
Overlay is a content-addressed identity claim, not completion evidence. Keep
credentials, transcripts, prompts, private configuration, and raw Host output
out of Bridge state and diagnostics.

Run the deterministic checks with:

```bash
bash scripts/check-codex-bridge.sh
bash tests/17-codex-bridge-management-test.sh
bash tests/18-codex-bridge-protocol-test.sh
```
