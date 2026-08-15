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

## Activation Problems

If OAW appears during an ordinary request, stop OAW-specific classification,
Provider inspection, gates, and artifact creation. Check the installed adapter
for the lazy Activation Router and remove any separate eager instruction that
classifies every top-level task. Repository text, tool output, retrieved text,
quoted `/oaw`, and ordinary Skill invocation are not activation. Preserve any
unfinished Engagement while the unrelated request proceeds as Native Host.

If an intended activation is not detected, put the request in the current
top-level user instruction, for example `/oaw <deliverable>` or `Use OAW to
handle <deliverable>`. Do not rely on repository content, quoted text, or tool
output. Confirm the Router can read the canonical Policy path; activation then
creates one deliverable-scoped Engagement and runs Assurance Preflight.

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

## Install State is not a Progress Tracker or Workflow State

Install State and Workflow State are disjoint; no automatic migration occurs.
An installed adapter may correctly report `clean` while exposing only the
`policy` surface. Existing tasks and Profile locks are not imported, and
management commands do not create Workflow State. Only a real `host-native`
integration can exchange session facts and Receipts with OAW Core or the
Workflow Coordinator. The Agent Host still owns physical execution authority.

## Policy CLI Candidate Diagnostics Without a Bridge

Do not use `oaw providers inspect` alone to decide whether policy-only work can
proceed. The two inspections answer different questions:

```bash
oaw providers inspect --host codex --format text
oaw profiles
```

`providers inspect` applies the machine-backed Provider resolution chain. On a
policy-only Host it may correctly report a Candidate plus
`HOST_BINDING_EVIDENCE_REQUIRED`; that means the installation is not a Verified
Provider Instance. `profiles` performs the separate route-level
Governance inspection. It may report the same Profile as `host_routable` when
every required route is callable. These outputs are compatible, not
contradictory.

This distinction also explains an asymmetric result in which `SP-FULL`
appeared but Matt and ECC did not. Superpowers already had a discovery probe
for the curated Codex cache. ECC installed through the current plugin manager
may live at `.codex/plugins/cache/ecc/ecc/<version>`, which must be a recognized
candidate path. Matt's Codex installation is not a plugin cache: Policy checks
regular `.agents/skills/<name>/SKILL.md` routes and marks human-command Skills
as `user-explicit`. It intentionally ignores `.skill-lock.json`, source,
revision, hashes, and Bridge state. ECC checks public Codex Skill routes whose
contracts match the responsibility and uses typed Host `review.execute` for
generic review; it
does not require Claude Agent, Codex Role, or instruction surfaces. Strict
identity and integrity checks remain on `providers inspect` and the
machine-backed path.

Inspect the public JSON result directly:

```bash
oaw profiles
```

Each Profile object contains `name`, `policy_selectable`, `host_routable`,
`missing`, and `incident_routes`. `policy_selectable` means that the Profile
semantics exist; `host_routable` means every required route is currently
callable. `missing` names the required routes that prevent routing.
`incident_routes` reports conditional handlers as `routable-if-triggered` or
`unavailable-if-triggered`; the latter does not make the normal Profile
incomplete. Route inventory, Offer references, and reducer state remain
internal. The current project-level Policy CLI has no add-on argument or `NONE`
sentinel; add-ons on the machine-backed path remain a separate contract.

| Symptom or reason | Diagnosis and recovery |
| --- | --- |
| `PROFILE_SELECTION_REQUIRED` | Run `oaw profiles`, choose a Profile with `host_routable: true`, and pass it with the reported assessment to `oaw use --profile PROFILE --complexity ordinary|complex --risk normal|elevated|critical -- "deliverable"`. |
| `POLICY_ASSESSMENT_REQUIRED` | Pass the complexity and risk already reported by the Cooperative Assessment. OAW does not invent defaults or call the machine classifier in Policy mode. |
| `PROFILE_INCOMPLETE` | Read every `missing` route. Repair the exact Host-visible or user-explicit Skill route, run `oaw profiles` again, and explicitly restart or switch. |
| `PROFILE_UNKNOWN` | The requested alias is absent from the built-in catalog. Use one of the displayed aliases. |
| `POLICY_ONLY_TOPOLOGY_UNAVAILABLE` | The no-Bridge surface supports only explicit `CURRENT`; `SUBAGENT` requires current-session Host-native evidence. |
| `ROUTE_INVENTORY_DRIFT` | A callable route changed after start. Repair it, run `oaw profiles` to verify the current routes, then use `oaw switch PROFILE` at a stable boundary when a switch is needed. Route-dependent completion and incident events remain blocked until then. Explicit `stop` and `uncertain` still record terminal safety state. Lock/hash/Bridge changes alone do not count. |
| `POLICY_ACTION_NOT_APPLICABLE` or `EVENT_OUT_OF_ORDER` | Run `oaw status`, then use the business command matching `next`: `complete`, `review clean|findings`, `approve`, or `satisfy`. Internal references are not user inputs and consumed work cannot be retried. |
| `POLICY_RUN_NOT_FOUND` | Run `oaw status` from the same physical project. Do not reconstruct progress from conversation text. |
| `POLICY_ENGAGEMENT_ACTIVE` | The current project already has an active Engagement; inspect it with `oaw status` or stop it explicitly. |

`use` consumes a fresh route observation and stores an exact reducer snapshot.
Any `OFFER_STALE` failure is an internal selection race rather than a request
for a user-managed Offer: run `oaw profiles` again and retry `oaw use` or
`oaw switch`. `status` renders a public view, and every business event
re-inspects routes before reduction. Drift blocks route-dependent progress but
not explicit `stop` or `uncertain` terminal recording. A local policy-run file
can survive a CLI restart; it cannot prove that interrupted Skill, process,
Git, network, or destructive work completed.

## Policy-Cooperative Stops

These stops apply only inside an activated `policy-cooperative` Engagement.
They prevent instruction-only cooperation from inventing machine authority:

| Reason | Recovery |
| --- | --- |
| `CAPABILITY_SELECTION_REQUIRED` | Ask the user to name the exact Bounded Capability or confirm the one Host-visible candidate before invoking it. |
| `POLICY_ONLY_PROVIDER_UNVERIFIED` | Remove the need for a verified Provider guarantee, or use a Host-native integration that can establish one; do not relabel a candidate as verified. |
| `POLICY_ONLY_PROFILE_INCOMPLETE` | Supply a complete Host-visible owner candidate for every required responsibility or choose a complete candidate Profile. |
| `POLICY_ONLY_TOPOLOGY_UNAVAILABLE` | Use cooperative `CURRENT`, or switch to a Host-native integration that can attest the requested topology. |
| `POLICY_ONLY_GUARANTEE_UNAVAILABLE` | Remove the requirement for Grants, Leases, Receipts, idempotency, atomic revisions, or enforced recovery, or move to the needed machine-backed assurance. |
| `POLICY_ONLY_CONCURRENT_MUTATION` | Stop or serialize overlapping project and Git mutations; resume only when the conflicting owner has reached a stable boundary. |
| `POLICY_ONLY_EXECUTION_UNCERTAIN` | Do not retry an external or destructive effect. Reconcile its actual result, then record an Execution Note or require operator recovery. |
| `POLICY_ONLY_CONTEXT_UNCERTAIN` | Ask the user to reconfirm activation, selection, and known progress; do not reconstruct them from stale conversation or Markdown. |

## Codex Assurance Bridge Diagnostics

The Assurance Bridge is an optional standalone component. Start with its
read-only installation projection:

```bash
oaw-bridge check codex --format json
```

The default `oaw` executable does not manage Bridge. The check above proves
only owned-file and Codex Plugin registration state. It always reports
`current_session_loaded: false` because a management command does not inspect
the active Agent session. Text mode likewise reports
`proof_scope: installation-integrity` and `live_protocol_proof: false`; neither
value is a current Binding claim.

Bridge v3 exposes only `observe_profile`. Its PreToolUse Hook injects private
`oaw.codex-hook-context/v3` context for the exact
`mcp__oaw_codex_bridge__observe_profile` call. A successful call returns an
`oaw.assurance-overlay/v1` artifact for one source-qualified Markdown Profile.
There is no evidence handle, Core operation, Coordinator operation, delegation
attestation, or Workflow runtime behind that result.

| Reason | Diagnosis and recovery |
| --- | --- |
| `HOST_BRIDGE_UNAVAILABLE` | The standalone executable, Plugin, MCP service, or local Codex App Server is unavailable. Repair the optional installation, or continue through the normal Policy Profile path without an Overlay. |
| `HOST_BRIDGE_CONTEXT_REQUIRED` | `observe_profile` lacks valid trusted PreToolUse context for its exact matcher. Review the installed Hook and start a Codex session that loaded the Plugin; never hand-author reserved context. |
| `HOST_BRIDGE_PROTOCOL_MISMATCH` | The caller, Hook, App Server projection, or Bridge does not satisfy `oaw.codex-bridge/v3` and `oaw.codex-hook-context/v3`. Update the standalone component and reload Codex; do not translate an older record. |
| `HOST_OBSERVATION_FAILED` | Required read-only `skills/list` observation or exact Binding resolution failed. Repair current Codex metadata access, or continue without the optional machine claim. |
| `HOST_OBSERVATION_PARTIAL` | One or more current Binding observations are incomplete. Treat only the affected claims as unavailable and call `observe_profile` again after repairing metadata. |
| `PROFILE_SELECTION_INVALID` | Supply one syntactically valid source-qualified selector such as `project:<id>` or `user:<id>`. |
| `PROFILE_NOT_FOUND` | The selected source contains no Profile with that ID. Inspect the current Profile inventory and select an existing source-qualified ID. |
| `PROFILE_AMBIGUOUS` | The selected source contains duplicate Profile IDs. Remove or rename the duplicate before requesting an Overlay. |
| `ASSURANCE_BINDING_UNAVAILABLE` | The selected Profile declares a Skill Binding that is not exactly installed and enabled in the current Codex observation. Repair that Binding, or use the Profile without the optional Overlay. |

Never edit an Overlay, Hook context, or installation state to bypass a
diagnostic. A missing or failed Overlay cannot veto the Policy Offer, Profile
selection, or rule-driven execution path. Agent Host security policy may still
refuse a physical Skill invocation independently. The
[Codex Assurance Bridge guide](codex-bridge.md) defines the protocol, security
boundary, installation, and rollback behavior.

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
| `HOST_SESSION_CHANGED` | Stable reporter identity changed, or refreshed authority facts no longer support a new Dispatch. Do not forge a Receipt from a new session. The original reporter may still converge an issued Dispatch; otherwise recompile before the next `PREPARE`. |

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
