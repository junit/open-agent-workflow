# OAW Provider Surface v4 05: Core and Coordinator Cutover Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkboxes for tracking.

**Goal:** Make Core compile only Lifecycle Bundle v4, cut Admission and Host receipts to the exact Binding/action contract, and cut every Coordinator record, consumer, and durable-state reader to v2 in one atomic migration. Old authority remains inert audit material and is rejected before dispatch.

**Architecture:** Core is the only policy compiler. A graph contains ordered Provider Bindings, neutral Host actions, and gates. Admission grants one exact Provider Binding or Host action at one execution.GraphCursor; gates never receive execution authority. Coordinator persists the cursor and append-only Host attestations, validates typed output evidence, and refuses old state before decoding embedded authority.

**Selected lifecycle:** SP-FULL / CURRENT / no Add-on.

**Depends on:** Plans 01-04. Plan 02 supplies Host Manifest/Inventory/Session v3; Plan 03 supplies Graph v4 and the package-neutral cursor; Plan 04 supplies the corrected built-in matrix. No compatibility reader, conversion, fallback, or alias remapping is permitted.

**Produces:** Lifecycle Bundle v4, Grant v3, Receipt v3, Conformance v4, Dispatch and Workflow state v2, plus package-local proof that old authority cannot dispatch. Plan 06 restores external consumers and whole-repository conformance.

## Contract and Version Lock

| Contract | Exact identifier | Owner |
| --- | --- | --- |
| Execution Graph | oaw.execution-graph/v4 | Plan 03, consumed here |
| Lifecycle Bundle | oaw.lifecycle-bundle/v4 | Task 1 |
| Capability Grant | oaw.capability-grant/v3 | Task 2 |
| Host Invocation Receipt | oaw.host-invocation-receipt/v3 | Task 2 |
| Host Conformance Transcript | oaw.host-conformance-transcript/v4 | Task 2 |
| Host Conformance Report | oaw.host-conformance-report/v4 | Task 2 |
| User Authorization | oaw.user-authorization/v1 | Tasks 2-3 |
| Explicit Invocation Attestation | oaw.explicit-invocation-attestation/v1 | Tasks 2-3 |
| Gate Attestation | oaw.gate-attestation/v1 | Task 3 |
| Dispatch Packet | oaw.dispatch-packet/v2 | Task 3 |
| Workflow Command | oaw.workflow-command/v2 | Task 3 |
| Workflow Result | oaw.workflow-result/v2 | Task 3 |
| Workflow Snapshot | oaw.workflow-snapshot/v2 | Task 3 |
| Workflow Revision | oaw.workflow-revision/v2 | Task 3 |
| Workflow Head | oaw.workflow-head/v1 | Pointer-only; verifies pointed Revision v2 first |

Grant v2, Receipt v2, Conformance Transcript/Report v3, Graph/Bundle v3, Dispatch v1, and Workflow Command/Result/Snapshot/Revision v1 are rejection fixtures only. They must not be active registry entries or production readers.

## Atomic Cutover Rule

Coordinator v2 is one cutover unit. Do not land or declare GREEN while any in-package engine, traversal, journal, transport, lease, projection, switch, cancel, recovery path, or test still reads v1 fields. The former Tasks 3-5 are merged into Task 3 with one owner and one final commit.

Old bytes remain with their modes and timestamps as audit history. No old record is reinterpreted, converted, or dispatched. A fresh START may create an independent v2 Workflow in the same state root without reading the old Workflow.

## File Map

### Core v4

| Path | Responsibility |
| --- | --- |
| internal/core/records.go | Exact selection and Lifecycle Bundle v4 |
| internal/core/compile.go | One inspect/compile path |
| internal/core/resolve.go | Validate Host Inventory v3 and Registry v4 inputs for Core |
| internal/core/core_test.go | Alias, USER-DEFINED, topology, evidence, and digest tests |

### Admission and Host authority

| Path | Responsibility |
| --- | --- |
| internal/admission/records.go | Grant v3 and Host-owned attestation records |
| internal/admission/admit.go | Target narrowing and authority checks |
| internal/admission/admission_test.go | Target, attestation, nonce, and old-grant tests |
| internal/host/records.go | Receipt v3, typed outputs, Conformance v4 records |
| internal/host/validate.go | Closed constructors and old-record rejection |
| internal/host/receipt.go | Receipt normalization and kind/output invariants |
| internal/host/receipt_test.go | Cursor, target, output, and old-receipt tests |
| internal/host/conformance.go | Transcript/Report v4 |
| internal/host/conformance_test.go | Conformance v4 and rejection tests |
| internal/host/conformance_fuzz_test.go | Closed conformance fuzzing |
| internal/host/records_test.go | Host constructors, cloning, hard cut |
| internal/assets/embed.go | Embed schema trees including schemas/v4/*.json |
| internal/assets/embed_test.go | Assert v4 embedding and inert old schemas |
| internal/assets/schemas/v3/host-integration.schema.json | Update Plan-02 schema to Report v4 |
| internal/schema/registry.go | Activate only new authority schemas |
| internal/schema/registry_test.go | Active-set and rejection tests |

### Atomic Coordinator v2

Production owner updates all of:

| Path | Responsibility |
| --- | --- |
| internal/coordinator/records.go | Command/Result/Snapshot/Revision/Dispatch v2 |
| internal/coordinator/engine.go | v2 exchange and cursor projection |
| internal/coordinator/start.go | START through Core Bundle v4 |
| internal/coordinator/dispatch.go | PREPARE and exact target dispatch |
| internal/coordinator/receipt.go | New receipt transition engine |
| internal/coordinator/switch.go | Generation switching |
| internal/coordinator/cancel.go | Cancellation and uncertain execution |
| internal/coordinator/journal.go | v2-only journal and pointer checks |
| internal/coordinator/transport.go | strict v2 transport |
| internal/coordinator/leases.go | Grant/generation/cursor leases |
| internal/coordinator/projection.go | cursor-based secret-free projection |

The same owner updates all tests: records_test.go, start_test.go, dispatch_test.go, receipt_test.go, switch_test.go, cancel_test.go, journal_test.go, transport_test.go, transport_internal_test.go, transport_fuzz_test.go, leases_test.go, projection_test.go, recovery_test.go, invariants_test.go, and validation_internal_test.go under internal/coordinator.

## Locked Authority Details

AuthorizationDecision is a closed enum with only allowed and denied.

~~~go
const (
    CapabilityGrantSchemaV3               = "oaw.capability-grant/v3"
    HostInvocationReceiptSchemaV3         = "oaw.host-invocation-receipt/v3"
    UserAuthorizationSchemaV1             = "oaw.user-authorization/v1"
    ExplicitInvocationAttestationSchemaV1 = "oaw.explicit-invocation-attestation/v1"
)

type GrantTargetKind string

const (
    GrantProviderBinding GrantTargetKind = "provider-binding"
    GrantHostAction      GrantTargetKind = "host-action"
)

type AuthorizationDecision string

const (
    AuthorizationAllowed AuthorizationDecision = "allowed"
    AuthorizationDenied  AuthorizationDecision = "denied"
)

type AuthorizationTarget struct {
    TargetKind      GrantTargetKind
    ProviderBinding *ProviderBindingAuthority
    HostAction      *HostActionAuthority
}

type OutputReference struct {
    ArtifactID string
    Schema     string
    Reference  string
    Digest     string
}
~~~

AuthorizationTarget is a closed oneOf: TargetKind and exactly one matching pointer are required. OutputReference lives in internal/host and is encoded with artifact_id, schema, reference, and digest.

| Record | Required locked fields |
| --- | --- |
| ProviderBindingAuthority | Provider ID and Instance digest; Distribution ID/revision/tree digest; Binding ID/surface/kind/reference/invocation/tree/evidence digests; input/output artifact IDs; input/outcome schemas; explicit-invocation requirement |
| HostActionAuthority | action ID; input/output artifact IDs; input/outcome schemas; maximum effects; resources; observation digest |
| UserAuthorization | schema, ID, issuer Host, Host session digest, evidence-handle digest, authorization nonce, Workflow/Bundle/generation/digest, cursor, AuthorizationTarget, decision, effects, resources, evidence, digest |
| ExplicitInvocationAttestation | schema, ID, issuer Host, Host session digest, evidence-handle digest, invocation nonce, Workflow/Bundle/generation/digest, cursor, exact ProviderBindingAuthority, evidence, digest |
| CapabilityGrant | schema, ID, Workflow/request/Bundle/generation/digest, cursor, target kind and one target, topology/session, effects/resources/termination, matching authorization and invocation-attestation digests, digest |
| InvocationReceipt | schema, kind, Workflow/Bundle/generation/digest, cursor, topology/session/dispatch, invocation handle/freshness/environment, outcome/failure, Outputs, evidence, digest |

UserAuthorization contains schema version, ID, issuer Host ID, Host session digest, evidence-handle digest, authorization nonce, Workflow/Bundle/generation/digest, cursor, an exact typed Provider Binding or Host action target, decision, effects, resources, evidence references, and digest.

ExplicitInvocationAttestation contains schema version, ID, issuer Host ID, Host session digest, evidence-handle digest, invocation nonce, Workflow/Bundle/generation/digest, cursor, the exact ProviderBindingAuthority target, evidence references, and digest.

ProviderBindingAuthority includes Provider Instance, Distribution ID/revision/tree digest, Binding ID/surface/kind/reference/invocation/tree/evidence digests, input/output artifact IDs, input/outcome schemas, and explicit-invocation requirement. HostActionAuthority includes action ID, input/output artifact IDs, input/outcome schemas, effects, resources, and observation digest.

Constructors reject malformed or noncanonical records. Coordinator histories reject duplicate ID, digest, or nonce and any reuse across Workflow/run, cursor, or Bundle generation. Idempotent replay returns the original result without appending. Capability Grant v3 retains the matching authorization and invocation-attestation digests.

network-mutate requires an allowed Host authorization whose exact target, effects, resources, Bundle, cursor, issuer, and current session match the Grant. Public Bridge input cannot author attestations; Plan 06 hydrates them only from the current trusted evidence handle.

Dispatch v2 carries the exact secret-free authorization and invocation records so Host execution can validate them. Public Bridge projection still excludes all three authority fields, including GateAttestation.

Host OutputReference contains ArtifactID, Schema, opaque Reference, and Digest. Provider/Host authority contains the expected output artifact/schema. A COMPLETED Receipt v3 must carry matching output and evidence. STARTED, PAUSED, FAILED, and CANCELLED cannot carry successful output. Coordinator validates cursor, dispatch, target, artifact, schema, reference digest, and evidence before advancing. Receipt v2 and NodeID remain rejected.

## Task 1: Compile Lifecycle Bundle v4

**Files:**
- Modify: internal/core/records.go
- Modify: internal/core/compile.go
- Modify: internal/core/resolve.go
- Modify: internal/core/core_test.go

- [ ] **Step 1: Write RED Core v4 tests**

Cover four aliases, one configured USER-DEFINED Recipe, both outer topologies, no Add-on, one incident Add-on, alternatives, overlays, confirmation mismatch, stale Provider/Host evidence, and current Codex exclusions. Assert every successful Bundle is v4 and embeds Graph v4.

- [ ] **Step 2: Run RED**

Run: rtk go test ./internal/core -run 'BundleV4|SelectionConfirmation|UserDefined|CurrentCodex'

Expected: FAIL because Core still constructs Bundle v3 and flat selections.

- [ ] **Step 3: Implement exact selection and one compiler path**

Canonicalize Add-ons as an identity set. Apply overlays only through Recipe precedence. Digest Profile, exact Recipe, topology, choices, Provider Instances, Host evidence, and graph preview. Use `profile.CompileProfile` for both inspection and exact Bundle creation. Consume expected ineligibility only through `CompileResult.Diagnostics()`; construct a Bundle only when `CompileResult.Graph()` returns `(record, true)`. Treat a non-nil compiler error as malformed trusted authority and never recover it as a selection diagnostic. Recommendations never populate selection.

- [ ] **Step 4: Run GREEN**

Run: rtk gofmt -w internal/core/records.go internal/core/compile.go internal/core/resolve.go internal/core/core_test.go

Then run: rtk go test ./internal/core

Expected: PASS.

- [ ] **Step 5: Commit Core v4**

~~~text
rtk git add -- internal/core/records.go internal/core/compile.go internal/core/resolve.go internal/core/core_test.go
rtk git commit -m "feat: compile lifecycle bundle v4"
~~~

## Task 2: Cut Admission, Receipt, and Conformance authority

**Files:**
- Modify: internal/admission/records.go
- Modify: internal/admission/admit.go
- Modify: internal/admission/admission_test.go
- Modify: internal/host/records.go
- Modify: internal/host/validate.go
- Modify: internal/host/receipt.go
- Modify: internal/host/receipt_test.go
- Modify: internal/host/conformance.go
- Modify: internal/host/conformance_test.go
- Modify: internal/host/conformance_fuzz_test.go
- Modify: internal/host/records_test.go
- Modify: internal/assets/embed.go
- Modify: internal/assets/embed_test.go
- Modify: internal/assets/schemas/v3/host-integration.schema.json
- Modify: internal/schema/registry.go
- Modify: internal/schema/registry_test.go
- Create: internal/assets/schemas/v3/capability-grant.schema.json
- Create: internal/assets/schemas/v3/host-invocation-receipt.schema.json
- Create: internal/assets/schemas/v4/host-conformance-transcript.schema.json
- Create: internal/assets/schemas/v4/host-conformance-report.schema.json
- Create: internal/assets/schemas/v1/user-authorization.schema.json
- Create: internal/assets/schemas/v1/explicit-invocation-attestation.schema.json

- [ ] **Step 1: Write RED authority and Host tests**

Cover Grant target oneOf, complete Provider/Host target pins, cursor, closed decisions, issuer/session/evidence handle, nonce freshness and reuse, exact network authorization, exact explicit invocation, typed outputs, receipt-kind rules, Conformance v4 pins, strict cloning, and old Grant/Receipt/Transcript rejection.

- [ ] **Step 2: Run RED**

Run: rtk go test ./internal/admission ./internal/host ./internal/schema ./internal/assets -run 'GrantV3|Authorization|ExplicitInvocation|ReceiptV3|ConformanceV4|OldAuthority'

Expected: FAIL because Grant/Receipt still use v2 flat node authority and Conformance still references Receipt v2.

- [ ] **Step 3: Implement exact Grant and Host attestations**

Issue one Grant for exactly one dispatchable profile.ResolvedBinding or profile.CompiledHostAction and one validated cursor. Reject credited and omitted units. Normalize non-empty effects/resources and require subsets of both the unit and Engine ceiling. Populate output artifact/schema from the pinned Capability/Binding or Host-action contract and reject a missing or inconsistent contract. Require exact Host authorization for network-mutate and exact fresh invocation attestation for human-explicit. Grant digest fields must equal the supplied record digests.

- [ ] **Step 4: Implement Receipt v3 and Conformance v4**

Replace NodeID with cursor. Validate opaque typed outputs, canonical evidence, and kind/outcome/output combinations. Re-pin Transcript/Report to Host Manifest/Inventory/Session v3 and Receipt v3. Update the Plan-02 Host Integration schema to reference ../v4/host-conformance-report.schema.json. Extend go:embed with schemas/v4/*.json.

- [ ] **Step 5: Activate only new authority schemas**

Register Grant v3, Receipt v3, Transcript/Report v4, User Authorization v1, and Explicit Invocation Attestation v1. Superseded files may remain inert but cannot be returned by the active registry.

- [ ] **Step 6: Run GREEN**

Run:

~~~text
rtk gofmt -w internal/admission/records.go internal/admission/admit.go internal/admission/admission_test.go internal/host/records.go internal/host/validate.go internal/host/receipt.go internal/host/receipt_test.go internal/host/conformance.go internal/host/conformance_test.go internal/host/conformance_fuzz_test.go internal/host/records_test.go internal/assets/embed.go internal/assets/embed_test.go internal/schema/registry.go internal/schema/registry_test.go
rtk go test ./internal/admission ./internal/host ./internal/schema ./internal/assets
~~~

Expected: PASS with old authority rejected.

- [ ] **Step 7: Commit Admission and Host authority**

Stage exactly the declared Task 2 paths, including schemas and embed changes,
then commit:

~~~text
rtk git add -- internal/admission/records.go internal/admission/admit.go internal/admission/admission_test.go internal/host/records.go internal/host/validate.go internal/host/receipt.go internal/host/receipt_test.go internal/host/conformance.go internal/host/conformance_test.go internal/host/conformance_fuzz_test.go internal/host/records_test.go internal/assets/embed.go internal/assets/embed_test.go internal/assets/schemas/v3/host-integration.schema.json internal/assets/schemas/v3/capability-grant.schema.json internal/assets/schemas/v3/host-invocation-receipt.schema.json internal/assets/schemas/v4/host-conformance-transcript.schema.json internal/assets/schemas/v4/host-conformance-report.schema.json internal/assets/schemas/v1/user-authorization.schema.json internal/assets/schemas/v1/explicit-invocation-attestation.schema.json internal/schema/registry.go internal/schema/registry_test.go
rtk git commit -m "feat: cut host invocation authority to v3"
~~~

## Task 3: Atomic Coordinator v2 records, execution, and recovery

The former record, traversal, and hard-rejection tasks are one task. internal/coordinator/receipt.go is Create, not Modify.

**Production:**
- Modify: internal/coordinator/records.go
- Modify: internal/coordinator/engine.go
- Modify: internal/coordinator/start.go
- Modify: internal/coordinator/dispatch.go
- Create: internal/coordinator/receipt.go
- Modify: internal/coordinator/switch.go
- Modify: internal/coordinator/cancel.go
- Modify: internal/coordinator/journal.go
- Modify: internal/coordinator/transport.go
- Modify: internal/coordinator/leases.go
- Modify: internal/coordinator/projection.go

**Tests:**
- Modify: internal/coordinator/records_test.go
- Modify: internal/coordinator/start_test.go
- Modify: internal/coordinator/dispatch_test.go
- Modify: internal/coordinator/receipt_test.go
- Modify: internal/coordinator/switch_test.go
- Modify: internal/coordinator/cancel_test.go
- Modify: internal/coordinator/journal_test.go
- Modify: internal/coordinator/transport_test.go
- Modify: internal/coordinator/transport_internal_test.go
- Modify: internal/coordinator/transport_fuzz_test.go
- Modify: internal/coordinator/leases_test.go
- Modify: internal/coordinator/projection_test.go
- Modify: internal/coordinator/recovery_test.go
- Modify: internal/coordinator/invariants_test.go
- Modify: internal/coordinator/validation_internal_test.go

**Schemas:**
- Create: internal/assets/schemas/v2/dispatch-packet.schema.json
- Create: internal/assets/schemas/v2/workflow-command.schema.json
- Create: internal/assets/schemas/v2/workflow-result.schema.json
- Create: internal/assets/schemas/v2/workflow-snapshot.schema.json
- Create: internal/assets/schemas/v2/workflow-revision.schema.json
- Create: internal/assets/schemas/v1/gate-attestation.schema.json
- Modify: internal/schema/registry.go
- Modify: internal/schema/registry_test.go

Lock these Coordinator constants before changing record fields:

~~~go
const (
    WorkflowCommandSchemaV2  = "oaw.workflow-command/v2"
    WorkflowResultSchemaV2   = "oaw.workflow-result/v2"
    WorkflowSnapshotSchemaV2 = "oaw.workflow-snapshot/v2"
    WorkflowRevisionSchemaV2 = "oaw.workflow-revision/v2"
    WorkflowHeadSchemaV1     = "oaw.workflow-head/v1"
    DispatchPacketSchemaV2   = "oaw.dispatch-packet/v2"
    GateAttestationSchemaV1  = "oaw.gate-attestation/v1"
)
~~~

GateAttestation is a closed record containing schema, Workflow/Bundle/generation/digest, exact cursor, Gate ID, declared catalog.GateAuthority, a satisfied or rejected decision, canonical Host evidence references, and digest. It cannot carry execution authority and never appears in a Grant.

- [ ] Write the complete RED inventory before production edits: versions, closed JSON, cursor lookup, Grant/Bundle pins, Dispatch target and attestations, gate-only advancement, ordered traversal, outputs, append-only histories, duplicate/reuse, leases, projection, switch/cancel/recovery, strict transport, and old-state refusal.
- [ ] Run: rtk go test ./internal/coordinator -run 'RecordV2|DispatchV2|Cursor|GateAttestation|AuthorizationHistory|OutputEvidence|Switch|Cancel|Recovery|OldState'

Expected: FAIL because records, engine, journal, and tests still use v1 and graph-node identity.

- [ ] Replace all Coordinator wire records together. Snapshot v2 uses Cursor and adds UserAuthorizations, InvocationAttestations, and GateAttestations append-only histories. Dispatch v2 carries TargetKind, Grant, and exact optional Host authorization and invocation records. Workflow Command also advances to v2 because START and PREPARE wire meaning changes.
- [ ] PREPARE allows empty effects/resources only for execution.CursorGate, requires one exact GateAttestation, commits state only, and returns no Grant/Dispatch.
- [ ] Use `profile.FirstActionableCursor`, `profile.UnitAtCursor`, and `profile.NextActionableCursor`. Consume `TraversalResult` exactly: `next` advances only to its non-nil cursor; `terminal`, `stop`, and `replan` require a nil cursor and enter only their corresponding Coordinator transition. Never recalculate ordinals, anchors, incident returns, or fallbacks. Dispatch ordered Coordinator Bindings and Host actions; gates, credited units, and omitted units never dispatch.
- [ ] Move receipt processing to new receipt.go. Validate active Grant/Dispatch, output/evidence closure, sequence, incident, boundary, and next cursor before mutation.
- [ ] Switch revokes old generation at a stable boundary. Cancel preserves uncertain invocation until terminal Host evidence. Leases pin Grant/generation/cursor. Projection excludes raw attestations, handles, credentials, and paths.
- [ ] Journal reads schema discriminators first. Workflow Head v1 is pointer-only and must confirm its pointed Revision is exactly v2 before embedded decode. Old state returns WORKFLOW_STATE_UNSUPPORTED without byte/mode/time changes. No compatibility path.
- [ ] Register only Coordinator v2 schemas plus pointer-only Head v1.
- [ ] Run gofmt on every Task 3 Go file:

~~~text
rtk gofmt -w internal/coordinator/records.go internal/coordinator/engine.go internal/coordinator/start.go internal/coordinator/dispatch.go internal/coordinator/receipt.go internal/coordinator/switch.go internal/coordinator/cancel.go internal/coordinator/journal.go internal/coordinator/transport.go internal/coordinator/leases.go internal/coordinator/projection.go internal/coordinator/records_test.go internal/coordinator/start_test.go internal/coordinator/dispatch_test.go internal/coordinator/receipt_test.go internal/coordinator/switch_test.go internal/coordinator/cancel_test.go internal/coordinator/journal_test.go internal/coordinator/transport_test.go internal/coordinator/transport_internal_test.go internal/coordinator/transport_fuzz_test.go internal/coordinator/leases_test.go internal/coordinator/projection_test.go internal/coordinator/recovery_test.go internal/coordinator/invariants_test.go internal/coordinator/validation_internal_test.go internal/schema/registry.go internal/schema/registry_test.go
~~~

- [ ] Run the atomic package gate:

~~~text
rtk go test -race ./internal/coordinator ./internal/admission ./internal/host ./internal/schema ./internal/assets -count=1
~~~

Expected: PASS only after every production and test consumer is migrated.

- [ ] Commit all Task 3 files once:

~~~text
rtk git add -- internal/coordinator/records.go internal/coordinator/engine.go internal/coordinator/start.go internal/coordinator/dispatch.go internal/coordinator/receipt.go internal/coordinator/switch.go internal/coordinator/cancel.go internal/coordinator/journal.go internal/coordinator/transport.go internal/coordinator/leases.go internal/coordinator/projection.go internal/coordinator/records_test.go internal/coordinator/start_test.go internal/coordinator/dispatch_test.go internal/coordinator/receipt_test.go internal/coordinator/switch_test.go internal/coordinator/cancel_test.go internal/coordinator/journal_test.go internal/coordinator/transport_test.go internal/coordinator/transport_internal_test.go internal/coordinator/transport_fuzz_test.go internal/coordinator/leases_test.go internal/coordinator/projection_test.go internal/coordinator/recovery_test.go internal/coordinator/invariants_test.go internal/coordinator/validation_internal_test.go internal/assets/schemas/v2/dispatch-packet.schema.json internal/assets/schemas/v2/workflow-command.schema.json internal/assets/schemas/v2/workflow-result.schema.json internal/assets/schemas/v2/workflow-snapshot.schema.json internal/assets/schemas/v2/workflow-revision.schema.json internal/assets/schemas/v1/gate-attestation.schema.json internal/schema/registry.go internal/schema/registry_test.go
rtk git commit -m "feat: atomically cut coordinator to v2"
~~~

## Plan 06 Handoff

Plan 05 ends at package-local GREEN. Plan 06 owns these consumers:

| Path |
| --- |
| internal/cli/provider_inputs.go |
| internal/cli/provider_inputs_test.go |
| internal/cli/workflow.go |
| internal/cli/workflow_test.go |
| internal/codexbridge/conformance_test.go |
| internal/codexbridge/service.go |
| internal/codexbridge/service_test.go |
| internal/codexbridge/version.go |
| internal/codexbridge/version_test.go |
| internal/dogfood/config.go |
| internal/dogfood/coordinator.go |
| internal/dogfood/current.go |
| internal/dogfood/dogfood_test.go |
| internal/dogfood/filesystem.go |
| internal/dogfood/records.go |
| internal/dogfood/repository.go |
| internal/integration/codex_bridge_blackbox_test.go |
| internal/integration/config_discovery_test.go |
| internal/integration/core_coordinator_cutover_test.go |
| internal/integration/host_configuration_test.go |
| internal/integration/testdata/core-coordinator/acme-provider.json |
| internal/integration/testdata/core-coordinator/acme-profile.json |
| internal/integration/testdata/core-coordinator/old-v1-revision.json |
| internal/integration/external_host_transcript_test.go |
| internal/integration/host_conformance_test.go |
| internal/integration/workflow_coordinator_test.go |
| internal/hosttest/fixture.go |
| internal/hosttest/provider.go |
| internal/hosttest/workflow.go |
| internal/hosttest/workflow_test.go |
| tests/16-core-coordinator-conformance-test.sh |
| scripts/check-core-coordinator-coverage.sh |
| internal/assets/generate/codex_host.go |
| internal/assets/generate/codex_host_test.go |
| internal/assets/generate/main.go |
| internal/assets/host-integrations.json |
| internal/assets/conformance/codex-host-v3.json |
| internal/builtin/load_test.go |

Plan 06 updates them to the locked table, regenerates active Host integration/conformance assets, and keeps old assets only as inert audit/rejection material. It also owns go test ./internal/integration, go test ./..., go vet ./..., aggregate coverage, black-box gates, Bridge/CLI/dogfood verification, docs parity, independent review, fresh Host observation, and the new Coordinator START. Plan 05 must not run those gates.

## Phase Verification

- [ ] Run: rtk go test ./internal/core ./internal/admission ./internal/host ./internal/schema ./internal/assets ./internal/coordinator -count=1
- [ ] Run: rtk go test -race ./internal/admission ./internal/host ./internal/coordinator -count=1
- [ ] Scan owned production packages for old versions, NodeID, and ActiveNodeID:

~~~text
rtk rg -n 'lifecycle-bundle/v3|execution-graph/v3|capability-grant/v2|host-invocation-receipt/v2|host-conformance-(transcript|report)/v3|dispatch-packet/v1|workflow-(command|result|snapshot|revision)/v1|NodeID|ActiveNodeID' internal/core internal/admission internal/host internal/coordinator --glob '*.go' --glob '!**/*_test.go'
~~~

Expected: no production reader, dispatcher, or old authority field; rejection tests may mention them.
- [ ] Run: rtk git diff --check
- [ ] Confirm this planning edit changed only this Plan 05 file.

Expected: package-local Core, Admission, Host, schema, and complete Coordinator execute only v4/v3/v2 authority; old state is inert; external and full-tree gates belong to Plan 06.
