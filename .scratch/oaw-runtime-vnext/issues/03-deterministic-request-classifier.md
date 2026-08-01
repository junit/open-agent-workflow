# 03 — Deterministic Request Classifier

**What to build:**

OAW accepts an untrusted Host Semantic Classifier proposal with closed traits
and produces a deterministic Classification Decision containing Request Mode,
Workflow Complexity, Risk Class, evidence requirements, and escalation reasons.
Missing or uncertain critical information fails upward.

**Blocked by:** 01 — Built-in Catalog and Contracts; 02 — Configuration, Trust and Provider Discovery

**Status:** ready-for-agent

- [ ] The classifier never calls a model API and never uses keyword-only natural
  language parsing as its authority.
- [ ] Decisions distinguish DIRECT, BOUNDED, and WORKFLOW from workflow complexity
  and risk class.
- [ ] Direct Mode requires clear scope, no public contract or high-risk semantic
  changes, and a focused verification command.
- [ ] Bounded Mode requires an exact user-authorized or trusted-rule Capability
  selector before admission.
- [ ] Workflow Mode is selected for unresolved requirements, architectural
  decisions, multiple responsibilities, sensitive mutations, multi-ticket work,
  missing valid proposals, or unknown critical traits.
- [ ] User and trusted-project rules can only raise mode, risk, or evidence
  requirements.
- [ ] Critical release scenarios in the classifier eval corpus are never released
  as Direct or Bounded.
