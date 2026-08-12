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

These scores guide OAW recommendations only after explicit activation has
created an Engagement. Normal Host Skill routing is outside this governance
path: before activation, the Host may automatically or explicitly select any
ordinary Skill without creating an OAW Request Mode. In `policy-cooperative`
use, the comparison can identify Host-visible Profile candidates; only
Core-backed inspection may call a Profile eligible.

Scores are on a 1.0 to 5.0 scale and appear in **Superpowers / Matt / ECC**
order. They help define an initial ownership map; they do not silently choose a
Profile. The user's explicit selection remains authoritative. A Provider's
brand does not determine its role: OAW Core compiles the selected Recipe and
assigns one owner to each responsibility. The optional Workflow Coordinator
records Workflow State, while the Agent Host performs `CURRENT` or native
`SUBAGENT` execution.

This comparison is source-pinned rather than name-inferred:

| Provider | Distribution | Source | Audited revision |
| --- | --- | --- | --- |
| Matt | `matt-skills` | `https://github.com/mattpocock/skills` | `84fdeffd12f2ee307994d1eb6feb48173b6e0502` |
| Superpowers | `superpowers` | `https://github.com/obra/superpowers` | `44c9b2d6e889982ac18c27d05a19fefe335194e1` |
| Superpowers | `superpowers-codex` | `https://github.com/openai/plugins` (`plugins/superpowers`) | `11c74d6ba24d3a6d48f54a194cd00ef3beea18f9` |
| ECC | `ecc` | `https://github.com/affaan-m/ECC` | `2d46e80e0925c7be0907f18c1812311ac212a6c5` |

The two Superpowers rows remain one Provider, `oaw/superpowers`. They are
non-interchangeable audited Distributions: Claude and direct upstream installs
use the obra tree, while the Codex packaged install uses the OpenAI plugin tree.
Each Distribution must match its own complete-tree digest; no file is ignored
and no Profile is added.

The audited names are surface-specific. Matt begins with `grill-with-docs`
and uses `to-spec`, `to-tickets`, `implement`, `tdd`, `diagnosing-bugs`, and
`code-review`. Superpowers references retain the `superpowers:` namespace.
ECC Skills, Claude custom Agents, Codex Roles, Instructions, Hooks, and tools
are separate evidence classes; no similar name is treated as substitution.

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
for complex work. Once a ticket is approved, `superpowers:writing-plans` owns
the per-ticket executable implementation plan. This is a responsibility split,
not concurrent ownership of the same artifact.

## Why the Ownership Map Was Corrected

An aggregate preference for one family would hide meaningful stage-level
differences. The hybrid therefore assigns exactly one owner to each
responsibility:

- Matt `grill-with-docs` (with credited `grilling` and `domain-modeling`),
  `to-spec`, and `to-tickets` own framing, specification, and ticket edges;
  Matt `tdd` and `diagnosing-bugs` own the hybrid's TDD and functional incident
  procedures.
- Superpowers owns executable ticket detail, workspace, inline implementation,
  standalone review/remediation, fresh verification, and closeout through the
  exact skills listed in the lifecycle matrix.
- An explicitly selected ECC resolver may own build, dependency, or type repair.
  An exact ECC specialist such as `ECC(security-review)` may produce only its
  declared bounded deliverable.

ECC specialists do not become lifecycle owners under `MATT-SP-HYBRID`.
Conversely, `ECC-FULL` remains available as an ECC-led lifecycle with neutral
Host/user controls. Its E2E specialists do not become broad verification, and
its reviewers do not become closeout owners. The score table does not remove
`SP-FULL`, `MATT-FULL`, `ECC-FULL`, or eligible user-defined Profiles from the
user's choices.

## How to Use This Comparison

Use the table as transparent design context for an activated OAW
recommendation. A machine-backed Workflow Startup Gate shows every eligible
Profile and proposed bounded add-on, then blocks for the user's choice. A
cooperative selection gate shows candidates and states its limitations instead
of claiming eligibility. If Provider versions or procedures change, the
evidence and scores should be reviewed together. A changed score alone must not
rewrite active Workflow authority or a cooperative Progress Tracker.

The [lifecycle guide](lifecycle.md) describes selection, persistence, and safe
switching. The normative rules are in
[policy/ENGINEERING.md](../../policy/ENGINEERING.md).
