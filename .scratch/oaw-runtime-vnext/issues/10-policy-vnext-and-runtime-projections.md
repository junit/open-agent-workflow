# 10 — Policy vNext and Runtime Projections

**What to build:**

OAW updates the portable Policy Plane to match Runtime vNext semantics while
preserving Policy-only fallback behavior. Runtime-managed runs emit project
projection documents that are human-readable, non-authoritative, and consistent
with the canonical glossary.

**Blocked by:** 03 — Deterministic Request Classifier; 04 — Profile Recipe Compiler; 07 — Workflow Runtime Orchestration; 09 — First Runtime Host and oaw run

**Status:** completed

- [x] Policy explains Direct, Bounded, and Workflow modes with Workflow-only
  Startup Gate activation.
- [x] Policy treats Superpowers, Matt, ECC, and third-party Providers through the
  same extensible Provider and Capability model.
- [x] ECC-FULL remains a valid complete lifecycle mapped to oaw/ecc-engineering.
- [x] CUSTOM-LOCKED is replaced by user-defined Profile selection semantics.
- [x] Policy-only Hosts do not claim Runtime admission, Grants, Resource Leases, or
  physical isolation.
- [x] Projection templates include selected Profile, Bundle generation, stage,
  active ticket, evidence references, and lag status without credentials or full
  Grants.

## Completion Record

Ticket 10 completed at implementation fixed point `8fdd78a`. The canonical
Policy Plane now classifies `DIRECT`, `BOUNDED`, and `WORKFLOW`, runs the
blocking Startup Gate only for Workflow Mode, treats built-in and third-party
Providers uniformly, preserves complete `ECC-FULL -> oaw/ecc-engineering`
ownership, and models `USER-DEFINED` as a configured Profile selection action.

Runtime projections expose selected Profile, Bundle generation, stage, an
independent optional Active Ticket, digest-pinned evidence references, and
explicit current/lagging status. Tests prove project JSON and Markdown exclude
credentials, full Grants, invocation/executor identity, raw output, and Grant
effects/resources/termination conditions. Policy-only Hosts explicitly make no
Runtime admission, Grant, Resource Lease, transition-enforcement, or physical
isolation claim.
