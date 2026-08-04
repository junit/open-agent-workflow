# OAW Core Coordinator Phase 01 Core Contracts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans in the current session to implement this plan task-by-task. This plan is locked to `CURRENT`; do not dispatch subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the old executor contract with `CURRENT` and `SUBAGENT`, upgrade Provider/Profile/config schemas without compatibility readers, and expose one stateless OAW Core API for classification, Host-scoped resolution, eligibility, and immutable Lifecycle Bundle compilation.

**Architecture:** Add a small `internal/execution` contract module shared by Catalog, Profile, Host, Core, and Coordinator. Keep classification, discovery, Registry, and Profile algorithms in their existing focused packages; add `internal/core` as their only caller-facing policy-decision facade. Changed schemas are hard replacements and old enum values fail explicitly.

**Tech Stack:** Go 1.26, JSON Schema Draft 2020-12, strict TOML decoding, canonical JSON digests, table-driven Go tests, repository shell gates.

---

**Selected execution:** `CURRENT`. Do not dispatch subagents. Do not publish an intermediate phase commit.

**Hard-cut integration boundary:** Phases 01-04 are one unreleased contract-replacement batch. Removing old public types in this phase intentionally leaves old Host, Runtime, Admission, and Runner consumers unable to compile until their owning phases replace or delete them. Do not add aliases, dual schema readers, deprecated fields, or temporary adapters to make an intermediate tree globally green. Run the leaf-package checks named by each task; the first required `go test ./...` gate is Phase 04 Task 4 after Runner deletion.

**Depends on:** Approved design `docs/superpowers/specs/2026-08-05-oaw-core-coordinator-hard-cutover-design.md`.

**Produces:** Core contracts consumed by Phase 02 and Phase 03.

## File Map

| Path | Responsibility |
| --- | --- |
| `internal/execution/records.go` | Closed topology and environment requirement value records. |
| `internal/execution/normalize.go` | Normalization, set intersection, and requirement satisfaction. |
| `internal/execution/execution_test.go` | Public-seam topology and environment tests. |
| `internal/catalog/records.go` | Provider v3 and Recipe v2 records. |
| `internal/catalog/decode.go` | Hard-version decoding and closed-field validation. |
| `internal/catalog/validate.go` | Topology, binding, environment, and Recipe invariants. |
| `internal/assets/schemas/v3/provider-descriptor.schema.json` | Provider descriptor v3 schema. |
| `internal/assets/schemas/v2/profile-recipe.schema.json` | Profile Recipe v2 schema. |
| `internal/assets/schemas/v3/user-config.schema.json` | User configuration v3 schema. |
| `internal/assets/providers/*.json` | Built-in Provider records using topology sets. |
| `internal/assets/recipes/*.json` | Built-in Recipes with environment requirements. |
| `internal/profile/{records,compile,validate}.go` | Topology-aware graph compilation. |
| `internal/core/records.go` | Caller-facing stateless Core request/result records. |
| `internal/core/classify.go` | Core-owned request classification facade. |
| `internal/core/resolve.go` | Core-owned Host-scoped Provider resolution facade. |
| `internal/core/compile.go` | Profile eligibility and Lifecycle Bundle compiler. |
| `internal/core/core_test.go` | Built-in, user-defined, topology, add-on, and digest tests. |
| `internal/config/{records,decode,snapshot}.go` | User config v3 and hard-version snapshots. |
| `internal/schema/registry.go` | Active schema set only. |

## Locked Records

Create these names exactly:

```go
package execution

type Topology string

const (
    TopologyCurrent  Topology = "CURRENT"
    TopologySubagent Topology = "SUBAGENT"
)

type EnvironmentDisposition string

const (
    DispositionInherited     EnvironmentDisposition = "inherited"
    DispositionHostConfigured EnvironmentDisposition = "host-configured"
    DispositionRestricted    EnvironmentDisposition = "restricted"
    DispositionUnknown       EnvironmentDisposition = "unknown"
    DispositionUnavailable   EnvironmentDisposition = "unavailable"
)

type EnvironmentRequirement struct {
    Surface              string                   `json:"surface" toml:"surface"`
    Required             bool                     `json:"required" toml:"required"`
    AcceptedDispositions []EnvironmentDisposition `json:"accepted_dispositions" toml:"accepted_dispositions"`
}

type EnvironmentObservation struct {
    Surface     string                 `json:"surface"`
    Disposition EnvironmentDisposition `json:"disposition"`
    Source      string                 `json:"source"`
    Digest      string                 `json:"digest"`
}
```

`NormalizeTopologies`, `IntersectTopologies`, and
`ValidateEnvironmentRequirements` return defensive, sorted values. Empty or
duplicate topology sets, unknown values, duplicate surfaces, and a required
surface without an accepted disposition fail closed.

Catalog records change atomically:

```go
type CapabilityRecord struct {
    ID                    string
    InputSchema           string
    OutcomeSchema         string
    MaximumEffects        []string
    Resources             []string
    RequestModes          []RequestMode
    Responsibilities      []string
    SupportedTopologies   []execution.Topology
    DelegationAllowList   []string
    HostBindings          []HostBinding
}

type HostBinding struct {
    Host       string
    Kind       string
    Reference  string
    Topologies []execution.Topology
}

type ProfileRecipeRecord struct {
    SchemaVersion            string                             `json:"schema_version" toml:"schema_version"`
    RecipeVersion            string                             `json:"recipe_version" toml:"recipe_version"`
    ID                       string                             `json:"id" toml:"id"`
    DisplayName              string                             `json:"display_name" toml:"display_name"`
    RequiredResponsibilities []string                           `json:"required_responsibilities" toml:"required_responsibilities"`
    Nodes                    []RecipeNode                       `json:"nodes" toml:"nodes"`
    IncidentRoutes           []IncidentRoute                    `json:"incident_routes" toml:"incident_routes"`
    Entry                    string                             `json:"entry" toml:"entry"`
    TerminalGates            []string                           `json:"terminal_gates" toml:"terminal_gates"`
    StableBoundaries         []string                           `json:"stable_boundaries" toml:"stable_boundaries"`
    EnvironmentRequirements []execution.EnvironmentRequirement `json:"environment_requirements" toml:"environment_requirements"`
}
```

Delete `ExecutorTopology`, `MainAgentAllowed`, `IsolatedRequired`, and the
`executor_topology` field. Do not retain aliases or a v2 Provider decoder.

## Task 1: Add the closed execution contract module

**Files:**
- Create: `internal/execution/records.go`
- Create: `internal/execution/normalize.go`
- Create: `internal/execution/execution_test.go`

- [ ] **Step 1: Write failing topology normalization tests**

Add `TestNormalizeTopologiesAcceptsOnlyCurrentAndSubagent`,
`TestIntersectTopologiesIsDeterministic`, and
`TestEnvironmentRequirementsFailClosed`. The first test must include the old
`INLINE`, `NATIVE_SUBAGENT`, `main-agent-allowed`, and `isolated-required`
values and assert `EXECUTION_TOPOLOGY_INVALID`.

- [ ] **Step 2: Run the tests to verify RED**

```bash
rtk go test ./internal/execution
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement the exact records and normalizers**

```go
func NormalizeTopologies(values []Topology) ([]Topology, error)
func IntersectTopologies(sets ...[]Topology) ([]Topology, error)
func NormalizeRequirements(values []EnvironmentRequirement) ([]EnvironmentRequirement, error)
func RequirementsSatisfied(requirements []EnvironmentRequirement, observations []EnvironmentObservation) error
```

Use `EXECUTION_TOPOLOGY_INVALID`, `ENVIRONMENT_REQUIREMENT_INVALID`, and
`ENVIRONMENT_REQUIREMENT_UNSATISFIED` as stable error prefixes. Never infer
`CURRENT` for an empty input.

- [ ] **Step 4: Format and run GREEN checks**

```bash
rtk gofmt -w internal/execution
rtk go test ./internal/execution
rtk go vet ./internal/execution
```

Expected: PASS.

- [ ] **Step 5: Commit the execution value module**

```bash
rtk git add internal/execution
rtk git commit -m "feat: define execution topology contracts"
```

## Task 2: Replace Provider and Recipe schemas

**Files:**
- Modify: `internal/catalog/records.go`
- Modify: `internal/catalog/decode.go`
- Modify: `internal/catalog/validate.go`
- Modify: `internal/catalog/decode_test.go`
- Modify: `internal/catalog/catalog_test.go`
- Delete: `internal/assets/schemas/v2/provider-descriptor.schema.json`
- Create: `internal/assets/schemas/v3/provider-descriptor.schema.json`
- Delete: `internal/assets/schemas/v1/profile-recipe.schema.json`
- Create: `internal/assets/schemas/v2/profile-recipe.schema.json`
- Modify: `internal/assets/providers/oaw-superpowers.json`
- Modify: `internal/assets/providers/oaw-matt.json`
- Modify: `internal/assets/providers/oaw-ecc.json`
- Modify: `internal/assets/recipes/oaw-delivery.json`
- Modify: `internal/assets/recipes/oaw-domain-engineering.json`
- Modify: `internal/assets/recipes/oaw-ecc-engineering.json`
- Modify: `internal/assets/recipes/oaw-reliable-feature.json`
- Modify: `internal/assets/recipes/oaw-hardening.json`
- Modify: `internal/assets/embed.go`
- Modify: `internal/assets/embed_test.go`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`

- [ ] **Step 1: Write hard-version and topology-set schema tests**

Use this minimum valid v3 Capability shape in tests:

```json
{
  "id": "implementation",
  "input_schema": "oaw.capability-input/v1",
  "outcome_schema": "oaw.capability-outcome/v1",
  "maximum_effects": ["read-project", "write-project"],
  "resources": ["project-worktree"],
  "request_modes": ["WORKFLOW"],
  "responsibilities": ["implementation"],
  "supported_topologies": ["CURRENT", "SUBAGENT"],
  "delegation_allow_list": [],
  "host_bindings": [{
    "host": "codex",
    "kind": "skill",
    "reference": "acme:implementation",
    "topologies": ["CURRENT", "SUBAGENT"]
  }]
}
```

Assert v2 Provider, v1 Recipe, singular topology fields, empty topology sets,
and a binding topology outside its Capability set fail. Add
`TestBuiltinBindingsRemainHostScoped`: Superpowers and ECC have separate
`codex` and `claude` binding records because their built-in discovery probes
declare both Hosts; Matt remains `codex`-only until a trusted Claude discovery
surface and binding are explicitly registered. No Host may satisfy another
Host's binding.

- [ ] **Step 2: Run schema tests to verify RED**

```bash
rtk go test ./internal/catalog ./internal/schema ./internal/assets -run 'Schema|Topology|Version|Builtin'
```

Expected: FAIL on missing v3/v2 schemas and old Go fields.

- [ ] **Step 3: Replace records, decoders, validators, and embedded schemas**

Set `ProviderDescriptorSchemaV3 = "oaw.provider-descriptor/v3"` and
`ProfileRecipeSchemaV2 = "oaw.profile-recipe/v2"`. Make `additionalProperties:
false` and require `environment_requirements` on every Recipe, using an empty
array when none are needed.

- [ ] **Step 4: Convert all built-in Providers and Recipes**

Every built-in workflow Capability and binding declares both possible
topologies. This is only descriptor eligibility; Phase 02 Host session evidence
still decides whether `SUBAGENT` is available. Preserve Provider IDs,
Capability IDs, Recipe IDs, responsibilities, graph edges, and ECC-FULL
coverage. Give Superpowers and ECC distinct `codex` and `claude` bindings with
the target Host's own Skill/Agent reference. Do not copy a discovered instance,
inventory observation, pin, or evidence digest across Hosts, and do not invent
a Matt Claude binding.

- [ ] **Step 5: Run catalog GREEN checks**

```bash
rtk gofmt -w internal/catalog internal/schema internal/assets
rtk go test ./internal/catalog ./internal/schema ./internal/assets
```

Expected: PASS; active schema registry contains no Provider v2 or Recipe v1.

- [ ] **Step 6: Commit the schema hard cut**

```bash
rtk git add internal/catalog internal/schema internal/assets
rtk git commit -m "feat: replace provider topology schemas"
```

## Task 3: Compile topology-aware graphs and explicit add-ons

**Files:**
- Modify: `internal/registry/records.go`
- Modify: `internal/registry/resolve.go`
- Modify: `internal/registry/registry_test.go`
- Modify: `internal/profile/records.go`
- Modify: `internal/profile/compile.go`
- Modify: `internal/profile/validate.go`
- Modify: `internal/profile/profile_test.go`

- [ ] **Step 1: Write failing compiler intersection tests**

Add tests proving the graph topology set is exactly:

```text
Host session topologies
AND every selected Capability topology set
AND every verified binding topology set
AND accepted environment requirements
```

Also prove an optional node appears only when its ID is explicitly present in
`CompileRequest.AddOns`; unknown, duplicate, unavailable, or required-node
add-ons fail with `PROFILE_ADD_ON_INVALID`.

- [ ] **Step 2: Run focused tests to verify RED**

```bash
rtk go test ./internal/registry ./internal/profile -run 'Topology|AddOn|Compile'
```

Expected: RED from the new assertions. After Provider v3 replaces the old
catalog fields, a compile error from the not-yet-replaced Host package is an
allowed batch blocker; any other compile error must be fixed in this task.

- [ ] **Step 3: Pin verified binding topology evidence**

```go
type VerifiedCapability struct {
    ID                    string
    Binding               catalog.HostBinding
    SupportedTopologies   []execution.Topology
    BindingEvidenceDigest string
}
```

Copy the selected binding topology set into the verified record; never widen it
from another binding belonging to the same Capability.

- [ ] **Step 4: Replace graph topology fields and compile request**

```go
type CompileRequest struct {
    Profile                 string
    Bindings                []ProfileBinding
    AddOns                  []string
    HostTopologies          []execution.Topology
    EnvironmentObservations []execution.EnvironmentObservation
}

type ExecutionGraphRecord struct {
    SchemaVersion           string                             `json:"schema_version"`
    HostID                  string                             `json:"host_id"`
    RecipeID                string                             `json:"recipe_id"`
    RecipeVersion           string                             `json:"recipe_version"`
    RecipeDigest            string                             `json:"recipe_digest"`
    Entry                   string                             `json:"entry"`
    Bindings                []ProfileBinding                   `json:"bindings"`
    ProviderInstances       []GraphProviderInstance            `json:"provider_instances"`
    Nodes                   []GraphNode                        `json:"nodes"`
    IncidentRoutes          []GraphIncidentRoute               `json:"incident_routes"`
    TerminalGates           []string                           `json:"terminal_gates"`
    StableBoundaries        []string                           `json:"stable_boundaries"`
    EligibleTopologies      []execution.Topology               `json:"eligible_topologies"`
    EnvironmentRequirements []execution.EnvironmentRequirement `json:"environment_requirements"`
    Digest                  string                             `json:"digest"`
}

type GraphNode struct {
    ID                     string                  `json:"id"`
    Kind                   catalog.NodeKind        `json:"kind"`
    Responsibility         string                  `json:"responsibility"`
    Phase                  string                  `json:"phase,omitempty"`
    Optional               bool                    `json:"optional,omitempty"`
    ProviderID             string                  `json:"provider_id"`
    ProviderInstanceDigest string                  `json:"provider_instance_digest"`
    CapabilityID           string                  `json:"capability_id"`
    Binding                catalog.HostBinding     `json:"binding"`
    InputSchema            string                  `json:"input_schema"`
    OutcomeSchema          string                  `json:"outcome_schema"`
    MaximumEffects         []string                `json:"maximum_effects"`
    Resources              []string                `json:"resources"`
    RequestModes           []catalog.RequestMode   `json:"request_modes"`
    SupportedTopologies    []execution.Topology    `json:"supported_topologies"`
    DelegationAllowList    []string                `json:"delegation_allow_list"`
    Transitions            []GraphTransition       `json:"transitions"`
}
```

Each `GraphNode` stores `SupportedTopologies`, not an executor kind. Reject a
required graph with an empty intersection using `PROFILE_TOPOLOGY_UNAVAILABLE`.

- [ ] **Step 5: Run compiler and determinism checks**

```bash
rtk gofmt -w internal/registry internal/profile
rtk go test ./internal/registry ./internal/profile
rtk go test -race ./internal/profile ./internal/registry
```

Expected after Phase 04 closes the batch: PASS with deterministic digests under
shuffled inputs. At this checkpoint, record the exact stale Host/Driver symbols
if they still block compilation and replay both commands in Phase 04 Task 4.

- [ ] **Step 6: Commit the compiler cut**

```bash
rtk git add internal/registry internal/profile
rtk git commit -m "feat: compile topology-aware profiles"
```

## Task 4: Replace user configuration with v3

**Files:**
- Modify: `internal/config/records.go`
- Modify: `internal/config/decode.go`
- Modify: `internal/config/snapshot.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/config/snapshot_test.go`
- Delete: `internal/assets/schemas/v2/user-config.schema.json`
- Create: `internal/assets/schemas/v3/user-config.schema.json`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`

- [ ] **Step 1: Write failing v3-only configuration tests**

Use `schema_version = "oaw.user-config/v3"` and v3 Provider/v2 Recipe source
fixtures. Assert v2 is rejected with `CONFIG_SCHEMA_UNSUPPORTED`; no migration
or rewritten output is produced.

- [ ] **Step 2: Run configuration tests to verify RED**

```bash
rtk go test ./internal/config ./internal/schema -run 'UserConfig|Schema|Snapshot'
```

- [ ] **Step 3: Replace active constants and source validation**

```go
const UserConfigSchemaV3 = "oaw.user-config/v3"
```

Require user Provider sources to decode as v3 and user Recipes as v2. Preserve
existing trust, pin, denial, installation, binding preference, bounded default,
and project trust semantics.

- [ ] **Step 4: Run GREEN and fuzz-adjacent decode checks**

```bash
rtk gofmt -w internal/config internal/schema
rtk go test ./internal/schema
rtk go test ./internal/config
```

Expected after Phase 04 closes the batch: PASS. The schema package must pass at
this checkpoint; the config package may report only the known old Host contract
compile blocker.

- [ ] **Step 5: Commit configuration v3**

```bash
rtk git add internal/config internal/schema internal/assets/schemas
rtk git commit -m "feat: require user configuration v3"
```

## Task 5: Add the stateless OAW Core compilation seam

**Files:**
- Create: `internal/core/records.go`
- Create: `internal/core/classify.go`
- Create: `internal/core/resolve.go`
- Create: `internal/core/compile.go`
- Create: `internal/core/core_test.go`

- [ ] **Step 1: Write failing eligibility and Bundle tests**

Test classification facade parity, Host-scoped resolution facade parity,
no-selection output, explicit selection, invalid Profile, invalid topology,
unselected add-on, ambiguous Provider, caller-authored digest rejection, and
equivalent-input determinism. Cover all four built-in Profiles plus one
user-defined Provider and Recipe.

- [ ] **Step 2: Run Core tests to verify RED**

```bash
rtk go test ./internal/core -run 'Core|Eligibility|LifecycleBundle'
```

Expected: FAIL because `internal/core` does not exist.

- [ ] **Step 3: Implement the locked Core records**

```go
type SelectionSource string

const (
    SelectionUser           SelectionSource = "user-selection"
    SelectionHostOnlyOption SelectionSource = "host-only-option"
)

type Selection struct {
    Profile        string                   `json:"profile"`
    ProfileSource  SelectionSource          `json:"profile_source"`
    Topology       execution.Topology       `json:"topology"`
    TopologySource SelectionSource          `json:"topology_source"`
    AddOns         []string                 `json:"add_ons"`
    Bindings       []profile.ProfileBinding `json:"bindings"`
}

type CompilationRequest struct {
    DeliverableID            string
    InputDigest              string
    Generation               uint64
    Classification           classification.ClassificationDecision
    Configuration            config.Snapshot
    Resolutions              registry.ResolutionReport
    Registry                 registry.Registry
    HostID                   string
    HostSessionDigest        string
    HostProviderInventoryDigest string
    HostTopologies           []execution.Topology
    EnvironmentObservations  []execution.EnvironmentObservation
    Selection                *Selection
}

type ResolutionRequest struct {
    Configuration config.Snapshot
    HostID         string
    Discovery      discovery.Report
    Inventory      *host.BindingInventory
}

type ResolutionResult struct {
    Report   registry.ResolutionReport
    Registry registry.Registry
    Digest   string
}

type EligibilityDiagnostic struct {
    Code         string             `json:"code"`
    ProviderID   string             `json:"provider_id,omitempty"`
    CapabilityID string             `json:"capability_id,omitempty"`
    Topology     execution.Topology `json:"topology,omitempty"`
    Detail       string             `json:"detail"`
}

type ProfileEligibility struct {
    Profile              string                  `json:"profile"`
    RecipeID             string                  `json:"recipe_id"`
    Eligible             bool                    `json:"eligible"`
    EligibleTopologies   []execution.Topology    `json:"eligible_topologies"`
    Diagnostics          []EligibilityDiagnostic `json:"diagnostics"`
    Recommended          bool                    `json:"recommended"`
    RecommendationReason string                  `json:"recommendation_reason,omitempty"`
}

type AddOnEligibility struct {
    NodeID               string                  `json:"node_id"`
    ProviderID           string                  `json:"provider_id"`
    CapabilityID         string                  `json:"capability_id"`
    EligibleTopologies   []execution.Topology    `json:"eligible_topologies"`
    Diagnostics          []EligibilityDiagnostic `json:"diagnostics"`
}

type LifecycleBundle struct {
    SchemaVersion               string                             `json:"schema_version"`
    ID                          string                             `json:"id"`
    DeliverableID               string                             `json:"deliverable_id"`
    InputDigest                 string                             `json:"input_digest"`
    Generation                  uint64                             `json:"generation"`
    Classification              classification.ClassificationDecision `json:"classification"`
    ClassificationDigest        string                             `json:"classification_digest"`
    Selection                   Selection                          `json:"selection"`
    HostID                      string                             `json:"host_id"`
    HostSessionDigest           string                             `json:"host_session_digest"`
    ProviderInventoryDigest     string                             `json:"provider_inventory_digest"`
    Configuration               config.SnapshotRecord              `json:"configuration"`
    ResolutionDigest            string                             `json:"resolution_digest"`
    RegistryDigest              string                             `json:"registry_digest"`
    ProviderInstances           []profile.GraphProviderInstance    `json:"provider_instances"`
    Graph                       profile.ExecutionGraphRecord        `json:"execution_graph"`
    Topology                    execution.Topology                  `json:"topology"`
    EnvironmentRequirements     []execution.EnvironmentRequirement `json:"environment_requirements"`
    EnvironmentObservations     []execution.EnvironmentObservation `json:"environment_observations"`
    AddOns                      []string                            `json:"add_ons"`
    Digest                      string                              `json:"digest"`
}

type CompilationResult struct {
    EligibleProfiles []ProfileEligibility
    EligibleAddOns   []AddOnEligibility
    Bundle           *LifecycleBundle
    Digest           string
}

func Compile(request CompilationRequest) (CompilationResult, error)
func Classify(proposal *classification.ClassificationProposal, rules classification.ClassificationRules) (classification.ClassificationDecision, error)
func Resolve(request ResolutionRequest) (ResolutionResult, error)
```

`ProfileSource` is always `user-selection`. `TopologySource` is
`user-selection` when more than one topology is eligible and may be
`host-only-option` only when `CURRENT` is the sole eligible topology. A
recommendation is never a valid selection source.

`Classify` is the only caller-facing classifier and delegates to the existing
deterministic classification package. `Resolve` is the only caller-facing
Host-scoped resolver and delegates to Registry resolution after validating that
Configuration, discovery evidence, and binding inventory agree on Host ID.
Callers do not construct a `ResolutionResult` or bypass its digest.

With `Selection == nil`, return only normalized eligibility. With a selection,
recompile and return one immutable `oaw.lifecycle-bundle/v3`. The Bundle pins
classification, selection, Host session, configuration, Registry, Provider,
graph, topology, environment, generation, and canonical digest. Never accept a
Bundle as input.

- [ ] **Step 4: Prove Bundle copies and digests are immutable**

Mutate every slice and nested record returned by `CompilationResult`, then call
`Compile` again and assert identical canonical bytes and digest.

- [ ] **Step 5: Run the Phase 01 leaf-package verification**

```bash
rtk gofmt -w internal/core
rtk go test ./internal/execution ./internal/catalog ./internal/schema ./internal/assets
rtk go test -race ./internal/execution ./internal/catalog
rtk bash scripts/check-docs.sh
rtk git diff --check
```

Expected: the migrated leaf packages pass. Core/Profile/Registry/Config tests
may remain blocked because their dependency closure still contains the old Host
records scheduled for Phase 02 and the old Driver scheduled for deletion in
Phase 04. Record only those known compile blockers; do not make them compatible.
Do not build or publish a release artifact.

- [ ] **Step 6: Commit OAW Core**

```bash
rtk git add internal/core
rtk git commit -m "feat: compile lifecycle bundles in OAW core"
```

## Phase 01 Completion Gate

- [ ] Active Provider, Recipe, and user-config schema registries contain only
      v3, v2, and v3 respectively.
- [ ] `rg` finds no `ExecutorTopology`, `executor_topology`,
      `main-agent-allowed`, or `isolated-required` reference in the migrated
      `internal/execution`, `internal/catalog`, `internal/profile`,
      `internal/registry`, `internal/config`, or `internal/core` contracts.
      Remaining matches must be confined to the Phase 02-04 deletion/replacement
      set and are not repaired.
- [ ] Core has no filesystem mutation, process invocation, state journal, or
      Host child creation code.
- [ ] `CURRENT` and `SUBAGENT` are the only topology values accepted by the new
      contract source; Phase 04 proves the same property for the complete tree.
