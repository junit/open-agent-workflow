# OAW Runtime vNext Ticket 02 Configuration, Trust and Provider Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build immutable effective configuration snapshots, exact project-trust evaluation, deterministic declarative Provider discovery, verified Provider Instances, and an Effective Registry without loading or executing Provider code.

**Architecture:** Strict TOML documents decode into closed Go records, normalize into compact Canonical JSON, pass embedded JSON Schema and domain validation, and merge as immutable whole records over the Ticket 01 built-in Catalog. User-owned trust records admit exact project content by physical root and canonical digests; discovery evaluates only the built-in safe probe vocabulary under an explicit user-home root. A registry resolver combines one configuration snapshot, discovery evidence, and an optional authoritative Host binding inventory to emit all Provider states and admit only pinned, verified instances.

**Tech Stack:** Go 1.26, `encoding/json`, `crypto/sha256`, `io/fs`, `os`, `path/filepath`, `github.com/BurntSushi/toml v1.6.0`, the existing Draft 2020-12 JSON Schema registry, table-driven Go tests, race tests, and the existing Bash regression harness.

---

## Scope Boundary

Ticket 02 owns configuration, trust, discovery evidence, Provider verification,
and Effective Registry construction. It does not:

- execute a Provider, Skill, Agent, Hook, command, script, or template;
- read remote content or interpolate environment variables inside records;
- compile Profile Recipes into Execution Graphs;
- classify an Engineering Request or issue a Capability Grant;
- select a Profile or alter an active Lifecycle Bundle;
- infer Host capabilities from process environment or installed instruction files;
- replace the Bash installer, add a daemon, or add Runtime State persistence;
- expose a new public CLI surface before Host registration and redaction rules are
  available in their approved tickets.

The package APIs accept explicit user configuration, project, user-home, and
Host binding inputs. Later CLI and Host Adapter tickets resolve those inputs.

## Locked Ticket 02 Contracts

### Source layout

Callers provide an optional user configuration root and optional project root:

```text
<user-config-root>/config.toml
<user-config-root>/<user referenced descriptor or recipe>

<physical-project-root>/.oaw/config.toml
<physical-project-root>/.oaw/<project referenced descriptor or recipe>
```

References are clean relative slash paths. Absolute paths, backslashes,
control characters, `.` or `..` path components, and symlink escapes are
rejected. The core does not read `HOME`, `XDG_CONFIG_HOME`, or the current
working directory.

### User configuration

The exact v1 shape is:

```toml
schema_version = "oaw.user-config/v1"
denied_providers = ["acme/disabled-suite"]

[[provider_descriptors]]
id = "acme/engineering-suite"
path = "providers/acme-engineering-suite.toml"
replace = false

[[profile_recipes]]
id = "acme/reliable-delivery"
path = "profiles/acme-reliable-delivery.toml"
replace = false

[[provider_pins]]
id = "oaw/superpowers"
location = "/Users/example/.codex/plugins/superpowers"
version = "1.2.3"

[[binding_preferences]]
provider_id = "oaw/superpowers"
capability_id = "implementation"
host = "codex"
kind = "skill"
reference = "superpowers:subagent-driven-development"

[[project_trust]]
root = "/physical/project/root"
config_digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
descriptor_digests = []
recipe_digests = []
```

All arrays normalize to non-nil sorted slices. `denied_providers` always wins.
Pins narrow candidate selection; binding preferences select only a binding that
the descriptor declares and the authoritative Host inventory reports.

### Project configuration

The exact v1 shape is deliberately authority-free:

```toml
schema_version = "oaw.project-config/v1"
required_providers = ["acme/engineering-suite"]
recommended_providers = ["oaw/superpowers"]

[[provider_descriptors]]
id = "acme/engineering-suite"
path = "providers/acme-engineering-suite.toml"
replace = false

[[profile_recipes]]
id = "acme/reliable-delivery"
path = "profiles/acme-reliable-delivery.toml"
replace = false

[[capability_limits]]
provider_id = "acme/engineering-suite"
capability_ids = ["review", "verification"]
```

Project configuration has no trust, enable, effect, resource, delegation,
binding, location, command, or authority field. Capability limits intersect
the descriptor; they never add a Capability. Required and recommended IDs are
recorded as policy inputs but do not enable or select Providers.

### Trust and replacement

- User config is trusted because its root is supplied by the caller as a
  user-owned input.
- Project content participates only when one user `project_trust` record matches
  the physical root, canonical project-config digest, sorted Descriptor digests,
  and sorted Recipe digests byte-for-byte.
- Any formatting-only TOML change that produces identical typed content keeps
  the canonical digest. Any semantic content change changes a digest.
- `oaw/*` Provider and Recipe namespaces remain reserved and cannot be supplied
  by user or project files.
- A duplicate third-party ID fails unless the later source has `replace=true`.
  Replacing an `oaw/*` built-in record is always rejected.
- An untrusted project is represented in the snapshot and resolution report but
  its Descriptor and Recipe records do not enter the effective Catalog.

### Canonical JSON and digest

Ticket 02 Canonical JSON is compact UTF-8 JSON emitted from closed typed records
whose fields have a fixed declaration order and whose collections are sorted.
Maps, floating-point values, functions, channels, complex values, and raw
interfaces are rejected by `canonicaljson.Marshal`. SHA-256 digests are 64
lowercase hexadecimal characters without a prefix.

### Discovery and candidates

- The only roots and probe kinds are those already admitted by Ticket 01:
  `user-home`, `path-exists`, and `one-level-version-path-exists`.
- Discovery receives an explicit user-home path, resolves it physically, uses
  Go filesystem APIs only, follows a symlink only when its physical destination
  remains under that root, and reads only regular files no larger than 4 MiB.
- Direct `path-exists` evidence for one Provider under one user-home root forms
  one direct candidate. Its version is `content-<evidence digest>`.
- Each matching immediate version directory from a
  `one-level-version-path-exists` probe forms a separate candidate whose version
  is the directory name and whose location is that physical version directory.
- Evidence is sorted by Provider ID, candidate key, probe ID, and physical path.
  Every file records its SHA-256 content digest. No match is `not-found`; one
  unresolved candidate remains `candidate`; multiple candidates without an
  exact pin are `ambiguous`.

### Binding inventory and verification

The Host supplies an optional authoritative inventory:

```go
type BindingInventory struct {
    Host     string
    Bindings []catalog.HostBinding
}
```

`nil` means binding availability is not authoritative and a single discovered
candidate remains `candidate`. A non-nil empty or incompatible inventory yields
`binding-unavailable`. For each admitted Capability, resolution chooses one
descriptor-declared binding present in the inventory; an exact user preference
wins, otherwise bytewise `(host, kind, reference)` order wins. Project
capability limits are applied before binding resolution.

A verified Provider Instance pins:

```go
type ProviderInstance struct {
    ProviderID         string
    DescriptorDigest   string
    Location           string
    Version            string
    ConfigurationDigest string
    BindingDigest      string
    EvidenceDigest     string
    Capabilities       []VerifiedCapability
    Digest             string
}
```

Only `verified` instances enter the Effective Registry. A partially installed
Provider can expose a verified subset of Capabilities; full-family eligibility
is evaluated later by the Recipe compiler against that verified subset.

### Provider states

Every resolution uses exactly one of:

```text
not-found
candidate
verified
ambiguous
incompatible
binding-unavailable
disabled
untrusted
```

User deny maps to `disabled`. An exact pin matching no candidate maps to
`incompatible`. A project-only record without exact user trust maps to
`untrusted`.

## File Structure

| Path | Responsibility |
| --- | --- |
| `go.mod`, `go.sum` | Pin BurntSushi TOML v1.6.0. |
| `internal/canonicaljson/canonical.go` | Emit closed deterministic JSON and SHA-256 digests. |
| `internal/canonicaljson/canonical_test.go` | Prove stable order and reject unsupported values. |
| `internal/catalog/records.go` | Add explicit TOML tags to Descriptor and Recipe records. |
| `internal/assets/schemas/v1/user-config.schema.json` | Validate normalized user configuration. |
| `internal/assets/schemas/v1/project-config.schema.json` | Validate normalized authority-free project configuration. |
| `internal/schema/registry.go` | Compile and expose the two configuration schemas. |
| `internal/schema/registry_test.go` | Prove both new schemas compile and reject authority fields. |
| `internal/config/records.go` | Define TOML/JSON DTOs, trust status, references, pins, preferences, and limits. |
| `internal/config/decode.go` | Strictly decode TOML, normalize collections, validate schema and domain invariants. |
| `internal/config/paths.go` | Resolve physical roots and read contained referenced files without symlink escape. |
| `internal/config/project.go` | Inspect canonical project content and evaluate exact user trust. |
| `internal/config/snapshot.go` | Merge whole records into an immutable effective Catalog and Configuration Snapshot. |
| `internal/config/config_test.go` | Cover strict decoding, namespace, duplicate, path, trust, merge, and immutability rules. |
| `internal/discovery/records.go` | Define immutable discovery evidence, candidates, and reports. |
| `internal/discovery/discover.go` | Execute admitted probes under an explicit physical user-home root. |
| `internal/discovery/discovery_test.go` | Cover matches, misses, version ordering, content digests, symlinks, limits, and determinism. |
| `internal/registry/records.go` | Define Provider state, verified Capability, Provider Instance, and report records. |
| `internal/registry/resolve.go` | Resolve pins, limits, bindings, all state outcomes, and instance digests. |
| `internal/registry/registry.go` | Store immutable verified instances and Capability indexes. |
| `internal/registry/registry_test.go` | Cover all eight states, exact pins, partial capabilities, copies, and deterministic digests. |
| `internal/integration/config_discovery_test.go` | Prove the complete built-in/user/trusted-project snapshot-to-registry path. |

### Task 1: Add Canonical JSON and strict TOML support

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `internal/canonicaljson/canonical.go`
- Create: `internal/canonicaljson/canonical_test.go`

- [ ] **Step 1: Write failing Canonical JSON tests**

Create tests with these exact assertions:

```go
func TestMarshalAndDigestAreDeterministic(t *testing.T) {
    value := struct {
        SchemaVersion string   `json:"schema_version"`
        Values        []string `json:"values"`
    }{"oaw.test/v1", []string{"a", "b"}}
    first, err := Marshal(value)
    if err != nil {
        t.Fatal(err)
    }
    second, _ := Marshal(value)
    if string(first) != `{"schema_version":"oaw.test/v1","values":["a","b"]}` || !bytes.Equal(first, second) {
        t.Fatalf("canonical outputs differ: %s / %s", first, second)
    }
    digest, encoded, err := Digest(value)
    if err != nil || !bytes.Equal(first, encoded) || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(digest) {
        t.Fatalf("Digest() = %q, %s, %v", digest, encoded, err)
    }
}

func TestMarshalRejectsOpenOrUnstableValues(t *testing.T) {
    for _, value := range []any{map[string]string{"a": "b"}, 1.5, func() {}} {
        if _, err := Marshal(value); err == nil || !strings.Contains(err.Error(), "CANONICAL_JSON_UNSUPPORTED") {
            t.Fatalf("Marshal(%T) error = %v", value, err)
        }
    }
}
```

- [ ] **Step 2: Run the tests to verify RED**

Run: `GOCACHE=/tmp/oaw-go-build-ticket02 go test ./internal/canonicaljson`

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement the closed encoder and digest helper**

Expose exactly:

```go
func Marshal(value any) ([]byte, error)
func Digest(value any) (digest string, encoded []byte, err error)
func DigestBytes(value []byte) string
```

Recursively inspect pointers, structs, arrays, slices, booleans, strings, and
integer kinds. Reject maps, floating point, interfaces holding open map values,
functions, channels, complex values, and invalid values with
`CANONICAL_JSON_UNSUPPORTED`. Use `json.Marshal` only after the closed-value
check, and hash the exact returned bytes.

- [ ] **Step 4: Add and pin the TOML dependency**

Run:

```bash
GOCACHE=/tmp/oaw-go-build-ticket02 go get github.com/BurntSushi/toml@v1.6.0
GOCACHE=/tmp/oaw-go-build-ticket02 go mod tidy
```

Expected: `go.mod` has one direct TOML dependency at v1.6.0 and preserves the
existing JSON Schema dependency.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w internal/canonicaljson/*.go && GOCACHE=/tmp/oaw-go-build-ticket02 go test ./internal/canonicaljson`

Expected: PASS.

```bash
git add go.mod go.sum internal/canonicaljson
git commit -m "feat: add canonical configuration encoding"
```

### Task 2: Define strict user and project configuration contracts

**Files:**
- Modify: `internal/catalog/records.go`
- Create: `internal/assets/schemas/v1/user-config.schema.json`
- Create: `internal/assets/schemas/v1/project-config.schema.json`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`
- Create: `internal/config/records.go`
- Create: `internal/config/decode.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write failing strict-decoding tests**

Cover:

```go
func TestDecodeUserConfigRejectsUnknownFields(t *testing.T)
func TestDecodeProjectConfigHasNoAuthorityFields(t *testing.T)
func TestDecodeConfigNormalizesEquivalentTOML(t *testing.T)
func TestDecodeProviderAndRecipeTOMLUseExistingCatalogContracts(t *testing.T)
func TestDecodeConfigRejectsDuplicateIDsUnsafePathsAndBadDigests(t *testing.T)
```

The equivalent-TOML test decodes two documents with different whitespace and
table order and asserts identical Canonical JSON and digest. The project test
includes `enabled_providers`, `binding_preferences`, and `authority` one at a
time and requires `CONFIG_UNKNOWN_FIELD` or `SCHEMA_VALIDATION_FAILED`.

- [ ] **Step 2: Run the tests to verify RED**

Run: `GOCACHE=/tmp/oaw-go-build-ticket02 go test ./internal/config ./internal/schema`

Expected: FAIL because the configuration package and schemas do not exist.

- [ ] **Step 3: Add TOML tags to existing Catalog records**

Every JSON field in `ProviderDescriptorRecord`, `DiscoveryProbe`,
`CapabilityRecord`, `HostBinding`, `ProfileRecipeRecord`, `RecipeNode`,
`RecipeTransition`, `IncidentRoute`, and `CapabilitySelector` receives the same
lowercase underscore TOML name, for example:

```go
ID string `json:"id" toml:"id"`
Discovery []DiscoveryProbe `json:"discovery" toml:"discovery"`
DelegationAllowList []string `json:"delegation_allow_list" toml:"delegation_allow_list"`
```

Do not change JSON names or validation behavior.

- [ ] **Step 4: Define exact configuration DTOs**

Create the constants and records described in Locked Contracts:

```go
const (
    UserConfigSchemaV1 = "oaw.user-config/v1"
    ProjectConfigSchemaV1 = "oaw.project-config/v1"
)

type ContentReference struct { ID, Path string; Replace bool }
type ProviderPin struct { ID, Location, Version string }
type BindingPreference struct { ProviderID, CapabilityID, Host, Kind, Reference string }
type ProjectTrust struct { Root, ConfigDigest string; DescriptorDigests, RecipeDigests []string }
type CapabilityLimit struct { ProviderID string; CapabilityIDs []string }
type UserConfigRecord struct { SchemaVersion string; DeniedProviders []string; ProviderDescriptors, ProfileRecipes []ContentReference; ProviderPins []ProviderPin; BindingPreferences []BindingPreference; ProjectTrust []ProjectTrust }
type ProjectConfigRecord struct { SchemaVersion string; RequiredProviders, RecommendedProviders []string; ProviderDescriptors, ProfileRecipes []ContentReference; CapabilityLimits []CapabilityLimit }
```

Use explicit `toml` and `json` tags on every field. Normalize every collection
to a non-nil slice and sort it by its stable identity. Reject duplicate stable
identities rather than silently collapsing them.

- [ ] **Step 5: Add embedded JSON Schemas**

Both schemas use Draft 2020-12, `additionalProperties: false`, exact schema
version constants, the Ticket 01 qualified/local ID patterns, lowercase
64-character digest patterns, `uniqueItems`, and maximum collection sizes of
1024. Project configuration intentionally omits every authority-bearing field.

Extend `schema.Registry` with:

```go
const UserConfigV1 = "https://open-agent-workflow.dev/schemas/v1/user-config.schema.json"
const ProjectConfigV1 = "https://open-agent-workflow.dev/schemas/v1/project-config.schema.json"
```

- [ ] **Step 6: Implement strict TOML normalization**

Expose:

```go
type Decoded[T any] struct { Record T; CanonicalJSON []byte; Digest string }
func DecodeUser(raw []byte, registry *schema.Registry) (Decoded[UserConfigRecord], error)
func DecodeProject(raw []byte, registry *schema.Registry) (Decoded[ProjectConfigRecord], error)
func DecodeProvider(raw []byte, registry *schema.Registry) (Decoded[catalog.ProviderDescriptorRecord], error)
func DecodeRecipe(raw []byte, registry *schema.Registry) (Decoded[catalog.ProfileRecipeRecord], error)
```

Reject inputs larger than 1 MiB, invalid UTF-8, TOML syntax errors, and every
`MetaData.Undecoded()` key. Normalize first, then Canonical JSON encode, JSON
Schema validate, and apply domain checks. Provider and Recipe TOML canonical
JSON must pass the existing schemas and `catalog.DecodeProvider` or
`catalog.DecodeRecipe` so TOML never creates a second contract.

- [ ] **Step 7: Verify and commit**

Run:

```bash
gofmt -w internal/catalog/records.go internal/config/*.go internal/schema/*.go
GOCACHE=/tmp/oaw-go-build-ticket02 go test ./internal/config ./internal/schema ./internal/catalog
```

Expected: PASS and no Ticket 01 JSON output changes except the intentional
addition of inert TOML struct tags.

```bash
git add internal/catalog/records.go internal/assets/schemas/v1 internal/schema internal/config
git commit -m "feat: define trusted configuration contracts"
```

### Task 3: Load contained content and evaluate exact Project Trust

**Files:**
- Create: `internal/config/paths.go`
- Create: `internal/config/project.go`
- Extend: `internal/config/config_test.go`

- [ ] **Step 1: Write failing path and trust tests**

Use `t.TempDir()` fixtures to cover:

```go
func TestInspectProjectProducesPhysicalCanonicalFingerprint(t *testing.T)
func TestInspectProjectRejectsSymlinkEscape(t *testing.T)
func TestInspectProjectRejectsAbsoluteTraversalAndBackslashReferences(t *testing.T)
func TestEvaluateProjectTrustRequiresEveryExactDigest(t *testing.T)
func TestFormattingOnlyProjectChangeKeepsTrust(t *testing.T)
func TestSemanticProjectChangeRevokesTrust(t *testing.T)
```

The trust test changes the config digest, one Descriptor digest, one Recipe
digest, and the physical root independently. Every mismatch returns
`ProjectUntrusted` with a stable reason; no mismatch returns `ProjectTrusted`.

- [ ] **Step 2: Run the tests to verify RED**

Run: `GOCACHE=/tmp/oaw-go-build-ticket02 go test ./internal/config -run 'Project|Path|Trust'`

Expected: FAIL because path and trust functions do not exist.

- [ ] **Step 3: Implement contained reads**

Expose package-private helpers:

```go
func physicalRoot(path string) (string, error)
func validateReferencePath(path string) error
func readContained(root, relative string, maximum int64) ([]byte, string, error)
```

`readContained` evaluates the final path physically, proves it remains under
the physical root with `filepath.Rel`, requires a regular file, enforces the
byte limit before and during read, and returns bytes plus physical path. Use
reason codes `CONFIG_PATH_INVALID`, `CONFIG_PATH_ESCAPE`,
`CONFIG_FILE_NOT_REGULAR`, and `CONFIG_FILE_TOO_LARGE`.

- [ ] **Step 4: Implement project inspection**

Define:

```go
type ProjectFingerprint struct {
    Root string
    Config ProjectConfigRecord
    ConfigDigest string
    DescriptorDigests []string
    RecipeDigests []string
    ProviderIDs []string
    RecipeIDs []string
}

func InspectProject(projectRoot string, registry *schema.Registry) (ProjectFingerprint, error)
```

Read `.oaw/config.toml` and referenced content under `.oaw`, decode every
record, require reference ID equality, sort all digest and ID lists, and retain
decoded content package-privately for the snapshot builder. Do not execute or
load any Provider code.

- [ ] **Step 5: Implement trust evaluation**

Define:

```go
type ProjectTrustStatus string
const (
    ProjectAbsent ProjectTrustStatus = "absent"
    ProjectTrusted ProjectTrustStatus = "trusted"
    ProjectUntrusted ProjectTrustStatus = "untrusted"
)

func EvaluateProjectTrust(records []ProjectTrust, fingerprint ProjectFingerprint) (ProjectTrustStatus, string)
```

Return stable reasons `PROJECT_TRUST_MISSING`, `PROJECT_ROOT_MISMATCH`,
`PROJECT_CONFIG_DIGEST_MISMATCH`, `PROJECT_DESCRIPTOR_DIGEST_MISMATCH`, or
`PROJECT_RECIPE_DIGEST_MISMATCH`. Do not treat an untrusted project as a fatal
permission grant or merge its records.

- [ ] **Step 6: Verify and commit**

Run: `gofmt -w internal/config/*.go && GOCACHE=/tmp/oaw-go-build-ticket02 go test ./internal/config`

Expected: PASS.

```bash
git add internal/config
git commit -m "feat: verify exact project configuration trust"
```

### Task 4: Build immutable Effective Configuration Snapshots

**Files:**
- Create: `internal/config/snapshot.go`
- Extend: `internal/config/config_test.go`

- [ ] **Step 1: Write failing snapshot and merge tests**

Cover:

```go
func TestLoadBuildsBuiltInOnlySnapshotWithoutFiles(t *testing.T)
func TestLoadMergesUserAndTrustedProjectWholeRecords(t *testing.T)
func TestLoadExcludesUntrustedProjectRecords(t *testing.T)
func TestUserDenyWinsOverEveryLayer(t *testing.T)
func TestReservedNamespaceAndImplicitReplacementFailClosed(t *testing.T)
func TestSnapshotIsImmutableAcrossFileChanges(t *testing.T)
func TestSnapshotDigestIsDeterministic(t *testing.T)
```

The merge fixture adds one user Provider, replaces it from an exactly trusted
project with `replace=true`, adds one custom Recipe, and asserts built-in aliases
remain intact. The untrusted fixture exposes the project Provider ID only
through `UntrustedProviderIDs()`.

- [ ] **Step 2: Run the tests to verify RED**

Run: `GOCACHE=/tmp/oaw-go-build-ticket02 go test ./internal/config -run 'Snapshot|Load|Deny|Namespace|Replacement'`

Expected: FAIL because Snapshot and Load do not exist.

- [ ] **Step 3: Implement immutable snapshot values**

Expose:

```go
type LoadOptions struct { UserConfigRoot, ProjectRoot string }
type ProviderSettings struct { ProviderID string; Disabled bool; Pin *ProviderPin; Preferences []BindingPreference; CapabilityLimit []string; Digest string }
type Snapshot struct { /* private copied fields */ }

func Load(options LoadOptions) (Snapshot, error)
func (s Snapshot) Digest() string
func (s Snapshot) Catalog() catalog.Catalog
func (s Snapshot) ProjectStatus() ProjectTrustStatus
func (s Snapshot) ProjectReason() string
func (s Snapshot) ProviderSettings(id string) ProviderSettings
func (s Snapshot) RequiredProviders() []string
func (s Snapshot) RecommendedProviders() []string
func (s Snapshot) UntrustedProviderIDs() []string
```

`Load` builds the embedded Catalog and Schema Registry, loads optional
`config.toml`, inspects optional project content, evaluates trust, merges
Provider and Recipe records in built-in/user/trusted-project order, and calls
`catalog.New` once. Missing user or project config is a valid empty layer.

- [ ] **Step 4: Implement whole-record merge and authority rules**

Use keyed slices internally but emit sorted slices. Reject:

```text
RESERVED_PROVIDER_NAMESPACE
RESERVED_RECIPE_NAMESPACE
DUPLICATE_PROVIDER_REPLACEMENT_REQUIRED
DUPLICATE_RECIPE_REPLACEMENT_REQUIRED
CONTENT_REFERENCE_ID_MISMATCH
CAPABILITY_LIMIT_UNKNOWN
BINDING_PREFERENCE_UNDECLARED
```

User deny is checked after every layer and encoded into ProviderSettings.
Project capability limits intersect multiple entries and never add unknown
Capabilities. Snapshot digest covers the effective Catalog digest, user config
digest, physical project root, project config digest and trust result, settings,
requirements, recommendations, and untrusted IDs.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w internal/config/*.go && GOCACHE=/tmp/oaw-go-build-ticket02 go test ./internal/config ./internal/builtin ./internal/catalog`

Expected: PASS; modifying fixture files after `Load` never changes an existing
snapshot, while a new `Load` receives a different digest.

```bash
git add internal/config
git commit -m "feat: build effective configuration snapshots"
```

### Task 5: Execute deterministic declarative discovery

**Files:**
- Create: `internal/discovery/records.go`
- Create: `internal/discovery/discover.go`
- Create: `internal/discovery/discovery_test.go`

- [ ] **Step 1: Write failing discovery tests**

Cover:

```go
func TestDiscoverBuiltInsProducesSortedEvidence(t *testing.T)
func TestDiscoverGroupsDirectEvidenceIntoOneCandidate(t *testing.T)
func TestDiscoverCreatesOneCandidatePerImmediateVersion(t *testing.T)
func TestDiscoverReportsNoCandidateForMissingFiles(t *testing.T)
func TestDiscoverRejectsEscapingSymlinkAndOversizedEvidence(t *testing.T)
func TestDiscoverIgnoresNestedVersionDirectories(t *testing.T)
func TestDiscoveryDigestIsIndependentOfDirectoryEnumerationOrder(t *testing.T)
```

Build a temporary user home containing the exact Ticket 01 built-in probe
paths. Write distinct bytes into each leaf and assert the recorded content
digests, physical paths, candidate versions, and report digest.

- [ ] **Step 2: Run the tests to verify RED**

Run: `GOCACHE=/tmp/oaw-go-build-ticket02 go test ./internal/discovery`

Expected: FAIL because discovery does not exist.

- [ ] **Step 3: Define immutable evidence records**

Expose:

```go
type Evidence struct { ProviderID, CandidateKey, ProbeID, Kind, Path, Version, ContentDigest string }
type Candidate struct { ProviderID, Key, Location, Version, EvidenceDigest string; Evidence []Evidence }
type Report struct { /* private copied candidates and digest */ }
func (r Report) Candidates(providerID string) []Candidate
func (r Report) Digest() string
```

All constructors sort and copy collections; getters return defensive copies.

- [ ] **Step 4: Implement safe probe execution**

Expose:

```go
type Options struct { UserHome string; MaximumEvidenceBytes int64 }
func Discover(value catalog.Catalog, options Options) (Report, error)
```

Default maximum evidence size is 4 MiB. Resolve the home physically once. A
path probe reads one admitted leaf. A version probe enumerates only immediate
prefix children in bytewise order and evaluates exactly one suffix per child.
Use the same contained-path proof as configuration without importing an open
filesystem plugin. Missing paths are normal absence; permission, escape,
non-regular, and size failures return stable `DISCOVERY_*` reason codes.

- [ ] **Step 5: Construct and hash candidates**

Group direct evidence by Provider and physical home root. Group versioned
evidence by Provider and physical version directory. Deduplicate identical
evidence, sort everything, derive direct content versions, and use
`canonicaljson.Digest` for evidence and report digests.

- [ ] **Step 6: Verify and commit**

Run: `gofmt -w internal/discovery/*.go && GOCACHE=/tmp/oaw-go-build-ticket02 go test ./internal/discovery ./internal/builtin`

Expected: PASS without executing Shell or reading environment variables.

```bash
git add internal/discovery
git commit -m "feat: discover provider installation evidence"
```

### Task 6: Resolve every Provider state and verify exact instances

**Files:**
- Create: `internal/registry/records.go`
- Create: `internal/registry/resolve.go`
- Create: `internal/registry/registry.go`
- Create: `internal/registry/registry_test.go`

- [ ] **Step 1: Write failing state-table tests**

Create one table case for each exact state:

```go
not-found              // no candidates
candidate              // one candidate, nil inventory
verified               // one candidate and at least one admitted binding
ambiguous              // multiple candidates and no matching pin
incompatible           // an exact location/version pin matches none
binding-unavailable    // authoritative inventory matches no admitted binding
disabled               // user deny
untrusted              // excluded project-only descriptor ID
```

Also cover exact pin disambiguation, user binding preference, deterministic
fallback binding selection, project capability intersection, partial verified
Capability coverage, defensive copies, and instance/report digest stability.

- [ ] **Step 2: Run the tests to verify RED**

Run: `GOCACHE=/tmp/oaw-go-build-ticket02 go test ./internal/registry`

Expected: FAIL because registry records and resolver do not exist.

- [ ] **Step 3: Define closed state and instance records**

Expose:

```go
type ProviderState string
const (
    NotFound ProviderState = "not-found"
    CandidateState ProviderState = "candidate"
    Verified ProviderState = "verified"
    Ambiguous ProviderState = "ambiguous"
    Incompatible ProviderState = "incompatible"
    BindingUnavailable ProviderState = "binding-unavailable"
    Disabled ProviderState = "disabled"
    Untrusted ProviderState = "untrusted"
)

type BindingInventory struct { Host string; Bindings []catalog.HostBinding }
type VerifiedCapability struct { ID string; Binding catalog.HostBinding }
type ProviderInstance struct { ProviderID, DescriptorDigest, Location, Version, ConfigurationDigest, BindingDigest, EvidenceDigest, Digest string; Capabilities []VerifiedCapability }
type ProviderResolution struct { ProviderID string; State ProviderState; Reason string; Instance *ProviderInstance; Candidates []discovery.Candidate }
type ResolutionReport struct { /* private sorted records and digest */ }
```

- [ ] **Step 4: Implement deterministic resolution**

Expose:

```go
func Resolve(snapshot config.Snapshot, evidence discovery.Report, inventory *BindingInventory) (ResolutionReport, Registry, error)
```

Resolution order is: user deny, untrusted IDs, evidence absence, exact pin,
ambiguity, authoritative inventory, project Capability limit, binding
preference/fallback, instance construction. Use stable reasons:

```text
PROVIDER_NOT_FOUND
PROVIDER_DISCOVERED_UNVERIFIED
PROVIDER_CANDIDATE_AMBIGUOUS
PROVIDER_PIN_INCOMPATIBLE
PROVIDER_BINDING_UNAVAILABLE
PROVIDER_DISABLED_BY_USER
PROVIDER_PROJECT_CONTENT_UNTRUSTED
PROVIDER_VERIFIED
```

Descriptor, provider settings, selected bindings, candidate evidence, instance,
and report digests all use Canonical JSON. Never infer a binding from the local
filesystem.

- [ ] **Step 5: Implement immutable Effective Registry**

Expose:

```go
type Registry struct { /* private copied verified instances and indexes */ }
func (r Registry) Providers() []ProviderInstance
func (r Registry) Provider(id string) (ProviderInstance, bool)
func (r Registry) Capability(providerID, capabilityID string) (VerifiedCapability, bool)
func (r Registry) Digest() string
```

Only verified resolutions are inserted. Sort Providers by ID and Capabilities
by ID. Duplicate verified Provider IDs fail with
`DUPLICATE_PROVIDER_INSTANCE`.

- [ ] **Step 6: Verify and commit**

Run: `gofmt -w internal/registry/*.go && GOCACHE=/tmp/oaw-go-build-ticket02 go test ./internal/registry ./internal/config ./internal/discovery`

Expected: PASS for all eight states and exact pin fields.

```bash
git add internal/registry
git commit -m "feat: verify providers into effective registry"
```

### Task 7: Prove the Ticket 02 vertical slice

**Files:**
- Create: `internal/integration/config_discovery_test.go`

- [ ] **Step 1: Write the cross-package integration fixture**

The test must:

1. load the built-in Catalog;
2. create user TOML with one third-party Descriptor, one custom Recipe, a user
   deny, and one exact project trust record;
3. create exact trusted project TOML that narrows one Provider Capability set;
4. create built-in and third-party discovery leaves under a temporary home;
5. discover deterministic evidence;
6. supply an explicit Codex binding inventory;
7. resolve an Effective Registry;
8. assert verified, disabled, and partial-capability outcomes;
9. mutate disk configuration and prove the existing Snapshot and Registry
   digests remain unchanged;
10. reload and prove the new Snapshot digest changes without affecting the
    previous objects.

- [ ] **Step 2: Add negative vertical fixtures**

Cover one-byte project Descriptor drift, an escaping symlink, two unpinned
version candidates, and an inventory lacking all declared bindings. Assert the
exact trust/provider states and reason codes rather than free-form error text.

- [ ] **Step 3: Run focused and race tests**

Run:

```bash
gofmt -w internal/integration/*.go
GOCACHE=/tmp/oaw-go-build-ticket02 go test ./internal/integration
GOCACHE=/tmp/oaw-go-build-ticket02 go test -race ./internal/config ./internal/discovery ./internal/registry ./internal/integration
```

Expected: PASS with deterministic digests and no race.

- [ ] **Step 4: Commit the vertical slice**

```bash
git add internal/integration
git commit -m "test: verify configuration discovery vertical slice"
```

### Task 8: Verify Ticket 02 and preserve Ticket 01/Bash authority

**Files:**
- Modify only if a failing check requires a scoped Ticket 02 correction: files created or listed above

- [ ] **Step 1: Run formatting and static checks**

```bash
test -z "$(gofmt -l cmd/oaw/*.go internal/*/*.go)"
GOCACHE=/tmp/oaw-go-build-ticket02 go vet ./...
git diff --check
```

Expected: all exit `0` and formatting/diff checks print nothing.

- [ ] **Step 2: Run all Go tests with race detection**

Run: `GOCACHE=/tmp/oaw-go-build-ticket02 go test -race ./...`

Expected: every package passes and no race is reported.

- [ ] **Step 3: Enforce repository-wide Go statement coverage**

```bash
coverage_file=$(mktemp /tmp/oaw-ticket-02-cover.XXXXXX)
GOCACHE=/tmp/oaw-go-build-ticket02 go test -coverprofile="$coverage_file" ./...
GOCACHE=/tmp/oaw-go-build-ticket02 go tool cover -func="$coverage_file"
GOCACHE=/tmp/oaw-go-build-ticket02 go tool cover -func="$coverage_file" | awk '/^total:/ { gsub("%", "", $3); if (($3 + 0) < 80) exit 1 }'
rm -f "$coverage_file"
```

Expected: total statement coverage is at least `80.0%`.

- [ ] **Step 4: Run documentation and Bash authority regressions**

```bash
bash scripts/check-docs.sh
bash tests/run.sh
```

Expected: documentation passes and the Bash runner ends with
`PASS: all implemented installer cases passed`.

- [ ] **Step 5: Review dependency and forbidden surfaces**

```bash
go list -m all
git status --short
forbidden_tracked=$(git diff --name-only b0a419d -- install.sh lib tests cmd/oaw internal/cli)
forbidden_untracked=$(git ls-files --others --exclude-standard -- lib tests)
test -z "$forbidden_tracked$forbidden_untracked"
```

Expected: BurntSushi TOML is the only new direct dependency; existing JSON
Schema dependencies remain pinned; no Ticket 02 change touches the Bash
installer, Bash test tree, Ticket 01 CLI, or `.serena/` metadata.

- [ ] **Step 6: Review security invariants**

Search the Ticket 02 diff and prove:

- no Shell execution exists in configuration, trust, discovery, or registry;
- no record supports commands, scripts, templates, environment interpolation,
  arbitrary expressions, remote URLs, credentials, or authority grants;
- project paths are physically contained and symlink escapes fail closed;
- user deny is applied before candidate verification;
- raw discovery paths are not written to CLI output;
- only verified instances enter Effective Registry;
- every returned collection is a defensive copy.

- [ ] **Step 7: Commit final scoped corrections only when needed**

If Steps 1-6 changed tracked Ticket 02 files:

```bash
git add go.mod go.sum internal
git commit -m "test: harden configuration discovery foundation"
```

Do not create an empty commit.

## Self-Review Record

- Spec sections 7 and 8 map to Tasks 2-6: inert Descriptors, exact trust,
  whole-record merge, user deny, immutable snapshots, discovery evidence,
  Provider states, pinned instances, and Effective Registry.
- Spec section 14 maps to Tasks 3 and 5: enumerated probes, Go filesystem APIs,
  physical containment, symlink rejection, and exact digests.
- Spec section 15 maps to every package: strict DTOs, private immutable domain
  state, copied collections, small dependency set, and no plugin runtime.
- Ticket 02 does not implement the Profile compiler from Ticket 04. It merely
  admits trusted Recipe records into the effective Catalog through the same
  Ticket 01 schema and domain contract.
- Ticket 02 does not claim Host binding availability from discovery. A nil
  authoritative inventory preserves `candidate`; only explicit inventory can
  produce `verified` or `binding-unavailable`.
- Configuration reload creates a new Snapshot; no API mutates an existing
  Snapshot, Registry, or active Lifecycle Bundle.
- The plan contains no unresolved placeholder, alternate API name, or silent
  default. Every new public package seam, state, reason code, file, command,
  verification threshold, and commit boundary is specified.
