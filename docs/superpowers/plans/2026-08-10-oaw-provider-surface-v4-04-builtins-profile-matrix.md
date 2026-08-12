# OAW Provider Surface v4 04: Built-ins and Profile Matrix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the invalid built-in Provider and Recipe records with source-pinned v4/v3 assets for `MATT-FULL`, `SP-FULL`, `ECC-FULL`, and the immutable `MATT-SP-HYBRID/default` template, then prove their host-specific matrix and compiler behavior through integration tests.

**Architecture:** A read-only source audit pins each Distribution revision and every Binding tree before the assets are authored. Host-qualified Descriptor Bindings preserve the exact upstream surface (`skill`, `agent`, `role`, or `instruction`); Recipe v3 records express ordered N:M pipelines, macro spans, neutral Host actions, gates, alternatives, incident routes, and explicit Hybrid overlays. Built-in loading validates the complete asset set, while a deterministic projection records the declared matrix without treating installation or a recommendation as eligibility.

**Tech Stack:** Go 1.26, strict JSON and canonical SHA-256, `internal/integrity` tree evidence, JSON assets, table-driven tests, `fstest.MapFS`, shell drift checks, and race-enabled integration tests.

---

**Selected lifecycle:** `SP-FULL / CURRENT / no Add-on`.

**Depends on:**

- Plan 01's `catalog` Descriptor v4, Recipe v3, ten-slot taxonomy, strict decoders, and validation errors.
- Plan 02's complete-tree integrity, Distribution-scoped discovery, Host Manifest/Session/Binding Inventory v3, Host action and delegation observations, and Registry v4 (`Registry.Bindings`, `Registry.Capability`, and `Registry.Digest`).
- Plan 03's `profile` compiler and Builder API, especially `CompileProfile`, `CompileRecipe`, `NewHostEvidence`, `ValidateExecutionGraphRecord`, `BuilderProjection`, and `ConfirmedRecipe`.
- The approved v4 design and the read-only audit evidence in `.scratch/oaw-provider-profile-audit/upstream-skill-audit.md` and `.scratch/oaw-provider-profile-audit/canonical-profile-stage-matrix.md`.

**Produces:** Three pinned Provider descriptors covering exactly three Providers and four Distributions, four active Recipe assets, four aliases, the source audit manifest, the generated Profile Matrix projection, built-in loader validation, and the complete built-in/profile integration test surface. This plan never installs a Provider, changes a Host configuration, or selects a lifecycle.

## Ownership and Cutover Rules

Plan 04 owns every file that describes or loads a built-in Provider/Profile and every cross-package test whose subject is a built-in Profile. In particular, it owns `internal/integration/profile_compiler_test.go`, `internal/builtin/load_test.go`, all four Provider/Recipe JSON families, the source audit and matrix projections, and `internal/assets/embed.go` entries needed by those projections.

The other plans retain these boundaries:

| Boundary | Owner | Plan 04 interaction |
| --- | --- | --- |
| Authority schemas and catalog records | Plan 01 | Consume only the exported v4/v3 records and constants. Do not add a second decoder or a compatibility reader. |
| Integrity, Host v3 evidence, Host actions, and Registry v4 | Plan 02 | Build test fixtures through the exported constructors; do not duplicate production validation. |
| Graph v4 compiler and USER-DEFINED Builder | Plan 03 | Call the locked public API; do not re-derive ownership, cursor order, or diagnostics in `builtin`. Execution found and corrected one Plan 03 implementation deviation: a credited internal unit may be the exact owner designated through its enclosing Recipe step, as already required by the locked macro rules. |
| Core, Admission, Host Receipt, Coordinator | Plan 05 | Consume the assets and graph; do not edit built-in fixtures or `profile_compiler_test.go`. |
| Codex Bridge, generated Host Integration, public docs, and START | Plan 06 | May update only Host fixture/conformance assertions in `internal/builtin/load_test.go` while preserving this plan's Profile coverage; must not modify `internal/builtin/matrix_test.go` or `internal/integration/profile_compiler_test.go`. |

Descriptor v4 and Recipe v3 authority schemas remain Plan 01 assets; Host schemas remain Plan 02 assets; Graph v4 remains a Plan 03 asset. `oaw.provider-source-audit/v1` and `oaw.profile-matrix/v1` are non-authoritative evidence/projection records with closed Go decoders and stored-digest validation. Plan 04 does not register either record as dispatch authority or create a second authority schema path.

The Descriptor/Recipe/load change is one hard cutover. Plan 01 removes the old schema constants and authority records, so a temporary state containing v4 assets with a v2 loader cannot compile. Tasks 2 and 3 below therefore have explicit package gates and never claim an intermediate GREEN state that cannot build.

There is no compatibility conversion for a v3 Provider descriptor, v2 Recipe, old graph, or old Bundle. The old `oaw/hardening` Recipe is removed from the active asset directory; its security concern remains a bounded specialist check only when a user explicitly selects a verified add-on.

## Locked Upstream Evidence

The source audit uses these immutable Distribution revisions and no branch or
floating tag:

| Provider ID | Distribution ID | Repository | Revision | Distribution root | Host use |
| --- | --- | --- | --- | --- | --- |
| `oaw/matt` | `matt-skills` | `https://github.com/mattpocock/skills` | `84fdeffd12f2ee307994d1eb6feb48173b6e0502` | `.` | Matt Host surfaces |
| `oaw/superpowers` | `superpowers` | `https://github.com/obra/superpowers` | `44c9b2d6e889982ac18c27d05a19fefe335194e1` | `.` | Claude and direct-upstream Codex installations |
| `oaw/superpowers` | `superpowers-codex` | `https://github.com/openai/plugins` | `11c74d6ba24d3a6d48f54a194cd00ef3beea18f9` | `plugins/superpowers` | OpenAI-packaged Codex cache installations |
| `oaw/ecc` | `ecc` | `https://github.com/affaan-m/ECC` | `2d46e80e0925c7be0907f18c1812311ac212a6c5` | `.` | ECC Host surfaces |

The audit manifest records a concrete `sha256:` tree digest for every listed Binding root and a complete Distribution tree digest. The canonical audited matrix digest remains `49ec1819ab22364d763d0875d9af299ee332de3d6d39a7178a715c2b13272ccf`; this value is evidence input, not a recommendation.

The earlier one-Distribution-per-Provider assertion is invalid and superseded.
Use `matt-skills` for `oaw/matt`, both `superpowers` and
`superpowers-codex` for the single `oaw/superpowers` Provider, and `ecc` for
`oaw/ecc`. This is exactly three Providers and four content-distinct
Distributions. `MATT-SP-HYBRID` remains a Recipe and never becomes a Provider.

Discovery channels select the Distribution whose immutable content they
actually install. Claude and direct-upstream Codex Superpowers channels point
to `superpowers`; the OpenAI-packaged Codex cache points to
`superpowers-codex`. A channel name does not create another Provider or
Profile. The `openai-api-curated` path component and the legacy
`sp-codex-curated-cache` probe ID identify an installation channel only;
`curated` is not a product identity.

### Locked discovery probes

Discovery remains diagnostic until Plan 02 intersects a complete Distribution/Binding tree and live Host observation. Use these exact built-in probes; no probe may treat its shared ancestor as provenance:

| Probe ID | Distribution | Host / logical surface | Kind | Candidate or prefix | Evidence |
| --- | --- | --- | --- | --- | --- |
| `matt-codex-skill-lock` | `matt-skills` | `codex` / `codex-user-skills` | `path-exists` | candidate `.agents` | `.skill-lock.json`; each selected skill must also match its exact manifest source/path and complete tree |
| `matt-claude-official-cache` | `matt-skills` | `claude` / `claude-plugin` | `one-level-version-path-exists` | prefix `.claude/plugins/cache/claude-plugins-official/mattpocock-skills` | `.claude-plugin/plugin.json` |
| `sp-claude-direct` | `superpowers` | `claude` / `claude-plugin` | `path-exists` | candidate `.claude/plugins/superpowers` | `skills/using-superpowers/SKILL.md` |
| `sp-codex-direct` | `superpowers` | `codex` / `codex-plugin` | `path-exists` | candidate `.codex/plugins/superpowers` | `skills/using-superpowers/SKILL.md` |
| `sp-claude-marketplace` | `superpowers` | `claude` / `claude-plugin` | `path-exists` | candidate `.claude/plugins/marketplaces/superpowers-marketplace` | `skills/using-superpowers/SKILL.md` |
| `sp-claude-official-cache` | `superpowers` | `claude` / `claude-plugin` | `one-level-version-path-exists` | prefix `.claude/plugins/cache/claude-plugins-official/superpowers` | `skills/using-superpowers/SKILL.md` |
| `sp-claude-marketplace-cache` | `superpowers` | `claude` / `claude-plugin` | `one-level-version-path-exists` | prefix `.claude/plugins/cache/superpowers-marketplace/superpowers` | `skills/using-superpowers/SKILL.md` |
| `sp-codex-curated-cache` | `superpowers-codex` | `codex` / `codex-plugin` | `one-level-version-path-exists` | prefix `.codex/plugins/cache/openai-api-curated/superpowers` | `skills/using-superpowers/SKILL.md` |
| `ecc-claude-marketplace` | `ecc` | `claude` / `claude-plugin` | `path-exists` | candidate `.claude/plugins/marketplaces/everything-claude-code/plugins/ecc` | `.codex-plugin/plugin.json` |
| `ecc-claude-cache` | `ecc` | `claude` / `claude-plugin` | `one-level-version-path-exists` | prefix `.claude/plugins/cache/everything-claude-code/ecc` | `.codex-plugin/plugin.json` |
| `ecc-codex-direct` | `ecc` | `codex` / `codex-plugin` | `path-exists` | candidate `.codex/plugins/ecc` | `.codex-plugin/plugin.json` |
| `ecc-codex-cache` | `ecc` | `codex` / `codex-plugin` | `one-level-version-path-exists` | prefix `.codex/plugins/cache/everything-claude-code/ecc` | `.codex-plugin/plugin.json` |

The Matt lock probe never verifies an unrelated same-name directory under `.agents/skills`; the lock entry must name `mattpocock/skills`, the pinned source, and the exact `skillPath`, followed by a complete content-equivalence check. Matt's pinned README and `.claude-plugin/plugin.json` establish the official Claude plugin distribution and its managed cache channel. Multiple probes for one Host share a logical surface (`claude-plugin` or `codex-plugin`); the probe ID/path records the installation channel, while the surface remains stable so one Host-qualified Binding can verify through any audited channel. ECC's old aggregate `~/.agents/skills/everything-claude-code/SKILL.md` probe is removed because it cannot prove the repository-root skill, Agent, Role, or instruction trees. A Host installation outside these exact channels is supplied through trusted user/project configuration and follows the same evidence rules.

### Exact audited Binding sets

Binding IDs are local, Host-and-Distribution-qualified IDs so distinct Host
surfaces and distinct Distributions cannot collide. The `reference` column is
the exact upstream invocation name; the ID prefix is OAW metadata and is never
presented as an upstream skill name. For every row, the Descriptor also records
the source revision, Distribution ID, independent relative `content_root` and
`install_root`, tree digest, artifact schemas, effects, resources, topology
support, and delegation requirements.

The path mapping is exact and source-pinned. Matt's `ContentRoot` is the full
upstream path listed below while its flattened Host `InstallRoot` is the skill
directory name, for example `skills/engineering/to-spec -> to-spec`.
Both Superpowers Distributions and ECC use repository-style Distribution
roots, so each Binding's `InstallRoot` equals its `ContentRoot` relative to the
selected Distribution root. A trusted user-defined descriptor may declare
another explicit mapping, but no built-in or resolver derives a path from a
basename, reference, Provider brand, or directory ancestry.

Matt source roots are `skills/engineering/grill-with-docs`, `skills/productivity/grilling`, `skills/engineering/domain-modeling`, `skills/engineering/to-spec`, `skills/engineering/to-tickets`, `skills/engineering/implement`, `skills/engineering/tdd`, `skills/engineering/diagnosing-bugs`, and `skills/engineering/code-review`.

| Host-qualified ID pattern | Host surface | Exact reference | Kind | Invocation | Span / macro contract |
| --- | --- | --- | --- | --- | --- |
| `codex-grill-with-docs`, `claude-grill-with-docs` | matching Host skill surface | `grill-with-docs` | `skill` | `human-explicit` | slot 1; credit-only `grilling` and `domain-modeling` |
| `codex-grilling`, `claude-grilling` | matching Host skill surface | `grilling` | `skill` | `model` | slot 1 internal procedure |
| `codex-domain-modeling`, `claude-domain-modeling` | matching Host skill surface | `domain-modeling` | `skill` | `model` | slot 1 internal procedure |
| `codex-to-spec`, `claude-to-spec` | matching Host skill surface | `to-spec` | `skill` | `human-explicit` | slot 2; synthesis only, never requirements elicitation |
| `codex-to-tickets`, `claude-to-tickets` | matching Host skill surface | `to-tickets` | `skill` | `human-explicit` | slot 3; ticket decomposition and acceptance edges |
| `codex-implement`, `claude-implement` | matching Host skill surface | `implement` | `skill` | `human-explicit` | slots 5-8 macro envelope: implementation owner in slot 5, credit-only `tdd` owner in slot 6, no slot 7 claim, and credit-only internal `code-review` owner in slot 8; no completion claim |
| `codex-tdd`, `claude-tdd` | matching Host skill surface | `tdd` | `skill` | `model` | slot 6 procedure; credit-only inside `implement` or standalone in Hybrid |
| `codex-diagnosing-bugs`, `claude-diagnosing-bugs` | matching Host skill surface | `diagnosing-bugs` | `skill` | `model` | conditional slot 7 only for functional, hard-bug, or performance incidents |
| `codex-code-review`, `claude-code-review` | matching Host skill surface | `code-review` | `skill` | `model` | slot 8; child plus parallel-child, or nested equivalents under outer `SUBAGENT` |

Matt has no `requirements`, `verification-loop`, workspace, completion, or generic build/dependency/type-repair Binding. Tests must search both IDs and exact references so a same-named skill from another Provider cannot be attributed to Matt.

Superpowers source roots are `skills/brainstorming`, `skills/writing-plans`,
`skills/using-git-worktrees`, `skills/subagent-driven-development`,
`skills/executing-plans`, `skills/test-driven-development`,
`skills/systematic-debugging`, `skills/requesting-code-review`,
`skills/receiving-code-review`, `skills/verification-before-completion`, and
`skills/finishing-a-development-branch`. Each root is audited independently in
both `superpowers` and `superpowers-codex`; matching relative paths or
references do not imply matching content.

Each audited reference produces exactly three Bindings under the unchanged
`oaw/superpowers` Provider:

- existing `codex-<stem>` uses `superpowers-codex` for the OpenAI-packaged
  Codex cache and lists `codex-upstream-<stem>`, then `claude-<stem>`, as its
  complete alternatives;
- new `codex-upstream-<stem>` uses `superpowers` for direct-upstream Codex and
  lists `codex-<stem>`, then `claude-<stem>`, as its complete alternatives; and
- `claude-<stem>` remains on `superpowers` and lists `codex-<stem>`, then
  `codex-upstream-<stem>`, as its complete alternatives.

| Packaged Codex / upstream Codex / Claude Binding IDs | Exact reference | Kind / invocation | Span / internal contract |
| --- | --- | --- | --- |
| `codex-brainstorming`, `codex-upstream-brainstorming`, `claude-brainstorming` | `superpowers:brainstorming` | `skill` / `model` | slots 1-3 macro envelope; parent owns slots 1-2 and dispatch-after `writing-plans` owns slot 3 after design approval |
| `codex-writing-plans`, `codex-upstream-writing-plans`, `claude-writing-plans` | `superpowers:writing-plans` | `skill` / `model` | slot 3 |
| `codex-using-git-worktrees`, `codex-upstream-using-git-worktrees`, `claude-using-git-worktrees` | `superpowers:using-git-worktrees` | `skill` / `model` | slot 4; one workspace result |
| `codex-subagent-driven-development`, `codex-upstream-subagent-driven-development`, `claude-subagent-driven-development` | `superpowers:subagent-driven-development` | `skill` / `model` | slots 4-10 macro envelope; dispatch-before workspace and dispatch-after finish are its only cross-skill calls; the parent owns implementation plus its embedded per-task/final two-stage review responsibility |
| `codex-executing-plans`, `codex-upstream-executing-plans`, `claude-executing-plans` | `superpowers:executing-plans` | `skill` / `model` | slots 4-10 inline macro envelope; dispatch-before workspace and dispatch-after finish |
| `codex-test-driven-development`, `codex-upstream-test-driven-development`, `claude-test-driven-development` | `superpowers:test-driven-development` | `skill` / `model` | standalone slot 6 procedure on both SDD and inline paths; SDD does not call this skill |
| `codex-systematic-debugging`, `codex-upstream-systematic-debugging`, `claude-systematic-debugging` | `superpowers:systematic-debugging` | `skill` / `model` | typed technical incident handler in slot 7 |
| `codex-requesting-code-review`, `codex-upstream-requesting-code-review`, `claude-requesting-code-review` | `superpowers:requesting-code-review` | `skill` / `model` | standalone slot 8 review dispatch on the inline path; reviewer child required; SDD instead owns its documented embedded two-stage review |
| `codex-receiving-code-review`, `codex-upstream-receiving-code-review`, `claude-receiving-code-review` | `superpowers:receiving-code-review` | `skill` / `model` | slot 8 remediation procedure; one finding at a time and re-review |
| `codex-verification-before-completion`, `codex-upstream-verification-before-completion`, `claude-verification-before-completion` | `superpowers:verification-before-completion` | `skill` / `model` | standalone slot 9 fresh proof on both execution paths; SDD does not call this skill |
| `codex-finishing-a-development-branch`, `codex-upstream-finishing-a-development-branch`, `claude-finishing-a-development-branch` | `superpowers:finishing-a-development-branch` | `skill` / `model` | slot 10; user-authority choice |

Every Superpowers `CapabilityRecord.binding_refs` contains all three matching
Binding IDs. A Capability never references only the packaged or only the
upstream pair, and alternative resolution never substitutes one
Distribution's digest for another.

All Superpowers references include the exact `superpowers:` namespace. SDD requires child delegation under `CURRENT` and nested-child delegation under outer `SUBAGENT`; its implementer, spec-review, and quality-review dispatches are embedded responsibilities described by SDD's own prompt assets, not invocations of `requesting-code-review`. Standalone `requesting-code-review` likewise requires a reviewer child or nested child according to the outer topology. Inline execution removes only SDD's implementation-child requirement and keeps the standalone reviewer-child requirement. Matt `code-review`, independently, retains its audited parallel review requirement. No lexical alternative is selected.

`finishing-a-development-branch` includes `network-write` in its maximum
effects because one user-selected closeout option can push/open a PR; a Recipe
user gate and Host approval remain mandatory before that option is exercised.

ECC source roots and surfaces are intentionally separated:

| Host/surface | Exact references and roots | Kind |
| --- | --- | --- |
| Cross-Host skill | `skills/intent-driven-development`, `skills/product-capability`, `skills/contract-first`, `skills/blueprint`, `skills/git-workflow`, `skills/tdd-workflow`, `skills/verification-loop`, `skills/security-review`, `skills/e2e-testing` | `skill` |
| Claude custom Agent | `agents/architect.md`, `agents/planner.md`, `agents/tdd-guide.md`, `agents/build-error-resolver.md`, `agents/code-reviewer.md`, `agents/security-reviewer.md`, `agents/e2e-runner.md` | `agent` |
| Codex Role | `.codex/agents/explorer.toml`, `.codex/agents/reviewer.toml`, `.codex/agents/docs-researcher.toml` | `role` |
| Legacy instruction | `commands/plan.md`, `commands/feature-dev.md` only when the active Host attests that exact instruction surface | `instruction` |
| Hook evidence | `hooks/` and the exact observed hook identity, including `delivery-gate` when present | evidence only; never a Binding |

There is no wildcard `orch-*` Binding. An ECC orchestration macro may be added in a later version only after an exact name, root, content digest, and all internal dependencies are audited. `e2e-runner` is E2E specialist evidence, not broad verification; `code-reviewer`/`reviewer` is review, not completion; `delivery-gate` is a Hook, not a delivery owner.

## Canonical Ten-Slot Matrix Contract

The JSON Recipes use the taxonomy IDs exactly in this order:

| # | Slot ID | Required outcome | Control records |
| --- | --- | --- | --- |
| 1 | `problem-framing` | aligned purpose, constraints, domain terms, decisions, success conditions | Provider outcome plus user-alignment gate |
| 2 | `solution-specification` | reviewable specification and test boundaries | Provider outcome plus user approval gate |
| 3 | `delivery-planning` | independently verifiable delivery units and executable plan | Provider outcome plus ticket approval gate |
| 4 | `workspace-preparation` | safe workspace and known baseline | Host action `workspace.prepare-or-confirm` plus readiness gate |
| 5 | `implementation` | approved bounded changes | Provider outcome |
| 6 | `implementation-tdd` | witnessed expected RED/GREEN cycle | procedure or credited macro call |
| 7 | `incident-recovery` | typed recovery, replan, or explicit stop | conditional incident routes; no unconditional fake stage |
| 8 | `review-remediation` | findings fixed/adjudicated and re-reviewed | assurance pipeline |
| 9 | `fresh-verification` | fresh claim-relevant command evidence | Provider/Host executor plus fresh-evidence gate |
| 10 | `closeout` | accepted and user-authorized delivery/preservation result | Provider/Host executor plus user gate |

Each Recipe has ten `slots` in canonical order, even when a slot's pipeline is empty because a macro unit spans it. A mandatory slot's `OutcomeOwner` points to the one expanded unit that produces its outcome; credited internal calls never create a second owner. `HostAction` records have no Provider selector. `GateRecord` records have no Provider selector. Conditional slot 7 may use `outcome_owner.kind = none` when all applicable incident routes explicitly stop or replan.

### MATT-FULL / `oaw/domain-engineering`

The Recipe is Matt-led and remains a selectable alias. Its exact sequence is:

```text
codex-or-claude grill-with-docs (calls grilling + domain-modeling)
  -> user shared-understanding gate
  -> to-spec (separate human-explicit invocation)
  -> spec-approval gate
  -> to-tickets (separate human-explicit invocation)
  -> ticket-approval gate
  -> workspace.prepare-or-confirm Host action
  -> implement(ticket) spanning the slots 5-8 macro envelope
       -> credit-only tdd
       -> credit-only internal code-review with required review children
       -> code-review owns slot 8 through the enclosing implement step
       -> findings transition to an explicit implement remediation packet
       -> the remediation implement run performs the fresh internal re-review
  -> verification.execute Host action + fresh-evidence gate
  -> closeout.execute Host action + acceptance/user-authority gate
```

The implementation descriptor never claims workspace creation, general delegation, broad verification, or completion. Its contiguous slots 5-8 span is a macro envelope, not an incident-recovery claim. Build, dependency, and type incidents have no Matt handler and stop by default. The descriptor's internal `code-review` is the slot 8 review procedure only; remediation is a distinct `implement` invocation, and that invocation performs the next internal review. Every `human-explicit` Binding pauses until an exact Host/user invocation attestation is supplied at PREPARE by Plan 05. Matt tracker-publishing Bindings declare `network-write` in their maximum effects, while the active invocation still requires user/Host authority.

### SP-FULL / `oaw/delivery`

The built-in Recipe uses the complete inline Superpowers path so it remains truthful under `CURRENT` without child delegation. The audited SDD Binding remains available to `USER-DEFINED` Recipes, but is not a single-step alternative inside `oaw/delivery`: SDD owns an embedded review/remediation responsibility, while the inline path uses a separate review pipeline, and Recipe v3 cannot atomically replace both the executor and that different owner/pipeline shape through one `AlternativeChoice`.

| Recipe shape | Main implementation macro | Cross-skill calls and resulting slot owners |
| --- | --- | --- |
| built-in `SP-FULL` | `executing-plans` | dispatch-before `using-git-worktrees`; standalone `test-driven-development`, `requesting-code-review -> receiving-code-review`, and `verification-before-completion`; dispatch-after `finishing-a-development-branch` |
| versioned `USER-DEFINED` SDD clone | `subagent-driven-development` | dispatch-before `using-git-worktrees`; SDD owns implementation and its embedded per-task/final review/remediation responsibility; standalone `test-driven-development` owns slot 6; standalone `verification-before-completion` owns slot 9; dispatch-after `finishing-a-development-branch` owns slot 10 |

Slots 1-2 are one `brainstorming` run with user design approval; its `writing-plans` continuation is dispatch-after and completes slot 3 exactly once under the enclosing brainstorming macro. Slot 7 routes typed technical failures to `systematic-debugging`. A user-defined SDD Recipe is eligible under `CURRENT` only with live child-delegation evidence and under outer `SUBAGENT` only with nested-child-delegation evidence. Its documented prompt-driven reviews remain inside the SDD responsibility and do not create a `requesting-code-review`, `test-driven-development`, or `verification-before-completion` InternalCall. The built-in inline path needs no implementation child but still needs the standalone reviewer child. Missing required delegation returns `HOST_FEATURE_UNATTESTED` or `PROFILE_TOPOLOGY_UNAVAILABLE`; it never becomes self-review or sequential simulation.

### ECC-FULL / `oaw/ecc-engineering`

The Recipe has Host-surface alternatives, not name substitution:

| Slot | Codex path when exactly observed | Claude path when exactly observed | Explicit gap rule |
| --- | --- | --- | --- |
| 1 | skill `intent-driven-development` | Agent `architect` | Claude Agent name cannot satisfy a Codex Role; missing both fails |
| 2 | skill `product-capability`, optional skill `contract-first` for shared-contract work | same exact skills | `contract-first` is conditional and cannot become a second owner |
| 3 | skill `blueprint` only when its large multi-session trigger is attested, or instruction `/plan` when that exact surface is observed | Agent `planner`, or skill `blueprint` when its trigger is true | no invented Codex planner/architect role; an unmet trigger is an exact diagnostic |
| 4 | Host `workspace.prepare-or-confirm` with skill `git-workflow` as guidance | same Host action with skill `git-workflow` | `git-workflow` does not itself prove a clean workspace |
| 5-6 | skill `tdd-workflow` spanning implementation and TDD | Agent `tdd-guide` or skill `tdd-workflow` | one outcome owner; no duplicate TDD peer |
| 7 | only explicitly typed regression route supplied by `tdd-workflow`; build/type/dependency route stops without a verified specialist | Agent `build-error-resolver` for build/type/dependency; functional route only when an exact supporting binding is present | `e2e-runner` is not a generic incident or verification handler |
| 8 | Role `reviewer` plus skill `tdd-workflow` remediation | Agent `code-reviewer` plus separately bound remediation procedure | reviewer is read/report-only; no completion claim |
| 9 | skill `verification-loop` | skill `verification-loop` | E2E is an optional specialist check, not this owner |
| 10 | Host `closeout.execute` with skill `git-workflow` guidance | same Host action and user gate | neither `code-reviewer` nor `delivery-gate` owns closeout |

The Recipe remains active as an alias but current Codex v1 evidence must fail closed because it reports only `skill`, `CURRENT`, and no live action/delegation/role facts. A future Host can compile the alias only after every selected alternative is verified.

### MATT-SP-HYBRID/default / `oaw/reliable-feature`

This is the preserved core composition, not a compatibility alias for an old graph. Its immutable `family` is `matt-superpowers`, its `template` is `default`, and its exact default is:

| Slot | Default owner/pipeline | Paused or credited surfaces |
| --- | --- | --- |
| 1 | Matt `grill-with-docs` | credit-only Matt `grilling` + `domain-modeling` |
| 2 | Matt `to-spec` | user approval gate |
| 3 | Matt `to-tickets` -> Superpowers `writing-plans` | Matt owns ticket edges; SP adds file/command detail |
| 4 | Superpowers `using-git-worktrees` | one workspace result |
| 5 | Superpowers `executing-plans` | SDD paused; no implementation child under the selected grant |
| 6 | Matt `tdd` | SP `test-driven-development` paused; no duplicate TDD owner |
| 7 | Matt `diagnosing-bugs` for functional/hard-bug/performance | build/dependency/type defaults to stop; ECC handler is an explicit add-on only |
| 8 | SP `requesting-code-review` -> `receiving-code-review` -> fresh re-review | Matt general review and SDD internal review paused; reviewer child remains required |
| 9 | SP `verification-before-completion` | fresh-evidence gate |
| 10 | SP `finishing-a-development-branch` | explicit user-authority gate |

The Recipe has no selected ECC Add-on. Its declared optional add-ons, when a user later creates a new versioned Recipe, are `ecc-build-repair` (ECC `build-error-resolver` on a Host that attests it) and `ecc-security-review` (ECC `security-review` or the exact verified Claude `security-reviewer` surface). Add-ons are incident/specialist checks only and cannot take implementation, TDD, review, verification, or closeout ownership. Selecting SDD creates a new versioned `USER-DEFINED` Recipe and requires live child or nested-child delegation; Matt remains the single TDD owner and Superpowers TDD remains paused. Changing any default choice never mutates `oaw/reliable-feature`.

## Task 1: Pin and verify the upstream source manifest

**Files:**

- Create: `internal/provideraudit/records.go`
- Create: `internal/provideraudit/records_test.go`
- Create: `internal/provideraudit/generate.go`
- Create: `internal/provideraudit/generate_test.go`
- Create: `cmd/oaw-provider-audit/main.go`
- Create: `cmd/oaw-provider-audit/main_test.go`
- Create: `scripts/audit-provider-sources.sh`
- Create: `internal/assets/audits/provider-sources-v4.json`
- Create: `tests/19-provider-source-audit-test.sh`

### RED

- [ ] **Step 1: Write strict manifest and tree-audit tests first.**

Add table-driven tests named `TestDecodeManifestRejectsUnknownFields`, `TestDecodeManifestRejectsRetiredVersion`, `TestManifestRequiresExactProviderPins`, `TestManifestRequiresUniqueBindingRoots`, `TestManifestRequiresExactInstallRootMappings`, `TestManifestRequiresPrefixedTreeDigests`, `TestManifestRequiresCanonicalMatrixDigest`, `TestBuildManifestUsesTrackedBindingRoots`, `TestBuildManifestRejectsRevisionDrift`, `TestBuildManifestRejectsMissingRoot`, and `TestAuditCLIUsesExplicitRoots`. The fixture must contain all four Distribution source/revision/root tuples and every root listed above while retaining exactly three Provider IDs; it must reject a missing `grill-with-docs`, a duplicate Binding ID or duplicate mapping for one Host-qualified Binding, an absent or inferred install root, a path containing `..`, a 64-character bare digest, and a branch name. The same upstream `ContentRoot` may legitimately occur on distinct Host-qualified rows when their exact Host or Distribution identities differ.

Add `tests/19-provider-source-audit-test.sh` as an offline test. It invokes the CLI's `--validate` mode against the committed JSON, checks the schema ID, all four exact Distribution pins, the canonical matrix digest, every Binding ID/reference/root, exactly three Provider IDs, and no extra Provider or Distribution. It does not require network access and exits nonzero for a malformed manifest.

- [ ] **Step 2: Run RED.**

Run:

```bash
rtk go test ./internal/provideraudit ./cmd/oaw-provider-audit -count=1
rtk bash tests/19-provider-source-audit-test.sh
```

Expected: compile or test failure because the records, generator, CLI, and committed manifest do not exist.

### GREEN

- [ ] **Step 3: Implement closed manifest records and deterministic generation.**

Implement these exported records and signatures exactly:

```go
const ProviderSourceAuditSchemaV1 = "oaw.provider-source-audit/v1"

type BindingSource struct {
    ID          string   `json:"id"`
    ContentRoot string   `json:"content_root"`
    InstallRoot string   `json:"install_root"`
    TreeDigest  string   `json:"tree_digest"`
    Kind        string   `json:"kind"`
    References  []string `json:"references"`
}

type BindingCheckout struct {
    ID          string `json:"id"`
    ContentRoot string `json:"content_root"`
    InstallRoot string `json:"install_root"`
    Root        string `json:"root"`
}

type Checkout struct {
    ProviderID       string            `json:"provider_id"`
    DistributionID  string            `json:"distribution_id"`
    SourceURI        string            `json:"source_uri"`
    Revision         string            `json:"revision"`
    Root             string            `json:"root"`
    DistributionRoot string           `json:"distribution_root"`
    BindingRoots     []BindingCheckout `json:"binding_roots"`
}

type ProviderSource struct {
    ProviderID            string          `json:"provider_id"`
    SourceURI             string          `json:"source_uri"`
    Revision              string          `json:"revision"`
    DistributionID        string          `json:"distribution_id"`
    DistributionRoot      string          `json:"distribution_root"`
    DistributionTreeDigest string         `json:"distribution_tree_digest"`
    Bindings              []BindingSource `json:"bindings"`
    EvidenceRoots         []string        `json:"evidence_roots"`
}

type Manifest struct {
    SchemaVersion         string           `json:"schema_version"`
    CanonicalMatrixDigest string           `json:"canonical_matrix_digest"`
    Providers             []ProviderSource `json:"providers"`
    Digest                string           `json:"digest"`
}

func Decode(raw []byte) (Manifest, error)
func Validate(value Manifest) error
func Build(checkouts []Checkout) (Manifest, error)
func (value Manifest) Digest() string
func (value Manifest) Binding(providerID, bindingID string) (BindingSource, bool)
```

`Decode` uses `json.Decoder.DisallowUnknownFields`, rejects trailing JSON, validates the exact revision strings, both clean relative root fields, exact built-in path mappings, and digest patterns, sorts only set-like evidence roots, and verifies the stored canonical digest. `Manifest.Providers` contains four Distribution source records across exactly three Provider IDs; repeated `oaw/superpowers` is valid only for the two exact, unique Distribution IDs above. `Build` accepts explicit `Checkout{ProviderID, DistributionID, SourceURI, Revision, Root, DistributionRoot, BindingRoots}` values whose `Root` is an exported tracked tree, calls `integrity.DigestTree` for the Distribution and every `ContentRoot`, retains the declared `InstallRoot` mapping without resolving it against the source checkout, and rejects a supplied Distribution tuple that differs from the locked specification. The CLI verifies `git rev-parse HEAD` on each source checkout before exporting the exact object; `Build` never hashes `.git` and never executes a network or Git mutation.

- [ ] **Step 4: Implement the read-only CLI and wrapper.**

The CLI accepts either:

```text
rtk go run ./cmd/oaw-provider-audit --validate --manifest internal/assets/audits/provider-sources-v4.json
rtk go run ./cmd/oaw-provider-audit --write --output internal/assets/audits/provider-sources-v4.json --matt-root /tmp/oaw-provider-audit/matt --superpowers-root /tmp/oaw-provider-audit/superpowers --openai-plugins-root /tmp/oaw-provider-audit/openai-plugins --ecc-root /tmp/oaw-provider-audit/ecc
rtk go run ./cmd/oaw-provider-audit --check --manifest internal/assets/audits/provider-sources-v4.json --matt-root /tmp/oaw-provider-audit/matt --superpowers-root /tmp/oaw-provider-audit/superpowers --openai-plugins-root /tmp/oaw-provider-audit/openai-plugins --ecc-root /tmp/oaw-provider-audit/ecc
```

The shell wrapper requires all four Distribution roots when supplied, otherwise creates a private `rtk mktemp -d`, performs read-only detached checkout of the four exact revisions, exports tracked content, runs the CLI, compares `--check` output byte-for-byte, and removes only that private directory. The `superpowers-codex` export is rooted at `plugins/superpowers` inside the pinned `openai/plugins` checkout. Network failure returns 77 for the optional drift check and never rewrites the committed manifest. No Provider install, user configuration, credentials, push, or publication is performed.

- [ ] **Step 5: Generate the concrete manifest and run GREEN.**

Run:

```bash
rtk bash scripts/audit-provider-sources.sh --write internal/assets/audits/provider-sources-v4.json
rtk bash tests/19-provider-source-audit-test.sh
rtk go test ./internal/provideraudit ./cmd/oaw-provider-audit -count=1
```

Expected: the manifest contains concrete `sha256:` digests for every root and the offline test passes. The optional source drift command may return 77 only when upstream cannot be read.

- [ ] **Step 6: Commit only source-audit files.**

```bash
rtk git add internal/provideraudit/records.go internal/provideraudit/records_test.go internal/provideraudit/generate.go internal/provideraudit/generate_test.go cmd/oaw-provider-audit/main.go cmd/oaw-provider-audit/main_test.go scripts/audit-provider-sources.sh internal/assets/audits/provider-sources-v4.json tests/19-provider-source-audit-test.sh
rtk git commit -m "test: pin provider binding source evidence"
```

## Task 2: Atomically replace built-in Descriptors, Recipes, and loader

**Files:**

- Modify: `internal/assets/providers/oaw-matt.json`
- Modify: `internal/assets/providers/oaw-superpowers.json`
- Modify: `internal/assets/providers/oaw-ecc.json`
- Modify: `internal/assets/recipes/oaw-domain-engineering.json`
- Modify: `internal/assets/recipes/oaw-delivery.json`
- Modify: `internal/assets/recipes/oaw-ecc-engineering.json`
- Modify: `internal/assets/recipes/oaw-reliable-feature.json`
- Delete: `internal/assets/recipes/oaw-hardening.json`
- Modify: `internal/assets/profile-aliases.json`
- Modify: `internal/builtin/load.go`
- Modify: `internal/builtin/load_test.go`

This task includes the complete Descriptor/Recipe/load cutover. Do not land a descriptor-only or recipe-only commit: after Plan 01, the old schema constants and Go records are gone, so all same-package consumers listed here must move together.

### RED

- [ ] **Step 1: Replace the old built-in test inventory before production edits.**

Rewrite `internal/builtin/load_test.go` with these exact tests:

```go
func TestBuiltInProviderDescriptorsV4(t *testing.T)
func TestSuperpowersDistributionsMatchHostInstallations(t *testing.T)
func TestBuiltInProviderPinsMatchSourceAudit(t *testing.T)
func TestBuiltInHostQualifiedBindingSets(t *testing.T)
func TestMattHasOnlyAuditedBindings(t *testing.T)
func TestMattRejectsFictionalRequirementsVerificationAndCompletion(t *testing.T)
func TestSuperpowersHasEveryAuditedReference(t *testing.T)
func TestSuperpowersMacroModesAreExact(t *testing.T)
func TestECCSeparatesSkillAgentRoleInstructionAndHookEvidence(t *testing.T)
func TestECCDoesNotMapE2EToVerificationOrReviewToCloseout(t *testing.T)
func TestBuiltInRecipeMatrixV3(t *testing.T)
func TestHybridDefaultProvenanceAndPausedOwners(t *testing.T)
func TestAliasesRemainExactlyFour(t *testing.T)
func TestHardeningAndRetiredAuthorityAreAbsent(t *testing.T)
func TestLoadRejectsMalformedBuiltInAsset(t *testing.T)
func TestLoadRejectsDescriptorOrRecipeRetiredSchema(t *testing.T)
func TestLoadRejectsSourceAuditDrift(t *testing.T)
func TestBuiltInAssetLoadIsDeterministic(t *testing.T)
```

The tests must compare exact ID/reference/kind/Host/invocation/content-root/install-root/span/internal-call sets, not only capability counts. They assert the flattened Matt mappings and repository-style Superpowers/ECC mappings independently. `TestSuperpowersDistributionsMatchHostInstallations` proves the direct Codex probe selects `superpowers`, the OpenAI-packaged cache probe selects `superpowers-codex`, existing `codex-*` Bindings use `superpowers-codex`, new `codex-upstream-*` and existing `claude-*` Bindings use `superpowers`, all three alternatives completely cross-reference one another, and every Superpowers Capability references all three sets. `TestMattRejectsFictionalRequirementsVerificationAndCompletion` searches both `BindingRecord.ID` and `BindingRecord.Reference` and asserts that no Matt record contains `requirements`, `verification-loop`, or a completion binding. `TestECCSeparatesSkillAgentRoleInstructionAndHookEvidence` asserts that `kind=agent` appears only with `host=claude`, the three exact role IDs are the only Codex role records, `commands/plan.md` and `commands/feature-dev.md` are instructions, and no Hook appears in `Bindings`. `TestSuperpowersMacroModesAreExact` asserts that SDD has exactly one dispatch-before workspace call and one dispatch-after finish call, that its embedded review is not encoded as another Binding call, and that TDD and fresh verification remain standalone Recipe units. It rejects fictional SDD calls to `test-driven-development`, `requesting-code-review`, or `verification-before-completion`.

- [ ] **Step 2: Run RED against the complete package boundary.**

Run:

```bash
rtk go test ./internal/builtin -run 'BuiltIn|Matt|Superpowers|ECC|Aliases|Hardening|Load' -count=1
```

Expected: failure from the old v3/v2 assets and loader. This is the intended RED state; do not weaken the test or add aliases for retired records.

### GREEN

- [ ] **Step 3: Write Descriptor v4 assets from the locked binding sets.**

Set every Descriptor to `schema_version = oaw.provider-descriptor/v4` and `descriptor_version = 4.0.0`. Add one `DistributionRecord` per pinned source and one Host-and-Distribution-qualified `BindingRecord` per exact Host surface. Matt and ECC retain their existing `codex-`/`claude-` rules. For Superpowers, existing `codex-` IDs are the OpenAI-packaged `superpowers-codex` Bindings, new `codex-upstream-` IDs are direct-upstream `superpowers` Bindings, and `claude-` IDs remain `superpowers` Bindings; the `reference` field remains the exact upstream name. Each trio's alternatives list the other two exact IDs, and every Superpowers `CapabilityRecord.binding_refs` includes the complete trio. Every Binding's `ContentRoot`, `InstallRoot`, and `tree_digest` must equal its own Distribution row in the source audit manifest; no source-equivalence shortcut is allowed.

Encode the Matt rows and responsibilities exactly as the audit table: `grill-with-docs` credits `grilling` and `domain-modeling`; `to-spec`, `to-tickets`, and `implement` are `human-explicit`; `implement` uses the slots 5-8 macro envelope, calls `tdd` and `code-review` as credit-only internals, has no incident, completion, or workspace responsibility, and the internal `code-review` is the exact slot 8 owner; `diagnosing-bugs` accepts only functional/hard-bug/performance incident types; `code-review` requires child and parallel-child (nested equivalents for outer `SUBAGENT`).

Encode the eleven Superpowers skill rows with the `superpowers:` reference namespace. `brainstorming` has a dispatch-after `writing-plans` call. SDD has only dispatch-before workspace and dispatch-after finish cross-skill calls; it directly declares its embedded per-task/final two-stage review responsibility, while `test-driven-development` and `verification-before-completion` are standalone Recipe units. It must not claim calls to those skills or to `requesting-code-review`. Inline execution has dispatch-before workspace and dispatch-after finish, with standalone TDD, review/remediation, and verification units in the Recipe. `requesting-code-review` and Matt `code-review` carry their exact reviewer delegation requirements. No internal call is both credited and dispatched.

Encode ECC's nine cross-Host skills, seven Claude Agents, three Codex Roles, and two exact instruction surfaces as separate Binding kinds. Do not create Codex Agent rows for `architect`, `planner`, `tdd-guide`, `build-error-resolver`, `code-reviewer`, `security-reviewer`, or `e2e-runner`; do not create a `delivery-gate` Binding. Capabilities reference `verification-loop` for broad verification, E2E only as an optional specialist, and `git-workflow` plus Host action/user gate for closeout.

- [ ] **Step 4: Write the four Recipe v3 assets and remove the hidden lifecycle.**

Set every active Recipe to `schema_version = oaw.profile-recipe/v3`, `taxonomy_version = oaw.lifecycle-taxonomy/v1`, and `recipe_version = 3.0.0`. Write ten slots in canonical order using the matrix above. Gates contain only `authority`, `predicate`, and evidence requirements; Host actions contain only their declared action IDs and artifact contracts; no gate has a Provider selector.

Use these exact Recipe identities and provenance:

| Recipe ID | Family | Template |
| --- | --- | --- |
| `oaw/domain-engineering` | `matt` | empty |
| `oaw/delivery` | `superpowers` | empty |
| `oaw/ecc-engineering` | `ecc` | empty |
| `oaw/reliable-feature` | `matt-superpowers` | `default` |

The MATT-FULL, SP-FULL, ECC-FULL, and Hybrid slot contracts are the locked sections above. Encode like-shaped Host alternatives through `BindingRecord.Alternatives`, stable Recipe step/overlay IDs, and exact selectors. Runtime requests pass `profile.AlternativeChoice` values for those declared identities; `AlternativeChoice` is not a second Recipe wire field and must not represent a whole pipeline-shape replacement. Never infer a Claude Agent from a same-named Codex role. Built-in SP-FULL uses the inline executor; a different SDD ownership shape is created through a versioned USER-DEFINED Recipe. The Hybrid's default `executing-plans` overlay pauses SDD and Superpowers TDD, retains Matt TDD, and uses the standalone Superpowers review pipeline. It is a suppression-only overlay with no `selected_alternative`; it does not invent a self-alternative for the already selected `executing-plans` Binding. Its optional ECC build-repair/security checks are add-ons, not active default units.

Delete `oaw-hardening.json` and assert no active Catalog or alias references it. Preserve exactly these aliases and no more: `SP-FULL -> oaw/delivery`, `MATT-FULL -> oaw/domain-engineering`, `ECC-FULL -> oaw/ecc-engineering`, and `MATT-SP-HYBRID -> oaw/reliable-feature`. `USER-DEFINED` is a selection action and never an alias.

- [ ] **Step 5: Switch the built-in loader and embedding atomically.**

Change `internal/builtin/load.go` to validate `ProviderDescriptorV4` and `ProfileRecipeV3`, enumerate exactly three Provider files and four Recipe files, decode the strict alias set, and reject any retired v3/v2 authority record. Read `audits/provider-sources-v4.json` through `provideraudit.Decode`, cross-check every Descriptor Distribution/Binding root/revision/digest before returning the Catalog, and expose these exact helpers:

```go
func LoadSourceAudit() (provideraudit.Manifest, error)
func loadSourceAuditFromFS(files fs.FS) (provideraudit.Manifest, error)
```

Keep `loadFromFS` deterministic and return stable `BUILTIN_*` error prefixes, including `BUILTIN_SOURCE_AUDIT_INVALID` for source drift. Existing `audits/*.json` embedding already covers the new manifest; the explicit Profile Matrix path is added to `internal/assets/embed.go` in Task 3. Do not add a compatibility reader or a temporary old-schema alias.

- [ ] **Step 6: Run the atomic GREEN gate.**

Run:

```bash
rtk gofmt -w internal/builtin/load.go internal/builtin/load_test.go
rtk go test ./internal/catalog ./internal/schema ./internal/builtin -run 'BuiltIn|Matt|Superpowers|ECC|Aliases|Hardening|Load' -count=1
```

Expected: all Descriptor/Recipe tests pass; the package contains exactly three Providers, four Distributions, four Recipes, and four aliases. The integration and matrix tests remain deferred to Tasks 3-4.

- [ ] **Step 7: Commit the hard cutover with exact paths.**

```bash
rtk git add internal/assets/providers/oaw-matt.json internal/assets/providers/oaw-superpowers.json internal/assets/providers/oaw-ecc.json internal/assets/recipes/oaw-domain-engineering.json internal/assets/recipes/oaw-delivery.json internal/assets/recipes/oaw-ecc-engineering.json internal/assets/recipes/oaw-reliable-feature.json internal/assets/recipes/oaw-hardening.json internal/assets/profile-aliases.json internal/builtin/load.go internal/builtin/load_test.go
rtk git commit -m "fix: replace builtin provider and profile assets"
```

## Task 3: Generate and validate the canonical Profile Matrix projection

**Files:**

- Create: `internal/assets/profile-matrix.json`
- Create: `internal/builtin/matrix.go`
- Create: `internal/builtin/matrix_test.go`
- Modify: `internal/builtin/load.go`
- Modify: `internal/builtin/load_test.go`
- Modify: `internal/assets/embed.go`

The matrix is a declarative projection of the four Recipes and their exact Descriptor Bindings. It does not contain live Host session evidence and therefore cannot make a Profile eligible. Runtime eligibility remains the Plan 03 compiler's decision.

### RED

- [ ] **Step 1: Write matrix record tests before adding the projection.**

Add these tests:

```go
func TestProfileMatrixProjectionHasCanonicalTenSlots(t *testing.T)
func TestProfileMatrixProjectionIncludesFourAliases(t *testing.T)
func TestProfileMatrixProjectionIncludesHostSurfaceAndSourcePins(t *testing.T)
func TestProfileMatrixProjectionPreservesPipelineAndMacroOrder(t *testing.T)
func TestProfileMatrixProjectionMarksHybridPausedBindings(t *testing.T)
func TestProfileMatrixProjectionRejectsFictionalOrHookBinding(t *testing.T)
func TestCommittedProfileMatrixMatchesProjectionByteForByte(t *testing.T)
func TestCommittedProfileMatrixRejectsDigestDrift(t *testing.T)
func TestLoadRejectsProfileMatrixDrift(t *testing.T)
```

The tests must assert the exact canonical matrix digest `49ec1819ab22364d763d0875d9af299ee332de3d6d39a7178a715c2b13272ccf`, four profile IDs, ten ordered slot IDs per profile, every Host-specific Binding reference, every Host action ID (`workspace.prepare-or-confirm`, `verification.execute`, `closeout.execute`), every macro disposition, and Hybrid paused/credited decisions. They must reject any matrix row whose Provider/Binding pair is absent from a Descriptor or whose reference is `requirements`, `delivery-gate`, or a fictional Codex Agent.

- [ ] **Step 2: Run RED.**

Run:

```bash
rtk go test ./internal/builtin -run 'ProfileMatrix|MatrixDrift' -count=1
```

Expected: failure because the projection type, validator, embedded asset, and loader check do not exist.

### GREEN

- [ ] **Step 3: Implement the closed projection and exact API.**

Add these records and functions in `internal/builtin/matrix.go`:

```go
const ProfileMatrixSchemaV1 = "oaw.profile-matrix/v1"

type MatrixBinding struct {
    ProviderID         string                      `json:"provider_id"`
    BindingID          string                      `json:"binding_id"`
    Host               string                      `json:"host"`
    Surface            string                      `json:"surface"`
    Kind               catalog.BindingKind        `json:"kind"`
    Reference          string                      `json:"reference"`
    Invocation         catalog.InvocationDisposition `json:"invocation"`
    StageSpan          []catalog.SlotID            `json:"stage_span"`
    MacroMode          catalog.InternalCallMode    `json:"macro_mode,omitempty"`
    DistributionRevision string                    `json:"distribution_revision"`
    BindingTreeDigest  string                      `json:"binding_tree_digest"`
    RequiredFeatures   []host.FeatureID            `json:"required_features"`
    Topologies         []execution.Topology        `json:"topologies"`
    Paused             bool                        `json:"paused"`
}

type MatrixSlot struct {
    SlotID       catalog.SlotID       `json:"slot_id"`
    Applicability catalog.SlotApplicability `json:"applicability"`
    OutcomeOwner string               `json:"outcome_owner"`
    Pipeline     []MatrixBinding      `json:"pipeline"`
    HostActionID string              `json:"host_action_id,omitempty"`
    GateIDs      []string             `json:"gate_ids"`
    IncidentTypes []string            `json:"incident_types"`
}

type MatrixProfile struct {
    Alias       string       `json:"alias"`
    RecipeID    string       `json:"recipe_id"`
    Family      string       `json:"family"`
    Template    string       `json:"template,omitempty"`
    RecipeDigest string      `json:"recipe_digest"`
    Slots       []MatrixSlot `json:"slots"`
}

type ProfileMatrixRecord struct {
    SchemaVersion         string          `json:"schema_version"`
    CanonicalMatrixDigest string          `json:"canonical_matrix_digest"`
    SourceAuditDigest     string          `json:"source_audit_digest"`
    Profiles              []MatrixProfile `json:"profiles"`
    Digest                string          `json:"digest"`
}

func BuildProfileMatrix(value catalog.Catalog, audit provideraudit.Manifest) (ProfileMatrixRecord, error)
func DecodeProfileMatrix(raw []byte) (ProfileMatrixRecord, error)
func ValidateProfileMatrix(value catalog.Catalog, audit provideraudit.Manifest, matrix ProfileMatrixRecord) error
func LoadProfileMatrix() (ProfileMatrixRecord, error)
func loadProfileMatrixFromFS(files fs.FS) (ProfileMatrixRecord, error)
```

`BuildProfileMatrix` walks Recipe slots in taxonomy order, resolves every selector against the corresponding Descriptor, copies source revision/tree evidence from the audit manifest, preserves pipeline/internal-call/overlay order, and sorts only profile IDs, set-like gates, and diagnostic identity sets. It does not sort a pipeline or macro call list. `DecodeProfileMatrix` is closed-world JSON with stored-digest verification. `ValidateProfileMatrix` checks the four aliases, exact Recipe digests, exact Binding provenance, ten slots, and the fixed canonical matrix digest. `Load` invokes this validator after Catalog loading; a changed matrix or source audit fails with `BUILTIN_PROFILE_MATRIX_INVALID`.

- [ ] **Step 4: Generate the committed projection under an explicit test-only update flag.**

Add a `-update` guard to `TestWriteProfileMatrix`; without that flag the test must never write. Generate and verify with:

```bash
rtk go test ./internal/builtin -run TestWriteProfileMatrix -update -count=1
rtk go test ./internal/builtin -run 'ProfileMatrix|MatrixDrift' -count=1
```

Expected: `internal/assets/profile-matrix.json` is canonical JSON, contains concrete revision/tree digests, and a second projection is byte-identical.

- [ ] **Step 5: Embed the matrix and run GREEN.**

Add `profile-matrix.json` to the explicit `//go:embed` list, update `load.go` to read it through `LoadProfileMatrix`, and extend `load_test.go` with `TestLoadEmbedsProfileMatrix`. Run:

```bash
rtk gofmt -w internal/builtin/matrix.go internal/builtin/matrix_test.go internal/builtin/load.go internal/builtin/load_test.go internal/assets/embed.go
rtk go test ./internal/catalog ./internal/schema ./internal/builtin -run 'ProfileMatrix|MatrixDrift|Load' -count=1
```

Expected: the committed projection and generated projection match byte-for-byte, and loader validation rejects any changed source/recipe/matrix digest.

- [ ] **Step 6: Commit the matrix projection.**

```bash
rtk git add internal/assets/profile-matrix.json internal/builtin/matrix.go internal/builtin/matrix_test.go internal/builtin/load.go internal/builtin/load_test.go internal/assets/embed.go
rtk git commit -m "test: lock builtin profile matrix parity"
```

## Task 4: Replace built-in/profile integration coverage

**Files:**

- Modify: `internal/integration/profile_compiler_test.go`
- Create: `internal/integration/profile_fixture_test.go`

This task owns all cross-package tests for built-in Profile compilation and the USER-DEFINED composition path. It does not alter production compiler behavior; failures are fixed in the owning Plan 01-03 package.

### RED

- [ ] **Step 1: Replace the stale v2 integration tests and create exact fixtures.**

Delete the old `GraphNode`, `ProfileBinding`, `HostTopologies`, and v2 `CompileError` helpers. Add a fixture builder in `profile_fixture_test.go` that:

1. loads the Catalog and source audit through `builtin.Load`/`builtin.LoadSourceAudit`;
2. builds a `host.Manifest`, `host.SessionSnapshot`, `host.BindingInventory`, and `host.EnvironmentReport` with the exported Plan 02 constructors;
3. sets every observed Binding's Host, surface, kind, exact reference, Distribution ID, revision, tree digest, and topology from the Descriptor;
4. marks live delegation features (`child-delegation`, `parallel-child-delegation`, `nested-child-delegation`, and `nested-parallel-child-delegation`) only in the complete fixture;
5. marks the three Host actions as live native actions only in the complete fixture;
6. constructs an immutable `profile.HostEvidence` with `profile.NewHostEvidence`; and
7. supplies an `EffectiveRegistry` fixture implementing all locked methods (`HostID`, `Providers`, `Provider`, `Binding`, `Bindings`, `Capability`, `Digest`) with every audited Binding retained, never a lexical fallback.

Add separate fixtures named `completeCodexEvidence`, `completeClaudeEvidence`, and `currentCodexV1Evidence`. The current v1 fixture deliberately exposes only `skill` Bindings, `CURRENT`, the currently observed Matt/Superpowers surfaces, no role/instruction/action records, and no child-delegation feature; it is not upgraded in this plan.

Replace `profile_compiler_test.go` with these exact tests:

```go
func TestAllFourBuiltInAliasesCompileWithCompleteCodexEvidence(t *testing.T)
func TestAllFourBuiltInAliasesCompileWithCompleteClaudeEvidence(t *testing.T)
func TestBuiltInGraphsMatchDeclaredMatrix(t *testing.T)
func TestMattGraphRequiresFreshExplicitInvocationAttestation(t *testing.T)
func TestMattGraphUsesNeutralHostActionsForWorkspaceVerificationAndCloseout(t *testing.T)
func TestMattGraphHasNoFictionalRequirementsVerificationOrCompletionBinding(t *testing.T)
func TestSuperpowersSDDRequiresChildAndNestedDelegation(t *testing.T)
func TestSuperpowersBuiltInInlineKeepsReviewerChildRequirement(t *testing.T)
func TestSuperpowersMacroInternalsAreDispatchedExactlyOnce(t *testing.T)
func TestECCCodexAndClaudeSurfaceChoicesRemainDistinct(t *testing.T)
func TestECCCurrentCodexV1ReportsExactMissingFacts(t *testing.T)
func TestECCNeverUsesE2EAsBroadVerificationOrReviewerAsCloseout(t *testing.T)
func TestHybridDefaultUsesMattTDDAndSPInlineReview(t *testing.T)
func TestHybridSDDCloneRetainsSingleMattTDD(t *testing.T)
func TestHybridNoAddOnStopsBuildTypeAndDependencyIncidents(t *testing.T)
func TestUSERDEFINEDCloneComposesVerifiedBindingsWithoutMutatingTemplate(t *testing.T)
func TestUSERDEFINEDUntrustedSameNameBindingIsUnavailable(t *testing.T)
func TestBuiltInCompilationDiagnosticsAreStableAndSorted(t *testing.T)
func TestEquivalentBuiltInInputsProduceIdenticalGraphDigest(t *testing.T)
```

- [ ] **Step 2: Run RED.**

Run:

```bash
rtk go test ./internal/integration -run 'BuiltIn|Superpowers|ECC|Hybrid|USERDEFINED|Matrix|Matt' -count=1
```

Expected: compile failure from stale v2 helper names and absent v4 fixture/compiler calls. Do not retain a compatibility adapter to make the old test compile.

### GREEN

- [ ] **Step 3: Implement complete synthetic Host/Registry fixtures.**

The fixture must use descriptor-derived values and the audit manifest, not fabricated skill names. For a complete Codex run, select the `codex-*` alternatives and attest all required skill/role/instruction/action/feature records. For a complete Claude run, select the `claude-*` alternatives and attest the Claude Agent records plus cross-Host skills. The fixture's `BindingEvidenceDigest` and Registry digest must change when any source or Host observation changes.

The current Codex v1 test must assert `CompileResult.Graph()` is false and collect exact diagnostic codes. It must distinguish `PROVIDER_BINDING_UNAVAILABLE`, `BINDING_KIND_UNSUPPORTED`, `HOST_FEATURE_UNATTESTED`, and `HOST_ACTION_UNAVAILABLE` where applicable; a generic “ECC not installed” message is insufficient. It must also prove that a same-named file under a shared `.agents/skills` ancestor cannot satisfy a Matt Binding.

- [ ] **Step 4: Implement the profile assertions against the public compiler.**

For every successful graph, assert:

```go
record.SchemaVersion == profile.ExecutionGraphSchemaV4
record.TaxonomyVersion == catalog.TaxonomyVersionV1
len(record.Slots) == 10
record.RegistryDigest == fixture.Registry.Digest()
record.HostEvidenceDigest == fixture.HostEvidence.Digest()
profile.ValidateExecutionGraphRecord(record) == nil
```

`TestBuiltInGraphsMatchDeclaredMatrix` compares each slot's owner, ordered pipeline, Host action ID, gate IDs, source revision, Binding tree digest, macro mode, and paused/credited decision with the committed `ProfileMatrixRecord`; it does not compare volatile Host evidence or opaque handles.

`TestMattGraphRequiresFreshExplicitInvocationAttestation` checks `RequiresExplicitInvocation` on `grill-with-docs`, `to-spec`, `to-tickets`, and `implement` and verifies that the compiler does not treat Profile selection as a recorded human invocation. `TestMattGraphUsesNeutralHostActionsForWorkspaceVerificationAndCloseout` asserts no Provider owns those three control actions.

`TestSuperpowersSDDRequiresChildAndNestedDelegation` compiles a versioned USER-DEFINED SDD clone, removes one feature at a time, and expects fail-closed diagnostics. `TestSuperpowersBuiltInInlineKeepsReviewerChildRequirement` proves the built-in path has no implementation-child requirement but keeps the standalone reviewer child. `TestSuperpowersMacroInternalsAreDispatchedExactlyOnce` counts cursors in the SDD clone and confirms exactly one dispatch-before workspace cursor and one dispatch-after finish cursor, while TDD and fresh verification each have one standalone cursor and the embedded SDD review creates no fictional Binding cursor.

`TestECCCodexAndClaudeSurfaceChoicesRemainDistinct` supplies a Codex Agent observation named `planner` and expects it not to satisfy the Claude Agent binding; it supplies the exact Codex `reviewer` role and accepts only the role path. It separately proves that a Claude Agent cannot satisfy a Codex skill selector. `TestECCNeverUsesE2EAsBroadVerificationOrReviewerAsCloseout` inspects the compiled owner kinds and rejects those substitutions.

`TestHybridDefaultUsesMattTDDAndSPInlineReview` asserts the exact default overlay: `executing-plans`, Matt `tdd`, Matt functional debugging, standalone SP review/remediation, SP fresh verification/finish, SDD and SP TDD paused, and no ECC Provider unit. `TestHybridSDDCloneRetainsSingleMattTDD` proves that a versioned SDD clone keeps only Matt TDD, uses SDD's embedded review responsibility, and requires the exact delegation evidence. `TestHybridNoAddOnStopsBuildTypeAndDependencyIncidents` asserts the three routes have `if_unavailable=stop`; it does not silently add ECC.

`TestUSERDEFINEDCloneComposesVerifiedBindingsWithoutMutatingTemplate` clones `oaw/reliable-feature` to a new versioned Recipe, replaces one ordered slot pipeline with two compatible audited Bindings, compiles it with the same compiler, and asserts the original Recipe digest, matrix, aliases, and default overlay are unchanged. `TestUSERDEFINEDUntrustedSameNameBindingIsUnavailable` inserts a same-name foreign Binding without a trusted Descriptor and expects `PROVIDER_PROVENANCE_MISMATCH` or `PROVIDER_BINDING_UNAVAILABLE`. The custom Recipe is not added to the built-in alias set.

- [ ] **Step 5: Run GREEN, race, and integration verification.**

Run:

```bash
rtk gofmt -w internal/integration/profile_compiler_test.go internal/integration/profile_fixture_test.go
rtk go test ./internal/catalog ./internal/profile ./internal/registry ./internal/host ./internal/builtin ./internal/integration -run 'BuiltIn|Superpowers|ECC|Hybrid|USERDEFINED|Matrix|Matt' -count=1
rtk go test -race ./internal/builtin ./internal/integration -run 'BuiltIn|Superpowers|ECC|Hybrid|USERDEFINED|Matrix|Matt' -count=1
```

Expected: all four aliases compile only with complete evidence, current Codex v1 fails with exact reasons, Hybrid defaults to `CURRENT` inline/no Add-on semantics, and all graph digests and matrix projections are deterministic.

- [ ] **Step 6: Commit only built-in/profile integration coverage.**

```bash
rtk git add internal/integration/profile_compiler_test.go internal/integration/profile_fixture_test.go
rtk git commit -m "test: verify builtin profile matrix compilation"
```

## Final Plan Verification

- [ ] Run the source audit and all focused tests:

```bash
rtk bash tests/19-provider-source-audit-test.sh
rtk go test ./internal/catalog ./internal/schema ./internal/provideraudit ./cmd/oaw-provider-audit ./internal/profile ./internal/registry ./internal/host ./internal/builtin ./internal/integration -count=1
rtk go test -race ./internal/builtin ./internal/integration -count=1
rtk go vet ./internal/provideraudit ./cmd/oaw-provider-audit ./internal/builtin ./internal/integration
```

- [ ] Run exact semantic scans against the changed assets:

```bash
rtk rg -n '"requirements"|"verification-loop"|"completion"' internal/assets/providers/oaw-matt.json
rtk rg -n '"kind": *"agent"|"kind": *"role"|"kind": *"instruction"|delivery-gate|e2e-runner' internal/assets/providers/oaw-ecc.json
rtk rg -n 'oaw/hardening|CUSTOM-LOCKED|oaw\.provider-descriptor/v3|oaw\.profile-recipe/v2' internal/assets/providers internal/assets/recipes internal/assets/profile-aliases.json internal/builtin
```

The first scan against the Matt asset must produce no output. The ECC scan must show only the exact declared Agent/Role/Instruction/E2E specialist rows and no Hook Binding or closeout/verification substitution. The retired-schema scan may match only explicit rejection fixtures.

- [ ] Check formatting and ownership without touching unrelated work:

```bash
rtk git diff --check -- docs/superpowers/plans/2026-08-10-oaw-provider-surface-v4-04-builtins-profile-matrix.md
rtk git status --short -- docs/superpowers/plans/2026-08-10-oaw-provider-surface-v4-04-builtins-profile-matrix.md
```

Expected: no plan whitespace errors, exact four aliases and four Recipes remain, all active Binding references trace to a pinned source root, no broad or fictional skill claim exists, and the integration tests own only v4 built-in/profile behavior.

## Acceptance Coverage Map

The v4 design's seventeen acceptance criteria are covered by this plan and the owning downstream plan as follows:

| Criterion | Coverage in this plan |
| --- | --- |
| 1. Ten canonical slots and typed ownership are machine validated | Task 2 Recipe tests and Task 3 matrix slot validation |
| 2. Ordered N:M pipelines and contiguous macro spans | Task 2 macro asset assertions; Task 3 order/parity tests; Task 4 cursor assertions |
| 3. Immutable Distribution and Host-surface provenance | Task 1 manifest tests; Task 2 source-pin tests; Task 4 Registry fixture |
| 4. Host actions and neutral gates cannot masquerade as Provider skills | Task 2 asset tests; Task 4 `TestMattGraphUsesNeutralHostActionsForWorkspaceVerificationAndCloseout` |
| 5. Exactly one applicable outcome owner | Task 2 Recipe ownership tests and Task 4 graph/matrix comparison |
| 6. Macro internals expand once without double dispatch | Task 2 `TestSuperpowersMacroModesAreExact`; Task 4 `TestSuperpowersMacroInternalsAreDispatchedExactlyOnce` |
| 7. All four built-in aliases remain active | Task 2 alias/Recipe tests and Task 4 four-alias compilation tests |
| 8. Matt uses `grill-with-docs`, never fictional `requirements` | Task 2 Matt negative tests and Task 4 Matt graph assertions |
| 9. Matt has no false verification or completion Binding | Task 2 Matt negative tests and Task 4 neutral-action assertions |
| 10. SP child/nested-child eligibility is enforced | Task 2 delegation metadata and Task 4 SDD/inline feature-removal tests |
| 11. ECC Skill/Agent/Role/Instruction/Hook surfaces stay distinct | Task 2 ECC surface test and Task 4 Codex/Claude distinction test |
| 12. Hybrid/default uses Matt TDD with compatible SP inline path | Task 2 Hybrid Recipe test and Task 4 Hybrid default/overlap tests |
| 13. USER-DEFINED combines verified compatible Bindings and fails closed | Task 4 clone/composition and untrusted same-name tests; Plan 03 owns Builder mechanics |
| 14. Retired authority cannot execute | Task 2 loader hard-cut tests; Plan 03/05 own graph/Bundle rejection |
| 15. Fresh Host re-observation and new START are truthful | Task 4 current-Codex gap fixture; Plan 06 owns live re-observation and START |
| 16. Tests, coverage, security, docs, and fresh verification pass | Task 1-4 focused/race/vet gates; Plans 05-06 own repository-wide gates |
| 17. No unrelated or external mutation | Read-only audit wrapper, exact-path commits, and final status/diff checks |

## Cross-Plan Questions That Must Be Closed Before Execution

1. Plan 01 must export the exact `BindingRecord` including `ContentRoot` and `InstallRoot`, `InternalCall`, `SlotRecipe`, `OutcomeOwner`, Host-action, neutral-gate, and shared Recipe normalization/digest contracts used by the JSON tables and must remove old records atomically; otherwise Task 2 cannot compile.
2. Plan 02 must preserve Host pause, cancellation, invocation deduplication, provider inventory, normalized receipts, and environment reporting while adding the four delegation Feature IDs and the three neutral Host actions. It must expose all Binding alternatives through `Registry.Bindings` and keep static `host-configured` evidence from satisfying `available`.
3. Plan 03 must retain the locked `CompileProfile`/`CompileRecipe` return contract (`CompileResult`, nil Go error for expected ineligibility), `NewHostEvidence` plus `ValidateHostEvidenceRecord`, macro dispositions, and cursor traversal. It must not require integration tests or built-in assets in its own package gate.
4. Plan 05 must record Matt's explicit user invocation attestations and host-action outcome artifacts durably before PREPARE/dispatch, and must consume the graph's exact cursors without re-compiling the matrix. Its full-tree gate cannot run before Plan 06's Bridge migration.
5. Plan 06 must keep old built-in/profile fixture rewrites out of its file map and must not edit `internal/integration/profile_compiler_test.go`. It may update only the Host fixture assertions in `internal/builtin/load_test.go`, preserving every Profile assertion established by this plan. The final Coordinator START must happen only after fresh matrix/integration evidence and a new Host observation.

No unresolved question permits a compatibility reader, a guessed skill, or a silent Host fallback. If a Host surface is not attested, the corresponding Profile/alternative remains visible with a reason-coded exclusion and does not compile.
