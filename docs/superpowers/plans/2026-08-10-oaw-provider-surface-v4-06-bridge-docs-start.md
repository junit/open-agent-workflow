# OAW Provider Surface v4 06: Codex Bridge, Documentation, and START Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Hard-cut Codex Bridge v2 to the complete Provider Surface v4 authority tuple, report only facts proved by stable Host APIs, publish the corrected four-Profile lifecycle matrix, run the first post-cutover full-tree gate, and submit one fresh SP-FULL / CURRENT / no Add-on Coordinator START that either commits v4 authority or fails closed.

**Architecture:** Plans 01-05 define the catalog, Host, compiler, Grant, Receipt, Conformance, and Coordinator records. This plan advances every Bridge, Hook, CLI, generated-asset, and black-box reference to those records in one atomic commit; there is no mixed-version executable interval. Codex observation remains conservative: skills/list, hooks/list, and config/read are the only allowlisted App Server metadata methods, complete Binding trees replace single-file evidence, and unavailable role, delegation, invocation, authorization, gate, and Host-action facts are never inferred.

**Tech Stack:** Go 1.26, Codex App Server JSON-RPC, MCP Go SDK, canonical JSON SHA-256, Markdown, Bash, race detector, coverage, and repository shell gates.

---

**Selected lifecycle:** SP-FULL / CURRENT / no Add-on.

**Depends on:** Plans 01-05. Plan 05 supplies focused Core/Coordinator gates and the final GraphCursor, Grant v3, Receipt v3, Conformance v4, and Workflow v2 contracts. Plan 06 owns the first post-cutover full-tree gate. The cancelled v3 Workflow workflow-88833e4868d3202599342e2c5c133f82 remains inert audit history and must not be resumed.

**Produces:** One atomic Codex Bridge v2 cutover commit, one policy/documentation commit, reviewed remediation when required, fresh full-tree evidence, and exactly one new START result.

## File Map

### Atomic Bridge Cutover

| Action | Path | Responsibility |
| --- | --- | --- |
| Modify | internal/codexbridge/protocol.go | Bridge v2 protocol, Hook Context v2, evidence handle v2, fact-digest, and four-operation records. |
| Modify | internal/codexbridge/protocol_test.go | Exact v2 constants, closed operation set, and v1 rejection tests. |
| Modify | internal/codexbridge/version.go | Complete canonical VersionEvidence tuple and negotiation. |
| Modify | internal/codexbridge/version_test.go | Independent mutation test for every VersionEvidence field and digest. |
| Modify | internal/codexbridge/cache.go | v2 handle issuance, session/CWD pinning, reset, TTL, and LRU validation. |
| Modify | internal/codexbridge/cache_test.go | v1 handle/context rejection, restart invalidation, expiry, and cloning tests. |
| Modify | internal/codexbridge/bindings.go | Exact enabled Skill to Distribution/Binding tree matching. |
| Modify | internal/codexbridge/bindings_test.go | Same-name, shared-ancestor, disabled, orphan, ambiguous, and drift tests. |
| Modify | internal/codexbridge/facts.go | Host v3 facts, feature/action observations, VersionEvidence negotiation, and digests. |
| Modify | internal/codexbridge/facts_test.go | Conservative Host v3 observation and private-data exclusion tests. |
| Modify | internal/codexbridge/evidence.go | Immutable v2 fact set validation and complete fact cloning. |
| Modify | internal/codexbridge/inputs.go | Closed Bridge projections for Core and Coordinator v2. |
| Modify | internal/codexbridge/service.go | v4 inspect/compile and v2 workflow exchange with eight-digest preflight. |
| Modify | internal/codexbridge/service_test.go | Builder, selection, Host-owned evidence, drift, and START tests. |
| Modify | internal/codexbridge/mcp.go | Four closed MCP tools and v2 Hook-context validation. |
| Modify | internal/codexbridge/mcp_test.go | Closed schemas, reserved context, caller-forged authority, and v1 rejection tests. |
| Modify | internal/codexbridge/conformance.go | Host Conformance Transcript v4 construction from Receipt v3. |
| Modify | internal/codexbridge/conformance_test.go | Separate Transcript v4 and Report v4 assertions. |
| Modify | internal/codexbridge/integration.go | Host Manifest v3 and Bridge Integration 2.0.0 declaration. |
| Modify | internal/codexbridge/integration_test.go | Integration v3, CURRENT-only, skill-only, and protocol-v2 tests. |
| Modify | internal/codexbridge/appserver/observation_test.go | Exact three-method allowlist and conservative metadata projection tests. |
| Modify | internal/codexbridge/hook/input.go | Strict Hook input boundary for the four Bridge tools. |
| Modify | internal/codexbridge/hook/input_test.go | Oversize, unknown-field, wrong-event, and v2 tool-boundary tests. |
| Modify | internal/codexbridge/hook/output.go | Inject only Hook Context v2 and validate only handle v2. |
| Modify | internal/codexbridge/hook/output_test.go | v1 denial, v2 rewrite, foreign-CWD, and edited-handle tests. |
| Modify | internal/cli/bridge.go | Serve and Hook entrypoints wired only to Bridge v2. |
| Modify | internal/cli/bridge_test.go | CLI Hook/serve v2 and installation-check separation tests. |
| Modify | internal/cli/provider_inputs.go | Validate Host Inventory v3 and Core/Registry v4 inputs. |
| Modify | internal/cli/provider_inputs_test.go | Exact Host v3 provider-input validation and stale-schema rejection. |
| Modify | internal/cli/workflow.go | Workflow Command/Result v2 output and rejection handling. |
| Modify | internal/cli/workflow_test.go | CLI v2 exchange, canonical result, and v1 command rejection tests. |
| Modify | internal/dogfood/config.go | Synthetic v4 Descriptor/Recipe and Host v3 configuration. |
| Modify | internal/dogfood/coordinator.go | Workflow v2 exchange, Dispatch v2, and Receipt v3 pins. |
| Modify | internal/dogfood/current.go | Cursor-based CURRENT preparation and receipt flow. |
| Modify | internal/dogfood/dogfood_test.go | Dogfood hard-cut and Host-owned evidence tests. |
| Modify | internal/dogfood/filesystem.go | Preserve persistence invariants for v2 records. |
| Modify | internal/dogfood/records.go | Local v2 command/receipt projection records. |
| Modify | internal/dogfood/repository.go | Preserve repository evidence under the v4 flow. |
| Modify | internal/integration/codex_bridge_conformance_test.go | End-to-end Host v3, Receipt v3, Transcript/Report v4 conformance. |
| Modify | internal/integration/codex_bridge_blackbox_test.go | Public v2 MCP, Builder diagnostics, and fresh START black-box coverage. |
| Modify | internal/integration/config_discovery_test.go | Descriptor v4, Recipe v3, `InstallRoot`, and Registry v4 vertical slice. |
| Modify | internal/integration/core_coordinator_cutover_test.go | Bundle/Graph v4 and complete Workflow v2 lifecycle. |
| Modify | internal/integration/host_configuration_test.go | Host v3 configuration and evidence boundary. |
| Modify | internal/integration/external_host_transcript_test.go | Receipt v3 and Conformance v4 external Host transcript. |
| Modify | internal/integration/host_conformance_test.go | Receipt v3 Conformance v4 integration coverage. |
| Modify | internal/integration/workflow_coordinator_test.go | Cursor-based Workflow v2 integration flow. |
| Modify | internal/integration/testdata/core-coordinator/acme-provider.json | Synthetic Provider Descriptor v4 fixture. |
| Modify | internal/integration/testdata/core-coordinator/acme-profile.json | Synthetic Profile Recipe v3 fixture. |
| Create | internal/integration/testdata/core-coordinator/old-v1-revision.json | Inert v1 rejection fixture only. |
| Modify | internal/hosttest/fixture.go | Shared Host v3 fixture. |
| Modify | internal/hosttest/provider.go | Shared Descriptor v4 and Registry v4 fixture. |
| Modify | internal/hosttest/workflow.go | Shared Dispatch v2 and Receipt v3 fixture. |
| Modify | internal/hosttest/workflow_test.go | Cursor/output/evidence fixture tests. |
| Modify | internal/assets/generate/codex_host.go | Generate Integration v3 with Bridge 2.0.0 and Conformance v4. |
| Modify | internal/assets/generate/codex_host_test.go | Canonical and idempotent Codex Host asset generation tests. |
| Modify | internal/assets/generate/main.go | Generate only the active Host v3/Conformance v4 asset set. |
| Modify | internal/assets/host-integrations.json | Active Codex Host Integration v3 record. |
| Modify | internal/assets/conformance/codex-host-v3.json | Active Codex Host v3 fixture carrying Transcript v4 records. |
| Modify | internal/assets/embed_test.go | Active schema/fixture embedding and canonical digest assertions. |
| Modify | internal/builtin/load_test.go | Preserve Plan 04 Profile coverage while updating only Host fixture assertions. |
| Modify | internal/codexbridge/install/assets/skills/oaw-codex-bridge/SKILL.md | Installed Bridge v2 invocation and fail-closed recovery instructions. |
| Modify | scripts/check-codex-bridge.sh | Focused Bridge tests, race, coverage, vet, and production legacy scan. |
| Modify | scripts/check-core-coordinator-coverage.sh | Bundle/Graph v4 and Workflow v2 coverage gate. |
| Modify | tests/16-core-coordinator-conformance-test.sh | Workflow v2 black-box and stale-record rejection. |
| Modify | tests/18-codex-bridge-protocol-test.sh | Hook/MCP/CLI v2 black-box contract and literal v1 rejection. |

Plan 02 creates internal/assets/conformance/codex-host-v3.json and deletes the old active codex-host-v1.json fixture before this plan begins. Plan 06 modifies the v3-named Host fixture in place because the Host contract remains v3 while its embedded Conformance contract advances to v4. Plan 02 retains the v1 audit fixture as historical evidence. Plan 04 owns the Profile assertions in `internal/builtin/load_test.go`; Plan 06 may update only its Host fixture/conformance assertions and must preserve all Plan 04 Profile coverage. No Plan 06 production file is deleted.

### Policy And Documentation

| Action | Path | Responsibility |
| --- | --- | --- |
| Modify | policy/ENGINEERING.md | Canonical ten-slot v4 lifecycle, four built-ins, USER-DEFINED composition, and hard-cut policy. |
| Modify | README.md | English public v4 summary and truthful current-Host limitations. |
| Modify | README-zh.md | Chinese public v4 summary with matching claims. |
| Modify | docs/en/architecture.md | v4 ownership and Host authority boundaries. |
| Modify | docs/en/lifecycle.md | Ten-slot lifecycle and exact Profile matrix. |
| Modify | docs/en/comparison.md | Source-pinned Provider comparison without fictional skills. |
| Modify | docs/en/adapters.md | Descriptor/Binding/Host evidence model. |
| Modify | docs/en/codex-bridge.md | Bridge v2 protocol, live proof, and fail-closed behavior. |
| Modify | docs/en/extending-adapters.md | USER-DEFINED Recipe and verified Binding composition. |
| Modify | docs/en/security.md | Complete-tree evidence, Host-owned authority, and stale-state rejection. |
| Modify | docs/en/troubleshooting.md | Exact v4 exclusion and recovery diagnostics. |
| Modify | docs/zh/architecture.md | Chinese architecture parity. |
| Modify | docs/zh/lifecycle.md | Chinese lifecycle and Profile-matrix parity. |
| Modify | docs/zh/comparison.md | Chinese Provider comparison parity. |
| Modify | docs/zh/adapters.md | Chinese adapter/evidence parity. |
| Modify | docs/zh/codex-bridge.md | Chinese Bridge v2 parity. |
| Modify | docs/zh/extending-adapters.md | Chinese USER-DEFINED composition parity. |
| Modify | docs/zh/security.md | Chinese security parity. |
| Modify | docs/zh/troubleshooting.md | Chinese diagnostics parity. |
| Modify | scripts/check-docs.sh | Matrix, upstream pin, version, and bilingual parity assertions. |
| Modify | tests/10-docs-test.sh | Public documentation and forbidden fictional-claim tests. |

### Review And Operational Evidence

| Action | Path | Responsibility |
| --- | --- | --- |
| Modify | lib/detect.sh | Review remediation only: hard-cut the test-only legacy management oracle to the same v4 Matt and ECC compatibility probes as Go management. |
| Modify | internal/check/diagnostics_test.go | Review remediation only: migrate legacy Provider-detection fixtures to the approved v4 Matt lockfile and ECC Plugin probes without changing production detection behavior. |
| Modify | internal/schema/registry_v2_test.go | Review remediation only: keep the valid Workflow Snapshot v2 fixture aligned with the closed, runtime-required Classification and Configuration projections. |
| Modify | scripts/smoke-linux.sh | Review remediation only: migrate the release-smoke START fixture to Workflow v2 and Host v3 while preserving the Coordinator-state and no-model boundaries. |
| Modify | tests/02-check-test.sh | Review remediation only: reject retired Matt/ECC Skill probes and exercise the v4 lockfile/Plugin readiness fixtures. |
| Modify | tests/11-check-parity-test.sh | Review remediation only: prove exact Bash/Go parity over retired probes and the active v4 Provider probes. |
| Modify | tests/14-cutover-release-test.sh | Review remediation only: reject stale authority literals in the Linux smoke and prove the Docker release path initializes current Coordinator state. |
| Create | .scratch/oaw-provider-surface-v4/evidence/review.md | Fixed-point correctness/security review record. |
| Create | .scratch/oaw-provider-surface-v4/evidence/reviewed-paths.txt | Sorted exact repository-relative path ledger for validated remediation. |
| Create | .scratch/oaw-provider-surface-v4/evidence/verification.md | Fresh command, status, digest, coverage, and baseline evidence. |
| Create | .scratch/oaw-provider-surface-v4/evidence/host-summary.json | Secret-free current Host observation summary. |
| Create | .scratch/oaw-provider-surface-v4/evidence/start-result.json | Secret-free accepted or rejected START result. |

Scratch evidence is never committed. A reviewed remediation path becomes part of the task file map only after its exact repository-relative path is written to reviewed-paths.txt, verified against the explicit File Maps in Plans 01-06, and recorded with its Modify, Create, or Delete disposition in review.md before the edit.

The `internal/check/diagnostics_test.go` entry is a review-discovered Plan 06
amendment. The post-cutover full-tree gate exposed three stable RED tests whose
fixtures still modeled the superseded Matt four-skill bundle and treated the ECC
Marketplace Plugin probe as insufficient. Remediation may update only those
fixtures and expectations to the v4 probes already exercised by
`internal/management/management_check_test.go`; it must not alter production
Provider discovery or broaden any compatibility indicator.

The `internal/schema/registry_v2_test.go` entry is a second review-discovered
Plan 06 amendment. Closing the Workflow Snapshot v2 projection exposed that its
positive AlternativeChoice fixture used empty Classification and Configuration
objects even though Coordinator validation requires the complete records.
Remediation may replace only those empty fixture objects with complete valid v4
projections; it must not weaken the active schema or alter production decoding.

The `lib/detect.sh`, `tests/02-check-test.sh`, and
`tests/11-check-parity-test.sh` entries are a third review-discovered Plan 06
amendment. The mandatory Task 4 shell suite proved that the test-only legacy
management oracle still accepted the retired Matt four-Skill bundle and generic
ECC Skill while Go management used the v4 lockfile and Plugin probes. The
remediation must preserve the oracle's read-only role, replace only those Matt
and ECC detection probes, and add negative legacy-probe plus positive v4-probe
parity evidence.

The `scripts/smoke-linux.sh` and `tests/14-cutover-release-test.sh` entries are
a fourth review-discovered Plan 06 amendment. After the Provider parity RED was
resolved, the mandatory Task 4 shell suite reached Docker release smoke and
proved that its embedded START fixture still used Workflow Command v1 and Host
Session v2. The hard-cut CLI rejected that request before Coordinator state
initialization, so the smoke's current-state assertion failed. Remediation must
add an exact stale-authority RED assertion, migrate only the embedded fixture
to the production Workflow v2 and Host v3 constructors, retain the expected
closed rejection caused by unavailable trusted Host-native evidence, and keep
the Policy-only file plus no-model boundaries unchanged.

## Locked Bridge Contract

~~~go
const (
    BridgeProtocolVersion  = "oaw.codex-bridge/v2"
    HookContextSchemaV2    = "oaw.codex-hook-context/v2"
    EvidenceHandleVersion  = "oaw.host-evidence-handle/v2"
    BridgeIntegrationVersion = "2.0.0"
)

type VersionEvidence struct {
    PluginVersion                   string   `json:"plugin_version"`
    BridgeProtocol                  string   `json:"bridge_protocol"`
    HookContextSchema               string   `json:"hook_context_schema"`
    IntegrationVersion              string   `json:"integration_version"`
    CodexVersion                    string   `json:"codex_version"`
    MetadataMethods                 []string `json:"metadata_methods"`
    ProviderDescriptorSchema        string   `json:"provider_descriptor_schema"`
    ProfileRecipeSchema             string   `json:"profile_recipe_schema"`
    HostManifestSchema              string   `json:"host_manifest_schema"`
    HostSessionSchema               string   `json:"host_session_schema"`
    HostBindingInventorySchema      string   `json:"host_binding_inventory_schema"`
    HostEnvironmentReportSchema     string   `json:"host_environment_report_schema"`
    HostInvocationReceiptSchema     string   `json:"host_invocation_receipt_schema"`
    HostConformanceTranscriptSchema string   `json:"host_conformance_transcript_schema"`
    HostConformanceReportSchema     string   `json:"host_conformance_report_schema"`
    ExecutionGraphSchema            string   `json:"execution_graph_schema"`
    LifecycleBundleSchema           string   `json:"lifecycle_bundle_schema"`
    CapabilityGrantSchema           string   `json:"capability_grant_schema"`
    DispatchPacketSchema            string   `json:"dispatch_packet_schema"`
    WorkflowCommandSchema           string   `json:"workflow_command_schema"`
    WorkflowResultSchema            string   `json:"workflow_result_schema"`
    WorkflowSnapshotSchema          string   `json:"workflow_snapshot_schema"`
    WorkflowRevisionSchema          string   `json:"workflow_revision_schema"`
    Digest                          string   `json:"digest"`
}
~~~

VersionEvidence has these exact target values:

| Field | Required value or rule |
| --- | --- |
| PluginVersion | The current installed Plugin/binary semantic version supplied to ServiceOptions; it is not inferred from the Integration version. |
| BridgeProtocol | oaw.codex-bridge/v2 |
| HookContextSchema | oaw.codex-hook-context/v2 |
| IntegrationVersion | 2.0.0 |
| CodexVersion | At or above MinimumCodexBaseline 0.146.1. |
| MetadataMethods | Canonical sorted unique methods; skills/list is required, config/read and hooks/list are optional. |
| ProviderDescriptorSchema | oaw.provider-descriptor/v4 |
| ProfileRecipeSchema | oaw.profile-recipe/v3 |
| HostManifestSchema | oaw.host-manifest/v3 |
| HostSessionSchema | oaw.host-session/v3 |
| HostBindingInventorySchema | oaw.host-binding-inventory/v3 |
| HostEnvironmentReportSchema | oaw.host-environment-report/v2 |
| HostInvocationReceiptSchema | oaw.host-invocation-receipt/v3 |
| HostConformanceTranscriptSchema | oaw.host-conformance-transcript/v4 |
| HostConformanceReportSchema | oaw.host-conformance-report/v4 |
| ExecutionGraphSchema | oaw.execution-graph/v4 |
| LifecycleBundleSchema | oaw.lifecycle-bundle/v4 |
| CapabilityGrantSchema | oaw.capability-grant/v3 |
| DispatchPacketSchema | oaw.dispatch-packet/v2 |
| WorkflowCommandSchema | oaw.workflow-command/v2 |
| WorkflowResultSchema | oaw.workflow-result/v2 |
| WorkflowSnapshotSchema | oaw.workflow-snapshot/v2 |
| WorkflowRevisionSchema | oaw.workflow-revision/v2 |
| Digest | canonicaljson.Digest over every preceding normalized field with Digest empty. |

Negotiation reconstructs and validates the canonical digest, mutates no caller storage, and checks every field independently. Any missing, duplicate, non-canonical, stale, or mismatched value returns HOST_BRIDGE_PROTOCOL_MISMATCH before Core, Admission, or Coordinator access. There is no compatibility decoder, v1 handle translation, old Workflow reader, or fallback tuple.

The public Workflow projection defines Bridge-owned Start, Prepare, Receipt, Switch, and Cancel input structs. It must not reuse coordinator.PrepareInput because that would expose Host-owned authority fields through generated MCP schemas. In particular, public callers cannot provide UserAuthorization, GateAttestation, or ExplicitInvocationAttestation. The Bridge hydrates Host Session, Inventory, Environment, Features, Actions, Registry, Gate, user authorization, and explicit invocation evidence only from the current Host evidence handle; when the stable Host surface cannot attest one, the operation fails closed.

Before every non-START Coordinator command, compare the active Bundle with the exact eight authority digests from Plan 05: Host Session, Environment Report, Provider Inventory, Host Features, Host Actions, Configuration, Resolution, and Registry. Discovery evidence may remain in the private fact set, but it is not substituted for any Bundle digest. A difference returns HOST_SESSION_CHANGED before a Grant, Dispatch Packet, state read with executable authority, or Receipt transition.

Current Codex observation remains conservative:

~~~text
binding kinds proven: skill
outer topologies proven: CURRENT
role, instruction, agent, and tool bindings: unavailable unless a stable live API is added and tested
child, parallel-child, nested-child, and nested-parallel-child delegation: unknown unless a stable live API reports available
workspace.prepare-or-confirm: unknown
verification.execute: unknown
closeout.execute: unknown
~~~

Static multi-agent configuration may project host-configured; it cannot become available. Repository presence, prompt text, filesystem role names, and Plugin branding never prove a live Binding or delegation feature.

## Task 1: Atomically Hard-Cut The Entire Codex Bridge To v2

**Files:** Every path in the Atomic Bridge Cutover table, with exactly its listed disposition. Do not commit any subset of this task.

- [ ] **Step 1: Establish an empty index and write all RED tests first**

Run:

~~~bash
rtk git diff --cached --quiet
~~~

Expected: exit 0. Update the listed Go tests plus tests/18-codex-bridge-protocol-test.sh before production code. The tests must independently prove:

1. all 24 VersionEvidence fields, including its canonical digest;
2. exact rejection of Bridge protocol v1, Hook Context v1, handle v1, Provider v3, Recipe v2, Graph/Bundle v3, Grant v2, Receipt v2, Conformance Transcript v3, Dispatch v1, and all four Workflow v1 records;
3. exactly four closed MCP operations and no caller-supplied Host authorization, invocation, or gate attestation;
4. complete Binding-tree evidence with no shared-ancestor or same-name shortcut;
5. Host Manifest/Session/Inventory v3, Environment Report v2, feature/action facts, and CURRENT-only topology;
6. Core inspection with four aliases plus USER-DEFINED Builder diagnostics, exact selection confirmation, Bundle/Graph v4, and no implicit selection;
7. Grant v3, Dispatch v2, Receipt v3, Transcript/Report v4, and Workflow v2 exchange;
8. CLI Bridge/Workflow v2 output, Hook v2 injection/denial, generated Integration assets, dogfood/hosttest consumers, and the black-box START boundary.

- [ ] **Step 2: Run the RED protocol and integration boundary**

Run:

~~~bash
rtk go test ./internal/codexbridge ./internal/codexbridge/appserver ./internal/codexbridge/hook ./internal/cli ./internal/dogfood ./internal/hosttest ./internal/integration ./internal/assets ./internal/assets/generate ./internal/builtin -run 'BridgeV2|VersionEvidence|HookContextV2|HandleV2|BindingTree|HostFactsV3|CoreInspectV4|CoreCompileV4|WorkflowV2|ReceiptV3|ConformanceV4|CodexHost|ProviderInputs|Dogfood|Hosttest' -count=1
rtk bash tests/18-codex-bridge-protocol-test.sh
~~~

Expected: FAIL because the production Bridge still contains v1 protocol/context/handle references, old Host/Bundle/Workflow schemas, single-file Binding evidence, and caller-visible Coordinator authority fields. Preserve the failing output for the later review record; do not weaken a test to obtain RED.

- [ ] **Step 3: Replace protocol, negotiation, cache, Hook, and CLI references together**

Implement the locked constants and VersionEvidence type exactly. Build the evidence with the ServiceOptions Plugin version, Bridge Integration 2.0.0, canonical metadata methods, and constants exported by Plans 01-05. Normalize then digest a defensive copy. Replace every Hook output and validation reference with HookContextSchemaV2 and EvidenceHandleVersion. Keep random opaque tokens, session/CWD digests, TTL, LRU, reset, strict decoding, and no raw handle logging.

Update internal/cli/bridge.go and internal/cli/workflow.go in the same worktree state: serve only Bridge v2, emit only Workflow Result v2, and reject a Workflow Command v1 before loading executable state. bridge check remains an installation-management operation; its independent management/install schema versions do not become evidence that the live protocol negotiated.

- [ ] **Step 4: Implement exact observation and immutable Host facts**

Use App Server skills/list only to establish enabled Skill name, canonical path, and scope. Resolve exactly one discovered Provider installation, resolve the Descriptor Binding `InstallRoot` below that exact installation root, compute `integrity.DigestTree`, and compare it with the digest pinned for the Distribution `ContentRoot`. Never derive one root from the other. Emit one Host Binding Observation only for an exact match. A disabled, orphan, ambiguous, same-name foreign, ancestor, symlinked, or drifting tree remains unavailable with a stable diagnostic.

Construct Host Manifest, Session, and Inventory v3 plus Environment Report v2. Preserve skills/list, hooks/list, and config/read as the complete metadata allowlist. Add feature and Host-action digests to immutable Facts and clone every nested slice. Report unknown or host-configured for unproved delegation/action surfaces; never probe a shell, infer a role from files, or project an absolute Skill path in public output.

- [ ] **Step 5: Implement closed Core and Coordinator projections**

Define Bridge-specific public Prepare and Receipt projections rather than embedding Coordinator input types that carry Host authority. Decode with unknown-field and trailing-value rejection. Hydrate all Host-owned facts from the v2 handle. Make core.inspect return the taxonomy, four aliases, USER-DEFINED Builder projection, exact exclusions, alternatives, and confirmation digest without a Bundle. Make core.compile require the exact confirmation digest and return only Bundle v4.

workflow_exchange accepts only Workflow Command v2. START uses Host-hydrated facts and can return only Workflow Result/Snapshot/Revision v2 containing Bundle/Graph v4. PREPARE cannot accept caller-forged UserAuthorization, GateAttestation, or ExplicitInvocationAttestation; network-mutate and human-explicit units therefore stop unless the current Host evidence proves the exact Plan 05 record. All non-START commands run the eight-digest preflight before executable state is used.

- [ ] **Step 6: Advance Conformance, Integration, installed instructions, and generated assets**

Build Transcript v4 and Report v4 as separate records over Receipt v3. Advance BridgeIntegrationVersion to 2.0.0 while retaining Host Integration schema v3. Update the installed Bridge skill to name only the four v2 tools, explain that observe_current is the live protocol proof, forbid old-handle reuse, and keep Host handles out of persisted artifacts.

Update the generator to read internal/assets/conformance/codex-host-v3.json, validate Transcript/Report v4, rebuild internal/assets/host-integrations.json, and preserve unrelated integrations. Preserve the Plan 02 audit fixture byte-for-byte as historical evidence. Run the generator twice; the second run must leave the same bytes:

~~~bash
rtk go run ./internal/assets/generate
rtk go test ./internal/assets/generate -run 'CodexHost' -count=1
rtk go run ./internal/assets/generate
rtk go test ./internal/assets/generate -run 'CodexHost' -count=1
~~~

Expected: both test runs PASS and internal/assets/host-integrations.json plus internal/assets/conformance/codex-host-v3.json remain canonical after the second generation.

- [ ] **Step 7: Format every changed Go file by exact path**

Run:

~~~bash
rtk gofmt -w internal/codexbridge/protocol.go internal/codexbridge/protocol_test.go internal/codexbridge/version.go internal/codexbridge/version_test.go internal/codexbridge/cache.go internal/codexbridge/cache_test.go internal/codexbridge/bindings.go internal/codexbridge/bindings_test.go internal/codexbridge/facts.go internal/codexbridge/facts_test.go internal/codexbridge/evidence.go internal/codexbridge/inputs.go internal/codexbridge/service.go internal/codexbridge/service_test.go internal/codexbridge/mcp.go internal/codexbridge/mcp_test.go internal/codexbridge/conformance.go internal/codexbridge/conformance_test.go internal/codexbridge/integration.go internal/codexbridge/integration_test.go internal/codexbridge/appserver/observation_test.go internal/codexbridge/hook/input.go internal/codexbridge/hook/input_test.go internal/codexbridge/hook/output.go internal/codexbridge/hook/output_test.go internal/cli/bridge.go internal/cli/bridge_test.go internal/cli/provider_inputs.go internal/cli/provider_inputs_test.go internal/cli/workflow.go internal/cli/workflow_test.go internal/dogfood/config.go internal/dogfood/coordinator.go internal/dogfood/current.go internal/dogfood/dogfood_test.go internal/dogfood/filesystem.go internal/dogfood/records.go internal/dogfood/repository.go internal/hosttest/fixture.go internal/hosttest/provider.go internal/hosttest/workflow.go internal/hosttest/workflow_test.go internal/integration/codex_bridge_conformance_test.go internal/integration/codex_bridge_blackbox_test.go internal/integration/config_discovery_test.go internal/integration/core_coordinator_cutover_test.go internal/integration/host_configuration_test.go internal/integration/external_host_transcript_test.go internal/integration/host_conformance_test.go internal/integration/workflow_coordinator_test.go internal/assets/generate/codex_host.go internal/assets/generate/codex_host_test.go internal/assets/generate/main.go internal/assets/embed_test.go internal/builtin/load_test.go
~~~

Expected: exit 0; no directory or pattern is passed to gofmt.

- [ ] **Step 8: Run GREEN focused Bridge gates**

Run:

~~~bash
rtk go test ./internal/codexbridge ./internal/codexbridge/appserver ./internal/codexbridge/hook ./internal/cli ./internal/dogfood ./internal/hosttest ./internal/integration ./internal/assets ./internal/assets/generate ./internal/builtin -count=1
rtk bash tests/18-codex-bridge-protocol-test.sh
rtk bash scripts/check-codex-bridge.sh
~~~

Expected: PASS. scripts/check-codex-bridge.sh is Bridge-only at this point: focused tests, race, at least 80.0 percent aggregate statement coverage for changed Bridge executable packages, focused vet, black-box protocol checks, and the strict production legacy scan. It must not call the documentation gate owned by Task 2.

- [ ] **Step 9: Verify the atomic diff and commit it once**

Run:

~~~bash
rtk git diff --check
rtk git diff --name-only
rtk git add -- internal/codexbridge/protocol.go internal/codexbridge/protocol_test.go internal/codexbridge/version.go internal/codexbridge/version_test.go internal/codexbridge/cache.go internal/codexbridge/cache_test.go internal/codexbridge/bindings.go internal/codexbridge/bindings_test.go internal/codexbridge/facts.go internal/codexbridge/facts_test.go internal/codexbridge/evidence.go internal/codexbridge/inputs.go internal/codexbridge/service.go internal/codexbridge/service_test.go internal/codexbridge/mcp.go internal/codexbridge/mcp_test.go internal/codexbridge/conformance.go internal/codexbridge/conformance_test.go internal/codexbridge/integration.go internal/codexbridge/integration_test.go internal/codexbridge/appserver/observation_test.go internal/codexbridge/hook/input.go internal/codexbridge/hook/input_test.go internal/codexbridge/hook/output.go internal/codexbridge/hook/output_test.go internal/cli/bridge.go internal/cli/bridge_test.go internal/cli/provider_inputs.go internal/cli/provider_inputs_test.go internal/cli/workflow.go internal/cli/workflow_test.go internal/dogfood/config.go internal/dogfood/coordinator.go internal/dogfood/current.go internal/dogfood/dogfood_test.go internal/dogfood/filesystem.go internal/dogfood/records.go internal/dogfood/repository.go internal/integration/codex_bridge_conformance_test.go internal/integration/codex_bridge_blackbox_test.go internal/integration/config_discovery_test.go internal/integration/core_coordinator_cutover_test.go internal/integration/host_configuration_test.go internal/integration/external_host_transcript_test.go internal/integration/host_conformance_test.go internal/integration/workflow_coordinator_test.go internal/integration/testdata/core-coordinator/acme-provider.json internal/integration/testdata/core-coordinator/acme-profile.json internal/integration/testdata/core-coordinator/old-v1-revision.json internal/hosttest/fixture.go internal/hosttest/provider.go internal/hosttest/workflow.go internal/hosttest/workflow_test.go internal/assets/generate/codex_host.go internal/assets/generate/codex_host_test.go internal/assets/generate/main.go internal/assets/host-integrations.json internal/assets/conformance/codex-host-v3.json internal/assets/embed_test.go internal/builtin/load_test.go internal/codexbridge/install/assets/skills/oaw-codex-bridge/SKILL.md scripts/check-codex-bridge.sh scripts/check-core-coordinator-coverage.sh tests/16-core-coordinator-conformance-test.sh tests/18-codex-bridge-protocol-test.sh
rtk git diff --cached --name-only
rtk git diff --cached --check
rtk git commit -m "feat: hard cut codex bridge to provider surface v4"
~~~

Expected: the cached-name output is exactly the 65 paths in the Atomic Bridge Cutover table, no policy/doc/scratch/user-owned path is staged, and the commit contains all old-to-new Bridge and downstream consumer references together.

## Task 2: Publish The Corrected Policy And Bilingual Matrix

**Files:** Every path in the Policy And Documentation table, with exactly the listed Modify disposition.

- [ ] **Step 1: Add RED policy and parity assertions**

Update scripts/check-docs.sh and tests/10-docs-test.sh first. Require the ten canonical slots, all four aliases, USER-DEFINED, the exact Matt/Superpowers/ECC upstream revision pins from Plan 04, all v4/v3/v2 contract versions, Host actions, neutral gates, Binding kinds, macro deduplication, and truthful current Codex exclusions.

The tests must reject these claims:

1. Matt requirements or verification-loop as shipped skills;
2. Matt owning workspace creation, broad final verification, remediation, or completion;
3. ECC e2e-runner or e2e-testing as broad final verification;
4. ECC code-reviewer as closeout/completion;
5. a Claude custom Agent name presented as a verified Codex Role;
6. static multi-agent configuration presented as live delegation;
7. any compatibility reader, alias removal, or Provider-brand selection.

- [ ] **Step 2: Run RED documentation gates**

Run:

~~~bash
rtk bash scripts/check-docs.sh
rtk bash tests/10-docs-test.sh
~~~

Expected: FAIL on the old flat responsibility map, stale versions, fictional Matt names, and Host eligibility claims.

- [ ] **Step 3: Rewrite policy from the machine-readable matrix**

Use internal/assets/profile-matrix.json and the pinned Plan 04 descriptors/Recipes as the source of truth. Keep DIRECT, BOUNDED, WORKFLOW, and the blocking Startup Gate. Replace the flat stage-owner narrative with the ten ordered slots, multi-Binding pipelines, macro/internal-call deduplication, typed incident routes, Host actions, neutral gates, alternatives, overlays, and fail-closed USER-DEFINED composition.

Document these exact built-in semantics:

| Profile | Required mapping |
| --- | --- |
| MATT-FULL | grill-with-docs expands grilling plus domain-modeling once; to-spec then to-tickets; implement includes Matt tdd and reporting review; diagnosing-bugs is functional incident handling; workspace, remediation, broad fresh verification, and closeout require separately verified Host procedures and otherwise remain exact gaps. |
| SP-FULL | brainstorming spans alignment/specification and calls writing-plans once; using-git-worktrees prepares the workspace; subagent-driven-development is the preferred macro when child delegation is live, with executing-plans as its declared alternative; test-driven-development, systematic-debugging, requesting-code-review, verification-before-completion, and finishing-a-development-branch retain their exact scopes. |
| ECC-FULL | Skills, Claude custom Agents, Codex Roles, Instructions, Hooks, and tools stay distinct; only exact Host-observed bindings compile; Codex never infers architect, planner, tdd-guide, build-error-resolver, or code-reviewer roles from Claude files; specialist E2E/review agents do not become broad verification or completion. |
| MATT-SP-HYBRID | Matt owns grill-with-docs, to-spec, to-tickets, tdd, and diagnosing-bugs; Superpowers owns writing-plans, using-git-worktrees, inline executing-plans, standalone review/remediation, verification-before-completion, and finishing-a-development-branch; an ECC build/type handler is only an explicitly selected Add-on. |

All four aliases remain active even when the current Host cannot compile one. Eligibility comes from verified Bindings, topology, delegation, invocation, Host actions, and user authority, never from deletion or compatibility fallback.

- [ ] **Step 4: Project exact English and Chinese documentation**

Keep the English and Chinese headings, tables, diagnostics, versions, and limitations in parity. Explain that FULL means the Provider-led lifecycle plus neutral Host/user controls, not Provider ownership of the Host. Explain USER-DEFINED as a versioned Recipe assembled from installed, trusted, Host-verified compatible Bindings, with exactly one outcome owner and no silent default.

In both languages distinguish installation integrity from live proof: bridge check reports managed files and registration only; a fresh observe_current negotiation proves the active Bridge v2 tuple. Document exact recovery for HOST_BRIDGE_PROTOCOL_MISMATCH, HOST_SESSION_CHANGED, PROVIDER_BINDING_CONTENT_MISMATCH, BINDING_EXPLICIT_INVOCATION_REQUIRED, HOST_FEATURE_UNATTESTED, HOST_ACTION_UNAVAILABLE, MACRO_INTERNAL_CONFLICT, PROFILE_TOPOLOGY_UNAVAILABLE, and WORKFLOW_STATE_UNSUPPORTED.

- [ ] **Step 5: Run GREEN and inspect the documentation diff**

Run:

~~~bash
rtk bash scripts/check-docs.sh
rtk bash tests/10-docs-test.sh
rtk git diff --check
~~~

Expected: PASS with matching English/Chinese contract claims, all source-pinned names present, all forbidden fictional claims absent, and no compatibility-reader language.

- [ ] **Step 6: Commit the exact documentation surface**

Run:

~~~bash
rtk git add -- policy/ENGINEERING.md README.md README-zh.md docs/en/architecture.md docs/en/lifecycle.md docs/en/comparison.md docs/en/adapters.md docs/en/codex-bridge.md docs/en/extending-adapters.md docs/en/security.md docs/en/troubleshooting.md docs/zh/architecture.md docs/zh/lifecycle.md docs/zh/comparison.md docs/zh/adapters.md docs/zh/codex-bridge.md docs/zh/extending-adapters.md docs/zh/security.md docs/zh/troubleshooting.md scripts/check-docs.sh tests/10-docs-test.sh
rtk git diff --cached --name-only
rtk git diff --cached --check
rtk git commit -m "docs: publish provider surface v4 policy"
~~~

Expected: the cached-name output is exactly the 21 paths in the Policy And Documentation table and no Bridge code, scratch evidence, or unrelated user path is staged.

## Task 3: Complete Independent Correctness And Security Review

**Files:** Modify lib/detect.sh, internal/check/diagnostics_test.go, internal/schema/registry_v2_test.go, tests/02-check-test.sh, and tests/11-check-parity-test.sh. Create .scratch/oaw-provider-surface-v4/evidence/review.md and .scratch/oaw-provider-surface-v4/evidence/reviewed-paths.txt. Any tracked remediation path must be made exact through the ledger rule in the File Map before editing.

- [ ] **Step 1: Run the fixed-point correctness review**

Use superpowers:requesting-code-review against the approved v4 design, Plans 01-06, the implementation baseline, and the complete committed diff. Require separate spec-compliance and code-quality sections. Record baseline commit, reviewed head, exact command/diff, severity, file/line, evidence, disposition, and re-review status in review.md.

The initial review gate is RED whenever any validated CRITICAL or HIGH finding remains open. MEDIUM and LOW findings require an explicit fix or reasoned disposition; silence is not a disposition.

- [ ] **Step 2: Run the security review**

Use the project security-review skill on complete-tree hashing, path containment, symlink and drift handling, strict decoding, canonical digests, v2 handle secrecy, App Server allowlisting, Host-owned authorization/invocation/gates, network-mutate, Coordinator journal/state, generated assets, and public-output redaction. Search for hardcoded credentials, absolute private paths, raw Hook payloads, and model transcripts.

Expected: every finding is recorded in review.md. Any CRITICAL/HIGH finding keeps the gate RED.

- [ ] **Step 3: Convert each validated remediation into an exact TDD entry**

Before editing a tracked file:

1. confirm its exact path already appears in a Plan 01-06 File Map;
2. write that exact path once, in byte-sorted order, to reviewed-paths.txt;
3. record Modify, Create, or Delete plus the exact failing test command and expected failure in review.md;
4. run the test RED;
5. implement the smallest fix;
6. run the exact test GREEN and the owning task gate;
7. request a fresh review of the fixed-point diff.

Reject blank lines, absolute paths, directory paths, leading dash, colon pathspec magic, dot-dot components, wildcard characters, duplicate paths, .scratch paths, .serena paths, CONTEXT.md, frame.json, and the root oaw binary from reviewed-paths.txt. Do not edit or stage a path before this validation.

- [ ] **Step 4: Stage reviewed remediation deterministically**

Run:

~~~bash
rtk git diff --cached --quiet
rtk git add --pathspec-from-file=.scratch/oaw-provider-surface-v4/evidence/reviewed-paths.txt
rtk git diff --cached --name-only
rtk git diff --cached --name-only | rtk diff -u .scratch/oaw-provider-surface-v4/evidence/reviewed-paths.txt -
rtk git diff --cached --check
~~~

Expected: the index starts empty; reviewed-paths.txt is sorted and unique; cached names equal it byte-for-byte; every cached path has a review.md disposition and RED/GREEN evidence; no open CRITICAL/HIGH finding remains.

- [ ] **Step 5: Commit reviewed fixes or prove no remediation commit is needed**

When reviewed-paths.txt is non-empty, run:

~~~bash
rtk git commit -m "fix: address provider surface review"
~~~

If a later mandatory Task 4 gate exposes a new stable hard-cut regression,
return to Task 3 before any tracked edit, extend the File Map and ledger through
the same review rule, commit one additional exact remediation, and restart Task
4 from Step 1. Task 4 itself remains verification-only.

When reviewed-paths.txt is empty, run:

~~~bash
rtk git diff --cached --quiet
rtk git status --short
~~~

Expected: either at least one exact reviewed-remediation commit exists, or the empty-index evidence is recorded in review.md. Every reopened remediation is separately reviewed and committed. Scratch review files remain untracked in all cases.

## Task 4: Run The First Post-Cutover Full-Tree Gate

**Files:** Create .scratch/oaw-provider-surface-v4/evidence/verification.md. This task makes no tracked edit and creates no commit.

Plan 05's focused Core/Coordinator gates are prerequisites. They do not substitute for this task's Bridge, race, coverage, full vet, full Go, shell, and documentation gates. This is the first gate after all authority producers and consumers have been cut over together.

- [ ] **Step 1: Re-run focused Core, Coordinator, Bridge, and documentation gates**

Run:

~~~bash
rtk go test ./internal/catalog ./internal/profile ./internal/registry ./internal/discovery ./internal/host ./internal/core ./internal/admission ./internal/coordinator ./internal/schema ./internal/builtin ./internal/integration -count=1
rtk go test ./internal/codexbridge ./internal/codexbridge/appserver ./internal/codexbridge/hook ./internal/cli ./internal/assets ./internal/assets/generate -count=1
rtk bash scripts/check-core-coordinator-coverage.sh
rtk bash scripts/check-codex-bridge.sh
rtk bash scripts/check-docs.sh
rtk bash tests/10-docs-test.sh
rtk bash tests/16-core-coordinator-conformance-test.sh
rtk bash tests/18-codex-bridge-protocol-test.sh
~~~

Expected: PASS. Both changed Core/Coordinator and Bridge executable packages report at least 80.0 percent aggregate statement coverage.

- [ ] **Step 2: Run race detection**

Run:

~~~bash
rtk go test -race ./internal/profile ./internal/registry ./internal/host ./internal/core ./internal/admission ./internal/coordinator ./internal/codexbridge/... ./internal/cli ./internal/integration -count=1
~~~

Expected: PASS with no race, timeout, leaked state lock, reused handle, or nondeterministic generated asset.

- [ ] **Step 3: Run full vet, full Go tests, and the repository shell suite**

Run:

~~~bash
rtk go vet ./...
rtk go test ./... -count=1
rtk bash tests/run.sh
~~~

Expected: PASS. A platform or Docker sub-gate may return its already documented unavailable status only when the gate itself proves the dependency is unavailable; an ordinary failure is not converted to unavailable.

- [ ] **Step 4: Hard-fail on every stale production authority identifier**

Run:

~~~bash
rtk rg -n 'oaw\.provider-descriptor/v3|oaw\.profile-recipe/v2|oaw\.provider-instance/v3|oaw\.provider-resolution-report/v3|oaw\.effective-registry/v3|oaw\.host-manifest/v2|oaw\.host-session/v2|oaw\.host-binding-inventory/v2|oaw\.host-invocation-receipt/v2|oaw\.host-conformance-(transcript|report)/v[23]|oaw\.execution-graph/v3|oaw\.lifecycle-bundle/v3|oaw\.capability-grant/v2|oaw\.dispatch-packet/v1|oaw\.workflow-(command|result|snapshot|revision)/v1|oaw\.codex-bridge/v1|oaw\.codex-hook-context/v1|oaw\.host-evidence-handle/v1' cmd internal --glob '!**/*_test.go' --glob '!**/testdata/**' --glob '!internal/assets/audits/**' --glob '!internal/assets/schemas/v1/dispatch-packet.schema.json' --glob '!internal/assets/schemas/v1/workflow-command.schema.json' --glob '!internal/assets/schemas/v1/workflow-result.schema.json' --glob '!internal/assets/schemas/v1/workflow-snapshot.schema.json' --glob '!internal/assets/schemas/v1/workflow-revision.schema.json' --glob '!internal/assets/schemas/v2/host-manifest.schema.json' --glob '!internal/assets/schemas/v2/host-session.schema.json' --glob '!internal/assets/schemas/v2/host-binding-inventory.schema.json' --glob '!internal/assets/schemas/v2/host-invocation-receipt.schema.json' --glob '!internal/assets/schemas/v2/host-conformance-transcript.schema.json' --glob '!internal/assets/schemas/v2/host-conformance-report.schema.json' --glob '!internal/assets/schemas/v2/capability-grant.schema.json' --glob '!internal/assets/schemas/v3/host-conformance-transcript.schema.json' --glob '!internal/assets/schemas/v3/host-conformance-report.schema.json'
~~~

Expected: no output and rg exit 1. The only permitted literals are in explicit rejection tests, testdata, internal/assets/audits, the individually named inert superseded schema files, and historical docs under docs/superpowers; those locations are excluded deliberately. Active schemas, generators, built-in assets, production Go, CLI code, Bridge installed assets, and Host integrations are not excluded.

Run the Provider-claim scan:

~~~bash
rtk rg -n '"requirements"|"verification-loop"' internal/assets/providers/oaw-matt.json
rtk rg -n 'e2e-runner.*(broad|fresh).*verification|code-reviewer.*(completion|closeout)|delivery-gate.*(completion|closeout)' policy README.md README-zh.md docs/en docs/zh
~~~

Expected: no output. General prose about requirements is allowed; the quoted JSON-name scan prevents only fictional Matt Binding IDs.
Machine-readable Provider ownership is enforced structurally by the focused
`internal/builtin` and generated-asset tests in Step 1. The second scan is
deliberately prose-only: applying a relationship regex to minified JSON would
join unrelated fields on the same physical line and produce false positives.

- [ ] **Step 5: Record fresh evidence and prove no verification commit exists**

For every command record UTC timestamp, exact command, exit status, secret-free output digest, implementation baseline, reviewed HEAD, coverage total, and any documented unavailable sub-gate in verification.md. Store no raw Host handle, credential, absolute Binding path, full Hook payload, or model transcript.

Run:

~~~bash
rtk git diff --cached --quiet
rtk git diff --check
rtk git status --short
~~~

Expected: no verification-only tracked change is staged or committed; only the declared scratch evidence and pre-existing unrelated user paths remain untracked or modified.

## Task 5: Re-Observe The Active Host And Submit One New START

**Files:** Create .scratch/oaw-provider-surface-v4/evidence/host-summary.json and .scratch/oaw-provider-surface-v4/evidence/start-result.json. This task makes no tracked edit and creates no commit.

- [ ] **Step 1: Check installation integrity without claiming live protocol proof**

Run:

~~~bash
rtk go run ./cmd/oaw bridge check codex --format text
~~~

Expected: the management result reports OAW-owned files, Marketplace registration, Plugin registration, installed version, and whether a new session is required. This command proves installation integrity only. It does not negotiate VersionEvidence and must never be cited as live protocol proof.

If drift, a version mismatch, or a required new session is reported, stop before START. Bridge update, Plugin/Marketplace mutation, and starting a replacement Host session require the user's explicit authorization; this plan grants none of them.

- [ ] **Step 2: Obtain the live proof with a fresh observe_current call**

Call observe_current once through the active Codex Host Bridge. Require the complete canonical VersionEvidence tuple above and a new opaque v2 handle bound to the current session and CWD.

Expected: either observation succeeds with Bridge v2 and secret-free Host facts, or it returns HOST_BRIDGE_PROTOCOL_MISMATCH or another exact closed diagnostic. A v1 context, v1 handle, stale digest, old schema tuple, missing skills/list method, or old session cannot continue. Keep the handle only in active process/session memory.

Write host-summary.json with only Provider state, short fact digests, CURRENT topology, Binding-kind availability, feature/action dispositions, diagnostics, and VersionEvidence digest. Exclude the handle token, absolute paths, credentials, raw Hook data, and model transcript.

- [ ] **Step 3: Inspect eligibility and bind the already approved selection**

Call core.inspect with the fresh handle and approved WORKFLOW classification evidence. Confirm that its inspection has no implicit selection, exposes all four aliases plus USER-DEFINED Builder data, and reports exact current-Host exclusions.

Use only the already confirmed SP-FULL / CURRENT / no Add-on selection and the exact confirmation digest returned for that choice. Do not switch Profile, add an Add-on, choose SUBAGENT, infer delegation, or reuse a digest from an earlier observation.

- [ ] **Step 4: Submit exactly one Coordinator START v2**

Call workflow_exchange with:

1. Workflow Command schema v2;
2. a newly generated idempotency key;
3. empty Workflow ID;
4. expected revision 0;
5. the fresh in-memory v2 Host handle;
6. the approved request/deliverable/classification evidence;
7. the exact SP-FULL / CURRENT / no Add-on selection and confirmation digest.

Never resume workflow-88833e4868d3202599342e2c5c133f82. Do not retry START with another key after an exact rejection.

Expected outcome is exactly one of:

~~~text
accepted: Workflow Result/Snapshot/Revision v2 at revision 1 with Lifecycle Bundle v4 and Execution Graph v4
rejected: one stable diagnostic set, with no executable old or partially compiled new Bundle
~~~

- [ ] **Step 5: Persist only the secret-free result**

Write start-result.json with result kind, new Workflow ID when committed, revision, Profile/topology/Add-on identity, Bundle/Graph schema plus digest when accepted, or exact diagnostics when rejected. Exclude the Host handle, idempotency key, absolute Binding paths, credentials, raw evidence bodies, Hook payload, and model transcript.

- [ ] **Step 6: Verify repository and external-action boundaries**

Run:

~~~bash
rtk git status --short
rtk git diff --check
rtk git diff --cached --check
~~~

Expected: no Provider installation, Bridge update, user-config mutation, push, publication, merge, credential change, destructive action, or unrelated user-file change occurred. The two operational evidence files remain untracked and are not committed.

## Final Acceptance

- [ ] All 17 approved v4 design criteria point to a passing test, review record, or fresh evidence digest.
- [ ] MATT-FULL, SP-FULL, ECC-FULL, and MATT-SP-HYBRID remain active catalog aliases with source-correct Bindings.
- [ ] USER-DEFINED can combine installed trusted Host-verified compatible Bindings per slot without a brand-specific code path.
- [ ] Bridge protocol, Hook Context, handle, Integration, generated assets, CLI, and Workflow exchange cut over atomically.
- [ ] VersionEvidence separately proves Transcript v4 and Report v4 and retains every listed safety field.
- [ ] Old Provider, Recipe, Graph, Bundle, Grant, Receipt, Conformance, Dispatch, Workflow, Bridge, Hook, and handle authority is inert.
- [ ] Current Codex reports only skill/CURRENT and live facts proved by stable APIs.
- [ ] Host-owned user authorization, explicit invocation, and gate evidence cannot be forged through public MCP input.
- [ ] Plan 05 focused gates and Plan 06 Bridge/race/coverage/full-tree/docs/shell gates all pass.
- [ ] Independent correctness and security review have no open CRITICAL/HIGH finding.
- [ ] A fresh observe_current call, not bridge check, proves the live v2 tuple.
- [ ] Exactly one new SP-FULL / CURRENT / no Add-on START result is recorded without secret/private evidence leakage.
- [ ] No unrelated user work or external resource is modified.
