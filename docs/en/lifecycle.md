# Request Modes, Profiles, and Lifecycle Locking

[简体中文](../zh/lifecycle.md) | [README](../../README.md)

This guide explains Open Agent Workflow (OAW) behavior. It is not a second
policy. `policy/ENGINEERING.md is normative`; if this guide differs from the
[canonical policy](../../policy/ENGINEERING.md), the policy wins.

## Three Request Modes

OAW Core classifies each new top-level engineering request before selecting an
engineering method:

| Request Mode | Execution contract | Lifecycle selection |
| --- | --- | --- |
| Direct Mode (`DIRECT`) | The Main Agent makes a small, clear, recoverable change and runs focused verification. | None |
| Bounded Mode (`BOUNDED`) | One exact Provider Capability produces one observable terminal deliverable. | None |
| Workflow Mode (`WORKFLOW`) | A compiled Profile Recipe coordinates multiple responsibilities and stages. | Required |

Direct Mode requires a known change point, clear requirements, bounded scope,
no unresolved architecture or domain decision, no public contract or sensitive
semantic change, and a known verification boundary. It creates no Capability,
Profile, Lifecycle Bundle, Startup Gate, or Workflow State.

Bounded Mode is the Atomic Skill mode. The user or a user-trusted rule selects
one exact Capability with declared effects, resources, evidence, and a terminal
condition. It cannot claim planning, implementation ownership, general review,
a remediation loop, Git completion, or a lifecycle stage. A required second
Capability or broader responsibility triggers reclassification.

Workflow Mode covers unresolved requirements or root cause, domain and
architecture decisions, interacting engineering responsibilities, public
contracts, schemas, dependencies, migrations, sensitive changes, multiple
tickets, and long-lived delegation.

Only Workflow Mode runs the Startup Gate. Complexity and Risk Class tune
recommendations and verification, but they do not activate lifecycle selection
for Direct or Bounded work. `DIRECT` and `BOUNDED` do not create Workflow State.

## Workflow Startup Gate

Before Workflow lifecycle work begins, OAW:

1. reads the canonical policy;
2. performs only enough read-only inspection to classify the request;
3. states Request Mode, Complexity, Risk Class, and concrete evidence;
4. resolves verified Provider Instances through OAW Core;
5. shows every eligible built-in and user-defined Profile, eligible `CURRENT`
   or native `SUBAGENT` topology, recommendations, exclusions, and proposed
   bounded add-ons;
6. waits for a blocking user choice with no timeout or silent default; and
7. has OAW Core compile and record the immutable Lifecycle Bundle.

Provider detection is diagnostic input. It never chooses a Capability, Profile,
or topology. Missing or ambiguous required Capabilities stop selection rather
than being silently omitted or replaced.

## Provider and Capability Model

Superpowers, Matt, ECC, and third-party Providers follow the same descriptor,
binding, verification, and compiler contract. OAW ships inert descriptors for
`oaw/superpowers`, `oaw/matt`, and `oaw/ecc`; it does not install Provider
skills. Built-in Provider discovery remains dynamic on the current Host.

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
installation hints are declarations only. Foreign diagnostics never enter
pins, Profile compilation, or a Lifecycle Bundle.

Users can register trusted third-party Providers, discovery descriptors,
Profile Recipes, bindings, pins, and denials. Trusted project configuration may
recommend or narrow those records but cannot create user trust or expand
authority. Only a verified Provider Instance can satisfy a Recipe Capability
selector.

A Provider's role is Recipe-specific. One Profile may use ECC for a complete
lifecycle; another may admit the same Provider only for build repair or a
security review. Full-family eligibility is based on verified Capability
coverage, using the same rule for built-in and user-registered Providers.

### Inspecting Provider Resolution

`oaw catalog list providers` shows declared Provider descriptors. Inspect
dynamic discovery for a selected Host with:

```bash
oaw providers inspect --host codex --format text
```

An ambiguous current-Host Provider lists every Candidate and an exact pin:

```toml
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-<sha256>"
evidence_digest = "<sha256>"
# location = "/exact/physical/path"
# version = "6.1.1"
```

OAW never chooses a Candidate or writes the pin. Active contracts reject
`oaw.provider-descriptor/v1` and `oaw.user-config/v1` rather than migrating
them. Start a new Workflow after changing user configuration so OAW Core sees
the new Configuration Snapshot.

## Built-in and User-Defined Profiles

The built-in Profile aliases remain stable:

| Selection | Recipe | Ownership contract |
| --- | --- | --- |
| `SP-FULL` | `oaw/delivery` | Superpowers owns the complete delivery lifecycle when all required Capabilities verify. |
| `MATT-FULL` | `oaw/domain-engineering` | Matt owns the complete domain-engineering lifecycle when all required Capabilities verify. |
| `ECC-FULL` | `oaw/ecc-engineering` | ECC owns the complete engineering lifecycle when all required Capabilities verify. |
| `MATT-SP-HYBRID` | `oaw/reliable-feature` | Matt and Superpowers use the fixed responsibility map below; exact ECC specialists remain bounded. |
| `USER-DEFINED` | Configured Recipe ID | A selection action for a versioned user-defined Recipe, not a fifth built-in Profile. |

`ECC-FULL` includes discovery, specification and planning, implementation,
testing, debugging and build repair, review, delegation, verification, and
completion. ECC is not reduced to hardening. Its specialist role in another
Recipe does not weaken the complete `oaw/ecc-engineering` option.

A Recipe must compile to exactly one owner for every applicable responsibility,
explicit transitions and terminal gates, bounded add-ons, and effects within
trusted authority. An ambiguous Recipe is rejected rather than repaired by
guessing.

## Execution Topologies

The Startup Gate uses two topology names:

| Topology | Contract |
| --- | --- |
| `CURRENT` | Execute in the current Agent session with the active Agent Host environment unchanged. |
| `SUBAGENT` | Ask the active Agent Host to create a child through its native Subagent facility. |

Topology eligibility is the intersection of the selected Profile, Capability
bindings, Host integration, and active Host session. Missing native Subagent
support makes `SUBAGENT` unavailable; OAW never substitutes a shell, model CLI,
container, or remote process. When both are eligible, the user chooses. When
only one is eligible, OAW records the reason.

Host environment observations use `inherited`, `host-configured`, `restricted`,
`unknown`, or `unavailable`. OAW does not reconstruct or guarantee unreported
MCP, Hook, Skill, Plugin, model, authentication, sandbox, approval, or tool
behavior.

## OAW Core, Coordinator, and Agent Host

OAW Core is required and stateless. It owns classification, Host-scoped
Provider resolution, eligibility, Profile compilation, and Lifecycle Bundle
construction. Callers never author a Bundle.

The Workflow Coordinator is optional and Workflow-only. It records revisions,
idempotency, cooperative Resource Leases, Receipts, evidence, pause,
cancellation, switching, and recovery for cooperating clients. It does not
create Agents, execute models, invoke Skills, use tools, or enforce the Host
sandbox.

The Agent Host owns physical execution authority. A Lifecycle Bundle,
Capability Grant, or Resource Lease expresses logical workflow authority only.
Codex has a policy integration by default and a separate audited host-native
Bridge at `oaw/codex-host` that must be explicitly installed and trusted. The
Bridge supports `CURRENT` and `skill` bindings only; all other Host surfaces
remain unknown unless the Host reports stable evidence.

For a Codex Host-native Workflow, the evidence path is:

```text
observe_current -> Core inspect -> explicit Startup Gate
                -> Core compile / Coordinator START
                -> current Codex session executes Skills and tools
```

Other built-in integrations remain policy surfaces unless their own Host-native
integration is explicitly installed and verified. None of these logical
records transfer physical execution authority from the Agent Host.

## Matt-Superpowers Stage Map

`MATT-SP-HYBRID` assigns one owner to each responsibility:

| Responsibility | Owner |
| --- | --- |
| Requirements and domain modeling | Matt |
| Product specification and acceptance criteria | Matt |
| Test-boundary selection and ticket decomposition | Matt |
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
results without changing those requirements or ticket boundaries. Matt `tdd`
is the only TDD procedure in this hybrid.

## Lifecycle Lock, Inheritance, and Add-ons

The lifecycle lock records task and deliverable identity, classification,
selected Profile and topology, selection sources, Bundle generation and
digest, stage owners, exact add-ons, active stage and ticket, allowed and
blocked actions, and canonical artifact or evidence references.

The lock persists across follow-ups, context compaction, and delegated work.
Exact bundle inheritance gives a Host-native Subagent the active Bundle
generation, graph node, admitted Capability, effects, resources, terminal
condition, and evidence requirements. The child cannot reopen Profile
arbitration or add a second owner. For multi-ticket work, ticket inheritance
keeps the same Bundle until the user switches at a stable boundary.

bounded add-ons authorize one exact specialist deliverable. For example,
`ECC(security-review)` may return a digest-pinned report, but it does not own
implementation, general review, Git, or completion. Security and coverage
requirements are constraints, not lifecycle selections.

## Workflow State and Projections

Coordinator-backed project Workflow files are human-readable, one-way views of
committed Workflow State. A projection may contain selected Profile and
topology, Bundle generation, stage, active ticket, digest-pinned evidence
references, and lag status. It excludes credentials, full Grants, sensitive
evidence, and raw Provider output.

Projection files are never parsed back as authority. A projection write failure
records lag and does not roll back the committed revision. A policy-only lock
is likewise non-authoritative and cannot grant physical execution authority.

## Stable Switching

The stable switching rules below govern every Workflow Profile and topology
change.

Only the user can change a selected Workflow Profile or topology. stable
switching is allowed at an approved specification, between completed tickets,
after a completed TDD or debugging cycle, after review, or after recorded
verification. A stable-boundary switch is not allowed during an active
Capability invocation, delegated work, unresolved merge, or incomplete
red-green cycle.

A switch compiles a new Bundle generation, revokes outstanding Grants from the
old generation, and preserves valid artifacts and evidence. It never rewrites
completed ownership, silently substitutes a Provider, or emulates an
unavailable topology.

## Complete Workflow Example

A repository needs a multi-ticket installer with path containment, recoverable
force, bilingual docs, and a final security assessment. The public behavior,
filesystem risk, several tickets, and remediation loop classify it as Workflow
Mode. OAW recommends the hybrid, `CURRENT`, and one bounded add-on.

The user makes this explicit selection:

```text
MATT-SP-HYBRID + ECC(security-review)
```

OAW Core compiles the Bundle. Matt owns requirements, specification, tickets,
TDD, and functional debugging; Superpowers owns executable plans,
implementation, review, remediation, verification, and completion; ECC owns
only the security report. The Agent Host executes the work and returns
Receipts. The Workflow Coordinator is optional.

After a ticket is verified, the user may make a stable-boundary switch to
another eligible Profile or topology. Until that explicit choice, the original
Bundle remains locked.

The [background](background.md) explains the motivation, and the
[comparison](comparison.md) records the experience-based inputs behind the
initial hybrid.
