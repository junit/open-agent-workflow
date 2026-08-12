# OAW Provider Surface v4 Fixed-Point Review

## Final Increment Review (2026-08-12)

The final increment was reviewed after the previous authority findings were remediated.

- START now closes embedded Graph Host evidence, Registry, Recipe, and Graph Selection pins against trusted inputs.
- Workflow Snapshot schema now requires the runtime-consistent active Grant/Dispatch pair and rejects inactive residual authority.
- Immutable source manifest reads are rooted, descriptor-bound, and revalidated after decode to reject replacement/symlink swaps.
- Distribution tree hashing records contained relative symlinks without following them and rejects escaping targets; fixed pinned-source audit passes.
- No new CRITICAL, HIGH, or MEDIUM correctness/security finding was found in the remediation increment.
- LOW follow-up: `cmd/oaw-provider-audit` still buffers unbounded `ls-tree` and blob output; add entry/byte/export limits in a future hardening change.
- Plan 06 live child-delegation START acceptance remains unavailable and is recorded as `NOT_SUBMITTED`, not passed by inference.

implementation_base: 2a4e9fb189e01d5c3f6fe7242d3f65bf500656e7
initial_reviewed_head: 8ce84e8
review_started_at: 2026-08-11T05:32:22Z
review_scope: complete committed diff from implementation_base through initial_reviewed_head
review_command: rtk git diff 2a4e9fb189e01d5c3f6fe7242d3f65bf500656e7..8ce84e8
selected_lifecycle: SP-FULL / CURRENT / no Add-on
review_completed_at: 2026-08-11T06:21:14Z
remediation_plan_amendments: 5e01027, 32cd1f8, 8312502, 3cff78d, dc2dae1
remediation_commit: 19451ed
reopened_remediation_commit: acbfd65
reopened_review_completed_at: 2026-08-11T07:18:10Z
increment_review_base: dc2dae1
increment_review_status: remediated

## Spec Compliance

### MEDIUM: Workflow Snapshot v2 schema is more permissive than Coordinator decoding

- location: internal/assets/schemas/v2/workflow-snapshot.schema.json:8
- location: internal/assets/schemas/v2/workflow-snapshot.schema.json:15
- evidence: root classification, bundle classification, and bundle configuration use
  `additionalProperties: true`, while Coordinator state decoding uses
  `json.Decoder.DisallowUnknownFields`. The public schema therefore accepts records
  that the authoritative runtime rejects.
- re-review evidence: after closing those objects, the inline `classification`
  and `configuration` definitions still omitted `required`. Both Go records
  marshal every non-optional field, so the schema continued accepting empty or
  structurally incomplete records outside the runtime contract.
- disposition: FIX
- remediation disposition: Modify internal/assets/embed_test.go
- remediation disposition: Modify internal/assets/schemas/v2/workflow-snapshot.schema.json
- remediation disposition: Modify internal/schema/registry_v2_test.go
- RED command: rtk go test ./internal/assets -run TestEmbeddedSchemasHaveStableMetadata -count=1
- expected RED: the active Workflow Snapshot v2 document contains open object schemas.
- RED command: rtk go test ./internal/assets -run TestWorkflowSnapshotV2RequiresRuntimeProjectionFields -count=1
- expected RED: inline classification and configuration definitions omit their
  runtime-required fields.
- plan amendment: 32cd1f8 adds the positive schema fixture path to the Plan 06 File Map
- RED command: rtk go test ./internal/assets ./internal/schema ./internal/coordinator -count=1
- observed RED: TestRegistryWorkflowSnapshotV2AcceptsAlternativeChoice uses
  empty Classification and Configuration objects that the corrected schema
  rejects.
- GREEN command: rtk go test ./internal/schema -run TestRegistryWorkflowSnapshotV2AcceptsAlternativeChoice -count=1
- GREEN command: rtk go test ./internal/assets -run TestEmbeddedSchemasHaveStableMetadata -count=1
- owning gate: rtk go test ./internal/assets ./internal/schema ./internal/coordinator -count=1
- re-review status: resolved; the schema is closed, runtime-required fields are
  explicit, closed Graph v4 item definitions are reused, and the positive
  registry fixture validates the complete projection.

### MEDIUM: Config v4 tests retain pre-cutover Recipe assumptions

- location: internal/config/config_test.go:624
- location: internal/config/config_test.go:1137
- evidence: the built-in-only test expects five Recipes although the approved v4
  catalog contains four active aliases/Recipes, and `testReviewRecipeTOML`
  declares no canonical slots. The empty Recipe causes valid merge coverage to
  fail and prevents the reserved-namespace negative test from reaching the rule
  it claims to exercise.
- disposition: FIX
- remediation disposition: Modify internal/config/config_test.go
- RED command: rtk go test ./internal/config -run '^TestLoadBuildsBuiltInOnlySnapshotWithoutFiles$' -count=1
- RED command: rtk go test ./internal/config -run '^TestLoadMergesUserAndTrustedProjectWholeRecords$' -count=1
- RED command: rtk go test ./internal/config -run '^TestLoadRejectsReservedUserRecipeNamespace$' -count=1
- observed RED: catalog count mismatch 3/4/4 and RECIPE_SLOT_COVERAGE_INVALID before the intended assertions.
- GREEN command: rtk go test ./internal/config -count=1
- owning gate: rtk go test ./internal/integrity ./internal/discovery ./internal/config ./internal/codexbridge/... -count=1
- re-review status: resolved; all config tests and the owning
  integrity/discovery/config/Bridge gate pass with the complete ten-slot fixture.

### MEDIUM: Legacy check diagnostics fixtures use superseded Provider probes

- location: internal/check/diagnostics_test.go:40
- location: internal/check/diagnostics_test.go:82
- location: internal/check/diagnostics_test.go:129
- evidence: three stable full-tree failures still construct Matt from four
  individual Skill files, treat the ECC Marketplace Plugin probe as missing,
  and use those same pre-v4 fixtures for the all-Provider readiness case. The
  production compatibility catalog now verifies Matt through
  `.agents/.skill-lock.json` or its Claude cache distribution and verifies ECC
  through the registered Plugin probe, matching the already migrated
  `internal/management/management_check_test.go` fixtures.
- disposition: FIX
- plan amendment: 5e01027 adds this exact test path to the Plan 06 File Map
- remediation disposition: Modify internal/check/diagnostics_test.go
- RED command: rtk go test ./internal/check -run 'TestExecuteRequiresCompleteMattCompatibilityBundle|TestExecuteKeepsECCCompatibilityIndicatorNarrow|TestExecuteReportsBuiltInProvidersAndProjectReadiness' -count=1
- expected RED: Matt remains missing after the obsolete four-skill fixture,
  the ECC Plugin probe is detected where the old expectation says missing, and
  the readiness fixture reports Matt/ECC missing.
- GREEN command: rtk go test ./internal/check -count=1
- owning gate: rtk go test ./... -count=1
- re-review status: resolved; the three original RED cases and all 35
  `internal/check` tests pass with the v4 probes.

### LOW: Codex App Server client metadata still reports 1.0.0

- location: internal/codexbridge/appserver/client.go:310
- evidence: App Server `clientInfo.version` remains `1.0.0` while the Bridge
  Integration declares `2.0.0`.
- disposition: DEFER
- rationale: this field is diagnostic client metadata and is not VersionEvidence
  authority. The path predates the v4 cutover and does not appear in Plans 01-06
  File Maps, so the current remediation ledger does not authorize an edit.
- re-review status: accepted follow-up risk

### MEDIUM: Legacy Bash check still accepts retired Matt and ECC probes

- location: lib/detect.sh:3
- location: tests/02-check-test.sh:71
- location: tests/11-check-parity-test.sh:140
- evidence: the mandatory shell suite failed after Go management correctly
  rejected the retired Matt four-Skill fixture. The parity test independently
  showed Bash reporting Matt/ECC detected from the four individual Matt Skills
  and `.agents/skills/everything-claude-code/SKILL.md`, while Go reported both
  missing. The test-only legacy management oracle therefore retained the exact
  invalid compatibility behavior removed by the v4 hard cut.
- disposition: FIX
- plan amendment: 8312502 adds the three exact shell paths to the Plan 06 File Map
- remediation disposition: Modify lib/detect.sh
- remediation disposition: Modify tests/02-check-test.sh
- remediation disposition: Modify tests/11-check-parity-test.sh
- RED command: rtk bash tests/02-check-test.sh
- observed RED: the retired four-Skill Matt fixture remained missing where the
  test expected detected.
- RED command: rtk bash tests/11-check-parity-test.sh
- observed RED: Bash reported Matt/ECC detected from retired probes while Go
  reported both missing.
- GREEN command: rtk bash tests/02-check-test.sh
- GREEN command: rtk bash tests/11-check-parity-test.sh
- owning gate: rtk bash tests/run.sh
- re-review status: resolved; the direct parity test and complete repository
  shell suite pass with retired probes rejected and active v4 probes detected.

### MEDIUM: Linux release smoke submits retired Workflow and Host authority

- location: scripts/smoke-linux.sh:157
- location: scripts/smoke-linux.sh:160
- location: tests/14-cutover-release-test.sh:270
- evidence: after the Provider parity remediation passed, the mandatory shell
  suite reached Docker Linux smoke. The embedded START request still declares
  `oaw.workflow-command/v1` and `oaw.host-session/v2`, so the hard-cut Workflow
  CLI returns a canonical rejection before Coordinator state initialization.
  The smoke then fails its current Workflow State assertion even though the CLI
  is correctly rejecting stale authority.
- disposition: FIX
- plan amendment: 3cff78d adds both exact release-smoke paths to the Plan 06 File Map
- remediation disposition: Modify scripts/smoke-linux.sh
- remediation disposition: Modify tests/14-cutover-release-test.sh
- RED command: rtk bash tests/14-cutover-release-test.sh
- observed RED: the release, wrapper, and Host-native gates passed, then Docker
  smoke failed because CURRENT Workflow exchange created no Workflow State.
- expected focused RED: tests/14-cutover-release-test.sh rejects Workflow v1 or
  Host Session v2 literals in scripts/smoke-linux.sh before building the Docker
  release fixture.
- observed focused RED: `FAIL: Linux smoke contains retired authority:
  oaw.workflow-command/v1`.
- intermediate GREEN evidence: after migrating the fixture through the same
  production constructors used by the Core/Coordinator conformance test, the
  Coordinator records root was initialized but the smoke exposed a second
  incorrect expectation that a rejected START must persist a Workflow record.
  `internal/coordinator/start_test.go` requires non-Workflow, Host-evidence, and
  invalid-Core rejections to leave no committed Workflow.
- GREEN command: rtk bash tests/14-cutover-release-test.sh
- owning gate: rtk bash tests/run.sh
- re-review status: resolved; the Linux smoke submits canonical Workflow v2,
  Selection v4, and Host Session v3 fields, receives the expected canonical
  rejection, initializes the current Coordinator state root, persists no
  rejected Workflow, preserves the Policy-only file, and launches no model.

### MEDIUM: Provider claim scan conflates unrelated fields in minified JSON

- location: docs/superpowers/plans/2026-08-10-oaw-provider-surface-v4-06-bridge-docs-start.md:564
- evidence: the Task 4 relationship regex scanned single-line generated JSON.
  It therefore matched legitimate `e2e-runner` specialist metadata followed by
  an unrelated `fresh-verification` field and matched `code-reviewer` followed
  much later by an unrelated `closeout` field on the same physical line.
  Structured tests already prove that e2e remains a non-owning specialist and
  that reviewer bindings cannot own closeout.
- disposition: FIX
- plan amendment: dc2dae1 restricts the relationship regex to prose surfaces
  and documents that generated assets remain owned by Step 1 structural tests.
- RED command: rtk rg -n 'e2e-runner.*(broad|fresh).*verification|code-reviewer.*(completion|closeout)|delivery-gate.*(completion|closeout)' internal/assets policy README.md README-zh.md docs/en docs/zh
- observed RED: internal/assets/profile-matrix.json and
  internal/assets/providers/oaw-ecc.json matched unrelated fields on minified
  lines despite correct structured ownership.
- GREEN command: rtk rg -n 'e2e-runner.*(broad|fresh).*verification|code-reviewer.*(completion|closeout)|delivery-gate.*(completion|closeout)' policy README.md README-zh.md docs/en docs/zh
- re-review status: resolved; the prose scan exits 1 with no output, while the
  focused builtin/generated-asset tests continue to validate machine-readable
  Provider responsibilities.

### MEDIUM: Public Bridge integration skipped the real SubagentStart path

- location: internal/integration/codex_bridge_blackbox_test.go:43
- evidence: the previous case constructed complete profile evidence directly,
  while the fixed Plan 06 contract requires real `SubagentStart` Hook recording,
  feature-store observation, and public `observe_current`/`core_inspect`
  eligibility evidence.
- disposition: FIX
- remediation disposition: Modify internal/integration/codex_bridge_blackbox_test.go
- plan amendment: the sixth review-discovered Plan 06 amendment records the
  boundary requirement; the integration path was already authorized.
- RED command: `rtk go test ./internal/integration -run '^TestSPFullCurrentConfirmationRequiresLiveChildDelegation$' -count=1`
- observed RED: exact Hook evidence was written but the MCP Service remained
  `HOST_FEATURE_UNATTESTED` because the test service was not wired to the real
  session feature store.
- GREEN command: the same command after wiring the fixture to the XDG-backed
  store.
- re-review status: resolved; missing, foreign-session, and exact-session
  evidence now cross the real Hook -> store -> public Bridge boundary.

### MEDIUM: Integrity hashing files were outside the locked review map

- location: internal/integrity/binding_root.go:1
- location: internal/integrity/binding_root_test.go:1
- location: internal/integrity/distribution_tree.go:1
- location: internal/integrity/distribution_tree_test.go:1
- evidence: these production integrity implementations were untracked and
  absent from Plans 01-06 and `reviewed-paths.txt`, despite being used by
  discovery and Provider source audit.
- disposition: DOCUMENT
- plan amendment: the sixth review-discovered Plan 06 amendment adds all four
  exact paths with Create dispositions and security responsibilities.
- re-review status: resolved for fixed-point accountability; implementation
  remains subject to the fresh focused integrity, discovery, and source-audit
  verification gates below.

## Code Quality

- spec compliance: resolved after the Task 4 shell suite exposed and the review
  fixed both the legacy Provider parity and Linux release-smoke hard-cut gaps;
  no CRITICAL or HIGH finding is open.
- standards review: approved. Remediation is limited to contract assertions and
  fixtures, uses existing test helpers and Provider probes, and does not add a
  new production abstraction or duplicate detection path.
- complete remediation package gate: `rtk go test ./internal/integrity
  ./internal/discovery ./internal/codexbridge/... ./internal/coordinator
  ./internal/schema ./internal/assets ./internal/config ./internal/check
  -count=1` passed 522 tests in 11 packages.
- full-tree re-review: `rtk go test ./... -count=1` passed 1677 tests, failed
  only the unchanged execution-baseline timing test
  `TestCodexLauncherOutlivesOpeningRequestContext`, and skipped 2 tests. The
  isolated command with `-count=10` passed 10/10. This remains a documented
  baseline flake and is not converted into a passing Task 4 gate.
- reopened shell remediation gate: `rtk bash tests/run.sh` passed every
  installer, documentation, Bash/Go parity, release, Docker Linux,
  Core/Coordinator, and Bridge protocol case.
- focused release GREEN: `rtk bash tests/14-cutover-release-test.sh` passed all
  four wrapper, archive, Host-native, and Docker Linux contracts.
- shell syntax: `rtk bash -n lib/detect.sh scripts/smoke-linux.sh
  tests/02-check-test.sh tests/11-check-parity-test.sh
  tests/14-cutover-release-test.sh` passed.
- ShellCheck reported only the pre-existing SC1007 at
  `tests/14-cutover-release-test.sh:379`; the line is outside the remediation
  diff and remains unchanged.
- `rtk git diff --check 2a4e9fb189e01d5c3f6fe7242d3f65bf500656e7`:
  passed.

## Security Review

- complete-tree hashing: passed inspection and focused tests. Binding roots are
  physical, symlinks and non-regular files are rejected, root identity is
  rechecked, and every regular file is checked before and after hashing.
- path containment and symlink handling: passed through `internal/integrity`
  and `internal/discovery`; no lexical escape or symlink fallback is admitted.
- strict decoding and canonical digests: passed through schema, Coordinator,
  Bridge, and generated-asset tests. Snapshot v2 now closes and requires the
  runtime Classification and Configuration projections.
- v2 handle secrecy, TTL, session, and CWD binding: passed Bridge cache/Hook
  tests. Handles remain opaque in memory, are TTL/LRU bounded, and validate
  version plus constant-time session/CWD digest headers.
- App Server method allowlist: passed; production allows exactly `skills/list`,
  `hooks/list`, and `config/read`, with only `skills/list` required.
- Host-owned authorization, invocation, and gate evidence: passed public MCP
  schema and hydration tests; those authority records are absent from caller
  input and come only from current Host facts or Coordinator state.
- non-START eight-digest preflight: passed; Session, Environment, Inventory,
  Features, Actions, Configuration, Resolution, and Registry digests are
  compared before executable state use or mutation.
- Coordinator journal and state integrity: passed strict decoding, canonical
  revision/digest, permission, and state-transition tests.
- generated assets and public-output redaction: passed; configuration projects
  dispositions only, and no raw private config or evidence handle is emitted.
- credential signature scan over added diff lines: no AWS, GitHub, OpenAI, or
  private-key signature matched.
- generic secret-assignment scan over added diff lines: no API key, access
  token, client secret, or password assignment matched.
- private absolute path scan over added diff lines: no real `/Users`, `/home`,
  or Windows user path matched.
- Hook/Transcript persistence scans: no production path writes ToolInput, raw
  Hook Context, Host evidence handles, or model transcripts to journal, state,
  logs, stdout, or stderr.
- reopened shell diff credential scan: no AWS, GitHub, OpenAI, private-key,
  API-key, token, client-secret, or password signature matched added lines.
- reopened shell diff private-path scan: no real macOS, Linux, or Windows user
  home path matched added lines.
- release fixture safety: the embedded values are canonical non-secret test
  digests generated through production constructors; the smoke still traps
  model commands, preserves the Policy-only file, and asserts that a rejected
  START creates no committed Workflow record.
- result: approved with no open CRITICAL/HIGH security finding.

## Increment Re-review

- fixed increment base: `dc2dae19779796820105c17429fb40e8acd4252d`.
- independent spec review: completed; its two MEDIUM findings are recorded and
  resolved above.
- independent security review: unavailable because the review agent returned
  503 `auth_unavailable`; no failed agent result was counted as review evidence.
- manual security re-review: complete over rooted Binding/Distribution hashing,
  Git object export and isolated Git environment, strict Hook decoding, bounded
  session/CWD feature evidence, reporter identity, PREPARE re-observation,
  Receipt recovery, cancellation, journal validation, public redaction, and
  generated assets.
- credential, secret assignment, and private absolute-path scans: no match.
- private-field persistence scan: only the public `session_id` in a synthetic
  Conformance transcript matched; no real session, raw Hook payload, evidence
  handle, credential, or transcript content is persisted.
- fresh focused, race, vet, full Go, shell, stale-authority, and diff gates all
  pass as recorded in verification.md.
- result: approved with no open CRITICAL, HIGH, MEDIUM, or LOW increment finding.
