# Explicit OAW Activation and Policy-Cooperative Degradation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make OAW completely opt-in per deliverable, preserve Native Host behavior until explicit activation, and give instruction-only Hosts a truthful `policy-cooperative` OAW contract.

**Architecture:** Installation continues to add a small Host-specific instruction, but that instruction becomes one shared semantic Activation Router instead of an always-on workflow bootstrap. After explicit activation, OAW determines its assurance level and then classifies only that Engagement as `DIRECT`, `BOUNDED`, or `WORKFLOW`; instruction-only Hosts cooperate through plans and trackers rather than impersonating Core or Coordinator records.

**Tech Stack:** Go management package, POSIX/Bash compatibility renderer, Markdown policy and bilingual documentation, shell black-box tests, Go unit tests, existing documentation checker.

---

## Scope and Guardrails

Implement only the approved design in
`docs/superpowers/specs/2026-08-12-oaw-explicit-activation-policy-cooperative-design.md`.

Do not change in this release:

- Classification, Provider, Profile, Bundle, Coordinator, Receipt, Evidence, or Bridge schemas.
- Core classification rules, the ten-slot engineering-delivery taxonomy, Profile Recipes, execution graphs, or Coordinator transition protocols.
- The `bounded_capability_defaults` data shape or claim that it supports trusted automatic routing.
- Host-native Bridge contracts or their current fail-closed guarantees.
- Active legacy Markdown task records. They are not migrated to Progress Trackers.

The worktree is already dirty. Before editing a file, inspect its existing diff
and preserve it. In particular, retain the current `Startup Gate Host capability
probe` language while moving it into the machine-backed Startup Gate. Stage
only this plan's hunks with `git add -p`; never use `git add -A`, destructive
reset, or checkout/restore against working-tree files.

## File Map

| File | Responsibility |
| --- | --- |
| `internal/management/render.go` | Canonical Go Activation Router and Host wrappers. |
| `internal/management/render_test.go` | Exact bytes and positive/negative Router contract. |
| `lib/render.sh` | Shell implementation of the same Router bytes. |
| `internal/management/update_test.go` | Valid legacy managed-block migration and repeat-update idempotency. |
| `internal/management/force_test.go` | Marker recovery reconstructs the new Router. |
| `tests/04-core-adapters-test.sh` | User-scope lazy Router behavior. |
| `tests/05-project-adapters-test.sh` | Project adapters, wrappers, migration, preservation, and idempotency. |
| `tests/12-install-parity-test.sh` | Existing Go/shell install parity regression. |
| `tests/13-mutation-parity-test.sh` | Existing Go/shell update/force/uninstall parity regression. |
| `policy/ENGINEERING.md` | Normative activation, assurance, cooperative mode, and stop conditions. |
| `scripts/check-docs.sh` | Policy and bilingual release-boundary checks. |
| `tests/10-docs-test.sh` | Checker fixture and documentation black-box tests. |
| `README.md`, `README-zh.md` | Product entrypoint and migration behavior. |
| `docs/en|zh/*.md`, `SECURITY*.md`, `CHANGELOG.md` | Matching operating, trust, and release documentation. |

## Canonical Router Contract

Every supported target body must convey these exact bytes, with only the policy
path interpolated. Cursor, Windsurf, and Copilot prepend only their required
frontmatter.

```text
Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. On explicit activation, read `<POLICY_PATH>` and apply it only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit closes the OAW Engagement.
```

An always-visible Router is not an activation. Claude and Gemini must no longer
emit `@<POLICY_PATH>`, because that imports the full policy eagerly.

### Task 1: Specify the Activation Router Before Changing Renderers

**Files:**

- Modify: `internal/management/render_test.go:9-106`
- Modify: `internal/management/update_test.go:1-360`
- Modify: `internal/management/force_test.go:48-85`
- Modify: `tests/04-core-adapters-test.sh:11-224`
- Modify: `tests/05-project-adapters-test.sh:1-760`

- [ ] **Step 1: Record the focused baseline.**

Run:

```bash
rtk go test ./internal/management -run 'TestRender|TestPrepareUpdate|TestPrepareUpdateForce' -count=1
rtk go test ./internal/management -cover
```

Expected: tests pass and package coverage is reported. This proves only that the
old eager behavior is the current baseline.

- [ ] **Step 2: Replace exact renderer fixtures with one Router expectation.**

At the top of `TestRenderTargetMatchesBashBytes`, define:

```go
router := "Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. On explicit activation, read `" + policyPath + "` and apply it only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit closes the OAW Engagement.\n"
```

Use `router` for every unwrapped target. Preserve these wrappers exactly:

```go
cursor := "---\ndescription: Open Agent Workflow lifecycle policy\nglobs: \"**/*\"\nalwaysApply: true\n---\n\n" + router
windsurf := "---\ntrigger: always_on\n---\n\n" + router
copilot := "---\napplyTo: \"**\"\n---\n\n" + router
```

Keep all 13 supported `(scope, targetID)` pairs. Update
`TestRenderManagedBlockWrapsExactRendererBytes` to wrap `router` instead of the
old Codex classification string.

- [ ] **Step 3: Add semantic assertions that prevent eager regression.**

Add below `TestRenderTargetMatchesBashBytes`:

```go
func TestRenderTargetEnforcesActivationRouterContract(t *testing.T) {
	policyPath := "/config/ENGINEERING.md"
	targets := []struct {
		scope scope
		id    targetID
	}{
		{"user", "claude"}, {"user", "codex"}, {"user", "gemini"}, {"user", "opencode"},
		{"project", "claude"}, {"project", "codex"}, {"project", "gemini"}, {"project", "opencode"},
		{"project", "cursor"}, {"project", "windsurf"}, {"project", "cline"}, {"project", "roo"}, {"project", "copilot"},
	}
	for _, target := range targets {
		t.Run(string(target.scope)+"/"+string(target.id), func(t *testing.T) {
			rendered, err := renderTarget(target.id, target.scope, policyPath)
			if err != nil {
				t.Fatal(err)
			}
			text := string(rendered)
			for _, required := range []string{
				"Open Agent Workflow is opt-in.",
				"explicitly asks to use OAW",
				"behave as the native Host",
				"do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state",
				"ordinary Skill invocation do not activate OAW",
				"On explicit activation, read `" + policyPath + "`",
				"apply it only to that deliverable",
				"Related follow-ups inherit activation; unrelated requests remain native",
				"explicit exit closes the OAW Engagement",
			} {
				if !strings.Contains(text, required) {
					t.Fatalf("%s/%s omits %q: %q", target.scope, target.id, required, text)
				}
			}
			for _, forbidden := range []string{
				"\n@" + policyPath + "\n",
				"For every new top-level engineering request, first read",
				"Before engineering lifecycle work, read",
				"classify it as DIRECT, BOUNDED, or WORKFLOW",
				"follow its blocking selection gate",
				"preserve the selected lifecycle bundle",
			} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("%s/%s retains forbidden %q: %q", target.scope, target.id, forbidden, text)
				}
			}
		})
	}
}
```

- [ ] **Step 4: Add an ordinary-update migration test for the exact legacy Claude block.**

Add `TestUpdateMigratesLegacyManagedBlockToActivationRouter` to
`internal/management/update_test.go`. Use `newPrepareFixture`,
`materializeInstallRequest`, `Update`, `parseInstallationState`, and
`serializeInstallState`.

The test must:

1. Install user-scoped `claude`.
2. Build the exact legacy managed block shown below.
3. Put byte sentinels `before\n` and `after\n` around it.
4. Change only the installed Claude state record checksum to
   `checksumBytes(legacyBlock)` and serialize the state.
5. Call `Update` with `NewSource("0.2.0", []byte("updated canonical policy\n"))`.
6. Assert sentinels survive, exactly one marker pair remains, eager text/import
   are absent, and `Open Agent Workflow is opt-in.` is present.
7. Render the expected new block, handle its error, and assert the new state
   checksum equals `checksumBytes(expectedBlock)`.
8. Snapshot target, policy, and state; call `Update` again; assert lines equal
   `oaw: unchanged: claude`, `oaw: unchanged: policy`, and
   `oaw: unchanged: state`, and every snapshot is identical.

Use this exact construction:

```go
legacyBlock := []byte(beginMarker + "\n" +
	"Before any new top-level engineering task that may use workflow skills, read and follow the Open Agent Workflow policy:\n" +
	"@" + installed.policyAction.destination + "\n" + endMarker + "\n")
targetBytes := append([]byte("before\n"), legacyBlock...)
targetBytes = append(targetBytes, []byte("after\n")...)
writePrepareFile(t, installed.targetActions[0].destination, targetBytes, 0o644)
```

Locate the Claude record by index, replace its checksum, serialize the state,
and write it to `installed.stateActions[0].destination` before update.

Use this assertion shape after update:

```go
expectedBlock, err := renderManagedBlock("claude", "user", installed.policyAction.destination)
if err != nil {
	t.Fatal(err)
}
updatedState, exists, err := readInstallationState(installed.stateActions[0].destination)
if err != nil || !exists {
	t.Fatalf("updated state: exists=%v err=%v", exists, err)
}
if got := findPreparedRecord(t, updatedState.targets, "claude").checksum; got != checksumBytes(expectedBlock) {
	t.Fatalf("updated checksum = %q, want %q", got, checksumBytes(expectedBlock))
}
```

- [ ] **Step 5: Extend force-marker recovery assertions.**

Inside `TestPrepareUpdateForceRepairsSingleMissingMarker`, after marker counts,
add:

```go
if !bytes.Contains(rendered, []byte("Open Agent Workflow is opt-in.")) {
	t.Fatalf("repaired target does not contain the activation router: %q", rendered)
}
if bytes.Contains(rendered, []byte("\n@"+installed.policyAction.destination+"\n")) {
	t.Fatalf("repaired target retains eager policy import: %q", rendered)
}
```

Keep the existing equality assertion. The current bytes and repaired bytes are
both rendered from the same checkout.

- [ ] **Step 6: Make user and project adapter tests assert lazy semantics.**

Add this helper separately to `tests/04-core-adapters-test.sh` and
`tests/05-project-adapters-test.sh`:

```bash
assert_lazy_router_file() {
  router_file=$1
  router_policy=$2
  router_description=$3

  grep -F 'Open Agent Workflow is opt-in.' "$router_file" >/dev/null ||
    fail "$router_description is missing opt-in activation"
  grep -F 'behave as the native Host' "$router_file" >/dev/null ||
    fail "$router_description does not preserve Native Host behavior"
  grep -F "On explicit activation, read \`$router_policy\`" "$router_file" >/dev/null ||
    fail "$router_description does not retain the canonical policy path"
  grep -F 'ordinary Skill invocation do not activate OAW' "$router_file" >/dev/null ||
    fail "$router_description incorrectly governs normal Skill routing"
  if grep -F "@$router_policy" "$router_file" >/dev/null ||
    grep -F 'For every new top-level engineering request, first read' "$router_file" >/dev/null ||
    grep -F 'Before engineering lifecycle work, read' "$router_file" >/dev/null ||
    grep -F 'classify it as DIRECT, BOUNDED, or WORKFLOW' "$router_file" >/dev/null; then
    fail "$router_description retains eager OAW activation"
  fi
}
```

In `tests/04-core-adapters-test.sh`, replace old exact eager blocks for Claude,
Codex, Gemini, and OpenCode with the Router. Retain marker count, personal
content, state, and idempotency checks. Reverse the Gemini assertion so
`@$OAW_POLICY` must be absent.

In `tests/05-project-adapters-test.sh`, replace every body with the Router while
retaining Cursor/Windsurf/Copilot frontmatter exactly. Assert no eager import in
Claude/Gemini. For shared Codex/OpenCode `AGENTS.md`, retain one block and equal
path/checksum assertions. In the copied-checkout update loop, assert every
updated target contains the Router and excludes all old eager phrases/imports.

- [ ] **Step 7: Run expected RED tests before implementation.**

Run:

```bash
rtk go test ./internal/management -run 'TestRender(Target|ManagedBlock)|TestUpdateMigratesLegacyManagedBlockToActivationRouter|TestPrepareUpdateForceRepairsSingleMissingMarker' -count=1
```

Expected: failures mention missing `Open Agent Workflow is opt-in.` or retained
eager text. The migration test must reach its semantic assertion rather than
being rejected as untracked drift.

For black-box RED verification, create a disposable sibling release:

```bash
OAW_ROUTER_TEST_RELEASE=$(mktemp -d "${TMPDIR:-/tmp}/oaw-router-tests.XXXXXX")
rtk cp install.sh "$OAW_ROUTER_TEST_RELEASE/install.sh"
rtk chmod 755 "$OAW_ROUTER_TEST_RELEASE/install.sh"
rtk go build -o "$OAW_ROUTER_TEST_RELEASE/oaw" ./cmd/oaw
OAW_INSTALLER="$OAW_ROUTER_TEST_RELEASE/install.sh" rtk bash tests/04-core-adapters-test.sh
OAW_INSTALLER="$OAW_ROUTER_TEST_RELEASE/install.sh" rtk bash tests/05-project-adapters-test.sh
```

Expected: updated assertions fail against the old renderers. Do not commit the
red state.

### Task 2: Implement the One-Source Router and Prove Installer Parity

**Files:**

- Modify: `internal/management/render.go:3-38`
- Modify: `lib/render.sh:6-90`
- Modify: Task 1 tests only if a fixture detail, not the contract, is wrong

- [ ] **Step 1: Implement the canonical Go Router.**

Add:

```go
const activationRouterFormat = "Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. On explicit activation, read `%s` and apply it only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit closes the OAW Engagement.\n"

func renderActivationRouter(policyPath string) string {
	return fmt.Sprintf(activationRouterFormat, policyPath)
}
```

Use `renderActivationRouter(policyPath)` for all unwrapped targets. Preserve
only these target-specific cases:

```go
case "project:cursor":
	rendered = "---\ndescription: Open Agent Workflow lifecycle policy\nglobs: \"**/*\"\nalwaysApply: true\n---\n\n" + renderActivationRouter(policyPath)
case "project:windsurf":
	rendered = "---\ntrigger: always_on\n---\n\n" + renderActivationRouter(policyPath)
case "project:copilot":
	rendered = "---\napplyTo: \"**\"\n---\n\n" + renderActivationRouter(policyPath)
```

Make `renderProjectBootstrap` delegate to the helper or remove it. Do not change
managed-block placement, ownership, or state logic.

- [ ] **Step 2: Emit identical bytes from the shell renderer.**

Add to `lib/render.sh`:

```bash
render_activation_router() {
  printf 'Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. On explicit activation, read `%s` and apply it only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit closes the OAW Engagement.\n' "$1"
}
```

Make `render_claude`, `render_codex`, `render_gemini`, `render_opencode`, and
`render_project_bootstrap` delegate to it. Keep wrapper metadata and
`render_target_content` dispatch unchanged. Do not emit `@`.

- [ ] **Step 3: Run focused Go tests and coverage.**

Run:

```bash
rtk go test ./internal/management -run 'TestRender|TestUpdateMigratesLegacyManagedBlockToActivationRouter|TestPrepareUpdateForceRepairsSingleMissingMarker' -count=1
rtk go test ./internal/management -cover
```

Expected: all pass and modified-code coverage remains at least 80%.

- [ ] **Step 4: Run black-box user/project adapter tests.**

```bash
OAW_ROUTER_TEST_RELEASE=$(mktemp -d "${TMPDIR:-/tmp}/oaw-router-tests.XXXXXX")
rtk cp install.sh "$OAW_ROUTER_TEST_RELEASE/install.sh"
rtk chmod 755 "$OAW_ROUTER_TEST_RELEASE/install.sh"
rtk go build -o "$OAW_ROUTER_TEST_RELEASE/oaw" ./cmd/oaw
OAW_INSTALLER="$OAW_ROUTER_TEST_RELEASE/install.sh" rtk bash tests/04-core-adapters-test.sh
OAW_INSTALLER="$OAW_ROUTER_TEST_RELEASE/install.sh" rtk bash tests/05-project-adapters-test.sh
```

Expected: both pass with content preservation, wrappers, shared destinations,
migration, repeat operations, and uninstall unchanged.

- [ ] **Step 5: Run Go/shell parity.**

```bash
rtk bash tests/12-install-parity-test.sh
rtk bash tests/13-mutation-parity-test.sh
```

Expected: both pass across install, update, force, and uninstall matrices.

- [ ] **Step 6: Commit the green renderer unit.**

```bash
rtk git diff --check -- internal/management/render.go internal/management/render_test.go internal/management/update_test.go internal/management/force_test.go lib/render.sh tests/04-core-adapters-test.sh tests/05-project-adapters-test.sh
rtk git diff -- internal/management/render.go internal/management/render_test.go internal/management/update_test.go internal/management/force_test.go lib/render.sh tests/04-core-adapters-test.sh tests/05-project-adapters-test.sh
```

Stage only new hunks with `git add -p`, then:

```bash
rtk git diff --cached --check
rtk git diff --cached --stat
rtk git commit -m "feat: make OAW activation opt-in"
```

Expected: only Router renderers and their tests are committed.

### Task 3: Define the Normative Explicit-Activation and Cooperative Policy

**Files:**

- Modify: `tests/10-docs-test.sh:57-316, 600-994`
- Modify: `scripts/check-docs.sh:20-35, 79-145, 399-453`
- Modify: `policy/ENGINEERING.md:1-433`

- [ ] **Step 1: Add failing policy contract checks before rewriting policy.**

In `scripts/check-docs.sh`, add an activation-policy fixture after the current
release boundaries. Require these exact literals from `policy/ENGINEERING.md`:

```text
Native Host is the default. It is not an OAW Request Mode.
Request Mode is evaluated only after explicit activation.
Assurance Level is orthogonal to Request Mode.
policy-cooperative
core-backed
coordinator-backed
Activated `BOUNDED` is not a generic Skill router.
The current `bounded_capability_defaults` interface does not define a matching predicate
Policy-only execution supports `CURRENT`. It cannot declare `SUBAGENT` eligible
Policy Workflow Plan
Progress Tracker
CAPABILITY_SELECTION_REQUIRED
POLICY_ONLY_PROVIDER_UNVERIFIED
POLICY_ONLY_PROFILE_INCOMPLETE
POLICY_ONLY_TOPOLOGY_UNAVAILABLE
POLICY_ONLY_GUARANTEE_UNAVAILABLE
POLICY_ONLY_CONCURRENT_MUTATION
POLICY_ONLY_EXECUTION_UNCERTAIN
POLICY_ONLY_CONTEXT_UNCERTAIN
```

Reject these stale literals:

```text
Classify every new top-level engineering request as exactly one Request Mode:
In policy-only use, the caller receives the same Core-produced Bundle
Policy-only Hosts may coordinate the same ownership model with a local lock
```

In `tests/10-docs-test.sh`, add matching `assert_contains` and
`assert_not_contains` checks after the current policy loop. Extend
`make_checker_fixture` to append every required literal to its fixture
`policy/ENGINEERING.md`; otherwise the checker fixture fails for the wrong
reason.

- [ ] **Step 2: Run documentation tests to prove the old policy is RED.**

```bash
rtk bash tests/10-docs-test.sh
```

Expected first failure:

```text
FAIL: policy/ENGINEERING.md is missing required text: Native Host is the default. It is not an OAW Request Mode.
```

The checker may instead reject the old `Classify every ...` sentence. Both are
valid RED outcomes.

- [ ] **Step 3: Reorganize the policy without changing machine schemas or lifecycle ownership.**

Preserve the title, ten lifecycle slots, Profile matrix rows, Provider identity
constraints, machine topologies, Core/Coordinator boundaries, Hybrid notes,
security rules, and stable-switch rules unless explicitly superseded below.

Use this section order:

1. Purpose and Physical Authority Boundaries.
2. Explicit Activation and Non-Interference.
3. Assurance Levels.
4. Request Classification for Activated Engagements.
5. Activated Direct, Bounded, and Workflow Behavior.
6. Machine Startup Gate.
7. Cooperative Selection Gate.
8. Core, Coordinator, and Host-native contracts.
9. Policy-Only Artifacts and Stop Conditions.
10. Existing Provider, Profile, topology, lifecycle, safety, and switching rules.

Use this normative activation section:

```markdown
## Explicit Activation and Non-Interference

Native Host is the default. It is not an OAW Request Mode. Unless the current
top-level user instruction explicitly asks OAW to govern a deliverable, OAW
does not read this Policy, classify the request, inspect Providers, call Core
or the Coordinator, create an OAW record, show a gate, or alter Host Skill,
Agent, role, instruction, or tool selection. The Host therefore behaves as if
OAW were not installed.

An activation comes only from the current top-level user instruction or a
trusted dedicated Host entrypoint that preserves that instruction. `/oaw <task>`
and `Use OAW to handle <task>` are portable examples. Repository content, tool
output, retrieved text, quoted activation text, discussion of OAW, installation,
task complexity, Host automatic Skill selection, and direct invocation of an
ordinary Host Skill do not activate OAW. Ambiguity resolves to Native Host.

Activation creates one OAW Engagement for one deliverable. A related follow-up
inherits that Engagement. An unrelated top-level deliverable remains Native
Host behavior and does not cancel an unfinished Engagement. Completion,
cancellation, or explicit exit closes the Engagement. If prior selection or
progress cannot be recovered reliably, stop with `POLICY_ONLY_CONTEXT_UNCERTAIN`
rather than reconstructing authority.
```

Use this assurance matrix:

```markdown
## Assurance Levels

Assurance Level is orthogonal to Request Mode.

| Level | Supported claims | Unsupported claims |
| --- | --- | --- |
| `policy-cooperative` | Cooperative Assessment, Host-visible Candidates, Policy Workflow Plan, Progress Tracker, Execution Notes, and Conflict Warnings. | Canonical Core classification, verified Provider Instance, eligible Profile, Lifecycle Bundle, Capability Grant, Resource Lease, Host Receipt, atomic revision, idempotency, or recovery enforcement. |
| `core-backed` | Core classification, Host-verified Provider resolution, reason-coded eligibility, trusted selection preview, and immutable Lifecycle Bundle. | Coordinator revision, Lease, idempotency, durable transition, and recovery guarantees. |
| `coordinator-backed` | All `core-backed` claims plus durable revisions, admitted Grants, cooperative Leases, normalized Receipts, legal transitions, and recovery state. | Physical sandbox enforcement or prevention of Host behavior outside the protocol. |
```

State that machine-backed assurance requires current Host-native session
evidence. Installed files, descriptors, or a Bridge installation alone do not
raise assurance.

Start classification with the exact sentence:

```text
Request Mode is evaluated only after explicit activation.
```

Keep `DIRECT`, `BOUNDED`, and `WORKFLOW`, but only as contracts inside an
Engagement. Native Host work has no OAW Request Mode.

- [ ] **Step 4: Define all activated mode behavior and both selection gates.**

Add the following normative rules:

- `DIRECT` is one small, clear, recoverable change with focused verification.
  It has no Profile gate, Lifecycle Bundle, Workflow State, or Policy Workflow
  Plan.
- Include the exact sentence `Activated \`BOUNDED\` is not a generic Skill
  router.` Bounded requires exactly one Capability, one named observable
  deliverable, declared cooperative effects/resources, one termination
  condition, and no lifecycle ownership, architecture decision, broad
  implementation ownership, remediation loop, or Git completion.
- An exact Skill/Capability named in the activated request is `user-explicit`.
  Otherwise show one Host-visible candidate and stop with
  `CAPABILITY_SELECTION_REQUIRED` until the user confirms.
- Include `The current \`bounded_capability_defaults\` interface does not define
  a matching predicate` and state it cannot be presented as implemented
  automatic trusted-rule routing.
- A second Capability, architecture decision, remediation loop, or wider
  effects/resources causes reclassification inside the same Engagement.
- Machine-backed Workflow keeps verified selection and immutable Bundle
  behavior. Policy-cooperative Workflow uses candidates, explicit user choice,
  `CURRENT`, a Policy Workflow Plan, and a Progress Tracker.

Split the current gate into `## Machine Startup Gate` and
`## Cooperative Selection Gate`. Move the already-edited `Startup Gate Host
capability probe` paragraph into Machine Startup Gate unchanged in meaning; it
remains the only controlled delegation exception.

The cooperative gate must perform these eight ordered actions:

1. Show `policy-cooperative`, Request Mode, complexity/risk assessment, and evidence.
2. Declare unavailable verified Provider, Bundle, Grant, Lease, Receipt, idempotency, and recovery guarantees.
3. Inspect only Host-visible metadata, configuration, candidates, and status.
4. Show complete, incomplete, and unavailable candidates and exact missing responsibilities.
5. Show only `CURRENT`.
6. Show every proposed Bounded add-on and its named deliverable.
7. Wait for explicit candidate, topology, add-on, and limitation acceptance.
8. Create Plan and Tracker before lifecycle work.

Include exactly:

```text
Policy-only execution supports `CURRENT`. It cannot declare `SUBAGENT` eligible because static instruction context is not current-session delegation evidence.
```

Call pre-gate policy/configuration/Provider metadata reading `Governance
Inspection`, and distinguish it from problem discovery, design, planning,
implementation, debugging, review, verification, and completion. If there is no
complete candidate, stop rather than waiting forever or inventing an owner.

- [ ] **Step 5: Reserve machine terminology and define cooperative artifacts.**

Add this exact table:

| Policy-only artifact or observation | Reserved machine term |
| --- | --- |
| Cooperative Assessment | Core Classification Decision |
| Host-visible Candidate | Verified Provider Instance |
| Bounded Plan | Capability Grant |
| Policy Workflow Plan | Lifecycle Bundle |
| Progress Tracker | Lifecycle Lock or Workflow State |
| Execution Note | Host Receipt |
| Conflict Warning | Resource Lease |

Define a Policy Workflow Plan with exactly these fields:

```text
assurance: policy-cooperative
activation_source: user-explicit
deliverable: <human-readable scope>
mode: WORKFLOW
complexity: <cooperative assessment>
risk: <cooperative assessment>
selected_profile_candidate: <id>
selection_source: user-explicit
topology: CURRENT
responsibility_map: <ten-slot candidate mapping>
accepted_limitations: <policy-only limitations>
status: active | completed | stopped
```

Call it explanatory human-readable content, not a schema. It must not fabricate
Bundle IDs, generations, digests, Grants, Leases, Receipts, or Coordinator
revisions. A Progress Tracker is best effort, not authoritative, atomic, or
guaranteed to survive context loss. Persistence may be claimed only after the
Host actually writes and recovers it in the project's existing docs layout.

Delete the claims that policy-only callers receive the same Core Bundle, that
policy-only Hosts coordinate through a local lock, or that Markdown lock state
persists authoritatively. Restrict Bundle and Workflow State sections to actual
Core/Coordinator records.

- [ ] **Step 6: Add all cooperative stop reasons and qualify switching/safety.**

Add this complete table:

| Reason | Required behavior |
| --- | --- |
| `CAPABILITY_SELECTION_REQUIRED` | Stop Bounded work until the user selects the candidate or a future exact trusted rule proves selection. |
| `POLICY_ONLY_PROVIDER_UNVERIFIED` | Stop work requiring a verified Provider or exact Binding guarantee. |
| `POLICY_ONLY_PROFILE_INCOMPLETE` | Stop Workflow selection when a necessary responsibility lacks a Host-visible candidate owner. |
| `POLICY_ONLY_TOPOLOGY_UNAVAILABLE` | Stop when requested OAW-managed topology is not `CURRENT`. |
| `POLICY_ONLY_GUARANTEE_UNAVAILABLE` | Stop work requiring Grant, Lease, Receipt, idempotency, atomic revision, or recovery enforcement. |
| `POLICY_ONLY_CONCURRENT_MUTATION` | Stop or serialize when another task may mutate overlapping project or Git resources. |
| `POLICY_ONLY_EXECUTION_UNCERTAIN` | Do not retry an external or destructive effect whose result is unknown. |
| `POLICY_ONLY_CONTEXT_UNCERTAIN` | Stop and require explicit reconfirmation when selection or progress cannot be recovered reliably. |

Scope expansion causes reclassification rather than a stop. At any cooperative
stop, only the user may explicitly exit OAW and return the request to Native
Host behavior.

Machine switching compiles a new Bundle. Cooperative switching requires
explicit candidate reselection and updates the Plan/Tracker. A policy-only Plan
cannot grant network, destructive, credential, deployment, data, or Git
authority beyond normal Host approval.

- [ ] **Step 7: Run policy checks and inspect preserved machine contracts.**

```bash
rtk scripts/check-docs.sh
rtk bash tests/10-docs-test.sh
rtk git diff --check -- policy/ENGINEERING.md scripts/check-docs.sh tests/10-docs-test.sh
rtk git diff --word-diff=plain -- policy/ENGINEERING.md
```

Expected: checks pass. Review the word diff to confirm no lifecycle slot owner,
Profile alias, Host action, neutral gate, macro contract, or machine schema term
changed accidentally.

- [ ] **Step 8: Commit the green policy unit.**

Because all three files already contain unrelated edits, use `git add -p` and
verify the index:

```bash
rtk git diff --cached --check
rtk git diff --cached --stat
rtk git commit -m "docs: define cooperative OAW policy"
```

Expected: only the rewritten policy and this feature's checker/test hunks are
committed; current Provider Surface v4 probe/Bridge changes remain preserved in
the worktree or are included only where the policy reorganization necessarily
moves their exact approved content.

### Task 4: Publish Matching Bilingual Documentation and Migration Guidance

**Files:**

- Modify: `README.md`, `README-zh.md`
- Modify: `docs/en/background.md`, `docs/zh/background.md`
- Modify: `docs/en/lifecycle.md`, `docs/zh/lifecycle.md`
- Modify: `docs/en/architecture.md`, `docs/zh/architecture.md`
- Modify: `docs/en/adapters.md`, `docs/zh/adapters.md`
- Modify: `docs/en/extending-adapters.md`, `docs/zh/extending-adapters.md`
- Modify: `docs/en/installer.md`, `docs/zh/installer.md`
- Modify: `docs/en/comparison.md`, `docs/zh/comparison.md`
- Modify: `docs/en/security.md`, `docs/zh/security.md`
- Modify: `docs/en/troubleshooting.md`, `docs/zh/troubleshooting.md`
- Modify: `docs/en/codex-bridge.md`, `docs/zh/codex-bridge.md`
- Modify: `SECURITY.md`, `SECURITY-zh.md`, `CHANGELOG.md`
- Modify: `scripts/check-docs.sh`, `tests/10-docs-test.sh`

- [ ] **Step 1: Add failing bilingual documentation contracts.**

Extend `scripts/check-docs.sh` with a compact activation-document fixture and
extend the `make_checker_fixture` function in `tests/10-docs-test.sh` to
provide its required literals. Add positive checks, in the relevant English and
Chinese files, for:

```text
Native Host
OAW Engagement
Assurance Preflight
policy-cooperative
core-backed
coordinator-backed
Policy Workflow Plan
Progress Tracker
CAPABILITY_SELECTION_REQUIRED
POLICY_ONLY_PROVIDER_UNVERIFIED
POLICY_ONLY_PROFILE_INCOMPLETE
POLICY_ONLY_TOPOLOGY_UNAVAILABLE
POLICY_ONLY_GUARANTEE_UNAVAILABLE
POLICY_ONLY_CONCURRENT_MUTATION
POLICY_ONLY_EXECUTION_UNCERTAIN
POLICY_ONLY_CONTEXT_UNCERTAIN
```

For Chinese prose, retain English machine identifiers and include explanations
using `原生 Host`, `OAW Engagement`, `保证等级预检`, `协作式 Policy Workflow
Plan`, and `Progress Tracker`.

Add negative checks across the relevant documents for:

```text
OAW performs enough read-only inspection to classify each top-level engineering
OAW Core classifies each new top-level engineering request
OAW 通过足够的只读检查，把每个顶层工程请求分类为
OAW Core 在选择工程方法前，对每个新顶层工程请求分类
Bounded Mode is the Atomic Skill mode.
Policy-only use follows the same lifecycle ownership rules
A policy-only lock
OAW writes a managed block containing `@<canonical-policy-path>`
Claude and Gemini use documented Markdown import behavior.
the lifecycle gate applies before engineering lifecycle work anywhere in the project
```

Change README heading checks from `## Task Gate`/`## 任务门禁` to
`## Explicit Activation`/`## 显式激活`. This makes the user-visible behavior
change a tested contract.

- [ ] **Step 2: Prove the public documents are RED before editing them.**

```bash
rtk scripts/check-docs.sh
rtk bash tests/10-docs-test.sh
```

Expected: the first error identifies an unchanged public document and a missing
activation contract, such as `README.md omits required release boundary: Native
Host`.

- [ ] **Step 3: Update README and background first.**

In both README files:

- Replace `Task Gate`/`任务门禁` with `Explicit Activation`/`显式激活`.
- State that install only distributes a lazy Router; it does not enroll ordinary
  work into OAW.
- Include the same flow in both languages:

```text
Native Host unless explicitly activated
    -> Assurance Preflight
    -> DIRECT / BOUNDED / WORKFLOW
    -> cooperative or machine-backed execution
```

- Give three examples: ordinary bug fix remains Host-native; ordinary direct
  Skill invocation remains Host-native; `/oaw` or natural-language activation
  creates one Engagement for one deliverable.
- Explain the behavior-breaking migration: ordinary requests are no longer
  always-on governed, old managed blocks update to the Router, and legacy
  Markdown locks are not automatically converted.

In `background.md`, replace installation-triggered arbitration wording with
explicit-deliverable governance. Preserve provider independence and
cross-client-overlap explanation; it now applies only after activation.

- [ ] **Step 4: Update lifecycle and architecture with the full operating model.**

In both `lifecycle.md` files:

- Insert Activation State and Assurance Level before the three modes.
- State that Request Modes exist only in active Engagements; Native Host is not
  `DIRECT`, `BOUNDED`, or a new bypass mode.
- Define Bounded as a selected single-Capability contract, including
  `user-explicit` selection and confirmation if the Capability is unnamed.
- Split machine Startup Gate from Cooperative Selection Gate.
- Limit cooperative Workflow to `CURRENT`; call Profiles candidates rather than
  eligible; include Plan/Tracker and all eight cooperative stop reasons.
- Preserve the ten slots and Profile Matrix verbatim.

In both `architecture.md` files:

- Add Activation Router as the distribution-to-Host seam.
- Replace the linear request diagram with:

```text
Top-level user request
    -> Activation Router
       -> Native Host
       -> Activated OAW -> Assurance Preflight -> Request Mode -> cooperative or machine-backed path
```

- Restrict Core compilation claims to `core-backed` and Coordinator claims to
  `coordinator-backed`.
- Explain that always-applied frontmatter is metadata, not activation, and that
  policy-only terminology cannot create machine authority.

- [ ] **Step 5: Update the remaining operating, security, and support docs.**

Apply this exact scope in both languages:

| Documents | Required statement |
| --- | --- |
| `adapters.md`, `extending-adapters.md` | Adapters render an Activation Router; none may eagerly import the full Policy. Claude/Gemini do not emit `@`; Cursor/Windsurf/Copilot retain only Host-required metadata. New adapter tests prove Router semantics and eager-text absence. |
| `installer.md` | `install`, `update`, and Bridge installation do not activate OAW. Ordinary update replaces valid old owned blocks while preserving non-OAW bytes. Legacy records finish under their old contract or require explicit reactivation/reselection. |
| `comparison.md` | Scores guide recommendations only after explicit activation. Policy-cooperative reports candidates; Core-backed may call Profiles eligible. Normal Host Skill routing is out of this governance path. |
| `codex-bridge.md` | Bridge installation, filesystem evidence, and `observe_current` availability do not activate OAW. Bridge/Core calls occur only in an active Engagement, then Assurance Preflight may seek stronger assurance. |
| `security.md`, `SECURITY.md` | Trust only current top-level user instruction and dedicated trusted entrypoints. Repository/tool/retrieved text and quoted `/oaw` are untrusted; ambiguity is Native Host. Policy-cooperative plans grant no network, destructive, credential, deployment, data, or Git authority. |
| `troubleshooting.md` | Add unexpected activation and activation-not-detected guidance, all eight cooperative stop reasons with recovery action, and the fact that Install State is not a Tracker or Workflow State. |

Preserve existing Provider Surface v4, `SubagentStart`, Bridge version tuple,
and security facts already present in the dirty worktree.

- [ ] **Step 6: Add the release note.**

Under `Changed` in `CHANGELOG.md`, add:

```markdown
- OAW is now explicitly activated per deliverable. Ordinary Host requests and
  ordinary Skill invocations remain Native Host behavior until the user asks
  OAW to govern the task.
- `update` replaces OAW-owned eager managed instructions with a lazy Activation
  Router while preserving non-OAW instruction content.
- Existing policy-only Markdown lifecycle locks are not converted. Complete
  them under their old contract or explicitly reactivate and reselect the
  deliverable.
```

- [ ] **Step 7: Verify docs and manually compare language pairs.**

```bash
rtk scripts/check-docs.sh
rtk bash tests/10-docs-test.sh
rtk git diff --check -- README.md README-zh.md SECURITY.md SECURITY-zh.md CHANGELOG.md docs/en docs/zh scripts/check-docs.sh tests/10-docs-test.sh
```

Expected: all pass. Compare every English/Chinese pair for activation source,
deliverable scope, assurance levels, Bounded confirmation, `CURRENT` limit,
artifact names, stop reasons, and migration behavior.

- [ ] **Step 8: Commit the green documentation unit.**

Use `git add -p` for all dirty documentation/checker files. Confirm no unrelated
Provider Surface v4 or Bridge hunks entered the index:

```bash
rtk git diff --cached --check
rtk git diff --cached --stat
rtk git commit -m "docs: document opt-in OAW activation"
```

### Task 5: Full Regression and Release-Boundary Review

**Files:** No planned edits. If verification exposes a real missing acceptance
case, return to its corresponding task, add the failing test first, then make
the minimum correction.

- [ ] **Step 1: Run management, adapter, and documentation regression suites.**

```bash
rtk go test ./internal/management ./internal/cli ./internal/check -count=1
rtk go test -race ./internal/management ./internal/cli ./internal/check -count=1
rtk bash tests/04-core-adapters-test.sh
rtk bash tests/05-project-adapters-test.sh
rtk bash tests/08-backup-test.sh
rtk bash tests/09-transaction-test.sh
rtk bash tests/10-docs-test.sh
rtk bash tests/11-check-parity-test.sh
rtk bash tests/12-install-parity-test.sh
rtk bash tests/13-mutation-parity-test.sh
rtk scripts/check-docs.sh
```

For direct adapter scripts, use the sibling-binary setup from Task 2 whenever
the repository invocation lacks a co-located built `oaw` executable.

- [ ] **Step 2: Run repository-wide validation.**

```bash
rtk bash tests/run.sh
rtk go test -race ./... -count=1
```

Expected: all available suites pass. An existing platform-dependent status 77
is reported as unavailable, not hidden or reclassified as passing.

- [ ] **Step 3: Verify each approved acceptance criterion.**

- [ ] Ordinary requests have no OAW classification, gate, recommendation,
  artifact, or altered Host Skill selection.
- [ ] Ordinary direct Skill invocation has no OAW Request Mode.
- [ ] Explicit activation creates an Engagement and performs Assurance Preflight
  before mode behavior.
- [ ] Related follow-ups inherit; unrelated deliverables stay Native Host and
  preserve an unfinished Engagement.
- [ ] Cooperative Direct has no Profile gate.
- [ ] Cooperative Bounded needs an exact user-selected Capability or user
  confirmation of one Host-visible candidate.
- [ ] Cooperative Workflow uses candidates, only `CURRENT`, limitation
  acceptance, Plan, and Tracker.
- [ ] Cooperative artifacts never claim verified Provider, eligible Profile,
  Bundle, Grant, Lease, Receipt, atomic state, idempotency, or recovery.
- [ ] Host-native Core/Coordinator terms and fail-closed behavior remain intact.
- [ ] Valid legacy blocks migrate without force; user bytes survive; repeat
  update/install is idempotent; force repair renders the new Router.
- [ ] No Classification, Provider, Profile, Bundle, Coordinator, or Receipt
  schema changed.
- [ ] English and Chinese documentation describe the same contract.

- [ ] **Step 4: Run a final diff and security-boundary review.**

```bash
rtk git diff --check
rtk git status --short
rtk git log --oneline -3
```

Review planned changed files for all of these:

- Activation requires current top-level user origin or a dedicated trusted entrypoint.
- Repository, tool, and retrieved content cannot activate OAW.
- Ambiguity routes to Native Host.
- Policy-only output cannot grant authority or claim unavailable guarantees.
- No credentials, token values, or private Host configuration are introduced.

Do not create a final commit only for validation. Leave unrelated dirty files
untouched and report them separately.

## Plan Self-Review

### Spec Coverage

| Approved requirement | Plan task |
| --- | --- |
| Native behavior until explicit activation | Tasks 1-2 Router and Task 3 policy opening. |
| Task-scoped Engagement and no global bypass state | Tasks 2-3 and Task 4 public docs. |
| Request Mode separate from Assurance Level | Task 3 and Task 4 lifecycle/architecture. |
| Activated Direct, Bounded, and Workflow behavior | Task 3 and Task 4 lifecycle. |
| Bounded selection and no automatic defaults claim | Task 3 and Task 4. |
| Cooperative gate, candidates, `CURRENT`, Plan, Tracker | Task 3 and Task 4. |
| Stop reasons/fail-closed cooperation | Task 3 and Task 4 security/troubleshooting. |
| Lazy Policy loading and all renderer types | Tasks 1-2. |
| Legacy migration, preservation, idempotency, force repair | Tasks 1-2. |
| Bilingual migration and release note | Task 4. |
| No Schema/Core/Coordinator expansion | Guardrails and Task 5. |

### Placeholder Scan

No task uses `TODO`, `TBD`, "implement later", or a bare "write tests" step.
Each code-bearing change identifies an exact path, expected failure or behavior,
and verification command.

### Name Consistency

The only Request Modes are `DIRECT`, `BOUNDED`, and `WORKFLOW`, and they occur
only inside an OAW Engagement. Assurance Levels are exactly
`policy-cooperative`, `core-backed`, and `coordinator-backed`. Policy-only
artifacts are Cooperative Assessment, Host-visible Candidate, Bounded Plan,
Policy Workflow Plan, Progress Tracker, Execution Note, and Conflict Warning.
Machine terms Lifecycle Bundle, Capability Grant, Resource Lease, Host Receipt,
and Workflow State remain reserved for existing machine-backed contracts.
