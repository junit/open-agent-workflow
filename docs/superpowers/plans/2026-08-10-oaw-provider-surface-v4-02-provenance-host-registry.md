# OAW Provider Surface v4 02: Provenance, Host, and Registry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve only Distribution-proven and content-exact Host Bindings, add truthful Host v3 evidence for Binding kinds, delegation features, and neutral actions, and retain every verified Binding alternative in Registry v4.

**Architecture:** Add one reusable integrity package for complete Binding-tree evidence, then thread that evidence through discovery, Host inventory, Config's catalog-v4 consumer lookups, and Registry resolution. Discovery reports candidates but never grants authority; Host observations prove exact callable surfaces; Registry intersects both with the trusted Descriptor. Multiple verified alternatives remain available for the compiler instead of being lexically collapsed.

**Tech Stack:** Go 1.26, `filepath.WalkDir`, `os.Lstat`, canonical JSON SHA-256, JSON Schema Draft 2020-12, table-driven and filesystem tests.

---

**Selected lifecycle:** `SP-FULL / CURRENT / no Add-on`.

**Depends on:** Plan 01 catalog records and active Provider Descriptor v4/Profile Recipe v3 schemas.

**Produces:** Host Manifest/Integration/Integration Set/Session/Binding Inventory v3, Conformance Transcript/Report v3 (Receipt v2 bridge), discovery report v3, User Config v3 consumers of catalog v4, Provider Instance/Resolution/Registry v4, and the exact verified Binding set consumed by Plan 03.

## Execution Boundary

Before Task 1, run `rtk git rev-parse HEAD`, `rtk git status --short`, and
`rtk git diff --check`. Preserve unrelated worktree changes and do not stage
them. Execute Tasks 1-5 in order, using the RED/GREEN checks and one focused
commit per task. The phase gates in this plan intentionally exclude downstream
Core, Profile compiler, Coordinator, Bridge, and built-in Provider/Recipe
matrix consumers; Plan 06 is the first full-tree gate after those owners have
migrated. No v2 authority record may be converted, aliased, or silently read
while this plan is implemented.

## File Map

| Path | Responsibility |
| --- | --- |
| `internal/integrity/tree.go` | Canonical complete-tree enumeration and digesting with symlink/path rejection. |
| `internal/integrity/tree_test.go` | Content, mode, path, symlink, and deterministic-order tests. |
| `internal/discovery/records.go` | Distribution-scoped evidence/candidate/report v3 records. |
| `internal/discovery/attest.go` | Exact Provider-root revision attestation and content-equivalence classification. |
| `internal/discovery/discover.go` | Candidate discovery plus exact Distribution and Binding-root evidence. |
| `internal/discovery/discovery_test.go` | Provenance disposition, shared-ancestor, and drift tests. |
| `internal/host/records.go` | Host Manifest/Integration/Session/Conformance records and v3 constants. |
| `internal/host/bindings.go` | Binding Inventory v3 records and validation. |
| `internal/host/actions.go` | Neutral Host action contracts and live availability observations. |
| `internal/host/validate.go` | Closed Host v3 constructors and old Host rejection. |
| `internal/host/decode.go` | Strict v3 Manifest decoding with no v2 conversion. |
| `internal/host/session.go` | Construct Session v3 against one exact Manifest v3. |
| `internal/host/builtin.go` | Decode only Integration Set v3 and deeply clone its records. |
| `internal/host/conformance.go` | Re-pin active conformance records to Session/Inventory v3. |
| `internal/host/environment.go` | Validate Environment Report against Session v3. |
| `internal/host/records_test.go` | Manifest, Session, Integration, cloning, and strict decode tests. |
| `internal/host/bindings_test.go` | Binding-kind, evidence-reference, digest, and inventory tests. |
| `internal/host/actions_test.go` | Delegation feature and neutral action observation tests. |
| `internal/host/validation_test.go` | v2 hard-cut and invalid v3 record tests. |
| `internal/host/builtin_test.go` | Built-in Integration Set v3 loading and immutability tests. |
| `internal/host/conformance_test.go` | Transcript/Report v3, Session/Inventory pins, and legacy receipt coverage. |
| `internal/host/conformance_fuzz_test.go` | Closed Conformance v3 decoding and defensive-boundary fuzzing. |
| `internal/assets/schemas/v3/host-manifest.schema.json` | Host Manifest v3 schema. |
| `internal/assets/schemas/v3/host-binding-inventory.schema.json` | Host Binding Inventory v3 schema. |
| `internal/assets/schemas/v3/host-session.schema.json` | Host Session v3 schema. |
| `internal/assets/schemas/v3/host-integration.schema.json` | Host Integration v3 referencing Manifest v3. |
| `internal/assets/schemas/v3/host-integration-set.schema.json` | Integration Set v3 containing only Integration v3 records. |
| `internal/assets/schemas/v3/host-conformance-transcript.schema.json` | Active transcript schema referencing Session/Inventory v3. |
| `internal/assets/schemas/v3/host-conformance-report.schema.json` | Active conformance report for the v3 transcript. |
| `internal/assets/host-integrations.json` | Regenerate the built-in Integration Set as v3. |
| `internal/assets/conformance/codex-host-v3.json` | Replace the active Codex transcript fixture with canonical Conformance v3. |
| `internal/assets/conformance/codex-host-v1.json` | Delete after the v3 fixture is active; it is a generated fixture, not retained authority. |
| `internal/assets/generate/codex_host.go` | Generate conservative v3 Host fixtures without importing the still-v1 Codex Bridge. |
| `internal/assets/generate/codex_host_test.go` | Assert host-only generated v3 assets are canonical and reproducible. |
| `internal/registry/records.go` | Provider Instance/Resolution v4 and verified Binding alternatives. |
| `internal/registry/resolve.go` | Exact Descriptor/Distribution/Host-evidence intersection. |
| `internal/registry/registry.go` | Immutable multi-Binding Registry v4 lookup API. |
| `internal/registry/registry_test.go` | Migrate existing asset-backed tests to compile against v4 records. |
| `internal/registry/resolve_internal_test.go` | Synthetic-source provenance, preference, alternatives, and immutability tests independent of Plan 04 assets. |
| `internal/profile/records.go` | Extend only the forward `EffectiveRegistry` consumer contract with Binding lookups. |
| `internal/config/decode.go` | Keep User Config v3 while accepting the five catalog Binding kinds and exact preference identities. |
| `internal/config/snapshot.go` | Resolve Binding preferences through Descriptor Binding IDs, never legacy HostBinding fields or ancestry. |
| `internal/config/config_test.go` | User Config v3/provider v4 consumer and Binding-preference hard-cut tests. |
| `internal/config/snapshot_test.go` | Migrate Host Integration fixtures to v3 so all Config tests compile at the hard cut. |
| `internal/assets/schemas/v3/user-config.schema.json` | User Config v3 enum for the five active Binding kinds. |
| `internal/schema/registry.go` | Register only active Host v3 schema identifiers. |
| `internal/schema/registry_test.go` | Validate active v3 schemas and reject superseded Host authority schemas. |
| `internal/assets/embed_test.go` | Refresh deferred Provider v4/Recipe v3 metadata and assert every active Host v3 schema/fixture. |

## Locked Evidence Records

```go
package integrity

type TreeEntry struct {
    Path    string `json:"path"`
    Mode    uint32 `json:"mode"`
    Size    int64  `json:"size"`
    Digest  string `json:"digest"`
}

type TreeEvidence struct {
    RootDigest string      `json:"root_digest"`
    Entries    []TreeEntry `json:"entries"`
}

func DigestTree(root string) (TreeEvidence, error)
```

`DigestTree` hashes a canonical, path-sorted list of regular files. Each entry records a slash-normalized relative path, executable mode bits (`mode & 0o111`), byte size, and a content digest matching `^sha256:[0-9a-f]{64}$`; `TreeEvidence.RootDigest` uses the same exact form. Add a dedicated `integrity.SHA256Digest([]byte) string` helper for these prefixed content digests; do not change `canonicaljson.Digest`, whose record digests remain bare lowercase hex. Directories are traversed but do not enter the digest. Empty trees, symlinks, FIFO/device/socket entries, escaping paths, root replacement, and detectable identity/size/mode/mtime drift during a read fail with `BINDING_TREE_INVALID`.

Discovery v3 keeps ordered evidence instead of an undefined map-shaped `BindingRoots`:

```go
const (
    discoveryEvidenceSchemaV3 = "oaw.discovery-evidence/v3"
    discoveryReportSchemaV3   = "oaw.discovery-report/v3"
)

type ProvenanceDisposition string

const (
    ProvenanceDistributionAttested ProvenanceDisposition = "distribution-attested"
    ProvenanceContentEquivalent    ProvenanceDisposition = "content-equivalent"
)

type BindingRootEvidence struct {
    BindingID   string                 `json:"binding_id"`
    ContentRoot string                 `json:"content_root"`
    InstallRoot string                 `json:"install_root"`
    Tree        integrity.TreeEvidence `json:"tree"`
}

type Evidence struct {
    ProviderID          string                `json:"provider_id"`
    HostID              string                `json:"host_id"`
    Surface             string                `json:"surface"`
    DistributionID      string                `json:"distribution_id"`
    ObservedRevision    string                `json:"observed_revision,omitempty"`
    InstallationKey     string                `json:"installation_key"`
    ProbeID             string                `json:"probe_id"`
    Kind                string                `json:"kind"`
    BindingRoots        []BindingRootEvidence `json:"binding_roots"`
    EvidenceReference   string                `json:"evidence_reference"`
    Digest              string                `json:"digest"`
}

type Candidate struct {
    ProviderID             string                `json:"provider_id"`
    HostID                 string                `json:"host_id"`
    Surface                string                `json:"surface"`
    DistributionID         string                `json:"distribution_id"`
    InstallationKey        string                `json:"installation_key"`
    DiagnosticLocation     string                `json:"diagnostic_location,omitempty"`
    ObservedRevision       string                `json:"observed_revision,omitempty"`
    DistributionTreeDigest string                `json:"distribution_tree_digest,omitempty"`
    Provenance             ProvenanceDisposition `json:"provenance,omitempty"`
    BindingRoots           []BindingRootEvidence `json:"binding_roots"`
    EvidenceDigest         string                `json:"evidence_digest"`
    Evidence               []Evidence            `json:"evidence"`
}

type Report struct {
    hostID     string
    candidates []Candidate
    digest     string
}

func (report Report) HostID() string
func (report Report) Candidates(providerID string) []Candidate
func (report Report) Digest() string
```

`discovery` is the sole owner of `ProvenanceDisposition`; Registry uses `discovery.ProvenanceDisposition` and Host observations do not pre-judge provenance. `ObservedRevision` is populated only when Provider-specific evidence attests it. A content-equivalent flattened Binding leaves it empty; Registry still pins the trusted Descriptor revision in the verified record. A diagnostic candidate that has not established either disposition leaves `Provenance` empty and cannot enter Registry. `DiagnosticLocation` never enters a Bundle, Registry, public Host summary, or evidence handle.

Plan 01's `BindingRecord` supplies the two distinct roots:

```go
type BindingRecord struct {
    // existing identity and semantic fields...
    ContentRoot string `json:"content_root" toml:"content_root"`
    InstallRoot string `json:"install_root" toml:"install_root"`
}
```

`ContentRoot` is the clean relative path inside the immutable Distribution;
`InstallRoot` is the clean relative observation path below the exact Host
installation root selected for that Binding. Catalog/schema validation rejects
empty, absolute, backslash, `.`, `..`, and escaping forms for both. Native
repository layouts may use the same value for both; flattened Matt uses, for
example, `ContentRoot=skills/engineering/to-spec` and `InstallRoot=to-spec`.
Discovery never derives one from the other, a basename, skill name, reference
string, directory ancestry, or Provider brand.

Revision attestation is deliberately filesystem- and manifest-based, with no shell or `git` subprocess. A `distribution-attested` candidate requires an exact Provider-specific installation root selected by the Descriptor's discovery probe and one of these proofs beneath that root: a strict immutable-source manifest containing the exact Distribution ID, revision, and Distribution tree digest, or an exact immutable revision directory named by that probe whose complete tree digest is computed. The attested revision and `DistributionTreeDigest` must equal the Descriptor Distribution record, and every declared Binding's exact `InstallRoot` must independently produce the tree digest pinned for its Distribution `ContentRoot`. A shared ancestor, a generic skills directory, a manifest outside the exact Provider root, or one matching evidence file never attests the Provider or its other Bindings.

When no exact revision proof exists, discovery may classify the candidate as `content-equivalent` only if every Binding required from that Distribution is present at its exact `InstallRoot` and every complete tree digest matches the digest pinned for the corresponding Descriptor `ContentRoot`. Such a candidate leaves both `ObservedRevision` and `DistributionTreeDigest` empty; it never fabricates an observed revision or Distribution attestation. A shared installation root is only a search base and contributes no provenance. Missing or mismatched trees retain a diagnostic candidate with a stable error (`PROVIDER_BINDING_CONTENT_MISMATCH` or `PROVIDER_PROVENANCE_MISMATCH`) but no disposition; Registry rejects it. Derive opaque `evidence://` references from canonical evidence identity and digest, never from an absolute path.

Host v3 uses these records:

```go
const (
    HostManifestSchemaV3              = "oaw.host-manifest/v3"
    HostIntegrationSchemaV3           = "oaw.host-integration/v3"
    HostIntegrationSetSchemaV3        = "oaw.host-integration-set/v3"
    HostSessionSchemaV3               = "oaw.host-session/v3"
    BindingInventorySchemaV3          = "oaw.host-binding-inventory/v3"
    HostConformanceTranscriptSchemaV3 = "oaw.host-conformance-transcript/v3"
    HostConformanceReportSchemaV3     = "oaw.host-conformance-report/v3"
)

type FeatureID string

const (
    FeatureChildDelegation          FeatureID = "child-delegation"
    FeatureParallelChildDelegation  FeatureID = "parallel-child-delegation"
    FeatureNestedChildDelegation    FeatureID = "nested-child-delegation"
    FeatureNestedParallelDelegation FeatureID = "nested-parallel-child-delegation"
)

// Feature remains the existing Host control-feature vocabulary. FeatureID is
// a separate delegation-observation vocabulary; adding delegation evidence
// must never replace pause, cancellation, deduplication, inventory, receipt,
// or environment control features declared by a native Host Manifest.
type Feature string

const (
    FeaturePause                    Feature = "pause"
    FeatureInvocationDedup          Feature = "invocation-deduplication"
    FeatureCancellation             Feature = "cancellation"
    FeatureProviderBindingInventory Feature = "provider-binding-inventory"
    FeatureNormalizedReceipts       Feature = "normalized-receipts"
    FeatureEnvironmentReporting     Feature = "environment-reporting"
)

type Availability string
type ObservationSource string

const (
    AvailabilityAvailable    Availability = "available"
    AvailabilityUnavailable  Availability = "unavailable"
    AvailabilityUnknown      Availability = "unknown"
    AvailabilityConfigured   Availability = "host-configured"

    SourceNativeAPI       ObservationSource = "native-api"
    SourceLiveHostIndex   ObservationSource = "live-host-index"
    SourceLiveFilesystem  ObservationSource = "live-host-filesystem"
    SourceStaticConfig    ObservationSource = "static-configuration"
)

type FeatureObservation struct {
    Feature FeatureID    `json:"feature"`
    State   Availability `json:"state"`
    Source  ObservationSource `json:"source"`
    EvidenceReference string  `json:"evidence_reference"`
    Digest  string       `json:"digest"`
}

type HostActionContract struct {
    ID             string   `json:"id"`
    InputSchema    string   `json:"input_schema"`
    OutcomeSchema  string   `json:"outcome_schema"`
    MaximumEffects []string `json:"maximum_effects"`
    Resources      []string `json:"resources"`
}

type HostActionObservation struct {
    Action HostActionContract `json:"action"`
    State  Availability       `json:"state"`
    Source ObservationSource  `json:"source"`
    EvidenceReference string  `json:"evidence_reference"`
    Digest string             `json:"digest"`
}

type Manifest struct {
    SchemaVersion       string                 `json:"schema_version" toml:"schema_version"`
    ManifestVersion     string                 `json:"manifest_version" toml:"manifest_version"`
    HostID              string                 `json:"host_id" toml:"host_id"`
    ControlSurface      ControlSurface         `json:"control_surface" toml:"control_surface"`
    Protocols           []string               `json:"protocols" toml:"protocols"`
    BindingKinds        []catalog.BindingKind  `json:"binding_kinds" toml:"binding_kinds"`
    SupportedTopologies []execution.Topology   `json:"supported_topologies" toml:"supported_topologies"`
    Features            []Feature             `json:"features" toml:"features"`
    DelegationFeatures  []FeatureID           `json:"delegation_features" toml:"delegation_features"`
    HostActions         []HostActionContract   `json:"host_actions" toml:"host_actions"`
    Digest              string                 `json:"digest" toml:"digest"`
}

type SessionSnapshot struct {
    SchemaVersion           string                  `json:"schema_version"`
    HostID                  string                  `json:"host_id"`
    IntegrationID           string                  `json:"integration_id"`
    IntegrationVersion      string                  `json:"integration_version"`
    SessionID               string                  `json:"session_id"`
    ManifestDigest          string                  `json:"manifest_digest"`
    SupportedTopologies     []execution.Topology    `json:"supported_topologies"`
    ProviderInventoryDigest string                  `json:"provider_inventory_digest"`
    FeatureObservations     []FeatureObservation    `json:"feature_observations"`
    FeatureDigest           string                  `json:"feature_digest"`
    HostActionObservations  []HostActionObservation `json:"host_action_observations"`
    HostActionDigest        string                  `json:"host_action_digest"`
    EnvironmentReportDigest string                  `json:"environment_report_digest"`
    SandboxPolicyDigest     string                  `json:"sandbox_policy_digest"`
    ApprovalPolicyDigest    string                  `json:"approval_policy_digest"`
    Digest                  string                  `json:"digest"`
}

// Conformance v3 is the atomic Plan 02 bridge: it embeds Host facts v3 and
// the still-active Invocation Receipt v2. Plan 05 rejects this transcript and
// advances Receipt to v3 and Conformance Transcript/Report to v4 together.
type ConformanceTranscript struct {
    SchemaVersion      string              `json:"schema_version"`
    Session            SessionSnapshot     `json:"session"`
    Inventory          BindingInventory    `json:"inventory"`
    EnvironmentReports []EnvironmentReport `json:"environment_reports"`
    Receipts           []InvocationReceipt `json:"receipts"`
    Invocations        []InvocationRecord  `json:"invocations"`
    Digest             string              `json:"digest"`
}

type ConformanceReport struct {
    SchemaVersion              string      `json:"schema_version" toml:"schema_version"`
    ManifestDigest             string      `json:"manifest_digest" toml:"manifest_digest"`
    TranscriptDigest           string      `json:"transcript_digest" toml:"transcript_digest"`
    VerifiedFeatures           []Feature   `json:"verified_features" toml:"verified_features"`
    VerifiedDelegationFeatures []FeatureID `json:"verified_delegation_features" toml:"verified_delegation_features"`
    VerifiedHostActionIDs      []string    `json:"verified_host_action_ids" toml:"verified_host_action_ids"`
    Diagnostics                []string    `json:"diagnostics" toml:"diagnostics"`
    Digest                     string      `json:"digest" toml:"digest"`
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

type IntegrationSetRecord struct {
    SchemaVersion string              `json:"schema_version"`
    Integrations  []IntegrationRecord `json:"integrations"`
}
```

Only `native-api`, `live-host-index`, or `live-host-filesystem` evidence from the active Host session may use `available`. Static configuration is projected as `host-configured` or `unknown` and never satisfies a required feature/action. Every evidence reference is an opaque `evidence://` URI; Host authority records never expose an absolute private path. The v3 Conformance Report continues to prove the six original control features, and separately reports only delegation features and Host actions backed by live `available` observations. The Environment Report and Invocation Receipt remain on their unchanged v2 wire records in this plan; they are not reinterpreted as v3.

The initial `HostActions` set is exactly `workspace.prepare-or-confirm`, `verification.execute`, and `closeout.execute`, with the input/outcome/effect/resource contracts from the v4 design. A policy Manifest declares no protocols, Binding kinds, control features, delegation features, or Host actions. A native Manifest preserves its existing control feature requirements and may additionally declare delegation features and Host actions; the constructor rejects duplicates, unknown identifiers, undeclared Session observations, and contract drift. Declared delegation/action surfaces may be observed as `unknown` or `unavailable`; those states remain truthful facts and are not silently promoted to conformance success or compiler availability.

Binding Inventory v3 pins exact Descriptor identities:

```go
type BindingObservation struct {
    HostID              string                        `json:"host_id"`
    ProviderID          string                        `json:"provider_id"`
    InstallationKey     string                        `json:"installation_key"`
    DistributionID      string                        `json:"distribution_id"`
    BindingID           string                        `json:"binding_id"`
    Surface             string                        `json:"surface"`
    Kind                catalog.BindingKind           `json:"kind"`
    Reference           string                        `json:"reference"`
    Invocation          catalog.InvocationDisposition `json:"invocation"`
    BindingTreeDigest   string                        `json:"binding_tree_digest"`
    Topologies          []execution.Topology          `json:"topologies"`
    Source              ObservationSource             `json:"source"`
    EvidenceReference   string                        `json:"evidence_reference"`
    Digest              string                        `json:"digest"`
}

type BindingInventory struct {
    SchemaVersion string               `json:"schema_version"`
    HostID        string               `json:"host_id"`
    Observations  []BindingObservation `json:"observations"`
    Digest        string               `json:"digest"`
}

func NewBindingObservation(input BindingObservation) (BindingObservation, error)
func BuildBindingInventoryV3(hostID string, observations []BindingObservation) (BindingInventory, error)
func ValidateBindingInventory(record BindingInventory) (BindingInventory, error)
func NewManifest(record Manifest) (Manifest, error)
func NewSessionSnapshot(manifest Manifest, input SessionSnapshot) (SessionSnapshot, error)
func NewFeatureObservation(input FeatureObservation) (FeatureObservation, error)
func NewHostActionObservation(input HostActionObservation) (HostActionObservation, error)
func NewConformanceTranscript(input ConformanceTranscript) (ConformanceTranscript, error)
func NewConformanceReport(input ConformanceReport) (ConformanceReport, error)
func NewIntegration(input IntegrationRecord) (IntegrationRecord, error)
func DecodeIntegrationJSON(raw []byte) (IntegrationRecord, error)
func DecodeIntegrationTOML(raw []byte) (IntegrationRecord, error)
func DecodeIntegrationSetJSON(raw []byte) (IntegrationSetRecord, error)
func CloneManifest(input Manifest) Manifest
func CloneSessionSnapshot(input SessionSnapshot) SessionSnapshot
func CloneBindingInventory(input BindingInventory) BindingInventory
func CloneConformanceTranscript(input ConformanceTranscript) ConformanceTranscript
func CloneConformanceReport(input ConformanceReport) ConformanceReport
func CloneIntegration(input IntegrationRecord) IntegrationRecord
```

Every constructor validates the caller-supplied schema before clearing and recomputing its digest, so old records cannot be normalized into v3. `NewManifest` hashes the canonical record with `Digest` empty; `ContentDigest` returns that normalized digest, and Integration/Session must pin the same value in `ManifestDigest`. Record and evidence digests remain bare 64-character lowercase hex from `canonicaljson.Digest`; only content/tree digests use the `sha256:` prefix. Clone helpers recursively copy every nested slice, action contract, observation, transcript record, and optional Conformance pointer. Replace `NewBindingInventory` with `BuildBindingInventoryV3`; do not retain a compatibility wrapper or dual reader.

Registry v4 retains Bindings, not one selected Binding per Capability:

```go
const (
    providerInstanceSchemaV4  = "oaw.provider-instance/v4"
    resolutionReportSchemaV4  = "oaw.provider-resolution-report/v4"
    effectiveRegistrySchemaV4 = "oaw.effective-registry/v4"
)

type VerifiedBinding struct {
    BindingID              string                        `json:"binding_id"`
    DistributionID         string                        `json:"distribution_id"`
    DistributionRevision   string                        `json:"distribution_revision"`
    DistributionTreeDigest string                        `json:"distribution_tree_digest"`
    Surface                string                        `json:"surface"`
    Kind                   catalog.BindingKind           `json:"kind"`
    Reference              string                        `json:"reference"`
    Invocation             catalog.InvocationDisposition `json:"invocation"`
    BindingTreeDigest      string                        `json:"binding_tree_digest"`
    SupportedTopologies    []execution.Topology          `json:"supported_topologies"`
    Delegation             catalog.DelegationRequirements `json:"delegation"`
    Provenance             discovery.ProvenanceDisposition `json:"provenance"`
    BindingEvidenceDigest  string                        `json:"binding_evidence_digest"`
}

type VerifiedCapability struct {
    ID                 string   `json:"id"`
    BindingIDs         []string `json:"binding_ids"`
    PreferredBindingID string   `json:"preferred_binding_id,omitempty"`
}

type ProviderInstance struct {
    ProviderID             string               `json:"provider_id"`
    HostID                 string               `json:"host_id"`
    DescriptorDigest       string               `json:"descriptor_digest"`
    DistributionID         string               `json:"distribution_id"`
    DistributionRevision   string               `json:"distribution_revision"`
    DistributionTreeDigest string               `json:"distribution_tree_digest"`
    InstallationKey        string               `json:"installation_key"`
    ConfigurationDigest    string               `json:"configuration_digest"`
    BindingInventoryDigest string               `json:"binding_inventory_digest"`
    EvidenceDigest         string               `json:"evidence_digest"`
    Bindings               []VerifiedBinding    `json:"bindings"`
    Capabilities           []VerifiedCapability `json:"capabilities"`
    Digest                 string               `json:"digest"`
}

type ProviderState string

const (
    ProviderNotFound          ProviderState = "not-found"
    ProviderCandidate         ProviderState = "candidate"
    ProviderVerified          ProviderState = "verified"
    ProviderAmbiguous         ProviderState = "ambiguous"
    ProviderIncompatible      ProviderState = "incompatible"
    ProviderBindingUnavailable ProviderState = "binding-unavailable"
    ProviderDisabled          ProviderState = "disabled"
    ProviderUntrusted         ProviderState = "untrusted"
)

type ProviderResolution struct {
    ProviderID string                `json:"provider_id"`
    State      ProviderState         `json:"state"`
    Reason     string                `json:"reason"`
    Instance   *ProviderInstance     `json:"instance,omitempty"`
    Candidates []discovery.Candidate `json:"candidates"`
}

type ResolutionReport struct {
    hostID      string
    resolutions []ProviderResolution
    digest      string
}

func (report ResolutionReport) HostID() string
func (report ResolutionReport) Resolutions() []ProviderResolution
func (report ResolutionReport) Resolution(providerID string) (ProviderResolution, bool)
func (report ResolutionReport) Digest() string

// Registry remains the package's immutable concrete value. Do not replace it
// with an interface bearing the same name; profile.EffectiveRegistry is the
// consumer-side interface.
type Registry struct {
    hostID        string
    providers     []ProviderInstance
    providerIndex map[string]int
    bindingIndex  map[string]map[string]int
    digest        string
}

func (registry Registry) HostID() string
func (registry Registry) Providers() []ProviderInstance
func (registry Registry) Provider(id string) (ProviderInstance, bool)
func (registry Registry) Binding(providerID, bindingID string) (VerifiedBinding, bool)
func (registry Registry) Bindings(providerID string) []VerifiedBinding
func (registry Registry) Capability(providerID, capabilityID string) (VerifiedCapability, bool)
func (registry Registry) Digest() string
```

All nested slices are defensively cloned. `VerifiedCapability.BindingIDs` is an identity-sorted set; its order never selects authority. `PreferredBindingID` is present only after one explicit compatible user preference resolves to exactly one retained Binding. With no preference it is empty; zero or multiple matches make the Provider resolution `incompatible` with `BINDING_PREFERENCE_INCOMPATIBLE`. Plan 02 changes only the forward `profile.EffectiveRegistry` contract; Plan 03 owns the compiler-wide Profile migration and its phase gate.

The consumer-side interface is the only interface change owned here and lives
in `internal/profile/records.go`; `registry.Registry` remains the public
concrete immutable producer:

```go
type EffectiveRegistry interface {
    HostID() string
    Providers() []registry.ProviderInstance
    Provider(id string) (registry.ProviderInstance, bool)
    Binding(providerID, bindingID string) (registry.VerifiedBinding, bool)
    Bindings(providerID string) []registry.VerifiedBinding
    Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool)
    Digest() string
}
```

## Task 1: Implement complete Binding-tree integrity

**Files:**
- Create: `internal/integrity/tree.go`
- Create: `internal/integrity/tree_test.go`

- [ ] **Step 1: Write RED filesystem tests**

Create fixtures with an empty tree, nested directories, two regular files, an executable file, a renamed file, a changed file, a FIFO, a replaced root, a symlink to an in-tree target, and a symlink to an outside target. Assert directories traverse successfully, exact prefixed digest form, deterministic order and defensive copies, different digest for content/mode/path changes, and `BINDING_TREE_INVALID` for empty/FIFO/replaced/symlink cases.

- [ ] **Step 2: Run RED**

Run: `rtk go test ./internal/integrity`

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement canonical enumeration and hashing**

Use `filepath.Abs`, `filepath.Clean`, `filepath.Rel`, `filepath.WalkDir`, and `os.Lstat`. Permit directories only for traversal; reject every non-directory entry that is a symlink or not a regular file. Pin the physical root identity before walking. Open each file with `os.Open`, compare pre-read and post-read `Stat` identity/size/mode/mtime, hash through `io.Copy`, sort entries by `Path`, and hash this exact canonical record through the prefixed digest helper:

```go
struct {
    SchemaVersion string      `json:"schema_version"`
    Entries       []TreeEntry `json:"entries"`
}{"oaw.binding-tree/v1", entries}
```

- [ ] **Step 4: Run GREEN and race checks**

Run: `rtk gofmt -w internal/integrity/tree.go internal/integrity/tree_test.go`
Run: `rtk go test -race ./internal/integrity`

Expected: PASS.

- [ ] **Step 5: Commit integrity support**

```bash
rtk git add internal/integrity/tree.go internal/integrity/tree_test.go
rtk git commit -m "feat: hash complete provider binding trees"
```

## Task 2: Make discovery Distribution-scoped

**Files:**
- Modify: `internal/discovery/records.go`
- Create: `internal/discovery/attest.go`
- Modify: `internal/discovery/discover.go`
- Modify: `internal/discovery/discovery_test.go`

- [ ] **Step 1: Write failing provenance tests**

Add cases proving:

- a Descriptor-pinned repository revision plus matching tree yields `distribution-attested`;
- a flattened exact Binding tree yields `content-equivalent`;
- a shared `~/.agents/skills` ancestor never establishes Matt provenance;
- same Binding name with different bytes yields `PROVIDER_BINDING_CONTENT_MISMATCH`;
- a repository revision or Distribution tree mismatch yields `PROVIDER_PROVENANCE_MISMATCH`;
- a Host surface mismatch and same IDs under a different Provider remain distinct;
- `BindingRoots` are sorted, deeply copied, and change the evidence/report digest;
- a content-equivalent candidate has no fabricated observed revision or Distribution tree digest.

- [ ] **Step 2: Run RED**

Run: `rtk go test ./internal/discovery -run 'Distribution|Provenance|ContentEquivalent|ContentMismatch|RevisionMismatch|Surface|SharedAncestor|Defensive'`

Expected: FAIL because v2 discovery evidence has no immutable Distribution fields.

- [ ] **Step 3: Advance discovery records to v3**

Replace discovery records with the locked v3 `Evidence`, `BindingRootEvidence`, `Candidate`, `ProvenanceDisposition`, and `Report`. Implement the exact-root manifest/revision-directory attestation algorithm above in `attest.go`; it reads the selected filesystem root directly and never invokes a shell or `git`. For every Descriptor Binding belonging to the candidate Distribution, resolve its clean `InstallRoot` under the exact Host installation root, call `integrity.DigestTree` on that observed tree, and compare the full result's `RootDigest` to the Descriptor Binding digest for its Distribution `ContentRoot` before setting either provenance disposition. The immutable source tree is resolved at `ContentRoot` only when checking the pinned Distribution/audit record; it is never inferred as the Host install path. Remove any rule that treats an ancestor directory or a single evidence file as proof for every Binding. Candidate detection may remain diagnostic when exact content is unavailable, but such a candidate cannot be verified later. Keep physical locations only in `DiagnosticLocation`; do not project them into Registry or Host authority.

- [ ] **Step 4: Run GREEN**

Run: `rtk gofmt -w internal/discovery/records.go internal/discovery/attest.go internal/discovery/discover.go internal/discovery/discovery_test.go`
Run: `rtk go test ./internal/discovery`

Expected: PASS.

- [ ] **Step 5: Commit discovery provenance**

```bash
rtk git add internal/discovery/records.go internal/discovery/attest.go internal/discovery/discover.go internal/discovery/discovery_test.go
rtk git commit -m "feat: bind discovery to provider distributions"
```

## Task 3: Replace Host Manifest, Session, and Binding Inventory with v3

**Files:**
- Modify: `internal/host/records.go`
- Modify: `internal/host/bindings.go`
- Create: `internal/host/actions.go`
- Modify: `internal/host/validate.go`
- Modify: `internal/host/decode.go`
- Modify: `internal/host/session.go`
- Modify: `internal/host/builtin.go`
- Modify: `internal/host/conformance.go`
- Modify: `internal/host/environment.go`
- Modify: `internal/host/records_test.go`
- Modify: `internal/host/bindings_test.go`
- Modify: `internal/host/validation_test.go`
- Create: `internal/host/actions_test.go`
- Modify: `internal/host/builtin_test.go`
- Modify: `internal/host/conformance_test.go`
- Modify: `internal/host/conformance_fuzz_test.go`
- Create: `internal/assets/schemas/v3/host-manifest.schema.json`
- Create: `internal/assets/schemas/v3/host-binding-inventory.schema.json`
- Create: `internal/assets/schemas/v3/host-session.schema.json`
- Create: `internal/assets/schemas/v3/host-integration.schema.json`
- Create: `internal/assets/schemas/v3/host-integration-set.schema.json`
- Create: `internal/assets/schemas/v3/host-conformance-transcript.schema.json`
- Create: `internal/assets/schemas/v3/host-conformance-report.schema.json`
- Modify: `internal/assets/host-integrations.json`
- Create: `internal/assets/conformance/codex-host-v3.json`
- Delete: `internal/assets/conformance/codex-host-v1.json`
- Modify: `internal/assets/generate/codex_host.go`
- Modify: `internal/assets/generate/codex_host_test.go`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`
- Modify: `internal/assets/embed_test.go`

- [ ] **Step 1: Write RED Host v3 tests**

Assert all five Binding kinds are accepted, `hook` is not a Binding kind, Provider/surface/source/evidence references are exact, and old v2 Manifest/Inventory/Session/Integration/Integration Set/Conformance records return `HOST_SCHEMA_UNSUPPORTED`. Prove the six original control features remain accepted and conformance-verifiable after adding the separate four-value delegation `FeatureID` vocabulary. Assert duplicate feature/action observations fail, static feature/action evidence cannot satisfy `available`, an observation not declared by the Manifest fails, and changing a Binding tree, feature, or action digest changes the Inventory and Session digest. Add active Integration/Conformance/Environment tests proving no production validator still requires Manifest/Session/Inventory v2, plus clone tests that mutate every nested slice and optional Conformance pointer. In `internal/assets/embed_test.go`, also replace Plan 01's deferred Provider v3/Recipe v2 metadata expectations with Provider v4/Recipe v3 before asserting the new Host resources.

- [ ] **Step 2: Run RED**

Run: `rtk go test ./internal/host ./internal/schema ./internal/assets ./internal/assets/generate -run 'HostV3|BindingInventoryV3|ControlFeature|FeatureObservation|HostAction|IntegrationV3|ConformanceV3|Defensive'`

Expected: FAIL because only skill/agent/tool and Host v2 records exist.

- [ ] **Step 3: Implement strict Host v3 constructors**

Add the locked constructors and deep-clone functions, retaining the existing `NewSessionSnapshot(manifest, input)` relationship. Replace the old `NewBindingInventory` surface with `BuildBindingInventoryV3` and update every Host caller. Each validator checks the supplied schema before normalization, so a v2 record cannot be silently rebuilt as v3. Keep `Feature` and all six control constants unchanged; add `FeatureID` only for delegation observations. Validate Session observations against the exact normalized Manifest, and sort set-like collections by stable identity. Preserve no raw credential, Hook command, absolute private path, or model transcript in any record. Reject absolute `Reference`/`EvidenceReference` values in Host authority; only discovery diagnostics may retain a physical location.

- [ ] **Step 4: Advance Integration and Conformance atomically**

Define and enforce `HostIntegrationSchemaV3`, `HostIntegrationSetSchemaV3`, `HostConformanceTranscriptSchemaV3`, and `HostConformanceReportSchemaV3` together with their complete Go records. Conformance v3 embeds Session/Inventory v3 and explicitly references the unchanged Environment Report/Invocation Receipt v2 schemas; Plan 05 will reject Transcript v3 when it advances Receipt to v3 and Conformance to v4. Update `NewIntegration`, strict JSON/TOML decoders, built-in loading, conformance construction/reporting, stored-session validation, and Environment validation at the same time. Preserve control-feature verification and add distinct live delegation/action verification; never infer either from static Manifest configuration.

- [ ] **Step 5: Activate schemas and regenerate built-in assets**

Register only the v3 Manifest/Inventory/Session/Integration/Integration Set/Conformance schemas and remove their v2 predecessors from active registration. Update every schema `$ref` together. Retain inactive v2 schema files only as historical contract artifacts; no decoder or registry entry may expose them as authority. Environment Report v2 remains because its wire shape is unchanged, but validation pins it to Session v3. Invocation Receipt v2 remains active only until Plan 05.

Generate canonical `codex-host-v3.json` and Integration Set v3, update the generator tests and embed test, then delete the superseded generated `codex-host-v1.json` transcript. In this phase, remove the generator and its test's imports of `internal/codexbridge`: that package still owns Bridge v1 and is intentionally cut over only in Plan 06. Use private generator constants for the existing Integration ID/version and construct a conservative host-only Manifest/Session/Inventory/Transcript fixture with no fabricated delegation/action availability; Plan 06 replaces those static fixture facts with live Bridge v2 observations.

Keep `audits/codex-host-v1.json` byte-for-byte as historical evidence, but do not reuse it as the active Integration v3 audit after its referenced v1 transcript is deleted. Build the active Integration's `AuditEvidence` from the v3 fixture and the scoped Host v3 security/conformance checks, and embed that canonical record inside `host-integrations.json`. `embed_test.go` may assert the historical v1 audit record is canonical, but must not dereference it as current authority; it separately verifies every active v3 audit reference and digest. Run the generator twice in a temporary copy and assert the second run is byte-identical.

- [ ] **Step 6: Run GREEN**

Run: `rtk gofmt -w internal/host/records.go internal/host/bindings.go internal/host/actions.go internal/host/validate.go internal/host/decode.go internal/host/session.go internal/host/builtin.go internal/host/conformance.go internal/host/environment.go internal/host/records_test.go internal/host/bindings_test.go internal/host/validation_test.go internal/host/actions_test.go internal/host/builtin_test.go internal/host/conformance_test.go internal/host/conformance_fuzz_test.go internal/schema/registry.go internal/schema/registry_test.go internal/assets/embed_test.go internal/assets/generate/codex_host.go internal/assets/generate/codex_host_test.go`
Run: `rtk go test ./internal/host ./internal/schema ./internal/assets ./internal/assets/generate`

Expected: PASS.

- [ ] **Step 7: Commit Host evidence v3**

```bash
rtk git add internal/host/records.go internal/host/bindings.go internal/host/actions.go internal/host/validate.go internal/host/decode.go internal/host/session.go internal/host/builtin.go internal/host/conformance.go internal/host/environment.go internal/host/records_test.go internal/host/bindings_test.go internal/host/validation_test.go internal/host/actions_test.go internal/host/builtin_test.go internal/host/conformance_test.go internal/host/conformance_fuzz_test.go internal/schema/registry.go internal/schema/registry_test.go internal/assets/embed_test.go internal/assets/generate/codex_host.go internal/assets/generate/codex_host_test.go internal/assets/host-integrations.json internal/assets/conformance/codex-host-v1.json internal/assets/conformance/codex-host-v3.json internal/assets/schemas/v3/host-manifest.schema.json internal/assets/schemas/v3/host-binding-inventory.schema.json internal/assets/schemas/v3/host-session.schema.json internal/assets/schemas/v3/host-integration.schema.json internal/assets/schemas/v3/host-integration-set.schema.json internal/assets/schemas/v3/host-conformance-transcript.schema.json internal/assets/schemas/v3/host-conformance-report.schema.json
rtk git commit -m "feat: define host binding evidence v3"
```

## Task 4: Migrate Config consumers to catalog v4 Bindings

**Files:**
- Modify: `internal/config/decode.go`
- Modify: `internal/config/snapshot.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/config/snapshot_test.go`
- Modify: `internal/assets/schemas/v3/user-config.schema.json`

Plan 01 deletes `catalog.HostBinding` and `CapabilitySelector`; this task is
the owning consumer cutover. It is intentionally adjacent to Host v3 because
Config imports Host records and Registry imports Config settings. Do not add a
compatibility alias in Catalog or Config.

- [ ] **Step 1: Write RED Config consumer tests**

Migrate every existing Host Integration fixture in `snapshot_test.go` to
Manifest/Integration v3; a focused package test still compiles all `_test.go`
files, so no v2 constructor or constant may remain. Add focused cases for
Provider Descriptor v4/Recipe v3 decoding through the
existing User Config v3 envelope, exact five-kind Binding preferences
(`skill`, `agent`, `role`, `instruction`, `tool`), rejection of `hook`, old
authority schema references, and a preference that resolves by one declared
Binding ID while zero or multiple matches return
`BINDING_PREFERENCE_UNDECLARED`. Preserve snapshot defensive copies and deny,
pin, installation, and project-trust behavior.

- [ ] **Step 2: Run RED**

Run: `rtk go test ./internal/config -run '^(TestDecodeProviderV4TOMLUsesCatalogContract|TestDecodeRecipeV3TOMLUsesCatalogContract|TestReferencedAuthorityHardCutRejectsV3AndV2|TestV4BindingPreferenceResolvesByBindingID|TestUserConfigV3AcceptsAllV4BindingKinds)$'`

Expected: FAIL because Config still traverses deleted `HostBinding`/legacy
selector fields and validates the old Binding-kind enum.

- [ ] **Step 3: Replace direct consumer lookups**

Keep `UserConfigSchemaV3`, `ContentReference`, provider pins, deny rules,
installation hints, and project trust unchanged. In `decode.go`, strict-decode
Provider/Recipe through the catalog v4/v3 decoders before schema validation so
retired authorities fail with their exact unsupported-schema codes; do not
normalize missing collections or add fallback readers. In `snapshot.go`, resolve
preferences by the closed relation

```text
preference.ProviderID -> ProviderDescriptorRecord
preference.CapabilityID -> CapabilityRecord
preference (HostID, Kind, Reference) -> exactly one BindingID in BindingRefs
```

Reject zero or multiple matches with `BINDING_PREFERENCE_UNDECLARED`; never
match by capability name, directory ancestry, Provider brand, or lexical first
Binding. Keep all existing immutable snapshot behavior.

- [ ] **Step 4: Update the User Config schema and tests**

Change only the Binding preference `kind` enum in
`internal/assets/schemas/v3/user-config.schema.json` to the five active
catalog kinds. Keep the envelope version and all unrelated fields unchanged.

- [ ] **Step 5: Run GREEN and commit Config consumers**

Run: `rtk gofmt -w internal/config/decode.go internal/config/snapshot.go internal/config/config_test.go internal/config/snapshot_test.go`
Run: `rtk go test ./internal/config -run '^(TestDecodeProviderV4TOMLUsesCatalogContract|TestDecodeRecipeV3TOMLUsesCatalogContract|TestReferencedAuthorityHardCutRejectsV3AndV2|TestV4BindingPreferenceResolvesByBindingID|TestUserConfigV3AcceptsAllV4BindingKinds)$'`

Expected: PASS. Do not run Config `Load` tests here if they still load the
pre-Plan-04 built-in Provider/Recipe assets; that asset migration remains
Plan 04 ownership.

```bash
rtk git add internal/config/decode.go internal/config/snapshot.go internal/config/config_test.go internal/config/snapshot_test.go internal/assets/schemas/v3/user-config.schema.json
rtk git commit -m "feat: migrate config to provider binding IDs"
```

## Task 5: Resolve exact multi-Binding Provider Instances

**Files:**
- Modify: `internal/registry/records.go`
- Modify: `internal/registry/resolve.go`
- Modify: `internal/registry/registry.go`
- Modify: `internal/registry/registry_test.go`
- Create: `internal/registry/resolve_internal_test.go`
- Modify: `internal/profile/records.go`

- [ ] **Step 1: Write RED Registry v4 tests**

Create `resolve_internal_test.go` in package `registry`, backed by a synthetic
package-private resolution source rather than `config.Load`. Construct one
Capability with two verified Binding alternatives and assert both remain
addressable while `PreferredBindingID` stays empty without a preference. Add
failures for mismatched Provider, Host, surface, revision, Distribution tree
digest, Binding tree digest, kind, reference, invocation disposition, duplicate
Host observations, and provenance. Assert zero-match and multi-match
preferences return `BINDING_PREFERENCE_INCOMPATIBLE` instead of selecting or
reordering the lexical first Binding. Migrate all existing fixtures in
`registry_test.go` from deleted catalog/Host records to the v4 records, states,
and expectations so the package compiles and is ready for Plan 04's new assets,
but do not use those pre-Plan-04 asset-backed tests as this phase's GREEN gate.

- [ ] **Step 2: Run RED**

Run: `rtk go test ./internal/registry -run '^(TestRegistryV4RetainsBindingAlternatives|TestRegistryV4RejectsEvidenceMismatches|TestRegistryV4AppliesExactPreference|TestRegistryV4DefensiveCopies)$'`

Expected: FAIL because Registry v3 stores one Binding per Capability.

- [ ] **Step 3: Implement v4 resolution order**

For each Provider:

1. apply deny, trust, pin, and candidate ambiguity checks;
2. require exact Provider/Host/surface plus Distribution ID/revision/tree evidence;
3. intersect each Descriptor Binding with one exact Host observation;
4. reject content/kind/reference/invocation differences;
5. retain all matching Bindings and build Capability `BindingIDs` from declared references;
6. apply a user preference's Provider/Capability/Host/kind/reference identity only when it identifies exactly one retained compatible Binding ID;
7. record provenance only after intersecting Descriptor, discovery, and live Host evidence;
8. compute Provider Instance, Resolution Report, and Registry v4 digests using the locked v4 schema constants.

The unchanged User Config v3 pin compares Provider/Host/Installation/Evidence
identity first. Its optional local `Location` compares only to
`Candidate.DiagnosticLocation`, and optional `Version` compares only to a real
`ObservedRevision`; neither value is copied into Provider Instance, Registry,
Bundle, or public Host evidence. A revision pin cannot match a
`content-equivalent` candidate with an empty observed revision. Do not select
by lexical order, Provider brand, previous Bundle, or recommendation.

Keep the public `Resolve(config.Snapshot, ...)` signature, but make it a thin
adapter over one package-private `resolutionSource` interface exposing only
`Catalog`, `ProviderSettings`, `RequiredProviders`, `RecommendedProviders`, and
`UntrustedProviderIDs`. `config.Snapshot` implements that interface unchanged;
the internal tests use a synthetic implementation. This is a testability seam,
not a second authority path: both adapters execute the same resolver and digest
logic.

- [ ] **Step 4: Preserve immutable lookup behavior**

`Providers`, `Bindings`, `Binding`, `Capability`, Resolution accessors, Candidate evidence, and every nested slice return defensive copies. The Registry digest includes every alternative, provenance disposition, preference field, and evidence digest. Keep `Registry` as the package's concrete immutable value; replace `profile.EffectiveRegistry` with the exact seven-method interface locked above without renaming or replacing `registry.Registry`. Do not otherwise migrate Profile compilation here; Plan 03 owns that work.

- [ ] **Step 5: Run GREEN**

Run: `rtk gofmt -w internal/registry/records.go internal/registry/resolve.go internal/registry/registry.go internal/registry/registry_test.go internal/registry/resolve_internal_test.go internal/profile/records.go`
Run: `rtk go test ./internal/registry -run '^(TestRegistryV4RetainsBindingAlternatives|TestRegistryV4RejectsEvidenceMismatches|TestRegistryV4AppliesExactPreference|TestRegistryV4DefensiveCopies)$'`

Expected: PASS.

- [ ] **Step 6: Commit Registry v4**

```bash
rtk git add internal/registry/records.go internal/registry/resolve.go internal/registry/registry.go internal/registry/registry_test.go internal/registry/resolve_internal_test.go internal/profile/records.go
rtk git commit -m "feat: retain verified provider binding alternatives"
```

## Phase Verification

- [ ] Run `rtk go test -race ./internal/integrity ./internal/discovery ./internal/host ./internal/schema ./internal/assets ./internal/assets/generate -count=1`.
- [ ] Run `rtk go test -race ./internal/config -run '^(TestDecodeProviderV4TOMLUsesCatalogContract|TestDecodeRecipeV3TOMLUsesCatalogContract|TestReferencedAuthorityHardCutRejectsV3AndV2|TestV4BindingPreferenceResolvesByBindingID|TestUserConfigV3AcceptsAllV4BindingKinds)$' -count=1`.
- [ ] Run `rtk go test -race ./internal/registry -run '^(TestRegistryV4RetainsBindingAlternatives|TestRegistryV4RejectsEvidenceMismatches|TestRegistryV4AppliesExactPreference|TestRegistryV4DefensiveCopies)$' -count=1`.
- [ ] Run `rtk go vet ./internal/integrity ./internal/discovery ./internal/host ./internal/registry ./internal/config ./internal/schema ./internal/assets ./internal/assets/generate`.
- [ ] Run `rtk rg -n 'discovery-(evidence|report)/v2|host-(manifest|session|binding-inventory|integration|integration-set|conformance-transcript|conformance-report)/v2|provider-instance/v3|provider-resolution-report/v3|effective-registry/v3|HostManifestSchemaV2|HostSessionSchemaV2|BindingInventorySchemaV2|HostIntegrationSchemaV2|HostIntegrationSetSchemaV2|HostConformanceTranscriptSchemaV2|HostConformanceReportSchemaV2|NewBindingInventory\(' internal/discovery internal/host internal/registry internal/profile/records.go internal/config internal/schema/registry.go internal/assets/host-integrations.json internal/assets/conformance/codex-host-v3.json internal/assets/schemas/v3`.
- [ ] Run `rtk rg -n 'HostBinding|CapabilitySelector|capability\.HostBindings|capability\.Selector' internal/config` and require no matches outside explicit negative test names or comments.
- [ ] Confirm any matches are restricted to explicit negative tests. The intentionally inactive files under `internal/assets/schemas/v2/` and old audit evidence are outside this active-surface search; no v2 registry entry, constructor, decoder, built-in asset, or compatibility wrapper remains.
- [ ] Run `rtk git diff --check` and inspect `rtk git status --short` without touching unrelated files.

Expected: exact Binding evidence is content-bound, alternatives are preserved, and static Host configuration cannot become live authority.
