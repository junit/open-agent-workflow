# OAW Runtime vNext Ticket 10: Policy vNext and Runtime Projections

> For inline execution: Matt `tdd` owns every RED/GREEN cycle. Superpowers
> owns this executable plan, implementation orchestration, review, verification,
> and completion. Do not dispatch subagents.

**Goal:** Align the canonical Policy Plane and bilingual lifecycle guidance
with Runtime vNext classification, extensible Provider/Capability selection,
and full-family ECC support. Extend project workflow projections with a
non-authoritative view of the selected Profile, Bundle generation, active
stage/ticket, digest-pinned evidence references, and explicit projection lag
status without exporting credentials, Grants, or raw Evidence.

**Canonical sources:** `.scratch/oaw-runtime-vnext/spec.md`,
`.scratch/oaw-runtime-vnext/issues/10-policy-vnext-and-runtime-projections.md`,
`CONTEXT.md`, `docs/adr/0003-add-optional-capability-admission-runtime.md`, and
`docs/adr/0004-implement-runtime-core-in-go.md`.

## Scope and contracts

This ticket owns:

- `policy/ENGINEERING.md` as the normative Policy Plane document;
- `docs/en/lifecycle.md` and `docs/zh/lifecycle.md` as synchronized guidance;
- projection JSON/Markdown templates and their public `WorkflowProjection`
  record;
- the canonical glossary entry and Runtime source for optional Active Ticket
  projection state;
- documentation and Runtime security contracts for projection redaction.

It does not change Runtime admission, Grant issuance, Resource Lease semantics,
Host integration, Profile compilation, or the authoritative Runtime State
schema. Policy-only Hosts remain explicitly outside Runtime admission,
Grants, Resource Leases, and physical isolation guarantees.

Projection field mapping is deterministic:

| Projection field | Authoritative Runtime source |
| --- | --- |
| `profile` | active Bundle `Selection.Profile` |
| `bundle_generation` | active Bundle `Generation` |
| `stage` | active Bundle graph node / `Workflow.ActiveNodeID` |
| `active_ticket` | independent `Workflow.ActiveTicket` state seeded by Workflow input |
| `evidence_references` | normalized Workflow Stage Observation references only |
| `lag_status` | committed projection-lag records for this Run/revision |

The projection contains only evidence references and digests. It must never
contain credentials, complete Grants, invocation/executor identities, raw
provider output, or arbitrary evidence content. Projection failures record lag
and never change the committed reply or Runtime State.

## Task 1: Projection RED/GREEN slice

**Files:**

- Modify: `internal/runtime/projection_test.go`
- Modify: `internal/runtime/workflow_helpers_test.go`
- Modify: `internal/runtime/projection.go`

1. Add failing tests asserting selected and unselected Workflow projections
   expose `Profile`, `BundleGeneration`, `Stage`, `ActiveTicket`, an empty or
   normalized `EvidenceReferences` set, and a structured `LagStatus`.
2. Add a committed-observation fixture proving evidence references are sorted,
   digest-pinned, and projected without Grant, Invocation, Executor, or raw
   output fields.
3. Add validation tests rejecting missing Profile/Stage/Ticket for a selected
   Bundle, invalid evidence references/digests, duplicate evidence, malformed
   lag status, and digest mismatches.
4. Run focused tests and verify RED:

   ```bash
   rtk go test ./internal/runtime -run 'Projection'
   ```

5. Extend `WorkflowProjection` and its constructor/validator/Markdown renderer.
   Keep existing Bundle and Host pin fields for compatibility; use the active
   Bundle's `Selection.Profile` as the projection Profile and its generation as
   `BundleGeneration`.
6. Aggregate only normalized Workflow observation evidence into a stable,
   deduplicated sorted list. Never copy `RawOutput` or any Grant record.
7. Represent lag with an explicit status (`current`, `lagging`) and the latest
   committed lag marker for the Run. A successful write emits `current`; a
   failed write records the existing immutable lag marker and leaves the
   committed response unchanged.
8. Run focused tests and then all Runtime tests:

   ```bash
   rtk gofmt -w internal/runtime/projection.go internal/runtime/projection_test.go internal/runtime/workflow_helpers_test.go
   rtk go test ./internal/runtime
   ```

## Task 2: Policy Plane vNext

**Files:**

- Modify: `policy/ENGINEERING.md`
- Modify: `docs/en/lifecycle.md`
- Modify: `docs/zh/lifecycle.md`
- Modify: `README.md`
- Modify: `README-zh.md`
- Modify: `tests/10-docs-test.sh`

1. Replace the two-level ordinary/complex entrypoint with `DIRECT`, `BOUNDED`,
   and `WORKFLOW` modes. State that only Workflow Mode activates the blocking
   Startup Gate; Direct and Bounded mode do not require lifecycle selection.
2. Define Direct as bounded main-agent work outside Runtime admission and
   Bounded as one explicitly selected, observable Capability with no lifecycle
   ownership. Keep Workflow Complexity and Risk orthogonal to Request Mode.
3. Define one extensible Provider/Capability model for built-in Superpowers,
   Matt, ECC, and user-trusted third-party Providers. State that detection is
   diagnostic and never selects a Profile. Preserve dynamic built-in discovery
   and declarative user/project registration boundaries.
4. Keep `SP-FULL`, `MATT-FULL`, `ECC-FULL`, and `MATT-SP-HYBRID` as selectable
   compatibility aliases. State that `ECC-FULL` is a complete lifecycle mapped
   to `oaw/ecc-engineering`, while ECC may also be a bounded specialist.
5. Replace `CUSTOM-LOCKED` as a fake Profile with `USER-DEFINED` selection:
   the user selects or authors a versioned Profile Recipe whose compiled graph
   has exactly one owner per responsibility, explicit transitions, and bounded
   add-ons. Runtime admits only verified Provider Capabilities.
6. Describe Workflow isolation and bundle inheritance, but explicitly state
   that Policy-only Hosts have no Runtime admission, Grant, Resource Lease, or
   physical isolation guarantee. Add the allowed-actions / stage ownership
   concept without claiming Markdown itself enforces it.
7. Update bilingual examples and README entrypoints to use the three modes,
   extensible profiles, and the new `USER-DEFINED` semantics.
8. Add black-box contracts for the new terms and for the absence of the stale
   `CUSTOM-LOCKED` Profile claim. Run:

   ```bash
   rtk bash tests/10-docs-test.sh
   rtk bash scripts/check-docs.sh
   ```

## Task 3: Review and verification

1. Review the diff against the Runtime vNext specification and this ticket.
   Confirm no projection writes back into Runtime State and no credentials,
   full Grants, or evidence content are serialized.
2. Run the complete verification matrix used by Tickets 09 and 08:

   ```bash
   rtk go test ./...
   rtk go vet ./...
   rtk go test -race ./...
   rtk bash tests/10-docs-test.sh
   rtk bash scripts/check-docs.sh
   rtk shellcheck install.sh scripts/check-docs.sh
   ```

3. Record review and verification evidence, mark the issue complete, and
   update the tracker only after all checks pass.
