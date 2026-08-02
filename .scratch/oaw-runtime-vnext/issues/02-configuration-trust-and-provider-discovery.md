# 02 — Configuration, Trust and Provider Discovery

**What to build:**

OAW reads built-in, user, and trusted-project configuration records, validates
trust and namespace rules, discovers built-in Provider installations through
declarative probes, and produces an Effective Configuration Snapshot plus an
Effective Registry without loading Provider code or allowing project
configuration to grant authority.

**Blocked by:** 01 — Built-in Catalog and Contracts

**Status:** completed

- [x] Human-authored configuration and Provider Descriptors use TOML and normalize
  to canonical JSON for hashing, validation, and state references.
- [x] User deny decisions override built-in, user, and project configuration.
- [x] Project configuration can request, recommend, or narrow behavior but cannot
  grant trust, widen authority, or enable a denied Provider.
- [x] Built-in Provider discovery produces deterministic evidence for Superpowers,
  Matt, and ECC installations.
- [x] Provider verification pins descriptor digest, location, version,
  configuration, binding, and evidence digest.
- [x] Provider state outcomes distinguish not found, candidate, verified,
  ambiguous, incompatible, binding unavailable, disabled, and untrusted.
- [x] Configuration changes do not alter an already active Lifecycle Bundle.
