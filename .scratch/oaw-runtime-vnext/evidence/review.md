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

## Ticket 03 Implementation Review

**Date:** 2026-08-01

**Fixed point:** `5104a01`

**Scope:** Deterministic Request Classifier

**Result:** Passed after scoped corrections

The implementation diff was reviewed separately against repository standards
and Ticket 03's issue, Runtime specification, and executable plan. The review
found no unresolved Critical or Important issues.

Corrections made before the final result:

- Bounded selector evidence no longer raises an otherwise valid Bounded request
  to Workflow; it remains an admission requirement.
- User and project policy layers are composed before application, making mode,
  risk, evidence, reasons, and decision digests order independent and monotonic.
- Typed proposals now enforce the same collection and reference limits as the
  JSON Schema, and raw proposal bytes must be valid UTF-8.
- Classification and validation were split into focused functions; unused
  Request Mode aliases and duplicated selector handling were removed.
- Exhaustive policy invariants and a fails-closed fuzz seam were added alongside
  the critical-release corpus.

The final diff does not parse request prose, call a model or network API, select
a Provider or Capability, issue authority, mutate Runtime State, or add a CLI
surface. The only selector retained in a decision is explicit Bounded intent;
Workflow decisions carry no Capability selector.

## Ticket 04 Implementation Review

**Date:** 2026-08-02

**Fixed point:** `ebc04a8`

**Scope:** Profile Recipe Compiler and deterministic Execution Graphs

**Review owner:** Superpowers (inline main-agent review; no subagents)

**Result:** Passed

The diff was checked against Ticket 04, specification section 9, and the
catalog/registry contracts. The compiler accepts a read-only `CatalogSource`
and `EffectiveRegistry`, resolves aliases and custom Recipes through one path,
and never discovers or executes Providers. Nodes are admitted only when the
queried Provider Instance and Capability identities match and the verified Host
Binding is declared by the selected descriptor. Provider brand names do not
participate in eligibility.

The graph is defensively immutable and pins normalized Recipe/Provider digests,
Capability contract limits, Host Binding, executor topology, Procedures,
Incident Routes, terminal gates, and stable boundaries. Binding duplicates,
unknown selectors, missing verification, identity mismatch, duplicate/missing
owners, unsupported effects/resources, unsupported Request Modes, missing
targets, unreachable controls, invalid Procedures/terminals, and unclosed loops
fail with stable compiler codes. Optional missing handlers are removed together
with their routes; required nodes are never silently omitted.

No Critical or Important findings remain. The only intentional non-blocking
verification limitation is that `govulncheck` is not installed in the current
environment.

## Ticket 05 Implementation Review

**Date:** 2026-08-02

**Fixed point:** `7a7d10b`

**Scope:** Direct Runtime protocol, durable Run journal, and Direct escalation

**Review owner:** Superpowers (inline main-agent review; no subagents)

**Result:** Passed after scoped corrections

The complete diff was checked against Ticket 05, specification sections 5,
10-12, 14-15, and 18, and the executable implementation plan. No unresolved
Critical or Important issues remain.

Corrections made during review:

- START replay now consults committed state before applying current
  classification rules, so a rule change cannot replace an idempotently stored
  reply.
- Matching orphan revisions can be promoted after a crash before `HEAD`, while
  valid but logically conflicting or malformed orphans fail closed.
- Loaded Direct state now validates classification, project identity,
  processed-message ordering and revision ownership, empty authority
  collections, event/reply shape, size limits, and every pinned digest.
- Windows `HEAD` replacement uses `MoveFileEx` with replace and write-through
  flags; Unix uses atomic rename and directory sync.
- The commit path strictly reloads the immutable revision after advancing
  `HEAD`, making persisted bytes the only reply source.

The implementation creates no Lifecycle Bundle, Capability Grant, Stage Grant,
Resource Lease, Provider invocation, Profile selection, Host dispatch, or CLI
transport. Scope expansion preserves `DIRECT`/`RELEASED` and returns only the
successor-Run recovery action. Direct release diagnostics explicitly disclaim
Capability admission, Host tool-call control, and Resource Lease guarantees.

## Ticket 06 Implementation Review

**Date:** 2026-08-02

**Fixed point:** `9ecdac8`

**Scope:** Bounded admission, immutable Grant issuance, dispatch handshake,
observations, recovery, and journal hardening

**Review owner:** Superpowers (inline main-agent review; no subagents)

**Result:** Passed after scoped remediation

The review checked selector provenance, verified Provider and Binding
resolution, authority/effect/resource narrowing, Main Agent topology, Resource
Lease deferral, immutable Grant identities, reply-before-return ordering,
replay and restart behavior, Host invocation boundaries, observation
normalization, blind retry prevention, future authority-field leakage, and
Direct compatibility.

One Important finding was corrected before completion: single-revision
semantic validation allowed a later re-signed revision to rewrite an earlier
Grant or processed-message history. `validateRevisionTransition` now enforces
immutable Run identity, append-only message history, legal Bounded state edges,
and immutable Grants/observations while loading the committed chain.

No unresolved Critical or Important findings remain. Runtime never invokes a
Host Binding; it persists `DISPATCH_AUTHORIZED` before any external Host may
act. Ticket 07 Resource Leases and Ticket 08 Host Manifest/Adapter fields remain
outside the Runtime state surface.

## Ticket 07 Implementation Review

**Date:** 2026-08-02

**Fixed point:** `8665458`

**Scope:** Workflow Profile selection, immutable Lifecycle Bundles, isolated
Stage Grants, Worktree leases, graph observations, stable-boundary switching,
one-way projections, restart recovery, and Direct/Bounded compatibility

**Review owner:** Superpowers (inline main-agent review; no subagents)

**Result:** Passed after scoped remediation

The review checked Workflow-only Gate enforcement, Bundle and graph pinning,
Provider invocation boundaries, Main Agent and Host-isolation rejection,
generation-bound Grants, review Executor freshness, cross-Run lease
serialization, switch timing, projection authority, journal recovery,
permissions, and credential/raw-output persistence.

Three findings were corrected before completion:

- Projection lag references were removed from authoritative `WorkflowState`;
  failures now exist only as immutable owner-only sidecars, and Runtime never
  reads them as authority.
- Built-in Superpowers, Matt, and ECC completion capabilities now declare both
  `git-repository` and `project-worktree` resources when allowing `git-local`,
  closing the Grant resource contract.
- Explicit stable-boundary switching can now adopt the current trusted
  Configuration and Registry in the new Bundle generation. Run and Project
  identity and every old Bundle remain immutable; old Engines cannot issue new
  Grants after the switch, while an Engine holding the current trusted inputs
  can continue.

No unresolved Critical, High, or Important findings remain. Runtime invokes no
Host Binding, projection sink failures cannot change a committed reply, and
projection deletion or tampering cannot influence inspection, admission,
recovery, or Bundle switching.

## Ticket 08 Implementation Review

**Date:** 2026-08-02

**Fixed point:** `60ea7ee`

**Scope:** Trusted Host Integration records, capability audit evidence,
deterministic Adapter conformance, Configuration pinning, Workflow admission,
Lifecycle Bundle Host identities, stable switching, projection summaries, and
security/recovery integration coverage

**Review owner:** Superpowers (inline main-agent review; no subagents)

**Result:** Passed after scoped remediation

The complete `main...HEAD` diff was checked for self-attested Features,
project-granted Host trust, forged Manifest/Audit/Report digests,
instruction-only promotion, incomplete Feature/check matrices, Runtime Adapter
invocation, Binding substitution, stale Integration reuse, unsafe Bundle
switching, mutable record leakage, transcript/raw-output leakage, unbounded
inputs, unstable reason codes, Direct/Bounded regressions, and premature first
Host selection.

One Important finding was corrected before completion: a Manifest could declare
multiple Binding kinds while the conformance harness exercised only the first
sorted kind. The harness now performs two deterministic invocations for every
declared kind, verifies exact Binding delivery, evidence normalization, Bundle
inheritance, deduplication, and native receipts across the complete declared
surface, and hashes the redacted multi-Binding transcript. A RED regression
test proved that substituting only a non-first Binding kind was previously
missed and is now rejected.

No unresolved Critical, High, or Important findings remain. User configuration
is the only external Host trust source; project configuration cannot register
one. Runtime consumes immutable records and Reports but never holds or invokes
a `ConformanceAdapter`. Old Bundle generations remain byte-for-byte immutable,
and a Host change is admitted only in a new stable-boundary generation. The
nine built-in integrations remain instruction-only, so Ticket 08 selects no
first production Runtime Host.

## Ticket 11 Implementation Review

**Date:** 2026-08-02

**Fixed point:** `07f1552`

**Scope:** Go `check` shadow command, frozen Bash compatibility diagnostics,
legacy Install State health, same-fixture parity harness, and bilingual
migration documentation

**Review owner:** Superpowers (two-axis inline main-agent review; no subagents)

**Result:** Passed after scoped remediation

### Standards

The complete `9e0daca...HEAD` diff was checked against `CONTRIBUTING.md`, the
repository's Bash 3.2, bilingual documentation, black-box fixture, provider
neutrality, and read-only network rules, plus the standard code-smell baseline.
The review removed the unused `Environment.TempDir` interface and internal
resolver parameter, bounded state reads, streamed checksums, managed-block
scanning, and version-directory discovery, and rejected non-regular inputs
before opening them. No documented-standard violation or unresolved baseline
smell remains.

### Spec

The diff was checked separately against Ticket 11, the approved Runtime vNext
specification, and the executable plan. Remediation aligned hidden Provider
version matching with shell glob rules, Bash TSV `IFS` and unterminated-record
behavior, partial stdout on path errors, empty-`HOME` path concatenation,
symlinked Provider indicators, long and unterminated managed lines, and
non-regular state, target, checksum, and version-root handling. The final
command keeps Bash authoritative, does not mutate Install State, does not
enumerate arbitrary configured Providers, and does not implement Go
`install`, `update`, or `uninstall`.

No unresolved Critical, High, Important, Standards, or Spec finding remains.
Summary: Standards 0 unresolved findings; Spec 0 unresolved findings.

## Ticket 09 Implementation Review

**Date:** 2026-08-03

**Fixed point:** `0cb396d`

**Scope:** Codex Host promotion, canonical Runtime transport, `oaw run`,
bounded Host dispatch, ordered Runtime handshake, Host capability gating,
configuration/discovery loading, and migration evidence.

**Review owner:** Superpowers (two-axis inline main-agent review; no subagents)

**Result:** Passed after scoped remediation

### Standards

The complete `main...0cb396d` diff was checked against `CONTRIBUTING.md`, the
Runtime vNext spec, the executable Ticket 09 plan, repository immutability and
file-size rules, provider neutrality, no-real-model-test policy, and the
standard code-smell baseline. The review found no unresolved documented
standard violation. The changed CLI and Host code keeps bounded inputs,
defensive copies, closed outcomes, and diagnostics out of machine stdout.

### Spec and Security

The review separately checked the selected Host identity, exact Manifest and
Conformance pins, unsupported-Host fallback, strict frame/reply canonicalization,
grant ordering, isolated Executor registration, exact Binding delivery,
deduplication, cancellation, normalized evidence, raw-output exclusion, and
Bash authority boundaries.

Five Important findings were corrected before closure:

- Workflow dispatch initially used the Grant `GraphDigest` as the Host Bundle
  identity. The loop now resolves the committed `LifecycleBundle.Digest` by
  `BundleID` and fails closed when the pinned Bundle is absent; Bounded dispatch
  retains its pinned `RegistryDigest` context.
- Resumed `CONTINUE` and `INSPECT` frames did not carry a project root, so a
  project configuration could be omitted during restart. `oaw run` now accepts
  a validated `--project-root`, and rejects a mismatch with a START frame's
  project identity.
- Strict transport decoding accepted an empty or unknown frame kind before
  deferring rejection to the Engine. The shared frame validator now closes the
  kind vocabulary at decode time, with deterministic and fuzz regressions.
- Concurrent deliveries of one Invocation ID could both start a Codex process
  before either result entered the cache. Runner attempts now coalesce both
  in-flight and completed duplicate delivery, including repeated failures, and
  the race corpus proves one process call.
- Runner deduplication was initially process-local, while each `oaw run`
  invocation processes one external frame. Before Host preparation, the runner
  now inspects authoritative Runtime state: an existing observation returns the
  committed state, and a previously authorized invocation without observation
  transitions to `EXECUTION_UNCERTAIN` instead of replaying. A POSIX fixture
  executable verifies START, dispatch, replay, exact one-process execution,
  stderr separation, and project Configuration reload through the public CLI.

The review also confirmed that the existing management renderer remains
unchanged: `oaw run --host codex` performs the exact capability gate at runtime,
while unsupported adapters remain Policy-only until a later management-cutover
ticket explicitly owns that change. No unresolved Critical, High, Important,
Standards, or Spec finding remains.

Summary: Standards 0 unresolved findings; Spec/Security 0 unresolved findings.

## Ticket 12 Implementation Review

**Date:** 2026-08-02

**Fixed point:** `6f15694`

**Scope:** Go `install` rendering and Install State compatibility, immutable
prepare/apply, scoped filesystem writes, the internal shadow command,
same-path mutating parity, and bilingual authority boundaries

**Review owner:** Superpowers (two-axis inline main-agent review; no subagents)

**Result:** Passed after scoped remediation

### Standards

The complete `main...6f15694` diff was checked against `CONTRIBUTING.md`, the
repository's Bash 3.2 and bilingual-documentation contracts, the standard
code-smell baseline, and the project file-size and immutability rules. One
documented limit was corrected: `install_prepare.go` had reached 803 lines.
Pure clone/value helpers now live in focused `install_values.go`, leaving the
prepare orchestration file at 753 lines without changing behavior. The final
diff contains no unrelated generated files, secret-like values, unsafe shell
expansion, Provider mutation, or unresolved documented-standard violation.

### Spec

The diff was checked separately against Ticket 12, Runtime vNext migration
sections 15-18, ADR 0004, and the executable plan. The review covered Bash and
public-CLI authority, immutable copied source/plan values, zero-write prepare
and dry-run, full preflight, rendering and checksum bytes, shared destinations,
cross-scope state rewrites, backup-reference preservation, symlink containment,
bounded reads, modes, temporary files, output/status/order, and Ticket 13 scope.

Two Important TOCTOU findings were remediated. Opening an allowed root now
verifies that the `os.Root` handle has the same filesystem identity as the
directory inspected immediately before opening. Scoped replacement also
revalidates the complete destination snapshot after directory preparation and
again before rename, so a newly appeared or changed foreign destination is not
silently overwritten. Deterministic RED tests reproduced both missing guards
and now pass. Earlier parity remediation also preserved caller-lexical XDG
roots across state discovery, fixing repeated-separator idempotent and
cross-scope installs without changing Bash.

The final command remains internal and test-only. `install.sh` is authoritative,
public `oaw install` is not routed, install creates no operation backup, valid
existing backup references survive, and Go `update`, `uninstall`, forced
recovery, and cutover remain in Tickets 13 and 14.

No unresolved Critical, High, Important, Standards, or Spec finding remains.
Summary: Standards 0 unresolved findings; Spec 0 unresolved findings.

## Ticket 13 Implementation Review

**Date:** 2026-08-02

**Fixed point:** `9fb61d8`

**Scope:** Go `update` and `uninstall` preparation/application, forced-recovery
backups, deterministic rollback, security containment, the internal management
shadow command, same-path Bash parity, and bilingual authority boundaries

**Review owner:** Superpowers (two-axis inline main-agent review; no subagents)

**Result:** Passed after scoped remediation

### Standards

The complete `main...9fb61d8` diff was checked against `CONTRIBUTING.md`, the
repository Bash 3.2, black-box, bilingual-documentation, provider-neutrality,
file-size, immutability, and security rules, plus the standard code-smell
baseline. Repeated update/uninstall preflight and oversized orchestration
functions were split into focused preparation, target, state, policy, apply
stage, backup-read, filesystem, and CLI parsing helpers. Changed production
functions are at most 50 lines, changed files remain below 800 lines, plan and
result slices are defensively copied, and the unused `later-preflight` parity
branch was removed. No unrelated generated file, secret, Provider mutation, or
unresolved documented-standard violation remains.

### Spec

The diff was checked separately against Ticket 13, the executable plan, Runtime
vNext migration and verification sections 17-18, ADR 0004, and the Bash oracle.
The review covered public authority, zero-write preparation/dry-run, immutable
plans, status/diagnostic/output ordering, shared destinations, cross-scope
state, manual recovery, verified backup activation, rollback, user-byte and
mode preservation, directory ownership, bounded reads, symlink/TOCTOU
containment, credential leakage, and Ticket 14 scope.

Eight scoped corrections were made before completion:

- Read-only active-backup verification, mutation effects, and directory
  rollback now use existing-only roots and cannot recreate a concurrently
  removed HOME/XDG root.
- Bounded preparation reads verify the same regular-file identity across
  inspect, open, read, and final path inspection.
- Backup candidates retain root/parent/final identity and revalidate it before
  backup creation and again before manifest activation.
- `manifest.tsv` is published only after the final source revalidation.
- Owned-directory removal pins and revalidates the parent handle and final inode
  immediately before removal, then syncs the parent.
- Directory rollback rejects unsafe coordinates and changed roots/directories,
  and restores the exact original mode with explicit chmod rather than umask.
- The duplicated preparation/apply/CLI orchestration and dead parity branch
  were removed without changing the Bash-visible contract.
- Canonical-equivalent paths produced from roots with repeated separators are
  accepted only when they exactly match the safe rebuilt coordinate or its
  complete `filepath.Clean` form; true escapes and partial lexical aliases
  remain rejected.

Deterministic RED tests reproduced missing-root creation, same-size inode
replacement, same-content backup-source replacement, rollback mode loss, and
forced recovery under repeated-separator HOME/XDG roots; all now pass. The
internal driver remains test-only, `install.sh` remains
authoritative, public `oaw install/update/uninstall` routing remains closed, and
Ticket 14 alone owns cutover.

No unresolved Critical, High, Important, Standards, or Spec finding remains.
Summary: Standards 0 unresolved findings; Spec 0 unresolved findings.

## Ticket 10 Implementation Review

**Date:** 2026-08-03

**Fixed point:** `8fdd78a`

**Scope:** Runtime vNext Policy Plane semantics, extensible Provider/Profile
selection, Workflow-only Startup Gate activation, Policy-only Host limits,
Runtime project projection fields/redaction, and bilingual lifecycle guidance

**Review owner:** Superpowers (two-axis inline main-agent review; no subagents)

**Result:** Passed after scoped remediation

### Standards

The complete `4a80ae0...8fdd78a` change was checked against
`CONTRIBUTING.md`, the repository Bash 3.2, black-box, bilingual-documentation,
provider-neutrality, file-size, immutability, input-validation, and security
rules, plus the standard code-smell baseline. The Bash and Go Codex renderers
remain byte-equivalent and are exercised through the real CLI black-box seam.
Projection values are immutable copies, evidence references are validated,
sorted, and deduplicated, and changed implementation files remain focused and
below repository size limits. No unrelated generated file, Provider mutation,
credential, unsafe path expansion, or unresolved documented-standard finding
remains.

### Spec

The diff was checked separately against Ticket 10, the approved Runtime vNext
specification, canonical glossary, ADRs 0003/0004, and the executable plan. The
review covered all three Request Modes, Workflow-only Gate activation,
built-in/third-party Provider symmetry, complete ECC lifecycle eligibility,
user-defined Profile selection, Workflow isolation, Policy-only disclaimers,
all required projection fields, one-way authority, lag behavior, and exclusion
of credentials, Grants, raw output, and sensitive Evidence content.

One Important domain finding was remediated before completion. The first
projection draft used immutable `DeliverableID` as `active_ticket`, contradicting
the glossary's explicit distinction between Deliverable and Ticket. Runtime now
stores an independent optional `ActiveTicket`, projections never infer it from
Deliverable identity, and focused RED/GREEN plus invariant tests prove the
separation and reject tampering.

No unresolved Critical, High, Important, Standards, or Spec finding remains.
Summary: Standards 0 unresolved findings; Spec 0 unresolved findings.

## Ticket 14 Implementation Review

**Date:** 2026-08-03

**Fixed point:** `445340d`

**Scope:** Public Go management cutover, offline compatibility wrapper,
cross-platform release archives, Install State and Runtime State separation,
truthful Host/Policy boundaries, and publication verification

**Review owner:** Superpowers (two-axis inline main-agent review; no subagents)

**Result:** Passed; available platform verification complete, optional WSL
smoke recorded as unavailable

### Standards and Security

The complete `main...9c0f3a8` diff was checked against `CONTRIBUTING.md`, the
repository's Bash 3.2 and bilingual-documentation contracts, wrapper and
release trust boundaries, file-size and immutability rules, input validation,
path containment, and the standard code-smell baseline. The wrapper resolves
only a precompiled sibling `oaw` or `oaw.exe`; it has no `PATH`, network,
compiler, or environment-override fallback. Release construction uses private
temporary staging, refuses caller-output collisions, packages only the named
files, cross-compiles with `CGO_ENABLED=0`, and emits sorted checksums. Empty or
relative state roots, symlink redirection, changed sources, drifted state, and
unverified backup coordinates continue to fail before mutation.

No secret-like value, executable download, unrelated generated file, unsafe
shell expansion, Provider promotion, or unresolved Critical, High, Important,
or Standards finding remains. The official Go vulnerability scan found no
reachable or imported-package vulnerability.

### Spec

The diff was checked separately against Ticket 14, the executable plan, Runtime
vNext specification sections 15-19, and ADR 0004. The review covered public CLI
routing and legacy statuses, wrapper byte/stream compatibility, changed-checkout
semantics, six archive targets, checksum and archive contents, current-platform
execution, Docker Linux smoke, optional WSL detection, Policy-only retention,
disjoint Install/Runtime State, the pinned
Codex Runtime-managed boundary, and truthful publication wording.

One Spec finding was corrected before the fixed point: release verification
hardcoded the WSL archive as `linux_amd64`, which would reject a native arm64
WSL environment despite shipping `linux_arm64`. The test now selects
`linux_$(go env GOARCH)` and accepts only the two released architectures.

A second Spec finding was corrected during the environment amendment: Docker
status 125 is now mapped to non-blocking `SKIP` only after a daemon recheck
fails. A reachable daemon preserves status 125, so malformed mounts, flags, or
entrypoints remain blocking rather than being hidden as unavailable.

Public management is Go-authoritative at this fixed point, while `install.sh`
is only an offline sibling-binary compatibility entrypoint. Management neither
imports existing Install State nor adopts Policy-only tasks into Runtime State.
Only the pinned Codex runner is Runtime-managed; every other adapter remains
Policy-only. No real or model-backed `codex exec` was invoked.

No unresolved Critical, High, Important, Standards, or Spec finding remains.
Docker Linux/arm64 smoke returned status 0 with the shared Linux assertions.
This macOS host returned status 77 from the optional WSL probe, which is
recorded as unavailable and is not a completion blocker under the approved
environment-aware policy. No native Windows or WSL execution is claimed.
