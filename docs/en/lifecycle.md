# Lifecycle Selection and Locking

[简体中文](../zh/lifecycle.md) | [README](../../README.md)

This guide explains Open Agent Workflow (OAW) lifecycle behavior. It is not a
second policy. The path policy/ENGINEERING.md is normative; read the
[canonical policy](../../policy/ENGINEERING.md) there. If explanatory prose
here differs from that file, the policy wins.

## When the Gate Runs

The startup gate runs before every new top-level engineering task that may use
workflow skills. It does not run for pure explanation, status reporting,
read-only lookup, or a direct command that does not start an engineering
lifecycle.

Before selection, OAW permits only enough read-only inspection to classify the
task. It does not start discovery, design, planning, implementation, TDD,
debugging, delegation, Git work, review, or completion. This creates a blocking user choice
before any family-specific owner can claim the deliverable.

The gate follows this sequence:

1. Read the canonical policy.
2. Inspect only enough context to classify the task.
3. State the classification and concrete evidence.
4. Show all five profiles, mark a recommendation, and list exact proposed
   specialist add-ons.
5. Wait for explicit user selection. There is no timeout or silent default.
6. Record and lock the selected bundle before lifecycle work begins.

Provider detection is diagnostic input. It can show that a required family is
missing, but it cannot choose a profile. If a selected profile requires a
missing capability, work stops until the user installs it or chooses another
profile.

## Ordinary and Complex Classification

An **ordinary task** is one coherent deliverable with mostly known
requirements, bounded dependencies, no architectural decision, and one
implementation plan. An ordinary feature or refactor commonly receives an
`SP-FULL` recommendation, but the user may choose any valid profile.

A **complex task** is domain-heavy, ambiguous, large, or risky. Indicators
include unresolved requirements, domain discovery, several subsystems,
migrations, architectural decisions, multiple delivery tickets, or elevated
security, data, operational, and blast-radius risk. When evidence is uncertain,
OAW classifies the task as complex and explains why. Complex work commonly
receives a `MATT-SP-HYBRID` recommendation.

Classification controls the recommendation and planning depth. It never acts
as automatic selection.

## Lifecycle Profiles

Every gate presents all profiles:

| Profile | Ownership contract |
| --- | --- |
| `SP-FULL` | Superpowers owns discovery through branch completion. Matt and ECC lifecycle owners remain paused. |
| `MATT-FULL` | Matt owns domain decisions, specification, tickets, implementation, TDD, debugging, review, commits, and completion evidence. Superpowers and ECC lifecycle owners remain paused. |
| `ECC-FULL` | ECC owns planning, implementation, testing, build repair, review, delegation, verification, and completion. Superpowers and Matt lifecycle owners remain paused. |
| `MATT-SP-HYBRID` | Matt and Superpowers follow the fixed stage map below. Exact ECC specialists may be selected only as bounded add-ons. |
| `CUSTOM-LOCKED` | The user supplies a complete map with one owner for every applicable responsibility, explicit transitions, and bounded add-ons. |

An ambiguous `CUSTOM-LOCKED` map is rejected rather than repaired by guessing.
A recommendation is always labeled as a recommendation and never hidden as a
default.

## Matt-Superpowers Stage Map

`MATT-SP-HYBRID` assigns one owner to each responsibility:

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
edges. A Superpowers executable plan may add paths, commands, and expected
results, but it may not change those requirements or ticket boundaries.

An expected RED test remains part of TDD. An unexpected functional failure
transfers to Matt debugging with the intended state, command, and output. A
strict build, dependency, or type failure goes only to the selected ECC
resolver.

## Lifecycle Lock and Bundle Inheritance

The **lifecycle lock** records the task identity, classification, selected
profile, selection source, stage owners, exact add-ons, active stage, active
ticket, and canonical artifact references. It persists for the entire
deliverable across follow-ups, context compaction, and delegated work.

**Bundle inheritance** means every dispatched agent receives the exact profile,
stage map, and add-ons. This bundle inheritance prevents the agent from reopening
family arbitration, adding a second lifecycle owner, or replacing an unavailable
capability. For multi-ticket
work, **ticket inheritance** applies the same locked bundle unless the user
changes it at an allowed boundary.

## Bounded Add-ons

A bounded add-on is an exact specialist capability selected for one declared
deliverable. For example, `ECC(security-review)` may produce a security report,
but it does not own implementation, general review, Git work, or completion
under `MATT-SP-HYBRID`. Once its report is delivered, control returns to the
recorded stage owner.

Outcome constraints such as security, coverage, style, or required checks are
not add-ons by themselves and do not select a workflow. The active owner remains
responsible for satisfying them.

## Stable Switching

Only the user can change a lifecycle lock. The stable switching rule allows a change at an
approved specification, between completed tickets, after a completed TDD or
debugging cycle, after review, or after recorded verification. Switching is not
allowed during delegated work, an unresolved merge, or an incomplete red-green
cycle.

A switch preserves valid artifacts and records the new selection. It does not
retroactively rewrite completed ownership or silently substitute a provider.

## Complete Locked-Bundle Example

Assume a repository needs a multi-ticket installer with path containment,
recoverable force, bilingual documentation, and a final security assessment.

### 1. Classification

OAW classifies this as a **complex task** because requirements span several
subsystems and tickets, filesystem mutation has security impact, and review
must close across multiple stages. It recommends the hybrid and proposes one
bounded security add-on.

### 2. Blocking Choice

The user is shown all five profiles and explicitly selects:

```text
MATT-SP-HYBRID + ECC(security-review)
```

This is the selection source. No work starts merely because the bundle was
recommended or its providers were detected.

### 3. Lock Record

The recorded bundle names Matt for requirements, specification, ticketing, TDD,
and hard-bug debugging; Superpowers for per-ticket plans, implementation,
review, remediation, verification, and completion; and ECC only for the bounded
security-review report. The active stage and ticket point to the canonical
specification, ticket, and executable plan.

### 4. Ticket Inheritance

When implementation moves from Ticket 01 to Ticket 02, ticket inheritance
copies the exact bundle. A dispatched implementation agent follows the approved
Superpowers plan and does not ask the user to select a family again. If a test
exposes an unexpected functional defect, the evidence transfers to Matt
debugging without changing the profile.

### 5. Specialist Return

At the declared security checkpoint, `ECC(security-review)` produces only its
report. Confirmed findings return to the Superpowers remediation and re-review
loop. ECC does not merge, commit, or claim general lifecycle ownership.

### 6. Stable-Boundary Switch

After a ticket is complete and its verification is recorded, the user may
request a stable-boundary switch, for example from the hybrid to `SP-FULL` for
the remaining approved tickets in the same deliverable. OAW records the new
choice at that boundary and preserves completed specifications, tickets, tests,
and review evidence. Until the user makes that explicit request, the original
bundle remains locked. A new unrelated task instead clears the old lock and
runs the startup gate again.

The [background](background.md) explains why the gate exists, and the
[comparison](comparison.md) documents the design evidence behind the hybrid.
