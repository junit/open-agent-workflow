# Codex Host Bridge

[Chinese](../zh/codex-bridge.md) | [Installer](installer.md) | [Architecture](architecture.md) | [Security](security.md)

The Codex Host Bridge is an explicit, audited Codex Plugin integration for Open Agent Workflow (OAW). It is separate from the policy adapter. The policy adapter distributes `ENGINEERING.md`; the Bridge adds an opt-in MCP server and four narrowly scoped `PreToolUse` Hooks.

The Bridge is a Host integration, not a second runtime. The active Codex session invokes Skills and tools and performs every physical effect. OAW observes secret-free Host metadata, compiles policy, and exchanges Coordinator records. OAW does not start a child session, execute a lifecycle Capability, invoke a model, or reconstruct the Codex environment.

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

The transaction renders a local marketplace named `oaw-local`, a Plugin named `oaw-codex-host`, and a checksum-pinned copy of the running OAW binary. Review all five rendered files before trusting the Plugin:

```text
.agents/plugins/marketplace.json
plugins/oaw-codex-host/.codex-plugin/plugin.json
plugins/oaw-codex-host/.mcp.json
plugins/oaw-codex-host/hooks/hooks.json
plugins/oaw-codex-host/skills/oaw-codex-bridge/SKILL.md
```

The Plugin manifest must point exactly to `./skills/`, `./.mcp.json`, and `./hooks/hooks.json`. The MCP map must invoke the OAW binary directly with `bridge serve codex`. The Hook file must contain exactly one `PreToolUse` matcher for each generated tool:

```text
mcp__oaw_codex_bridge__observe_current
mcp__oaw_codex_bridge__core_inspect
mcp__oaw_codex_bridge__core_compile
mcp__oaw_codex_bridge__workflow_exchange
```

Open Codex `/hooks` and review the exact four matchers and command path. Do not use a trust-bypass flag. Start a new Codex session after installation; the previous session cannot be reported as having loaded the new Plugin.

## Current Session Contract

Bridge v1 supports only the `CURRENT` topology. The current Codex session is the physical executor. There is no `SUBAGENT` implementation, shell fallback, container fallback, alternate user home, projected configuration, or copied MCP, Hook, Skill, Plugin, model, authentication, sandbox, or approval environment. An unavailable Host surface remains `unknown` unless Codex reports stable, allowlisted evidence.

Only `observe_current` can establish current-session identity. Its Hook is strictly read-only and injects reserved context after Codex creates the public tool input. The semantic output is the official nested envelope:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "updatedInput": {
      "_oaw_host_context": {
        "schema_version": "oaw.codex-hook-context/v1",
        "bridge_protocol_version": "oaw.codex-bridge/v1",
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

The other three Hook matchers validate `host_evidence_handle`, exact session, and exact working directory. A valid later operation emits zero stdout bytes so the normal Codex MCP approval policy remains active. A missing, edited, expired, foreign, or malformed handle returns the nested deny form:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny"
  }
}
```

Every invocation must have `hook_event_name == PreToolUse`. A wrong event, missing context, or foreign session fails closed. Only the read-only observation rewrite may receive an automatic `allow` decision.

## Observation And Workflow

In a fresh trusted session, use the Bridge operations in this order:

```text
observe_current
core.inspect
core.compile
workflow_exchange
```

`observe_current` returns a short Host summary and an opaque `HostEvidenceHandle`. The handle is a cache reference, not a credential, Lifecycle Bundle field, or durable evidence artifact. Do not copy it into Workflow State, logs, tickets, or reports.

`core.inspect` resolves Host-scoped Provider Candidates and exact enabled Skill bindings. v1 uses `skills/list` as the Provider binding authority. A Skill verifies only when its exact name and normalized path map to exactly one discovered Provider installation and Capability. Disabled, missing, orphaned, ambiguous, or foreign-Host Skills never create authority. Superpowers, Matt, ECC, and user Providers use the same matching algorithm; no brand exception promotes a Candidate.

`core.compile` accepts the explicit Profile and `CURRENT` selection. The Startup Gate still applies to `WORKFLOW`; Bridge observation only supplies Host evidence and never selects a Profile. `workflow_exchange` exchanges the compiled Bundle and Coordinator records. Codex remains responsible for executing selected Skills and tools.

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

After install or update, start a new Codex session. After uninstall, start a new session and verify that only the policy-only surface remains; a management check never claims that the active session loaded the Bridge.

## Diagnostics And Recovery

Host integration diagnostics are distinct from Provider and Profile decisions. The following codes are stable v1 reasons. Every recovery action is explicit; none silently selects a Profile or changes Host authority.

| Code | Meaning | Recovery |
| --- | --- | --- |
| `HOST_BRIDGE_UNAVAILABLE` | Plugin or MCP Bridge is unavailable in this session. | Run `oaw bridge check codex --format json`, then install or enable the Plugin and start a new session. |
| `HOST_BRIDGE_CONTEXT_REQUIRED` | The trusted Hook did not inject valid context. | Open Codex `/hooks`, review and trust the four exact matchers, then start a new session. |
| `HOST_BRIDGE_PROTOCOL_MISMATCH` | Plugin, Hook, Bridge, Core, or Host protocol versions differ. | Run `oaw bridge update codex`, review the Hook again, and start a new session. |
| `HOST_EVIDENCE_HANDLE_REQUIRED` | A later operation omitted its current handle. | Call `observe_current` in the active session, then retry the operation. |
| `HOST_EVIDENCE_HANDLE_INVALID` | The handle is malformed, edited, unknown, evicted, or from a restarted Bridge. | Call `observe_current` and use only the returned handle. |
| `HOST_EVIDENCE_EXPIRED` | The handle exceeded its bounded TTL. | Call `observe_current` again before `core.inspect` or `core.compile`. |
| `HOST_EVIDENCE_SESSION_MISMATCH` | The handle belongs to another session or working directory. | Stop the stale exchange; call `observe_current` in the current session and recompile. |
| `HOST_OBSERVATION_FAILED` | Required stable metadata observation failed. | Run `oaw bridge check codex --format json`; repair the local Codex/App Server capability before retrying. |
| `HOST_OBSERVATION_PARTIAL` | Optional environment metadata is incomplete. | Run `core.inspect` again and keep every unavailable field explicitly `unknown`. |
| `HOST_SESSION_CHANGED` | Facts pinned by the active Bundle changed. | Pause the Workflow, call `observe_current`, and pass the Startup Gate again before compiling. |

Existing layers retain their own reasons, including `HOST_BINDING_EVIDENCE_REQUIRED`, `HOST_BINDING_INVENTORY_INVALID`, `PROVIDER_CANDIDATE_AMBIGUOUS`, `PROVIDER_PIN_INCOMPATIBLE`, `PROVIDER_BINDING_UNAVAILABLE`, and `PROFILE_TOPOLOGY_UNAVAILABLE`.

Management diagnostics include `BRIDGE_INSTALL_NOT_INSTALLED`, `BRIDGE_INSTALL_DRIFT`, `BRIDGE_INSTALL_STATE_INVALID`, `BRIDGE_INSTALL_AUTHORITY_MISMATCH`, and `BRIDGE_INSTALL_ROLLBACK_INCOMPLETE`. Run `oaw bridge check codex --format json` first. Do not use a force override for unsafe state, symlink containment, or unrecorded files; preserve the exact files and state for manual recovery.

## Security Boundary

The Hook is the only current-session identity source. A Skill instruction, prompt, filesystem candidate, or user-authored input cannot substitute for it. `skills/list` is the only v1 Provider binding authority. `plugin/list` is not a production dependency. Raw Hook commands, credentials, MCP environment values, headers, tokens, arbitrary Plugin settings, and App Server configuration are not retained.

The Bridge uses the normal current user environment to query metadata; it does not create an alternate user home or projected environment. A malicious process running as the same user can interfere with local programs. This is a cooperative Host integration, not an operating-system authentication or isolation boundary.

Do not publish handles, absolute Skill paths, raw Hook commands, or credentials in tickets, Workflow State, evidence, logs, or screenshots. Keep local dogfood records under `.scratch/oaw-codex-host-bridge-dogfood/` and untracked.

## Uninstall

After a controlled pilot or when the Bridge is no longer trusted:

```bash
oaw bridge uninstall codex --format json
oaw bridge check codex --format json
```

Uninstall invokes official Codex Plugin removal before removing the OAW-owned local marketplace and clean payload. Drifted or unrecorded files remain. Start a fresh Codex session and confirm that the policy-only surface does not report current-session evidence. There is no compatibility fallback to an old Runner or a projected Host environment.
