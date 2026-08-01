# 11 — Go check Black-box Parity

**What to build:**

OAW adds a Go implementation path for the read-only check behavior that matches
current Bash black-box behavior command by command while Bash remains the
authoritative management interface.

**Blocked by:** 01 — Built-in Catalog and Contracts; 02 — Configuration, Trust and Provider Discovery

**Status:** ready-for-agent

- [ ] Existing Bash check behavior remains unchanged.
- [ ] Go check fixtures reproduce Bash output, status codes, target resolution,
  provider diagnostics, and read-only filesystem behavior.
- [ ] Drift, scope, target, and provider detection parity failures block promotion.
- [ ] The parity harness can run Bash and Go implementations against the same
  isolated fixtures.
- [ ] Documentation and diagnostics state that Go check is shadow or parity mode
  until explicitly cut over.
