# MATT-SP-HYBRID Selection Evidence

## Isolated run

- Temporary project: `/tmp/oaw-matt-sp-hybrid.VIPAA4/project`
- Isolated `HOME`, `XDG_CONFIG_HOME`, and `XDG_STATE_HOME`: under
  `/tmp/oaw-matt-sp-hybrid.VIPAA4/`
- Installation: project scope, Codex target only
- Assurance: `policy-cooperative`; no Assurance or Bridge component was
  installed
- Workspace: the requested fresh project was itself the isolated Git checkout;
  no nested worktree or shared project state was used

The activated request was:

```text
Use OAW with MATT-SP-HYBRID to deliver a maintenance-window conflict check.
Keep the work in this fresh project and prove Matt owns the domain edges while
Superpowers owns executable planning and implementation.
```

## Responsibility ownership

| Responsibility | Owner and observed result |
| --- | --- |
| Problem framing | Matt `grill-with-docs` and `domain-modeling`; `CONTEXT.md` fixed Maintenance Window, Boundary Touch, Overlap, and Maintenance Plan. |
| Specification | Matt `to-spec`; `SPEC.md` records user stories, contract, boundaries, tests, and non-goals. |
| Delivery edges | Matt `to-tickets`; `tickets.md` has two tracer-bullet slices and one explicit blocker. |
| Executable planning | Superpowers `writing-plans`; `plan.md` adds paths, commands, and expected results without changing Matt edges. |
| Workspace | Superpowers `using-git-worktrees`; the fresh `/tmp` project already supplied isolation, so no nested worktree was created. |
| Implementation | Superpowers `executing-plans` inline; commits follow the approved plan. |
| TDD | Matt `tdd` only; RED/GREEN evidence is recorded separately. Superpowers TDD was not used as a second owner. |
| Review | Superpowers review checklist and receiving rules; Host-native review because physical reviewer-agent dispatch was unavailable. |
| Verification | Superpowers `verification-before-completion`; fresh commands ran after removing `oaw`. |
| Closeout | Superpowers `finishing-a-development-branch` semantics applied to the isolated artifact; no public merge was requested. |

The readable Skills were selected by their declared Responsibility and
semantics. No Provider, revision, physical cache path, route contract, or
machine evidence participated in Profile selection.

The Matt `grill-with-docs` entry is disabled for automatic invocation and its
interactive `grilling` helper requires a user-at-a-time interview. The Issue,
CONTEXT, and ADRs supplied a complete bounded intent with no unresolved product
decision, so the domain model was synthesized without inventing a question or
claiming an interactive gesture.

## Missing machine metadata did not block

In the intentionally empty isolated environment, `oaw check` reported
`provider superpowers: missing`, `provider matt: missing`, and `provider ecc:
missing`, while the project Codex installation remained clean. The model still
loaded the readable Hybrid Profile and completed the mixed ownership path.
