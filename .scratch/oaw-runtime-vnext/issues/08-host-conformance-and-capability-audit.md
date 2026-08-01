# 08 — Host Conformance and Capability Audit

**What to build:**

OAW defines and runs a Host Adapter conformance suite that proves which
integration level, isolation behavior, native invocation behavior,
deduplication, cancellation, and observation guarantees a Host can provide
before any Runtime-managed Workflow uses that Host.

**Blocked by:** 07 — Workflow Runtime Orchestration

**Status:** ready-for-agent

- [ ] Host manifests are pinned through trusted integration records and cannot
  self-enable features from per-run frames.
- [ ] Instruction-only integrations cannot claim Runtime admission or isolation.
- [ ] Runner-managed and native-managed integrations pass fixtures for isolated
  Executor creation, exact Binding invocation, pause behavior, Bundle
  inheritance, evidence return, and invocation deduplication.
- [ ] Missing required features deny Runtime-managed Workflow with a stable reason
  code.
- [ ] The first Runtime Host remains unselected until the official capability audit
  and conformance proof complete.
