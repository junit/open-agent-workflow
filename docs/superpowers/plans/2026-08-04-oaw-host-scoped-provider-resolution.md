# OAW Host-Scoped Provider Resolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Provider discovery, pins, Host Binding verification, Registry resolution, Profile compilation, and Runtime Bundles strictly scoped to the selected Host while preserving dynamic third-party Provider and Profile loading.

**Architecture:** Introduce Host-aware Provider Descriptor v2 records and Host-scoped Discovery Reports. A Host Adapter produces independent Binding Inventory evidence; Registry resolution accepts only exact Host/Installation/Binding matches. Runtime and CLI share one input assembly, while foreign-Host discovery is diagnostic-only and never enters authority state.

**Tech Stack:** Go 1.26, embedded Draft 2020-12 JSON schemas, BurntSushi TOML, canonical JSON/SHA-256 digests, table-driven Go tests, race tests, Bash black-box tests, and Docker Linux/arm64 smoke verification.

---

**Design source:** [2026-08-04-oaw-host-scoped-provider-resolution-design.md](../specs/2026-08-04-oaw-host-scoped-provider-resolution-design.md)

**Execution choice:** Prior user direction disables subagent execution. Use `superpowers:executing-plans` inline in a fresh implementation worktree based on this plan commit, which descends from design commit `f088572`; do not dispatch implementation subagents. Keep the existing dirty files in the primary worktree untouched.

## File Map

| Unit | Files | Responsibility |
| --- | --- | --- |
| Schema contracts | `internal/assets/schemas/v2/provider-descriptor.schema.json`, `internal/assets/schemas/v2/user-config.schema.json`, `internal/schema/registry.go`, `internal/schema/registry_test.go`, `internal/assets/embed.go` | Compile and validate active v2 Descriptor and user configuration contracts. |
| Catalog/discovery | `internal/catalog`, `internal/discovery`, `internal/assets/providers`, `internal/builtin` | Represent Host-aware probes and produce Host-scoped Distribution and Installation evidence. |
| User configuration | `internal/config` and configuration tests | Load Host-scoped pins, preferences, and installation hints into immutable snapshots. |
| Host observation | `internal/host/bindings.go`, `internal/host/codex/inventory.go`, `internal/hosttest/provider.go` | Observe actual Codex skills/agents/tools and normalize Binding Inventory evidence. |
| Registry | `internal/registry` and integration tests | Resolve only selected-Host candidates and exact Host Installation bindings. |
| Host/Runtime | `internal/host`, `internal/profile`, `internal/runtime`, `internal/admission` | Require inventory conformance and carry Host scope into graph, Bundle, and state validation. |
| CLI | `internal/cli/provider_inputs.go`, `providers.go`, `run_runtime.go` | Assemble one selected-Host authority path and render current/foreign diagnostics. |
| Documentation/evidence | bilingual README/docs, `policy/ENGINEERING.md`, controlled pilot evidence | Explain the cutover and prove macOS/Docker/pilot behavior. |

## Task 1: Add v2 Schema Contracts

**Files:**
- Create: `internal/assets/schemas/v2/provider-descriptor.schema.json`
- Create: `internal/assets/schemas/v2/user-config.schema.json`
- Modify: `internal/assets/embed.go`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`

- [ ] **Step 1: Write failing schema registration tests.**

Add tests that use the new schema IDs before the resources exist:

```go
func TestRegistryValidatesHostScopedProviderDescriptorV2(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v2","descriptor_version":"2.0.0","id":"acme/suite","display_name":"Acme Suite","discovery":[{"id":"codex","hosts":["codex"],"surface":"codex-skills","distribution":"acme","kind":"path-exists","root":"user-home","candidate_path":".agents/skills/acme","evidence_path":"review/SKILL.md"}],"capabilities":[]}`)
	if err := registry.Validate(ProviderDescriptorV2, raw); err != nil {
		t.Fatalf("Validate(v2 descriptor) error = %v", err)
	}
}

func TestRegistryRejectsV1ProviderDescriptorFromActiveV2Schema(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v1","descriptor_version":"1.0.0","id":"acme/suite","display_name":"Acme Suite","discovery":[],"capabilities":[]}`)
	if err := registry.Validate(ProviderDescriptorV2, raw); err == nil {
		t.Fatal("v1 descriptor unexpectedly validated against v2")
	}
}

func TestRegistryValidatesHostScopedUserConfigV2(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"schema_version":"oaw.user-config/v2","denied_providers":[],"provider_descriptors":[],"profile_recipes":[],"host_integrations":[],"provider_installations":[],"provider_pins":[],"binding_preferences":[],"bounded_capability_defaults":[],"project_trust":[]}`)
	if err := registry.Validate(UserConfigV2, raw); err != nil {
		t.Fatalf("Validate(v2 user config) error = %v", err)
	}
}
```

- [ ] **Step 2: Run focused schema tests and verify RED.**

Run: `go test ./internal/schema -run 'TestRegistryValidatesHostScopedProviderDescriptorV2|TestRegistryRejectsV1ProviderDescriptorFromActiveV2Schema|TestRegistryValidatesHostScopedUserConfigV2' -count=1`

Expected: FAIL because `ProviderDescriptorV2`, `UserConfigV2`, and the v2 embedded resources are undefined.

- [ ] **Step 3: Add v2 schemas and registry resources.**

Define the IDs and resource entries:

```go
const (
	ProviderDescriptorV2 = "https://open-agent-workflow.dev/schemas/v2/provider-descriptor.schema.json"
	UserConfigV2         = "https://open-agent-workflow.dev/schemas/v2/user-config.schema.json"
)

resources := []struct{ path, id string }{
	{"schemas/v2/provider-descriptor.schema.json", ProviderDescriptorV2},
	{"schemas/v2/user-config.schema.json", UserConfigV2},
	{"schemas/v1/provider-descriptor.schema.json", ProviderDescriptorV1},
	{"schemas/v1/user-config.schema.json", UserConfigV1},
	{"schemas/v1/profile-recipe.schema.json", ProfileRecipeV1},
	{"schemas/v1/profile-alias-set.schema.json", ProfileAliasSetV1},
	{"schemas/v1/project-config.schema.json", ProjectConfigV1},
	{"schemas/v1/classification-proposal.schema.json", ClassificationProposalV1},
	{"schemas/v1/host-manifest.schema.json", HostManifestV1},
	{"schemas/v1/host-integration.schema.json", HostIntegrationV1},
	{"schemas/v1/host-integration-set.schema.json", HostIntegrationSetV1},
	{"schemas/v1/runtime-frame.schema.json", RuntimeFrameV1},
	{"schemas/v1/runtime-reply.schema.json", RuntimeReplyV1},
}
```

The two v1 resources remain registered only so this foundation commit keeps
the pre-cutover codebase green. Task 2 deletes ProviderDescriptorV1 after every
Descriptor caller moves to v2; Task 3 deletes UserConfigV1 after every user
configuration caller moves to v2. Neither remains in the final authority path.

The v2 Descriptor schema must be closed and require `hosts`, `surface`, `distribution`, `kind`, and either `candidate_path`/`evidence_path` or `prefix`/`evidence_path`. Keep the current safe relative-path pattern. Remove `all-paths-exist` from the active vocabulary because production Discovery does not implement it.

The v2 user schema must require `provider_installations`. A pin requires `provider_id`, `host_id`, `installation_key`, and `evidence_digest`; an installation hint requires `provider_id`, `host_id`, `surface_id`, `location`, and `discovery_probe_id`; binding preferences use `host_id`.

- [ ] **Step 4: Embed the v2 directory.**

```go
//go:embed schemas/v1/*.json schemas/v2/*.json providers/*.json recipes/*.json profile-aliases.json host-integrations.json
var embedded embed.FS
```

- [ ] **Step 5: Verify and commit the schema foundation.**

Run: `go test ./internal/schema -count=1`

Run: `bash scripts/check-docs.sh`

Expected: PASS.

```bash
git add internal/assets/schemas/v2 internal/assets/embed.go internal/schema/registry.go internal/schema/registry_test.go
git commit -m "feat: add host-scoped provider schemas"
```

## Task 2: Cut Catalog and Discovery Over to Host-aware Descriptors

**Files:**
- Modify: `internal/catalog/records.go`, `decode.go`, `validate.go`, `catalog.go`
- Modify: `internal/discovery/records.go`, `discover.go`, `discovery_test.go`
- Modify: `internal/assets/providers/oaw-superpowers.json`, `oaw-matt.json`, `oaw-ecc.json`
- Modify: `internal/builtin/load.go`, `internal/management/providers.go`
- Modify: catalog, registry, runtime, config, and integration descriptor fixtures
- Delete: `internal/assets/schemas/v1/provider-descriptor.schema.json`

- [ ] **Step 1: Add failing Host isolation tests.**

```go
func TestDiscoverScopesCandidatesToSelectedHost(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".codex/skills/acme/review/SKILL.md", "codex")
	writeFile(t, home, ".claude/skills/acme/review/SKILL.md", "claude")
	value := testCatalog(t,
		catalog.DiscoveryProbe{ID: "codex", Hosts: []string{"codex"}, Surface: "codex-skills", Distribution: "acme", Kind: "path-exists", Root: "user-home", CandidatePath: ".codex/skills/acme", EvidencePath: "review/SKILL.md"},
		catalog.DiscoveryProbe{ID: "claude", Hosts: []string{"claude"}, Surface: "claude-skills", Distribution: "acme", Kind: "path-exists", Root: "user-home", CandidatePath: ".claude/skills/acme", EvidencePath: "review/SKILL.md"},
	)
	codex, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	got := codex.Candidates("acme/suite")
	if len(got) != 1 || got[0].HostID != "codex" || strings.Contains(got[0].Location, ".claude") {
		t.Fatalf("Codex candidates = %#v", got)
	}
}
```

Add a shared-directory test that calls Discovery once for `codex` and once for `claude` against a probe with both Hosts and asserts different InstallationKeys.

- [ ] **Step 2: Run tests and verify RED.**

Run: `go test ./internal/catalog ./internal/discovery -count=1`

Expected: FAIL because the Host-aware fields and `Options.HostID` do not exist.

- [ ] **Step 3: Replace the active catalog probe record.**

```go
type DiscoveryProbe struct {
	ID            string   `json:"id" toml:"id"`
	Hosts         []string `json:"hosts" toml:"hosts"`
	Surface       string   `json:"surface" toml:"surface"`
	Distribution  string   `json:"distribution" toml:"distribution"`
	Kind          string   `json:"kind" toml:"kind"`
	Root          string   `json:"root" toml:"root"`
	CandidatePath string   `json:"candidate_path,omitempty" toml:"candidate_path"`
	EvidencePath  string   `json:"evidence_path,omitempty" toml:"evidence_path"`
	Prefix        string   `json:"prefix,omitempty" toml:"prefix"`
}
```

Set `ProviderDescriptorSchemaV2 = "oaw.provider-descriptor/v2"`. Validate every Host with `ParseLocalID`; require unique Hosts and non-empty surface/distribution; validate direct/versioned shapes; reject all other kinds; require each Capability binding Host to be declared by at least one probe.

- [ ] **Step 4: Rewrite the built-in discovery blocks.**

Use these exact mappings:

```text
Superpowers
  claude-direct: claude / claude-plugin / checkout / .claude/plugins/superpowers
  codex-direct: codex / codex-plugin / checkout / .codex/plugins/superpowers
  claude-marketplace-checkout: claude / claude-marketplace / marketplace /
    .claude/plugins/marketplaces/superpowers-marketplace
  claude-official-cache: claude / claude-plugin-cache / claude-official /
    .claude/plugins/cache/claude-plugins-official/superpowers
  claude-marketplace-cache: claude / claude-plugin-cache / marketplace /
    .claude/plugins/cache/superpowers-marketplace/superpowers
  codex-curated-cache: codex / codex-plugin-cache / openai-curated /
    .codex/plugins/cache/openai-api-curated/superpowers
  evidence path for every entry: skills/using-superpowers/SKILL.md

Matt
  four codex probes, surface codex-user-skills, distribution matt-skills,
  candidate root .agents/skills, evidence paths to each current skill/SKILL.md.

ECC
  ecc-global-skill: codex / codex-user-skills / ecc-global /
    .agents/skills/everything-claude-code / SKILL.md
  marketplace checkout/cache probes: claude with their current physical roots
    and relative .codex-plugin/plugin.json or skill evidence.
```

Set all built-ins to `oaw.provider-descriptor/v2` and descriptor version `2.0.0`. Do not invent Claude Capability bindings.

- [ ] **Step 5: Implement Host-scoped records and keys.**

```go
type Options struct {
	HostID               string
	UserHome             string
	MaximumEvidenceBytes int64
	Installations        []InstallationHint
}

type InstallationHint struct {
	ProviderID       string
	HostID           string
	SurfaceID        string
	Location         string
	DiscoveryProbeID string
}
```

Candidate and Evidence records must include `HostID`, `SurfaceID`, `DistributionKey`, and `InstallationKey`. Replace `Candidate.Key` and `Evidence.CandidateKey` with InstallationKey.

Set the active record names to `oaw.discovery-evidence/v2` and
`oaw.discovery-report/v2`. Report stores the selected Host ID, exposes
`HostID() string`, and includes Host ID in its canonical digest.

Derive identities from canonical JSON:

```go
func deriveInstallationKey(hostID, surfaceID, distributionKey string) string {
	digest, _, _ := canonicaljson.Digest(struct {
		HostID string `json:"host_id"`
		SurfaceID string `json:"surface_id"`
		DistributionKey string `json:"distribution_key"`
	}{hostID, surfaceID, distributionKey})
	return "installation-" + digest
}
```

DistributionKey must digest Provider ID, distribution, physical location, version, and EvidenceDigest. Report digests include HostID and all candidates.

- [ ] **Step 6: Implement direct, versioned, and explicit-hint Discovery.**

Reject empty Host ID. Execute only probes listing the selected Host. Direct probes resolve CandidatePath under the root and EvidencePath under the candidate. Versioned probes enumerate immediate children of Prefix and read EvidencePath under each child. Explicit hints locate the named Provider/Probe, require Host membership, canonicalize the configured location, and evaluate the same relative EvidencePath. Preserve current symlink containment, regular-file, UTF-8/control-character, and 4 MiB evidence limits.

- [ ] **Step 7: Update compatibility diagnostics and all Descriptor fixtures.**

Change `internal/management/providers.go` to use CandidatePath/EvidencePath and Prefix/EvidencePath. Migrate raw descriptors in catalog, config, registry, runtime, and integration tests to v2. Switch `builtin.Load` and `config.DecodeProvider` to `schema.ProviderDescriptorV2`. Remove `ProviderDescriptorV1` registration and delete the v1 Provider schema file after `rg 'ProviderDescriptorV1|oaw.provider-descriptor/v1' internal` returns no authority-path matches.

- [ ] **Step 8: Verify and commit Descriptor/Discovery cutover.**

Run: `go test ./internal/catalog ./internal/discovery ./internal/builtin ./internal/management -count=1`

Expected: PASS for Host isolation, shared physical installation separation, deterministic keys, containment, bounded evidence, and v1 rejection.

```bash
git add internal/catalog internal/discovery internal/assets/providers internal/builtin internal/management internal/config internal/registry internal/runtime internal/integration internal/assets/schemas/v1/provider-descriptor.schema.json
git commit -m "feat: scope provider discovery to host installations"
```

## Task 3: Add Host-scoped User Configuration and Pins

**Files:**
- Modify: `internal/config/records.go`, `decode.go`, `snapshot.go`, `config.go`
- Modify: `internal/assets/schemas/v2/user-config.schema.json`, `internal/schema/registry.go`
- Modify: config, registry, runtime, integration, and hosttest user-config fixtures
- Delete: `internal/assets/schemas/v1/user-config.schema.json`

- [ ] **Step 1: Add failing Host pin tests.**

```go
func TestDecodeUserAcceptsIndependentHostPins(t *testing.T) {
	raw := []byte(`schema_version = "oaw.user-config/v2"

[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-codex"
evidence_digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "claude"
installation_key = "installation-claude"
evidence_digest = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
`)
	decoded, err := DecodeUser(raw, testSchemaRegistry(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Record.ProviderPins) != 2 || decoded.Record.ProviderPins[0].HostID != "claude" || decoded.Record.ProviderPins[1].HostID != "codex" {
		t.Fatalf("pins = %#v", decoded.Record.ProviderPins)
	}
}
```

Add cases for duplicate same-Host pins, missing EvidenceDigest, unsafe installation location, duplicate installation identity, and a binding preference using `host` instead of `host_id`.

- [ ] **Step 2: Run tests and verify RED.**

Run: `go test ./internal/config -run 'TestDecodeUserAcceptsIndependentHostPins' -count=1`

Expected: FAIL because v2 record fields are absent.

- [ ] **Step 3: Replace records and normalize Host keys.**

```go
type ProviderPin struct {
	ProviderID      string `json:"provider_id" toml:"provider_id"`
	HostID          string `json:"host_id" toml:"host_id"`
	InstallationKey string `json:"installation_key" toml:"installation_key"`
	EvidenceDigest  string `json:"evidence_digest" toml:"evidence_digest"`
	Location        string `json:"location,omitempty" toml:"location"`
	Version         string `json:"version,omitempty" toml:"version"`
}

type ProviderInstallation struct {
	ProviderID       string `json:"provider_id" toml:"provider_id"`
	HostID           string `json:"host_id" toml:"host_id"`
	SurfaceID        string `json:"surface_id" toml:"surface_id"`
	Location         string `json:"location" toml:"location"`
	DiscoveryProbeID string `json:"discovery_probe_id" toml:"discovery_probe_id"`
}
```

Rename BindingPreference.Host to HostID/`host_id`. Add ProviderInstallations to UserConfigRecord. Set `UserConfigSchemaV2`, reject every other user schema, and validate through `schema.UserConfigV2`.

- [ ] **Step 4: Validate and sort configuration.**

Require a qualified Provider ID, local Host ID, non-empty InstallationKey, and 64-hex EvidenceDigest for pins. Require local Surface/Probe IDs and an absolute clean location for installation hints. Sort pins by ProviderID+HostID, preferences by ProviderID+HostID+CapabilityID, and hints by ProviderID+HostID+SurfaceID+Location. Reject duplicate keys. Location/version pin assertions are optional but must be clean when present.

- [ ] **Step 5: Build immutable settings per Provider/Host.**

Add HostID to ProviderSettings. Collect known Hosts from probes, bindings, pins, preferences, and hints; create one setting per `(ProviderID, HostID)`; apply global denial/project limits to each; attach only same-Host pins/preferences. Sort settings by the combined key and expose:

```go
func (snapshot Snapshot) ProviderSettings(providerID, hostID string) ProviderSettings
func (snapshot Snapshot) ProviderInstallations() []ProviderInstallation
```

Include settings and installations in SnapshotRecord/digests. Set the active snapshot record to `oaw.configuration-snapshot/v2`.

- [ ] **Step 6: Migrate user config fixtures and remove v1 support.**

Change every fixture under `internal/config`, `internal/registry`, `internal/runtime`, `internal/integration`, and `internal/hosttest` to `oaw.user-config/v2`. Pins use all four identity fields; binding preferences use `host_id`. Switch schema registration to UserConfigV2 and delete the v1 user schema file after `rg 'UserConfigV1|oaw.user-config/v1' internal` contains only explicit rejection tests.

- [ ] **Step 7: Verify and commit Host-scoped configuration.**

Run: `go test ./internal/config ./internal/schema ./internal/hosttest -count=1`

Expected: PASS for Host pin coexistence, duplicate rejection, immutable snapshots, project trust, installation hints, and old-schema rejection.

```bash
git add internal/config internal/schema internal/assets/schemas internal/registry internal/runtime internal/integration internal/hosttest
git commit -m "feat: add host-scoped provider configuration"
```

## Task 4: Add Host Binding Inventory and the Codex Observer

**Files:**
- Create: internal/host/bindings.go
- Create: internal/host/bindings_test.go
- Create: internal/host/codex/inventory.go
- Create: internal/host/codex/inventory_test.go
- Create: internal/hosttest/provider.go
- Modify: internal/hosttest/fixture.go

- [ ] **Step 1: Write failing normalized-inventory tests.**

Add this record-level test:

~~~go
func TestNewBindingInventoryPinsHostInstallationAndEvidence(t *testing.T) {
	observations := []host.BindingObservation{{
		HostID:            "codex",
		InstallationKey:   "installation-acme",
		Binding:           catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:review"},
		Source:            "host-filesystem",
		EvidenceReference: "/host/acme/skills/review/SKILL.md",
		Digest:            strings.Repeat("a", 64),
	}}
	first, err := host.NewBindingInventory("codex", observations)
	if err != nil {
		t.Fatal(err)
	}
	second, err := host.NewBindingInventory("codex", append([]host.BindingObservation{}, observations...))
	if err != nil {
		t.Fatal(err)
	}
	if first.HostID != "codex" || first.Digest == "" || first.Digest != second.Digest || len(first.Observations) != 1 {
		t.Fatalf("inventories = %#v / %#v", first, second)
	}
	observations[0].Binding.Reference = "changed"
	if first.Observations[0].Binding.Reference != "acme:review" {
		t.Fatal("BindingInventory shares caller storage")
	}
}
~~~

Add table cases that reject an empty or invalid Host ID, a Binding Host different from the inventory Host, an empty InstallationKey, unsupported Source, unsafe EvidenceReference, invalid digest, and duplicate Host/Installation/Binding observations.

- [ ] **Step 2: Write failing Codex filesystem-observation tests.**

Build one v2 test Descriptor and discover it on a temporary Codex Home. Assert these cases:

~~~go
func TestObserveBindingsRequiresPhysicalCodexEvidence(t *testing.T) {
	fixture := newCodexInventoryFixture(t)
	inventory, err := codex.ObserveBindings(
		fixture.Catalog,
		fixture.Discovery,
		codex.InventoryOptions{UserHome: fixture.Home, CodexConfigRoot: fixture.CodexRoot},
	)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.HostID != "codex" || len(inventory.Observations) != 1 {
		t.Fatalf("inventory = %#v", inventory)
	}
	observation := inventory.Observations[0]
	if observation.InstallationKey != fixture.InstallationKey ||
		observation.Binding != (catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:review"}) ||
		observation.Source != "host-filesystem" ||
		observation.Digest == "" {
		t.Fatalf("observation = %#v", observation)
	}
}
~~~

The fixture must place the discovery marker and acme/review/SKILL.md beneath the same Candidate root. Add negative cases for a Descriptor declaration with no file, a matching skill file outside the Candidate root, a symlink escape, an oversized file, a malformed Codex agent registry entry, an agent config outside the Candidate, and a tool binding with no Host-native observation. Add an ordering test whose repeated inventory digests match.

- [ ] **Step 3: Run the focused tests and verify RED.**

Run: go test ./internal/host ./internal/host/codex -run 'BindingInventory|ObserveBindings' -count=1

Expected: FAIL because BindingObservation, BindingInventory, NewBindingInventory, InventoryOptions, and ObserveBindings do not exist.

- [ ] **Step 4: Implement immutable Host inventory records.**

Create these records in internal/host/bindings.go:

~~~go
const BindingInventorySchemaV1 = "oaw.host-binding-inventory/v1"

type BindingObservation struct {
	HostID            string              `json:"host_id"`
	InstallationKey   string              `json:"installation_key"`
	Binding           catalog.HostBinding `json:"binding"`
	Source            string              `json:"source"`
	EvidenceReference string              `json:"evidence_reference"`
	Digest            string              `json:"digest"`
}

type BindingInventory struct {
	HostID       string               `json:"host_id"`
	Observations []BindingObservation `json:"observations"`
	Digest       string               `json:"digest"`
}
~~~

NewBindingInventory validates Host/local IDs with catalog.ParseLocalID, permits only host-index, host-filesystem, and native-probe Sources, rejects control characters and non-absolute filesystem evidence references, validates 64-hex digests, sorts by Host/Installation/Binding/Source/EvidenceReference, rejects duplicate Host/Installation/Binding tuples, and digests the schema version plus normalized observations. CloneBindingInventory must deep-copy the observation slice.

- [ ] **Step 5: Implement the Codex observer against actual Host surfaces.**

Use this public seam:

~~~go
type InventoryOptions struct {
	UserHome             string
	CodexConfigRoot      string
	MaximumEvidenceBytes int64
}

func ObserveBindings(
	value catalog.Catalog,
	report discovery.Report,
	options InventoryOptions,
) (host.BindingInventory, error)
~~~

Require report.HostID() == "codex". Inspect only current-report Candidates. Catalog bindings are the allow-list of references to observe, never evidence by themselves.

For skill bindings, enumerate the Candidate's actual Codex skill surface and
strictly parse each `SKILL.md` frontmatter name. Match a Descriptor binding
only after an observed skill name exists; the Descriptor is an allow-list, not
the source of the observation. Cover these Codex-native layouts:

~~~text
namespaced reference superpowers:writing-plans
  <candidate>/skills/writing-plans/SKILL.md

bare reference to-spec on a codex-user-skills surface
  <candidate>/to-spec/SKILL.md
~~~

For a namespaced binding, require the suffix after the colon to equal the
observed skill name and derive the namespace from the actual plugin surface.
For a bare binding, require the whole reference to equal the observed skill
name. This is Provider-neutral and must work for the `acme/suite` fixture
without an Acme-specific branch.

For agent bindings, strictly decode the [agents.<reference>] entry in Codex config.toml, resolve its config_file relative to the containing config file, and require the physical config file to be contained by the Candidate root. Do not infer an agent from an unregistered file. Produce no tool observation without a future Host-native registry or explicit native probe.

Reuse the discovery safety limits: clean absolute roots, physical containment, no symlink escape, regular files only, bounded reads, valid UTF-8, and no control characters. EvidenceReference is the canonical physical file path and Digest is the SHA-256 of the observed Host-owned bytes. Pass all observations through host.NewBindingInventory.

- [ ] **Step 6: Add shared Host test fixtures.**

internal/hosttest/provider.go must expose builders that return a v2 Descriptor, a same-Host discovery report, and a matching inventory without copying Descriptor bindings into inventory. The inventory helper writes real fixture files and calls the Host observer. Update existing Host fixtures to use these builders so later Registry and Runtime tests cannot accidentally reintroduce synthetic catalog inventory.

- [ ] **Step 7: Verify and commit Host observation.**

Run: go test ./internal/host ./internal/host/codex ./internal/hosttest -count=1

Expected: PASS for immutability, deterministic digests, exact installation association, containment, bounded evidence, agent registry validation, and declaration-without-evidence rejection.

~~~bash
git add internal/host/bindings.go internal/host/bindings_test.go internal/host/codex/inventory.go internal/host/codex/inventory_test.go internal/hosttest
git commit -m "feat: observe host provider bindings"
~~~

## Task 5: Make Registry Resolution Strictly Host-scoped

**Files:**
- Modify: internal/registry/records.go
- Modify: internal/registry/resolve.go
- Modify: internal/registry/registry.go
- Modify: internal/registry/registry_test.go
- Modify: internal/integration/config_discovery_test.go
- Modify: runtime, integration, and admission Registry fixtures

- [ ] **Step 1: Replace synthetic-inventory tests with exact Host/Installation tests.**

Add a table whose successful row uses one discovered Candidate and this inventory:

~~~go
inventory, err := host.NewBindingInventory("codex", []host.BindingObservation{{
	HostID:            "codex",
	InstallationKey:   candidate.InstallationKey,
	Binding:           catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:review"},
	Source:            "host-filesystem",
	EvidenceReference: filepath.Join(candidate.Location, "skills", "review", "SKILL.md"),
	Digest:            strings.Repeat("a", 64),
}})
if err != nil {
	t.Fatal(err)
}
report, effective, err := registry.Resolve(snapshot, "codex", discovered, &inventory)
~~~

Add failing rows for selected Host/discovery mismatch, selected Host/inventory mismatch, foreign Candidate, nil inventory, empty inventory, wrong InstallationKey, wrong Binding Host, wrong reference, stale evidence pin, and two same-Host candidates. Assert nil or empty inventory uses HOST_BINDING_EVIDENCE_REQUIRED; a non-empty inventory with no exact match uses PROVIDER_BINDING_UNAVAILABLE.

- [ ] **Step 2: Run Registry tests and verify RED.**

Run: go test ./internal/registry ./internal/integration -run 'Resolve|Provider|Host' -count=1

Expected: FAIL because Resolve has no selected Host parameter and Registry/Instance records have no Host identity.

- [ ] **Step 3: Replace Registry records with v2 Host identities.**

Use these active records:

~~~go
type VerifiedCapability struct {
	ID                    string              `json:"id"`
	Binding               catalog.HostBinding `json:"binding"`
	BindingEvidenceDigest string              `json:"binding_evidence_digest"`
}

type ProviderInstance struct {
	ProviderID             string               `json:"provider_id"`
	HostID                 string               `json:"host_id"`
	DescriptorDigest       string               `json:"descriptor_digest"`
	DistributionKey        string               `json:"distribution_key"`
	InstallationKey        string               `json:"installation_key"`
	Location               string               `json:"location"`
	Version                string               `json:"version"`
	ConfigurationDigest    string               `json:"configuration_digest"`
	BindingInventoryDigest string               `json:"binding_inventory_digest"`
	EvidenceDigest         string               `json:"evidence_digest"`
	Capabilities           []VerifiedCapability `json:"capabilities"`
	Digest                 string               `json:"digest"`
}
~~~

ResolutionReport stores HostID, exposes HostID() string, and uses oaw.provider-resolution-report/v2. Provider Instance uses oaw.provider-instance/v2. Registry stores HostID, exposes HostID() string, digests oaw.effective-registry/v2 plus Host ID and providers, and rejects an Instance whose Host differs with HOST_PROVIDER_SCOPE_MISMATCH.

- [ ] **Step 4: Implement the fixed Host-scoped resolution algorithm.**

Change the API to:

~~~go
func Resolve(
	snapshot config.Snapshot,
	hostID string,
	evidence discovery.Report,
	inventory *host.BindingInventory,
) (ResolutionReport, Registry, error)
~~~

Validate hostID, evidence.HostID(), and inventory.HostID before iterating Providers. Read snapshot.ProviderSettings(providerID, hostID). Select only exact-host Candidates; apply the pin by ProviderID, HostID, InstallationKey, and EvidenceDigest, then optional Location/Version assertions. Apply ambiguity only after that isolation.

For the selected Candidate, index observations by InstallationKey plus Binding. Intersect only Descriptor bindings for hostID with observations for selected.InstallationKey. Apply capability limits and Host-scoped preferences after the intersection. Copy each observation Digest into VerifiedCapability.BindingEvidenceDigest. Build the Instance with Candidate Distribution/Installation identity and the complete inventory digest.

- [ ] **Step 5: Preserve diagnostics without granting authority.**

Use these outcomes:

~~~text
zero current Candidate                         not-found / PROVIDER_NOT_FOUND
one Candidate and nil or empty inventory       candidate / HOST_BINDING_EVIDENCE_REQUIRED
multiple current Candidates                    ambiguous / PROVIDER_CANDIDATE_AMBIGUOUS
pin identity or assertion mismatch             incompatible / PROVIDER_PIN_INCOMPATIBLE
non-empty inventory with no exact match        binding-unavailable / PROVIDER_BINDING_UNAVAILABLE
exact Candidate and observed binding subset    verified / PROVIDER_VERIFIED
~~~

ResolutionReport may retain all selected-Host Candidates for inspection. It must never contain a foreign Candidate.

- [ ] **Step 6: Migrate every Registry caller and fixture.**

Replace registry.BindingInventory with host.BindingInventory. Every call passes an explicit Host ID and a discovery Report for that Host. Delete helpers that derive an inventory from catalog Capability declarations, including ticket07Bindings where it is used as authority. Test fixtures must create real BindingObservation values linked to the discovered InstallationKey.

- [ ] **Step 7: Verify and commit Registry resolution.**

Run: go test ./internal/registry ./internal/profile ./internal/admission ./internal/runtime ./internal/integration -count=1

Expected: PASS with no foreign Candidate admission, no stale-pin drift, exact binding evidence, immutable v2 Instances, and deterministic Host-scoped Registry digests.

~~~bash
git add internal/registry internal/profile internal/admission internal/runtime internal/integration internal/hosttest
git commit -m "feat: resolve providers within host scope"
~~~

## Task 6: Require Provider Inventory Conformance on Runtime-managed Hosts

**Files:**
- Modify: internal/host/records.go
- Modify: internal/host/validate.go
- Modify: internal/host/conformance.go
- Modify: internal/host/admission.go
- Modify: internal/host/records_test.go
- Modify: internal/host/conformance_test.go
- Modify: internal/host/conformance_fuzz_test.go
- Modify: internal/host/admission_test.go
- Modify: internal/host/codex_manifest_test.go
- Modify: internal/assets/host-integrations.json
- Modify: internal/assets/schemas/v1/host-manifest.schema.json
- Modify: internal/assets/schemas/v1/host-integration.schema.json
- Modify: internal/config and integration Host fixtures

- [ ] **Step 1: Add failing Feature and conformance tests.**

Add FeatureProviderBindingInventory and CheckProviderBindingInventory. Extend conforming test adapters with:

~~~go
func (conformingAdapter) ObserveProviderBindings(request host.BindingInventoryFixtureRequest) (host.BindingInventory, error) {
	return host.NewBindingInventory(request.HostID, []host.BindingObservation{{
		HostID:            request.HostID,
		InstallationKey:   request.InstallationKey,
		Binding:           request.Binding,
		Source:            "native-probe",
		EvidenceReference: "evidence://host-conformance/provider-binding",
		Digest:            request.EvidenceChallengeDigest,
	}})
}
~~~

Assert that RunConformance fails the new check for a copied declaration, wrong Host, wrong InstallationKey, wrong Binding, or wrong evidence digest. Assert AdmitWorkflow rejects a Runtime-managed integration whose manifest/conformance omits the inventory Feature.

- [ ] **Step 2: Run focused Host tests and verify RED.**

Run: go test ./internal/host ./internal/integration -run 'Conformance|Admission|Manifest|Inventory' -count=1

Expected: FAIL because the inventory Feature, fixture request, and Adapter method do not exist.

- [ ] **Step 3: Extend conformance without weakening exact invocation.**

Add:

~~~go
type BindingInventoryFixtureRequest struct {
	HostID                  string              `json:"host_id"`
	InstallationKey         string              `json:"installation_key"`
	Binding                 catalog.HostBinding `json:"binding"`
	EvidenceChallengeDigest string              `json:"evidence_challenge_digest"`
}

type ProviderBindingObserver interface {
	ObserveProviderBindings(BindingInventoryFixtureRequest) (BindingInventory, error)
}
~~~

RunConformance requires this optional interface only when the Manifest declares provider-binding-inventory. The fixture uses a deterministic InstallationKey and binding, then verifies exactly one matching observation and the challenge digest. Keep exact-binding-invocation as an independent check; passing one cannot satisfy the other.

- [ ] **Step 4: Make managed Host admission require the Feature.**

Add provider-binding-inventory to knownFeatures and known Check IDs. Validation requires every runner-managed or native-managed Manifest to declare the Feature, while instruction-only Hosts retain no Features. RuntimeFrame gains HostID; AdmitWorkflow requires frame.HostID == integration.Manifest.HostID before checking graph bindings.

- [ ] **Step 5: Regenerate the built-in Codex Integration record.**

Add provider-binding-inventory to the Codex Manifest Feature list and a passed matching conformance check. Recompute ManifestDigest, Conformance TranscriptDigest/check evidence/Digest, and Integration Digest through NewManifest, RunConformance, NewConformanceReport, and NewIntegration. Do not hand-edit digest values.

Use a temporary in-repository Go fixture under .scratch/refresh-host-integration that strictly decodes the current set, replaces only oaw/codex-runner with the constructor output, emits JSON, and is removed before commit. Verify the final built-in set still leaves every non-Codex Host instruction-only.

- [ ] **Step 6: Update all conformance adapters and schemas.**

Implement ObserveProviderBindings on conforming, fuzz, hosttest, and integration fixture adapters. Update host-manifest and host-integration schema Feature/Check enums. Update call-count assertions to include one inventory observation and keep raw Host evidence excluded from reports and Runtime replies.

- [ ] **Step 7: Verify and commit Host conformance.**

Run: go test ./internal/host ./internal/config ./internal/integration -run 'Host|Conformance|Inventory' -count=1

Run: go test ./internal/host -run TestBuiltinCodexRuntimeIntegrationIsSelectedAndPinned -count=1

Expected: PASS with the new Feature pinned into the built-in Codex manifest and no promotion of policy-only Hosts.

~~~bash
git add internal/host internal/hosttest internal/config internal/integration internal/assets/host-integrations.json internal/assets/schemas/v1/host-manifest.schema.json internal/assets/schemas/v1/host-integration.schema.json
git commit -m "feat: require host binding inventory conformance"
~~~

## Task 7: Carry Host Scope Through Profiles, Runtime State, and Bundles

**Files:**
- Modify: internal/profile/records.go
- Modify: internal/profile/compile.go
- Modify: internal/profile/validate.go
- Modify: internal/profile/profile_test.go
- Modify: internal/runtime/records.go
- Modify: internal/runtime/workflow_records.go
- Modify: internal/runtime/workflow_start.go
- Modify: internal/runtime/workflow_dispatch.go
- Modify: internal/runtime/workflow_host.go
- Modify: internal/runtime/workflow_validation.go
- Modify: internal/runtime/journal.go
- Modify: internal/runtime/bounded.go
- Modify: Runtime and integration tests

- [ ] **Step 1: Add failing Profile Host-scope tests.**

Extend EffectiveRegistry with HostID() string and add:

~~~go
func TestCompileRecipeRejectsProviderFromAnotherHost(t *testing.T) {
	fixture := profileFixture(t)
	fixture.registry.hostID = "codex"
	instance := fixture.registry.providers["acme/suite"]
	instance.HostID = "claude"
	fixture.registry.providers["acme/suite"] = instance
	_, err := profile.CompileRecipe(fixture.catalog, fixture.registry, fixture.recipe, nil)
	var compileErr *profile.CompileError
	if !errors.As(err, &compileErr) || compileErr.Code != "HOST_PROVIDER_SCOPE_MISMATCH" {
		t.Fatalf("CompileRecipe() error = %v", err)
	}
}
~~~

Add a successful compile assertion that graph.HostID() and every GraphProviderInstance.HostID equal codex.

- [ ] **Step 2: Add failing Runtime Host-lock tests.**

Add tests that create a Codex Registry and then pass a Claude RuntimeFrame. Assert NewEngine or Profile selection returns HOST_PROVIDER_SCOPE_MISMATCH before graph compilation, admission, Grant creation, projection, or journal mutation.

Add a Bundle assertion:

~~~go
if bundle.HostID != "codex" ||
	bundle.Graph.HostID != "codex" ||
	bundle.ProviderInstances[0].HostID != "codex" ||
	bundle.BindingInventoryDigest == "" {
	t.Fatalf("Host-scoped Bundle = %#v", bundle)
}
~~~

Tamper each Host field and BindingInventoryDigest independently and assert validateLifecycleBundle rejects it. Add a stable-boundary switch test proving a Host change cannot reuse the previous Run or append a Bundle generation.

- [ ] **Step 3: Run focused tests and verify RED.**

Run: go test ./internal/profile ./internal/runtime ./internal/integration -run 'Host|Bundle|Profile|Registry' -count=1

Expected: FAIL because EffectiveRegistry, ExecutionGraph, Workflow state, and LifecycleBundle do not carry Host scope.

- [ ] **Step 4: Cut Execution Graph records to v2.**

Use oaw.execution-graph/v2 and add HostID:

~~~go
type GraphProviderInstance struct {
	ProviderID     string `json:"provider_id"`
	HostID         string `json:"host_id"`
	InstanceDigest string `json:"instance_digest"`
}

type ExecutionGraphRecord struct {
	SchemaVersion     string                  `json:"schema_version"`
	HostID            string                  `json:"host_id"`
	RecipeID          string                  `json:"recipe_id"`
	RecipeVersion     string                  `json:"recipe_version"`
	RecipeDigest      string                  `json:"recipe_digest"`
	Entry             string                  `json:"entry"`
	Bindings          []ProfileBinding        `json:"bindings"`
	ProviderInstances []GraphProviderInstance `json:"provider_instances"`
	Nodes             []GraphNode             `json:"nodes"`
	IncidentRoutes    []GraphIncidentRoute    `json:"incident_routes"`
	TerminalGates     []string                `json:"terminal_gates"`
	StableBoundaries  []string                `json:"stable_boundaries"`
	Digest            string                  `json:"digest"`
}
~~~

CompileRecipe validates a non-empty Registry Host, every Provider Instance Host, every verified Binding Host, and every resulting graph node against that Host. HostID participates in graph digesting and validation. Keep Recipe selectors Provider-neutral: they still select provider_id plus capability_id.

- [ ] **Step 5: Cut persisted Workflow authority records to v2.**

Use oaw.lifecycle-bundle/v2 and oaw.runtime-snapshot/v2. Add HostID and BindingInventoryDigest to LifecycleBundle; add HostID to WorkflowState and BoundedState. BoundedOptions gains HostID. WorkflowOptions derives Host scope from Host.RuntimeFrame.HostID and requires it to equal Registry.HostID().

The Bundle constructor copies HostID from the admitted Host and BindingInventoryDigest from the Registry Instances. If a graph uses several Provider Instances, require their BindingInventoryDigest values to be identical because one assembly produced one Host inventory. Include both fields in Bundle seed/digest, cloning, journal validation, projection source records, stable-switch checks, and append-only invariants.

- [ ] **Step 6: Reject stale or foreign Engine inputs before state mutation.**

workflowConfigurationReady and bounded configuration readiness require:

~~~go
options.Registry.HostID() == options.Host.HostID
options.Resolutions.HostID() == options.Host.HostID
~~~

For BOUNDED use options.HostID in the same comparison. A Runtime Protocol-only DIRECT Run may have no Provider Host; any BOUNDED or WORKFLOW Run requires one. Existing v1 persisted snapshots, graphs, or Bundles fail with RUN_STATE_REVISION_INVALID or UNSUPPORTED_SCHEMA_VERSION; no migration path is added.

INSPECT and CONTINUE revalidate the active Bundle Host against the current RuntimeFrame and Registry before issuing a Grant or lease. Foreign diagnostics are not present in WorkflowOptions and therefore cannot enter a snapshot.

- [ ] **Step 7: Update grants and admission consistency checks.**

Admission verifies ProviderInstance.HostID, Registry.HostID(), Graph HostID, node Binding Host, and request Host ID all agree. CapabilityGrant continues to pin ProviderInstanceDigest; no second Host field is needed inside the Grant because Bundle/Registry identity already carries it. Add mismatch tests before IssueBoundedGrant and IssueWorkflowGrant.

- [ ] **Step 8: Verify and commit Host-locked Runtime state.**

Run: go test ./internal/profile ./internal/admission ./internal/runtime ./internal/integration -count=1

Run: go test -race ./internal/runtime ./internal/host ./internal/registry ./internal/profile -count=1

Expected: PASS with Host identity in graph, state, and Bundle digests; mismatches fail before journal changes; stable switching preserves immutable completed generations.

~~~bash
git add internal/profile internal/admission internal/runtime internal/integration
git commit -m "feat: pin runtime authority to host scope"
~~~

## Task 8: Build One CLI Authority Path and Separate Foreign Diagnostics

**Files:**
- Modify: internal/cli/provider_inputs.go
- Modify: internal/cli/provider_inputs_test.go
- Modify: internal/cli/providers.go
- Modify: internal/cli/providers_test.go
- Modify: internal/cli/run_runtime.go
- Modify: internal/cli/run_runtime_test.go
- Modify: internal/cli/run_host_test.go
- Delete: catalogHostBindings from internal/cli/run_runtime.go

- [ ] **Step 1: Add failing selected/foreign assembly tests.**

Create a temporary HOME containing one Codex Superpowers installation and one Claude Superpowers installation. loadProviderInputs for codex must return:

~~~go
if inputs.HostID != "codex" ||
	inputs.Discovery.HostID() != "codex" ||
	inputs.Registry.HostID() != "codex" ||
	inputs.Inventory == nil ||
	len(inputs.Foreign) != 1 ||
	inputs.Foreign[0].HostID != "claude" {
	t.Fatalf("provider inputs = %#v", inputs)
}
for _, candidate := range inputs.Discovery.Candidates("oaw/superpowers") {
	if candidate.HostID != "codex" || strings.Contains(candidate.Location, ".claude") {
		t.Fatalf("current Candidate = %#v", candidate)
	}
}
~~~

Assert the Registry digest and resolution report are identical with IncludeForeignDiagnostics false and true. Add a Claude policy-only inspection case that returns candidates, nil inventory, and an empty Registry instead of PROVIDER_HOST_UNSUPPORTED.

- [ ] **Step 2: Add failing inspection contract tests.**

Update JSON expectations to oaw.provider-inspection/v2. Assert:

~~~text
current_host.host_id = codex
current_host.candidates contain only codex
current_host.observed_bindings contain only codex
foreign_hosts[claude] contains the Claude diagnostic Candidate
foreign_hosts never contain provider_pin
verified Instance Host/Installation/Inventory digests are rendered
~~~

Add a foreign-only case: current resolution remains PROVIDER_NOT_FOUND while the foreign section contains diagnostic_reason = PROVIDER_FOREIGN_HOST_ONLY. Add a text-output test that the exact pin fragment contains provider_id, host_id, installation_key, and evidence_digest.

- [ ] **Step 3: Run focused CLI tests and verify RED.**

Run: go test ./internal/cli -run 'Provider|Inspect|Runtime|Host' -count=1

Expected: FAIL because provider assembly still scans globally, synthesizes catalog bindings, blocks policy-only inspection, and has no foreign projection.

- [ ] **Step 4: Replace providerInputs with selected-Host authority plus optional diagnostics.**

Use:

~~~go
type providerInputOptions struct {
	HostID                    string
	ProjectRoot               string
	UserConfigRoot            string
	UserHome                  string
	IncludeForeignDiagnostics bool
}

type foreignProviderDiscovery struct {
	HostID    string
	Discovery discovery.Report
}

type providerInputs struct {
	HostID            string
	RuntimeManaged    bool
	Configuration     config.Snapshot
	Discovery         discovery.Report
	Inventory         *host.BindingInventory
	Resolutions       registry.ResolutionReport
	Registry          registry.Registry
	Foreign           []foreignProviderDiscovery
	UserConfigPath    string
	UserConfigExists  bool
}
~~~

Load configuration once. Convert snapshot.ProviderInstallations() to discovery.InstallationHint values and pass only same-Host hints to each Discover call. For the selected Host, call the Codex observer only for the admitted Codex Host Adapter. Policy-only Hosts pass nil inventory and can produce diagnostics but never a verified Instance.

Resolve only the selected Discovery/Inventory pair. When IncludeForeignDiagnostics is true, discover every other explicitly declared Host into separate sorted reports after selected resolution. Never pass those reports to Resolve, Profile compilation, Runtime options, pins, or state.

- [ ] **Step 5: Remove runtime eligibility from read-only inspection.**

providers inspect requires a known Host identity, not RuntimeEntrypointAllowed. It may inspect Claude or another policy-only Host. oaw run retains RuntimeEntrypointAllowed and still supports only the selected Codex runner. Set:

~~~go
host.RuntimeFrame{
	HostID:       inputs.HostID,
	IntegrationID: host.SelectedRuntimeIntegrationID,
}
~~~

Pass inputs.HostID to BoundedOptions. Delete catalogHostBindings completely and verify this search is empty:

Run: rg 'catalogHostBindings|Bindings: catalogHost' internal

Expected: no matches.

- [ ] **Step 6: Implement v2 current and foreign inspection projections.**

The current section renders selected Host metadata, discovery/inventory/resolution/Registry digests, observations, current Candidates, and verified Instances. Candidate output includes HostID, SurfaceID, DistributionKey, InstallationKey, Location, Version, and EvidenceDigest.

Generate a pin only for a current Candidate:

~~~go
config.ProviderPin{
	ProviderID:      resolution.ProviderID,
	HostID:          inputs.HostID,
	InstallationKey: candidate.InstallationKey,
	EvidenceDigest:  candidate.EvidenceDigest,
	Location:        candidate.Location,
	Version:         candidate.Version,
}
~~~

The foreign section contains Host ID, report digest, Provider ID, diagnostic reason, and Candidate identity. It renders no inventory, Instance, Registry, or pin.

- [ ] **Step 7: Keep Runtime denials stable and path-free.**

newCLIEngine uses the same selected-Host assembly but sets IncludeForeignDiagnostics false. Convert assembly failures to the stable resolution reason plus:

~~~text
Run oaw providers inspect --host <host> for physical evidence.
~~~

Do not include Candidate paths, config paths, raw Host output, or foreign details in Runtime stdout/stderr. Add tests using a unique secret path and assert it is absent from denial JSON, diagnostics, Runtime State, and projections.

- [ ] **Step 8: Verify and commit CLI authority separation.**

Run: go test ./internal/cli ./internal/runtime ./internal/integration -count=1

Run: go test -race ./internal/cli ./internal/host/codex ./internal/runtime -count=1

Expected: PASS for selected-Host authority, policy-only inspection, exact pin output, foreign diagnostics, path-free Runtime denial, and shared assembly.

~~~bash
git add internal/cli internal/runtime internal/integration
git commit -m "feat: separate host provider authority and diagnostics"
~~~

## Task 9: Document, Verify, and Re-run the Controlled P2 Gate

**Files:**
- Modify: README.md
- Modify: README-zh.md
- Modify: docs/en/architecture.md
- Modify: docs/zh/architecture.md
- Modify: docs/en/lifecycle.md
- Modify: docs/zh/lifecycle.md
- Modify: docs/en/troubleshooting.md
- Modify: docs/zh/troubleshooting.md
- Modify: docs/en/security.md
- Modify: docs/zh/security.md
- Modify: policy/ENGINEERING.md
- Modify: scripts/smoke-linux.sh
- Modify: relevant black-box tests
- Regenerate: controlled pilot P2 evidence beneath /Users/wifibaby4u/LLM/.oaw-pilots/2026-08-04-controlled-dogfooding-rerun-01/evidence/p2-runtime

- [ ] **Step 1: Update bilingual operator and architecture documentation.**

In each English/Chinese pair, document this exact chain:

~~~text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
~~~

Explain that Codex and Claude Code are independent Hosts; shared files yield separate Host Installation identities; Descriptor bindings and installation hints are declarations only; policy-only Hosts can show Candidates but cannot verify Runtime Instances; and foreign diagnostics never become pins or authority.

Replace location-and-version pin examples with provider_id, host_id, installation_key, evidence_digest, plus optional location/version. Document v1 rejection and the stable reasons HOST_BINDING_EVIDENCE_REQUIRED, PROVIDER_BINDING_UNAVAILABLE, PROVIDER_FOREIGN_HOST_ONLY, PROVIDER_PIN_INCOMPATIBLE, and HOST_PROVIDER_SCOPE_MISMATCH.

- [ ] **Step 2: Align the normative policy and Linux release smoke.**

Update policy/ENGINEERING.md Provider model language so Verified Provider Instance explicitly means one Host Installation plus Host-owned Binding evidence. Keep Profile ownership unchanged, including ECC-FULL as a complete lifecycle.

Extend scripts/smoke-linux.sh with a fixture HOME that contains Codex and Claude markers. Run providers inspect --host codex --format json and assert the current section contains no .claude path while the foreign diagnostic section does. The smoke remains read-only and must not create user config, pins, or Runtime State.

- [ ] **Step 3: Run formatting, schema, unit, race, coverage, and black-box verification.**

Run these commands from the implementation worktree:

~~~bash
gofmt -w internal
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
bash tests/run.sh
bash scripts/check-docs.sh
git diff --check
~~~

Run coverage:

~~~bash
go test -coverprofile=/tmp/oaw-host-scoped-provider.cover ./... -count=1
go tool cover -func=/tmp/oaw-host-scoped-provider.cover
~~~

Expected: all commands PASS and total statement coverage is at least 80%. Review uncovered Host mismatch, path containment, and Runtime denial branches and add focused tests if the threshold or critical-branch coverage is missing.

If shellcheck is installed, also run:

~~~bash
shellcheck -S warning -x install.sh tests/*.sh scripts/*.sh
~~~

If it is unavailable, record SKIP and continue; do not install a tool only for
this verification pass.

- [ ] **Step 4: Build releases and verify Linux/arm64 through Docker.**

~~~bash
scripts/build-release.sh /tmp/oaw-host-scoped-provider-release
scripts/smoke-docker.sh /tmp/oaw-host-scoped-provider-release/open-agent-workflow_0.1.0_linux_arm64.tar.gz
~~~

Expected on the current Apple Silicon Docker server: PASS for linux/arm64. If Docker CLI, daemon, or pinned image is unavailable, record SKIP from exit 77 and continue. Do not attempt native Linux execution on macOS.

Run the WSL probe:

~~~bash
scripts/smoke-wsl.sh /tmp/oaw-host-scoped-provider-release/open-agent-workflow_0.1.0_linux_arm64.tar.gz
~~~

Expected on macOS: exit 77 with SKIP. WSL unavailability does not block continued verification.

- [ ] **Step 5: Re-run the controlled deterministic and race gates.**

From the implementation worktree:

~~~bash
go test ./internal/classification ./internal/admission ./internal/runtime ./internal/integration ./internal/cli -run 'Direct|Workflow|ProfileSelection|Lock|Lease|Deduplic|Duplicate|Cancel|Uncertain|Recovery|RawOutput|ProjectRoot|Provider|Host' -count=3
go test -race ./internal/runtime ./internal/host ./internal/host/codex ./internal/registry -run 'Lease|Concurrent|Deduplic|Cancel|Uncertain|Provider|Host|Inventory' -count=3
~~~

Expected: deterministic pass^3 = 100% and no race report. Record fresh counts and stdout/stderr SHA-256 digests in the existing P2 summary without erasing the prior baseline.

- [ ] **Step 6: Re-run real-HOME inspection without writing a pin.**

Build the current binary:

~~~bash
go build -o /Users/wifibaby4u/LLM/.oaw-pilots/2026-08-04-controlled-dogfooding-rerun-01/bin/oaw-host-scope ./cmd/oaw
~~~

Run it with the real HOME and isolated pilot XDG roots:

~~~bash
env HOME=/Users/wifibaby4u XDG_CONFIG_HOME=/Users/wifibaby4u/LLM/.oaw-pilots/2026-08-04-controlled-dogfooding-rerun-01/config/p2-host-scope XDG_STATE_HOME=/Users/wifibaby4u/LLM/.oaw-pilots/2026-08-04-controlled-dogfooding-rerun-01/state/p2-host-scope /Users/wifibaby4u/LLM/.oaw-pilots/2026-08-04-controlled-dogfooding-rerun-01/bin/oaw-host-scope providers inspect --host codex --format json
~~~

Assert the current Host section contains only Codex Candidates and observations, Claude installations appear only under foreign diagnostics, no config.toml or pin was written, and Superpowers becomes verified only if the exact Codex Binding evidence is observed. Replace provider-inspection.md with a v2 evidence report that states those assertions and records all authority digests.

- [ ] **Step 7: Hold the live Host and P3 boundaries.**

Resume the bounded P2 live Codex Host smoke only after inspection shows one Codex-scoped verified Provider Instance and a Host-scoped Bundle can compile. Immediately before invoking Codex, stop for explicit user approval because it is a networked external-model dispatch. If unavailable after the approved bounded retry policy, record INCONCLUSIVE and continue the local work.

Do not begin the P3 controlled write in open-code-review. This plan exits after the Host-scoped P2 gate and evidence are complete.

- [ ] **Step 8: Review final scope and commit documentation/verification changes.**

Run:

~~~bash
rg -n 'catalogHostBindings|oaw.provider-descriptor/v1|oaw.user-config/v1|ProviderSettings([^,]+)' internal
git status --short
git diff --stat
git diff --check
~~~

Expected: no production authority-path matches for removed v1 contracts or catalogHostBindings; only explicit old-schema rejection tests may mention v1.

~~~bash
git add README.md README-zh.md docs/en docs/zh policy/ENGINEERING.md scripts/smoke-linux.sh tests
git commit -m "docs: explain host-scoped provider authority"
~~~

Do not stage controlled-pilot evidence, unrelated primary-worktree files, or external repository content in this repository commit.

## Plan Self-Review Results

**Spec coverage:** Sections 1-6 are the cross-cutting goal and authority model reflected by the plan architecture and file map. Sections 7-9 map to Tasks 1-3; Host Binding Inventory maps to Tasks 4 and 6; Registry resolution maps to Task 5; Profile/Runtime authority maps to Task 7; diagnostics and safety map to Task 8; the complete verification matrix and pilot exit criteria map to Task 9. No approved requirement is left without an implementation or verification step.

**Placeholder scan:** The plan contains no red-flag placeholder tokens, deferred implementation instructions, continuation markers, or unspecified test steps.

**Type and API consistency:** Discovery exposes Report.HostID; configuration exposes ProviderSettings(providerID, hostID) and ProviderInstallations; Host owns BindingInventory; Registry Resolve accepts snapshot, hostID, discovery Report, and Host inventory; ResolutionReport and Registry expose HostID; Profile EffectiveRegistry consumes Registry.HostID; RuntimeFrame, Graph, Bundle, and state use the same HostID. The CLI is the only environment-specific assembler and foreign reports are absent from every authority API.

**Execution boundary:** Implementation remains inline in a fresh worktree. The plan does not authorize a live external-model dispatch, a push, or P3 work.
