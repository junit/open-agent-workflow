# 10 — Policy vNext and Runtime Projections

**What to build:**

OAW updates the portable Policy Plane to match Runtime vNext semantics while
preserving Policy-only fallback behavior. Runtime-managed runs emit project
projection documents that are human-readable, non-authoritative, and consistent
with the canonical glossary.

**Blocked by:** 03 — Deterministic Request Classifier; 04 — Profile Recipe Compiler; 07 — Workflow Runtime Orchestration; 09 — First Runtime Host and oaw run

**Status:** in-progress

- [ ] Policy explains Direct, Bounded, and Workflow modes with Workflow-only
  Startup Gate activation.
- [ ] Policy treats Superpowers, Matt, ECC, and third-party Providers through the
  same extensible Provider and Capability model.
- [ ] ECC-FULL remains a valid complete lifecycle mapped to oaw/ecc-engineering.
- [ ] CUSTOM-LOCKED is replaced by user-defined Profile selection semantics.
- [ ] Policy-only Hosts do not claim Runtime admission, Grants, Resource Leases, or
  physical isolation.
- [ ] Projection templates include selected Profile, Bundle generation, stage,
  active ticket, evidence references, and lag status without credentials or full
  Grants.
