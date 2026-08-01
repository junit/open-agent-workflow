# 01 — Built-in Catalog and Contracts

**What to build:**

OAW exposes a versioned Go contract foundation for the Runtime vNext domain and
a deterministic built-in catalog for Superpowers, Matt, ECC, the built-in
Profile Recipes, compatibility aliases, and schema metadata. A developer can
run a local catalog command and receive stable human-readable and machine-
readable output without changing existing Bash management behavior.

**Blocked by:** None — can start immediately

**Status:** ready-for-agent

- [ ] The repository has an initial Go module and a testable package layout for
  schemas, provider descriptors, capability declarations, recipe declarations,
  aliases, and catalog loading.
- [ ] Built-in catalog records exist for oaw/superpowers, oaw/matt, and oaw/ecc.
- [ ] Built-in recipes exist for oaw/delivery, oaw/domain-engineering,
  oaw/ecc-engineering, oaw/reliable-feature, and oaw/hardening.
- [ ] Compatibility aliases map SP-FULL, MATT-FULL, ECC-FULL, and MATT-SP-HYBRID
  to their approved built-in recipes.
- [ ] The catalog validates duplicate IDs, missing references, unsupported schema
  versions, missing capabilities, missing recipe responsibility ownership, and
  alias targets.
- [ ] Human and JSON catalog inspection are deterministic across runs.
- [ ] Existing Bash installer commands and regression tests continue to pass.
