# OAW Runtime vNext Host Conformance and Capability Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Admit a Runtime-managed Workflow only through a digest-pinned, user- or built-in-trusted Host Integration whose official capability audit and deterministic Adapter Conformance Report prove every required Workflow guarantee.

**Architecture:** Add an inert `internal/host` domain for Host Manifests, trusted Integration Records, capability-audit evidence, effective per-run narrowing, and an executable in-process conformance fixture interface. Extend Configuration Snapshot to load built-in instruction-only records and user-trusted Integration Records, then replace Ticket 07's self-attested `PhysicalIsolation` boolean with a Host frame that only selects a pinned Integration and narrows temporarily unavailable features. Runtime pins the admitted Integration and Manifest digests in each Lifecycle Bundle and still never invokes a Host Binding; Ticket 09 remains responsible for selecting and driving the first production Runtime Host.

**Tech Stack:** Go 1.26, embedded Draft 2020-12 JSON Schemas, BurntSushi TOML, canonical JSON digests, existing Configuration Snapshot and Workflow journal, table-driven conformance fixtures, race tests, fuzzing, and repository Bash compatibility checks.

---

## Scope Boundary

Ticket 08 owns:

- closed Host Manifest, Host Integration, audit-evidence, Conformance Report,
  and effective Host-frame records;
- built-in instruction-only Integration Records for the currently supported
  instruction targets without promoting any target to Runtime-managed status;
- user-trusted Host Integration references loaded only from user configuration;
- deterministic conformance fixtures for isolated Executor creation, exact
  Binding invocation, pause, Bundle inheritance, normalized Evidence return,
  invocation deduplication, cancellation, and native invocation when claimed;
- Workflow admission from the pinned Configuration Snapshot and per-run
  narrowing only;
- explicit Lifecycle Bundle pins for Integration, Manifest, audit, and
  Conformance identities;
- stable denial codes for absent, instruction-only, unaudited, nonconforming,
  temporarily unavailable, or Binding-incompatible Host integrations.

It does not:

- select a first production Runtime Host;
- add `oaw run`, a machine JSON transport, a Host Driver, Hook, Plugin, MCP
  client, or third-party executable plugin;
- invoke a Provider Capability or Host Binding from Runtime;
- let project configuration register or trust a Host Integration;
- change Direct or Bounded admission, authority, or lease semantics;
- replace Host sandbox, filesystem, process, Git, network, or credential
  permissions;
- claim that instruction-only integrations provide Runtime isolation.

## Locked Domain Interfaces

The implementation introduces these public records in `internal/host`:

```go
const (
    HostManifestSchemaV1      = "oaw.host-manifest/v1"
    HostIntegrationSchemaV1   = "oaw.host-integration/v1"
    ConformanceReportSchemaV1 = "oaw.host-conformance-report/v1"
    ConformanceSuiteV1        = "oaw.host-conformance/v1"
)

type IntegrationLevel string

const (
    InstructionOnly IntegrationLevel = "instruction-only"
    RunnerManaged   IntegrationLevel = "runner-managed"
    NativeManaged   IntegrationLevel = "native-managed"
)

type Feature string

const (
    FeatureIsolatedExecutor      Feature = "isolated-executor"
    FeatureExactBindingInvocation Feature = "exact-binding-invocation"
    FeaturePause                  Feature = "pause"
    FeatureBundleInheritance     Feature = "bundle-inheritance"
    FeatureEvidenceReturn        Feature = "evidence-return"
    FeatureInvocationDedup       Feature = "invocation-deduplication"
    FeatureCancellation          Feature = "cancellation"
    FeatureNormalizedObservation Feature = "normalized-observation"
    FeatureNativeInvocation      Feature = "native-invocation"
)

type Manifest struct {
    SchemaVersion   string           `json:"schema_version" toml:"schema_version"`
    ManifestVersion string           `json:"manifest_version" toml:"manifest_version"`
    HostID          string           `json:"host_id" toml:"host_id"`
    IntegrationLevel IntegrationLevel `json:"integration_level" toml:"integration_level"`
    Protocols       []string         `json:"protocols" toml:"protocols"`
    BindingKinds    []string         `json:"binding_kinds" toml:"binding_kinds"`
    Features        []Feature        `json:"features" toml:"features"`
}

type AuditEvidenceReference struct {
    Reference string `json:"reference" toml:"reference"`
    Digest    string `json:"digest" toml:"digest"`
}

type AuditEvidence struct {
    Status     string                   `json:"status" toml:"status"`
    References []AuditEvidenceReference `json:"references" toml:"references"`
    Digest     string                   `json:"digest" toml:"digest"`
}

type ConformanceCheck struct {
    ID       string `json:"id" toml:"id"`
    Passed   bool   `json:"passed" toml:"passed"`
    Evidence string `json:"evidence" toml:"evidence"`
}

type ConformanceReport struct {
    SchemaVersion   string             `json:"schema_version" toml:"schema_version"`
    SuiteVersion    string             `json:"suite_version" toml:"suite_version"`
    IntegrationID  string             `json:"integration_id" toml:"integration_id"`
    ManifestDigest string             `json:"manifest_digest" toml:"manifest_digest"`
    Checks          []ConformanceCheck `json:"checks" toml:"checks"`
    TranscriptDigest string           `json:"transcript_digest" toml:"transcript_digest"`
    Passed          bool               `json:"passed" toml:"passed"`
    Digest          string             `json:"digest" toml:"digest"`
}

type IntegrationRecord struct {
    SchemaVersion      string             `json:"schema_version" toml:"schema_version"`
    IntegrationVersion string             `json:"integration_version" toml:"integration_version"`
    ID                 string             `json:"id" toml:"id"`
    Manifest           Manifest           `json:"manifest" toml:"manifest"`
    ManifestDigest     string             `json:"manifest_digest" toml:"manifest_digest"`
    Audit              AuditEvidence      `json:"audit" toml:"audit"`
    Conformance        *ConformanceReport `json:"conformance,omitempty" toml:"conformance"`
    Digest             string             `json:"digest" toml:"digest"`
}

type RuntimeFrame struct {
    IntegrationID      string    `json:"integration_id"`
    UnavailableFeatures []Feature `json:"unavailable_features"`
}
```

All constructors normalize sets, compute canonical content digests, return
defensive copies, and reject caller-provided digest forgery. Authored TOML/JSON
records must carry the computed digests so configuration cannot silently repair
tampering. Instruction-only records have pending audit evidence, no
Conformance Report, empty Runtime protocols/features/binding kinds, and can
never be admitted.

`WorkflowOptions.Host` becomes `host.RuntimeFrame`. `LifecycleBundle` gains:

```go
HostIntegrationID     string `json:"host_integration_id"`
HostIntegrationDigest string `json:"host_integration_digest"`
HostManifestDigest    string `json:"host_manifest_digest"`
HostAuditDigest       string `json:"host_audit_digest"`
HostConformanceDigest string `json:"host_conformance_digest"`
```

The Bundle fields are immutable history. An explicit stable-boundary Bundle
switch may adopt a different current trusted Host Integration only as part of
the new generation, just as it may adopt a new Configuration/Registry.

## Task 1: Define closed Host records and schemas

**Files:**
- Create: `internal/assets/schemas/v1/host-manifest.schema.json`
- Create: `internal/assets/schemas/v1/host-integration.schema.json`
- Modify: `internal/assets/embed.go`
- Modify: `internal/assets/embed_test.go`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`
- Create: `internal/host/records.go`
- Create: `internal/host/validate.go`
- Create: `internal/host/decode.go`
- Create: `internal/host/records_test.go`

- [x] **Step 1: Write failing schema and record tests.**

  Cover all three Integration Levels, the exact closed Feature vocabulary,
  deterministic set ordering, defensive copies, canonical Manifest/Audit/
  Report/Integration digests, strict JSON and TOML decoding, invalid UTF-8,
  trailing JSON, unknown fields, oversize input, duplicate values, invalid IDs,
  unsupported protocols or binding kinds, and digest tampering. Assert that an
  instruction-only record rejects Runtime protocols, binding kinds, Features,
  passed audit claims, or a Conformance Report. Assert runner/native records
  reject missing passed audit evidence, incomplete required Features, absent or
  failed Reports, stale suite versions, and Manifest/Integration mismatches.

- [x] **Step 2: Run the focused tests and verify RED.**

  ```bash
  rtk go test ./internal/assets ./internal/schema ./internal/host -run 'Host|Manifest|Integration|ConformanceRecord'
  ```

  Expected: `internal/host` and the new schema constants do not exist.

- [x] **Step 3: Add strict schemas and immutable domain constructors.**

  Register both embedded schemas. Implement `NewManifest`, `NewAuditEvidence`,
  `NewConformanceReport`, `NewIntegration`, `DecodeIntegrationJSON`,
  `DecodeIntegrationTOML`, `ValidateIntegrationRecord`, `CloneIntegration`, and
  stable typed errors. Validate all authored digests; never overwrite a wrong
  digest during decode. Keep each file below 400 lines and every validator
  fail-closed on unknown enum values.

- [x] **Step 4: Run focused tests and coverage.**

  ```bash
  rtk gofmt -w internal/host internal/assets internal/schema
  rtk go test ./internal/assets ./internal/schema ./internal/host
  rtk go test ./internal/host -coverprofile=/tmp/oaw-host-ticket08.cover
  rtk go tool cover -func=/tmp/oaw-host-ticket08.cover
  ```

  Expected: all pass and `internal/host` statement coverage is at least 90%.

- [x] **Step 5: Commit.**

  ```bash
  rtk git add internal/assets internal/schema internal/host
  rtk git commit -m "feat: define trusted host integration records"
  ```

## Task 2: Execute deterministic Adapter conformance fixtures

**Files:**
- Create: `internal/host/conformance.go`
- Create: `internal/host/conformance_test.go`
- Create: `internal/host/conformance_fuzz_test.go`

- [ ] **Step 1: Write failing conformance-suite tests.**

  Define a deterministic fake Adapter and prove a runner-managed Integration
  passes isolated Executor creation, exact Binding delivery, pause receipt,
  Bundle digest inheritance, normalized Evidence return, duplicate invocation
  coalescing, and cancellation. Prove native-managed additionally requires a
  native invocation receipt. For each check, add a malicious or incomplete fake
  that returns the wrong Executor, Binding, invocation ID, Bundle digest,
  Evidence digest, duplicate outcome, pause state, cancellation state, or native
  marker and assert the report fails with that exact check ID. Raw adapter output
  must not enter a Report or transcript.

- [ ] **Step 2: Run the tests and verify RED.**

  ```bash
  rtk go test ./internal/host -run 'ConformanceSuite|ConformanceFailure|InstructionOnly'
  ```

  Expected: the Adapter fixture interface and `RunConformance` are undefined.

- [ ] **Step 3: Implement the pure conformance harness.**

  Add closed request/receipt records and this seam:

  ```go
  type ConformanceAdapter interface {
      CreateExecutor(ExecutorFixtureRequest) (ExecutorFixtureReceipt, error)
      Invoke(InvocationFixtureRequest) (ObservationFixtureReceipt, error)
      Pause(PauseFixtureRequest) (PauseFixtureReceipt, error)
      Cancel(CancelFixtureRequest) (CancelFixtureReceipt, error)
  }

  func RunConformance(
      integrationID string,
      manifest Manifest,
      adapter ConformanceAdapter,
  ) (ConformanceReport, error)
  ```

  Use fixed fixture identities derived from the Integration and Manifest
  digests. Invoke the same logical invocation twice to prove deduplication.
  Canonicalize only normalized receipts into the transcript digest. Return a
  complete failed Report for behavioral nonconformance; reserve Go errors for
  invalid inputs or an unusable harness.

- [ ] **Step 4: Add fail-closed fuzz coverage and run tests.**

  Fuzz receipt validation and Report decoding with bounded values; a panic,
  unbounded allocation, unknown Feature/check, or accepted digest mismatch is a
  failure.

  ```bash
  rtk gofmt -w internal/host
  rtk go test ./internal/host -count=1
  rtk go test ./internal/host -run '^$' -fuzz FuzzConformanceReceiptFailsClosed -fuzztime 2s
  ```

- [ ] **Step 5: Commit.**

  ```bash
  rtk git add internal/host
  rtk git commit -m "feat: add host adapter conformance suite"
  ```

## Task 3: Pin trusted Host Integrations in Configuration Snapshot

**Files:**
- Create: `internal/assets/host-integrations.json`
- Create: `internal/assets/schemas/v1/host-integration-set.schema.json`
- Modify: `internal/assets/embed.go`
- Modify: `internal/schema/registry.go`
- Modify: `internal/host/decode.go`
- Create: `internal/host/builtin.go`
- Modify: `internal/config/records.go`
- Modify: `internal/config/decode.go`
- Modify: `internal/config/snapshot.go`
- Modify: `internal/config/project.go`
- Modify: `internal/assets/schemas/v1/user-config.schema.json`
- Modify: `internal/assets/schemas/v1/project-config.schema.json`
- Modify: `internal/config/config_test.go`
- Create: `internal/config/snapshot_test.go`
- Create: `internal/integration/host_configuration_test.go`

- [ ] **Step 1: Write failing configuration trust tests.**

  Add built-in-record tests for every supported instruction target (`claude`,
  `codex`, `gemini`, `opencode`, `cursor`, `windsurf`, `cline`, `roo`, and
  `copilot`). Each built-in record must be instruction-only, deterministic, and
  unable to satisfy Runtime admission. Add user-root fixtures that reference a
  complete third-party Integration TOML by ID/path/replace and prove the loaded
  Snapshot pins a defensive record and changes its digest. Prove project TOML
  cannot add, replace, select, audit, or conform a Host; `host_integrations` and
  `runtime_host` remain unknown project authority fields. Reject reserved
  `oaw/*` user IDs, duplicates without explicit replacement, ID mismatch,
  unsafe paths, symlinks escaping the user root, and stale Manifest/Report
  digests.

- [ ] **Step 2: Run tests and verify RED.**

  ```bash
  rtk go test ./internal/host ./internal/config ./internal/integration -run 'HostIntegration|HostConfiguration|Project.*Host'
  ```

- [ ] **Step 3: Load built-in and user-trusted records.**

  Add `HostIntegrations []ContentReference` only to `UserConfigRecord`; extend
  the user schema and normalization with a unique stable ID. Load a closed
  embedded Integration Set containing instruction-only records for the nine
  current targets, merge user TOML records with the same reserved-namespace and
  explicit-replacement rules as Provider Descriptors, and never consult project
  configuration for Host trust. Keep Host Integration source paths and raw audit
  documents out of Snapshot.

- [ ] **Step 4: Extend immutable Snapshot records and digests.**

  Store sorted `[]host.IntegrationRecord`, add `HostIntegration(id)` and
  `HostIntegrations()` defensive accessors, include the complete records in
  `SnapshotRecord` and `snapshotRecordContent`, and extend all clone helpers.
  Equivalent user TOML and equivalent Integration ordering must yield identical
  Snapshot digests.

- [ ] **Step 5: Run focused, integration, and compatibility tests.**

  ```bash
  rtk gofmt -w internal/assets internal/schema internal/host internal/config internal/integration
  rtk go test ./internal/assets ./internal/schema ./internal/host ./internal/config ./internal/integration
  rtk go test ./...
  ```

  Expected: all pre-Ticket-08 tests remain green; no production Runtime Host is
  selected by Configuration loading.

- [ ] **Step 6: Commit.**

  ```bash
  rtk git add internal/assets internal/schema internal/host internal/config internal/integration
  rtk git commit -m "feat: pin trusted host integrations in configuration"
  ```

## Task 4: Enforce Host admission and pin it in Lifecycle Bundles

**Files:**
- Create: `internal/host/admission.go`
- Create: `internal/host/admission_test.go`
- Modify: `internal/runtime/workflow_records.go`
- Modify: `internal/runtime/workflow_start.go`
- Modify: `internal/runtime/workflow_grants.go`
- Modify: `internal/runtime/workflow_dispatch.go`
- Modify: `internal/runtime/workflow_validation.go`
- Modify: `internal/runtime/workflow_start_test.go`
- Modify: `internal/runtime/workflow_grants_test.go`
- Modify: `internal/runtime/workflow_dispatch_test.go`
- Modify: `internal/runtime/workflow_invariants_test.go`
- Modify: `internal/runtime/workflow_helpers_test.go`

- [ ] **Step 1: Write failing pure admission tests.**

  Test `AdmitWorkflow` with a trusted Integration Record, Runtime frame, and the
  graph's exact Host Bindings. A conforming runner/native record succeeds and
  returns immutable effective Features. Unknown IDs, instruction-only level,
  pending/failed audit, missing/stale Conformance, absent Runtime Protocol,
  required Feature gaps, frame-unavailable required Features, Host-ID mismatch,
  unsupported Binding kind, and unknown frame Features fail with stable codes.
  Prove the frame cannot name a higher level, claim Features, replace digests, or
  change the Integration identity after a Bundle is created.

- [ ] **Step 2: Run focused tests and verify RED.**

  ```bash
  rtk go test ./internal/host ./internal/runtime -run 'HostAdmission|Workflow.*Host|Bundle.*Host'
  ```

- [ ] **Step 3: Implement Host admission and remove self-attestation.**

  Replace `WorkflowHostDeclaration{PhysicalIsolation bool}` with
  `host.RuntimeFrame{IntegrationID, UnavailableFeatures}`. Resolve the selected
  record only through `WorkflowOptions.Configuration`; validate audit,
  Conformance, Protocol, Features, and every compiled graph Binding after Profile
  compilation but before Bundle creation. Use stable Runtime error mapping:

  ```text
  HOST_INTEGRATION_REQUIRED
  HOST_INTEGRATION_NOT_ADMITTED
  HOST_RUNTIME_REQUIREMENTS_UNMET
  HOST_BINDING_UNSUPPORTED
  HOST_INTEGRATION_CHANGED
  ```

  Keep `HOST_ISOLATION_UNAVAILABLE` only as a compatibility diagnostic nested
  under `HOST_RUNTIME_REQUIREMENTS_UNMET`, not as evidence from a boolean frame.

- [ ] **Step 4: Pin Host identities in Bundle generations.**

  Extend `newLifecycleBundle`, clone/validation helpers, canonical digesting,
  projection summaries, and revision-edge validation with the five locked Host
  fields. Initial selection pins the admitted record. Stage Grant, dispatch,
  observation, restart, and inspection revalidate against the active Bundle.
  Explicit stable-boundary switching may pin a different current trusted
  Integration only in the new generation; old Bundles remain byte-for-byte
  immutable and old Engines cannot continue.

- [ ] **Step 5: Update existing Workflow fixtures and run tests.**

  Replace every synthetic `PhysicalIsolation: true` fixture with a user-trusted,
  audited, conforming Integration Record whose Host ID matches the verified
  Provider Bindings. Preserve tests proving Main Agent Executors are rejected,
  Runtime invokes no Binding, Direct/Bounded behavior is unchanged, and all
  caller-owned slices are copied.

  ```bash
  rtk gofmt -w internal/host internal/runtime internal/integration
  rtk go test ./internal/host ./internal/runtime ./internal/integration -run 'Host|Workflow|Direct|Bounded'
  rtk go test ./...
  ```

- [ ] **Step 6: Commit.**

  ```bash
  rtk git add internal/host internal/runtime internal/integration
  rtk git commit -m "feat: require conforming hosts for workflows"
  ```

## Task 5: Add conformance recovery, security, and non-selection integration coverage

**Files:**
- Create: `internal/integration/host_conformance_test.go`
- Modify: `internal/integration/workflow_runtime_test.go`
- Modify: `internal/runtime/projection_test.go`
- Modify: `internal/runtime/workflow_invariants_test.go`

- [ ] **Step 1: Write end-to-end integration tests.**

  Build a real user configuration containing a third-party Host Integration,
  run its fake Adapter through the conformance suite, reload the Configuration
  Snapshot, resolve Providers, select a Workflow Profile, issue a Stage Grant,
  authorize dispatch, return a normalized observation, restart the Engine, and
  inspect the same pinned Host identities. Exercise both runner-managed and
  native-managed records without making either a production selection.

- [ ] **Step 2: Add failure and tamper scenarios.**

  Cover instruction-only fallback, missing Feature, failed audit, failed/stale
  Report, per-run narrowing, wrong Binding Host/kind, changed current
  Integration after Bundle creation, tampered historical Bundle pins, projection
  deletion/tampering, and a second Engine with stale Host configuration. Every
  denial must leave HEAD unchanged and preserve old Bundles.

- [ ] **Step 3: Add security and authority scans.**

  Place credentials, raw audit text, and raw Adapter output in fixture inputs.
  Assert none enters Runtime state, projections, Conformance Reports, or errors.
  Assert owner-only Runtime/projection permissions remain unchanged and project
  config cannot grant Host trust. Confirm Runtime never invokes the fake Adapter;
  only the explicit conformance harness may do so.

- [ ] **Step 4: Run focused race and recovery tests.**

  ```bash
  rtk gofmt -w internal/integration internal/runtime internal/host
  rtk go test -race ./internal/host ./internal/runtime ./internal/integration -run 'Host|Workflow'
  rtk go test ./... -count=1
  ```

- [ ] **Step 5: Commit.**

  ```bash
  rtk git add internal/host internal/runtime internal/integration
  rtk git commit -m "test: prove host conformance admission boundaries"
  ```

## Task 6: Review, verify, and close Ticket 08

**Files:**
- Modify: `.scratch/oaw-runtime-vnext/workflow.md`
- Modify: `.scratch/oaw-runtime-vnext/issues/08-host-conformance-and-capability-audit.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/review.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/verification.md`
- Modify: `docs/superpowers/plans/2026-08-02-oaw-runtime-vnext-08-host-conformance-and-capability-audit.md`

- [ ] **Step 1: Run the complete verification matrix.**

  ```bash
  rtk gofmt -w internal/host internal/config internal/runtime internal/integration internal/assets internal/schema
  rtk git diff --check
  rtk go test ./... -count=1
  rtk go test -race ./...
  rtk go vet ./...
  rtk go test ./internal/host -coverprofile=/tmp/oaw-host-ticket08.cover -count=1
  rtk go tool cover -func=/tmp/oaw-host-ticket08.cover
  rtk go test ./internal/runtime -coverprofile=/tmp/oaw-runtime-ticket08.cover -count=1
  rtk go tool cover -func=/tmp/oaw-runtime-ticket08.cover
  rtk go test ./... -coverprofile=/tmp/oaw-ticket08-all.cover -count=1
  rtk go tool cover -func=/tmp/oaw-ticket08-all.cover
  rtk bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
  rtk shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
  rtk bash tests/run.sh
  rtk go test ./internal/classification -run '^$' -fuzz FuzzDecodeProposalFailsClosed -fuzztime 2s
  rtk go test ./internal/host -run '^$' -fuzz FuzzConformanceReceiptFailsClosed -fuzztime 2s
  rtk env GOOS=linux GOARCH=amd64 go build -o /tmp/oaw-ticket08-linux ./cmd/oaw
  rtk env GOOS=windows GOARCH=amd64 go build -o /tmp/oaw-ticket08-windows.exe ./cmd/oaw
  rtk go run golang.org/x/vuln/cmd/govulncheck@latest ./...
  ```

  Expected: every test/race/vet/build/security check passes; `internal/host` and
  `internal/runtime` statement coverage are each at least 90%; repository
  coverage remains at least 80%; no reachable vulnerability is reported.

- [ ] **Step 2: Perform inline Superpowers review and remediation.**

  Review `main...HEAD` for self-attested Features, project-granted Host trust,
  forged Manifest/Audit/Report digests, instruction-only promotion, incomplete
  Feature/check matrices, Runtime Adapter invocation, Binding substitution,
  stale Integration reuse, unsafe Bundle switching, mutable record leakage,
  transcript/raw-output leakage, unbounded inputs, unstable reason codes,
  Direct/Bounded regressions, and premature first-Host selection. Fix all
  Critical/High/Important findings and rerun the full matrix.

- [ ] **Step 3: Close the tracker and evidence.**

  Set Ticket 08 to `completed`, check every acceptance item, set the tracker
  `current_stage: completed` and `active_ticket` to Ticket 08, add this plan path,
  record exact implementation commits, review dispositions, test counts,
  coverage, fuzz/build/Bash/vulnerability results, and explicitly state that no
  first Runtime Host has been selected. Preserve `.serena/` and every unrelated
  worktree.

- [ ] **Step 4: Commit documentation closure.**

  ```bash
  rtk git add .scratch/oaw-runtime-vnext docs/superpowers/plans/2026-08-02-oaw-runtime-vnext-08-host-conformance-and-capability-audit.md
  rtk git commit -m "docs: close host conformance ticket"
  ```

## Plan Self-Review

- Spec sections 11 and 13 are covered by Tasks 1-5: Workflow admission comes
  from a trusted Integration Record and proven Host features, never from a
  self-attested boolean or project authority.
- Ticket 08's fixture list is covered by Task 2 and re-exercised through the
  Configuration/Runtime path in Task 5, including deduplication and cancellation
  from the issue's full “What to build” statement.
- Runtime does not gain an Adapter field or call the conformance interface;
  conformance is an explicit audit operation and Runtime consumes only its
  immutable report.
- Built-in records remain instruction-only, so this ticket cannot select or
  promote the first Runtime Host and Ticket 09's explicit user-selection gate
  remains intact.
- Direct and Bounded paths do not resolve Host Integration records and retain
  their Ticket 05/06 authority semantics.
- The plan contains no placeholders; every task names exact files, interfaces,
  failure codes, commands, expected outcomes, and commit boundaries.
