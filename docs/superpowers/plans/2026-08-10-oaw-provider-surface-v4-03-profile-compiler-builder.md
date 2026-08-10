# OAW Provider Surface v4 03: Profile Compiler and Builder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Compile Profile Recipe v3 into an immutable Execution Graph v4 with ordered N:M Binding pipelines, exact alternative/Add-on/overlay selection, macro expansion, typed incident pipelines, Host actions, neutral gates, deterministic traversal, and a fail-closed USER-DEFINED Profile Builder.

**Architecture:** Replace the one-node/one-Capability compiler in one atomic `internal/profile` cutover. The compiler resolves every declared candidate against trusted Descriptor and Registry records, expands every declared macro branch, validates declared contracts, applies the normalized user selection in Recipe order, binds one validated Host evidence snapshot and one outer topology, then materializes a digest-pinned graph and canonical cursor traversal. `internal/profile` owns workflow semantics, `internal/execution` owns the package-neutral cursor value, OAW Core remains a facade, and the Coordinator consumes traversal without re-deriving it.

**Tech Stack:** Go 1.26, canonical JSON SHA-256, immutable value wrappers with defensive copies, closed JSON Schema Draft 2020-12 records, table-driven tests, race tests, and fuzzed strict record validation.

---

**Selected lifecycle:** `SP-FULL / CURRENT / no Add-on`.

**Depends on:** Plan 01 Provider Descriptor v4/Profile Recipe v3/catalog taxonomy records and Plan 02 Registry v4/Host v3 evidence records.

**Produces:** `oaw.execution-graph/v4`, immutable compile results, canonical graph traversal, and Builder projections consumed by Plan 05 Core compilation.

**Ownership boundary:** Plan 03 does not modify `internal/integration/profile_compiler_test.go` or built-in Descriptor/Recipe assets. Plan 04 owns all four built-in Profile fixtures and their cross-package integration coverage. Plan 05 owns Core, Admission, Host Receipt, and Coordinator consumers.

## File Map

| Path | Responsibility |
| --- | --- |
| `internal/execution/cursor.go` | Package-neutral cursor kinds, value validation, and canonical identity validation. |
| `internal/execution/cursor_test.go` | Closed cursor kinds, non-zero ordinal, defensive identity, and invalid-text tests. |
| `internal/profile/records.go` | Selection, resolved units, graph v4, diagnostics, compile-result wrappers, clone helpers, and public compiler interfaces. |
| `internal/profile/host_evidence.go` | Construct one trusted compiler evidence snapshot from validated Host v3 records. |
| `internal/profile/host_evidence_test.go` | Host/session/inventory/environment pinning, live-source, drift, and defensive-copy tests. |
| `internal/profile/resolve.go` | Resolve aliases, all declared default/alternative/Add-on/incident candidates, and exact verified Bindings. |
| `internal/profile/macro.go` | Expand every declared macro branch depth-first with exact `credit-only`, `dispatch-before`, and `dispatch-after` semantics. |
| `internal/profile/pipeline.go` | Apply normalized choices and overlays, then validate artifacts, ownership, procedures, actions, gates, effects, resources, topology, and incidents. |
| `internal/profile/traversal.go` | Assign cursor anchors/ordinals and expose validated first/unit/next traversal APIs for Plan 05. |
| `internal/profile/compile.go` | Orchestrate the locked compiler order and separate stable diagnostics from internal errors. |
| `internal/profile/validate.go` | Strict Graph v4 validation, reachability, terminal coverage, route closure, digest pins, and traversal consistency. |
| `internal/profile/profile_test.go` | Public API, selection, graph, topology, ownership, incident, and immutable-result tests plus shared fixtures. |
| `internal/profile/pipeline_test.go` | N:M pipeline, artifact edge, Add-on, alternative, overlay, and owner tests. |
| `internal/profile/macro_test.go` | Macro mode, conflict, ordering, recursion, and no-double-dispatch tests. |
| `internal/profile/traversal_test.go` | Anchor, ordinal, skip, transition, incident, terminal, and cursor lookup tests. |
| `internal/profile/fuzz_test.go` | Strict graph-record validation and panic resistance. |
| `internal/profile/builder.go` | USER-DEFINED clone/new/edit, candidate projection, exact preview, and confirmation. |
| `internal/profile/builder_test.go` | Builder immutability, topology, preview, drift, confirmation, and digest tests. |
| `internal/assets/schemas/v4/execution-graph.schema.json` | Closed Execution Graph v4 wire schema. |
| `internal/schema/registry.go` | Activate Graph v4 and remove Graph v3 authority. |
| `internal/schema/registry_test.go` | Graph v4 metadata, embedded-resource, and old-graph rejection tests. |

## Atomic Cutover Rule

Tasks 1 through 4 from the earlier decomposition are one Task 1 here. `records.go`, `compile.go`, `validate.go`, and every old-type consumer in `internal/profile` change together. Do not delete `GraphNode`, `ProfileBinding`, or the old graph fields in an intermediate commit. Do not claim GREEN until the complete `internal/profile` package, shared cursor package, and schema registry compile together. The detailed RED inventory below must be written before any production edit; the only commit occurs after the whole cutover passes.

## Locked Cursor Contract

```go
package execution

type CursorKind string

const (
    CursorBinding    CursorKind = "binding"
    CursorHostAction CursorKind = "host-action"
    CursorGate       CursorKind = "gate"
    CursorTerminal   CursorKind = "terminal"
)

type GraphCursor struct {
    SlotID  string     `json:"slot_id"`
    Kind    CursorKind `json:"kind"`
    UnitID  string     `json:"unit_id"`
    Ordinal uint64     `json:"ordinal"`
}

func NewGraphCursor(slotID string, kind CursorKind, unitID string, ordinal uint64) (GraphCursor, error)
func ValidateGraphCursor(cursor GraphCursor) error
```

`ValidateGraphCursor` accepts only the four constants, requires non-empty trimmed Slot/Unit IDs without control characters, and requires `Ordinal >= 1`. It validates the package-neutral value only. `profile.ValidateGraphCursor` below additionally proves that the cursor identifies exactly one unit in one validated graph.

## Locked Compiler Records

The following records live in package `profile`. Catalog, Host, Registry, and execution types referenced here are owned by Plans 01 and 02.

```go
const (
    ExecutionGraphSchemaV4  = "oaw.execution-graph/v4"
    HostEvidenceSchemaV1    = "oaw.profile-host-evidence/v1"
)

type AlternativeChoice struct {
    SlotID        catalog.SlotID          `json:"slot_id"`
    StepID        string                  `json:"step_id"`
    AlternativeID string                  `json:"alternative_id"`
    Selector      catalog.BindingSelector `json:"selector"`
}

type Selection struct {
    Profile      string              `json:"profile"`
    RecipeID     string              `json:"recipe_id"`
    RecipeDigest string              `json:"recipe_digest"`
    Topology     execution.Topology  `json:"topology"`
    AddOns       []string            `json:"add_ons"`
    Alternatives []AlternativeChoice `json:"alternatives"`
    Overlays     []string            `json:"overlays"`
    Digest       string              `json:"digest"`
}

type CompileRequest struct {
    Profile      string
    Topology     execution.Topology
    AddOns       []string
    Alternatives []AlternativeChoice
    Overlays     []string
    Host         HostEvidence
}

type DispatchDisposition string

const (
    DispatchByCoordinator DispatchDisposition = "dispatch"
    CreditInternalOnly    DispatchDisposition = "credited-internal"
    OmittedBySelection    DispatchDisposition = "omitted"
)

type ResolvedBinding struct {
    Cursor                     execution.GraphCursor         `json:"cursor"`
    UnitID                     string                        `json:"unit_id"`
    StepID                     string                        `json:"step_id"`
    AnchorSlotID               catalog.SlotID                `json:"anchor_slot_id"`
    SlotIDs                    []catalog.SlotID              `json:"slot_ids"`
    ProviderID                 string                        `json:"provider_id"`
    ProviderInstanceDigest     string                        `json:"provider_instance_digest"`
    BindingID                  string                        `json:"binding_id"`
    DistributionID             string                        `json:"distribution_id"`
    DistributionRevision       string                        `json:"distribution_revision"`
    DistributionTreeDigest     string                        `json:"distribution_tree_digest"`
    Surface                    string                        `json:"surface"`
    Kind                       catalog.BindingKind           `json:"kind"`
    Reference                  string                        `json:"reference"`
    Invocation                 catalog.InvocationDisposition `json:"invocation"`
    BindingTreeDigest          string                        `json:"binding_tree_digest"`
    InputArtifact              string                        `json:"input_artifact"`
    OutputArtifact             string                        `json:"output_artifact"`
    Responsibilities           []catalog.ResponsibilityClaim `json:"responsibilities"`
    MaximumEffects             []string                      `json:"maximum_effects"`
    Resources                  []string                      `json:"resources"`
    SupportedTopologies        []execution.Topology          `json:"supported_topologies"`
    Delegation                 catalog.DelegationRequirements `json:"delegation"`
    RequiredFeatures           []host.FeatureID              `json:"required_features"`
    FeatureEvidenceDigests     []string                      `json:"feature_evidence_digests"`
    Disposition                DispatchDisposition           `json:"disposition"`
    MacroMode                  catalog.InternalCallMode      `json:"macro_mode,omitempty"`
    ParentUnitID               string                        `json:"parent_unit_id,omitempty"`
    RequiresExplicitInvocation bool                          `json:"requires_explicit_invocation"`
    BindingEvidenceDigest      string                        `json:"binding_evidence_digest"`
}

type CompiledOwner struct {
    Kind         catalog.OutcomeOwnerKind `json:"kind"`
    UnitID       string                   `json:"unit_id"`
    ProviderID   string                   `json:"provider_id,omitempty"`
    BindingID    string                   `json:"binding_id,omitempty"`
    HostActionID string                   `json:"host_action_id,omitempty"`
}

type CompiledHostAction struct {
    Cursor            execution.GraphCursor `json:"cursor"`
    ID                string                `json:"id"`
    InputArtifact     string                `json:"input_artifact"`
    OutputArtifact    string                `json:"output_artifact"`
    InputSchema       string                `json:"input_schema"`
    OutcomeSchema     string                `json:"outcome_schema"`
    MaximumEffects    []string              `json:"maximum_effects"`
    Resources         []string              `json:"resources"`
    ObservationDigest string                `json:"observation_digest"`
}

type CompiledGate struct {
    Cursor               execution.GraphCursor             `json:"cursor"`
    ID                   string                            `json:"id"`
    Authority            catalog.GateAuthority             `json:"authority"`
    Predicate            string                            `json:"predicate"`
    EvidenceRequirements []catalog.EvidenceRequirementRecord `json:"evidence_requirements"`
}

type GraphTransition struct {
    Signal string         `json:"signal"`
    Target catalog.SlotID `json:"target"`
}

type GraphProviderInstance struct {
    ProviderID     string `json:"provider_id"`
    HostID         string `json:"host_id"`
    InstanceDigest string `json:"instance_digest"`
}

type CompiledIncidentRoute struct {
    IncidentType    string                  `json:"incident_type"`
    HandlerSlotID   catalog.SlotID          `json:"handler_slot_id"`
    HandlerPipeline []execution.GraphCursor `json:"handler_pipeline"`
    ReturnTo         catalog.SlotID         `json:"return_to"`
    IfUnavailable    catalog.IncidentFallback `json:"if_unavailable"`
}

type CompiledSlot struct {
    SlotID          catalog.SlotID            `json:"slot_id"`
    Applicability   catalog.SlotApplicability `json:"applicability"`
    Active          bool                      `json:"active"`
    EntryArtifact   string                    `json:"entry_artifact"`
    OutcomeArtifact string                    `json:"outcome_artifact"`
    OutcomeOwner    CompiledOwner             `json:"outcome_owner"`
    Pipeline        []ResolvedBinding         `json:"pipeline"`
    HostAction      *CompiledHostAction       `json:"host_action,omitempty"`
    Gates           []CompiledGate            `json:"gates"`
    Transitions     []GraphTransition         `json:"transitions"`
    Terminal        bool                      `json:"terminal"`
    Traversal       []execution.GraphCursor   `json:"traversal"`
}

type CompileDecision struct {
    SlotID       catalog.SlotID       `json:"slot_id,omitempty"`
    StepID       string               `json:"step_id,omitempty"`
    UnitID       string               `json:"unit_id,omitempty"`
    AddOnID      string               `json:"add_on_id,omitempty"`
    AlternativeID string              `json:"alternative_id,omitempty"`
    OverlayID    string               `json:"overlay_id,omitempty"`
    IncidentType string               `json:"incident_type,omitempty"`
    Disposition  DispatchDisposition `json:"disposition"`
    ReasonCode   string               `json:"reason_code"`
    Detail       string               `json:"detail"`
}

type ExecutionGraphRecord struct {
    SchemaVersion           string                             `json:"schema_version"`
    HostID                  string                             `json:"host_id"`
    HostEvidenceDigest      string                             `json:"host_evidence_digest"`
    RegistryDigest          string                             `json:"registry_digest"`
    TaxonomyVersion         string                             `json:"taxonomy_version"`
    RecipeID                string                             `json:"recipe_id"`
    RecipeVersion           string                             `json:"recipe_version"`
    RecipeDigest            string                             `json:"recipe_digest"`
    Selection               Selection                          `json:"selection"`
    ProviderInstances       []GraphProviderInstance            `json:"provider_instances"`
    EntrySlotID             catalog.SlotID                     `json:"entry_slot_id"`
    Slots                   []CompiledSlot                     `json:"slots"`
    IncidentRoutes          []CompiledIncidentRoute            `json:"incident_routes"`
    StableBoundaries        []string                           `json:"stable_boundaries"`
    Topology                execution.Topology                 `json:"topology"`
    EnvironmentRequirements []execution.EnvironmentRequirement `json:"environment_requirements"`
    Decisions               []CompileDecision                  `json:"decisions"`
    Digest                  string                             `json:"digest"`
}

type ExecutionGraph struct {
    record ExecutionGraphRecord
}

type CompileDiagnostic struct {
    Code          string             `json:"code"`
    SlotID        catalog.SlotID     `json:"slot_id,omitempty"`
    StepID        string             `json:"step_id,omitempty"`
    ProviderID    string             `json:"provider_id,omitempty"`
    BindingID     string             `json:"binding_id,omitempty"`
    AddOnID       string             `json:"add_on_id,omitempty"`
    AlternativeID string             `json:"alternative_id,omitempty"`
    OverlayID     string             `json:"overlay_id,omitempty"`
    IncidentType  string             `json:"incident_type,omitempty"`
    Topology      execution.Topology `json:"topology,omitempty"`
    Detail        string             `json:"detail"`
}

type CompileResult struct {
    graph       *ExecutionGraph
    diagnostics []CompileDiagnostic
    digest      string
}

type HostEvidenceRecord struct {
    SchemaVersion           string                             `json:"schema_version"`
    HostID                  string                             `json:"host_id"`
    Topology                execution.Topology                 `json:"topology"`
    FeatureObservations     []host.FeatureObservation          `json:"feature_observations"`
    ActionObservations      []host.HostActionObservation       `json:"action_observations"`
    EnvironmentObservations []execution.EnvironmentObservation `json:"environment_observations"`
    SessionDigest           string                             `json:"session_digest"`
    ManifestDigest          string                             `json:"manifest_digest"`
    InventoryDigest         string                             `json:"inventory_digest"`
    FeatureDigest           string                             `json:"feature_digest"`
    ActionDigest            string                             `json:"action_digest"`
    EnvironmentDigest       string                             `json:"environment_digest"`
    Digest                  string                             `json:"digest"`
}

type HostEvidence struct {
    record HostEvidenceRecord
}

type TraversalUnit struct {
    Cursor          execution.GraphCursor `json:"cursor"`
    ProviderBinding *ResolvedBinding       `json:"provider_binding,omitempty"`
    HostAction      *CompiledHostAction    `json:"host_action,omitempty"`
    Gate            *CompiledGate          `json:"gate,omitempty"`
    Terminal        bool                   `json:"terminal"`
}

type TraversalDisposition string

const (
    TraversalNext     TraversalDisposition = "next"
    TraversalTerminal TraversalDisposition = "terminal"
    TraversalStop     TraversalDisposition = "stop"
    TraversalReplan   TraversalDisposition = "replan"
)

type TraversalResult struct {
    Disposition TraversalDisposition    `json:"disposition"`
    Cursor      *execution.GraphCursor `json:"cursor,omitempty"`
}

type EffectiveRegistry interface {
    HostID() string
    Providers() []registry.ProviderInstance
    Provider(id string) (registry.ProviderInstance, bool)
    Binding(providerID, bindingID string) (registry.VerifiedBinding, bool)
    Bindings(providerID string) []registry.VerifiedBinding
    Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool)
    Digest() string
}

type CatalogSource interface {
    Providers() []catalog.ProviderDescriptorRecord
    Recipes() []catalog.ProfileRecipeRecord
    Aliases() []catalog.ProfileAliasRecord
}
```

Plan 03 consumes Plan 02's final seven-method `EffectiveRegistry` interface without widening or shadowing it; Plan 02's concrete immutable `registry.Registry` implements every method. `Responsibilities` remains the typed `catalog.ResponsibilityClaim` collection from Descriptor v4 and is never flattened into unscoped strings.

`CompileRequest`, `Selection`, and `HostEvidence` contain no explicit-invocation receipt or boolean. A `human-explicit` Descriptor compiles to `RequiresExplicitInvocation=true`; Plan 05 requires a fresh Host-owned attestation for the exact cursor/run during PREPARE.

## Locked Internal Compiler Types

```go
type resolvedStep struct {
    Step    catalog.PipelineStep
    Binding *ResolvedBinding
}

type resolvedPipeline struct {
    ID          string
    Steps       []resolvedStep
    Diagnostics []CompileDiagnostic
}

type resolvedAlternative struct {
    Choice   AlternativeChoice
    Pipeline resolvedPipeline
}

type resolvedSlot struct {
    Taxonomy       catalog.SlotDefinition
    Recipe         catalog.SlotRecipe
    Active         bool
    EntryArtifact  string
    OutcomeArtifact string
    Owner          CompiledOwner
    DefaultPipeline resolvedPipeline
    Alternatives   []resolvedAlternative
    HostAction     *CompiledHostAction
    Gates          []CompiledGate
    Transitions    []GraphTransition
    Terminal       bool
}

type resolvedAddOn struct {
    Record   catalog.AddOnRecord
    Pipeline resolvedPipeline
}

type resolvedIncidentRoute struct {
    IncidentType    string
    HandlerSlotID   catalog.SlotID
    HandlerPipeline resolvedPipeline
    ReturnTo         catalog.SlotID
    IfUnavailable    catalog.IncidentFallback
}

type resolvedRecipe struct {
    Slots          []resolvedSlot
    AddOns         []resolvedAddOn
    IncidentRoutes []resolvedIncidentRoute
}
```

Unselected alternatives and Add-ons remain candidate records with their own diagnostics. An unavailable step has `Binding=nil`; never encode absence as a zero `ResolvedBinding`. Its unavailability does not make an exact base selection ineligible. The Builder projects candidate diagnostics; exact compilation promotes diagnostics only from the selected effective branch.

## Locked Public API and Value Semantics

```go
func NewHostEvidence(manifest host.Manifest, session host.SessionSnapshot, inventory host.BindingInventory, environment host.EnvironmentReport) (HostEvidence, error)
func (evidence HostEvidence) Record() HostEvidenceRecord
func (evidence HostEvidence) Digest() string
func (record HostEvidenceRecord) ContentDigest() string
func ValidateHostEvidenceRecord(record HostEvidenceRecord) error

func CompileProfile(source CatalogSource, verified EffectiveRegistry, request CompileRequest) (CompileResult, error)
func CompileRecipe(source CatalogSource, verified EffectiveRegistry, recipe catalog.ProfileRecipeRecord, request CompileRequest) (CompileResult, error)

func (result CompileResult) Graph() (ExecutionGraphRecord, bool)
func (result CompileResult) Diagnostics() []CompileDiagnostic
func (result CompileResult) Digest() string

func (graph ExecutionGraph) Record() ExecutionGraphRecord
func (record ExecutionGraphRecord) ContentDigest() string
func ValidateExecutionGraphRecord(record ExecutionGraphRecord) error

func ValidateGraphCursor(record ExecutionGraphRecord, cursor execution.GraphCursor) error
func FirstActionableCursor(record ExecutionGraphRecord) (execution.GraphCursor, error)
func UnitAtCursor(record ExecutionGraphRecord, cursor execution.GraphCursor) (TraversalUnit, error)
func NextActionableCursor(record ExecutionGraphRecord, cursor execution.GraphCursor, signal, incidentType string) (TraversalResult, error)
```

Unknown requested Profile, expected unavailability, unsupported topology, missing selected Binding, and invalid user selection return a digest-pinned `CompileResult` with sorted diagnostics, no graph, and a nil Go error. Malformed trusted Catalog/Registry/Host records, invalid stored digests, and canonicalization or internal invariant failures return a non-nil Go error and the zero `CompileResult`. Diagnostics sort by code, slot, step, Provider, Binding, Add-on, alternative, overlay, incident type, topology, and detail. `Detail` is stable, bounded, and secret-free: never interpolate an absolute path, evidence handle, credential, transcript, or raw wrapped error into a returned diagnostic or graph decision.

Every accessor above returns a deep copy. Callers cannot mutate graph slots, nested Binding slices, Host observations, diagnostics, Builder projections, or confirmed records through returned storage. `CompileResult.Digest` covers exactly one cloned graph or the complete sorted diagnostic list; both/neither is invalid.

`NewHostEvidence` is the only supported constructor. It validates the Host v3 Manifest, Session, and Binding Inventory plus the still-active Environment Report v2 before copying anything. It requires `manifest.HostID == session.HostID == inventory.HostID`, requires `session.ManifestDigest`, `session.ProviderInventoryDigest`, and `session.EnvironmentReportDigest` to equal the three supplied record digests, and calls `host.ValidateEnvironmentReport(session, environment)` to prove the CURRENT session or exact SUBAGENT parent/session relation. `HostEvidenceRecord.Topology` is exactly `environment.Topology`, not the Session's wider supported-topology set. The constructor rechecks that only live sources claim `available`, clones all observations, hashes the `HostEvidenceSchemaV1` domain, and then calls `ValidateHostEvidenceRecord`. The validator rejects a zero/manual record, any unknown schema or topology, unsorted/duplicate observations, non-live `available` evidence, digest mismatch, or an observation inconsistent with its pinned digest. Both compiler entry points call it again and reject a Registry with a different Host ID or a request topology different from the evidence topology.

Selection normalization rejects an unknown/duplicate Add-on, more than one alternative for the same slot/step, a selector that differs from the declared alternative, and an unknown/duplicate overlay. A valid alternative must be named in the default Binding's `BindingRecord.Alternatives`; `AlternativeID` equals the alternative Binding ID, and the selector names that exact Binding under the same Provider. An overlay-selected alternative must resolve unambiguously to one declared step; ambiguity is `OVERLAY_INVALID`. Add-ons are stored as an identity-sorted set, alternatives in taxonomy slot plus Recipe step order, and overlays in the Recipe's declared precedence. Normalize every required empty collection to a non-nil empty slice before hashing or returning records; nil and empty caller slices cannot produce different authority. `Selection.Digest` hashes Profile, Recipe ID/digest, one topology, and those exact normalized collections with its own Digest field cleared.

Recipe records do not carry their own digest in Catalog v3. Every compiler path calls Plan 01's `catalog.NormalizeAndDigestRecipe(source.Providers(), recipe)` and uses its normalized deep copy and returned bare lowercase digest. Do not construct a temporary Catalog or duplicate Catalog normalization inside `profile`. Alias compilation, direct `CompileRecipe`, Builder preview, confirmation, and Graph materialization all use that one API; no caller-supplied Recipe digest is trusted.

## Locked Compile Order

```go
func resolveRecipe(source CatalogSource, profile string) (catalog.ProfileRecipeRecord, []CompileDiagnostic, error)
func normalizeSelection(recipe catalog.ProfileRecipeRecord, recipeDigest, profile string, request CompileRequest) (Selection, []CompileDiagnostic, error)
func resolveDeclaredPipelines(source CatalogSource, verified EffectiveRegistry, recipe catalog.ProfileRecipeRecord) (resolvedRecipe, []CompileDiagnostic, error)
func expandDeclaredMacros(source CatalogSource, verified EffectiveRegistry, recipe resolvedRecipe) (resolvedRecipe, []CompileDiagnostic, error)
func validateDeclaredContracts(recipe catalog.ProfileRecipeRecord, resolved resolvedRecipe) ([]CompileDiagnostic, error)
func applySelections(recipe catalog.ProfileRecipeRecord, resolved resolvedRecipe, selection Selection) (resolvedRecipe, []CompileDecision, []CompileDiagnostic, error)
func applyOverlays(recipe catalog.ProfileRecipeRecord, resolved resolvedRecipe, selection Selection) (resolvedRecipe, []CompileDecision, []CompileDiagnostic, error)
func validateEffectiveGraph(recipe catalog.ProfileRecipeRecord, resolved resolvedRecipe) ([]CompileDiagnostic, error)
func bindHostEvidence(resolved resolvedRecipe, evidence HostEvidence, topology execution.Topology) (resolvedRecipe, []CompileDecision, []CompileDiagnostic, error)
func validateReachabilityAndTerminalCoverage(recipe catalog.ProfileRecipeRecord, resolved resolvedRecipe) ([]CompileDiagnostic, error)
func materializeGraph(recipe catalog.ProfileRecipeRecord, resolved resolvedRecipe, selection Selection, evidence HostEvidence, verified EffectiveRegistry, decisions []CompileDecision) (ExecutionGraph, error)
```

Call these functions in the listed order, with `catalog.NormalizeAndDigestRecipe` immediately after Recipe resolution and before `normalizeSelection`. `CompileProfile` resolves exactly one alias/Recipe first; an unknown user profile returns stable selection diagnostics, while malformed or ambiguous trusted alias authority returns a Go error. `CompileRecipe` starts with its supplied record. Malformed Catalog authority returns a Go error before pipeline resolution. Resolution retains every declared default, alternative, optional Add-on, and incident-handler candidate plus candidate-scoped availability diagnostics. Every Recipe Add-on becomes one `resolvedAddOn`; do not collapse Add-ons into a single field on a slot. Macro expansion then expands every retained branch before any user choice can suppress a mandatory internal call. Declared-contract validation checks every expanded branch: malformed artifacts, ownership, macro envelopes, or references invalidate the trusted Recipe, while Host/Registry unavailability remains scoped to its candidate. `applySelections` consumes the normalized Add-on identity set, merges selected Add-on pipelines into their target slots in preserved Recipe declaration order, and applies unique slot/step alternative choices. `applyOverlays` runs only after expansion and applies the selected overlay identity set in the Recipe's declared precedence, never caller or lexical order. Effective validation, exact topology/Host binding, reachability, and materialization operate only on the selected branch.

Append decisions returned by selection, overlay, and Host-binding phases, then canonicalize them by taxonomy slot, Recipe step, expanded-unit order, Add-on, alternative, overlay, incident type, disposition, reason code, and detail. Populate the typed identity fields rather than hiding identities in `Detail`. Host binding owns decisions for unavailable optional units and inactive conditional slots; those decisions cannot be reconstructed later from diagnostics. `materializeGraph` receives that complete decision list and the exact Registry, writes `RegistryDigest=verified.Digest()`, derives an identity-sorted Provider Instance set only from retained units, writes the normalized Selection and Host evidence digest, normalizes every required empty graph collection to a non-nil slice, assigns all cursors, validates the finished record, and computes the final digest. A referenced unit whose Provider Instance is missing or whose digest differs is an internal/trusted-record error, not a recoverable selection. No post-materialization mutation is permitted.

Derive `ResolvedBinding.RequiredFeatures` from `DelegationRequirements` and the exact outer topology. Under `CURRENT`, evaluate only `Child` and `ParallelChild` against `child-delegation` and `parallel-child-delegation`. Under top-level `SUBAGENT`, evaluate only `NestedChild` and `NestedParallel` against `nested-child-delegation` and `nested-parallel-child-delegation`. Preserve the matching live observation digests in the same feature order. Never let evidence for one topology satisfy the other topology's delegation requirement.

An overlay may be suppression-only: an omitted `selected_alternative` applies
only its explicit `paused_bindings`. This records immutable template choices
without fabricating a self-alternative; when `selected_alternative` is present,
the existing exact-one alternative validation still applies.

## Locked Macro Rules

- Expansion is depth-first with a recursion stack keyed by `provider_id + binding_id`.
- Child Unit IDs derive from parent Unit ID, declared call ordinal, and Binding ID; changing declaration order changes the graph digest.
- `credit-only` creates one `CreditInternalOnly` unit, credits its declared contiguous Slot IDs, receives no Grant, and is skipped automatically by actionable traversal.
- `dispatch-before` creates one `DispatchByCoordinator` unit before its parent when both anchor to the same slot; a different earlier anchor follows canonical slot order.
- `dispatch-after` creates one `DispatchByCoordinator` unit after its parent when both anchor to the same slot; a different later anchor follows canonical slot order.
- A macro parent or child appears once at `AnchorSlotID`, the first slot in its ordered contiguous `SlotIDs`. The unit's `SlotIDs` records every credited slot. A credited slot's `OutcomeOwner` references that Unit ID only when the Recipe designates it as the slot outcome owner; no credited slot duplicates the pipeline unit.
- Mandatory suppression, cycles, peer duplication, duplicate credit/dispatch, undeclared alternatives, conflicting responsibility namespaces, and SDD plus Matt TDD overlap return `MACRO_INTERNAL_CONFLICT`.
- `grill-with-docs` credits `grilling` and `domain-modeling` once. SDD dispatches workspace before, credits TDD/task review, and dispatches final verification/finish after. Inline `executing-plans` dispatches workspace before and finish after while retaining standalone TDD/review/verification.

## Locked Cursor and Traversal Rules

Materialization processes the ten slots in taxonomy order. A multi-slot Binding unit is materialized in `CompiledSlot.Pipeline` and `CompiledSlot.Traversal` only at its `AnchorSlotID`; its ordered `SlotIDs` carries the other credits, and only a slot actually owned by that unit points `OutcomeOwner` at it. Within each active slot materialization emits anchored pipeline units in expanded order, then the Host action, then gates in Recipe order, then one terminal marker when the slot is terminal. `Ordinal` is one-based within that slot across every emitted entry, including `credited-internal` and any `omitted` Binding retained inside an active slot. An inactive conditional slot has an empty pipeline and traversal; its omission is retained in `ExecutionGraphRecord.Decisions`. Each emitted cursor is stored on its unit and in `CompiledSlot.Traversal`.

The cursor anchor is immutable:

- Binding: `SlotID=AnchorSlotID`, `Kind=binding`, `UnitID=ResolvedBinding.UnitID`.
- Host action: its owning slot, `Kind=host-action`, `UnitID=action ID`.
- Gate: its owning slot, `Kind=gate`, `UnitID=gate ID`.
- Terminal: its terminal slot, `Kind=terminal`, `UnitID=terminal:<slot-id>`.

`ValidateExecutionGraphRecord` requires every traversal cursor to resolve to exactly one byte-equal unit, every materialized unit to appear exactly once, no duplicate cursor/ordinal, and every incident `HandlerPipeline` cursor to resolve to an anchored handler unit in its declared handler slot. `FirstActionableCursor` and `NextActionableCursor` skip `credited-internal` and `omitted` Bindings without surfacing a Coordinator cursor. They never skip Host actions or gates. At the end of a slot, traversal follows only the exact signal transition. An incident type enters only its exact `HandlerPipeline`; a successful handler follows its declared `ReturnTo` slot, while an unavailable handler follows only its declared `IfUnavailable` stop/replan policy.

`NextActionableCursor` accepts either an empty event for sequential advancement, one signal, or one incident type; signal plus incident together is invalid. It returns `TraversalNext` with exactly one defensively copied cursor, or `TraversalTerminal`, `TraversalStop`, or `TraversalReplan` with a nil cursor. `TraversalTerminal` is legal only after the supplied cursor resolves to the graph's terminal marker. The two fallback dispositions come only from the exact incident route. Plan 05 must use this result and must not recalculate ordinal, anchor, transition, terminal, or fallback rules.

Every incident route remains materialized even when its handler is not selected or not available. `HandlerSlotID` is the handler Binding's incident-recovery anchor. An available handler has its fully expanded cursor pipeline; an unavailable handler has an empty `HandlerPipeline`, one omitted decision, and its declared `IfUnavailable` behavior. That fallback makes the base graph closed rather than silently selecting another Provider. If the user explicitly selected an Add-on that supplies the handler, however, unavailable Add-on selection remains a compile diagnostic and produces no graph.

## Task 1: Atomically Cut Over the Profile Compiler and Execution Graph

**Files:**
- Create: `internal/execution/cursor.go`
- Create: `internal/execution/cursor_test.go`
- Modify: `internal/profile/records.go`
- Create: `internal/profile/host_evidence.go`
- Create: `internal/profile/host_evidence_test.go`
- Create: `internal/profile/resolve.go`
- Create: `internal/profile/macro.go`
- Create: `internal/profile/pipeline.go`
- Create: `internal/profile/traversal.go`
- Modify: `internal/profile/compile.go`
- Modify: `internal/profile/validate.go`
- Modify: `internal/profile/profile_test.go`
- Create: `internal/profile/pipeline_test.go`
- Create: `internal/profile/macro_test.go`
- Create: `internal/profile/traversal_test.go`
- Create: `internal/profile/fuzz_test.go`
- Create: `internal/assets/schemas/v4/execution-graph.schema.json`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`

- [ ] **Step 1: Write the complete RED inventory before production edits**

Add these exact tests and keep their assertions independent:

```go
func TestGraphCursorRejectsUnknownKindEmptyIdentityAndZeroOrdinal(t *testing.T)
func TestGraphV4PinsRegistryHostSelectionProviderDecisionsAndTopology(t *testing.T)
func TestGraphV4RequiresTenCanonicalSlotsAndOneEntry(t *testing.T)
func TestGraphV4RejectsCursorUnitTraversalOrDigestDrift(t *testing.T)
func TestGraphRecordAndCompileResultAreDeeplyImmutable(t *testing.T)
func TestNewHostEvidencePinsOneValidatedSession(t *testing.T)
func TestNewHostEvidenceRejectsHostDigestEnvironmentOrSourceDrift(t *testing.T)
func TestNewHostEvidenceRejectsAbsoluteEvidenceReferences(t *testing.T)
func TestValidateHostEvidenceRecordRejectsManualTopologyAndDigestDrift(t *testing.T)
func TestHostEvidenceRecordIsDeeplyImmutable(t *testing.T)
func TestCompileProfileReturnsDiagnosticsForExpectedUnavailability(t *testing.T)
func TestCompileProfileReturnsErrorForMalformedTrustedEvidence(t *testing.T)
func TestCompileRecipeUsesTheSamePipelineAsCompileProfile(t *testing.T)
func TestCompileRecipeDigestMatchesCatalogNormalizedRecord(t *testing.T)
func TestNormalizeSelectionCanonicalizesAddOnsAlternativesAndRecipeOverlayOrder(t *testing.T)
func TestUnselectedUnavailableAddOnDoesNotFailExactCompilation(t *testing.T)
func TestSelectedUnavailableAddOnReturnsStableDiagnostic(t *testing.T)
func TestSelectedAlternativeResolvesExactVerifiedBinding(t *testing.T)
func TestSelectedOverlayUsesRecipePrecedenceAfterMacroExpansion(t *testing.T)
func TestCompileOrderedMultiBindingPipeline(t *testing.T)
func TestCompileRejectsUnknownBinding(t *testing.T)
func TestCompileRejectsMissingOrAmbiguousOwner(t *testing.T)
func TestCompileRejectsIntraAndCrossSlotArtifactMismatch(t *testing.T)
func TestCompileRejectsNonContiguousStageSpan(t *testing.T)
func TestCompileValidatesMacroEnvelopeArtifacts(t *testing.T)
func TestCompileKeepsProcedureAttachedWithoutMainlineTransition(t *testing.T)
func TestGrillWithDocsCreditsInternalCallsOnce(t *testing.T)
func TestSDDDispatchesBeforeCreditsAndDispatchesAfterOnce(t *testing.T)
func TestInlineExecutorRetainsStandaloneTDDReviewAndVerification(t *testing.T)
func TestHybridDefaultRetainsMattTDDAndExcludesSDDAndSPTDD(t *testing.T)
func TestMacroRejectsCycleMandatoryPausePeerDuplicateAndOwnerConflict(t *testing.T)
func TestCompileValidatesCurrentAndSubagentDelegationIndependently(t *testing.T)
func TestCompileRejectsStaticOrUnattestedFeatureAndActionEvidence(t *testing.T)
func TestHumanExplicitBindingCompilesWithoutInvocationEvidence(t *testing.T)
func TestIncidentRouteUsesExpandedHandlerPipeline(t *testing.T)
func TestIncidentFallbackUsesOnlyDeclaredStopOrReplan(t *testing.T)
func TestCompileRejectsUnreachableSlotNonTerminalCycleAndMissingUserGate(t *testing.T)
func TestCompileAcceptsClosedRemediationLoopWithTerminalExit(t *testing.T)
func TestTraversalAnchorsMultiSlotUnitOnce(t *testing.T)
func TestTraversalAssignsStablePerSlotOrdinals(t *testing.T)
func TestTraversalSkipsCreditedAndOmittedBindings(t *testing.T)
func TestTraversalFollowsExactSignalIncidentReturnAndTerminal(t *testing.T)
func TestTraversalReturnsDistinctStopAndReplanResults(t *testing.T)
func TestTraversalRejectsCursorForAnotherGraphUnit(t *testing.T)
func TestMaterializeGraphPersistsAllDecisionsAndRegistryDigest(t *testing.T)
func TestSchemaRegistryActivatesOnlyExecutionGraphV4(t *testing.T)
```

Seed `FuzzExecutionGraphV4FailsClosed` with one valid Graph v4 JSON record. Mutate schema version, every digest pin, selection, slot activity, owner, pipeline order, cursor kind/anchor/ordinal, traversal membership, incident handler pipeline, Host evidence pin, topology, terminal marker, and unknown fields. Every invalid input must return `PROFILE_GRAPH_RECORD_INVALID` without panic.

- [ ] **Step 2: Run the complete RED gate**

Run: `rtk go test ./internal/execution ./internal/profile ./internal/schema -count=1`

Expected: FAIL because Cursor, Graph v4, Host evidence, candidate resolution, macro, incident pipeline, traversal, and Graph v4 schema contracts do not exist.

- [ ] **Step 3: Implement the shared cursor and trusted Host evidence wrapper**

Implement `internal/execution/cursor.go` and `internal/profile/host_evidence.go` exactly as the locked contracts above. Use `catalog.NormalizeAndDigestRecipe` as the only Recipe normalization/digest path. Reject a zero or manually assembled `HostEvidence`; preserve no absolute Host path, credential, transcript, or user authorization text.

- [ ] **Step 4: Replace every profile record and public compiler entry point atomically**

Replace old graph nodes, flat Binding selections, eligible-topology sets, and terminal-gate fields with the locked records and APIs. Implement the public signatures exactly:

```go
func CompileProfile(source CatalogSource, verified EffectiveRegistry, request CompileRequest) (CompileResult, error)
func CompileRecipe(source CatalogSource, verified EffectiveRegistry, recipe catalog.ProfileRecipeRecord, request CompileRequest) (CompileResult, error)
```

Do not retain an overload or compatibility adapter returning the old `ExecutionGraph`. Remove `CompileError` from the expected-unavailability path; stable compiler diagnostics live only in `CompileResult.Diagnostics()`.

- [ ] **Step 5: Implement declared-candidate resolution and expansion**

Resolve exact `provider_id + binding_id` identities through Registry v4. Compare each retained Binding with the trusted Descriptor's Distribution, surface, kind, reference, invocation, tree digest, artifacts, effects, resources, topologies, delegation, and responsibilities. Keep diagnostics on unselected candidates. Expand every declared default/alternative/Add-on/incident branch depth-first before applying a user choice.

- [ ] **Step 6: Apply the normalized selection and validate the effective graph**

Apply the normalized Add-on set and unique alternative choices, then apply overlay IDs only in Recipe precedence after macro expansion. Validate adjacent and cross-slot artifacts, macro envelopes, contiguous spans, typed ownership, procedures, neutral gates, Host actions, effects/resources, and the exact outer topology. Persist a reason-coded decision for every selected alternative, omitted Add-on, overlay pause/conflict, credited internal, and omitted conditional unit.

- [ ] **Step 7: Implement incident pipelines and canonical traversal**

Compile incident handlers into `HandlerPipeline` cursor lists after macro expansion and Host binding. Implement anchor/ordinal assignment and all four traversal APIs exactly as locked above. Ensure a multi-slot unit is dispatched once and credited elsewhere by Unit ID, not copied into another slot pipeline.

- [ ] **Step 8: Implement strict Graph v4 validation and schema registration**

Create a closed Draft 2020-12 schema with `$id=https://open-agent-workflow.dev/schemas/v4/execution-graph.schema.json`, `additionalProperties: false` at every object, exact enums, strict cursor/unit and transition shapes, and ten-slot constraints enforced jointly by schema and Go validation. Record/evidence/selection/Registry/Provider digests are bare `[0-9a-f]{64}`; content and tree digests use `sha256:<64 lowercase hex>`; Distribution revisions use Plan 01's immutable revision pattern. Add `schema.ExecutionGraphV4` with that exact URL, register Graph v4 as active, and remove every standalone Graph v3 registration or profile validation path without a compatibility reader. Plan 05 separately replaces the old Workflow schemas that embed Graph v3.

- [ ] **Step 9: Run the atomic GREEN gate**

Run: `rtk gofmt -w internal/execution/cursor.go internal/execution/cursor_test.go internal/profile/records.go internal/profile/host_evidence.go internal/profile/host_evidence_test.go internal/profile/resolve.go internal/profile/macro.go internal/profile/pipeline.go internal/profile/traversal.go internal/profile/compile.go internal/profile/validate.go internal/profile/profile_test.go internal/profile/pipeline_test.go internal/profile/macro_test.go internal/profile/traversal_test.go internal/profile/fuzz_test.go internal/schema/registry.go internal/schema/registry_test.go`

Run: `rtk go test -race ./internal/execution ./internal/profile ./internal/schema -count=1`

Expected: PASS. The profile package contains no old graph consumer and all RED inventory tests run.

- [ ] **Step 10: Commit the atomic compiler cutover**

```bash
rtk git add internal/execution/cursor.go internal/execution/cursor_test.go internal/profile/records.go internal/profile/host_evidence.go internal/profile/host_evidence_test.go internal/profile/resolve.go internal/profile/macro.go internal/profile/pipeline.go internal/profile/traversal.go internal/profile/compile.go internal/profile/validate.go internal/profile/profile_test.go internal/profile/pipeline_test.go internal/profile/macro_test.go internal/profile/traversal_test.go internal/profile/fuzz_test.go internal/assets/schemas/v4/execution-graph.schema.json internal/schema/registry.go internal/schema/registry_test.go
rtk git commit -m "feat: compile immutable provider profile graphs"
```

## Locked Builder Records

```go
type BuilderCandidate struct {
    ProviderID            string              `json:"provider_id"`
    BindingID             string              `json:"binding_id"`
    Kind                  catalog.BindingKind `json:"kind"`
    Topology              execution.Topology  `json:"topology"`
    InputArtifact         string              `json:"input_artifact"`
    OutputArtifact        string              `json:"output_artifact"`
    MaximumEffects        []string            `json:"maximum_effects"`
    Resources             []string            `json:"resources"`
    RequiredFeatures      []host.FeatureID    `json:"required_features"`
    Compatible            bool                `json:"compatible"`
    Diagnostics           []CompileDiagnostic `json:"diagnostics"`
    BindingEvidenceDigest string              `json:"binding_evidence_digest"`
}

type BuilderSlot struct {
    SlotID           catalog.SlotID          `json:"slot_id"`
    EntryArtifact    string                  `json:"entry_artifact"`
    OutcomeArtifact  string                  `json:"outcome_artifact"`
    SelectedPipeline []catalog.PipelineStep  `json:"selected_pipeline"`
    SelectedOwner    catalog.OutcomeOwner    `json:"selected_owner"`
    MacroPreview     []ResolvedBinding       `json:"macro_preview"`
    HostAction       *CompiledHostAction     `json:"host_action,omitempty"`
    Gates            []CompiledGate          `json:"gates"`
    IncidentRoutes   []CompiledIncidentRoute `json:"incident_routes"`
    Candidates       []BuilderCandidate      `json:"candidates"`
    Diagnostics      []CompileDiagnostic     `json:"diagnostics"`
}

type BuilderBaseKind string

const (
    BuilderBaseCanonical BuilderBaseKind = "canonical-lifecycle"
    BuilderBaseRecipe    BuilderBaseKind = "recipe"
)

type BuilderSelectionRequest struct {
    Profile      string              `json:"profile"`
    Topology     execution.Topology  `json:"topology"`
    AddOns       []string            `json:"add_ons"`
    Alternatives []AlternativeChoice `json:"alternatives"`
    Overlays     []string            `json:"overlays"`
}

type BuilderProjection struct {
    TaxonomyVersion       string                `json:"taxonomy_version"`
    BaseKind              BuilderBaseKind       `json:"base_kind"`
    BaseRecipeID          string                `json:"base_recipe_id"`
    BaseDigest            string                `json:"base_digest"`
    HostEvidenceDigest    string                `json:"host_evidence_digest"`
    RegistryDigest        string                `json:"registry_digest"`
    Request               BuilderSelectionRequest `json:"request"`
    Selection             *Selection            `json:"selection,omitempty"`
    SelectionDigest       string                `json:"selection_digest,omitempty"`
    Slots                 []BuilderSlot         `json:"slots"`
    PreviewGraph          *ExecutionGraphRecord `json:"preview_graph,omitempty"`
    PreviewGraphDigest    string                `json:"preview_graph_digest,omitempty"`
    Diagnostics           []CompileDiagnostic   `json:"diagnostics"`
    ConfirmationDigest    string                `json:"confirmation_digest,omitempty"`
    Digest                string                `json:"digest"`
}

type ConfirmedRecipe struct {
    Recipe             catalog.ProfileRecipeRecord `json:"recipe"`
    RecipeDigest       string                      `json:"recipe_digest"`
    Selection          Selection                   `json:"selection"`
    RegistryDigest     string                      `json:"registry_digest"`
    ProviderInstances  []GraphProviderInstance     `json:"provider_instances"`
    HostEvidenceDigest string                      `json:"host_evidence_digest"`
    Graph              ExecutionGraphRecord        `json:"graph"`
    ConfirmationDigest string                      `json:"confirmation_digest"`
    Digest             string                      `json:"digest"`
}

type RecipeEdit struct {
    SlotID       catalog.SlotID         `json:"slot_id"`
    Pipeline     []catalog.PipelineStep `json:"pipeline"`
    OutcomeOwner catalog.OutcomeOwner   `json:"outcome_owner"`
}

func NewRecipe(newID, version string) (catalog.ProfileRecipeRecord, error)
func CloneRecipe(source CatalogSource, base, newID, version string) (catalog.ProfileRecipeRecord, error)
func EditRecipe(recipe catalog.ProfileRecipeRecord, edits []RecipeEdit) (catalog.ProfileRecipeRecord, error)
func BuildProjection(source CatalogSource, registry EffectiveRegistry, host HostEvidence, recipe catalog.ProfileRecipeRecord, baseKind BuilderBaseKind, base string, request BuilderSelectionRequest) (BuilderProjection, error)
func ConfirmRecipe(source CatalogSource, registry EffectiveRegistry, host HostEvidence, recipe catalog.ProfileRecipeRecord, request BuilderSelectionRequest, projection BuilderProjection, expectedConfirmationDigest string) (ConfirmedRecipe, error)
func CloneBuilderProjection(value BuilderProjection) BuilderProjection
func ValidateBuilderProjection(value BuilderProjection) error
func CloneConfirmedRecipe(value ConfirmedRecipe) ConfirmedRecipe
func ValidateConfirmedRecipe(value ConfirmedRecipe) error
```

The projection request is for one exact topology and exact Add-on/alternative/overlay choice set. `BuilderCandidate.Compatible` therefore has one unambiguous meaning. Canonicalize `BuilderSelectionRequest` independently of caller order and retain it even for an incomplete draft. Emit Builder slots in taxonomy order, candidates by Provider ID then Binding ID then kind, macro previews in compiler order, and every diagnostic in the compiler's stable order. `Selection` and `SelectionDigest` are present only after the Recipe is complete enough for the normal compiler selection path; never accept a caller-constructed output `Selection` as Builder input. For `BuilderBaseCanonical`, `BaseDigest` is `canonicaljson.Digest(struct{TaxonomyVersion string; Slots []catalog.SlotDefinition}{catalog.TaxonomyVersionV1, catalog.CanonicalSlots()})` and `BaseRecipeID` is empty. For `BuilderBaseRecipe`, `BaseRecipeID` is the selected source Recipe ID and `BaseDigest` is that normalized Recipe's digest. `BuildProjection` uses the same compiler pipeline as `CompileProfile` for an exact preview, expands macro previews, and includes candidate-scoped diagnostics for unavailable choices.

`ConfirmationDigest` is present only when the preview has one valid graph, one canonical Selection, and no blocking projection diagnostics. Candidate-scoped diagnostics on unselected incompatible candidates remain visible and do not block confirmation. The confirmation digest covers the exact Recipe digest, request, Selection/Selection digest, Host evidence digest, Registry digest, Provider Instance digests, expanded preview graph digest, and projection content excluding `ConfirmationDigest` and `Digest`. After setting that optional value, `BuilderProjection.Digest` hashes the complete projection with only `Digest` cleared. `ConfirmRecipe` requires the user-returned expected digest, validates the supplied projection, rebuilds it from the supplied request and current trusted inputs, compares every pin and the projection content, and rejects any Recipe, topology, Add-on, alternative, overlay, Host, Registry, Provider, or preview drift with `PROFILE_SELECTION_INVALID`. `ConfirmedRecipe.Digest` independently hashes the exact Recipe, canonical Selection, Registry/provider pins, Host evidence digest, Graph, and `ConfirmationDigest` with its own Digest field cleared.

## Task 2: Add the USER-DEFINED Profile Builder

**Files:**
- Create: `internal/profile/builder.go`
- Create: `internal/profile/builder_test.go`

- [ ] **Step 1: Write the Builder RED inventory**

```go
func TestBuilderClonesBuiltInWithoutMutation(t *testing.T)
func TestBuilderStartsFromCanonicalLifecycle(t *testing.T)
func TestBuilderListsTrustedCandidatesAndMarksExactVerifiedCompatibility(t *testing.T)
func TestBuilderGreysOutBindingWithStableReason(t *testing.T)
func TestBuilderPreviewExpandsMacrosAndIncidentHandlerPipeline(t *testing.T)
func TestBuilderEditsOrderedPipelineAndOwnerImmutably(t *testing.T)
func TestBuilderRejectsMissingOrDuplicateOwner(t *testing.T)
func TestBuilderProjectsActionsIncidentsEffectsAndFeatures(t *testing.T)
func TestBuilderRejectsStaleHostRegistryProviderOrPreview(t *testing.T)
func TestBuilderRequiresExactReturnedConfirmationDigest(t *testing.T)
func TestBuilderRejectsTopologyAddOnAlternativeOrOverlayDrift(t *testing.T)
func TestBuilderPinsConfirmedRecipeSelectionGraphAndAllDigests(t *testing.T)
func TestBuilderProjectionAndConfirmedRecipeAreDeeplyImmutable(t *testing.T)
```

Include a same-name installed skill with no trusted v4 Descriptor and assert it is absent from eligible candidates. Include one trusted but unavailable Binding and assert it is visible only as incompatible with stable diagnostics.

- [ ] **Step 2: Run RED**

Run: `rtk go test ./internal/profile -run 'Builder|UserDefined|ConfirmedRecipe' -count=1`

Expected: FAIL because the Builder records and exact preview/confirmation functions do not exist.

- [ ] **Step 3: Implement immutable Builder records and exact confirmation**

Implement the locked Builder records and signatures. `NewRecipe` creates a ten-slot canonical draft with `Family="user-defined"` and no template, `CloneRecipe` preserves the source `Family` and `Template` provenance without mutating the source, and `EditRecipe` returns a new value. The projection's `BaseRecipeID` and `BaseDigest` pin the exact cloned source. A draft may be structurally incomplete: `BuildProjection` still returns its canonical request, slot contracts, trusted candidates, and stable draft diagnostics, but it leaves `Selection`, `SelectionDigest`, `PreviewGraph`, `PreviewGraphDigest`, and `ConfirmationDigest` empty. Once the draft is complete, `BuildProjection` invokes the same normalized compiler path as `CompileRecipe`; compiler diagnostics remain the exact preview diagnostics. Only `ConfirmRecipe` may produce a confirmed valid Recipe/Graph pair, and incomplete drafts never enter Catalog or Bundle authority.

- [ ] **Step 4: Run GREEN**

Run: `rtk gofmt -w internal/profile/builder.go internal/profile/builder_test.go`

Run: `rtk go test -race ./internal/profile -run 'Builder|UserDefined|ConfirmedRecipe' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Builder projection and confirmation**

```bash
rtk git add internal/profile/builder.go internal/profile/builder_test.go
rtk git commit -m "feat: project composable user profiles"
```

## Phase Verification

- [ ] Run `rtk go test -race ./internal/execution ./internal/profile ./internal/schema -count=1`.
- [ ] Run `rtk go test ./internal/profile -run '^$' -fuzz '^FuzzExecutionGraphV4FailsClosed$' -fuzztime=10s`.
- [ ] Run `rtk go test -cover ./internal/execution ./internal/profile ./internal/schema -count=1` and require at least 80.0 percent statement coverage for every changed package.
- [ ] Run `rtk go vet ./internal/execution ./internal/profile ./internal/schema`.
- [ ] Run `rtk rg -n 'ExecutionGraphSchemaV3|RecipeNode|ProfileBinding|GraphNode|(^|[[:space:]])ExplicitInvocation[[:space:]]+(bool|string)|RESPONSIBILITY_OWNER|PROFILE_OWNER_' internal/profile internal/execution`.
- [ ] Confirm no production match remains; negative tests may contain old names only as explicit hard-rejection fixtures.
- [ ] Run `rtk rg -n 'TO''DO|TB''D|FIX''ME|<dire''ctory>|<pa''th>|gof''mt -w internal/pro''file([[:space:]]|$)' docs/superpowers/plans/2026-08-10-oaw-provider-surface-v4-03-profile-compiler-builder.md` and require no output.
- [ ] Run `rtk git diff --check` and inspect `rtk git status --short`.
- [ ] Confirm the Plan 03 diff contains no `internal/integration/`, built-in asset, Core, Admission, Host Receipt, or Coordinator path.

Expected: the same compiler powers exact eligibility, USER-DEFINED previews, confirmation, and final graph construction; arbitrary installed content never becomes authority; Plan 04 can add four-profile fixtures without changing the compiler contract.
