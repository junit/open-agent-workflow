# ADR 0012: Retain Only Profile-Binding Machine Assurance

## Status

Accepted

## Date

2026-08-15

## Context

ADR 0011 made the selected Markdown Policy Profile the only source of workflow
semantics and deferred one question: which parts of the pre-release machine
stack have a concrete reason to survive extraction. The repository currently
contains Provider discovery and integrity checks, fixed Recipe compilation,
request classification, Lifecycle Bundles, Capability Grants, durable Workflow
revisions, Resource Leases, Dispatch Packets, Receipts, and a Codex Bridge that
exposes all of those operations.

At this decision point, the only non-test callers of `internal/coordinator` are
the default CLI, `internal/codexbridge`, and the repository's dogfood harness.
There is no public package or external integration consumer. The implemented
state protocol therefore demonstrates code coverage, not product ownership.

Keeping that stack by default would preserve a second workflow model and make
the optional product more expensive than its demonstrated outcome. Deleting it
all would also discard the narrower ability to identify exactly which installed
Provider and Host Binding corresponds to a Skill named by a selected Profile.

This decision retains behavior, not existing packages. An implemented package
survives only when it is the smallest implementation of a retained use case.

## Decision Drivers

- Static Policy must remain the complete normal product.
- A machine result must have a named user or integration outcome and owner.
- Machine metadata must not restate or control Profile semantics.
- Optional components must have one-way dependencies toward the Policy Profile.
- Pre-release features without an owner default to deletion, not preservation.
- Host observations are evidence, not physical enforcement or execution proof.

## Considered Options

### Keep the existing Core, Coordinator, and Bridge

This preserves implemented behavior, but also preserves classification, Recipe,
execution graph, and transition authorities that ADR 0011 rejected. No external
consumer owns their maintenance or requires their guarantees.

### Delete every machine component

This is the smallest system. It also removes exact Profile-to-Binding
attestation, the one machine result that Policy prose and advisory inspection
cannot provide.

### Retain one attestation use case and its Host evidence adapter

Keep only an Assurance Overlay for exact Profile-to-Binding attestation and an
optional Bridge that can supply current Host facts. Delete machine workflow and
coordination behavior. This is the selected option.

## Decision

### Retained use cases

| Use case | User or integration outcome | Owner |
| --- | --- | --- |
| Profile-bound Binding attestation | An explicitly invoked audit or CI integration can identify one exact selected Policy Profile snapshot and the exact installed Provider and Host Binding claimed for each covered Skill or Host-action occurrence. | `oaw-assurance` owns Overlay issuance, validation, and diagnostics. |
| Current Host evidence acquisition | A Codex integration can supply secret-free, current-session Host facts to `oaw-assurance` without asking the user to transcribe installation or invocation metadata. | `oaw-bridge` owns Codex observation and transport; `oaw-assurance` owns whether those facts support a claim. |

The second use case exists only to feed the first. Bridge observation alone does
not create an Assurance Overlay and is not a new workflow product.

There is no retained machine coordination use case. No user or external
integration currently owns durable OAW Workflow revisions, idempotency,
Resource Leases, Dispatch Packets, Gates, Receipts, recovery, or machine Profile
switching. Those behaviors are deletion candidates. Git, CI, the Agent Host,
and user approval continue to own their existing physical outcomes.

### Minimal Assurance Overlay

An Assurance Overlay is an immutable, content-addressed machine artifact with
only these semantic fields:

1. a schema version;
2. exactly one source-qualified Policy Profile reference containing its `id`
   and full-document content digest;
3. one or more Binding claims, each keyed by a deterministic occurrence
   reference that resolves to an existing Skill or Host action in that exact
   Profile;
4. the exact Provider, distribution, Host, Binding kind, invocation surface,
   and content identities needed for each Binding claim;
5. secret-free evidence references used to establish those identities; and
6. an issuer identity and artifact digest.

The Profile content digest pins the complete Markdown rules. Occurrence
references are derived locators, not copied workflow semantics. The validator
must reject an unknown, ambiguous, duplicate, or cross-Profile occurrence
reference. Failure to resolve any requested claim prevents Overlay issuance but
does not affect whether the Profile can be selected or followed.

An Overlay has no fields for Responsibilities, Skill composition, order,
rules, alternatives, Add-ons, planning depth, Risk, Request Mode, topology,
gates, effects, approval, progress, completion, or Profile switching. It cannot
add, remove, reorder, substitute, or assign a Skill. It cannot claim that a
Skill ran or that a deliverable completed. A future execution-evidence contract
requires a separately owned use case and ADR.

### Dependency direction

The target dependency graph is one-way:

```text
oaw-bridge
    |
    v
oaw-assurance
    |
    v
shared read-only Policy Profile reader
    |
    v
selected Markdown Policy Profile
```

The default `oaw` binary may share the read-only Profile reader for advisory
inspection. It must not import, install, start, or require `oaw-assurance` or
`oaw-bridge`. `oaw-assurance` must not import Bridge code. Neither optional
component may write a Profile or publish an alternative Profile model.

The Agent Host retains physical invocation authority. A Host security policy
may refuse an invocation even when an Overlay exists; that refusal neither
invalidates nor changes the referenced Policy Profile.

### Default CLI exclusions

Before extraction begins, the default CLI boundary is fixed to installation
management (`install`, `update`, `check`, and `uninstall`) and advisory Profile
inspection (`profile list`, `profile show`, and `profile check`). The following
current surfaces are excluded from the target default `oaw` binary:

- `profiles`, `use`, `status`, `complete`, `review`, `approve`, `satisfy`,
  `switch`, `incident`, `uncertain`, and `stop`;
- `catalog`, `providers inspect`, and `workflow exchange`;
- `runtime`, `run`, and every Bridge install, check, update, uninstall, serve,
  or Hook command; and
- all flags or prompts for Request Mode, Complexity, Risk, topology, Add-on
  sentinels, Recipe selection, Lifecycle Bundle confirmation, or machine
  workflow state.

Bridge and Assurance, when explicitly installed, have their own executables,
commands, state roots, and release artifacts. The default installer manages
neither component.

### Retention and deletion map

No current package is retained wholesale.

| Current behavior | Target disposition |
| --- | --- |
| Markdown Profile loading and digesting | Retain once in a shared read-only Profile reader. |
| Provider discovery, distribution integrity, Registry resolution, Host Binding inventory, and canonical digests | Retain only the minimum code directly needed to issue or validate a Binding claim; keep it behind `oaw-assurance`. |
| Codex native Skill, Hook, and configuration observation | Retain only as `oaw-bridge` input to Assurance. |
| Fixed aliases, Recipes, execution graphs, Profile eligibility, recommendations, and `USER-DEFINED` builder | Delete; the Markdown Profile already defines the method. |
| Formal request classification and Complexity or Risk records | Delete; they are model judgments in Policy. |
| Lifecycle Bundle compilation and selection confirmation | Replace with the minimal Assurance Overlay; delete the old Bundle schema and compiler. |
| Capability Grants, authority ceilings, Dispatch Packets, Resource Leases, Gate Attestations, Receipts, Workflow revisions, replay, recovery, and machine switches | Delete; no retained use case owns them. |
| `core.inspect`, `core.compile`, and `workflow_exchange` Bridge operations and the Hooks that exist only to police those operations | Delete during Bridge extraction. |
| Provider/catalog and machine workflow commands in the default CLI | Delete during the default CLI hard cut. |
| Old machine dogfood, schema assets, and provider-audit code | Delete when their only consumer is a removed Recipe, Bundle, Coordinator, or duplicate catalog authority. |

Support code such as canonical encoding survives only when imported by a
retained product path. Tests are not owners and cannot by themselves justify a
production behavior.

## Consequences

### Positive

- The optional product has one narrow, explainable result.
- Policy Profile semantics remain single-source and model-readable.
- The Coordinator and its state protocol can be removed instead of extracted.
- Bridge extraction becomes a Host adapter task rather than preservation of a
  second runtime.
- Machine claim failure is isolated from normal engineering work.

### Negative

- Existing machine workflow APIs and artifacts are removed without replacement.
- An Overlay proves identity and mapping, not invocation or completion.
- The Profile reader must expose stable derived occurrence references without
  turning Markdown into a second workflow schema.
- Existing discovery and integrity code must be reduced rather than moved as a
  block.

### Risks and Mitigations

- **Semantic smuggling through Overlay fields:** use an allowlisted Overlay
  decoder and reject every field that could redefine Profile behavior.
- **Misleading partial claims:** report exactly which Profile occurrences are
  covered and never label an Overlay as workflow completion evidence.
- **Reverse dependencies:** add import guards for the default CLI, Assurance,
  and Bridge boundaries.
- **Preservation by package inertia:** require a retained-use-case reference for
  every production package moved into an optional component.

## Follow-up

- Issue #6 extracts `oaw-assurance` and implements only the retained Overlay.
- Issue #7 extracts `oaw-bridge`, retains Host observation, and removes Core and
  Coordinator operations.
- Issue #8 hard-cuts the default CLI to the static product.
- Issue #9 deletes machine-shaped Policy progress and persistence.
- Issue #10 deletes Recipe, classification, Bundle, Coordinator, and duplicate
  catalog authorities left after extraction.

## Related Decisions

- Adds the optional-component retention boundary deliberately deferred by ADR
  0011 without changing its static Policy product.
- Narrows ADR 0010's optional machine extension to one attestation use case and
  supersedes its provisional retention of Core and Coordinator behavior.
- Supersedes ADR 0009's Coordinator retention while preserving its Host-owned
  physical execution boundary.
