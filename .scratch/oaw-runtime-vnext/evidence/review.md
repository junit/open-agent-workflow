# OAW Runtime vNext Written-Spec Review

**Date:** 2026-08-01
**Scope:** `CONTEXT.md`, the Runtime vNext specification and design tracker,
ADR 0003, and ADR 0004
**Review owner:** Superpowers
**Result:** Ready for user written-spec review; implementation planning has not
started

## Review Method

Two independent read-only reviews checked the approved design against the
request model, Provider extensibility requirements, Runtime authority boundary,
migration constraints, glossary consistency, and ADR consequences. Subsequent
user written-spec review corrected the `ECC-FULL` capability assumption. A final
self-review checked the resulting schema vocabulary and control-flow ordering.

No Critical findings were reported. All Important findings were corrected
before this evidence was recorded.

## Findings and Dispositions

| Finding | Disposition |
| --- | --- |
| Executor logical identity conflicted with required Workflow context isolation. | The glossary now separates authority bookkeeping from physical context isolation, and the spec requires a trusted conforming Host integration. |
| Read-only `INSPECT` appeared to create a Run Revision. | Only mutating `START` and `CONTINUE` exchanges commit transitions; `INSPECT` reads a committed snapshot. |
| Bounded Mode did not define Capability selection or Main Agent execution. | Mode classification no longer implies Capability selection; exact user intent or a user-trusted rule is required, ambiguity pauses, and `executor_topology` controls Main Agent eligibility. |
| Host frames could self-attest isolation, deduplication, and cancellation features. | `START` references a pinned built-in or user-trusted Host integration record; per-run declarations may only narrow its Manifest. |
| Resource Lease wording overclaimed protection for Direct work. | Lease guarantees now cover only Runtime-admitted write-capable Capability invocations; Direct is explicitly outside that guarantee. |
| External dispatch crash windows were underspecified. | The protocol now persists Grant issuance, requires durable Host preparation, commits `DISPATCH_AUTHORIZED`, and treats post-authorization ambiguity as `EXECUTION_UNCERTAIN`. |
| Child Capability narrowing relied on an undefined Capability hierarchy. | Parent Grants now contain a closed delegation allow-list; child effects, resources, and onward delegation must also narrow. |
| Pre-parity Go migration language could imply early authority. | Go is explicitly non-authoritative shadow code until command-level Bash parity. |
| The manual tracker could be mistaken for future authoritative Runtime State. | It is now labeled a Policy-Plane design tracker and declared non-authoritative, one-way provenance. |
| The draft incorrectly inferred that ECC's specialist strengths prevented it from owning a complete lifecycle. | Restored `ECC-FULL` as an explicit alias to `oaw/ecc-engineering`; kept `oaw/hardening` as a separate composed Recipe. Eligibility now depends on verified lifecycle Capability coverage, as it does for every Provider. |
| The first real Host and conformance timing were unclear. | Host promotion is deferred to the Host-audit migration phase after official-capability audit and conformance. |

## Final Assessment

The corrected specification consistently models OAW as a dual-plane engineering
runtime: Policy remains portable; Runtime admits Capabilities and transitions
without claiming Host sandbox authority. Provider discovery is extensible and
declarative, Profile Recipes are user-configurable control graphs, Workflow
selection is explicit, and built-in Superpowers/Matt/ECC support is ordinary
catalog data plus OAW Recipes. ECC can own `oaw/ecc-engineering` as a complete
lifecycle or provide bounded Capabilities to another Recipe; neither role is
inferred from its comparative strengths.

The design is ready for user review. It is not yet an executable implementation
plan and authorizes no Go or Policy vNext implementation work.
