# Maintenance Window Check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:executing-plans` to implement this plan task-by-task.

**Goal:** Deliver a dependency-free `windowcheck` command that validates
same-day Maintenance Windows and reports Overlap without persistence.

**Architecture:** The `maintenance` package is the one domain seam. It parses
and orders half-open windows, accepts Boundary Touch, and returns a stable
summary or domain error. The command adapter only validates argument count and
maps the domain result to stdout/stderr and exit status.

**Tech Stack:** Go standard library, Go tests, no external modules.

---

### Task 1: Accept Non-Overlapping Windows

**Files:**
- Create: `internal/maintenance/maintenance.go`
- Test: `internal/maintenance/maintenance_test.go`

- [ ] Write the failing `Evaluate` test for unordered input with Boundary
  Touch, run it, then add the parser and valid summary.
- [ ] Rerun the focused test and commit `feat: accept non-overlapping
  maintenance plans`.

### Task 2: Reject Overlap And Invalid Windows

**Files:**
- Modify: `internal/maintenance/maintenance.go`
- Test: `internal/maintenance/maintenance_test.go`

- [ ] Add the literal Overlap and malformed/backwards cases and observe RED.
- [ ] Compare ordered windows with `current.start < previous.end`, reject
  malformed clocks and `end <= start`, then rerun the domain suite.
- [ ] Commit `feat: reject maintenance overlaps`.

### Task 3: Add The CLI Adapter

**Files:**
- Create: `cmd/windowcheck/main.go`
- Test: `cmd/windowcheck/main_test.go`

- [ ] Add tests for valid output, domain error status, and missing arguments;
  observe `run` RED.
- [ ] Implement the thin adapter with status `2` for usage and status `1` for
  domain errors.
- [ ] Run `go test ./...` and `go vet ./...`, then commit
  `feat: add windowcheck CLI`.

### Task 4: Review And Verify

- [ ] Reread this plan, the Matt spec, tickets, and the diff against the domain
  terms.
- [ ] Apply the Superpowers review checklist; record findings and remediation.
- [ ] Run fresh tests, vet, coverage, and representative `go run` commands
  after removing the temporary `oaw` installer executable.
- [ ] Confirm no files, state, network, or optional OAW component are required.
