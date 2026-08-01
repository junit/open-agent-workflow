# 07 — Workflow Runtime Orchestration

**What to build:**

OAW starts a Workflow engineering run, requests explicit Profile Selection,
creates an immutable Lifecycle Bundle, drives the compiled Execution Graph
through Grants and observations, enforces one active mutating Executor per
physical Worktree, and produces projection artifacts without treating them as
authority.

**Blocked by:** 04 — Profile Recipe Compiler; 05 — Direct Runtime Vertical Slice; 06 — Bounded Admission and Dispatch

**Status:** ready-for-agent

- [ ] Only Workflow Mode triggers the blocking Profile-selection Gate.
- [ ] Bundle generation pins selection, recipe digest, Provider Instances,
  bindings, add-ons, configuration snapshot, and graph digest.
- [ ] Workflow execution requires Host-declared physical isolation from the Main
  Agent.
- [ ] Runtime refuses write-capable concurrent Stage Grants for the same physical
  Worktree resource.
- [ ] Review uses a fresh read-only Executor by default.
- [ ] Stable-boundary switching creates a new Bundle generation and revokes old
  outstanding Grants.
- [ ] Project projections are one-way outputs and never parsed back into Runtime
  authority.
