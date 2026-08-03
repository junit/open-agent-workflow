# OAW Provider Resolution Diagnostics Design

**Date:** 2026-08-03
**Phase:** 16
**Status:** Approved for implementation planning
**Lifecycle:** MATT-SP-HYBRID
**Execution topology:** Inline execution in a new isolated worktree

## 1. Purpose

Phase 15 controlled dogfooding proved that OAW correctly refuses to select an
ambiguous Provider candidate. The live Codex START discovered three valid
`oaw/superpowers` candidates and stopped before Host dispatch, but the operator
received only the generic Runtime diagnostic `CAPABILITY_NOT_VERIFIED`.

The registry already retained the specific
`PROVIDER_CANDIDATE_AMBIGUOUS` resolution and every candidate. The CLI discarded
that Resolution Report while constructing the Runtime Engine, so the Runtime
could see only an Effective Registry that lacked Superpowers.

Phase 16 makes Provider resolution observable without weakening the existing
fail-closed authority model. It adds a read-only Provider inspection command
and passes the same Resolution Report to Runtime diagnostics.

## 2. Goals

1. Explain every dynamically discovered Provider state through a stable,
   read-only operator command.
2. Preserve the concrete Provider resolution reason when a requested Bounded
   Capability or selected Workflow Profile cannot compile.
3. Generate exact, deterministic Provider pin suggestions without writing
   configuration or selecting a candidate.
4. Use one configuration, discovery, binding-inventory, and resolution path for
   Provider inspection and Runtime execution.
5. Apply the same behavior to built-in and trusted third-party Providers.
6. Preserve Runtime State immutability and avoid persisting candidate paths in
   Runtime diagnostics.

## 3. Non-goals

Phase 16 does not:

- automatically select a Provider candidate;
- add discovery precedence, newest-version preference, Host affinity, or
  built-in Provider special cases;
- merge multiple installations into one Provider Instance;
- create, edit, or repair user configuration;
- change Provider pin matching semantics;
- migrate or rewrite existing Runtime State or Phase 15 evidence;
- change Profile ownership, Capability contracts, or Host admission;
- publish, push, merge, tag, or release a build.

## 4. Governing Invariants

The following invariants remain authoritative:

1. Discovery reports evidence; it never selects authority.
2. Only a verified Provider Instance enters the Effective Registry.
3. An ambiguous or incompatible Provider remains unavailable to admission and
   Profile compilation.
4. Provider pins are user-owned authority configuration. OAW may suggest an
   exact pin but must not write it.
5. Runtime diagnostics may explain an authority failure but cannot repair or
   override it.
6. A configuration change creates a new Configuration Snapshot. An existing
   Engineering Run or Lifecycle Bundle never absorbs it.

## 5. Architecture

Provider inspection and Runtime execution share one read-only assembly path:

```text
Configuration Snapshot
        |
        v
Dynamic Discovery
        |
        v
Registry.Resolve
        |
        +---- Effective Registry      admission and compilation input
        |
        +---- Resolution Report       diagnostic input only
                    |
          +---------+---------+
          |                   |
          v                   v
 providers inspect      Runtime diagnostics
```

The CLI owns environment-specific assembly: locating the user configuration,
loading trusted project configuration, resolving the physical user home,
running declarative discovery, constructing the authoritative Host binding
inventory, and calling `registry.Resolve`.

The shared result contains exactly:

- the immutable Configuration Snapshot;
- the immutable Discovery Report;
- the immutable Provider Resolution Report;
- the Effective Registry.

`oaw run` and `oaw providers inspect` must call this shared assembly path. They
must not independently reproduce discovery or registry settings.

## 6. Responsibility Boundaries

### 6.1 Discovery

`internal/discovery` remains unchanged in authority and selection behavior. It
returns sorted candidate evidence under the existing containment, symlink,
regular-file, and 4 MiB read limits.

### 6.2 Registry

`internal/registry` remains the sole owner of candidate-to-instance resolution.
It applies user denial, trust, evidence availability, exact Provider pins,
ambiguity checks, binding inventory, Capability limits, and binding preferences
in the existing order.

The Resolution Report remains diagnostic. The Effective Registry remains the
only source of verified Provider Instances and Capabilities.

### 6.3 Profile Compiler

The Profile compiler remains the sole resolver of aliases, Profile Bindings,
optional nodes, and required Capability selectors. Runtime must not traverse a
Recipe independently to infer the Provider that failed.

When compilation fails because a required Provider or Capability is not
verified, the typed `CompileError` must carry the resolved `ProviderID` and
`CapabilityID`. These fields describe the selector after aliases and Profile
Bindings have been applied. Other compiler errors do not acquire Provider
resolution metadata.

### 6.4 Runtime

Bounded and Workflow Runtime options receive the Resolution Report alongside
the Effective Registry. Runtime consults it only after normal Capability
verification or Profile compilation fails.

Runtime cannot use the report to add an Instance to the Registry, choose a
candidate, change a selector, or bypass compilation.

### 6.5 CLI

The CLI owns rendering and operator guidance. It may show physical candidate
locations only in response to the explicit inspection command. Normal Runtime
diagnostics remain path-free.

## 7. Provider Inspection Command

The new public command is:

```text
oaw providers inspect --host codex \
  [--project-root /absolute/path] \
  [--format text|json]
```

`--host` is required because a Provider cannot become verified without an
authoritative Host binding inventory. `--project-root`, when present, follows
the same clean absolute-path validation used by `oaw run`.

The command is local and read-only. It does not create an Engineering Run,
invoke a Provider or Host process, call a model, access the network, or modify
configuration.

### 7.1 Exit Status

| Status | Meaning |
| --- | --- |
| `0` | Inspection completed, regardless of individual Provider states. |
| `64` | Invalid command arguments. |
| `65` | Configuration, catalog, discovery, or registry assembly failed. |
| `69` | The requested Host is unsupported for this inspection path. |
| `70` | Structured output could not be encoded. |
| `74` | Output could not be written. |

An ambiguous, missing, disabled, or incompatible Provider is data in a
successful report, not a command execution failure.

### 7.2 Text Output

Text output sorts Providers by qualified ID and candidates by their existing
deterministic candidate order. Each Provider section includes its state and
reason. Candidate sections include version, canonical physical location, and
Evidence digest.

For every ambiguous candidate, text output renders one exact TOML suggestion:

```toml
[[provider_pins]]
id = "oaw/superpowers"
location = "/exact/physical/path"
version = "6.1.1"
```

The suggestion always includes both `location` and `version`. A version-only
pin may remain ambiguous when several installations expose the same version.

When the user configuration file does not exist, the output identifies its
expected path and renders a complete document beginning with:

```toml
schema_version = "oaw.user-config/v1"
```

When the file exists, output renders appendable pin fragments and does not
repeat or rewrite the existing document.

TOML strings must be encoded by a structured TOML-safe renderer. Paths and IDs
must never be interpolated into an unescaped template.

### 7.3 JSON Output

JSON output uses `schema_version = "oaw.provider-inspection/v1"` and contains:

- Host ID and inspected configuration path;
- whether the user configuration file exists;
- Configuration, Catalog, Discovery, Resolution, and Effective Registry
  digests;
- sorted Provider results;
- Provider ID, state, and stable reason for every result;
- verified Instance identity fields when available;
- sorted candidate version, location, and Evidence digest;
- structured `provider_pin` suggestions with `id`, `location`, and `version`.

The JSON output does not contain indicator file contents, raw Provider output,
credentials, environment values, or Host/model transcripts.

## 8. Runtime Diagnostic Mapping

Runtime maps an unavailable requested Provider to its existing stable registry
reason:

```text
PROVIDER_NOT_FOUND
PROVIDER_DISCOVERED_UNVERIFIED
PROVIDER_CANDIDATE_AMBIGUOUS
PROVIDER_PIN_INCOMPATIBLE
PROVIDER_BINDING_UNAVAILABLE
PROVIDER_DISABLED_BY_USER
PROVIDER_PROJECT_CONTENT_UNTRUSTED
```

### 8.1 Bounded Mode

When a Bounded selector fails Capability verification, Runtime queries the
Resolution Report using that selector's Provider ID.

If the Provider has a non-verified resolution, the Bounded reply retains the
existing fail-closed status and reply topology but replaces the generic
`CAPABILITY_NOT_VERIFIED` diagnostic code with the concrete Provider reason.
Its message names the Provider ID, state, candidate count, the inspection
command, and the need to begin a new Run after configuration changes.

If the Provider is verified, Runtime preserves the original Capability or
admission error. Resolution diagnostics must not mask a missing Capability,
unsupported Request Mode, invalid binding, or contract mismatch.

### 8.2 Workflow Mode

Initial Workflow START remains independent of Provider availability and enters
the Startup Gate normally. Unrelated ambiguous Providers never block initial
selection.

After `PROFILE_SELECTED`, the Profile compiler resolves the selected Recipe,
aliases, and Profile Bindings. If it returns a typed missing-Capability error,
Runtime queries the Resolution Report for the resolved Provider ID.

If that Provider is non-verified, the command returns the concrete Provider
reason. If the Provider is verified or the error has no Provider selector
metadata, Runtime preserves `PROFILE_SELECTION_INVALID` and the underlying
compiler reason.

When more than one required Provider is unavailable, deterministic compiler
order identifies the first blocking Provider. The inspection command remains
the complete view of all Provider states.

### 8.3 Runtime State Privacy

Runtime diagnostic messages may persist:

- Provider ID;
- Provider state;
- stable reason;
- candidate count;
- the generic inspection command;
- the instruction to start a new Run after changing configuration.

They must not persist candidate locations, evidence file paths, file contents,
or rendered pin fragments. Those appear only in explicit inspection output.

## 9. Configuration and Recovery Semantics

OAW never creates or edits the user configuration during inspection or Runtime
failure handling.

The operator recovery sequence is:

1. Run `oaw providers inspect --host codex` with the same project root, if any.
2. Select one candidate explicitly.
3. Add the exact suggested pin to the user-owned configuration.
4. Begin a new START using the new Configuration Snapshot.

An existing Run cannot continue after its Configuration Snapshot changes. OAW
must not silently reload the pin into that Run or mutate an active Lifecycle
Bundle.

## 10. Error Handling

The shared assembly path returns stable, wrapped failures for:

- configuration loading;
- physical user-home resolution;
- discovery execution;
- registry resolution;
- unsupported Host inventory;
- output encoding and writing.

The inspect command writes structured report data only to stdout. Human-readable
errors go to stderr. Runtime output preserves the existing canonical JSON-only
stdout rule and sends human diagnostics to stderr where applicable.

A failed inspection must not emit a partial report that could be mistaken for
a complete resolution view.

## 11. Security and Privacy

1. Inspection performs no Provider execution, Host dispatch, model invocation,
   network access, or configuration mutation.
2. Existing discovery containment and symlink defenses remain mandatory.
3. Candidate paths are shown only after an explicit local inspection command.
4. Indicator contents and raw Provider output are never rendered.
5. TOML suggestions use structured escaping to prevent configuration
   injection.
6. Tests verify that user configuration content, mode, and modification time do
   not change.
7. The implementation must not read or store credentials.

## 12. Confirmed Test Seams

Matt TDD will use vertical red-green slices at these user-approved public
boundaries.

### 12.1 Registry Seam

Test `registry.Resolve` for:

- unpinned ambiguity;
- exact location-and-version pin disambiguation;
- an incompatible pin;
- deterministic candidate order;
- no newest-version, Host-affinity, or built-in precedence;
- equivalent behavior for built-in and third-party Providers.

### 12.2 Runtime Seam

Test `runtime.Engine.Exchange` for:

- Bounded ambiguity returning `PROVIDER_CANDIDATE_AMBIGUOUS`;
- every other non-verified Provider state mapping to its stable reason;
- a verified Provider's Capability failure retaining its original error;
- initial Workflow START ignoring unrelated ambiguity;
- selected Workflow Profile reporting the resolved bound Provider;
- unrelated Provider ambiguity not blocking a valid selected Profile;
- Runtime diagnostics excluding candidate paths and evidence paths.

### 12.3 CLI Seam

Test `cli.RunWithInput` for:

- deterministic text output;
- the `oaw.provider-inspection/v1` JSON contract;
- complete digests and stable sorting;
- one exact pin object and TOML fragment per ambiguous candidate;
- safe TOML escaping;
- missing versus existing user configuration guidance;
- third-party Provider output;
- successful exit status for unresolved Provider states;
- stable argument and assembly error statuses;
- no configuration content, permission, or modification-time changes.

### 12.4 Codex Runner Seam

Test the public runner boundary for:

- ambiguity stopping before Host preparation or invocation;
- zero Host processes and model calls for the unpinned case;
- an isolated temporary user configuration with an explicit pin reaching the
  existing verified/admission path;
- Runtime stdout remaining canonical JSON.

## 13. Verification Matrix

Implementation verification runs in this order:

1. focused red-green package tests;
2. all affected package tests;
3. `go test ./...`;
4. race tests for affected registry, Runtime, and CLI packages;
5. coverage verification at or above 80 percent;
6. Docker Linux/arm64 verification;
7. WSL smoke when available, otherwise an explicit `SKIP`;
8. an actual-home, read-only inspection smoke;
9. `git diff --check` and focused security review.

macOS is the native development environment. Other available environments run
through Docker. An unavailable environment does not block progress when its
result is recorded as `SKIP`.

## 14. Isolation and Evidence

Implementation occurs in a new Phase 16 isolated worktree. It must not reuse
the Phase 15 dogfooding evidence worktree or modify the evidence under:

```text
/Users/wifibaby4u/LLM/.oaw-pilots/2026-08-03-controlled-dogfooding/evidence
```

Tests use temporary configuration roots. A real-home smoke may inspect the
installed candidates but may not write the real user configuration.

## 15. Acceptance Criteria

Phase 16 is complete when:

1. `oaw providers inspect --host codex` reports the three actual Superpowers
   candidates with exact versions, canonical locations, and independent pin
   suggestions.
2. An unpinned Bounded request for Superpowers returns
   `PROVIDER_CANDIDATE_AMBIGUOUS`, not only `CAPABILITY_NOT_VERIFIED`.
3. The ambiguous request performs zero Host dispatches and zero model calls.
4. An exact pin in an isolated temporary configuration resolves one verified
   Superpowers Provider Instance and reaches the existing admission path.
5. Workflow Profile selection diagnoses only Providers required by the selected
   Recipe and resolved Profile Bindings.
6. Built-in and third-party Providers use the same implementation path.
7. No command automatically selects a candidate or writes user configuration.
8. No Runtime diagnostic persists candidate paths or Provider contents.
9. All focused, full, race, coverage, and available platform checks pass, with
   unavailable platform checks recorded as `SKIP`.
10. Phase 15 evidence remains byte-for-byte unchanged.

