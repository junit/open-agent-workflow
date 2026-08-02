# 05 — Direct Runtime Vertical Slice

**What to build:**

OAW starts a Runtime exchange for a Direct engineering request, records a
classification decision, returns a release response to the Host, and makes clear
that Direct work is outside Capability admission and Resource Lease guarantees.

**Blocked by:** 03 — Deterministic Request Classifier

**Status:** completed

- [x] START accepts a valid classification proposal and project identity.
- [x] The Runtime commits a durable Direct decision before replying.
- [x] The reply releases execution to the Host without creating a Lifecycle Bundle,
  Provider Grant, Stage Grant, or Resource Lease.
- [x] Direct release diagnostics truthfully state that OAW does not control
  subsequent Host tool calls.
- [x] Direct scope expansion returns an escalation requirement instead of silently
  upgrading in place.
- [x] Read-only inspection of the Direct run returns the committed snapshot.
