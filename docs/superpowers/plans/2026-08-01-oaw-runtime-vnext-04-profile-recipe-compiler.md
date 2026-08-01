# OAW Runtime vNext Ticket 04 Profile Recipe Compiler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Compile built-in and user-defined Profile Recipes plus exact Profile Bindings into immutable, deterministic Execution Graphs backed only by verified Provider Instances and Capabilities.

**Architecture:** A new pure `internal/profile` compiler resolves an alias or Recipe ID, applies exact selector bindings, joins the selected Capability against the Effective Registry and Catalog descriptor, validates ownership and closed graph semantics, then emits a normalized graph whose digest pins the normalized Recipe and resolved Provider Instance digests. The compiler never discovers Providers, selects a Profile for the user, invokes a Capability, mutates configuration, or infers eligibility from Provider brand.

**Tech Stack:** Go 1.26, existing `catalog`, `registry`, and `canonicaljson` packages, table-driven public-seam tests, deterministic integration fixtures, race tests, and existing repository verification commands.

---

## Scope Boundary

Ticket 04 owns Profile resolution and Execution Graph compilation only. It does not:

- classify an Engineering Request or run the Startup Gate;
- discover, verify, install, or execute a Provider;
- issue Capability or Stage Grants;
- persist Lifecycle Bundles, Runs, revisions, leases, or evidence;
- infer a fallback Provider, Capability, Host Binding, effect, or resource;
- change Recipe topology through Profile Bindings;
- expose a new CLI or Runtime Protocol command.

## Locked Public Seams

```go
type EffectiveRegistry interface {
    Provider(id string) (registry.ProviderInstance, bool)
    Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool)
}

type CatalogSource interface {
    Providers() []catalog.ProviderDescriptorRecord
    Recipes() []catalog.ProfileRecipeRecord
    Aliases() []catalog.ProfileAliasRecord
}

type CompileRequest struct {
    Profile  string
    Bindings []ProfileBinding
}

type ProfileBinding struct {
    Selector            catalog.CapabilitySelector
    PreferredProviderID string
}

func CompileProfile(
    available CatalogSource,
    verified EffectiveRegistry,
    request CompileRequest,
) (ExecutionGraph, error)

func CompileRecipe(
    available CatalogSource,
    verified EffectiveRegistry,
    recipe catalog.ProfileRecipeRecord,
    bindings []ProfileBinding,
) (ExecutionGraph, error)
```

`CompileProfile` resolves either one exact Recipe ID or one built-in/catalog
alias, then delegates to `CompileRecipe`. A binding matches the Recipe's exact
`provider_id` plus `capability_id` selector and may replace only the Provider ID;
the Capability ID and graph topology remain unchanged. Multiple bindings for
the same selector fail closed. An unbound selector uses its declared Provider.

The graph exposes defensive-copy accessors and pins:

- schema, Recipe ID/version/digest, entry, stable boundaries, and terminal gates;
- normalized applied bindings and Provider ID/instance-digest references;
- node ID, kind, responsibility, Procedure phase, optionality, and transitions;
- exact Provider ID, Provider Instance digest, Capability ID, and Host Binding;
- input/outcome schema, maximum effects, resources, Request Modes,
  Executor topology, and delegation allow-list;
- typed Incident Routes and a deterministic graph digest.

Stable compiler codes are `PROFILE_NOT_FOUND`, `PROFILE_CAPABILITY_MISSING`,
`PROFILE_SELECTOR_AMBIGUOUS`, `PROFILE_OWNER_MISSING`,
`PROFILE_OWNER_DUPLICATE`, `PROFILE_EFFECT_UNSUPPORTED`,
`PROFILE_REQUEST_MODE_UNSUPPORTED`, `PROFILE_NODE_MISSING`,
`PROFILE_SELECTOR_NOT_FOUND`,
`PROFILE_GRAPH_UNREACHABLE`, `PROFILE_LOOP_NOT_CLOSED`, and
`PROFILE_TERMINAL_INVALID`.

## Graph Invariants

- Every retained required responsibility has exactly one owner.
- Only a verified Provider Instance and verified Capability can back a node.
- The selected descriptor Capability must declare the node responsibility and
  `WORKFLOW` Request Mode; its effects and resources must be in the v1 closed
  vocabulary.
- Missing optional nodes are omitted. Incident Routes to an omitted optional
  handler are omitted; any retained transition to an omitted node fails closed.
- Procedures attach to a retained phase and do not own control transitions.
- The entry and every routed control node can reach a listed terminal gate.
- Every retained non-Procedure node is reachable from the entry or a typed
  Incident Route. Cycles must have an explicit exit that reaches a terminal.
- Terminal gates are retained gate nodes with no outgoing transition.
- Equivalent inputs normalize all semantically unordered collections before
  Canonical JSON hashing and produce the same digest.

## File Structure

| Path | Responsibility |
| --- | --- |
| `internal/profile/records.go` | Compiler request/binding, immutable graph records, accessors, and stable compiler errors. |
| `internal/profile/compile.go` | Profile/alias lookup, exact binding resolution, verified Capability join, normalization, and digest creation. |
| `internal/profile/validate.go` | Ownership, Procedure, reachability, loop, terminal, effect, resource, and Request Mode invariants. |
| `internal/profile/profile_test.go` | Public `CompileProfile`/`CompileRecipe` behavior and failure-code tests. |
| `internal/integration/profile_compiler_test.go` | Built-in aliases, custom Recipe parity, optional handlers, and deterministic corpus. |

### Task 1: Compile one verified Recipe into an immutable graph

**Files:**
- Create: `internal/profile/records.go`
- Create: `internal/profile/compile.go`
- Create: `internal/profile/profile_test.go`

- [ ] **Step 1: Write the failing public-seam tracer test**

Add `TestCompileRecipePinsVerifiedCapabilityContract` using one valid phase and
one terminal gate. Supply a small `EffectiveRegistry` test implementation at
the package boundary. Assert the graph pins exact Recipe, Provider Instance,
Capability, Host Binding, effects/resources/topology, entry, transition, and
terminal values. Mutating any returned slice must not mutate the graph.

- [ ] **Step 2: Run the focused test to verify RED**

```bash
rtk go test ./internal/profile -run TestCompileRecipePinsVerifiedCapabilityContract
```

Expected: FAIL because `internal/profile` does not exist.

- [ ] **Step 3: Implement the minimum records and verified join**

Create the locked public seams, private graph slice storage, defensive-copy
accessors, and an `EffectiveRegistry` interface accepted by `registry.Registry`.
Resolve every retained node against both the selected Provider descriptor and
verified registry entries. Never construct or repair missing verification.

- [ ] **Step 4: Normalize and hash the first graph**

Normalize all non-nil slices and sort nodes, transitions, contracts, Provider
references, boundaries, and gates by stable keys. Compute a normalized Recipe
digest independently of Catalog insertion order and include it plus exact
Provider Instance digests in the closed graph digest record.

- [ ] **Step 5: Format and run the first GREEN check**

```bash
rtk gofmt -w internal/profile
rtk go test ./internal/profile -run TestCompileRecipePinsVerifiedCapabilityContract
rtk go vet ./internal/profile
```

Expected: PASS.

### Task 2: Enforce bindings, coverage, ownership, effects, and modes

**Files:**
- Extend: `internal/profile/compile.go`
- Create: `internal/profile/validate.go`
- Extend: `internal/profile/profile_test.go`

- [ ] **Step 1: Write one failing vertical slice per resolution behavior**

Add public-seam tests proving:

- an exact binding selects the preferred verified Provider while retaining the
  Capability ID and control topology;
- an unbound selector retains its declared Provider;
- duplicate bindings fail with `PROFILE_SELECTOR_AMBIGUOUS`;
- required missing Provider/Capability coverage fails with
  `PROFILE_CAPABILITY_MISSING`;
- zero and multiple owners fail with `PROFILE_OWNER_MISSING` and
  `PROFILE_OWNER_DUPLICATE`;
- a Capability without `WORKFLOW` fails with
  `PROFILE_REQUEST_MODE_UNSUPPORTED`;
- defensive closed-vocabulary validation maps unsupported effects to
  `PROFILE_EFFECT_UNSUPPORTED`.

- [ ] **Step 2: Run each new case to observe RED**

```bash
rtk go test ./internal/profile -run 'Binding|Capability|Owner|Effect|RequestMode'
```

- [ ] **Step 3: Implement exact binding and contract validation**

Index bindings by the original Recipe selector, reject duplicates before node
resolution, and resolve only the selected exact Provider/Capability pair.
Look up declaration metadata from the selected Provider descriptor; do not rely
on `VerifiedCapability` for effects, resources, responsibilities, or modes.
Validate responsibility ownership after optional-node resolution.

- [ ] **Step 4: Run focused GREEN and race checks**

```bash
rtk gofmt -w internal/profile
rtk go test ./internal/profile
rtk go test -race ./internal/profile
```

Expected: PASS.

### Task 3: Validate explicit closed control flow and optional handlers

**Files:**
- Extend: `internal/profile/validate.go`
- Extend: `internal/profile/profile_test.go`

- [ ] **Step 1: Write failing graph-invariant tests**

Cover retained transition targets, Procedure phase attachment, terminal-gate
kind/out-degree, unreachable nodes, a remediation cycle with no terminal exit,
a closed remediation cycle, and optional Incident Handlers. Assert stable
`PROFILE_NODE_MISSING`, `PROFILE_GRAPH_UNREACHABLE`,
`PROFILE_LOOP_NOT_CLOSED`, and `PROFILE_TERMINAL_INVALID` codes.

- [ ] **Step 2: Run focused tests to observe RED**

```bash
rtk go test ./internal/profile -run 'Graph|Loop|Terminal|Procedure|Optional|Incident'
```

- [ ] **Step 3: Implement closed graph validation**

Build control adjacency from retained non-Procedure nodes. Seed forward
reachability with the entry and retained Incident Route handlers. Build reverse
reachability from terminal gates. Detect cyclic strongly connected components;
a cycle without an edge/path to a terminal fails as an unclosed loop. Reject
all other unreachable or terminal-invalid shapes.

- [ ] **Step 4: Run focused GREEN and coverage checks**

```bash
rtk gofmt -w internal/profile
rtk go test -race ./internal/profile
rtk go test -cover ./internal/profile
```

Expected: PASS with at least 90% `internal/profile` statement coverage.

### Task 4: Prove aliases, full-family coverage, custom parity, and determinism

**Files:**
- Extend: `internal/profile/compile.go`
- Extend: `internal/profile/profile_test.go`
- Create: `internal/integration/profile_compiler_test.go`

- [ ] **Step 1: Write failing alias and integration corpus tests**

Load the built-in Catalog and compile `SP-FULL`, `MATT-FULL`, `ECC-FULL`, and
`MATT-SP-HYBRID` using verified instances synthesized from the Catalog
descriptors. Prove each alias resolves only when every required Capability is
verified, including complete ECC lifecycle coverage. For the hybrid, prove
missing optional ECC repair coverage omits those nodes and routes without
weakening required ownership.

Add one `acme/*` Recipe to a Catalog and prove `CompileProfile` sends it through
the same compiler contract. Reorder Recipe nodes, transitions, routes,
responsibilities, boundaries, bindings, and registry values and assert the graph
digest remains identical.

- [ ] **Step 2: Run the integration tests to observe RED**

```bash
rtk go test ./internal/integration -run 'Profile|Alias|Recipe|Deterministic'
```

- [ ] **Step 3: Implement Profile lookup and complete normalization**

Resolve one exact alias or Recipe ID; unknown input returns `PROFILE_NOT_FOUND`.
Do not hardcode Provider brands or alias-specific eligibility checks. Complete
normalization so all semantically unordered inputs hash identically while node
and edge identities remain explicit.

- [ ] **Step 4: Run focused and repository Go verification**

```bash
rtk gofmt -w internal/profile internal/integration
rtk go test ./internal/profile ./internal/integration
rtk go test -race ./internal/profile ./internal/integration
rtk go test ./...
```

Expected: PASS.

### Task 5: Review and complete Ticket 04 verification

**Files:**
- Modify only Ticket 04 code/tests if review finds a defect.
- Modify: `.scratch/oaw-runtime-vnext/issues/04-profile-recipe-compiler.md`
- Modify: `.scratch/oaw-runtime-vnext/workflow.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/review.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/verification.md`

- [ ] **Step 1: Review the complete diff against the approved specification**

Inspect `git diff main...HEAD` for Provider-brand assumptions, unverified
fallbacks, mutable slice exposure, order-dependent digests, incomplete loops,
optional-node widening, unstable error codes, and changes outside Ticket 04.
Record findings and remediation in the existing review evidence file.

- [ ] **Step 2: Run fresh Go quality gates**

```bash
rtk go vet ./...
rtk go test -race ./...
rtk go test -coverprofile=/tmp/oaw-ticket-04-coverage.out ./...
rtk go tool cover -func=/tmp/oaw-ticket-04-coverage.out
rtk go test -cover ./internal/profile
```

Require at least 80% repository statement coverage and 90% `internal/profile`
coverage.

- [ ] **Step 3: Run repository compatibility and security gates**

```bash
rtk bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
rtk shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
rtk bash tests/run-tests.sh
rtk go test ./internal/catalog ./internal/config ./internal/discovery ./internal/registry ./internal/classification ./internal/profile ./internal/integration
rtk govulncheck ./...
```

If `govulncheck` is unavailable, record that limitation explicitly instead of
claiming the gate passed.

- [ ] **Step 4: Record ticket evidence and completion state**

Check each acceptance item only after its implementation and fresh evidence are
identified. Update the tracker to Ticket 04 complete and the next unblocked
ticket only after all gates pass. Keep `.serena/` untracked and out of commits.

- [ ] **Step 5: Commit completion evidence**

```bash
rtk git add internal/profile internal/integration docs/superpowers/plans/2026-08-01-oaw-runtime-vnext-04-profile-recipe-compiler.md .scratch/oaw-runtime-vnext
rtk git commit -m "docs: record ticket 04 verification"
rtk git status --short --branch
```

Expected: Ticket 04 files are committed and the worktree contains no tracked
changes.
