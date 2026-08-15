# ADR 0002: Keep Machine Evidence Optional And Separate

## Status

Accepted

## Date

2026-08-16

## Context

Some audits need an exact claim about the selected Profile bytes, installed
Provider identity, or Host Binding. Readable Policy rules cannot by themselves
make those machine-verifiable claims. Adding that evidence to the normal Policy
path would make everyday work depend on scanners, digests, and Host integration
availability.

## Decision Drivers

- Static Policy must remain the complete normal path.
- Machine evidence must have one narrow, inspectable meaning.
- Evidence failure must not change Profile availability.
- Optional integration must not gain model or filesystem execution authority.
- Default installation must not install optional components.

## Decision

Machine evidence is implemented by separate executables:

- `oaw-assurance` inspects one source-qualified Markdown Profile and issues or
  verifies a content-addressed Assurance Overlay.
- `oaw-bridge` is an optional Codex integration that observes secret-free
  current Host facts and supplies them to the Assurance path.

An Assurance Overlay may identify exact Profile occurrences, Provider content,
Host Binding kinds, invocation surfaces, and evidence references. It cannot
copy or redefine Responsibilities, select or reorder Skills, authorize an
invocation, manage progress, or claim that work completed.

The dependency boundary is one-way: the optional executables may read a Policy
Profile, while the default `oaw` executable, installer, Policy Set, and Profile
selection path must not import, install, invoke, or require them.

Bridge observation is read-only evidence acquisition. The Host may refuse any
physical invocation regardless of an Overlay, and missing Bridge evidence
removes only the machine claim.

## Consequences

### Positive

- Audit users can obtain exact identity evidence.
- Normal delivery remains unaffected by missing or failed optional components.
- Optional component code can be security-reviewed independently.
- The Profile remains the only engineering-method authority.

### Negative

- Evidence tooling has a separate installation and support lifecycle.
- An Overlay proves identity and mapping, not execution or completion.
- Codex observation depends on the Host's supported integration surface.

## Required Invariants

1. `cmd/oaw` has no dependency on Assurance or Bridge packages.
2. `oaw-assurance` does not depend on Bridge implementation.
3. `oaw-bridge` exposes observation for Assurance, not workflow operations.
4. Optional failures never veto a Policy-valid workflow.
5. No optional record may restate Profile semantics.
