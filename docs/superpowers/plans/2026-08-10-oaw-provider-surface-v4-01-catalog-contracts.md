# OAW Provider Surface v4 01: Catalog Contracts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Hard-replace Provider Descriptor v3 and Profile Recipe v2 with the Provider-neutral v4/v3 catalog contracts, including the ten canonical lifecycle slots, typed ownership, immutable Distribution evidence, ordered pipelines, macros, Host actions, neutral gates, incidents, overlays, and strict schema rejection.

**Architecture:** Keep wire records, deep-copy logic, and structural validation in `internal/catalog`, with one focused taxonomy file for stable slot definitions. The record/decoder/validator/catalog/test files are cut over as one same-package change so deleting v3/v2 symbols never leaves a half-migrated package. JSON Schema, embedding, and the production built-in loader's schema IDs advance together. The User Config envelope remains v3; Plan 02 migrates its decoder/snapshot consumers atomically with Host v3 because Config imports Host records that still depend on the deleted catalog types. Downstream Provider consumers may remain uncompiled until Plans 02-04 replace them, but no compatibility reader is introduced.

**Tech Stack:** Go 1.26, JSON Schema Draft 2020-12, strict JSON/TOML decoding, canonical JSON SHA-256 digests, table-driven tests.

---

**Selected lifecycle:** `SP-FULL / CURRENT / no Add-on`.

**Depends on:** `docs/superpowers/specs/2026-08-10-oaw-provider-surface-contract-v4-design.md` and audit digest `49ec1819ab22364d763d0875d9af299ee332de3d6d39a7178a715c2b13272ccf`.

**Hard-cut boundary:** Do not add v3/v2 readers, deprecated aliases, conversion helpers, permissive unknown-field handling, or fallback defaults. The old catalog records are deleted only in the atomic catalog cutover in Task 2; every same-package consumer and test is migrated in that task. The active schema registry, v4 embed pattern, and production built-in loader constants are switched together in Task 3. Config/Host/Registry consumers are an atomic Plan 02 cutover, and built-in Provider/Recipe JSON plus their asset assertions remain Plan 04 ownership. During Plans 01-04 run only the focused package/build checks named below. The first required `rtk go test ./...` gate is Plan 06 after every authority and Bridge consumer has moved to the new records.

## Execution Baseline

Before Task 1, use `superpowers:using-git-worktrees` to create the isolated implementation worktree. Run `rtk git rev-parse HEAD`, `rtk git status --short`, and `rtk git diff --check`; require no staged change and no undeclared worktree change. Create `.scratch/oaw-provider-surface-v4/evidence/execution-baseline.md` with `apply_patch`, recording the returned 40-character commit after `implementation_base: ` plus the original workspace status digest. This untracked record is the comparison base required by Plan 06 Task 8; never substitute a later implementation commit.

## File Map

| Path | Responsibility |
| --- | --- |
| `internal/catalog/taxonomy.go` | Stable ten-slot taxonomy, machine kinds, typed responsibility namespaces, and canonical ordering. |
| `internal/catalog/records.go` | Provider Descriptor v4, Distribution, Binding, Capability, Recipe v3, pipeline, gate, incident, overlay, and alias records. |
| `internal/catalog/decode.go` | Strict v4/v3 decoders and explicit old-schema rejection. |
| `internal/catalog/validate.go` | Cross-record identities, Distribution/Binding references, slot structure, owner, macro, gate, overlay, and artifact invariants. |
| `internal/catalog/catalog.go` | Immutable Catalog storage, shared Recipe normalization/digesting, and digest over only the active contract set. |
| `internal/catalog/catalog_test.go` | Cross-record and immutability tests for the new Catalog. |
| `internal/catalog/decode_test.go` | Closed-world decoder and old-schema rejection tests. |
| `internal/catalog/taxonomy_test.go` | Canonical slot order and namespace tests. |
| `internal/schema/registry.go` | Register only the active Provider v4 and Recipe v3 schemas. |
| `internal/schema/registry_test.go` | Schema registration, strict-field, and old-contract absence tests. |
| `internal/assets/embed.go` | Add the `schemas/v4/*.json` embed pattern required by the active registry. |
| `internal/builtin/load.go` | Compile-time migration to the active schema IDs only; no asset or matrix ownership. |
| `internal/assets/schemas/v4/provider-descriptor.schema.json` | Closed Provider Descriptor v4 schema. |
| `internal/assets/schemas/v3/profile-recipe.schema.json` | Closed Profile Recipe v3 schema. |
| `internal/assets/schemas/v3/provider-descriptor.schema.json` | Delete from the active embedded schema set. |
| `internal/assets/schemas/v2/profile-recipe.schema.json` | Delete from the active embedded schema set. |
| `.scratch/oaw-provider-surface-v4/evidence/execution-baseline.md` | Untracked pre-implementation commit and user-work baseline. |

## Locked Catalog Records

Use these exported names throughout all later plans:

```go
const (
    ProviderDescriptorSchemaV4 = "oaw.provider-descriptor/v4"
    ProfileRecipeSchemaV3      = "oaw.profile-recipe/v3"
    ProfileAliasSetSchemaV1    = "oaw.profile-alias-set/v1"
    TaxonomyVersionV1          = "oaw.lifecycle-taxonomy/v1"
)

type SlotID string

const (
    SlotProblemFraming        SlotID = "problem-framing"
    SlotSolutionSpecification SlotID = "solution-specification"
    SlotDeliveryPlanning      SlotID = "delivery-planning"
    SlotWorkspacePreparation  SlotID = "workspace-preparation"
    SlotImplementation        SlotID = "implementation"
    SlotImplementationTDD     SlotID = "implementation-tdd"
    SlotIncidentRecovery      SlotID = "incident-recovery"
    SlotReviewRemediation     SlotID = "review-remediation"
    SlotFreshVerification     SlotID = "fresh-verification"
    SlotCloseout              SlotID = "closeout"
)

type MachineKind string

const (
    MachineStage            MachineKind = "stage"
    MachineHostActionGate   MachineKind = "host-action+neutral-gate"
    MachineProcedure        MachineKind = "procedure"
    MachineIncidentHandler  MachineKind = "incident-handler"
    MachineAssuranceLoop    MachineKind = "assurance-loop"
    MachineHostProviderGate MachineKind = "host-or-provider-procedure+neutral-gate"
    MachineTerminalSequence MachineKind = "terminal-sequence+user-gate"
)

type OwnershipNamespace string

const (
    OwnershipStage      OwnershipNamespace = "stage"
    OwnershipProcedure  OwnershipNamespace = "procedure"
    OwnershipIncident   OwnershipNamespace = "incident"
    OwnershipAssurance  OwnershipNamespace = "assurance"
    OwnershipHostAction OwnershipNamespace = "host-action"
    OwnershipGate       OwnershipNamespace = "gate"
)

type BindingKind string

const (
    BindingSkill       BindingKind = "skill"
    BindingAgent       BindingKind = "agent"
    BindingRole        BindingKind = "role"
    BindingInstruction BindingKind = "instruction"
    BindingTool        BindingKind = "tool"
)

type InvocationDisposition string
type InternalCallMode string

const (
    InvocationHumanExplicit InvocationDisposition = "human-explicit"
    InvocationModel         InvocationDisposition = "model"
    InvocationHost          InvocationDisposition = "host"
    InvocationInternal      InvocationDisposition = "internal"

    InternalCreditOnly     InternalCallMode = "credit-only"
    InternalDispatchBefore InternalCallMode = "dispatch-before"
    InternalDispatchAfter  InternalCallMode = "dispatch-after"
)

type SlotDefinition struct {
    ID              SlotID      `json:"id"`
    DisplayName     string      `json:"display_name"`
    MachineKind     MachineKind `json:"machine_kind"`
    RequiredOutcome string      `json:"required_outcome"`
}

type ResponsibilityClaim struct {
    Namespace    OwnershipNamespace `json:"namespace" toml:"namespace"`
    Name         string             `json:"name" toml:"name"`
    SlotID       SlotID             `json:"slot_id" toml:"slot_id"`
    OutcomeOwner bool               `json:"outcome_owner" toml:"outcome_owner"`
}

type DelegationRequirements struct {
    Child          bool `json:"child" toml:"child"`
    ParallelChild  bool `json:"parallel_child" toml:"parallel_child"`
    NestedChild    bool `json:"nested_child" toml:"nested_child"`
    NestedParallel bool `json:"nested_parallel_child" toml:"nested_parallel_child"`
}

type InternalCall struct {
    BindingID string           `json:"binding_id" toml:"binding_id"`
    Required  bool             `json:"required" toml:"required"`
    Mode      InternalCallMode `json:"mode" toml:"mode"`
    StageSpan []SlotID         `json:"stage_span" toml:"stage_span"`
}
```

`CanonicalSlots()` returns exactly these ten `SlotDefinition` values in this
order. It returns a defensive copy; no caller may mutate the package-owned
taxonomy. `MachineKind` already distinguishes the three control-bearing slots;
do not add a second boolean whose meaning can disagree with it.

| ID | DisplayName | MachineKind | RequiredOutcome |
| --- | --- | --- | --- |
| `problem-framing` | `Requirements and domain alignment` | `stage` | `Purpose, constraints, domain terms, decisions, and success conditions are user-aligned.` |
| `solution-specification` | `Solution specification and test boundaries` | `stage` | `A reviewable solution specification and test boundaries are approved.` |
| `delivery-planning` | `Delivery planning, decomposition, and acceptance items` | `stage` | `Work is decomposed into independently verifiable units sufficient for the selected executor.` |
| `workspace-preparation` | `Workspace preparation` | `host-action+neutral-gate` | `The selected workspace is safe, initialized, and has a known baseline.` |
| `implementation` | `Implementation execution` | `stage` | `Approved changes are produced with bounded effects and progress evidence.` |
| `implementation-tdd` | `TDD and implementation testing` | `procedure` | `Expected behavior drives a witnessed RED/GREEN cycle and focused tests.` |
| `incident-recovery` | `Conditional debugging and repair` | `incident-handler` | `A typed unexpected failure is diagnosed and returns to a declared stage, replans, or stops.` |
| `review-remediation` | `Review and remediation` | `assurance-loop` | `Findings are reported, fixed or adjudicated, and re-reviewed.` |
| `fresh-verification` | `Fresh final verification` | `host-or-provider-procedure+neutral-gate` | `Claim-relevant commands run after remediation and produce fresh evidence.` |
| `closeout` | `Completion and delivery` | `terminal-sequence+user-gate` | `Acceptance is reconciled and the user-authorized delivery or preservation action is recorded.` |

The Descriptor v4 records are:

```go
type ProviderDescriptorRecord struct {
    SchemaVersion     string               `json:"schema_version" toml:"schema_version"`
    DescriptorVersion string               `json:"descriptor_version" toml:"descriptor_version"`
    ID                string               `json:"id" toml:"id"`
    DisplayName       string               `json:"display_name" toml:"display_name"`
    Distributions     []DistributionRecord `json:"distributions" toml:"distributions"`
    Discovery         []DiscoveryProbe     `json:"discovery" toml:"discovery"`
    Bindings          []BindingRecord      `json:"bindings" toml:"bindings"`
    Capabilities      []CapabilityRecord   `json:"capabilities" toml:"capabilities"`
}

type DistributionRecord struct {
    ID         string `json:"id" toml:"id"`
    SourceURI  string `json:"source_uri" toml:"source_uri"`
    Revision   string `json:"revision" toml:"revision"`
    TreeDigest string `json:"tree_digest" toml:"tree_digest"`
}

type DiscoveryProbe struct {
    ID             string   `json:"id" toml:"id"`
    Hosts          []string `json:"hosts" toml:"hosts"`
    Surface        string   `json:"surface" toml:"surface"`
    DistributionID string   `json:"distribution_id" toml:"distribution_id"`
    Kind           string   `json:"kind" toml:"kind"`
    Root           string   `json:"root" toml:"root"`
    CandidatePath  string   `json:"candidate_path,omitempty" toml:"candidate_path"`
    EvidencePath   string   `json:"evidence_path,omitempty" toml:"evidence_path"`
    Prefix         string   `json:"prefix,omitempty" toml:"prefix"`
}

type BindingRecord struct {
    ID                  string                 `json:"id" toml:"id"`
    DistributionID      string                 `json:"distribution_id" toml:"distribution_id"`
    ContentRoot         string                 `json:"content_root" toml:"content_root"`
    InstallRoot         string                 `json:"install_root" toml:"install_root"`
    TreeDigest          string                 `json:"tree_digest" toml:"tree_digest"`
    Host                string                 `json:"host" toml:"host"`
    Surface             string                 `json:"surface" toml:"surface"`
    Kind                BindingKind            `json:"kind" toml:"kind"`
    Reference           string                 `json:"reference" toml:"reference"`
    Invocation          InvocationDisposition  `json:"invocation" toml:"invocation"`
    Responsibilities    []ResponsibilityClaim  `json:"responsibilities" toml:"responsibilities"`
    InputArtifact       string                 `json:"input_artifact" toml:"input_artifact"`
    OutputArtifact      string                 `json:"output_artifact" toml:"output_artifact"`
    MaximumEffects      []string               `json:"maximum_effects" toml:"maximum_effects"`
    Resources           []string               `json:"resources" toml:"resources"`
    SupportedTopologies []execution.Topology   `json:"supported_topologies" toml:"supported_topologies"`
    Delegation          DelegationRequirements `json:"delegation" toml:"delegation"`
    StageSpan           []SlotID               `json:"stage_span" toml:"stage_span"`
    InternalCalls       []InternalCall         `json:"internal_calls" toml:"internal_calls"`
    Alternatives        []string               `json:"alternatives" toml:"alternatives"`
    Conflicts           []string               `json:"conflicts" toml:"conflicts"`
}

type CapabilityRecord struct {
    ID             string        `json:"id" toml:"id"`
    InputSchema    string        `json:"input_schema" toml:"input_schema"`
    OutcomeSchema  string        `json:"outcome_schema" toml:"outcome_schema"`
    RequestModes   []RequestMode `json:"request_modes" toml:"request_modes"`
    BindingRefs    []string      `json:"binding_refs" toml:"binding_refs"`
}
```

All content digests match `^sha256:[0-9a-f]{64}$`. `DistributionRecord.Revision` matches `^(?:[0-9a-f]{40}|sha256:[0-9a-f]{64})$`: either an immutable lowercase Git object ID or a content-addressed immutable source identifier. `ContentRoot` and `InstallRoot` are independently required, clean, relative, slash-separated paths and cannot contain an empty, `.`, or `..` component. `ContentRoot` resolves only inside the immutable Distribution; `InstallRoot` resolves only below the exact Host installation root. They may be equal for repository-style installations or differ for flattened installations, and no consumer may infer either path from the other.

The Recipe v3 records are:

```go
type ProfileRecipeRecord struct {
    SchemaVersion           string                             `json:"schema_version" toml:"schema_version"`
    TaxonomyVersion         string                             `json:"taxonomy_version" toml:"taxonomy_version"`
    RecipeVersion           string                             `json:"recipe_version" toml:"recipe_version"`
    ID                      string                             `json:"id" toml:"id"`
    DisplayName             string                             `json:"display_name" toml:"display_name"`
    Family                  string                             `json:"family" toml:"family"`
    Template                string                             `json:"template,omitempty" toml:"template"`
    Slots                   []SlotRecipe                       `json:"slots" toml:"slots"`
    AddOns                 []AddOnRecord                      `json:"add_ons" toml:"add_ons"`
    IncidentRoutes          []IncidentRoute                    `json:"incident_routes" toml:"incident_routes"`
    Overlays                []OverlayRecord                    `json:"overlays" toml:"overlays"`
    StableBoundaries        []string                           `json:"stable_boundaries" toml:"stable_boundaries"`
    EnvironmentRequirements []execution.EnvironmentRequirement `json:"environment_requirements" toml:"environment_requirements"`
}

type SlotRecipe struct {
    SlotID         SlotID            `json:"slot_id" toml:"slot_id"`
    Applicability  SlotApplicability `json:"applicability" toml:"applicability"`
    OutcomeOwner   OutcomeOwner      `json:"outcome_owner" toml:"outcome_owner"`
    Pipeline       []PipelineStep    `json:"pipeline" toml:"pipeline"`
    HostAction     *HostActionRef    `json:"host_action,omitempty" toml:"host_action"`
    Gates          []GateRecord      `json:"gates" toml:"gates"`
    Transitions    []RecipeTransition `json:"transitions" toml:"transitions"`
}

type BindingSelector struct {
    ProviderID string `json:"provider_id" toml:"provider_id"`
    BindingID  string `json:"binding_id" toml:"binding_id"`
}

type PipelineStep struct {
    ID                     string          `json:"id" toml:"id"`
    Selector               BindingSelector `json:"binding_selector" toml:"binding_selector"`
    StageSpan              []SlotID        `json:"stage_span" toml:"stage_span"`
    RequiredInputArtifact  string          `json:"required_input_artifact" toml:"required_input_artifact"`
    ProducedOutputArtifact string          `json:"produced_output_artifact" toml:"produced_output_artifact"`
}

type OutcomeOwner struct {
    Kind       OutcomeOwnerKind `json:"kind" toml:"kind"`
    StepID     string           `json:"step_id,omitempty" toml:"step_id"`
    HostAction string           `json:"host_action,omitempty" toml:"host_action"`
}

type SlotApplicability string
type OutcomeOwnerKind string
type GateAuthority string
type IncidentFallback string
type AddOnKind string

const (
    SlotMandatory   SlotApplicability = "mandatory"
    SlotConditional SlotApplicability = "conditional"

    OwnerProviderBinding OutcomeOwnerKind = "provider-binding"
    OwnerHostAction      OutcomeOwnerKind = "host-action"
    OwnerNone            OutcomeOwnerKind = "none"

    GateOAWCore GateAuthority = "oaw-core"
    GateHost    GateAuthority = "host"
    GateUser    GateAuthority = "user"

    IncidentStop   IncidentFallback = "stop"
    IncidentReplan IncidentFallback = "replan"

    AddOnIncidentHandler AddOnKind = "incident-handler"
    AddOnSpecialistCheck AddOnKind = "specialist-check"
)

type HostActionRef struct {
    ID             string `json:"id" toml:"id"`
    InputArtifact  string `json:"input_artifact" toml:"input_artifact"`
    OutputArtifact string `json:"output_artifact" toml:"output_artifact"`
}

type EvidenceRequirementRecord struct {
    Kind        string `json:"kind" toml:"kind"`
    Minimum     uint64 `json:"minimum" toml:"minimum"`
    Description string `json:"description" toml:"description"`
}

type GateRecord struct {
    ID                   string                      `json:"id" toml:"id"`
    Authority            GateAuthority               `json:"authority" toml:"authority"`
    Predicate            string                      `json:"predicate" toml:"predicate"`
    EvidenceRequirements []EvidenceRequirementRecord `json:"evidence_requirements" toml:"evidence_requirements"`
}

type RecipeTransition struct {
    Signal string `json:"signal" toml:"signal"`
    Target SlotID `json:"target" toml:"target"`
}

type IncidentRoute struct {
    IncidentType  string           `json:"incident_type" toml:"incident_type"`
    Handler       BindingSelector  `json:"handler" toml:"handler"`
    ReturnTo      SlotID           `json:"return_to" toml:"return_to"`
    IfUnavailable IncidentFallback `json:"if_unavailable" toml:"if_unavailable"`
}

type OverlayRecord struct {
    ID                  string            `json:"id" toml:"id"`
    Precedence          []string          `json:"precedence" toml:"precedence"`
    PausedBindings      []BindingSelector `json:"paused_bindings" toml:"paused_bindings"`
    SelectedAlternative string            `json:"selected_alternative" toml:"selected_alternative"`
    Rationale           string            `json:"rationale" toml:"rationale"`
}

type AddOnRecord struct {
    ID                   string                      `json:"id" toml:"id"`
    Kind                 AddOnKind                   `json:"kind" toml:"kind"`
    Selector             BindingSelector            `json:"binding_selector" toml:"binding_selector"`
    SlotID               SlotID                     `json:"slot_id" toml:"slot_id"`
    IncidentTypes        []string                   `json:"incident_types" toml:"incident_types"`
    EvidenceRequirements []EvidenceRequirementRecord `json:"evidence_requirements" toml:"evidence_requirements"`
}
```

`OutcomeOwnerKind` accepts only `provider-binding`, `host-action`, and `none`. `SlotApplicability` accepts only `mandatory` and `conditional`. Gate authority accepts only `oaw-core`, `host`, and `user`; `GateRecord` has no Provider or Binding selector. Provider ownership is mandatory for problem framing, solution specification, delivery planning, implementation, implementation TDD, and review/remediation. Workspace preparation accepts a Provider step or `workspace.prepare-or-confirm`; fresh verification accepts a Provider step or `verification.execute`; closeout accepts a Provider step or `closeout.execute`. `none` is accepted only for an inactive conditional incident-recovery slot; it never covers a mandatory engineering outcome.

## Task 1: Add the canonical lifecycle taxonomy

**Files:**
- Create: `internal/catalog/taxonomy.go`
- Create: `internal/catalog/taxonomy_test.go`

- [ ] **Step 1: Write the failing taxonomy tests**

```go
func TestCanonicalSlotsAreStableAndDefensive(t *testing.T) {
    got := CanonicalSlots()
    want := []SlotDefinition{
        {SlotProblemFraming, "Requirements and domain alignment", MachineStage, "Purpose, constraints, domain terms, decisions, and success conditions are user-aligned."},
        {SlotSolutionSpecification, "Solution specification and test boundaries", MachineStage, "A reviewable solution specification and test boundaries are approved."},
        {SlotDeliveryPlanning, "Delivery planning, decomposition, and acceptance items", MachineStage, "Work is decomposed into independently verifiable units sufficient for the selected executor."},
        {SlotWorkspacePreparation, "Workspace preparation", MachineHostActionGate, "The selected workspace is safe, initialized, and has a known baseline."},
        {SlotImplementation, "Implementation execution", MachineStage, "Approved changes are produced with bounded effects and progress evidence."},
        {SlotImplementationTDD, "TDD and implementation testing", MachineProcedure, "Expected behavior drives a witnessed RED/GREEN cycle and focused tests."},
        {SlotIncidentRecovery, "Conditional debugging and repair", MachineIncidentHandler, "A typed unexpected failure is diagnosed and returns to a declared stage, replans, or stops."},
        {SlotReviewRemediation, "Review and remediation", MachineAssuranceLoop, "Findings are reported, fixed or adjudicated, and re-reviewed."},
        {SlotFreshVerification, "Fresh final verification", MachineHostProviderGate, "Claim-relevant commands run after remediation and produce fresh evidence."},
        {SlotCloseout, "Completion and delivery", MachineTerminalSequence, "Acceptance is reconciled and the user-authorized delivery or preservation action is recorded."},
    }
    if len(got) != len(want) { t.Fatalf("slot count = %d", len(got)) }
    for i := range want {
        if got[i] != want[i] { t.Fatalf("slot[%d] = %#v, want %#v", i, got[i], want[i]) }
    }
    got[0].ID = "changed"
    if CanonicalSlots()[0].ID != SlotProblemFraming { t.Fatal("taxonomy exposed mutable storage") }
}
```

- [ ] **Step 2: Run RED**

Run: `rtk go test ./internal/catalog -run CanonicalSlots`

Expected: FAIL because `CanonicalSlots` and the new slot constants do not exist.

- [ ] **Step 3: Implement the locked taxonomy values**

Add the exact constants above and a package-owned `[10]SlotDefinition`. Give each slot its design-approved machine kind and required outcome. Return `append([]SlotDefinition(nil), canonicalSlots[:]...)` from `CanonicalSlots`.

- [ ] **Step 4: Format the taxonomy files**

Run: `rtk gofmt -w internal/catalog/taxonomy.go internal/catalog/taxonomy_test.go`

Expected: exit 0 and only the two named files are formatted.

- [ ] **Step 5: Run GREEN**

Run: `rtk go test ./internal/catalog -run '^TestCanonicalSlotsAreStableAndDefensive$' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit the taxonomy**

```bash
rtk git add internal/catalog/taxonomy.go internal/catalog/taxonomy_test.go
rtk git commit -m "feat: define canonical lifecycle taxonomy"
```

## Task 2: Atomically replace the complete catalog package contract

Deleting `RecipeNode`, `HostBinding`, and the v3/v2 constants before their
same-package consumers move makes `internal/catalog` uncompilable. Treat this
task as one atomic TDD unit: write all RED tests, replace records, decoders,
validators, clone/digest logic, and existing tests, obtain one package-wide
GREEN, then make one commit. Do not commit or hand off an intermediate state.

**Files:**
- Modify: `internal/catalog/records.go`
- Modify: `internal/catalog/decode.go`
- Modify: `internal/catalog/validate.go`
- Modify: `internal/catalog/catalog.go`
- Modify: `internal/catalog/decode_test.go`
- Modify: `internal/catalog/catalog_test.go`

- [ ] **Step 1: Replace decoder tests with synthetic v4/v3 RED fixtures**

In `internal/catalog/decode_test.go`, define `validProviderV4Record()` and
`validRecipeV3Record()` helpers using the locked records in this plan. The
Provider fixture has one immutable Distribution, one Codex skill Binding with
independent non-empty `ContentRoot` and `InstallRoot`, one Capability
referring to that Binding, and non-nil values for every required
collection. The Recipe fixture contains all ten canonical slots in canonical
order, uses a Provider owner for the six engineering slots, the declared Host
actions for workspace and closeout, a conditional `none` incident slot, and a
Provider-owned fresh-verification slot. Separate table rows replace workspace
and closeout with legal Provider owners. Every pipeline artifact edge matches.

Add these exact tests:

```go
func TestDecodeProviderV4AcceptsCompleteClosedRecord(t *testing.T)
func TestDecodeRecipeV3AcceptsCompleteClosedRecord(t *testing.T)
func TestDecodeAliasSetV1AcceptsCompleteClosedRecord(t *testing.T)
func TestDecodeAuthorityRejectsRetiredSchemas(t *testing.T)
func TestDecodeProviderV4RejectsUnknownField(t *testing.T)
func TestDecodeRecipeV3RejectsUnknownFieldAndTrailingValue(t *testing.T)
func TestDecodeAliasSetV1RejectsUnknownFieldAndInvalidIdentity(t *testing.T)
func TestDecodedV4RecordsOwnAllNestedStorage(t *testing.T)
func TestDecodeProviderV4RejectsInvalidDistributionBindingAndMacroShapes(t *testing.T)
func TestDecodeRecipeV3RejectsInvalidOwnerGateIncidentAndOverlayShapes(t *testing.T)
```

Keep `DecodeAliasSet` on `ProfileAliasSetSchemaV1`; v4 does not version or
replace the four-alias envelope. The accepting fixture contains one valid
alias and the rejection table covers an unknown field, an invalid alias, an
invalid Recipe ID, a duplicate alias, and a nil `aliases` collection. Mutate
the returned alias slice and prove a second decode of the original bytes is
unchanged.

`TestDecodeAuthorityRejectsRetiredSchemas` must use old records containing
only fields shared with the new structs so rejection is caused by the schema
version, not by an unknown legacy field:

```go
tests := []struct {
    name string
    raw  []byte
    run  func([]byte) error
    code string
}{
    {
        name: "provider v3",
        raw:  []byte(`{"schema_version":"oaw.provider-descriptor/v3"}`),
        run: func(raw []byte) error { _, err := DecodeProvider(raw); return err },
        code: "UNSUPPORTED_PROVIDER_SCHEMA",
    },
    {
        name: "recipe v2",
        raw:  []byte(`{"schema_version":"oaw.profile-recipe/v2"}`),
        run: func(raw []byte) error { _, err := DecodeRecipe(raw); return err },
        code: "UNSUPPORTED_RECIPE_SCHEMA",
    },
}
```

Delete `TestBuiltinBindingsRemainHostScoped` from this file. It reads the old
embedded assets and belongs to Plan 04's `internal/builtin/load_test.go`; do
not retain it against v3 assets in a catalog unit test.

- [ ] **Step 2: Replace catalog invariant tests with the complete RED matrix**

In `internal/catalog/catalog_test.go`, base every mutation on the synthetic
fixtures from Step 1 and add one table row for each exact contract:

| Test mutation | Expected code |
| --- | --- |
| Discovery or Binding names an absent Distribution | `PROVIDER_DISTRIBUTION_NOT_FOUND` |
| Distribution revision is a branch, uppercase hash, or neither 40 lowercase hex nor a prefixed digest | `PROVIDER_DISTRIBUTION_REVISION_INVALID` |
| Distribution tree digest is unprefixed, uppercase, or not 64 lowercase hex digits | `PROVIDER_DISTRIBUTION_DIGEST_INVALID` |
| Binding tree digest is unprefixed, uppercase, or not 64 hex digits | `PROVIDER_BINDING_DIGEST_INVALID` |
| Binding content or install root is empty, absolute, contains backslash, or contains empty, `.` or `..` components | `PROVIDER_BINDING_PATH_INVALID` |
| Capability names an absent Binding | `PROVIDER_BINDING_NOT_FOUND` |
| Capability repeats a Binding reference | `CAPABILITY_BINDING_AMBIGUOUS` |
| Recipe taxonomy is not `oaw.lifecycle-taxonomy/v1` | `RECIPE_TAXONOMY_UNSUPPORTED` |
| Recipe omits or duplicates one canonical slot | `RECIPE_SLOT_COVERAGE_INVALID` |
| Mandatory engineering slot has `none`, a Host action, or an unqualified step | `OUTCOME_OWNER_MISSING` |
| Designated step and a second active Binding both claim the slot outcome | `OUTCOME_OWNER_AMBIGUOUS` |
| Pipeline input differs from the preceding output | `PIPELINE_ARTIFACT_INCOMPATIBLE` |
| Binding or internal-call span is empty, non-contiguous, reversed, or outside the parent span | `STAGE_SPAN_INVALID` |
| Internal call mode is outside the three closed values | `INTERNAL_CALL_MODE_INVALID` |
| Dispatch-before/after targets an `internal` Binding | `INTERNAL_CALL_NOT_HOST_CALLABLE` |
| Macro child is both an internal call and a peer pipeline step | `MACRO_INTERNAL_CONFLICT` |
| Gate JSON contains a Provider/Binding selector | `INVALID_PROFILE_RECIPE` |
| A pipeline selector names a Host action instead of a Provider Binding | `PROVIDER_BINDING_NOT_FOUND` |
| Incident handler Provider or Binding is absent | `INCIDENT_HANDLER_UNAVAILABLE` |
| Overlay selects no declared alternative or pauses a mandatory call | `OVERLAY_INVALID` |
| Alias names a missing Recipe | `ALIAS_RECIPE_NOT_FOUND` |

Add `TestCatalogV4PreservesSemanticOrderAndOwnsNestedStorage`. Mutate every
nested returned collection, including
`EnvironmentRequirement.AcceptedDispositions`, and the `HostAction` pointer,
then assert a second read and `Catalog.Digest()` are unchanged. Reverse a
pipeline, internal-call list, stage span, Add-on declaration list, gate list,
transition list, and overlay precedence independently and assert the digest
changes. Add-on declaration order is semantic because Plan 03 merges selected
Add-ons into one slot in that order; only the user's selected Add-on identity
set is sorted later. Reorder only these set-like fields and assert the digest
stays the same: Providers, Recipes, aliases, Distributions, Discovery probes,
probe Hosts, Bindings, Capabilities, request modes, responsibility claims,
effects, resources, supported topologies, Binding references, alternatives,
conflicts, incident routes, Add-on incident types, evidence requirements,
paused bindings, stable boundaries, and environment requirements.

- [ ] **Step 3: Run the package RED before replacing production types**

Run: `rtk go test ./internal/catalog -run '^(TestDecodeProviderV4|TestDecodeRecipeV3|TestDecodeAliasSetV1|TestDecodeAuthorityRejectsRetiredSchemas|TestDecodedV4RecordsOwnAllNestedStorage|TestCatalogV4)' -count=1`

Expected: FAIL to compile with undefined v4 record fields/constants and stale
v2 graph references. This is the expected RED; do not add temporary aliases.

- [ ] **Step 4: Replace all catalog records and strict decoders**

Implement the locked records above in `internal/catalog/records.go`. Delete
`ProviderDescriptorSchemaV3`, `ProfileRecipeSchemaV2`, `NodeKind`,
`RecipeNode`, `CapabilitySelector`, `HostBinding`,
`RequiredResponsibilities`, `Entry`, and `TerminalGates`. Do not leave type
aliases, deprecated constants, conversion functions, or dual decode branches.

In `internal/catalog/decode.go`, keep duplicate-field rejection,
`json.Decoder.DisallowUnknownFields()`, and trailing-value rejection.
`DecodeProvider` accepts only `ProviderDescriptorSchemaV4`; `DecodeRecipe`
accepts only `ProfileRecipeSchemaV3`. Check the schema version before required
field validation so the two retired schemas return the exact unsupported
codes. Require every schema-required collection to be present and non-nil;
omission is not normalized into an empty collection. Preserve the strict
`DecodeAliasSet` v1 path, including duplicate-field/trailing-value rejection,
alias/Recipe identity validation, duplicate-alias rejection, and defensive
copying; Profile Alias Set v1 is not part of this hard cut.

- [ ] **Step 5: Replace structural and cross-record validation in the same change**

In `internal/catalog/decode.go`, validate single-record lexical and shape
rules. In `internal/catalog/validate.go`, build exact indexes for Provider,
Distribution, Binding, Capability, slot, step, gate, Add-on, incident, overlay,
and alias IDs, then validate references without name-based fallback.

Apply these ownership rules exactly:

```text
problem-framing         provider-binding
solution-specification provider-binding
delivery-planning      provider-binding
workspace-preparation  provider-binding or host-action:workspace.prepare-or-confirm
implementation         provider-binding
implementation-tdd     provider-binding
incident-recovery      provider-binding or none when conditional/inactive
review-remediation     provider-binding
fresh-verification     provider-binding or host-action:verification.execute
closeout               provider-binding or host-action:closeout.execute
```

The designated Provider owner must reference one pipeline step whose expanded
macro contains exactly one Binding with a matching `ResponsibilityClaim` and
`OutcomeOwner=true`. That owner may be the pipeline Binding itself or one of
its credited internal descendants; the Recipe still designates the enclosing
step, while the compiled graph records the exact owning unit. No other active
step or descendant may claim that slot outcome. A gate never counts as an
owner. Require all ten slots exactly once and in canonical order. Require
every span to be a non-empty contiguous subsequence of `CanonicalSlots()`.
Preserve pipeline, stage-span, internal-call, gate, transition, and
overlay-precedence order. A credit-only internal call may reference a normally
callable Binding because the parent performs it; dispatch-before/after requires
a non-`internal` invocation disposition.
`MaximumEffects` includes explicit `network-write` because audited Provider
skills may publish issue-tracker records or push/open a PR after user approval;
omitting that maximum effect would understate authority. Recipe gates, Host
approvals, and Grants still decide whether a particular invocation may perform
the effect.

- [ ] **Step 6: Replace deep cloning, normalization, and Catalog digesting in the same change**

In `internal/catalog/catalog.go`, deep-clone every slice and pointer named in
the locked records, including nested internal-call spans, evidence
requirements, `HostAction`, and every
`EnvironmentRequirement.AcceptedDispositions` slice. Normalize only the
set-like fields listed in Step 2; preserve Add-on, pipeline, internal-call,
stage-span, gate, transition, and overlay-precedence order. Hash Providers,
Recipes, and aliases only after validation, normalization, and defensive
cloning. Use `canonicaljson.Digest` for the normalized Catalog payload and
retain its existing bare lowercase 64-hex digest representation; do not add a
second legacy digest path. Put shared normalization helpers in the catalog
package and invoke them from `DecodeProvider`, `DecodeRecipe`,
`DecodeAliasSet`, and `New`, so referenced TOML/JSON records and programmatic
Catalog construction produce the same canonical value without sorting any
semantic sequence.

Export exactly one shared Recipe canonicalization entry point:

```go
func NormalizeAndDigestRecipe(providers []ProviderDescriptorRecord, recipe ProfileRecipeRecord) (ProfileRecipeRecord, string, error)
```

It performs the same structural, cross-reference, order-preserving
normalization used by `New`, returns a deep clone plus the bare lowercase
`canonicaljson.Digest` of that normalized Recipe, and never constructs a
partial public Catalog. `New` must call the same internal implementation.
Plan 03, Core, and the Builder call this API directly; they must not recreate a
temporary Catalog or duplicate canonicalization rules.

- [ ] **Step 7: Format every file in the atomic cutover**

Run: `rtk gofmt -w internal/catalog/records.go internal/catalog/decode.go internal/catalog/validate.go internal/catalog/catalog.go internal/catalog/decode_test.go internal/catalog/catalog_test.go`

Expected: exit 0 and only the six named files are formatted.

- [ ] **Step 8: Run package-wide GREEN**

Run: `rtk go test ./internal/catalog -count=1`

Expected: PASS for taxonomy, IDs, strict decoding, validation, immutability,
order, digest, and retired-schema rejection. No test reads built-in v3 assets.

- [ ] **Step 9: Prove the catalog hard cut**

Run: `rtk rg -n 'ProviderDescriptorSchemaV3|ProfileRecipeSchemaV2|NodeKind|RecipeNode|CapabilitySelector|HostBinding|RequiredResponsibilities|TerminalGates' internal/catalog`

Expected: no matches.

Run: `rtk rg -n 'oaw\.provider-descriptor/v3|oaw\.profile-recipe/v2' internal/catalog`

Expected: exactly two matches, both in
`TestDecodeAuthorityRejectsRetiredSchemas` negative fixtures.

- [ ] **Step 10: Commit the atomic catalog hard cut**

```bash
rtk git add internal/catalog/records.go internal/catalog/decode.go internal/catalog/validate.go internal/catalog/catalog.go internal/catalog/decode_test.go internal/catalog/catalog_test.go
rtk git commit -m "feat: replace provider and recipe catalog contracts"
```

## Task 3: Activate v4/v3 schemas and the production loader

**Files:**
- Delete: `internal/assets/schemas/v3/provider-descriptor.schema.json`
- Delete: `internal/assets/schemas/v2/profile-recipe.schema.json`
- Create: `internal/assets/schemas/v4/provider-descriptor.schema.json`
- Create: `internal/assets/schemas/v3/profile-recipe.schema.json`
- Modify: `internal/assets/embed.go`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`
- Modify: `internal/builtin/load.go`

Plan 02 owns `internal/config/decode.go`, `internal/config/snapshot.go`,
`internal/config/config_test.go`, and the User Config v3 Binding-kind enum.
Those files must move with the Host v3/catalog consumer cutover because Config
imports Host production records that still use `catalog.HostBinding` before
Plan 02. Plan 04 owns all files under `internal/assets/providers/`,
`internal/assets/recipes/`, `internal/assets/profile-aliases.json`, and the
behavior assertions in `internal/builtin/load_test.go`. Plan 01 owns only the
`schemas/v4/*.json` pattern in `internal/assets/embed.go`; Plan 04 may append
its profile-matrix asset pattern there in its own task, but must not remove or
rewrite the v4 schema pattern.

- [ ] **Step 1: Add RED schema registry tests**

In `internal/schema/registry_test.go`, replace the old provider/recipe test
fixtures with `TestRegistryUsesProviderV4AndRecipeV3Only` and
`TestRegistryRejectsRetiredProviderAndRecipeSchemas`. Assert the exact URL
constants below compile, old URL lookups return `UNKNOWN_SCHEMA`, and unknown
fields fail `SCHEMA_VALIDATION_FAILED`. Update every remaining direct
reference in these existing tests rather than deleting their coverage:

```text
TestRegistryValidatesKnownSchemaAndRejectsUnknown
TestRegistryValidatesHostScopedProviderDescriptorV3
TestRegistryRejectsV2ProviderDescriptorFromActiveV3Schema
TestRegistryRejectsTrailingJSON
TestRegistryRejectsSchemaViolations
```

Rename the two v3-named tests to v4/hard-cut names and use synthetic v4
fixtures for trailing JSON, unknown-field, unsafe-path, Distribution, and
Binding assertions. Do not retain a v3/v2 positive fixture.

Run: `rtk go test ./internal/schema -run '^(TestRegistryUsesProviderV4AndRecipeV3Only|TestRegistryRejectsRetiredProviderAndRecipeSchemas|TestRegistryValidatesKnownSchemaAndRejectsUnknown|TestRegistryValidatesHostScopedProviderDescriptorV4|TestRegistryRejectsTrailingJSON|TestRegistryRejectsSchemaViolations)$' -count=1`

Expected: RED because the registry constants and v4 schema resources do not
yet exist. The failure must name a new identifier or schema-read path, not a
missing legacy alias.

- [ ] **Step 2: Add the closed Draft 2020-12 schemas and registry IDs**

Delete these active authority assets and do not recreate them under another
name:

```text
internal/assets/schemas/v3/provider-descriptor.schema.json
internal/assets/schemas/v2/profile-recipe.schema.json
```

Create these exact resources:

```text
internal/assets/schemas/v4/provider-descriptor.schema.json
  $id = https://open-agent-workflow.dev/schemas/v4/provider-descriptor.schema.json
internal/assets/schemas/v3/profile-recipe.schema.json
  $id = https://open-agent-workflow.dev/schemas/v3/profile-recipe.schema.json
```

Every object uses `additionalProperties: false`. Required arrays use explicit
`minItems`; enum values mirror the locked Go types. Content and tree digests
use `^sha256:[0-9a-f]{64}$`; revisions use
`^(?:[0-9a-f]{40}|sha256:[0-9a-f]{64})$`; paths reject absolute, backslash,
empty, `.`, and `..` components. Both `content_root` and `install_root` are
required on every Binding and validated independently. Pipeline, stage-span, internal-call, gate,
transition, precedence, and paused-binding arrays remain ordered.

In `internal/schema/registry.go`, replace the retired constants with exactly:

```go
ProviderDescriptorV4 = "https://open-agent-workflow.dev/schemas/v4/provider-descriptor.schema.json"
ProfileRecipeV3      = "https://open-agent-workflow.dev/schemas/v3/profile-recipe.schema.json"
```

Register the two new paths and remove the two old paths. Do not leave exported
aliases for the retired IDs. Keep `ProfileAliasSetV1` and all unrelated active
schema IDs unchanged.

In `internal/assets/embed.go`, add the exact `schemas/v4/*.json` pattern to the
existing `//go:embed` directive. Without this same-commit change,
`schema.New(assets.FS())` returns `SCHEMA_READ_FAILED` for the v4 Provider
schema. Do not add Provider/Recipe/profile-matrix asset paths here; Plan 04
owns those built-in assets. Plan 02 updates the schema metadata table in
`internal/assets/embed_test.go` when its Host-schema cutover makes that package
testable again.

- [ ] **Step 3: Switch the production built-in loader to the active IDs**

In `internal/builtin/load.go`, change only the two registry calls to
`schema.ProviderDescriptorV4` and `schema.ProfileRecipeV3`. Do not change asset
paths, glob behavior, catalog construction, or tests owned by Plan 04.

- [ ] **Step 4: Format the schema and loader cutover**

Run: `rtk gofmt -w internal/assets/embed.go internal/schema/registry.go internal/schema/registry_test.go internal/builtin/load.go`

Expected: exit 0 and only the four owned Go files are formatted.

- [ ] **Step 5: Run focused GREEN checks before built-in assets migrate**

Run: `rtk go test ./internal/catalog -count=1`

Expected: PASS.

Run: `rtk go test ./internal/schema -count=1`

Expected: PASS; registry compilation sees only the new Provider/Recipe
resources and all unrelated schema families remain available.

Run: `rtk go build ./internal/builtin`

Expected: PASS for the direct loader consumer. Calling `builtin.Load` before
Plan 04 is expected to report `BUILTIN_PROVIDER_INVALID` or
`BUILTIN_RECIPE_INVALID` against the old assets; this is a hard-cut migration,
not a reason to restore a compatibility reader.

- [ ] **Step 6: Prove schema and loader hard cut**

Run: `rtk rg -n 'ProviderDescriptorSchemaV3|ProfileRecipeSchemaV2|ProviderDescriptorV3|ProfileRecipeV2' internal/catalog internal/schema internal/builtin/load.go`

Expected: old Go identifiers have no matches. The two old wire strings occur
only in the explicitly named negative tests under `internal/catalog` and
`internal/schema`. Config/Host references are intentionally migrated by Plan
02 in their atomic consumer cutover; they are not compatibility paths.

- [ ] **Step 7: Commit schema and loader activation**

```bash
rtk git add internal/assets/schemas/v3/provider-descriptor.schema.json internal/assets/schemas/v2/profile-recipe.schema.json internal/assets/schemas/v4/provider-descriptor.schema.json internal/assets/schemas/v3/profile-recipe.schema.json internal/assets/embed.go internal/schema/registry.go internal/schema/registry_test.go internal/builtin/load.go
rtk git commit -m "feat: activate provider surface v4 schemas"
```

## Phase Verification

- [ ] Run `rtk go test ./internal/catalog ./internal/schema -count=1`.
- [ ] Run `rtk go build ./internal/builtin`.
- [ ] Run `rtk go vet ./internal/catalog ./internal/schema`.
- [ ] Run `rtk rg -n 'ProviderDescriptorSchemaV3|ProfileRecipeSchemaV2|ProviderDescriptorV3|ProfileRecipeV2' internal/catalog internal/schema internal/builtin/load.go` and require no matches.
- [ ] Run `rtk rg -n 'oaw\.provider-descriptor/v3|oaw\.profile-recipe/v2' internal/catalog internal/schema` and require matches only in named negative tests.
- [ ] Run `rtk git diff --check` and require exit 0.
- [ ] Run `rtk git status --short`; confirm only declared Plan 01 paths plus preserved pre-existing user files are present.

Expected: catalog/schema tests, the production built-in loader build, and vet
pass. Old authority strings exist only as negative fixtures; no old identifier,
compatibility reader, alias, or conversion path exists. Config/Host consumer
tests are a Plan 02 gate, built-in behavior is a Plan 04 gate, and repository-
wide `rtk go test ./... -count=1` remains the Plan 06 gate.
