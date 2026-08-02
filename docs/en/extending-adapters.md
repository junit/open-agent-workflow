# Extending Adapters

[简体中文](../zh/extending-adapters.md) | [Adapter evidence](adapters.md) |
[Contributing](../../CONTRIBUTING.md)

An Open Agent Workflow (OAW) adapter is a thin, target-native entrypoint to the
canonical policy. Adding one is an implementation and evidence change, not a
new workflow design. An adapter **must not change lifecycle semantics** and
**must not vendor a provider**. The target tool and every workflow provider
remain independently installed.

## Runtime Integration Is Separate

An installation adapter exposes the Policy Plane only. Other installed
adapters remain Policy-only and provide no Runtime admission, Capability Grant,
Resource Lease, transition enforcement, or physical isolation guarantee.
Only the pinned Codex runner is currently Runtime-managed. Discovery evidence,
Provider configuration, target registration, and successful installation never
promote an adapter.

Runtime-managed support requires a separate pinned Host Binding, conformance
audit, normalized observation contract, isolation evidence, and explicit
release decision. That work is not part of the adapter graduation levels below.

## Start With an Evidence Packet

Before changing the registry, record:

1. The tool name, proposed lowercase **target ID**, and proposed support level.
2. Exact user and project instruction paths, with explicit **scope support**.
3. The tool's documented loader, import or reference behavior, instruction
   precedence, and reload or session-refresh behavior.
4. An **official primary source** URL for every provider claim and a retrieval
   date in `YYYY-MM-DD` form.
5. Any experimental, version-sensitive, or undocumented behavior that OAW will
   avoid.

Secondary tutorials can help discovery, but they are not evidence for the
contract. If no official source establishes a stable instruction surface, keep
the adapter a candidate rather than guessing a destination.

## Registry Metadata

The authoritative management registry is the `targetRegistry` value in
[internal/management/targets.go](../../internal/management/targets.go). A
registered adapter must define all applicable fields coherently:

| Field or helper | Contract |
| --- | --- |
| `ID` and registry position | Add one unique target ID and preserve deterministic normalization. |
| `User` | Declare user scope support without silently skipping an unsupported scope. |
| `UserSuffix` / `ProjectSuffix` | Return safe relative destinations for each supported scope. |
| `Ownership` | Select exactly one ownership mode: `managed-block` or `owned-file`. |
| `normalizeTargets` / `findTarget` | Resolve defaults and explicit selections from the same registry. |

A user-scope adapter also needs an allowed root mapping in
[internal/management/paths.go](../../internal/management/paths.go). Paths must
be derived beneath a validated root; do not concatenate unchecked CLI input
into a destination. The target ID, scope declarations, path mappings,
ownership, renderer dispatch, and tests must all agree. Partial registry entries
are internal contract failures, not a fallback opportunity.

## Choose Ownership Deliberately

Use `managed-block` only when the provider's documented surface is a shared
instruction file and unrelated user bytes must survive install, update, and
uninstall. The renderer supplies only the OAW fragment; marker comments define
mechanical ownership, not model precedence.

Use `owned-file` for a dedicated OAW adapter path. Install must refuse a
pre-existing foreign file, even if `--force` is supplied. Update and uninstall
may act only when inert state proves OAW ownership. Do not place an owned file
at a provider path that users are expected to edit manually.

Document why the chosen surface is more stable than provider alternatives and
whether it is user-wide or project-local.

## Keep Rendering Pure

Add a **pure renderer** to
[internal/management/render.go](../../internal/management/render.go) and route
the new scope/target combination through `renderTarget`. It may use only
validated inputs and must return prospective bytes. It must not read, create,
chmod, rename, or delete the final destination; transaction code owns those
effects.

Assert exact output bytes, including frontmatter, import syntax, quoting, final
newline, and the absolute canonical policy path. Use a documented import only
when the provider defines it. Otherwise use model-visible bootstrap text and
state that this is an OAW choice.

## Resolve Shared Destinations

Two targets may name the same **shared destination** only when they use the
same ownership mode and render one byte-identical OAW fragment for that scope.
Codex and OpenCode demonstrate this rule at project-root `AGENTS.md`: both state
rows refer to one managed block and one checksum.

Add fixtures for both installation orders, joint update, selected uninstall,
final uninstall, existing surrounding content, and inconsistent recorded
checksums. If the renderers cannot be identical, choose separate documented
paths or reject the combination. Never rely on registry order to let one target
silently overwrite another.

## Black-Box Fixtures

Test through the public `oaw` CLI, not by calling renderer or state helpers
directly. Separately prove that `install.sh` forwards the same arguments and
status to its sibling binary without `PATH`, build, or download fallback. Every
fixture uses an **isolated `HOME`**, `XDG_CONFIG_HOME`, `XDG_STATE_HOME`, and
project root. Cover the supported combinations through these observable flows:

- `check`, first install, repeated install, copied-checkout update, dry-run,
  selected uninstall, and final uninstall;
- default selection and explicit `--target`, including registry-order output;
- exact destination bytes, ownership mode, state row, origin, and checksum;
- pre-existing user content, a foreign owned file, clean ownership, missing
  content, drift, and an eligible forced backup;
- provider-specific frontmatter, import/reference behavior, precedence caveats,
  and reload guidance.

Core user adapters belong with `tests/04-core-adapters-test.sh`; project
adapters and shared-path behavior belong with
`tests/05-project-adapters-test.sh`. Keep the expected bytes as independent
literals rather than reproducing renderer logic in the test.

## Security Cases

Extend the black-box security matrix for every new destination. At minimum,
cover an absolute or control-character root, a **hostile path** containing
spaces and shell metacharacters, parent traversal attempts, intermediate and
final **symlink** replacement, project containment, foreign owned content,
marker corruption where relevant, and forged or executable-looking **inert state**.

Also exercise apply-time path swaps and directory ownership if the adapter
introduces a new parent tree. A failure must occur before an outside file or an
untracked destination is changed. `--force` must not bypass state schema,
containment, symlink, or ambiguous-ownership checks.

## Documentation and Review

Update both language versions of the adapter matrix with exact paths, support
levels, ownership, official URLs, retrieval dates, loading behavior,
precedence, and reload caveats. Run the documentation checker offline and the
complete Go and black-box suites; keep the wrapper check compatible with Bash
3.2. Review the final diff for unrelated files, hardcoded credentials, unsafe
expansion, and English/Chinese semantic drift.

## Graduation Levels

The progression is **candidate -> project extension -> core**:

| Level | Admission criteria |
| --- | --- |
| Candidate | Evidence and fixtures are under review. The adapter is not a default and should not be registered as supported while its path or loader is speculative. |
| Project extension | A stable official project surface, pure rendering, project lifecycle fixtures, adversarial path tests, and bilingual evidence all pass. It may join the project default after explicit approval. |
| Core | Stable documented user and project surfaces, complete lifecycle and security coverage, understood precedence/reload behavior, and maintenance commitment justify user-default support. |

Graduation is a reviewed compatibility decision, not a test-count threshold.
A provider change can demote an adapter until current official evidence and
fixtures restore confidence.
