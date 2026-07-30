# OAW Ticket 06 Bilingual Documentation and Extension Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish complete English and Chinese documentation for OAW's rationale, lifecycle, installer, adapters, security model, and contribution contract.

**Architecture:** Root READMEs provide equivalent product entrypoints; paired `docs/en` and `docs/zh` trees hold durable detail. A local documentation check enforces required pairs, relative links, current CLI examples, adapter support labels, and comparison caveats.

**Tech Stack:** Markdown, Apache-2.0 text, dependency-free shell documentation checks.

**Canonical sources:** `.scratch/open-agent-workflow/spec.md`, `.scratch/open-agent-workflow/issues/06-bilingual-docs-and-extension-contract.md`, `CONTEXT.md`, ADRs, implemented CLI behavior, official adapter research.

---

### Task 1: Add licensing, governance, and automated document contracts

**Files:**
- Create: `LICENSE`
- Create: `CONTRIBUTING.md`
- Create: `CONTRIBUTING-zh.md`
- Create: `SECURITY.md`
- Create: `SECURITY-zh.md`
- Create: `CODE_OF_CONDUCT.md`
- Create: `CHANGELOG.md`
- Create: `scripts/check-docs.sh`
- Create: `tests/07-docs-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Write failing required-document tests**

Assert Apache License 2.0 identifies year `2026` and copyright holder `Open Agent Workflow contributors`; both contribution guides name the black-box installer test seam and adapter evidence contract; both security guides define private reporting without inventing a live email address; the code of conduct uses Contributor Covenant 2.1 with a neutral enforcement contact instruction; and `CHANGELOG.md` contains an unreleased `0.1.0` entry without claiming publication.

- [ ] **Step 2: Verify RED**

Run: `bash tests/07-docs-test.sh`

Expected: FAIL listing the missing governance files.

- [ ] **Step 3: Add exact governance content**

Use the unmodified Apache-2.0 license body. Contribution documents must require an issue-sized vertical change, Bash 3.2 compatibility, no provider vendoring, black-box CLI tests, English/Chinese parity for user-visible behavior, official adapter sources with retrieval dates, and no remote publication from installer code. Security documents must describe supported versions, report contents, response expectations without a guaranteed SLA, and the installer trust boundary. The changelog records the local candidate's policy, adapters, installer lifecycle, safety properties, and documentation as unreleased work.

- [ ] **Step 4: Implement the documentation checker**

`scripts/check-docs.sh` verifies an explicit paired-file list, rejects absolute local Markdown links and missing relative link targets, checks that every public CLI command appears in both READMEs, and requires the phrase `experience-based` plus its Chinese equivalent `基于经验` in comparison documents. It performs no network requests.

- [ ] **Step 5: Verify governance contracts**

Run: `bash tests/07-docs-test.sh`

Expected: governance assertions pass; README and detailed-doc assertions remain RED.

- [ ] **Step 6: Commit licensing and governance**

```bash
git add LICENSE CONTRIBUTING.md CONTRIBUTING-zh.md SECURITY.md SECURITY-zh.md CODE_OF_CONDUCT.md CHANGELOG.md scripts/check-docs.sh tests/07-docs-test.sh tests/run.sh
git commit -m "docs: add open source project governance"
```

### Task 2: Write equivalent English and Chinese entrypoints

**Files:**
- Create: `README.md`
- Create: `README-zh.md`
- Modify: `tests/07-docs-test.sh`

- [ ] **Step 1: Add failing README structure assertions**

Require both READMEs to contain language navigation and sections for background, problems, capabilities, quick start, task gate, profiles, hybrid stages, supported targets, safety, update/uninstall, provider prerequisites, documentation, contributing, license, and local-only v0.1 status.

- [ ] **Step 2: Verify RED**

Run: `bash tests/07-docs-test.sh`

Expected: FAIL because root READMEs are absent.

- [ ] **Step 3: Write the English README**

Lead with `Open Agent Workflow` and the statement that OAW arbitrates independently installed workflow providers across agent tools. Show exact local commands:

```bash
./install.sh check
./install.sh install
./install.sh install --project /path/to/repository
./install.sh update --dry-run
./install.sh uninstall
```

State that no profile is silently selected, OAW does not install Superpowers/Matt/ECC, updates use the current checkout, drift fails closed, and `--force` backs up first. Include the six-stage comparison summary and mark scores as experience-based design input. State that machine-readable status is a reserved post-v0.1 extension and that v0.1 output is human-readable only.

- [ ] **Step 4: Write the Chinese README with equivalent meaning**

Use `Open Agent Workflow（OAW）` as the product signal. Preserve command and profile IDs verbatim, translate every safety caveat and support level, and link each English detail document to its Chinese counterpart.

- [ ] **Step 5: Verify README parity**

Run: `bash tests/07-docs-test.sh`

Expected: both entrypoint structures, CLI examples, caveats, and cross-language links pass.

- [ ] **Step 6: Commit bilingual entrypoints**

```bash
git add README.md README-zh.md tests/07-docs-test.sh
git commit -m "docs: explain Open Agent Workflow in two languages"
```

### Task 3: Document background, comparison, and lifecycle semantics

**Files:**
- Create: `docs/en/background.md`
- Create: `docs/zh/background.md`
- Create: `docs/en/comparison.md`
- Create: `docs/zh/comparison.md`
- Create: `docs/en/lifecycle.md`
- Create: `docs/zh/lifecycle.md`
- Modify: `tests/07-docs-test.sh`

- [ ] **Step 1: Add failing semantic coverage assertions**

Require the background pair to cover overlapping automatic triggers, cross-client drift, and provider independence. Require the comparison pair to contain all six numeric rows and the corrected ownership: planning Matt for complex work, implementation Superpowers, TDD Matt, debugging Matt, review Superpowers, completion Superpowers. Require lifecycle docs to define ordinary/complex classification, all five profiles, blocking user choice, lifecycle lock, bundle inheritance, bounded add-ons, and stable switching.

- [ ] **Step 2: Verify RED**

Run: `bash tests/07-docs-test.sh`

Expected: FAIL listing absent detailed documents.

- [ ] **Step 3: Write the background and comparison pairs**

Explain scoring criteria as procedure completeness, correctness discipline, ambiguity handling, review closure, verification strength, and operational overhead. Publish the approved table values: planning `4.8/5.0/3.8`, implementation `5.0/4.2/3.7`, TDD `4.8/4.9/4.1`, debugging `4.7/5.0/2.8`, review `5.0/4.8/4.4`, completion `5.0/3.6/4.0`. Label them experience-based, version-sensitive, non-benchmark judgments.

- [ ] **Step 4: Write the lifecycle pair**

Mirror the canonical policy in explanatory prose without creating a second normative policy. Include one complete `MATT-SP-HYBRID + ECC(security-review)` example showing classification, choice, locked bundle, ticket inheritance, and a stable-boundary switch.

- [ ] **Step 5: Verify semantic coverage**

Run: `bash tests/07-docs-test.sh`

Expected: all background, score, correction, and lifecycle assertions pass.

- [ ] **Step 6: Commit workflow rationale**

```bash
git add docs/en/background.md docs/zh/background.md docs/en/comparison.md docs/zh/comparison.md docs/en/lifecycle.md docs/zh/lifecycle.md tests/07-docs-test.sh
git commit -m "docs: document workflow arbitration model"
```

### Task 4: Document architecture, installer, and adapter evidence

**Files:**
- Create: `docs/en/architecture.md`
- Create: `docs/zh/architecture.md`
- Create: `docs/en/installer.md`
- Create: `docs/zh/installer.md`
- Create: `docs/en/adapters.md`
- Create: `docs/zh/adapters.md`
- Modify: `tests/07-docs-test.sh`

- [ ] **Step 1: Add failing implementation-parity checks**

Assert architecture docs name canonical config/state paths, pure renderers, prepare/apply phases, block-vs-file ownership, state schema, backups, and marker non-semantics. Installer docs must list every command/flag/exit behavior. Adapter docs must list all nine exact user/project paths, user/project support levels, import/reference behavior, precedence/reload caveats, primary URLs, and retrieval date `2026-07-30`.

- [ ] **Step 2: Verify RED**

Run: `bash tests/07-docs-test.sh`

Expected: FAIL because architecture, installer, and adapter pairs are absent.

- [ ] **Step 3: Write architecture and installer pairs from implementation**

Include a text data flow from checkout policy through renderer, preflight, backup, atomic apply, state, and target. Document default target sets, target CSV normalization, isolated XDG state, local-checkout updates, dry-run guarantees, drift exit behavior, and exact uninstall ownership rules.

- [ ] **Step 4: Write the sourced adapter matrix**

For Claude, Codex, Gemini, OpenCode, Cursor, Windsurf/Devin, Cline, Roo Code, and Copilot, distinguish documented behavior from OAW's mechanical merge choices. Explicitly note Claude HTML comments are stripped from injected context, Codex/OpenCode lack documented Markdown imports, Cursor requires `.mdc`, Windsurf prefers `.devin/rules`, and nested Copilot AGENTS behavior is experimental and therefore unused.

- [ ] **Step 5: Verify implementation/documentation parity**

Run: `bash tests/07-docs-test.sh`

Expected: all paths, commands, support labels, URLs, dates, and caveats pass.

- [ ] **Step 6: Commit operating documentation**

```bash
git add docs/en/architecture.md docs/zh/architecture.md docs/en/installer.md docs/zh/installer.md docs/en/adapters.md docs/zh/adapters.md tests/07-docs-test.sh
git commit -m "docs: describe installer and adapter architecture"
```

### Task 5: Publish extension, security, and troubleshooting guides

**Files:**
- Create: `docs/en/extending-adapters.md`
- Create: `docs/zh/extending-adapters.md`
- Create: `docs/en/security.md`
- Create: `docs/zh/security.md`
- Create: `docs/en/troubleshooting.md`
- Create: `docs/zh/troubleshooting.md`
- Modify: `tests/07-docs-test.sh`

- [ ] **Step 1: Add failing extension and recovery checks**

Require extension docs to define metadata, target IDs, scopes, path ownership, render purity, shared-destination collisions, fixtures, official evidence, security cases, and graduation support levels. Require security/troubleshooting docs to explain threat boundaries, symlink rejection, inert state, preflight, force backups, drift diagnosis, recovery, missing providers, stale agent sessions, and uninstall refusal.

- [ ] **Step 2: Verify RED**

Run: `bash tests/07-docs-test.sh`

Expected: FAIL listing the six absent guides.

- [ ] **Step 3: Write the extension contract pair**

Specify that a new adapter adds registry metadata, a pure renderer, destination ownership, exact black-box fixtures, official source URLs and dates, scope support, shared-path compatibility, and adversarial containment tests. It cannot change lifecycle semantics or vendor a provider.

- [ ] **Step 4: Write security and troubleshooting pairs**

Provide exact diagnostic sequences beginning with `./install.sh check`, then `update --dry-run`, backup inspection, and an explicitly scoped forced command. Explain that users restore backups manually by following `manifest.tsv`, and that restarting the target agent is required when its documented loader lacks hot reload.

- [ ] **Step 5: Run Ticket 06 verification**

Run: `bash scripts/check-docs.sh && bash tests/07-docs-test.sh && bash tests/run.sh`

Expected: all required bilingual pairs and local links pass; the full installer suite remains green; no documentation checker accesses the network.

- [ ] **Step 6: Commit extension and operations guides**

```bash
git add docs/en/extending-adapters.md docs/zh/extending-adapters.md docs/en/security.md docs/zh/security.md docs/en/troubleshooting.md docs/zh/troubleshooting.md tests/07-docs-test.sh
git commit -m "docs: publish adapter and recovery guides"
```
