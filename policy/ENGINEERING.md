# Open Agent Workflow Policy

## Purpose

Open Agent Workflow (OAW) prevents independently installed engineering
methodologies from starting competing planning, implementation, testing,
debugging, review, or completion lifecycles. It does not install or own those
providers. This policy governs selection and ownership only.

## Mandatory Startup Gate

For every new top-level engineering task that may use workflow skills:

1. Read this policy before invoking a family-specific lifecycle skill.
2. Perform only enough read-only inspection to classify the task.
3. State the classification and concrete evidence.
4. Present every lifecycle profile and all proposed specialist add-ons.
5. Wait for the user's explicit choice. There is no timeout or silent default.
6. Record the selected bundle and use it for the entire deliverable.

Before selection, do not start discovery, design, planning, implementation,
TDD, debugging, delegation, Git work, review, or completion. Pure explanation,
status reporting, read-only lookup, and direct non-workflow commands do not run
this gate.

## Classification

An ordinary task is one coherent deliverable with mostly known requirements,
bounded dependencies, no architectural decision, and one implementation plan.

A task is complex, domain-heavy, ambiguous, or large when requirements remain
unresolved; domain rules require discovery; several subsystems, migrations, or
architectural decisions are involved; multiple delivery tickets are needed;
or security, data, operational, or blast-radius risk is high. When uncertain,
classify as complex and explain why.

## Lifecycle Profiles

Always show all profiles. Mark a recommendation, never a default.

| Profile | Ownership |
| --- | --- |
| `SP-FULL` | Superpowers owns the complete lifecycle. |
| `MATT-FULL` | Matt owns the complete lifecycle. |
| `ECC-FULL` | ECC owns the complete lifecycle. |
| `MATT-SP-HYBRID` | Stage ownership follows the explicit map below. |
| `CUSTOM-LOCKED` | The user supplies a complete, conflict-free stage map. |

Recommend `SP-FULL` for an ordinary feature or refactor,
`MATT-SP-HYBRID` for complex or ambiguous work, `MATT-FULL` for a hard
standalone investigation, and `ECC-FULL` for ECC-dominant security,
evaluation, framework, or build work. The user's choice always wins.

For `CUSTOM-LOCKED`, validate that every applicable responsibility has exactly
one owner, transitions are explicit, and add-ons are bounded. Reject an
ambiguous map rather than guessing.

## Lifecycle Lock

Record the task identity, classification, selected profile, selection source,
stage owners, exact add-ons, active stage, active ticket, and canonical
artifact references. A dispatched agent inherits the exact bundle and must not
run family arbitration again.

The lock persists across follow-ups, context compaction, and delegated work on
the same deliverable. Only the user can change it. A new unrelated engineering
task clears the lock and runs the startup gate again.

If a required provider capability is missing, stop, name it, and ask the user
to install it or choose another profile. Never silently omit or replace a
required stage.

## Full-Family Profiles

Under `SP-FULL`, Superpowers owns discovery, design, planning, workspace,
implementation, TDD, debugging, delegation, review, remediation, verification,
and branch completion. Matt and ECC lifecycle owners remain paused.

Under `MATT-FULL`, Matt owns discovery, domain decisions, specification,
tickets, implementation, TDD, debugging, review, delegation, commits, and
completion evidence. Superpowers and ECC lifecycle owners remain paused.

Under `ECC-FULL`, ECC owns planning, implementation, testing, debugging and
build repair, review, delegation, verification, and completion. Superpowers
and Matt lifecycle owners remain paused.

## Matt-Superpowers Hybrid

`MATT-SP-HYBRID` assigns exactly one owner to each responsibility:

| Responsibility | Owner |
| --- | --- |
| Requirements and domain modeling | Matt |
| Product specification and acceptance criteria | Matt |
| Test-seam selection and ticket decomposition | Matt |
| Per-ticket executable implementation plan | Superpowers `writing-plans` |
| Workspace and Git setup | Superpowers |
| Implementation orchestration and code changes | One Superpowers executor |
| TDD method and red-green loop | Matt `tdd` |
| Functional and hard-bug debugging | Matt `diagnosing-bugs` |
| Build, dependency, and type repair | Selected ECC resolver, or none |
| Spec compliance and code-quality review | Superpowers |
| Review remediation and re-review | Superpowers |
| Fresh verification and branch completion | Superpowers |
| Specialist checks | Only explicitly named bounded add-ons |

Matt's specification and tickets are canonical for requirements and delivery
edges. Superpowers plans may add exact paths, commands, code steps, and
expected results, but may not change requirements or ticket boundaries. If a
plan exposes a requirement gap, repair the Matt source first.

Use Matt `tdd` as the only TDD procedure in this hybrid; pause Superpowers TDD.
Use one Superpowers implementation executor and do not invoke Matt
implementation. Keep Superpowers review and completion active; pause Matt and
ECC general review.

An expected RED test is not a debugging incident. For an unexpected failure,
record the intended state, command, and output, pause implementation, and
transfer that evidence to Matt debugging. A strictly build, dependency, or
type failure may go only to the selected ECC resolver.

## Bounded Add-ons

An add-on from a non-owning provider is authorized only for its named
deliverable. It cannot take over discovery, planning, implementation, TDD,
debugging, general review, delegation, Git, or completion. End it when the
deliverable is complete and return its evidence to the lifecycle owner.

Outcome constraints such as security, coverage, style, or required checks do
not select a workflow. The active owner satisfies them using its own procedure.

## Artifacts

Use the project's existing documentation layout. For a local hybrid tracker:

```text
CONTEXT.md
.scratch/<feature>/workflow.md
.scratch/<feature>/spec.md
.scratch/<feature>/issues/<NN>-<slug>.md
.scratch/<feature>/evidence/review.md
.scratch/<feature>/evidence/verification.md
docs/superpowers/plans/YYYY-MM-DD-<feature>-<NN>-<ticket>.md
```

Do not create duplicate Superpowers design specs or Matt implementation plans.

## Stable Switching

Only the user may switch a locked profile. Switch at an approved specification,
between tickets, after a completed TDD or debugging cycle, after review, or
after recorded verification. Do not switch during delegated work, an
unresolved merge, or an incomplete red-green cycle. Preserve valid artifacts.

## Neutral Safety Rules

- Observe fresh verification output before claiming completion.
- Preserve unrelated user changes.
- Do not perform destructive Git or filesystem operations without approval.
- Diagnose root causes before bug fixes.
- Treat network tools as read-only unless the user authorizes mutation.
- Detection reports capabilities; it never selects a lifecycle profile.
