# OAW Provider Surface v4 Verification

## Final Increment (2026-08-13)

- implementation base before final commit: `ed88a6c8369b4625f93ae0e10724492db58dbe18`
- reviewed product commit: `71bedc9`
- implementation diff digest against the base: `42f38fb5d65c0253d0855b08f3a8bc0248816afad2ecb9002b0fad16a3a1542b`
- `rtk go test ./... -count=1`: exit 0; 1810 passed in 32 packages.
- `rtk go test -race ./internal/profile ./internal/registry ./internal/host ./internal/core ./internal/admission ./internal/coordinator ./internal/codexbridge/... ./internal/cli ./internal/integration -count=1`: exit 0; 838 passed in 12 packages.
- `rtk go vet ./...`: exit 0.
- `rtk bash tests/run.sh`: exit 0; all repository shell cases passed, including Docker Linux smoke and Provider source audit.
- `rtk bash scripts/check-core-coordinator-coverage.sh`: exit 0; aggregate coverage 82.6 percent.
- `rtk bash scripts/check-codex-bridge.sh`: exit 0; aggregate Bridge coverage 81.6 percent and deterministic gate PASS.
- `rtk bash scripts/check-docs.sh`: exit 0.
- `rtk bash tests/10-docs-test.sh`, `rtk bash tests/16-core-coordinator-conformance-test.sh`, and `rtk bash tests/18-codex-bridge-protocol-test.sh`: exit 0; all black-box/documentation contracts passed.
- `rtk bash scripts/check-codex-bridge.sh`: exit 0; aggregate Bridge coverage 81.6 percent and deterministic gate PASS.
- `rtk bash scripts/audit-provider-sources.sh --check internal/assets/audits/provider-sources-v4.json`: exit 0 against all four pinned upstream checkouts.
- TDD remediation covered START/SWITCH Graph selection and classification alias isolation, Recipe content digest recomputation, Host component authority pins, Workflow Snapshot active grant/dispatch invariants, and immutable manifest same-inode rewrite detection.
- Coordinator START live acceptance remains `NOT_SUBMITTED`: current Host child-delegation evidence was unavailable/expired, so no claim of accepted START is made.
- known residual review note: Provider audit Git export buffers full tree/blob objects; current hardcoded pinned-source scope limits exploitability, but explicit export limits remain a follow-up.

The final increment also passed the retained-classification alias regression five consecutive times. No tracked verification-only change is staged; `.serena/` and the local `oaw` binary remain excluded.

implementation_base: 2a4e9fb189e01d5c3f6fe7242d3f65bf500656e7
reviewed_head: 71bedc9
selected_lifecycle: SP-FULL / CURRENT / no Add-on
verification_started_at: 2026-08-11T06:25:32Z
verification_restarted_at: 2026-08-11T07:24:19Z
verification_restarted_again_at: 2026-08-11T07:43:30Z

## Task 4 Full-Tree Gates

Results are recorded from fresh commands against the reviewed HEAD. RTK emits
secret-free aggregate summaries; the pass/fail count or exact diagnostic is the
recorded output projection. No Host handle, credential, absolute Binding path,
raw Hook payload, or model transcript is retained.

### Attempt 1 (invalidated by stable shell regression)

- focused Core/Coordinator tests: 660 passed in 11 packages.
- focused Bridge/CLI/assets tests: 225 passed in 6 packages.
- Core/Coordinator aggregate coverage: 82.6 percent.
- Bridge aggregate coverage: 80.5 percent; deterministic Bridge gate PASS.
- documentation and three black-box gates: PASS.
- race gate: 786 passed in 12 packages.
- `rtk go vet ./...`: exit 0.
- first full Go run: 1677 passed, 1 execution-baseline App Server timing
  failure, 2 skipped; isolated reproduction passed 20/20.
- second exact full Go run: 1678 passed in 32 packages.
- shell suite: FAILED in `tests/02-check-test.sh`; the retired Matt four-Skill
  fixture was still expected as active. `tests/11-check-parity-test.sh`
  independently FAILED because Bash accepted retired Matt/ECC probes while Go
  rejected them.
- resumed shell evidence after that remediation: Provider check and parity
  cases passed, then Docker Linux smoke FAILED because its embedded START still
  used Workflow Command v1 and Host Session v2 and therefore created no current
  Workflow State.
- disposition: returned to Task 3. This attempt is not final acceptance
  evidence and Task 4 will restart from Step 1 after reviewed remediation.

### Attempt 2 (fresh gate against reviewed HEAD acbfd65)

#### Step 1: focused Core, Coordinator, Bridge, and documentation gates

- `rtk go test ./internal/catalog ./internal/profile ./internal/registry
  ./internal/discovery ./internal/host ./internal/core ./internal/admission
  ./internal/coordinator ./internal/schema ./internal/builtin
  ./internal/integration -count=1`: exit 0; 660 passed in 11 packages;
  output SHA-256 `e08b0d6b13a91a3f4e9eff41243dd9a14f44998369ef008a03d41788a776724e`.
- `rtk go test ./internal/codexbridge ./internal/codexbridge/appserver
  ./internal/codexbridge/hook ./internal/cli ./internal/assets
  ./internal/assets/generate -count=1`: exit 0; 225 passed in 6 packages;
  output SHA-256 `e73b75cfd8eab76557e1328f3bd3775e85212f28624fc98b0e6a1d0d6eab7f09`.
- `rtk bash scripts/check-core-coordinator-coverage.sh`: exit 0; aggregate
  coverage 82.6 percent; output SHA-256
  `d4dbd454bc019b1eab1bbc915bab8c247bfd3834567182d2bb499d2e91d39e63`.
- `rtk bash scripts/check-codex-bridge.sh`: exit 0; aggregate coverage
  80.5 percent and deterministic Bridge gate PASS; output SHA-256
  `b3ca3cdf3a05904daa186dbdc10e563aabe374fa8b839c64f0679b21de7d7ee6`.
- `rtk bash scripts/check-docs.sh`: exit 0; output SHA-256
  `94571b35043b23560245f9e246b76287dba028c49013e35e87da6a29d028374d`.
- `rtk bash tests/10-docs-test.sh`: exit 0; output SHA-256
  `409626973f92b3467014b41ce1ee035f7007380b7e320e04c7b3297763ed16b6`.
- `rtk bash tests/16-core-coordinator-conformance-test.sh`: exit 0; output
  SHA-256 `fb9915644cd34359cf2b7e0ea466c88145e9480f0f02bdfbf38bab12b9732c4d`.
- `rtk bash tests/18-codex-bridge-protocol-test.sh`: exit 0; output SHA-256
  `84e78f21e5682ffbbe91da3f10befa73df2cd73a9915a839a7674a73a96ecf34`.

#### Step 2: race detection

- `rtk go test -race ./internal/profile ./internal/registry ./internal/host
  ./internal/core ./internal/admission ./internal/coordinator
  ./internal/codexbridge/... ./internal/cli ./internal/integration -count=1`:
  exit 0; 786 passed in 12 packages; output SHA-256
  `63150f0d286837176d3f681378130ff1027b8a6702e39253fdf9e05548169838`.

#### Step 3: full vet, Go, and repository shell suite

- `rtk go vet ./...`: exit 0; output SHA-256
  `01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b`.
- first `rtk go test ./... -count=1`: exit 1; 1677 passed, the known
  `TestCodexLauncherOutlivesOpeningRequestContext` App Server timing baseline
  failed with `context deadline exceeded`, and 2 skipped; output SHA-256
  `52c62d8157d5cae0eeb7b15022fde46fec8a680ea038847074bd6529d29c6774`.
- `rtk go test ./internal/codexbridge/appserver -run
  TestCodexLauncherOutlivesOpeningRequestContext -count=20`: exit 0; 20 passed;
  output SHA-256
  `8f509d0048cc89bb53d750eb33da94351dc1b63ebf8e9351f7912259d71ae86c`.
- exact full `rtk go test ./... -count=1` rerun: exit 0; 1678 passed in
  32 packages; output SHA-256
  `3529e446905bb8ef270fee20da04eec71222780bbddef2d14d2a75486baf1500`.
- `rtk bash tests/run.sh`: exit 0; all installer, documentation, parity,
  release, Docker Linux, Core/Coordinator, Bridge management, and protocol
  cases passed; output SHA-256
  `3dbea9ae1a0179c8509a1c59f0d1713c51ba218e688a303bfab1cd9d40077d91`.

#### Attempt 2 disposition

- the production stale-authority scan and Matt fictional-ID scan were empty as
  required, but the prose relationship scan also included minified generated
  JSON and produced stable false positives by joining unrelated fields on one
  physical line.
- disposition: Plan 06 gate corrected in dc2dae1. Attempt 2 is retained as
  evidence but is not final acceptance; Task 4 restarts from Step 1 against the
  amended reviewed HEAD.

### Attempt 3 (fresh gate against reviewed HEAD dc2dae1)

#### Step 1: focused gates

- focused Core/Coordinator: exit 0; 660 passed in 11 packages; output SHA-256
  `e08b0d6b13a91a3f4e9eff41243dd9a14f44998369ef008a03d41788a776724e`.
- focused Bridge/CLI/assets: exit 0; 225 passed in 6 packages; output
  SHA-256 `e73b75cfd8eab76557e1328f3bd3775e85212f28624fc98b0e6a1d0d6eab7f09`.
- Core/Coordinator coverage: exit 0; 82.6 percent; output SHA-256
  `d4dbd454bc019b1eab1bbc915bab8c247bfd3834567182d2bb499d2e91d39e63`.
- Bridge deterministic gate: exit 0; 80.5 percent; output SHA-256
  `b3ca3cdf3a05904daa186dbdc10e563aabe374fa8b839c64f0679b21de7d7ee6`.
- documentation gate: exit 0; output SHA-256
  `94571b35043b23560245f9e246b76287dba028c49013e35e87da6a29d028374d`.
- documentation black box: exit 0; output SHA-256
  `409626973f92b3467014b41ce1ee035f7007380b7e320e04c7b3297763ed16b6`.
- Core/Coordinator conformance: exit 0; output SHA-256
  `fb9915644cd34359cf2b7e0ea466c88145e9480f0f02bdfbf38bab12b9732c4d`.
- Bridge protocol black box: exit 0; output SHA-256
  `84e78f21e5682ffbbe91da3f10befa73df2cd73a9915a839a7674a73a96ecf34`.

#### Step 2: race detection

- exact Task 4 race command: exit 0; 786 passed in 12 packages; output
  SHA-256 `63150f0d286837176d3f681378130ff1027b8a6702e39253fdf9e05548169838`.

#### Step 3: full vet, Go, and repository shell suite

- `rtk go vet ./...`: exit 0; output SHA-256
  `01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b`.
- first full Go run: exit 1; 1677 passed, the unchanged App Server timing
  baseline failed once, and 2 skipped; output SHA-256
  `7162a2b1ca6184da2783b16361d0f2da29eb0ba544b52f3eccbaf39e805c277a`.
- isolated App Server timing test: exit 0; 20 passed; output SHA-256
  `8f509d0048cc89bb53d750eb33da94351dc1b63ebf8e9351f7912259d71ae86c`.
- exact full Go rerun: exit 0; 1678 passed in 32 packages; output SHA-256
  `3529e446905bb8ef270fee20da04eec71222780bbddef2d14d2a75486baf1500`.
- repository shell suite: exit 0; every implemented case passed; output
  SHA-256 `3dbea9ae1a0179c8509a1c59f0d1713c51ba218e688a303bfab1cd9d40077d91`.

#### Step 4: stale authority and Provider claims

- production stale-authority scan: exit 1 with no output; output SHA-256
  `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
- Matt fictional Binding-ID scan: exit 1 with no output; output SHA-256
  `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
- prose Provider-ownership scan: exit 1 with no output; output SHA-256
  `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
- machine-readable Provider ownership remains covered by the passing focused
  `internal/builtin` and generated-asset tests from Step 1.

#### Step 5: repository evidence boundary

- `rtk git diff --cached --quiet`: exit 0.
- `rtk git diff --check`: exit 0.
- `rtk git status --short --branch`: only the declared untracked
  `.scratch/oaw-provider-surface-v4/` evidence and pre-existing `.serena/`
  directory remain; no tracked or staged verification change exists.
- final reviewed HEAD: `dc2dae19779796820105c17429fb40e8acd4252d`.
- verification completed at `2026-08-11T07:57:26Z`.

### Attempt 4 (fresh gate against unstaged increment after dc2dae1)

- verification window: 2026-08-12; implementation base remains
  `2a4e9fb189e01d5c3f6fe7242d3f65bf500656e7`; fixed increment base is
  `dc2dae19779796820105c17429fb40e8acd4252d`.
- increment diff digest after verification:
  `5fd392dcdb01aee1a06a320ccbac01fe00396a4797a83779782b7879d16262fd`.
- focused Bridge boundary RED/GREEN: the exact integration test first failed
  with `HOST_FEATURE_UNATTESTED` after real Hook recording, then passed after
  wiring the MCP fixture to the same XDG-backed feature store.
- focused Core/Coordinator command: exit 0; 674 passed in 11 packages.
- focused Bridge/CLI/assets command: exit 0; 257 passed in 6 packages.
- Core/Coordinator coverage command: exit 0; aggregate 82.5 percent.
- Bridge deterministic/coverage command: exit 0; aggregate 81.4 percent.
- documentation and three black-box commands: exit 0; all reported PASS.
- exact Task 4 race command: exit 0; no race diagnostic.
- `rtk go vet ./...`: exit 0.
- `rtk go test ./... -count=1`: exit 0; the documented App Server timing
  baseline did not recur.
- `rtk bash tests/run.sh`: exit 0; all repository shell cases passed.
- stale production authority, Matt fictional Binding ID, and prose Provider
  claim scans: exit 1 with no output, as required.
- credential signature, secret assignment, and private absolute-path scans over
  added lines: exit 1 with no output.
- Hook/private-field persistence scan produced one synthetic generated
  Conformance JSON line because it contains the public `session_id` field; it
  contains no real session, private Hook payload, Host evidence handle, or
  persistence code and is not a leakage finding.
- `rtk git diff --check dc2dae1`: exit 0; `rtk git diff --cached --check`:
  exit 0; the index is empty.
- independent spec review found two MEDIUM gaps; both were remediated and
  reverified. The independent security review agent returned 503
  `auth_unavailable`; the complete manual security review and scans found no
  open CRITICAL, HIGH, MEDIUM, or LOW finding.
- verification completed at `2026-08-12T07:16:46Z`.
