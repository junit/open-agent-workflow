# OAW Core Coordinator Phase 05 Policy and Documentation Cutover Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans in the current session to implement this plan task-by-task. This plan is locked to `CURRENT`; do not dispatch subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the canonical Policy, bilingual product documentation, decision records, changelog, and documentation gates describe only OAW Core, the optional Workflow Coordinator, and Host-owned `CURRENT` or native `SUBAGENT` execution.

**Architecture:** Treat `policy/ENGINEERING.md` as the normative portable contract and every other product document as a projection of the same vocabulary. Historical plans and ADR bodies remain available as history, but status banners make supersession explicit; active documentation and executable docs gates reject every removed execution claim.

**Tech Stack:** Markdown, Bash 3.2-compatible documentation gates, Go embedded-policy tests, bilingual English/Simplified Chinese documentation.

---

**Selected execution:** `CURRENT`. Do not dispatch subagents. Do not publish or push.

**Depends on:** Phase 04 has deleted the execution Runtime and restored the full repository test suite to GREEN.

**Produces:** Canonical product language and decision status for the hard cutover; no historical compatibility contract.

## File Map

| Path | Responsibility |
| --- | --- |
| `policy/ENGINEERING.md` | Normative Request Mode, Startup Gate, Core, Coordinator, topology, ownership, and safety policy. |
| `policy_test.go` | Embedded-policy copy and required/forbidden contract assertions. |
| `README.md`, `README-zh.md` | Primary product boundary, quick start, topology, Profile, state, and verification guidance. |
| `SECURITY.md`, `SECURITY-zh.md` | Root security policy for logical versus physical authority and secret-free state. |
| `CHANGELOG.md` | Unreleased hard-cut removal and replacement record. |
| `docs/en/*.md`, `docs/zh/*.md` | Paired architecture, lifecycle, adapter, installer, security, troubleshooting, background, comparison, and extension documentation. |
| `docs/adr/0003-*.md`, `0007-*.md`, `0008-*.md`, `0009-*.md` | Explicit decision supersession and acceptance chain. |
| `docs/superpowers/specs/2026-08-04-oaw-host-native-execution-topology-design.md` | Superseded design banner. |
| `docs/superpowers/specs/2026-08-05-oaw-core-coordinator-hard-cutover-design.md` | Approved design status. |
| `scripts/check-docs.sh` | Fast paired-doc, required-claim, forbidden-claim, and link gate. |
| `tests/10-docs-test.sh` | Black-box governance documentation contract. |

## Canonical Vocabulary

Use these terms exactly in active product documentation:

| Concept | Canonical term and claim |
| --- | --- |
| Required product logic | `OAW Core`; stateless classification, resolution, compilation, and Lifecycle Bundle construction. |
| Optional durable state | `Workflow Coordinator`; revisions, idempotency, cooperative Resource Leases, evidence, pause, cancellation, switching, and recovery. |
| Physical execution owner | `Agent Host`; owns Agents, model calls, MCP, Hooks, Skills, Plugins, authentication, tools, sandbox, and approvals. |
| Current-context topology | `CURRENT` / `Current session` / `当前会话`. |
| Child-context topology | `SUBAGENT` / `Subagent` / `子 Agent`; Host-native only. |
| Static Host surface | `policy` or `host-native`. |
| Durable state | `Workflow State` under `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/workflows`. |
| Logical authority | `Capability Grant` and cooperative `Resource Lease`; never a physical sandbox claim. |

The following are forbidden in canonical Policy and current product docs except
inside a clearly labeled removal table in `CHANGELOG.md`:

```text
Runtime Plane
Runtime-managed
oaw runtime exchange
oaw run --host codex
oaw/codex-runner
runner-managed
native-managed
INLINE
NATIVE_SUBAGENT
main-agent-allowed
isolated-required
private HOME
Codex Runner
```

Historical ADR bodies, completed plans, and superseded Specs are exempt from
the vocabulary scan only when their status banner points to ADR 0009 or the
2026-08-05 design.

## Task 1: Rewrite the canonical Policy around Core, Coordinator, and Host

**Files:**
- Modify: `policy/ENGINEERING.md`
- Modify: `policy_test.go`

- [ ] **Step 1: Write failing embedded-policy contract assertions**

Extend `policy_test.go` with `TestCanonicalPolicyDefinesCoreCoordinatorHostBoundary` and `TestCanonicalPolicyRejectsRemovedExecutionContracts`. Require these literals:

```go
required := []string{
    "OAW Core",
    "Workflow Coordinator",
    "Agent Host",
    "CURRENT",
    "SUBAGENT",
    "Only Workflow Mode runs the Startup Gate",
    "DIRECT and BOUNDED do not create Workflow State",
    "OAW never starts a model process",
    "The Agent Host owns physical execution authority",
}
forbidden := []string{
    "Runtime Plane", "Runtime-managed", "oaw runtime exchange",
    "oaw run --host codex", "oaw/codex-runner", "runner-managed",
    "native-managed", "INLINE", "NATIVE_SUBAGENT",
    "main-agent-allowed", "isolated-required", "private HOME",
}
```

Use byte containment against `CanonicalPolicy()` and report the exact missing
or forbidden literal.

- [ ] **Step 2: Run the policy tests to verify RED**

```bash
rtk go test . -run 'CanonicalPolicy'
```

Expected: FAIL on missing Core/Coordinator/Host clauses and old Runtime terms.

- [ ] **Step 3: Replace the Policy structure and execution claims**

Use this exact top-level order:

```text
Purpose and Authority Boundaries
Request Classification
Direct Mode
Bounded Mode
Workflow Mode
Mandatory Startup Gate
OAW Core
Provider, Capability, and Profile Model
Execution Topologies
Workflow Coordinator
Agent Host Integration
Lifecycle Bundle and Lock
Matt-Superpowers Hybrid
Bounded Add-ons and Security
Artifacts and Projections
Stable Switching
Neutral Safety Rules
```

Lock these normative clauses in the corresponding sections:

- `DIRECT` has no Capability, Profile, Bundle, Startup Gate, or Workflow State.
- `BOUNDED` selects one exact Capability and one terminal deliverable; it has no Profile or Workflow State.
- `WORKFLOW` alone runs the blocking Startup Gate and produces a Bundle after explicit Profile, topology, and add-on selection.
- Core is required and stateless; callers never author a Bundle.
- Coordinator is optional, Workflow-only, and controls cooperating-client state transitions, not execution.
- `CURRENT` uses the active session unchanged.
- `SUBAGENT` means only a child created by the active Host's native facility; missing support makes it ineligible and creates no process fallback.
- Environment observations are `inherited`, `host-configured`, `restricted`, `unknown`, or `unavailable`; OAW neither reconstructs nor guarantees unreported Host state.
- Every built-in and user Provider uses the generic descriptor/binding/compiler path; `ECC-FULL` remains a complete lifecycle.
- Policy-only Hosts retain lifecycle coordination but make no atomic revision, lease, idempotency, or physical containment claim.

- [ ] **Step 4: Run GREEN and embedded-copy checks**

```bash
rtk gofmt -w policy_test.go
rtk go test . -run 'CanonicalPolicy'
rtk go test ./internal/management -run 'Policy|Render|Managed'
```

Expected: PASS; management still distributes the exact canonical bytes.

- [ ] **Step 5: Commit the normative Policy cutover**

```bash
rtk git add policy/ENGINEERING.md policy_test.go
rtk git commit -m "docs: replace runtime policy with core coordination"
```

## Task 2: Rewrite product identity, architecture, and lifecycle pairs

**Files:**
- Modify: `README.md`
- Modify: `README-zh.md`
- Modify: `docs/en/background.md`
- Modify: `docs/zh/background.md`
- Modify: `docs/en/architecture.md`
- Modify: `docs/zh/architecture.md`
- Modify: `docs/en/lifecycle.md`
- Modify: `docs/zh/lifecycle.md`
- Modify: `docs/en/comparison.md`
- Modify: `docs/zh/comparison.md`

- [ ] **Step 1: Add failing architecture and lifecycle assertions**

In `tests/10-docs-test.sh`, require both language pairs to contain:

```text
OAW Core
Workflow Coordinator
Agent Host
CURRENT
SUBAGENT
policy
host-native
Workflow State
SP-FULL
MATT-FULL
ECC-FULL
MATT-SP-HYBRID
USER-DEFINED
```

Require the English architecture to include this control flow, and the Chinese
pair to include the same node names:

```text
Request -> OAW Core -> Lifecycle Bundle -> Agent Host -> Receipt
                          |
                          +-> optional Workflow Coordinator
```

- [ ] **Step 2: Run the docs contract to verify RED**

```bash
rtk bash tests/10-docs-test.sh
```

Expected: FAIL on the old Runtime and Runner projections.

- [ ] **Step 3: Rewrite README and background boundaries**

Replace each `Cutover and Runtime Boundaries` section with `Core, Coordination,
and Host Boundaries`. State that installation management only distributes the
Policy, Core compiles lifecycle contracts, Coordinator is optional, and the
Host performs every effect. Keep management commands, supported targets,
Provider diagnostics, approved Profile ownership, comparison caveats, and
release-smoke guidance unchanged unless their wording contains a removed term.

- [ ] **Step 4: Rewrite the architecture pair**

Document four modules: Distribution, Core, optional Coordinator, and external
Agent Host. Keep Install State paths and management transaction semantics.
Replace the old state section with:

```text
Install State: .../open-agent-workflow/installations/
Workflow State: .../open-agent-workflow/workflows/
Relationship: disjoint namespaces; no migration or implicit adoption
```

Explain that project Workflow documents are one-way, non-authoritative
projections and that Resource Leases coordinate compliant Workflows rather
than locking the operating system.

- [ ] **Step 5: Rewrite the lifecycle and comparison pairs**

Show the three Request Modes, one Workflow Gate, generic Provider compilation,
the two topology choices, topology eligibility intersection, exact Bundle
inheritance, stable switching, and the Matt-Superpowers ownership table. Keep
the approved numeric family scores and their experience-based/non-benchmark
caveats. Do not imply that a Provider brand fixes its role; Recipes assign
responsibilities.

- [ ] **Step 6: Run paired-doc and vocabulary checks**

```bash
rtk bash tests/10-docs-test.sh
rtk bash scripts/check-docs.sh
```

Expected: the new assertions pass; any remaining failure names an exact stale
literal or missing bilingual contract.

- [ ] **Step 7: Commit product and architecture documentation**

```bash
rtk git add README.md README-zh.md docs/en/background.md docs/zh/background.md docs/en/architecture.md docs/zh/architecture.md docs/en/lifecycle.md docs/zh/lifecycle.md docs/en/comparison.md docs/zh/comparison.md tests/10-docs-test.sh
rtk git commit -m "docs: explain core coordinator and host boundaries"
```

## Task 3: Rewrite adapter, installer, extension, and troubleshooting pairs

**Files:**
- Modify: `docs/en/adapters.md`
- Modify: `docs/zh/adapters.md`
- Modify: `docs/en/extending-adapters.md`
- Modify: `docs/zh/extending-adapters.md`
- Modify: `docs/en/installer.md`
- Modify: `docs/zh/installer.md`
- Modify: `docs/en/troubleshooting.md`
- Modify: `docs/zh/troubleshooting.md`

- [ ] **Step 1: Replace the old adapter contract assertions**

In `tests/10-docs-test.sh`, remove Runner containment requirements such as
`--ignore-user-config`, `--ignore-rules`, `--disable hooks`, Runner signals,
and pinned Codex claims. Require adapter docs to distinguish:

```text
policy: instruction distribution only; CURRENT; no Coordinator guarantee
host-native: session facts + Core/Coordinator protocol + Host execution
SUBAGENT availability is session-dependent
all nine built-in integrations are policy surfaces
```

Require extension docs to state that a Host integration reports facts and
receipts but never gives OAW a model command, credential, Hook payload, or
private Plugin/MCP configuration.

- [ ] **Step 2: Run the focused documentation gate to verify RED**

```bash
rtk bash tests/10-docs-test.sh
```

- [ ] **Step 3: Rewrite adapter and extension documentation**

Preserve target paths, scope support, precedence, reload behavior, evidence
dates, ownership modes, renderer purity, collision handling, hostile-path and
symlink fixtures. Replace Runtime support levels with `policy` and
`host-native`; describe session reports, topology-aware binding inventories,
Dispatch Packets, and Receipts without claiming invocation containment.

- [ ] **Step 4: Rewrite installer and troubleshooting state guidance**

Preserve public management syntax, exits, state fingerprints, backups,
rollback, and wrapper behavior. Replace every Install-State/Runtime-State
comparison with Install State versus optional Workflow State. Add exact
diagnosis for `SCHEMA_UNSUPPORTED`, `WORKFLOW_STATE_UNSUPPORTED`,
`SUBAGENT_UNAVAILABLE`, `HOST_SESSION_CHANGED`, and explicit pre-release state
reset; never instruct OAW to delete an unknown state root automatically.

- [ ] **Step 5: Run docs and management behavior checks**

```bash
rtk bash tests/10-docs-test.sh
rtk bash scripts/check-docs.sh
rtk bash tests/01-cli-test.sh
rtk bash tests/05-policy-coordination-test.sh
```

Expected: PASS; documentation changes do not alter management behavior.

- [ ] **Step 6: Commit operating documentation**

```bash
rtk git add docs/en/adapters.md docs/zh/adapters.md docs/en/extending-adapters.md docs/zh/extending-adapters.md docs/en/installer.md docs/zh/installer.md docs/en/troubleshooting.md docs/zh/troubleshooting.md tests/10-docs-test.sh
rtk git commit -m "docs: document host-native integration surfaces"
```

## Task 4: Rewrite security boundaries and the changelog

**Files:**
- Modify: `SECURITY.md`
- Modify: `SECURITY-zh.md`
- Modify: `docs/en/security.md`
- Modify: `docs/zh/security.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add failing security-boundary assertions**

Require both security pairs to contain `logical workflow authority`, `Host
sandbox and approvals`, `secret-free`, `opaque digest`, `cooperating clients`,
and `OAW never starts a model CLI`. Reject claims that a Grant contains Host
tools or that OAW guarantees MCP/Hook/Skill/Plugin inheritance.

- [ ] **Step 2: Run security and docs tests to verify RED**

```bash
rtk bash tests/06-security-test.sh
rtk bash tests/10-docs-test.sh
```

- [ ] **Step 3: Rewrite the security documents**

Keep installer path, symlink, inert-state, backup, transaction, and reporting
guidance. Define Core inputs and Coordinator state as secret-free; list API
keys, tokens, private Hook payloads, raw Provider output, and full MCP/Plugin
config as forbidden storage. State that a Grant may be narrower than the Host
sandbox but cannot physically stop out-of-protocol Host actions.

- [ ] **Step 4: Add an Unreleased changelog hard-cut entry**

Under `Unreleased`, record these exact categories:

```text
Added: OAW Core, optional Workflow Coordinator, CURRENT/SUBAGENT, Host session reports
Changed: Provider v3, Recipe v2, user config v3, Host v2, Grant v2, Workflow v1
Removed: oaw run --host codex, oaw runtime exchange, Codex Runner, private HOME/Skill staging, old schemas/state readers/aliases
Security: Host retains physical authority; Coordinator records no credentials or private extension configuration
```

The removal list is the only current product file allowed to quote obsolete
command names.

- [ ] **Step 5: Run GREEN checks and commit**

```bash
rtk bash tests/06-security-test.sh
rtk bash tests/10-docs-test.sh
rtk bash scripts/check-docs.sh
rtk git add SECURITY.md SECURITY-zh.md docs/en/security.md docs/zh/security.md CHANGELOG.md tests/06-security-test.sh tests/10-docs-test.sh scripts/check-docs.sh
rtk git commit -m "docs: define core coordinator security boundary"
```

## Task 5: Accept ADR 0009 and close the supersession chain

**Files:**
- Modify: `docs/adr/0003-add-optional-capability-admission-runtime.md`
- Modify: `docs/adr/0007-use-host-native-execution-topologies.md`
- Modify: `docs/adr/0008-treat-subagent-environment-as-host-owned.md`
- Modify: `docs/adr/0009-separate-core-coordination-and-host-execution.md`
- Modify: `docs/superpowers/specs/2026-08-04-oaw-host-native-execution-topology-design.md`
- Modify: `docs/superpowers/specs/2026-08-05-oaw-core-coordinator-hard-cutover-design.md`

- [ ] **Step 1: Write failing status assertions in `scripts/check-docs.sh`**

Require these exact status lines:

```text
ADR 0003: Superseded by ADR 0009
ADR 0007: Superseded by ADR 0009; Host-native control is retained
ADR 0008: Superseded by ADR 0009; Host-owned environment semantics are retained
ADR 0009: Accepted
2026-08-04 Spec: Superseded by the 2026-08-05 OAW Core and Workflow Coordinator hard-cutover design
2026-08-05 Spec: Approved for implementation
```

- [ ] **Step 2: Run the gate to verify RED**

```bash
rtk bash scripts/check-docs.sh
```

- [ ] **Step 3: Update status banners without rewriting history**

Change only status and short supersession notes in ADRs 0003/0007/0008 and the
2026-08-04 Spec. Set ADR 0009 and the 2026-08-05 Spec to their accepted states.
Do not rewrite historical decision bodies to use current vocabulary; their
value is explaining why the rejected designs existed.

- [ ] **Step 4: Run status and link checks**

```bash
rtk bash scripts/check-docs.sh
rtk rg -n '^## Status|^\*\*Status:' docs/adr/0003-add-optional-capability-admission-runtime.md docs/adr/0007-use-host-native-execution-topologies.md docs/adr/0008-treat-subagent-environment-as-host-owned.md docs/adr/0009-separate-core-coordination-and-host-execution.md docs/superpowers/specs/2026-08-04-oaw-host-native-execution-topology-design.md docs/superpowers/specs/2026-08-05-oaw-core-coordinator-hard-cutover-design.md
```

Expected: one unambiguous active decision, ADR 0009.

- [ ] **Step 5: Commit decision status**

```bash
rtk git add docs/adr/0003-add-optional-capability-admission-runtime.md docs/adr/0007-use-host-native-execution-topologies.md docs/adr/0008-treat-subagent-environment-as-host-owned.md docs/adr/0009-separate-core-coordination-and-host-execution.md docs/superpowers/specs/2026-08-04-oaw-host-native-execution-topology-design.md docs/superpowers/specs/2026-08-05-oaw-core-coordinator-hard-cutover-design.md scripts/check-docs.sh
rtk git commit -m "docs: accept core coordinator architecture"
```

## Task 6: Make stale execution claims an executable repository failure

**Files:**
- Modify: `scripts/check-docs.sh`
- Modify: `tests/10-docs-test.sh`

- [ ] **Step 1: Add a scoped forbidden-vocabulary scan**

Scan these current sources:

```text
policy/ENGINEERING.md
README.md README-zh.md
SECURITY.md SECURITY-zh.md
docs/en/*.md docs/zh/*.md
```

Use the forbidden list in this plan. Do not scan `docs/adr`,
`docs/superpowers`, or `CHANGELOG.md`. Print file, line, and literal for every
failure.

- [ ] **Step 2: Add required boundary assertions**

Require every architecture/lifecycle/security pair to identify Core,
Coordinator, Host, both topologies, and the physical/logical authority split.
Keep the existing bilingual pairing, management commands, comparison scores,
adapter paths, security, and black-box installer assertions.

- [ ] **Step 3: Run the complete Phase 05 gate**

```bash
rtk go test ./...
rtk bash tests/10-docs-test.sh
rtk bash scripts/check-docs.sh
rtk bash tests/15-host-execution-boundary-test.sh
rtk git diff --check
```

Expected: PASS; source-boundary and docs-boundary scans agree.

- [ ] **Step 4: Commit the permanent documentation gate**

```bash
rtk git add scripts/check-docs.sh tests/10-docs-test.sh
rtk git commit -m "test: reject stale execution documentation"
```

## Phase 05 Completion Gate

- [ ] Canonical Policy contains no execution Runtime, process Runner, private environment, or removed topology claim.
- [ ] English and Chinese documents use the same Core/Coordinator/Host model and state paths.
- [ ] `CURRENT` and `SUBAGENT` are the only active topology names.
- [ ] All built-in integrations are documented honestly as `policy` until a real Host-native integration exists.
- [ ] ADR 0009 is Accepted; ADRs 0003, 0007, and 0008 and the 2026-08-04 Spec have explicit supersession banners.
- [ ] Historical bodies remain readable but cannot be mistaken for current contracts.
- [ ] Docs, policy embedding, management, security, and source-boundary gates pass.
