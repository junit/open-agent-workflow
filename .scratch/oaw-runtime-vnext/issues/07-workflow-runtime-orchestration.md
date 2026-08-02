# 07 — Workflow Runtime Orchestration

**What to build:**

OAW starts a Workflow engineering run, requests explicit Profile Selection,
creates an immutable Lifecycle Bundle, drives the compiled Execution Graph
through Grants and observations, enforces one active mutating Executor per
physical Worktree, and produces projection artifacts without treating them as
authority.

**Blocked by:** 04 — Profile Recipe Compiler; 05 — Direct Runtime Vertical Slice; 06 — Bounded Admission and Dispatch

**Status:** completed

- [x] Only Workflow Mode triggers the blocking Profile-selection Gate.
- [x] Bundle generation pins selection, recipe digest, Provider Instances,
  bindings, add-ons, configuration snapshot, and graph digest.
- [x] Workflow execution requires Host-declared physical isolation from the Main
  Agent.
- [x] Runtime refuses write-capable concurrent Stage Grants for the same physical
  Worktree resource.
- [x] Review uses a fresh read-only Executor by default.
- [x] Stable-boundary switching creates a new Bundle generation and revokes old
  outstanding Grants.
- [x] Project projections are one-way outputs and never parsed back into Runtime
  authority.

**Implementation fixed point:** `8665458`

**Evidence:** `.scratch/oaw-runtime-vnext/evidence/review.md` and
`.scratch/oaw-runtime-vnext/evidence/verification.md`
