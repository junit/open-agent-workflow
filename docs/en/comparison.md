# Initial Workflow Family Comparison

[简体中文](../zh/comparison.md) | [README](../../README.md)

This comparison explains why Open Agent Workflow (OAW) offers full-family
Profiles as well as the predefined Matt-Superpowers hybrid. It compares the
workflow procedures available from Superpowers, Matt Pocock skills, and
Everything Claude Code (ECC). It does not compare model quality or agent-tool
quality.

## Interpretation Limits

The scores are experience-based judgments derived from the provider procedures
reviewed during the local v0.1 design. They are version-sensitive because
provider skills, agents, triggers, and documentation can change. The table is
not a universal benchmark, an empirical performance study, or a promise that
one family wins every repository and task.

Scores are on a 1.0 to 5.0 scale and appear in **Superpowers / Matt / ECC**
order. They help define an initial ownership map; they do not silently choose a
Profile. The user's explicit selection remains authoritative. A Provider's
brand does not determine its role: OAW Core compiles the selected Recipe and
assigns one owner to each responsibility. The optional Workflow Coordinator
records Workflow State, while the Agent Host performs `CURRENT` or native
`SUBAGENT` execution.

## Criteria

Each stage was judged using the same six criteria:

| Criterion | Question |
| --- | --- |
| procedure completeness | Does the family cover the stage from entry conditions through an observable result? |
| correctness discipline | Does it enforce evidence, tests, validation, and clear failure handling? |
| ambiguity handling | Does it expose unknowns and resolve them before irreversible work? |
| review closure | Does feedback return through remediation and re-review instead of ending at a finding list? |
| verification strength | Does completion depend on fresh, relevant evidence rather than assertion? |
| operational overhead | Is the procedure proportionate, composable, and practical for repeated use? |

Operational overhead is a design tradeoff, not a reward for doing less. A
slightly heavier procedure can still score highly when its controls are
proportionate to the risk it manages.

## Approved v0.1 Scores and Ownership

| Stage | Superpowers | Matt | ECC | Corrected hybrid owner |
| --- | ---: | ---: | ---: | --- |
| Planning | 4.8 | 5.0 | 3.8 | Matt for complex work |
| Implementation | 5.0 | 4.2 | 3.7 | Superpowers |
| TDD | 4.8 | 4.9 | 4.1 | Matt |
| Debugging | 4.7 | 5.0 | 2.8 | Matt |
| Review | 5.0 | 4.8 | 4.4 | Superpowers |
| Completion | 5.0 | 3.6 | 4.0 | Superpowers |

The planning row needs one qualification. Matt owns requirements, domain
modeling, product specification, test-seam selection, and ticket decomposition
for complex work. Once a ticket is approved, Superpowers `writing-plans` owns
the per-ticket executable implementation plan. This is a responsibility split,
not concurrent ownership of the same artifact.

## Why the Ownership Map Was Corrected

An aggregate preference for one family would hide meaningful stage-level
differences. The hybrid therefore assigns exactly one owner to each
responsibility:

- Matt owns requirements, domain modeling, specification, ticket decomposition,
  TDD method, and functional or hard-bug debugging.
- Superpowers owns workspace and Git setup, implementation orchestration, code
  changes, spec compliance review, quality review, remediation, re-review,
  fresh verification, and branch completion.
- An explicitly selected ECC resolver may own build, dependency, or type repair.
  An exact ECC specialist such as `ECC(security-review)` may produce only its
  declared bounded deliverable.

ECC specialists do not become lifecycle owners under `MATT-SP-HYBRID`.
Conversely, `ECC-FULL` remains available when the user wants ECC to own the
complete lifecycle. The score table does not remove `SP-FULL`, `MATT-FULL`,
`ECC-FULL`, or eligible user-defined Profiles from the user's choices.

## How to Use This Comparison

Use the table as transparent design context for a recommendation. Before work
starts, OAW still shows every profile and any proposed bounded add-ons, then
blocks for the user's choice. If provider versions or procedures change, the
evidence and scores should be reviewed together. A changed score alone must not
rewrite an active lifecycle lock.

The [lifecycle guide](lifecycle.md) describes selection, persistence, and safe
switching. The normative rules are in
[policy/ENGINEERING.md](../../policy/ENGINEERING.md).
