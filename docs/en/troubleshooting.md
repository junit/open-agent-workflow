# Troubleshooting

[简体中文](../zh/troubleshooting.md) | [Installer reference](installer.md) |
[Security model](security.md)

Use the same binary, scope, and target selection throughout diagnosis. A
release archive already contains `oaw`; a source checkout must first run
`go build -o ./oaw ./cmd/oaw`. OAW does not fetch releases or repair workflow
providers, and v0.1 management output is human-readable. Preserve the complete
command and stderr when asking for help.

## Safe Diagnostic Sequence

Start read-only:

```bash
./oaw check
# Compatibility wrapper:
./install.sh check
```

For project scope, repeat the exact scope you intend to mutate:

```bash
./oaw check --project /absolute/path --target claude
# Compatibility wrapper:
./install.sh check --project /absolute/path --target claude
```

`check exits 0` after reporting `clean`, `drift`, `invalid-state`, or
`not-installed`. Read the `installed <target>:` lines; exit 0 means the check
completed, not that a later mutation is authorized.

Next preview an existing installation update:

```bash
./oaw update --dry-run
./oaw update --project /absolute/path --target claude --dry-run
# Compatibility wrapper:
./install.sh update --dry-run
./install.sh update --project /absolute/path --target claude --dry-run
```

`./install.sh update --dry-run` performs the same state, ownership, source, and
path preparation as update but writes no managed content, state, backup, or
directory. If no installation state exists, `update exits 66`; preview
`install --dry-run` instead, then use `install` to create the missing target.

Do not add `--force` until you have identified the changed files, understood
which bytes OAW owns, and reviewed the preview. For eligible recorded drift,
use one explicitly scoped command:

```bash
./oaw update --project /absolute/path --target claude --force
# Compatibility wrapper:
./install.sh update --project /absolute/path --target claude --force
```

The exact example above is intentionally narrow. Do not replace the project or
target with a broad guess. A normal drifted **mutation exits 65** before writes;
force is justified only when the recorded ownership and backup are acceptable.

## Read `check` Output

| Output | Meaning | Next action |
| --- | --- | --- |
| `provider <name>: missing` | A **missing provider** was not detected at its expected instruction root. | Install or repair that provider independently, or select a lifecycle bundle that does not require it. OAW never installs providers. |
| `target <id>: detected` | The target tool's instruction root was found. | This is readiness information, not proof that the running agent loaded the adapter. |
| `installed <id>: not-installed` | No valid state row owns this target. | Use `install --dry-run`, then `install`. Do not use update to add it. |
| `installed <id>: clean` | Recorded policy and target ownership match disk. | If behavior is stale, inspect provider loading and restart the target agent. |
| `installed <id>: drift` | Managed bytes, policy, or a recorded file no longer match. | Compare the destination with the state-backed expectation; preview update or uninstall before deciding on force. |
| `installed <id>: invalid-state` | State shape, binding, registry metadata, or shared ownership cannot be trusted. | Do not force. Preserve state and files for manual diagnosis. |

A **missing provider** does not necessarily stop adapter file installation, but
the selected workflow capability will remain unavailable. Provider detection
cannot choose a lifecycle profile or substitute another family.

### Host-scoped Provider diagnosis

Provider authority is built in this order:

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

Run `oaw providers inspect --host codex --format text` with the same
`--project-root`, if the Workflow used one. Codex and Claude Code are independent
Hosts even when they reference shared files. The current section contains only
the selected Host's Candidates and observations. A `policy` Host may show
Candidates, but a Candidate alone is not a Verified Provider Instance. The foreign section is
diagnostic-only and never supplies a pin or authority. Descriptor bindings and
installation hints are declarations, not Host Binding Evidence.

Interpret the stable reasons as follows:

| Reason | Meaning |
| --- | --- |
| `HOST_BINDING_EVIDENCE_REQUIRED` | The selected Host has Candidates but no Host-owned Binding Inventory. |
| `PROVIDER_BINDING_UNAVAILABLE` | Inventory exists but no exact Installation/Capability/Binding observation matches. |
| `PROVIDER_FOREIGN_HOST_ONLY` | A Candidate exists only in a foreign diagnostic Host and remains unusable for current authority. |
| `PROVIDER_PIN_INCOMPATIBLE` | The current-Host pin no longer matches installation identity or evidence. |
| `HOST_PROVIDER_SCOPE_MISMATCH` | Registry, Instance, Bundle, or Agent Host identities disagree. |

`PROVIDER_CANDIDATE_AMBIGUOUS` requires the operator to select one current-Host
Candidate and add the exact suggestion to user-owned configuration:

```toml
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-<sha256>"
evidence_digest = "<sha256>"
# location = "/exact/physical/path"
# version = "6.1.1"
```

OAW does not choose a Candidate and does not write the pin. Begin a new Workflow
after changing configuration. `oaw.provider-descriptor/v1` and
`oaw.user-config/v1` are unsupported active inputs; replace them with explicit
v3 records rather than expecting migration.

## Install State Is Not Workflow State

Install State and Workflow State are disjoint; no automatic migration occurs.
An installed adapter may correctly report `clean` while exposing only the
`policy` surface. Existing tasks and Profile locks are not imported, and
management commands do not create Workflow State. Only a real `host-native`
integration can exchange session facts and Receipts with OAW Core or the
Workflow Coordinator. The Agent Host still owns physical execution authority.

## Codex Host Bridge Diagnostics

Start with the read-only management projection:

```bash
oaw bridge check codex --format json
```

The management check proves file and registration state only. It always reports
`current_session_loaded: false`; only trusted `observe_current` Hook input in a
fresh Codex session can establish current-session evidence.

Text mode states `proof_scope: installation-integrity` and
`live_protocol_proof: false`. Never cite that result as live Bridge proof. A
reported update or required new session is an operator decision; stop before
START until it is explicitly performed and a fresh observation succeeds.

| Reason | Diagnosis and recovery |
| --- | --- |
| `HOST_BRIDGE_UNAVAILABLE` | The Plugin or MCP Bridge is unavailable. Install or enable it, inspect Codex `/hooks`, and start a new session. |
| `HOST_BRIDGE_CONTEXT_REQUIRED` | MCP was called without trusted Hook context. Review and trust the exact four Hook matchers, then start a new session. |
| `HOST_BRIDGE_PROTOCOL_MISMATCH` | The complete v4/v3/v2 VersionEvidence tuple differs. Stop; after explicit operator authorization, update the Bridge, review Hooks, start a new session, and observe again. |
| `HOST_EVIDENCE_HANDLE_REQUIRED` | A later operation omitted its current handle. Call `observe_current` and retry with the returned handle. |
| `HOST_EVIDENCE_HANDLE_INVALID` | The handle is malformed, edited, unknown, evicted, or from a restarted Bridge. Discard it and call `observe_current` again. |
| `HOST_EVIDENCE_EXPIRED` | The handle exceeded its bounded TTL. Call `observe_current` before retrying `core_inspect` or `core_compile`. |
| `HOST_EVIDENCE_SESSION_MISMATCH` | The handle belongs to another session or working directory. Stop before mutation and observe again in the current session. |
| `HOST_OBSERVATION_FAILED` | Required stable metadata, especially `skills/list`, failed. Repair the local Codex/App Server capability; affected Providers remain unverified. |
| `HOST_OBSERVATION_PARTIAL` | Optional Hook or configuration metadata is incomplete. Keep unavailable fields `unknown`; do not infer inheritance. |
| `HOST_SESSION_CHANGED` | Facts pinned by the active Bundle changed. Pause, observe again, return to the Startup Gate, and compile a new Bundle generation. |
| `PROVIDER_BINDING_CONTENT_MISMATCH` | The exact enabled Skill or complete Binding tree differs from the pinned Distribution evidence. Repair or select the exact trusted installation; never accept a same-name or partial-tree match. |
| `BINDING_EXPLICIT_INVOCATION_REQUIRED` | A human-explicit Binding lacks current Host/user invocation attestation. Obtain that exact invocation or stop; prompt text is not attestation. |
| `HOST_FEATURE_UNATTESTED` | A Recipe needs child, nested-child, or another live feature that the current Host did not attest. Use an eligible Recipe/topology or repair stable Host evidence. |
| `HOST_ACTION_UNAVAILABLE` | A required `workspace.prepare-or-confirm`, `verification.execute`, or `closeout.execute` action is unavailable. Supply an exact verified Host procedure or choose another eligible Recipe. |

`skills/list` is the required v2 Skill-observation authority; optional
`hooks/list` and `config/read` complete the metadata allowlist. `plugin/list` is
not a production dependency. Do not repair these reasons by editing a handle,
inventing a binding, copying Host configuration, or launching another process.
The [Codex Host Bridge guide](codex-bridge.md) defines installation, Hook, and
rollback behavior.

## Workflow Coordination Errors

These reason codes belong to the Core, Coordinator, or Host integration rather
than installation management:

| Reason | Diagnosis and recovery |
| --- | --- |
| `SCHEMA_UNSUPPORTED` | A Workflow command or result uses a retired schema. Update the caller and construct a new command; do not translate the record in place. |
| `WORKFLOW_STATE_UNSUPPORTED` | The selected Workflow State root contains a retired or unknown journal schema. Stop cooperating clients, preserve the exact state directory, and perform the explicit pre-release reset below. |
| `SUBAGENT_UNAVAILABLE` | The active Host session cannot create a native child. Return to the Startup Gate and select `CURRENT`, or repair native Host support; never launch a model process as fallback. |
| `MACRO_INTERNAL_CONFLICT` | Expansion found a duplicate owner or uncredited internal call. Fix the versioned Recipe so credit and dispatch edges execute exactly once. |
| `PROFILE_TOPOLOGY_UNAVAILABLE` | The Profile, Binding, delegation, or active Host cannot support the requested topology. Return to the Startup Gate; do not simulate the topology. |
| `HOST_SESSION_CHANGED` | Session identity, topology availability, or a pinned Host fact digest changed. Discard the stale Dispatch Packet, obtain a new Host session report, and recompile eligibility before dispatch. |

For an explicit pre-release state reset, first stop every client using the
Workflow, verify the exact path under
`${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/workflows`, and move
only the identified Workflow directory to a reviewed backup name. Start a new
Workflow from current configuration. OAW never deletes an unknown state root
automatically, and an operator must not remove the XDG state root broadly.

## Clean Files but Stale Agent Behavior

Agent tools differ in precedence and reload behavior. First confirm the exact
path and loader in the [adapter matrix](adapters.md). Check whether another
user, project, nested, team, or organization instruction has higher documented
precedence.

When the provider does not document live rule reload, **restart the target agent**
or application after an install or update. For tools with a documented refresh
command, use that command and inspect the loaded context. OAW marker comments
do not force reload and do not establish model precedence.

For bootstrap adapters, confirm the running agent can read the absolute
canonical policy path. For documented-import adapters, use the provider's
context inspection facility where available. A clean OAW check cannot prove
that a provider or model honored the instruction.

## Drift and Invalid State

Drift normally means a user, tool, or checkout changed recorded OAW bytes.
Preserve the current file before editing. Run the dry-run with the same scope
and selected targets, and inspect whether it proposes `would-update`,
`would-remove`, or `would-backup`.

`--force` can repair only recoverable drift tied to valid state. It cannot:

- adopt an untracked owned file;
- repair malformed or forged state;
- follow or replace a symlink;
- escape the project or XDG containment root;
- choose between duplicate, nested, or otherwise ambiguous markers.

An ambiguous marker case may create a backup and then stop with `manual
recovery required`. That is a refusal, not a partial success. Compare the
original, expected OAW fragment, and backup before editing anything.

## Inspect and Restore Backups

A successful forced mutation prints `backup: <directory>`. A forced dry-run
prints `would-backup: <directory>` but does not create it. Under the reported
directory, open `manifest.tsv` as text. Its header records format, operation,
and scope; each `artifact` row records:

```text
artifact<TAB>original-absolute-path<TAB>backup-path<TAB>checksum
```

Verify that every original path belongs to the intended scope, that each
backup file exists, and that its checksum matches the manifest. Never source or
execute `manifest.tsv`. Stop the affected agent/tool before restoration, then
**restore backups manually**, one reviewed artifact at a time, from the listed
backup path to the listed original path. Preserve modes and re-run `check`
afterward.

The Go manager attempts reverse-order rollback when a reported apply operation
fails. Replacement remains atomic per destination, not simultaneously across
all destinations, and process or machine crashes are outside that automatic
rollback path. If stderr reports `rollback failed` with status 74, use the
manifest and command output to identify which artifacts require manual restore.
Do not copy the entire backup directory over `HOME`, an XDG root, or a project.

## Update Problems

- `no installation state; run install first`: update exits 66. Run a scoped
  install dry-run and then install the target.
- `selected target is not installed`: update cannot add targets. Use install.
- `installed content differs from this checkout`: the running binary embeds a
  different source version or policy. For source use, rebuild `./oaw` from the
  checkout you intend to trust; release users should use the verified archive
  binary.
- Exit 70 for `VERSION`, policy, or `precompiled sibling binary is missing or not executable`:
  rebuild the source binary or restore the binary from the
  verified release archive. The wrapper never searches `PATH`, builds, or
  fetches a replacement.
- A path, containment, control-character, or symlink diagnostic: correct the
  root or filesystem layout. `--force` is not an override.

## Uninstall Refusal

An **uninstall refusal** protects content whose ownership cannot be proved.
Common causes are drift, invalid state, a changed or missing recorded target, a
symlink swap, inconsistent shared-destination checksums, or ambiguous markers.
Run `check`, then the corresponding scoped `uninstall --dry-run`. If state is
valid and drift is intentional, review an explicitly scoped forced uninstall;
otherwise use manual recovery.

Uninstall without state is a guarded successful no-op, but untracked OAW
markers still cause exit 65. A non-empty OAW-created directory is retained and
reported unchanged rather than recursively deleted. Clean uninstall never
removes surrounding user instructions or independently installed providers.

If evidence is still unclear, stop mutation and retain the checkout version,
full output, state file, destination bytes, and any backup path for a private
report under the [security policy](../../SECURITY.md).
