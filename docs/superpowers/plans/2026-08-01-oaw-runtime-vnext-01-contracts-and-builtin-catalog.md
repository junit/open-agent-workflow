# OAW Runtime vNext Ticket 01 Contracts and Built-in Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the Go contract foundation and a deterministic, inspectable built-in Provider and Profile Recipe catalog without changing the authoritative Bash management interface.

**Architecture:** A small Go module defines strict record contracts and validated immutable catalog snapshots. Versioned JSON Schema documents and built-in Provider, Recipe, and alias records are repository assets embedded into the `oaw` binary; every record passes Draft 2020-12 validation and strict Go decoding before it enters the Catalog. The human-facing catalog command reads only those assets and never performs discovery, trust resolution, Provider execution, installation, or Runtime transitions.

**Tech Stack:** Go 1.26, `encoding/json`, `embed`, `crypto/sha256`, `github.com/santhosh-tekuri/jsonschema/v6 v6.0.2`, table-driven Go tests, and the existing Bash regression harness.

**Canonical sources:** `.scratch/oaw-runtime-vnext/spec.md`, `.scratch/oaw-runtime-vnext/issues/01-contracts-and-builtin-catalog.md`, `CONTEXT.md`, `docs/adr/0003-add-optional-capability-admission-runtime.md`, and `docs/adr/0004-implement-runtime-core-in-go.md`.

---

## File Map

| Path | Responsibility |
| --- | --- |
| `go.mod` | Pin the module identity and Go language version. |
| `go.sum` | Pin JSON Schema validator dependency content. |
| `cmd/oaw/main.go` | Translate process arguments and streams into the internal CLI runner. |
| `internal/cli/run.go` | Parse the Ticket 01 command surface and return stable exit codes. |
| `internal/cli/catalog.go` | Render deterministic catalog text and JSON. |
| `internal/cli/catalog_test.go` | Test public catalog behavior through the CLI seam. |
| `internal/catalog/ids.go` | Validate Provider, Recipe, Capability, node, and alias identifiers. |
| `internal/catalog/ids_test.go` | Lock identifier and version rules. |
| `internal/catalog/records.go` | Define strict JSON decoding records and closed enumerations. |
| `internal/catalog/decode.go` | Strictly decode one Provider, Recipe, or alias-set record. |
| `internal/catalog/decode_test.go` | Lock strict JSON and schema-version behavior. |
| `internal/catalog/catalog.go` | Construct a sorted, copied, immutable-by-API Catalog snapshot. |
| `internal/catalog/validate.go` | Enforce cross-record ownership, selector, graph, and alias invariants. |
| `internal/catalog/catalog_test.go` | Test invalid catalogs and defensive-copy behavior. |
| `internal/assets/embed.go` | Incrementally embed schemas and built-in records and expose a read-only filesystem. |
| `internal/assets/schemas/v1/provider-descriptor.schema.json` | Define the Provider Descriptor JSON contract. |
| `internal/assets/schemas/v1/profile-recipe.schema.json` | Define the Profile Recipe JSON contract. |
| `internal/assets/schemas/v1/profile-alias-set.schema.json` | Define the compatibility-alias JSON contract. |
| `internal/schema/registry.go` | Compile embedded Draft 2020-12 schemas and validate raw records without network loading. |
| `internal/schema/registry_test.go` | Prove schemas compile and reject malformed records. |
| `internal/assets/providers/oaw-superpowers.json` | Declare Superpowers discovery evidence and capabilities. |
| `internal/assets/providers/oaw-matt.json` | Declare Matt discovery evidence and capabilities. |
| `internal/assets/providers/oaw-ecc.json` | Declare ECC discovery evidence and capabilities. |
| `internal/assets/recipes/oaw-delivery.json` | Declare the Superpowers complete lifecycle. |
| `internal/assets/recipes/oaw-domain-engineering.json` | Declare the Matt complete lifecycle. |
| `internal/assets/recipes/oaw-ecc-engineering.json` | Declare the ECC complete lifecycle. |
| `internal/assets/recipes/oaw-reliable-feature.json` | Declare the built-in Matt/Superpowers composition. |
| `internal/assets/recipes/oaw-hardening.json` | Declare the separate composed hardening lifecycle. |
| `internal/assets/profile-aliases.json` | Map four compatibility aliases to built-in Recipes. |
| `internal/builtin/load.go` | Load embedded records through the same strict catalog constructors used by later external records. |
| `internal/builtin/load_test.go` | Prove embedded records load, validate, sort, and remain stable. |

## Locked Ticket 01 Contracts

- Module path: `github.com/wifibaby4u/open-agent-workflow`.
- Go directive: `go 1.26`.
- Ticket 01 pins exactly one direct dependency,
  `github.com/santhosh-tekuri/jsonschema/v6 v6.0.2`, and commits the
  resulting `go.sum`.
- Record schema versions are `oaw.provider-descriptor/v1`, `oaw.profile-recipe/v1`, and `oaw.profile-alias-set/v1`.
- Descriptor and Recipe content versions use three-component decimal semantic versions such as `1.0.0`.
- Built-in qualified IDs use exactly two lowercase slash-separated segments and reserve the `oaw/` namespace.
- Capability and graph-node IDs are lowercase slugs containing letters, digits, dots, and hyphens.
- Catalog order is bytewise ascending by Provider ID, Recipe ID, and alias.
- CLI surface:

~~~text
oaw catalog list providers [--format text|json]
oaw catalog list recipes [--format text|json]
oaw catalog list aliases [--format text|json]
oaw catalog validate [--format text|json]
~~~

- Exit `0` means success, `64` means invalid command syntax, and `65` means embedded catalog invalidity.
- The existing `install.sh` remains unchanged and authoritative.

### Task 1: Initialize the Go module and identifier value objects

**Files:**
- Create: `go.mod`
- Create: `internal/catalog/ids_test.go`
- Create: `internal/catalog/ids.go`

- [ ] **Step 1: Write failing identifier and version tests**

Create table-driven tests that accept the built-in qualified IDs and reject uppercase names, missing namespaces, extra slash segments, traversal-like values, whitespace, and control characters:

~~~go
package catalog

import "testing"

func TestParseQualifiedID(t *testing.T) {
    t.Parallel()

    valid := []string{"oaw/superpowers", "oaw/reliable-feature", "acme/engineering-suite"}
    for _, input := range valid {
        input := input
        t.Run(input, func(t *testing.T) {
            t.Parallel()
            got, err := ParseQualifiedID(input)
            if err != nil {
                t.Fatalf("ParseQualifiedID(%q): %v", input, err)
            }
            if got.String() != input {
                t.Fatalf("String() = %q, want %q", got.String(), input)
            }
        })
    }

    invalid := []string{
        "", "oaw", "OAW/ecc", "oaw/ecc/full", "oaw/../ecc",
        "oaw/ecc ", "oaw/ecc\n", "/ecc", "oaw/",
    }
    for _, input := range invalid {
        input := input
        t.Run("invalid_"+input, func(t *testing.T) {
            t.Parallel()
            if _, err := ParseQualifiedID(input); err == nil {
                t.Fatalf("ParseQualifiedID(%q) succeeded", input)
            }
        })
    }
}

func TestParseLocalIDAndAlias(t *testing.T) {
    t.Parallel()

    for _, input := range []string{"implementation", "functional-debugging", "security.review"} {
        if _, err := ParseLocalID(input); err != nil {
            t.Fatalf("ParseLocalID(%q): %v", input, err)
        }
    }
    for _, input := range []string{"SP-FULL", "MATT-FULL", "ECC-FULL", "MATT-SP-HYBRID"} {
        if _, err := ParseAlias(input); err != nil {
            t.Fatalf("ParseAlias(%q): %v", input, err)
        }
    }
    if _, err := ParseAlias("custom locked"); err == nil {
        t.Fatal("ParseAlias accepted whitespace")
    }
}

func TestParseContentVersion(t *testing.T) {
    t.Parallel()

    if got, err := ParseContentVersion("1.0.0"); err != nil || got.String() != "1.0.0" {
        t.Fatalf("ParseContentVersion(1.0.0) = %q, %v", got.String(), err)
    }
    for _, input := range []string{"", "1", "1.0", "v1.0.0", "1.0.0-beta", "01.0.0"} {
        if _, err := ParseContentVersion(input); err == nil {
            t.Fatalf("ParseContentVersion(%q) succeeded", input)
        }
    }
}
~~~

- [ ] **Step 2: Run the test to verify RED**

Run: `go test ./internal/catalog`

Expected: FAIL because the repository has no Go module and the parsing functions do not exist.

- [ ] **Step 3: Add the module contract**

Create `go.mod` exactly as:

~~~go.mod
module github.com/wifibaby4u/open-agent-workflow

go 1.26
~~~

- [ ] **Step 4: Implement immutable identifier values**

Use private storage, constructors, and string accessors:

~~~go
package catalog

import (
    "fmt"
    "regexp"
)

var (
    qualifiedIDPattern = regexp.MustCompile("^[a-z0-9][a-z0-9.-]*/[a-z0-9][a-z0-9._-]*$")
    localIDPattern     = regexp.MustCompile("^[a-z0-9][a-z0-9.-]*$")
    aliasPattern       = regexp.MustCompile("^[A-Z0-9]+(?:-[A-Z0-9]+)*$")
    versionPattern     = regexp.MustCompile("^(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)$")
)

type QualifiedID struct{ value string }
type LocalID struct{ value string }
type Alias struct{ value string }
type ContentVersion struct{ value string }

func ParseQualifiedID(value string) (QualifiedID, error) {
    if !qualifiedIDPattern.MatchString(value) {
        return QualifiedID{}, fmt.Errorf("INVALID_QUALIFIED_ID: %q", value)
    }
    return QualifiedID{value: value}, nil
}

func ParseLocalID(value string) (LocalID, error) {
    if !localIDPattern.MatchString(value) {
        return LocalID{}, fmt.Errorf("INVALID_LOCAL_ID: %q", value)
    }
    return LocalID{value: value}, nil
}

func ParseAlias(value string) (Alias, error) {
    if !aliasPattern.MatchString(value) {
        return Alias{}, fmt.Errorf("INVALID_PROFILE_ALIAS: %q", value)
    }
    return Alias{value: value}, nil
}

func ParseContentVersion(value string) (ContentVersion, error) {
    if !versionPattern.MatchString(value) {
        return ContentVersion{}, fmt.Errorf("INVALID_CONTENT_VERSION: %q", value)
    }
    return ContentVersion{value: value}, nil
}

func (value QualifiedID) String() string   { return value.value }
func (value LocalID) String() string       { return value.value }
func (value Alias) String() string         { return value.value }
func (value ContentVersion) String() string { return value.value }
~~~

- [ ] **Step 5: Format and verify GREEN**

Run: `gofmt -w internal/catalog/ids.go internal/catalog/ids_test.go && go test ./internal/catalog`

Expected: PASS for all identifier and content-version cases.

- [ ] **Step 6: Commit the module and value objects**

~~~bash
git add go.mod internal/catalog/ids.go internal/catalog/ids_test.go
git commit -m "chore: initialize Go runtime contracts"
~~~

### Task 2: Define strict record contracts and JSON Schema assets

**Files:**
- Modify: `go.mod`
- Create: `go.sum`
- Create: `internal/catalog/records.go`
- Create: `internal/catalog/decode.go`
- Create: `internal/catalog/decode_test.go`
- Create: `internal/assets/embed.go`
- Create: `internal/assets/embed_test.go`
- Create: `internal/assets/schemas/v1/provider-descriptor.schema.json`
- Create: `internal/assets/schemas/v1/profile-recipe.schema.json`
- Create: `internal/assets/schemas/v1/profile-alias-set.schema.json`
- Create: `internal/schema/registry.go`
- Create: `internal/schema/registry_test.go`

- [ ] **Step 1: Write failing strict-decoding tests**

Cover one valid minimal record per kind, an unknown JSON field, trailing JSON,
an unsupported schema version, a duplicate array value, and an invalid closed
enum. Assert stable reason-code prefixes:

~~~go
func TestDecodeProviderRejectsUnknownField(t *testing.T) {
    raw := []byte("{\"schema_version\":\"oaw.provider-descriptor/v1\",\"descriptor_version\":\"1.0.0\",\"id\":\"oaw/test\",\"display_name\":\"Test\",\"discovery\":[],\"capabilities\":[],\"extra\":true}")
    _, err := DecodeProvider(raw)
    if err == nil || !strings.Contains(err.Error(), "INVALID_PROVIDER_DESCRIPTOR") {
        t.Fatalf("DecodeProvider() error = %v", err)
    }
}

func TestDecodeRecipeRejectsUnsupportedSchema(t *testing.T) {
    raw := []byte("{\"schema_version\":\"oaw.profile-recipe/v2\",\"recipe_version\":\"1.0.0\",\"id\":\"oaw/test\",\"display_name\":\"Test\",\"required_responsibilities\":[],\"nodes\":[],\"incident_routes\":[],\"entry\":\"start\",\"terminal_gates\":[],\"stable_boundaries\":[]}")
    _, err := DecodeRecipe(raw)
    if err == nil || !strings.Contains(err.Error(), "UNSUPPORTED_RECIPE_SCHEMA") {
        t.Fatalf("DecodeRecipe() error = %v", err)
    }
}
~~~

- [ ] **Step 2: Run the strict-decoding test to verify RED**

Run: `go test ./internal/catalog`

Expected: FAIL because record types and decoders do not exist.

- [ ] **Step 3: Define closed enums and JSON records**

Use string-backed types with constants for:

~~~go
const (
    ProviderDescriptorSchemaV1 = "oaw.provider-descriptor/v1"
    ProfileRecipeSchemaV1      = "oaw.profile-recipe/v1"
    ProfileAliasSetSchemaV1    = "oaw.profile-alias-set/v1"
)

type RequestMode string
const (
    RequestModeBounded  RequestMode = "BOUNDED"
    RequestModeWorkflow RequestMode = "WORKFLOW"
)

type ExecutorTopology string
const (
    MainAgentAllowed ExecutorTopology = "main-agent-allowed"
    IsolatedRequired ExecutorTopology = "isolated-required"
)

type NodeKind string
const (
    PhaseNode           NodeKind = "phase"
    ProcedureNode       NodeKind = "procedure"
    IncidentHandlerNode NodeKind = "incident-handler"
    CheckpointNode      NodeKind = "checkpoint"
    GateNode            NodeKind = "gate"
)
~~~

Define records with JSON tags for every field below:

~~~go
type ProviderDescriptorRecord struct {
    SchemaVersion     string             `json:"schema_version"`
    DescriptorVersion string             `json:"descriptor_version"`
    ID                string             `json:"id"`
    DisplayName       string             `json:"display_name"`
    Discovery         []DiscoveryProbe   `json:"discovery"`
    Capabilities      []CapabilityRecord `json:"capabilities"`
}

type DiscoveryProbe struct {
    ID     string   `json:"id"`
    Kind   string   `json:"kind"`
    Root   string   `json:"root"`
    Path   string   `json:"path,omitempty"`
    Prefix string   `json:"prefix,omitempty"`
    Suffix string   `json:"suffix,omitempty"`
    Paths  []string `json:"paths,omitempty"`
}

type CapabilityRecord struct {
    ID                  string            `json:"id"`
    InputSchema         string            `json:"input_schema"`
    OutcomeSchema       string            `json:"outcome_schema"`
    MaximumEffects      []string          `json:"maximum_effects"`
    Resources           []string          `json:"resources"`
    RequestModes        []RequestMode     `json:"request_modes"`
    Responsibilities    []string          `json:"responsibilities"`
    ExecutorTopology    ExecutorTopology  `json:"executor_topology"`
    DelegationAllowList []string          `json:"delegation_allow_list"`
    HostBindings        []HostBinding     `json:"host_bindings"`
}

type HostBinding struct {
    Host      string `json:"host"`
    Kind      string `json:"kind"`
    Reference string `json:"reference"`
}

type ProfileRecipeRecord struct {
    SchemaVersion             string       `json:"schema_version"`
    RecipeVersion             string       `json:"recipe_version"`
    ID                        string       `json:"id"`
    DisplayName               string       `json:"display_name"`
    RequiredResponsibilities  []string     `json:"required_responsibilities"`
    Nodes                     []RecipeNode `json:"nodes"`
    IncidentRoutes            []IncidentRoute `json:"incident_routes"`
    Entry                     string       `json:"entry"`
    TerminalGates             []string     `json:"terminal_gates"`
    StableBoundaries          []string     `json:"stable_boundaries"`
}

type RecipeNode struct {
    ID             string             `json:"id"`
    Kind           NodeKind           `json:"kind"`
    Responsibility string             `json:"responsibility"`
    Selector       CapabilitySelector `json:"selector"`
    Phase          string             `json:"phase,omitempty"`
    Optional       bool               `json:"optional,omitempty"`
    Transitions    []RecipeTransition `json:"transitions"`
}

type RecipeTransition struct {
    Signal string `json:"signal"`
    Target string `json:"target"`
}

type IncidentRoute struct {
    Incident string `json:"incident"`
    Handler  string `json:"handler"`
}

type CapabilitySelector struct {
    ProviderID   string `json:"provider_id"`
    CapabilityID string `json:"capability_id"`
}

type ProfileAliasSetRecord struct {
    SchemaVersion string               `json:"schema_version"`
    Aliases       []ProfileAliasRecord `json:"aliases"`
}

type ProfileAliasRecord struct {
    Alias    string `json:"alias"`
    RecipeID string `json:"recipe_id"`
}
~~~

- [ ] **Step 4: Implement one strict decoder**

Use the same helper for all record kinds:

~~~go
func strictDecode(data []byte, destination any) error {
    decoder := json.NewDecoder(bytes.NewReader(data))
    decoder.DisallowUnknownFields()
    if err := decoder.Decode(destination); err != nil {
        return err
    }
    if decoder.Decode(&struct{}{}) != io.EOF {
        return errors.New("trailing JSON value")
    }
    return nil
}
~~~

Each exported decoder wraps syntax/type errors with its stable record code,
checks its exact schema-version constant, validates all IDs and content
versions with Task 1 constructors, rejects duplicate members in set-like
arrays, validates closed enum values, and returns newly allocated slices.

- [ ] **Step 5: Author the three Draft 2020-12 schemas**

Each schema must declare `$schema`, an absolute `$id`,
`additionalProperties: false` at every object level, the exact
schema-version constant, the identifier patterns from Task 1, unique set-like
arrays, and these closed vocabularies:

| Field | Allowed values |
| --- | --- |
| probe kind | `path-exists`, `all-paths-exist`, `one-level-version-path-exists` |
| safe root | `user-home`, `xdg-config-home`, `project-root` |
| request mode | `BOUNDED`, `WORKFLOW` |
| executor topology | `main-agent-allowed`, `isolated-required` |
| effect | `read-project`, `write-project`, `run-process`, `git-local`, `network-read` |
| resource | `project`, `project-worktree`, `git-repository` |
| binding kind | `skill`, `agent`, `tool` |
| node kind | `phase`, `procedure`, `incident-handler`, `checkpoint`, `gate` |
| transition signal | `succeeded`, `finding`, `remediated` |
| incident type | `functional-failure`, `build-failure`, `dependency-failure`, `type-failure`, `security-finding` |

Define discovery probes with a JSON Schema `oneOf`, not one object
whose fields are all optional:

| Probe kind | Required payload | Forbidden payload |
| --- | --- | --- |
| `path-exists` | `id`, `kind`, `root`, `path` | `prefix`, `suffix`, `paths` |
| `all-paths-exist` | `id`, `kind`, `root`, non-empty unique `paths` | `path`, `prefix`, `suffix` |
| `one-level-version-path-exists` | `id`, `kind`, `root`, `prefix`, `suffix` | `path`, `paths` |

Every path component is a non-empty relative slash-separated path, rejects
backslashes, control characters, absolute paths, empty segments, dot segments,
and parent segments. The version probe permits exactly one directory entry
between prefix and suffix; its data never contains a glob or regular expression.
Probe IDs are unique local IDs within one Provider Descriptor.

Provider capabilities require non-empty input/outcome schema identifiers,
request modes, and Host Bindings. Their responsibility array may be empty for a
specialist-only Capability. Every Recipe node requires one selector and an
explicit transition array. Phase, procedure, incident-handler, and gate nodes
require one non-empty responsibility; checkpoint nodes may use a
specialist-only Capability and an empty responsibility. A procedure requires a
`phase` reference to a phase node and has no graph transitions because
its red-green or delegation loop runs inside that phase. Non-procedure nodes
must not set `phase`. Alias sets require at least one alias. JSON
Schema handles shape; Go validation handles cross-record references and
conditional rules.

- [ ] **Step 6: Embed assets behind a read-only filesystem**

Create:

~~~go
package assets

import (
    "embed"
    "io/fs"
)

//go:embed schemas/v1/*.json
var embedded embed.FS

func FS() fs.FS {
    return embedded
}
~~~

Task 3 extends the directive with `providers/*.json` only after those
files exist. Task 4 extends it with `recipes/*.json` and
`profile-aliases.json` only after those files exist. Every commit
therefore builds independently.

- [ ] **Step 7: Pin and implement the JSON Schema validator**

Compile only the three known embedded record schema IDs.
`oaw.capability-input/v1` and
`oaw.capability-outcome/v1` remain declared external Provider
contract identifiers and are not treated as catalog-record schemas:

~~~go
const (
    ProviderDescriptorV1 = "https://open-agent-workflow.dev/schemas/v1/provider-descriptor.schema.json"
    ProfileRecipeV1      = "https://open-agent-workflow.dev/schemas/v1/profile-recipe.schema.json"
    ProfileAliasSetV1    = "https://open-agent-workflow.dev/schemas/v1/profile-alias-set.schema.json"
)

type Registry struct {
    schemas map[string]*jsonschema.Schema
}

func New(files fs.FS) (*Registry, error) {
    compiler := jsonschema.NewCompiler()
    compiler.DefaultDraft(jsonschema.Draft2020)

    resources := []struct {
        path string
        id   string
    }{
        {"schemas/v1/provider-descriptor.schema.json", ProviderDescriptorV1},
        {"schemas/v1/profile-recipe.schema.json", ProfileRecipeV1},
        {"schemas/v1/profile-alias-set.schema.json", ProfileAliasSetV1},
    }

    for _, resource := range resources {
        data, err := fs.ReadFile(files, resource.path)
        if err != nil {
            return nil, fmt.Errorf("SCHEMA_READ_FAILED: %s: %w", resource.path, err)
        }
        var document any
        decoder := json.NewDecoder(bytes.NewReader(data))
        decoder.UseNumber()
        if err := decoder.Decode(&document); err != nil {
            return nil, fmt.Errorf("SCHEMA_DECODE_FAILED: %s: %w", resource.path, err)
        }
        if err := compiler.AddResource(resource.id, document); err != nil {
            return nil, fmt.Errorf("SCHEMA_REGISTER_FAILED: %s: %w", resource.id, err)
        }
    }

    compiled := make(map[string]*jsonschema.Schema, len(resources))
    for _, resource := range resources {
        value, err := compiler.Compile(resource.id)
        if err != nil {
            return nil, fmt.Errorf("SCHEMA_COMPILE_FAILED: %s: %w", resource.id, err)
        }
        compiled[resource.id] = value
    }
    return &Registry{schemas: compiled}, nil
}
~~~

Implement `Validate(schemaID string, raw []byte) error` by rejecting
unknown schema IDs, decoding exactly one JSON value with
`json.Decoder.UseNumber`, rejecting trailing content, and calling
`Schema.Validate`. Do not install a URL loader or fetch a remote
schema.

After `registry.go` imports the validator, run:

~~~bash
go get github.com/santhosh-tekuri/jsonschema/v6@v6.0.2
go mod tidy
~~~

Expected: `go.mod` contains the direct
`github.com/santhosh-tekuri/jsonschema/v6 v6.0.2` requirement and
`go.sum` pins its dependency graph.

- [ ] **Step 8: Verify schemas and strict decoding**

Add a test that reads every embedded schema, decodes it into
`map[string]any`, and asserts Draft 2020-12, the expected absolute
schema ID, object root type, and `additionalProperties == false`.

Run: `gofmt -w internal/catalog/*.go internal/assets/*.go internal/schema/*.go && go test ./internal/catalog ./internal/assets ./internal/schema`

Expected: PASS; unknown fields, trailing JSON, unsupported versions, invalid
enums, invalid IDs, and duplicate set members are rejected with stable codes.

- [ ] **Step 9: Commit the contract schemas**

~~~bash
git add go.mod go.sum internal/catalog internal/assets internal/schema
git commit -m "feat: define catalog schema contracts"
~~~

### Task 3: Add built-in Provider Descriptors

**Files:**
- Modify: `internal/assets/embed.go`
- Create: `internal/assets/providers/oaw-superpowers.json`
- Create: `internal/assets/providers/oaw-matt.json`
- Create: `internal/assets/providers/oaw-ecc.json`
- Create: `internal/builtin/load_test.go`

- [ ] **Step 1: Write failing Provider catalog tests**

Load all files under `providers/*.json`, validate each with
`schema.ProviderDescriptorV1`, strictly decode it, and
assert exact Provider IDs, versions, and capability IDs. The expected Provider
order is:

~~~go
[]string{"oaw/ecc", "oaw/matt", "oaw/superpowers"}
~~~

Assert every discovery path is relative, clean, contains no backslash or
control character, and does not contain a `.` or `..` segment. Add table tests
that reject missing, mixed, or surplus payload fields for each probe kind with
`DISCOVERY_PROBE_SHAPE_INVALID`, and unsafe path payloads with
`DISCOVERY_PATH_INVALID`. Assert every Capability has at least one
binding and never permits Direct Mode.

- [ ] **Step 2: Run the Provider tests to verify RED**

Run: `go test ./internal/builtin`

Expected: FAIL because the Provider assets and loader do not exist.

- [ ] **Step 3: Author exact declarative discovery probes**

Use only these records:

| Provider | Probe records |
| --- | --- |
| `oaw/superpowers` | `claude-direct`, `codex-direct`, and `claude-marketplace-checkout` are `path-exists` probes for `.claude/plugins/superpowers/skills/using-superpowers/SKILL.md`, `.codex/plugins/superpowers/skills/using-superpowers/SKILL.md`, and `.claude/plugins/marketplaces/superpowers-marketplace/skills/using-superpowers/SKILL.md`. `claude-official-cache`, `claude-marketplace-cache`, and `codex-curated-cache` are `one-level-version-path-exists` probes with prefixes `.claude/plugins/cache/claude-plugins-official/superpowers`, `.claude/plugins/cache/superpowers-marketplace/superpowers`, and `.codex/plugins/cache/openai-api-curated/superpowers`, respectively; all use suffix `skills/using-superpowers/SKILL.md`. |
| `oaw/matt` | `matt-to-spec` and `matt-to-tickets` are separate `path-exists` probes for `.agents/skills/to-spec/SKILL.md` and `.agents/skills/to-tickets/SKILL.md`. Either produces candidate evidence; later binding verification determines which lifecycle Capabilities are actually available. |
| `oaw/ecc` | `ecc-global-skill`, `ecc-marketplace-plugin`, and `ecc-marketplace-skill` are `path-exists` probes for `.agents/skills/everything-claude-code/SKILL.md`, `.claude/plugins/marketplaces/everything-claude-code/plugins/ecc/.codex-plugin/plugin.json`, and `.claude/plugins/marketplaces/everything-claude-code/.agents/skills/everything-claude-code/SKILL.md`. `ecc-versioned-cache` is a `one-level-version-path-exists` probe with prefix `.claude/plugins/cache/everything-claude-code/ecc` and suffix `.codex-plugin/plugin.json`. |

Every probe uses `root: "user-home"`. These records are evidence
declarations only. A match produces `candidate` evidence, never a
verified Provider Instance or full-family eligibility. No matches produce
`not-found`; multiple physical matches remain distinct evidence and
may require a pin. Task 02 implements probe execution, Host Binding validation,
Capability verification, and final Provider state.

- [ ] **Step 4: Author the exact Capability matrices**

Every descriptor uses version `1.0.0`. Every Capability uses
`oaw.capability-input/v1` and `oaw.capability-outcome/v1`.
Lifecycle bindings use `host: "codex"`; verification in later tickets
may mark a binding unavailable and thereby make a Recipe ineligible.

Superpowers:

| Capability | Responsibilities | Binding | Modes | Maximum effects | Resources | Delegation allow-list |
| --- | --- | --- | --- | --- | --- | --- |
| `discovery-design` | requirements, domain-modeling, specification | skill `superpowers:brainstorming` | WORKFLOW | read-project, write-project | project-worktree | empty |
| `implementation-planning` | ticket-decomposition, implementation-planning | skill `superpowers:writing-plans` | WORKFLOW | read-project, write-project | project-worktree | empty |
| `workspace` | workspace | skill `superpowers:using-git-worktrees` | WORKFLOW | read-project, write-project, run-process, git-local | project-worktree, git-repository | empty |
| `implementation` | implementation, delegation | skill `superpowers:subagent-driven-development` | WORKFLOW | read-project, write-project, run-process, git-local | project-worktree, git-repository | tdd, debugging, review, remediation, verification |
| `tdd` | tdd | skill `superpowers:test-driven-development` | WORKFLOW | read-project, write-project, run-process | project-worktree | empty |
| `debugging` | functional-debugging, build-repair, dependency-repair, type-repair | skill `superpowers:systematic-debugging` | BOUNDED, WORKFLOW | read-project, write-project, run-process | project-worktree | empty |
| `review` | review | skill `superpowers:requesting-code-review` | BOUNDED, WORKFLOW | read-project | project | empty |
| `remediation` | remediation | skill `superpowers:subagent-driven-development` | WORKFLOW | read-project, write-project, run-process | project-worktree | tdd, debugging, review |
| `verification` | verification | skill `superpowers:verification-before-completion` | BOUNDED, WORKFLOW | read-project, run-process | project | empty |
| `completion` | completion | skill `superpowers:finishing-a-development-branch` | WORKFLOW | read-project, run-process, git-local | git-repository | empty |

Matt:

| Capability | Responsibilities | Binding | Modes | Maximum effects | Resources | Delegation allow-list |
| --- | --- | --- | --- | --- | --- | --- |
| `specification` | requirements, domain-modeling, specification | skill `to-spec` | WORKFLOW | read-project, write-project | project-worktree | empty |
| `tickets` | ticket-decomposition, implementation-planning | skill `to-tickets` | WORKFLOW | read-project, write-project | project-worktree | empty |
| `implementation` | workspace, implementation, delegation | skill `implement` | WORKFLOW | read-project, write-project, run-process, git-local | project-worktree, git-repository | tdd, debugging, review, verification |
| `tdd` | tdd | skill `tdd` | WORKFLOW | read-project, write-project, run-process | project-worktree | empty |
| `debugging` | functional-debugging, build-repair, dependency-repair, type-repair | skill `diagnosing-bugs` | BOUNDED, WORKFLOW | read-project, write-project, run-process | project-worktree | empty |
| `review` | review | skill `code-review` | BOUNDED, WORKFLOW | read-project | project | empty |
| `remediation` | remediation | skill `implement` | WORKFLOW | read-project, write-project, run-process | project-worktree | tdd, debugging, review |
| `verification` | verification | skill `verification-loop` | BOUNDED, WORKFLOW | read-project, run-process | project | empty |
| `completion` | completion | skill `implement` | WORKFLOW | read-project, run-process, git-local | git-repository | empty |

ECC:

| Capability | Responsibilities | Binding | Modes | Maximum effects | Resources | Delegation allow-list |
| --- | --- | --- | --- | --- | --- | --- |
| `planning` | requirements, domain-modeling, specification, ticket-decomposition, implementation-planning | agent `planner` | WORKFLOW | read-project, write-project | project-worktree | architecture |
| `architecture` | none; callable only as a delegated specialist | agent `architect` | BOUNDED, WORKFLOW | read-project, write-project | project-worktree | empty |
| `implementation` | workspace, implementation, delegation | agent `tdd-guide` | WORKFLOW | read-project, write-project, run-process, git-local | project-worktree, git-repository | tdd, functional-debugging, build-repair, review, verification |
| `tdd` | tdd | agent `tdd-guide` | WORKFLOW | read-project, write-project, run-process | project-worktree | empty |
| `functional-debugging` | functional-debugging | agent `tdd-guide` | BOUNDED, WORKFLOW | read-project, write-project, run-process | project-worktree | empty |
| `build-repair` | build-repair, dependency-repair, type-repair | agent `build-error-resolver` | BOUNDED, WORKFLOW | read-project, write-project, run-process | project-worktree | empty |
| `review` | review | agent `code-reviewer` | BOUNDED, WORKFLOW | read-project | project | security-review |
| `remediation` | remediation | agent `tdd-guide` | WORKFLOW | read-project, write-project, run-process | project-worktree | tdd, functional-debugging, review |
| `verification` | verification | agent `e2e-runner` | BOUNDED, WORKFLOW | read-project, run-process | project | empty |
| `completion` | completion | agent `code-reviewer` | WORKFLOW | read-project, run-process, git-local | git-repository | empty |
| `security-review` | none; usable as a checkpoint or Bounded add-on | agent `security-reviewer` | BOUNDED, WORKFLOW | read-project, network-read | project | empty |

Capabilities with no canonical responsibility contain an empty
`responsibilities` array, as permitted by the Task 2 schema. A
Recipe node still requires a non-empty responsibility. This explicitly
represents specialist-only capabilities without turning them into lifecycle
owners.

- [ ] **Step 5: Verify Provider records**

Extend the embed directive to:

~~~go
//go:embed schemas/v1/*.json providers/*.json
var embedded embed.FS
~~~

Run: `go test ./internal/catalog ./internal/builtin`

Expected: PASS with three strictly decoded descriptors, exact sorted IDs, 10
Superpowers Capabilities, 9 Matt Capabilities, and 11 ECC Capabilities.

- [ ] **Step 6: Commit built-in Provider records**

~~~bash
git add internal/assets/providers internal/builtin/load_test.go
git commit -m "feat: add built-in provider descriptors"
~~~

### Task 4: Add built-in Profile Recipes and compatibility aliases

**Files:**
- Modify: `internal/assets/embed.go`
- Create: `internal/assets/recipes/oaw-delivery.json`
- Create: `internal/assets/recipes/oaw-domain-engineering.json`
- Create: `internal/assets/recipes/oaw-ecc-engineering.json`
- Create: `internal/assets/recipes/oaw-reliable-feature.json`
- Create: `internal/assets/recipes/oaw-hardening.json`
- Create: `internal/assets/profile-aliases.json`
- Modify: `internal/builtin/load_test.go`

- [ ] **Step 1: Write failing Recipe and alias tests**

Validate every Recipe with `schema.ProfileRecipeV1` and the alias set with
`schema.ProfileAliasSetV1` before strict decoding. Assert these sorted Recipe
IDs:

~~~go
[]string{
    "oaw/delivery",
    "oaw/domain-engineering",
    "oaw/ecc-engineering",
    "oaw/hardening",
    "oaw/reliable-feature",
}
~~~

Assert the exact alias map:

~~~go
map[string]string{
    "SP-FULL":         "oaw/delivery",
    "MATT-FULL":       "oaw/domain-engineering",
    "ECC-FULL":        "oaw/ecc-engineering",
    "MATT-SP-HYBRID":  "oaw/reliable-feature",
}
~~~

Assert no alias named `CUSTOM-LOCKED` exists.

- [ ] **Step 2: Run the Recipe tests to verify RED**

Run: `go test ./internal/builtin`

Expected: FAIL because the five Recipe records and alias set are absent.

- [ ] **Step 3: Use one exact canonical responsibility set**

Full-family Recipes require this canonical responsibility set:

~~~text
requirements
domain-modeling
specification
ticket-decomposition
implementation-planning
workspace
delegation
implementation
tdd
functional-debugging
build-repair
dependency-repair
type-repair
review
remediation
verification
completion
~~~

Each responsibility appears exactly once among non-optional Recipe nodes.
Recipe version is `1.0.0`. Every Recipe uses this normal control
cycle:

~~~text
requirements
  -> domain-modeling
  -> specification
  -> ticket-decomposition
  -> implementation-planning
  -> workspace
  -> implementation
       procedures: delegation, tdd
  -> review
       succeeded -> verification
       finding   -> remediation -> review
  -> verification
       succeeded -> completion
       finding   -> remediation
~~~

Expected RED and the red-green loop remain inside the TDD Procedure and do not
emit a graph incident. Functional, build, dependency, and type incidents route
through the Recipe-level
`incident_routes` table. Each incident-handler transitions on
`succeeded` to remediation, which returns on `remediated` to
review. Completion is a gate node with no outgoing transition and is the
terminal gate. Every Recipe exposes these stable boundaries:

~~~text
specification-approved
ticket-complete
tdd-cycle-complete
debugging-cycle-complete
review-complete
verification-complete
~~~

- [ ] **Step 4: Author the three complete single-Provider Recipes**

For `oaw/delivery`, bind every responsibility to
`oaw/superpowers` using the Capability that declares it in Task 3.
For `oaw/domain-engineering`, bind every responsibility to
`oaw/matt`. For `oaw/ecc-engineering`, bind every
responsibility to `oaw/ecc`.

Use phase nodes for requirements through verification, a gate node for
completion, procedure nodes for TDD and delegation attached to the
implementation phase, and incident-handler nodes for functional, build,
dependency, and type debugging. Encode every arrow from Step 3 as an explicit
signal/target transition. Add one unique
`incident_routes` record for each normalized incident type and require
its handler target to have kind `incident-handler`.

- [ ] **Step 5: Author oaw/reliable-feature with explicit hybrid ownership**

Use this exact ownership map:

| Responsibility | Provider / Capability |
| --- | --- |
| requirements, domain-modeling, specification | `oaw/matt / specification` |
| ticket-decomposition | `oaw/matt / tickets` |
| implementation-planning | `oaw/superpowers / implementation-planning` |
| workspace | `oaw/superpowers / workspace` |
| implementation, delegation | `oaw/superpowers / implementation` |
| tdd | `oaw/matt / tdd` |
| functional-debugging | `oaw/matt / debugging` |
| review | `oaw/superpowers / review` |
| remediation | `oaw/superpowers / remediation` |
| verification | `oaw/superpowers / verification` |
| completion | `oaw/superpowers / completion` |

Build, dependency, and type repair are optional incident-handler nodes bound to
`oaw/ecc / build-repair`. They are not required responsibilities for
Recipe availability. A compiler in Ticket 04 either resolves those optional
nodes or removes them and emits the explicit pause/escalation route; it never
substitutes another Provider silently.

- [ ] **Step 6: Author oaw/hardening as a distinct composition**

Use Matt specification and ticketing, Superpowers planning/workspace/
implementation/remediation/verification/completion, and ECC functional-debug,
build-repair, review, and security-review nodes. Security review is a checkpoint
with an empty lifecycle responsibility after implementation and again after
remediation; it transitions on `succeeded` toward review and on
`finding` toward remediation. It is not a lifecycle-selection rule.
This Recipe has its own ID and never aliases `ECC-FULL`.

- [ ] **Step 7: Author the alias-set record**

Create exactly:

~~~json
{
  "schema_version": "oaw.profile-alias-set/v1",
  "aliases": [
    {"alias": "ECC-FULL", "recipe_id": "oaw/ecc-engineering"},
    {"alias": "MATT-FULL", "recipe_id": "oaw/domain-engineering"},
    {"alias": "MATT-SP-HYBRID", "recipe_id": "oaw/reliable-feature"},
    {"alias": "SP-FULL", "recipe_id": "oaw/delivery"}
  ]
}
~~~

- [ ] **Step 8: Embed and verify Recipe and alias assets**

Extend the embed directive to its final Ticket 01 form:

~~~go
//go:embed schemas/v1/*.json providers/*.json recipes/*.json profile-aliases.json
var embedded embed.FS
~~~

Run: `go test ./internal/catalog ./internal/builtin`

Expected: PASS with five sorted Recipes, four sorted aliases, complete
single-Provider coverage for Superpowers, Matt, and ECC, and no
`CUSTOM-LOCKED` record.

- [ ] **Step 9: Commit Recipes and aliases**

~~~bash
git add internal/assets/recipes internal/assets/profile-aliases.json internal/builtin/load_test.go
git commit -m "feat: add built-in profile recipes"
~~~

### Task 5: Construct and validate the immutable built-in Catalog

**Files:**
- Create: `internal/catalog/catalog.go`
- Create: `internal/catalog/validate.go`
- Create: `internal/catalog/catalog_test.go`
- Create: `internal/builtin/load.go`
- Modify: `internal/builtin/load_test.go`

- [ ] **Step 1: Write failing cross-record invariant tests**

Starting from valid minimal fixtures, make one mutation per test and assert the
stable error code:

| Mutation | Expected code |
| --- | --- |
| duplicate Provider ID | `DUPLICATE_PROVIDER_ID` |
| duplicate Capability ID inside one Provider | `DUPLICATE_CAPABILITY_ID` |
| duplicate discovery probe ID inside one Provider | `DUPLICATE_DISCOVERY_PROBE_ID` |
| probe payload does not match its exact kind | `DISCOVERY_PROBE_SHAPE_INVALID` |
| probe contains an unsafe relative path | `DISCOVERY_PATH_INVALID` |
| delegation allow-list names a missing local Capability | `DELEGATION_CAPABILITY_NOT_FOUND` |
| duplicate Host Binding inside one Capability | `DUPLICATE_HOST_BINDING` |
| missing selector Provider | `RECIPE_PROVIDER_NOT_FOUND` |
| missing selector Capability | `RECIPE_CAPABILITY_NOT_FOUND` |
| Capability does not declare node responsibility | `CAPABILITY_RESPONSIBILITY_MISMATCH` |
| required responsibility has no owner | `RESPONSIBILITY_OWNER_MISSING` |
| required responsibility has two non-optional owners | `RESPONSIBILITY_OWNER_DUPLICATE` |
| duplicate Recipe node ID | `DUPLICATE_RECIPE_NODE_ID` |
| entry or transition references a missing node | `RECIPE_NODE_NOT_FOUND` |
| duplicate signal on one node | `DUPLICATE_TRANSITION_SIGNAL` |
| procedure omits or misidentifies its containing phase | `PROCEDURE_PHASE_INVALID` |
| procedure contains a graph transition | `PROCEDURE_TRANSITION_FORBIDDEN` |
| duplicate normalized incident route | `DUPLICATE_INCIDENT_ROUTE` |
| incident route target is not an incident-handler | `INCIDENT_HANDLER_INVALID` |
| terminal gate does not name a gate node | `TERMINAL_GATE_INVALID` |
| alias target missing | `ALIAS_RECIPE_NOT_FOUND` |
| duplicate alias | `DUPLICATE_PROFILE_ALIAS` |

Add a defensive-copy test: construct a Catalog, mutate every source slice and
every slice returned by an accessor, then prove a fresh accessor still returns
the original records.

- [ ] **Step 2: Run catalog validation tests to verify RED**

Run: `go test ./internal/catalog`

Expected: FAIL because the Catalog constructor and validator do not exist.

- [ ] **Step 3: Implement a single validated constructor**

Expose:

~~~go
func New(
    providers []ProviderDescriptorRecord,
    recipes []ProfileRecipeRecord,
    aliases []ProfileAliasRecord,
) (Catalog, error)
~~~

The constructor:

1. deep-copies all records;
2. validates record-local invariants;
3. builds local ID indexes;
4. validates all Recipe selectors, delegation references, and responsibility ownership;
5. validates phase/procedure attachment, graph transitions, incident routes,
   terminal gates, and aliases;
6. sorts copies by canonical ID;
7. computes SHA-256 over canonical JSON of the sorted snapshot; and
8. stores only private slices and the digest.

Expose accessors `Providers()`, `Recipes()`,
`Aliases()`, and `Digest()`. Every slice accessor returns a
new deep copy. Do not expose maps or mutable internal slices.

- [ ] **Step 4: Load built-ins through strict decoders**

Implement:

~~~go
func Load() (catalog.Catalog, error)
~~~

Read sorted asset paths from `assets.FS()`, construct one schema Registry,
validate and strictly decode each Provider and Recipe independently, validate
and decode the single alias set, and call `catalog.New`. Wrap errors with the
asset path and preserve the stable reason code. Do not inspect the working
directory, user home, XDG directories, environment variables, or current Host.

- [ ] **Step 5: Lock deterministic digest and order behavior**

In tests, call `builtin.Load()` twice and assert identical digest,
Provider order, Recipe order, alias order, and JSON representation. Do not pin a
literal digest in source; pin semantic inputs and equality so intentional asset
changes do not require an unexplained magic-value edit.

- [ ] **Step 6: Run catalog tests to verify GREEN**

Run: `gofmt -w internal/catalog/*.go internal/builtin/*.go && go test ./internal/catalog ./internal/builtin`

Expected: PASS for every local and cross-record invariant, stable ordering,
defensive copies, and repeatable digest.

- [ ] **Step 7: Commit Catalog construction**

~~~bash
git add internal/catalog internal/builtin
git commit -m "feat: validate the built-in catalog"
~~~

### Task 6: Expose deterministic catalog inspection

**Files:**
- Create: `internal/cli/run.go`
- Create: `internal/cli/catalog.go`
- Create: `internal/cli/catalog_test.go`
- Create: `cmd/oaw/main.go`

- [ ] **Step 1: Write failing CLI seam tests**

Test through:

~~~go
func Run(args []string, stdout io.Writer, stderr io.Writer) int
~~~

Keep the production seam closed and the test seam package-private:

~~~go
type catalogLoader func() (catalog.Catalog, error)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
    return run(args, stdout, stderr, builtin.Load)
}

func run(args []string, stdout io.Writer, stderr io.Writer, load catalogLoader) int
~~~

Cover help, unknown commands, missing list kind, unknown format, all three text
lists, all three JSON lists, validation text/JSON, deterministic repeated
output, and an injected invalid-Catalog loader returning exit `65`.
The exact successful text is:

~~~text
provider oaw/ecc version=1.0.0 capabilities=11
provider oaw/matt version=1.0.0 capabilities=9
provider oaw/superpowers version=1.0.0 capabilities=10
~~~

~~~text
recipe oaw/delivery version=1.0.0
recipe oaw/domain-engineering version=1.0.0
recipe oaw/ecc-engineering version=1.0.0
recipe oaw/hardening version=1.0.0
recipe oaw/reliable-feature version=1.0.0
~~~

~~~text
alias ECC-FULL recipe=oaw/ecc-engineering
alias MATT-FULL recipe=oaw/domain-engineering
alias MATT-SP-HYBRID recipe=oaw/reliable-feature
alias SP-FULL recipe=oaw/delivery
~~~

~~~text
catalog valid providers=3 recipes=5 aliases=4
~~~

- [ ] **Step 2: Run CLI tests to verify RED**

Run: `go test ./internal/cli`

Expected: FAIL because the CLI runner does not exist.

- [ ] **Step 3: Implement closed parsing and stable diagnostics**

Parse only the Ticket 01 surface. Accept `--format text`,
`--format=text`, `--format json`, and
`--format=json` exactly once. Reject every other option with:

~~~text
oaw: INVALID_ARGUMENT: unknown format "yaml"
~~~

Use the same `oaw: INVALID_ARGUMENT: ` prefix with a concrete parser reason
for every syntax failure. Print usage to stderr for exit `64`. Load the
Catalog only after syntax succeeds. The injected duplicate-Provider fixture
must render:

~~~text
oaw: CATALOG_INVALID: DUPLICATE_PROVIDER_ID: oaw/ecc
~~~

- [ ] **Step 4: Render text and JSON without maps**

Text uses the exact lines from Step 1. JSON uses typed response structs so
field order and shape are intentional:

~~~go
type catalogListResponse struct {
    SchemaVersion string `json:"schema_version"`
    Kind          string `json:"kind"`
    Digest        string `json:"digest"`
    Items         any    `json:"items"`
}

type catalogValidationResponse struct {
    SchemaVersion string `json:"schema_version"`
    Valid         bool   `json:"valid"`
    Digest        string `json:"digest"`
    Providers     int    `json:"providers"`
    Recipes       int    `json:"recipes"`
    Aliases       int    `json:"aliases"`
}
~~~

Set `schema_version` to `oaw.catalog-output/v1`, disable
HTML escaping, and end every JSON response with one newline. Never include
absolute discovery evidence, environment values, or raw Host state.

- [ ] **Step 5: Add the process entrypoint**

Create:

~~~go
package main

import (
    "os"

    "github.com/wifibaby4u/open-agent-workflow/internal/cli"
)

func main() {
    os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
~~~

- [ ] **Step 6: Verify CLI behavior**

Run:

~~~bash
gofmt -w cmd/oaw/*.go internal/cli/*.go
go test ./internal/cli
go run ./cmd/oaw catalog validate
go run ./cmd/oaw catalog list providers --format json
~~~

Expected: tests PASS; validation prints
`catalog valid providers=3 recipes=5 aliases=4`; JSON stdout decodes
as one `oaw.catalog-output/v1` object with three sorted Provider
items; stderr is empty.

- [ ] **Step 7: Commit catalog inspection**

~~~bash
git add cmd/oaw internal/cli
git commit -m "feat: expose built-in catalog inspection"
~~~

### Task 7: Verify the Ticket 01 vertical slice and preserve Bash authority

**Files:**
- Modify only if a failing check requires a Ticket 01 correction: files created in Tasks 1-6

- [ ] **Step 1: Run formatting and static checks**

Run:

~~~bash
test -z "$(gofmt -l cmd/oaw/*.go internal/*/*.go)"
go vet ./...
git diff --check
~~~

Expected: all commands exit `0`; `gofmt` and
`git diff --check` print nothing.

- [ ] **Step 2: Run unit tests with the race detector**

Run: `go test -race ./...`

Expected: every Go package reports `ok` or has no test files; no race
is reported.

- [ ] **Step 3: Enforce repository-wide Go statement coverage**

Run:

~~~bash
coverage_file=$(mktemp /tmp/oaw-ticket-01-cover.XXXXXX)
go test -coverprofile="$coverage_file" ./...
go tool cover -func="$coverage_file"
go tool cover -func="$coverage_file" | awk '/^total:/ { gsub("%", "", $3); if (($3 + 0) < 80) exit 1 }'
~~~

Expected: tests PASS and the final `total` statement coverage is at
least `80.0%`. Keep the generated file outside the repository.

- [ ] **Step 4: Prove deterministic CLI output**

Run:

~~~bash
first_output=$(go run ./cmd/oaw catalog list recipes --format json)
second_output=$(go run ./cmd/oaw catalog list recipes --format json)
test "$first_output" = "$second_output"
go run ./cmd/oaw catalog validate --format json
~~~

Expected: equality exits `0`; validation emits one JSON object with
`valid: true`, 3 Providers, 5 Recipes, 4 aliases, and a non-empty
SHA-256 digest.

- [ ] **Step 5: Re-run the authoritative Bash and documentation suites**

Run:

~~~bash
bash scripts/check-docs.sh
bash tests/run.sh
~~~

Expected: documentation checks pass and the Bash runner ends with
`PASS: all implemented installer cases passed`. No existing Bash
output, status code, installed policy, or state behavior changes.

- [ ] **Step 6: Review the final diff for scope**

Run:

~~~bash
git status --short
git cat-file -e '17893ef^{commit}'
git diff --stat 17893ef
forbidden_tracked=$(git diff --name-only 17893ef -- install.sh lib tests)
forbidden_untracked=$(git ls-files --others --exclude-standard -- lib tests)
test -z "$forbidden_tracked$forbidden_untracked"
~~~

Expected: the approved ticket-decomposition baseline exists; the
two forbidden-surface variables are empty, proving no committed, staged,
unstaged, or untracked Ticket 01 change appears in `install.sh`, `lib/`, or
the Bash test tree. Only Go contracts/assets/CLI and the approved lifecycle
artifacts are new or modified. Ignore the pre-existing untracked `.serena/`
directory.

- [ ] **Step 7: Commit final Ticket 01 corrections, if the verification steps changed tracked files**

When and only when Steps 1-6 required a scoped correction:

~~~bash
git add go.mod cmd internal
git commit -m "test: verify runtime catalog foundation"
~~~

If verification required no tracked correction, do not create an empty commit.

## Self-Review Record

- Spec sections 7 and 9 are covered by Provider, Capability, Recipe, alias, and
  full-family contracts.
- Spec section 15 is covered by the Go module, domain-oriented internal
  packages, strict DTO decoding, copied collections, and embedded repository
  assets.
- Spec section 16 is covered only for the Ticket 01 catalog surface; Runtime,
  discovery, Profile validation against Provider Instances, trust, and
  installation commands remain in their approved later tickets.
- Spec section 17 is respected because Bash remains authoritative and no
  installer command changes.
- Spec section 18 is covered by TDD, race, vet, coverage, deterministic CLI,
  Bash regression, and documentation checks proportionate to this ticket.
- Ticket 01 does not execute discovery probes, verify Provider Instances,
  compile Recipes, select a Runtime Host, issue Grants, persist Runtime State,
  change Policy, or port Bash management behavior.
- No implementation step contains an unresolved placeholder; every contract,
  path, command, expected result, and commit boundary is explicit.
