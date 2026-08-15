# MATT-SP-HYBRID Delivery Tickets

These are local tracer-bullet tickets for the isolated dogfood project. The
parent GitHub Issue remains unchanged; no unrelated public issues are created.

## 01 - Accept Non-Overlapping Maintenance Windows

**Blocked by:** None - can start immediately.

**What it delivers:** `windowcheck` accepts ordered and unordered same-day
windows, reports a valid plan, and treats Boundary Touch as valid.

- [ ] Parse the declared clock format and same-day bounds.
- [ ] Detect valid plans independent of input order.
- [ ] Preserve the domain terms in the user-visible summary.

## 02 - Reject Conflicting Or Malformed Windows

**Blocked by:** 01 - Accept Non-Overlapping Maintenance Windows.

**What it delivers:** `windowcheck` rejects Overlap and malformed windows with
actionable diagnostics and a non-zero status without writing partial output.

- [ ] Name the first overlapping pair.
- [ ] Reject malformed and backwards windows.
- [ ] Keep invalid input separate from valid plan output.
