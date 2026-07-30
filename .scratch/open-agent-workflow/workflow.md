# Open Agent Workflow Lifecycle Record

```yaml
task: open-agent-workflow-v0.1
classification: complex-domain-heavy-ambiguous-large
profile: MATT-SP-HYBRID
selection_source: user-selected-option-1
bundle: MATT-SP-HYBRID + ECC(security-review)
ecc_addons:
  - security-review
ecc_build_resolver: none
current_stage: workspace-setup
active_ticket: 01-installer-foundation-and-check
spec: .scratch/open-agent-workflow/spec.md
tickets: .scratch/open-agent-workflow/issues/
execution_plans: docs/superpowers/plans/
review_evidence: .scratch/open-agent-workflow/evidence/review.md
verification_evidence: .scratch/open-agent-workflow/evidence/verification.md
tdd_owner: matt
implementation_owner: superpowers
debug_owner: matt
review_owner: superpowers
completion_owner: superpowers
```

## Confirmed Product Decisions

- Product name: Open Agent Workflow (OAW)
- Repository name: `open-agent-workflow`
- License: Apache-2.0
- Provider policy: detect external providers; never vendor or install them
- Core user targets: Claude Code, Codex CLI, Gemini CLI, OpenCode
- Extension project targets: Cursor, Windsurf, Cline, Roo Code, GitHub Copilot
- Default scope: core targets at user scope; all targets available at project scope
- Platform baseline: macOS, Linux, and WSL with Bash 3.2+
- Runtime policy: no required Node.js, Python, or jq dependency
- Configuration ownership: XDG OAW directory, not `~/.agents/rules`
- Drift policy: fail closed; `--force` requires a backup before mutation
- Update source: current local checkout only
- Test seam: black-box execution against isolated HOME and XDG directories
- Remote publication: out of scope until separately approved

## Approved Ticket Graph

Approved by the user on 2026-07-30.

| Ticket | Blocked by |
| --- | --- |
| `01-installer-foundation-and-check` | None |
| `02-claude-user-lifecycle` | 01 |
| `03-core-user-adapters` | 02 |
| `04-project-and-extension-adapters` | 02 |
| `05-drift-backups-and-hardening` | 02, 03, 04 |
| `06-bilingual-docs-and-extension-contract` | 01, 02, 03, 04, 05 |
| `07-security-review-and-release-verification` | 01, 02, 03, 04, 05, 06 |

## Executable Plans

Self-reviewed on 2026-07-30 for specification coverage, placeholders, and
cross-plan interface consistency.

- `docs/superpowers/plans/2026-07-30-open-agent-workflow-01-installer-foundation-and-check.md`
- `docs/superpowers/plans/2026-07-30-open-agent-workflow-02-claude-user-lifecycle.md`
- `docs/superpowers/plans/2026-07-30-open-agent-workflow-03-core-user-adapters.md`
- `docs/superpowers/plans/2026-07-30-open-agent-workflow-04-project-and-extension-adapters.md`
- `docs/superpowers/plans/2026-07-30-open-agent-workflow-05-drift-backups-and-hardening.md`
- `docs/superpowers/plans/2026-07-30-open-agent-workflow-06-bilingual-docs-and-extension-contract.md`
- `docs/superpowers/plans/2026-07-30-open-agent-workflow-07-security-review-and-release-verification.md`
