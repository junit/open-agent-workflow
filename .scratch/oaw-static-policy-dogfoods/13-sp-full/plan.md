# SP-FULL Slugify Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:executing-plans` to implement this plan task-by-task.

**Goal:** Deliver and verify a small Go slugify command under the managed
SP-FULL Profile in an isolated no-Bridge project.

**Architecture:** Put transformation semantics in `internal/slug` and keep the
command as a thin adapter. Preserve the isolated project, test, review, and
verification evidence in this dogfood evidence set.

**Tech Stack:** Go standard library, `go test`, project-scoped OAW Policy Set.

---

### Task 1: Transformation Contract

**Files:**
- Create: `project/go.mod`
- Create: `project/internal/slug/slug_test.go`
- Create: `project/internal/slug/slug.go`

- [ ] Write table-driven tests for mixed punctuation, edge separators,
  Unicode, and empty input.
- [ ] Run `go test ./internal/slug` and record the expected undefined-symbol
  RED failure.
- [ ] Implement the minimal single-pass Unicode transformation.
- [ ] Run `go test ./internal/slug` and record GREEN.

### Task 2: CLI Adapter

**Files:**
- Create: `project/cmd/slugify/main_test.go`
- Create: `project/cmd/slugify/main.go`

- [ ] Write tests for one valid argument and invalid argument counts.
- [ ] Run `go test ./cmd/slugify` and record the expected undefined-symbol RED
  failure.
- [ ] Implement `run(args, stdout, stderr)` and the process adapter.
- [ ] Run `go test ./...` and record GREEN.

### Task 3: Isolated Policy Run and Evidence

**Files:**
- Create: `evidence/selection.md`
- Create: `evidence/tdd.md`
- Create: `evidence/review.md`
- Create: `evidence/verification.md`

- [ ] Copy the project to a fresh temporary directory with isolated HOME and
  XDG roots.
- [ ] Install only the project Codex Policy integration and remove the
  temporary `oaw` executable.
- [ ] Confirm the managed SP-FULL Profile and Superpowers references are
  readable without the old startup form.
- [ ] Perform a fresh review of source, tests, and acceptance boundaries.
- [ ] Run the full project tests and CLI examples after the executable removal.
