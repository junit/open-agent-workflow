# 06 — Bounded Admission and Dispatch

**What to build:**

OAW admits exactly one Bounded Capability invocation for one declared
deliverable, validates selector provenance, resolves exactly one verified
Provider Instance, issues an immutable scoped Grant, and stops when additional
capabilities, loops, or scope expansion are required.

**Blocked by:** 02 — Configuration, Trust and Provider Discovery; 03 — Deterministic Request Classifier; 05 — Direct Runtime Vertical Slice

**Status:** ready-for-agent

- [ ] Missing or ambiguous Capability selection returns CAPABILITY_SELECTION_REQUIRED.
- [ ] A top-level Bounded request can use only explicit user intent or a matching
  user-trusted default rule as selector authority.
- [ ] Main Agent execution is allowed only when the Capability declares compatible
  topology and bounded effects.
- [ ] Child authority, resources, effects, and delegation remain narrower than the
  parent Grant.
- [ ] Additional Capability needs, remediation loops, architectural decisions, or
  scope expansion return MODE_ESCALATION_REQUIRED.
- [ ] Grant issuance, dispatch preparation, authorization, observation, and
  uncertainty states are durable and replay-safe.
