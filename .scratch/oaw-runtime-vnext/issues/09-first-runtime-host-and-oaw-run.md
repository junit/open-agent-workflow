# 09 — First Runtime Host and oaw run

**What to build:**

After a Host passes official capability audit and the user explicitly selects
it, OAW exposes oaw run as the reference Runtime entrypoint for that Host and
drives Runtime Protocol exchanges without changing unsupported Hosts from
Policy-only behavior.

**Blocked by:** 08 — Host Conformance and Capability Audit; explicit user Host selection

**Status:** ready-for-agent

- [ ] The selected Host has a pinned conforming Manifest and integration record.
- [ ] oaw run uses the same Runtime Protocol as native Host integrations.
- [ ] Machine-facing exchange output is canonical JSON on stdout with diagnostics
  on stderr.
- [ ] Unsupported Hosts remain Policy-only and do not claim Runtime guarantees.
- [ ] Runtime-aware entrypoints are installed only where exact Host capability
  permits them.
- [ ] User selection of the first Runtime Host is recorded as migration evidence.
