# 04 — Profile Recipe Compiler

**What to build:**

OAW compiles Profile Recipes and Profile Bindings into deterministic Execution
Graphs using verified Provider Instances, exact Capability selectors, typed
Procedures, Incident Handlers, Checkpoints, terminal gates, and stable
boundaries. Invalid ownership, missing coverage, or ambiguous resolution fails
closed.

**Blocked by:** 01 — Built-in Catalog and Contracts; 02 — Configuration, Trust and Provider Discovery

**Status:** ready-for-agent

- [ ] Full-family eligibility is determined by verified Capability coverage for
  the selected Recipe, not Provider brand or hardcoded specialty.
- [ ] Every applicable responsibility has exactly one owner.
- [ ] SP-FULL, MATT-FULL, ECC-FULL, and MATT-SP-HYBRID aliases compile only when
  their mapped recipe and Capability requirements verify.
- [ ] User-defined Profiles compile through the same contract as built-in Profiles.
- [ ] Incident and remediation loops are explicit and closed.
- [ ] The emitted Execution Graph digest is deterministic for equivalent inputs.
- [ ] Compilation reports stable reason codes for missing capabilities, duplicate
  owners, ambiguous selectors, and unsupported effects.
