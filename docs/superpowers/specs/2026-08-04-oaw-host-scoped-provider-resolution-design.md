# OAW Host-Scoped Provider Resolution Design

**Date:** 2026-08-04  
**Status:** Approved for specification  
**Lifecycle:** MATT-SP-HYBRID  
**Scope:** Provider discovery, Host binding verification, pins, and runtime resolution

## 1. Summary

OAW currently treats Provider discovery as a global scan of the user HOME. A
single Provider resolution can therefore combine Claude Code and Codex
installations, apply a pin from one Host to another, or mark a Provider
verified from bindings declared only by its Descriptor. This is an authority
boundary defect, not merely a diagnostic defect.

This design makes Host identity part of Provider resolution from the first
observation through the final Lifecycle Bundle:

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

The selected Host is the first authority boundary. Discovery, pins, binding
inventory, Registry resolution, Profile compilation, and Runtime state all
operate on that Host scope. Other Hosts may be inspected for diagnostics, but
their observations never enter the selected Host's Effective Registry.

The design supports Superpowers, Matt, ECC, and future third-party Providers
with one extensible model. A Provider may be a complete lifecycle, a stage
owner, or a bounded add-on according to the selected Profile Recipe; its brand
does not grant authority.

## 2. Evidence and Problem Statement

The controlled dogfooding rerun proved the current Runtime and management
surfaces, but Provider inspection exposed the following defects:

1. `internal/discovery` records have no Host or installation-surface identity.
2. `config.ProviderPin` has no Host scope.
3. `registry.Resolve` considers all candidates for a Provider before applying
   Host compatibility.
4. Provider input assembly scans the whole HOME and passes a synthetic binding
   list assembled from Descriptor declarations.
5. The Superpowers Descriptor scans Claude and Codex paths while declaring
   only Codex bindings.

These defects permit false ambiguity, cross-Host candidate selection, and
verification without Host-owned evidence. The previous Phase 16 diagnostics
spec is therefore superseded for Provider resolution semantics; its read-only
inspection and diagnostic-only principles remain useful inputs.

## 3. Goals

1. Make a Candidate, Pin, and Provider Instance unambiguously Host-scoped.
2. Discover only the selected Host's declared installation surfaces for
   authority resolution.
3. Preserve dynamic discovery for built-in and user-registered Providers.
4. Allow an explicitly configured non-standard installation location without
   allowing configuration to forge Host binding evidence.
5. Verify a Capability only when a Host Adapter observes the exact Binding and
   associates it with the exact Host Installation.
6. Keep the existing Profile Recipe model extensible across Providers and
   Hosts.
7. Make Runtime and inspection use the same immutable input assembly.
8. Fail closed when Host evidence, pins, schemas, or scope are invalid.

## 4. Non-Goals

This change does not:

- make Claude Code or another policy-only Host Runtime-managed;
- invoke a model or Provider process during read-only inspection;
- automatically select a Profile or Provider candidate;
- merge installations from different Hosts into one authority record;
- add newest-version or path-precedence heuristics;
- replace the Provider-neutral Profile Recipe and Capability model;
- migrate old v1 configuration, reports, or Runtime State;
- make a Descriptor or user configuration an authority source;
- redesign the Lifecycle Bundle or Host Runtime protocol beyond adding the
  required Host-scoped identities and inventory evidence.

## 5. Terminology and Identity

### 5.1 Provider Family

The stable qualified Provider ID, such as `oaw/superpowers` or
`acme/engineering-suite`. A family is a catalog identity, not an installed
instance.

### 5.2 Distribution

A physical Provider distribution, checkout, cache entry, or versioned package.
Its identity is derived from Provider ID, declared distribution source,
canonical physical location, version, and discovery evidence.

### 5.3 Host Installation

A Distribution as exposed through one Host installation surface. Its identity
includes Host ID and surface ID in addition to the Distribution identity. A
shared physical directory can therefore yield two Host Installations, one for
Codex and one for Claude Code, without sharing authority.

### 5.4 Host Binding Evidence

Host-owned observation that a Capability Binding is actually available in the
selected Host and belongs to the selected Host Installation. Descriptor
`host_bindings` are an allowed contract; they are not observations.

### 5.5 Provider Instance

A discovered and verified Host Installation with a configuration snapshot,
exact evidence, and verified Capability instances. Only Provider Instances
enter the Effective Registry or Profile compilation.

## 6. Architecture

```text
Selected Host and Host Integration
              |
              v
Host-specific Discovery Probes
              |
              v
Host-scoped Candidate Report
              |
              +---- optional foreign-host diagnostic report
              |
              v
Host-scoped Pin
              |
              v
Host Adapter Binding Inventory
              |
              v
Exact Installation and Binding Match
              |
              v
Host-scoped Provider Instance / Effective Registry
              |
              v
Profile Recipe compilation and Lifecycle Bundle
```

The CLI owns environment-specific assembly. `oaw run` and
`oaw providers inspect` call the same assembly function and receive:

- the immutable Configuration Snapshot;
- the selected-Host Discovery Report;
- the selected-Host Binding Inventory;
- the selected-Host Provider Resolution Report;
- the selected-Host Effective Registry;
- an optional diagnostic-only foreign-host report.

The foreign report is never passed to Registry resolution, Profile
compilation, admission, or Bundle creation.

## 7. Provider Descriptor Contract

The active Provider Descriptor schema becomes `oaw.provider-descriptor/v2`.
No v1 descriptor is decoded or implicitly upgraded.

Each Discovery Probe must declare its Host scope and installation surface:

```json
{
  "id": "codex-curated-cache",
  "hosts": ["codex"],
  "surface": "codex-plugin-cache",
  "distribution": "openai-curated",
  "kind": "one-level-version-path-exists",
  "root": "user-home",
  "prefix": ".codex/plugins/cache/openai-api-curated/superpowers",
  "evidence_path": "skills/using-superpowers/SKILL.md"
}
```

Descriptor rules:

- `hosts` is a non-empty explicit list; no implicit Host and no wildcard;
- `surface` identifies a Host installation surface;
- `distribution` identifies the physical distribution source;
- probes for different Hosts are separate observations, even when their paths
  happen to be equal;
- a direct probe declares `candidate_path` as the installation root and an
  `evidence_path` relative to it, so unrelated direct probes cannot collapse
  into the whole user HOME;
- versioned probes derive the installation root from the version directory;
- Capability `host_bindings` must use a Host listed by the relevant probe;
- a Descriptor may declare multiple Host bindings for one Capability, but each
  Host requires independent Host evidence;
- Descriptor metadata remains inert and contains no executable discovery code.

Built-in Superpowers, Matt, and ECC descriptors are rewritten with explicit
surfaces for each Host they actually support. OAW adds a Host entry only when
the installation and binding contracts are known. A path under `.claude`
never proves a Codex installation, and a Codex binding is not implied by a
Claude probe. A Host with no actual binding evidence remains unverified even
when a descriptor is found.

## 8. Discovery Records and Scope

The active discovery evidence and report schemas become v2. Their records
include Host and surface identity:

```go
type Evidence struct {
    ProviderID      string
    HostID          string
    SurfaceID       string
    InstallationKey string
    ProbeID         string
    Kind            string
    Path            string
    Version         string
    ContentDigest   string
}

type Candidate struct {
    ProviderID      string
    HostID          string
    SurfaceID       string
    DistributionKey string
    InstallationKey string
    Location        string
    Version         string
    EvidenceDigest  string
    Evidence        []Evidence
}

type Report struct {
    HostID      string
    Candidates  []Candidate
    Digest      string
}
```

`discovery.Discover` receives the selected Host ID and only executes probes
whose `hosts` contain that Host. Its accumulator key includes Provider ID,
Host ID, surface, distribution, and canonical installation location. Evidence
from another Host cannot be merged into the selected Host's candidate.

An optional diagnostic call may discover other registered Hosts. It returns a
separate report with an explicit `foreign_host` projection. It is not an
alternative authority input.

All records are sorted before digesting. Host, surface, installation, and
evidence fields are included in canonical digests so equivalent input order
produces equivalent output and different Host instances cannot share a digest
by accident.

## 9. Configuration and Pins

The active user configuration schema becomes `oaw.user-config/v2`. Old v1
configuration is rejected with `UNSUPPORTED_SCHEMA_VERSION`; OAW does not
rewrite it.

Provider pins are keyed by `(provider_id, host_id)`:

```toml
schema_version = "oaw.user-config/v2"

[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "<digest-derived-key>"
evidence_digest = "<sha256>"
location = "/exact/physical/path"
version = "6.1.1"
```

`provider_id`, `host_id`, `installation_key`, and `evidence_digest` are the
identity match. `location` and `version`, when present, are readable assertions
and must also match. A changed Provider installation or evidence invalidates
the pin and requires a fresh inspection; OAW never drifts to another
candidate.

Pins for another Host remain in the configuration snapshot but are not read by
the selected Host. Duplicate pins for one Provider and Host are invalid.

Existing `binding_preferences` become Host-scoped by using `host_id`. They
select among exact observed bindings only; they cannot create inventory or
override a missing observation. Global Provider denials remain global security
constraints.

Users may register third-party descriptors and recipes as before. They may
also provide an optional Host-scoped installation hint:

```toml
[[provider_installations]]
provider_id = "acme/engineering-suite"
host_id = "codex"
surface_id = "custom-user-install"
location = "/absolute/provider/location"
discovery_probe_id = "codex-direct"
```

The hint only supplies a discovery location. The descriptor's evidence probes
must still pass: Discovery evaluates the referenced probe's relative
`evidence_path` beneath the configured location. The Host Adapter must still
observe the actual binding. User configuration cannot create trust, authority,
or a Verified Provider Instance by declaration alone.

## 10. Host Binding Inventory

The Host Adapter owns observation of the Host's native installation surface.
The normalized input to Registry resolution is:

```go
type BindingObservation struct {
    HostID            string
    InstallationKey   string
    Binding           catalog.HostBinding
    Source            string // host-index, host-filesystem, native-probe
    EvidenceReference string
    Digest            string
}

type BindingInventory struct {
    HostID       string
    Observations []BindingObservation
    Digest       string
}
```

Inventory invariants:

1. Inventory Host ID, Binding Host, and selected Host ID must be identical.
2. Each observation identifies an Installation and exact Host Binding. The
   Registry maps that Binding to a Descriptor Capability after observation.
3. The observation source and digest refer to Host-owned evidence, not a
   Descriptor field copied into the inventory.
4. An observation must associate its source with the Candidate's
   InstallationKey. A binding found elsewhere is not a match.
5. Inventory is deterministic, sorted, bounded, and digest-pinned.

The Codex Host Adapter observes the Host's actual local skill, agent, and tool
surfaces. For skills it enumerates the current Codex skill roots and records
the visible reference, source path, and content digest. For agents it observes
the configured Codex agent registry. Tools require a Host-native registry or
explicit runtime observation; a Descriptor cannot invent one. Claude Code and
other policy-only Hosts may expose candidate diagnostics without claiming
Runtime verification.

The Host Integration manifest gains a conformance requirement for
Host-scoped Provider Binding Inventory where Runtime verification is claimed.
The existing exact-binding invocation feature remains necessary but is not a
substitute for installation ownership evidence.

`catalogHostBindings` is removed. The Host Adapter, not the catalog, produces
the inventory.

## 11. Registry Resolution

`registry.Resolve` accepts one selected Host-scoped Discovery Report and one
Host-scoped Binding Inventory. The resolution algorithm is fixed:

1. Validate the selected Host and inventory Host match.
2. Load descriptor, denial, trust, capability-limit, preference, and Host-scoped
   pin settings from the Configuration Snapshot.
3. Select only candidates whose `HostID` equals the selected Host.
4. Apply the pin for `(ProviderID, HostID)` if one exists.
5. Reject zero matches as not found or pin-incompatible; reject more than one
   match as ambiguous.
6. Intersect the descriptor's allowed Host bindings with exact inventory
   observations for the selected Candidate's InstallationKey.
7. Apply capability limits and binding preferences only after the exact match.
8. Build a Provider Instance containing Host, Distribution, Installation, and
   Binding Evidence identities.
9. Add only that verified Instance to the Effective Registry.

The Registry itself is Host-scoped and records `HostID`. A Registry lookup by
Provider ID remains compatible with existing Profile selectors because one
Registry represents one Host. A lookup or Instance with a different Host is a
`HOST_PROVIDER_SCOPE_MISMATCH` error.

The Provider Resolution Report remains diagnostic and contains all current
Host candidates. A foreign report is never consulted for candidate selection.

## 12. Profile Compilation and Runtime State

Profile Recipes continue to select `provider_id + capability_id`. Compilation
receives the selected Host's Effective Registry, so the same user-defined
Recipe may be eligible on multiple Hosts only when each Host verifies all
required Capabilities.

Provider Instance v2 includes:

- Provider ID and Host ID;
- descriptor and configuration digests;
- Distribution and Installation keys;
- canonical location and version;
- discovery evidence digest;
- binding inventory evidence digest;
- sorted verified Capability instances;
- a digest over all of the above.

Lifecycle Bundle and Lifecycle Lock record the selected Host and the exact
Provider Instance digests. A configuration change, Host change, or evidence
change cannot mutate an existing Bundle; it requires a new Run or an allowed
stable-boundary switch.

The Runtime Engine rejects a Registry whose Host differs from the Runtime Host
Frame before admission or Profile compilation. Foreign inspection data never
changes Runtime State, authority, or Resource Leases.

## 13. Diagnostics

Runtime diagnostics are stable and path-free. The inspection command is the
explicit operator surface for physical locations and evidence.

Current Host resolution reasons:

| Condition | Reason |
| --- | --- |
| no current Host candidate | `PROVIDER_NOT_FOUND` |
| candidate only on another Host | `PROVIDER_FOREIGN_HOST_ONLY` as diagnostic detail |
| multiple current Host candidates | `PROVIDER_CANDIDATE_AMBIGUOUS` |
| current Host pin does not match | `PROVIDER_PIN_INCOMPATIBLE` |
| Host Adapter has no inventory evidence | `HOST_BINDING_EVIDENCE_REQUIRED` |
| inventory does not match Installation/Capability/Binding | `PROVIDER_BINDING_UNAVAILABLE` |
| Registry and Runtime Host differ | `HOST_PROVIDER_SCOPE_MISMATCH` |
| active input uses a removed schema | `UNSUPPORTED_SCHEMA_VERSION` |

`oaw providers inspect --host <host>` renders the current Host's candidates,
observed bindings, resolution states, verified Instances, and exact Host-scoped
pin fragments. An optional foreign-host diagnostic section identifies other
Host installations but cannot render them as current-Host pins.

`oaw run` uses the same assembly path and returns only the stable reason and a
pointer to inspection. It never prints provider filesystem paths or raw Host
transcripts in Runtime replies.

## 14. Security and Safety

- Host ID, surface ID, installation key, and all path inputs are validated at
  configuration and discovery boundaries.
- User-configured installation locations are canonicalized, contained where
  the selected Host surface requires containment, and subject to the existing
  symlink, regular-file, control-character, and evidence-size limits.
- Host-owned evidence is digest-pinned; changes invalidate the Provider
  Instance and its Pin.
- Descriptor declarations cannot create authority, bypass Host inventory, or
  select a Profile.
- Foreign Host observations cannot enter current Host Registry or Bundle state.
- Inspection is read-only and never writes configuration, state, or pins.
- Runtime errors remain path-free and do not disclose raw Provider output,
  credentials, or model transcripts.
- A malformed or unsupported schema fails closed; no implicit v1 fallback is
  permitted.

## 15. Verification Matrix

### Discovery and Identity

1. Claude and Codex markers coexist; each Host report contains only its own
   probes.
2. A shared physical directory declared for two Hosts produces two different
   InstallationKeys.
3. Multiple Codex installations produce Codex-only ambiguity.
4. A foreign-only installation produces no current Host candidate and is
   visible only in the foreign diagnostic projection.
5. Candidate, evidence, report, and instance digests are stable across input
   ordering and distinct across Host scope.

### Pins and Configuration

6. Codex and Claude pins for one Provider coexist without cross-selection.
7. A current-Host pin with a changed installation or evidence digest is
   incompatible.
8. Duplicate `(provider_id, host_id)` pins are rejected.
9. Non-standard `provider_installations` locations discover a Provider only
   after Descriptor evidence passes.
10. v1 descriptors/configuration/reports are rejected rather than migrated.

### Binding Verification

11. Descriptor-declared binding without Host inventory remains unverified.
12. Wrong Host, wrong InstallationKey, wrong Capability, or wrong reference
    cannot verify a binding.
13. Exact Host/Installation/Capability/Binding evidence verifies one
    Capability and produces a digest-pinned Instance.
14. The catalog cannot synthesize inventory from Descriptor declarations.

### Runtime and Profiles

15. The Runtime rejects a Registry whose Host differs from its Host Frame.
16. A custom Profile compiles on a Host with complete verified coverage and
    fails deterministically on a Host missing a required binding.
17. Foreign reports never change admission, Profile compilation, Bundle, or
    Resource Lease state.

### Environment and Regression

18. Full Go and CLI black-box suites pass on macOS.
19. Linux/arm64 smoke and focused Provider tests pass through Docker.
20. WSL is recorded as `SKIP` when unavailable; it is not treated as a failed
    prerequisite for continued work.
21. Existing controlled dogfooding P0/P1/P2 deterministic and race controls
    remain green before any live Host dispatch is resumed.

## 16. Cutover and Pilot Exit Criteria

The implementation is ready for the next pilot only when:

1. The new v2 catalog/configuration/report assets are embedded and validated.
2. Provider inspection reports separate Codex and Claude candidates in the
   controlled HOME fixture.
3. Codex Host Binding Inventory is independently observed and conformance
   evidence is refreshed.
4. P2 live Host smoke can start only with a Codex-scoped Verified Provider
   Instance and Bundle.
5. The existing provider-resolution and race evidence is regenerated under the
   new schema and references the new Host-scoped digests.
6. No implementation path retains `catalogHostBindings` or applies a global
   Provider Pin.

Only after these conditions are met may the controlled write phase resume. A
failed Host evidence check pauses the pilot; it does not authorize a fallback
to global discovery or Descriptor-declared bindings.
