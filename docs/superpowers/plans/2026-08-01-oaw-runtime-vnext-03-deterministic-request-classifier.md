# OAW Runtime vNext Ticket 03 Deterministic Request Classifier Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Validate an untrusted Host Classification Proposal and deterministically emit a monotonic Direct, Bounded, or Workflow decision with complexity, risk, evidence requirements, and stable escalation reasons.

**Architecture:** The Host supplies a closed, evidence-backed semantic proposal; it never supplies authority and the Runtime never parses arbitrary natural language or calls a model API. A pure Policy Classifier validates the proposal, applies immutable built-in safety floors followed by user and trusted-project minimums, and returns a new decision value. Bounded classification carries an exact selector when present but never resolves a Provider or Capability; admission remains a later Registry concern.

**Tech Stack:** Go 1.26, `encoding/json`, the existing embedded Draft 2020-12 schema registry, `canonicaljson`, table-driven unit tests, deterministic evaluation fixtures, race tests, and the existing repository verification commands.

---

## Scope Boundary

Ticket 03 owns proposal validation and deterministic classification only. It does
not:

- parse request prose, call a model API, inspect a filesystem, or infer Host state;
- select a Provider, resolve a Capability, issue a Grant, or start a Runtime Run;
- compile Profile Recipes or activate the Workflow Startup Gate;
- persist Runtime State, mutate project files, or add a CLI command;
- weaken built-in safety floors because a user or project rule is lower.

The classifier receives explicit typed values. A missing, malformed, unknown, or
uncertain critical trait returns a conservative `WORKFLOW`/`complex` decision,
with `CLASSIFICATION_UNAVAILABLE` or a more specific stable escalation reason.

## Locked Domain Contract

### Proposal

`ClassificationProposal` is the untrusted Host input:

```go
type ClassificationProposal struct {
    SchemaVersion      string
    Traits             []TraitObservation
    Resources          []Resource
    Evidence           []ProposalEvidence
    CapabilitySelector *CapabilitySelector
}
```

The schema version is `oaw.classification-proposal/v1`. Traits use only this
closed vocabulary:

```text
scope-clear, change-point-known, recoverable,
focused-verification-known, bounded-capability-request,
architecture-decision, public-contract-change, schema-change,
dependency-change, security-sensitive, data-sensitive, deployment-change,
domain-uncertainty, root-cause-uncertain, multiple-responsibilities,
multiple-tickets, long-lived-delegation, destructive-mutation,
critical-release
```

Every critical trait must appear exactly once with `true` or `false`. An
explicit `unknown` value, an omitted critical trait, an unknown trait, duplicate
trait, invalid resource, invalid evidence, or invalid selector is not a
permission to guess; it produces Workflow fallback.

`Resource` values are `project`, `project-worktree`, `git-repository`,
`public-api`, `schema`, `dependency`, `security`, `data`, `deployment`,
`credentials`, `network`, and `destructive`.

`ProposalEvidence` contains an `EvidenceKind`, non-empty reference, and a
lowercase 64-character digest. The classifier recognizes `scope`,
`change-point`, `verification`, `capability-selector`, `security-acceptance`,
`negative-test`, `architecture`, `authorization`, and `recovery`.

`CapabilitySelector` contains a qualified Provider ID, local Capability ID, and
source `user-intent` or `trusted-rule`. The classifier preserves it only for a
`bounded-capability-request`; it never resolves it.

### Decision and rules

```go
type ClassificationDecision struct {
    RequestMode          RequestMode
    WorkflowComplexity   *WorkflowComplexity
    RiskClass            RiskClass
    EvidenceRequirements []EvidenceRequirement
    EscalationReasons    []string
    CapabilitySelector   *CapabilitySelector
}

type ClassificationRules struct {
    User    PolicyLayer
    Project PolicyLayer
}

type PolicyLayer struct {
    MinimumMode       RequestMode
    MinimumRisk       RiskClass
    ProtectedResources []Resource
    RequiredEvidence  []EvidenceKind
}
```

`Classify(proposal *ClassificationProposal, rules ClassificationRules)` applies
immutable built-in floors first, then takes the maximum Request Mode and Risk
Class from user/project minimums, unions protected-resource and evidence
requirements, and never lowers a built-in result. It returns a decision digest
computed from Canonical JSON. Invalid policy values return an error; invalid or
missing proposals return the conservative decision rather than an error.

Stable reasons include `CLASSIFICATION_UNAVAILABLE`,
`CLASSIFICATION_TRAIT_MISSING`, `CLASSIFICATION_TRAIT_UNCERTAIN`,
`CLASSIFICATION_TRAIT_UNKNOWN`, `DIRECT_SCOPE_UNCLEAR`,
`DIRECT_VERIFICATION_REQUIRED`, `WORKFLOW_REQUIRED_ARCHITECTURE`,
`WORKFLOW_REQUIRED_PUBLIC_CONTRACT`, `WORKFLOW_REQUIRED_SCHEMA`,
`WORKFLOW_REQUIRED_DEPENDENCY`, `WORKFLOW_REQUIRED_SECURITY`,
`WORKFLOW_REQUIRED_DATA`, `WORKFLOW_REQUIRED_DEPLOYMENT`,
`WORKFLOW_REQUIRED_UNRESOLVED`, `WORKFLOW_REQUIRED_MULTIPLE_RESPONSIBILITIES`,
`WORKFLOW_REQUIRED_MULTIPLE_TICKETS`, `WORKFLOW_REQUIRED_DELEGATION`,
`WORKFLOW_REQUIRED_DESTRUCTIVE`, `WORKFLOW_REQUIRED_CRITICAL_RELEASE`,
`POLICY_PROTECTED_RESOURCE`, `POLICY_MINIMUM_MODE`,
`POLICY_MINIMUM_RISK`, and `CAPABILITY_SELECTION_REQUIRED`.

## File Structure

| Path | Responsibility |
| --- | --- |
| `internal/assets/schemas/v1/classification-proposal.schema.json` | Closed JSON Schema for Host proposals. |
| `internal/schema/registry.go` | Register and expose the proposal schema. |
| `internal/schema/registry_test.go` | Prove the proposal schema compiles and rejects open fields. |
| `internal/classification/records.go` | Closed enums, proposal/decision/rule records, normalization and defensive copies. |
| `internal/classification/decode.go` | Strict JSON proposal decoding and schema/domain validation. |
| `internal/classification/classify.go` | Pure monotonic classification, risk floors, evidence requirements, and digest construction. |
| `internal/classification/classification_test.go` | Proposal seam, Direct/Bounded/Workflow behavior, policy monotonicity, and digest tests. |
| `internal/classification/eval_test.go` | Versioned critical-release evaluation corpus; no critical case may be Direct or Bounded. |

### Task 1: Add the closed proposal schema and domain records

**Files:**
- Create: `internal/assets/schemas/v1/classification-proposal.schema.json`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`
- Create: `internal/classification/records.go`
- Create: `internal/classification/decode.go`
- Create: `internal/classification/classification_test.go`

- [ ] **Step 1: Write failing proposal seam tests**

Add tests with these behaviors:

```go
func TestDecodeProposalAcceptsClosedEvidenceBackedRecord(t *testing.T)
func TestDecodeProposalRejectsUnknownFieldsTrailingJSONAndUnknownTraits(t *testing.T)
func TestDecodeProposalRejectsDuplicateTraitsInvalidDigestsAndBadSelectors(t *testing.T)
func TestProposalNormalizationIsDeterministicAndDefensive(t *testing.T)
```

The valid fixture uses every critical trait exactly once, `resources: []`,
three evidence records (`scope`, `change-point`, `verification`), and no
selector. Assertions must verify normalized trait order, non-nil empty slices,
lowercase digest validation, and that changing a returned slice does not alter
the decoded value.

- [ ] **Step 2: Run the tests to verify RED**

Run:

```bash
GOCACHE=/tmp/oaw-go-build-ticket03 go test ./internal/classification ./internal/schema
```

Expected: FAIL because the classification package, proposal schema, and
registry constant do not exist.

- [ ] **Step 3: Add and register the Draft 2020-12 proposal schema**

Add `classification-proposal.schema.json` with `additionalProperties: false`,
the exact schema version constant, maximum collection sizes of 64 traits, 32
resources, and 128 evidence records, unique closed enum values, and nested
selector/evidence objects with no executable or authority fields. Extend
`schema.Registry` with:

```go
const ClassificationProposalV1 = "https://open-agent-workflow.dev/schemas/v1/classification-proposal.schema.json"
```

Register it in `New`, and add a registry test that validates one normalized
proposal and rejects an `extra` property.

- [ ] **Step 4: Implement closed records and strict decoding**

Implement the exact public enums and structs from the Locked Domain Contract,
plus:

```go
func DecodeProposal(raw []byte) (ClassificationProposal, error)
func (value ClassificationDecision) Digest() string
```

`DecodeProposal` must use a `json.Decoder` with `DisallowUnknownFields`, reject
trailing JSON and payloads over 1 MiB, validate UTF-8 through JSON decoding,
validate the embedded schema, normalize collections by stable key, and reject
duplicate traits/resources/evidence identities. It must use
`catalog.ParseQualifiedID` and `catalog.ParseLocalID` for selector IDs.

The records must not contain maps, interfaces, commands, paths, environment
expressions, or authority fields. Collection getters and clone helpers return
new slices.

- [ ] **Step 5: Run focused tests and commit**

Run:

```bash
gofmt -w internal/classification/*.go internal/schema/*.go
GOCACHE=/tmp/oaw-go-build-ticket03 go test ./internal/classification ./internal/schema
GOCACHE=/tmp/oaw-go-build-ticket03 go vet ./internal/classification ./internal/schema
```

Expected: PASS. Commit:

```bash
git add internal/assets/schemas/v1/classification-proposal.schema.json internal/schema internal/classification
git commit -m "feat: define classification proposal contract"
```

### Task 2: Implement deterministic Direct, Bounded, and Workflow classification

**Files:**
- Create: `internal/classification/classify.go`
- Extend: `internal/classification/classification_test.go`

- [ ] **Step 1: Write failing mode decision tests**

Add table cases:

```go
func TestClassifyClearBoundedDirectRequest(t *testing.T)
func TestClassifyBoundedRequestRequiresExactSelector(t *testing.T)
func TestClassifyWorkflowTriggersFromSemanticTraits(t *testing.T)
func TestClassifyMissingOrUncertainCriticalTraitsFailsUpward(t *testing.T)
```

The clear Direct case sets scope, change point, recovery, and focused
verification true and every risk/complexity trait false; it must return
`DIRECT`, `RiskClassNormal`, nil workflow complexity, and no Provider choice.
The Bounded case sets `bounded-capability-request` true and carries an exact
`user-intent` selector; omitting the selector must retain `BOUNDED` while adding
`CAPABILITY_SELECTION_REQUIRED` and `capability-selector` evidence. Architecture,
public contract, schema, dependency, security, data, deployment, unresolved,
multiple-responsibility, multiple-ticket, delegation, destructive, and critical
release cases must return `WORKFLOW`/`complex` with their stable reason.

- [ ] **Step 2: Run tests to verify RED**

Run:

```bash
GOCACHE=/tmp/oaw-go-build-ticket03 go test ./internal/classification -run 'Classify|Workflow|Bounded|Direct'
```

Expected: FAIL because the classifier function does not exist.

- [ ] **Step 3: Implement the pure base classifier**

Implement:

```go
func Classify(proposal *ClassificationProposal, rules ClassificationRules) (ClassificationDecision, error)
```

Validation must first reject invalid rules and then either normalize a valid
proposal or create a conservative Workflow/complex decision with
`CLASSIFICATION_UNAVAILABLE`. For a valid proposal, calculate risk (`critical`
for destructive or critical-release traits, `elevated` for protected semantic
changes, otherwise `normal`), collect stable reasons, and choose:

```text
workflow trigger -> WORKFLOW/complex
bounded-capability-request -> BOUNDED
otherwise -> DIRECT
```

Direct requires all four bounded-scope traits true and no workflow trigger;
missing scope or focused verification adds its stable Direct reason and raises
to Workflow. A selector is copied only for Bounded. No code path may inspect
free-form text or import an HTTP/model client.

- [ ] **Step 4: Add evidence requirements and decision digest**

Require `scope`, `change-point`, and `verification` evidence for a clear Direct
decision. Require `capability-selector` for Bounded and add
`security-acceptance`, `negative-test`, `recovery`, or `architecture` when the
corresponding risk/complexity trait is true. Missing requirements are sorted,
deduplicated, surfaced in `EvidenceRequirements`, and force Workflow when the
proposal is otherwise insufficient. Hash a closed record containing the exact
decision fields with `canonicaljson.Digest`; never include raw proposal text.

- [ ] **Step 5: Run focused tests and commit**

Run:

```bash
gofmt -w internal/classification/*.go
GOCACHE=/tmp/oaw-go-build-ticket03 go test ./internal/classification
GOCACHE=/tmp/oaw-go-build-ticket03 go test -race ./internal/classification
```

Expected: PASS. Commit:

```bash
git add internal/classification
git commit -m "feat: classify requests deterministically"
```

### Task 3: Enforce monotonic user/project policy layers

**Files:**
- Extend: `internal/classification/classify.go`
- Extend: `internal/classification/classification_test.go`

- [ ] **Step 1: Write failing policy monotonicity tests**

Add:

```go
func TestUserAndProjectRulesOnlyRaiseModeRiskAndEvidence(t *testing.T)
func TestProtectedResourcesRaiseWorkflowWithoutSelectingProvider(t *testing.T)
func TestPolicyRulesAreOrderIndependentAndDigestStable(t *testing.T)
```

Use a Direct proposal and apply User `MinimumMode: BOUNDED`, Project
`MinimumMode: WORKFLOW`, User `MinimumRisk: elevated`, and duplicate required
evidence in reverse order. Assert the result is Workflow/complex/elevated,
contains one sorted copy of each requirement, and carries no selector. Apply
lowering rules after raising rules and prove the decision does not downgrade.

- [ ] **Step 2: Run tests to verify RED**

Run:

```bash
GOCACHE=/tmp/oaw-build-ticket03 go test ./internal/classification -run 'Policy|Protected|Order'
```

Expected: FAIL because policy layers are not yet applied.

- [ ] **Step 3: Implement monotonic rule composition**

Normalize empty layer values to the safe identity (`DIRECT`, `normal`, empty
collections), take the maximum mode/risk using explicit ordered enums, union
protected resources and evidence by sorted identity, and add
`POLICY_PROTECTED_RESOURCE`, `POLICY_MINIMUM_MODE`, or `POLICY_MINIMUM_RISK`
only when a layer raises the base decision. Never replace a base Workflow with
a lower layer value and never use a rule to fill in a Capability selector.

- [ ] **Step 4: Verify policy invariants and commit**

Run:

```bash
gofmt -w internal/classification/*.go
GOCACHE=/tmp/oaw-build-ticket03 go test ./internal/classification
GOCACHE=/tmp/oaw-build-ticket03 go test -race ./internal/classification
```

Expected: PASS with at least 90% classification package coverage. Commit:

```bash
git add internal/classification
git commit -m "feat: enforce monotonic classification policy rules"
```

### Task 4: Add critical-release evaluation corpus and cross-package proof

**Files:**
- Create: `internal/classification/eval_test.go`
- Create: `internal/integration/classification_test.go`

- [ ] **Step 1: Write the critical evaluation corpus**

Create a table of named cases for public-contract release, schema migration,
dependency upgrade, security-sensitive mutation, credential/data operation,
deployment change, destructive migration, unresolved architecture, and
multi-ticket delegation. Each case must use only typed traits/resources, set
the critical trait true, and assert `WORKFLOW`, `complex`, and never `DIRECT` or
`BOUNDED`. Keep the table as the versioned release corpus; do not add keyword
strings or model calls.

- [ ] **Step 2: Add the cross-package policy projection test**

Use `config.Load(LoadOptions{})` only to prove the classifier accepts an
explicit empty configuration projection; pass a User/Project
`ClassificationRules` value explicitly and assert a Project minimum can raise
an ordinary Direct proposal to Workflow while a later lower rule cannot undo
it. This test must not select a Provider, invoke discovery, or mutate a file.

- [ ] **Step 3: Run focused race and full verification**

Run:

```bash
gofmt -w internal/classification/*.go internal/integration/*.go
GOCACHE=/tmp/oaw-go-build-ticket03 go test ./internal/classification ./internal/integration
GOCACHE=/tmp/oaw-go-build-ticket03 go test -race ./internal/classification ./internal/integration
GOCACHE=/tmp/oaw-go-build-ticket03 go vet ./internal/classification ./internal/integration
```

Expected: PASS; the classifier package remains at least 90% covered.

- [ ] **Step 4: Commit the corpus and integration proof**

```bash
git add internal/classification internal/integration
git commit -m "test: verify classification safety corpus"
```

### Task 5: Ticket 03 completion verification

**Files:**
- Modify only Ticket 03 files above if a verification failure requires a
  scoped correction.

- [ ] **Step 1: Run formatting, vet, and race tests**

```bash
test -z "$(gofmt -l cmd/oaw/*.go internal/*/*.go)"
GOCACHE=/tmp/oaw-build-ticket03 go vet ./...
GOCACHE=/tmp/oaw-build-ticket03 go test -race ./...
git diff --check
```

- [ ] **Step 2: Enforce repository-wide and classifier coverage**

```bash
GOCACHE=/tmp/oaw-build-ticket03 go test -coverprofile=/tmp/oaw-ticket-03-cover.out ./...
GOCACHE=/tmp/oaw-build-ticket03 go tool cover -func=/tmp/oaw-ticket-03-cover.out
GOCACHE=/tmp/oaw-build-ticket03 go tool cover -func=/tmp/oaw-ticket-03-cover.out | awk '/^total:/ { gsub("%", "", $3); if (($3 + 0) < 80) exit 1 }'
```

The repository total must remain at least 80%; classification statements must
remain at least 90%.

- [ ] **Step 3: Check forbidden surfaces and security invariants**

Confirm the Ticket 03 diff does not touch `install.sh`, `lib/`, `tests/`,
`cmd/oaw`, or `internal/cli`; contains no Shell/process/network/model API;
rejects unknown fields and traits; never parses free-form request text; never
selects a Provider or Capability; and only raises mode/risk/evidence through
user/project rules.

- [ ] **Step 4: Commit only scoped corrections**

If Steps 1-3 changed tracked Ticket 03 files:

```bash
git add internal/assets/schemas/v1/classification-proposal.schema.json internal/schema internal/classification internal/integration
git commit -m "test: harden deterministic classifier verification"
```

Do not create an empty commit or add a CLI/runtime surface in this ticket.

## Self-Review Record

- Spec sections 5 and 6 map to the closed proposal, orthogonal mode/complexity/
  risk decision, conservative fallback, exact Bounded selector requirement,
  and Workflow-only Startup Gate boundary in Tasks 1-3.
- Spec sections 14, 15, and 18 map to inert records, strict schema decoding,
  no model/network execution, immutable values, deterministic digests, race
  tests, and the critical-release corpus in Tasks 1-5.
- Ticket 03 intentionally does not compile Profiles, resolve Provider
  Instances, issue Grants, or add `oaw run`; those are Tickets 04-05.
- Detection reports and Classification Decisions never select a Profile,
  Provider, or Capability. A Bounded selector is preserved only as explicit
  user/trusted intent for the later admission layer.
