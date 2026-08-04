# Request Modes, Profiles, and Lifecycle Locking

[简体中文](../zh/lifecycle.md) | [README](../../README.md)

This guide explains Open Agent Workflow (OAW) behavior. It is not a second
policy. `policy/ENGINEERING.md is normative`; read the
[canonical policy](../../policy/ENGINEERING.md). If this guide differs from
that file, the policy wins.

## Three Request Modes

OAW classifies each new top-level engineering request before choosing an
execution model:

| Request Mode | Execution model | Lifecycle selection |
| --- | --- | --- |
| Direct Mode (`DIRECT`) | The Main Agent makes a small, clear, recoverable change and runs focused verification. | None |
| Bounded Mode (`BOUNDED`) | One exact Provider Capability produces one observable deliverable with declared effects, resources, and termination. | None |
| Workflow Mode (`WORKFLOW`) | A compiled Profile Recipe coordinates multiple responsibilities and stages. | Required |

Direct Mode requires a known change point, clear requirements, bounded scope,
no unresolved architecture or domain decision, no public contract or sensitive
semantic change, and a known verification seam. It creates no Lifecycle Bundle
and invokes no engineering Provider Capability.

Bounded Mode is the Atomic Skill mode. The user or a user-trusted rule selects
one exact Capability. It cannot claim planning, implementation ownership,
general review, a remediation loop, Git completion, or any lifecycle stage. A
required second Capability or broader responsibility triggers reclassification.

Workflow Mode covers unresolved requirements or root cause, domain and
architecture decisions, interacting engineering responsibilities, public
contracts, schemas, dependencies, migrations, sensitive changes, multiple
tickets, and long-lived delegation.

Only Workflow Mode runs the Startup Gate. Complexity and Risk Class still tune
recommendations and verification, but they do not activate lifecycle selection
for Direct or Bounded work.

## Workflow Startup Gate

Before Workflow lifecycle work begins, OAW:

1. reads the canonical policy;
2. performs only enough read-only inspection to classify the request;
3. states Request Mode, Complexity, Risk Class, and concrete evidence;
4. shows every eligible built-in and user-defined Profile, marks a
   recommendation, and lists exact proposed bounded add-ons;
5. waits for a blocking user choice with no timeout or silent default;
6. compiles the selected Recipe against verified Capabilities and records the
   Lifecycle Bundle.

Provider detection is diagnostic input. It never chooses a Capability or
Profile. Missing or ambiguous required Capabilities stop selection rather than
being silently omitted or replaced.

## Extensible Provider and Capability Model

Superpowers, Matt, ECC, and third-party Providers follow the same contract.
OAW ships inert descriptors for `oaw/superpowers`, `oaw/matt`, and `oaw/ecc`;
it does not install their skill content. Built-in Provider discovery remains
dynamic on the current Host.

Provider verification follows this exact chain:

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

Codex and Claude Code are independent Hosts, so shared physical files still
produce separate Host Installation identities. Descriptor bindings and
installation hints are declarations only and cannot create Host Binding
Evidence. A Policy-only Host can expose Candidates but cannot verify a Runtime
Instance. Foreign diagnostics never enter pins, Registry resolution, Profile
compilation, admission, or a Lifecycle Bundle.

Users can register trusted third-party Providers, declarative discovery
descriptors, Profile Recipes, bindings, pins, and denials in configuration.
Trusted project configuration may recommend or narrow those records, but it
cannot create user trust or expand authority. Only a verified Provider Instance
can satisfy a Recipe Capability selector.

A Provider's role is Recipe-specific. One Profile may use ECC for a complete
lifecycle; another may admit the same Provider only for build repair or a
security review. Full-family eligibility is based on verified Capability
coverage, using the same rule for built-in and user-registered Providers.

### Inspecting Provider resolution

`oaw catalog list providers` shows declared Provider descriptors. It does not
show installed Provider Instances. Inspect dynamic discovery and verification
for the selected Host with:

```bash
oaw providers inspect --host codex --format text
```

The command is read-only. For an ambiguous current-Host Provider it lists every
Candidate and an exact Host-scoped pin:

```toml
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-<sha256>"
evidence_digest = "<sha256>"
# location = "/exact/physical/path"
# version = "6.1.1"
```

OAW never chooses a Candidate or writes the pin. The active contracts reject
`oaw.provider-descriptor/v1` and `oaw.user-config/v1` instead of migrating
them. `HOST_BINDING_EVIDENCE_REQUIRED`, `PROVIDER_BINDING_UNAVAILABLE`,
`PROVIDER_FOREIGN_HOST_ONLY`, `PROVIDER_PIN_INCOMPATIBLE`, and
`HOST_PROVIDER_SCOPE_MISMATCH` retain stable meanings across inspection and
Runtime denial. After changing user configuration, begin a new Run so it
captures the new Configuration Snapshot.

## Built-in and User-Defined Profiles

The built-in Profile aliases remain stable:

| Selection | Recipe | Ownership contract |
| --- | --- | --- |
| `SP-FULL` | `oaw/delivery` | Superpowers owns the complete delivery lifecycle when all required Capabilities verify. |
| `MATT-FULL` | `oaw/domain-engineering` | Matt owns the complete domain-engineering lifecycle when all required Capabilities verify. |
| `ECC-FULL` | `oaw/ecc-engineering` | ECC owns the complete engineering lifecycle when all required Capabilities verify. |
| `MATT-SP-HYBRID` | `oaw/reliable-feature` | Matt and Superpowers use the fixed responsibility map below; exact ECC specialists remain bounded. |
| `USER-DEFINED` | configured Recipe ID | This is a selection action for a versioned user-defined Recipe, not a fifth built-in Profile. |

`ECC-FULL` includes discovery, specification and planning, implementation,
testing, debugging and build repair, review, delegation, verification, and
completion. ECC is therefore not reduced to hardening. Its specialist role in
another Recipe does not weaken the complete `oaw/ecc-engineering` option.

A user-defined Recipe must compile to exactly one owner for every applicable
responsibility, explicit transitions and terminal gates, bounded add-ons, and
effects within trusted authority. An ambiguous Recipe is rejected rather than
repaired by guessing.

## Matt-Superpowers Stage Map

`MATT-SP-HYBRID` assigns one owner to each responsibility:

| Responsibility | Owner |
| --- | --- |
| Requirements and domain modeling | Matt |
| Product specification and acceptance criteria | Matt |
| Test-seam selection and ticket decomposition | Matt |
| Per-ticket executable implementation plan | Superpowers `writing-plans` |
| Workspace and Git setup | Superpowers |
| Implementation orchestration and code changes | One Superpowers executor |
| TDD method and red-green loop | Matt `tdd` |
| Functional and hard-bug debugging | Matt `diagnosing-bugs` |
| Build, dependency, and type repair | Selected ECC Incident Handler, or none |
| Spec compliance and code-quality review | Superpowers |
| Review remediation and re-review | Superpowers |
| Fresh verification and branch completion | Superpowers |
| Specialist checks | Only explicitly selected bounded add-ons |

Matt specifications and tickets remain canonical for requirements and delivery
edges. A Superpowers plan may add paths, commands, code steps, and expected
results without changing those requirements or ticket boundaries.

Matt `tdd` is the only TDD procedure in this hybrid. An expected RED test stays
inside that loop. An unexpected functional failure transfers to Matt debugging
with the intended state, command, and output. A strict build, dependency, or
type failure may route only to the selected ECC Incident Handler.

## Isolation and Host Guarantees

On a Runtime-managed Host, Workflow Capabilities marked `isolated-required`
run in separate Executor contexts. This bundle inheritance gives every
Executor the exact Profile, Bundle generation, active graph node, admitted Capability,
allowed effects and resources, termination condition, and evidence
requirements. The Executor cannot reopen family arbitration or add a second
owner.

On a Policy-only Host, the same lifecycle lock and allowed-action map are
instruction-level coordination. OAW cannot claim Runtime admission, Grants,
Resource Leases, transition enforcement, or physical isolation there. Host or
agent behavior may provide additional isolation, but it is not an OAW Runtime
guarantee.

## Lifecycle Lock, Inheritance, and Add-ons

The lifecycle lock records task and deliverable identity, classification,
selected Profile, selection source, Bundle generation/digest, stage owners,
exact add-ons, active stage, active ticket, allowed and blocked actions, and
canonical artifact or evidence references.

The lock persists across follow-ups, context compaction, and delegated work.
Bundle inheritance applies it to every dispatched Executor. For multi-ticket
work, ticket inheritance keeps the same Bundle until the user switches at a
stable boundary.

bounded add-ons authorize one exact specialist deliverable. For example,
`ECC(security-review)` may return a digest-pinned report, but it does not own
implementation, general review, Git work, or completion. Security and coverage
requirements are constraints, not lifecycle selections.

## Runtime Project Projections

Runtime-managed project workflow files are human-readable downstream views of
committed Runtime State. A projection includes the selected Profile, Bundle
generation, stage, active ticket, digest-pinned evidence references, and lag
status. It excludes credentials, full Grants, sensitive evidence content, and
raw Provider output.

The optional active ticket is an independent delivery-tracking reference. It
is never inferred from or used as an alias for the Workflow Deliverable ID.

Projection files are never parsed back as authority. A projection write
failure records lag and never rolls back the committed Runtime revision.

## Stable Switching

Only the user can change a selected Workflow Profile. stable switching is
allowed at an approved specification, between completed tickets, after a
completed TDD or debugging cycle, after review, or after recorded verification.
A stable-boundary switch is not allowed during an active Capability invocation,
delegated work, an unresolved merge, or an incomplete red-green cycle.

A switch compiles a new Bundle generation, revokes outstanding Grants from the
old generation, and preserves valid artifacts and evidence. It never rewrites
completed ownership or silently substitutes a Provider.

## Complete Workflow Example

A repository needs a multi-ticket installer with path containment, recoverable
force, bilingual docs, and a final security assessment. The public behavior,
filesystem risk, several tickets, and remediation loop classify it as Workflow
Mode. OAW recommends the hybrid and proposes one bounded add-on.

The user makes this explicit blocking choice:

```text
MATT-SP-HYBRID + ECC(security-review)
```

The compiled Bundle assigns Matt to requirements, specification, ticketing,
TDD, and functional debugging; Superpowers to executable plans,
implementation, review, remediation, verification, and completion; and ECC
only to the security report. Each Executor inherits the Bundle and active
ticket. The ECC report returns to Superpowers remediation without taking over
the lifecycle.

After one ticket is verified, the user may make a stable-boundary switch to
another eligible built-in or user-defined Profile. Until that explicit choice,
the original Bundle remains locked.

The [background](background.md) explains the motivation, and the
[comparison](comparison.md) records the experience-based inputs behind the
initial hybrid.
