# Extending Adapters

[简体中文](../zh/extending-adapters.md) | [Adapter evidence](adapters.md) |
[Contributing](../../CONTRIBUTING.md)

An Open Agent Workflow (OAW) adapter is a thin, target-native entrypoint to the
canonical policy. Adding one is an implementation and evidence change, not a
new workflow design. An adapter **must not change lifecycle semantics** and
**must not vendor a provider**. The target tool and every workflow provider
remain independently installed.

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

The registry in [lib/targets.sh](../../lib/targets.sh) is executable metadata.
A registered adapter must define all applicable entries coherently:

| Function | Contract |
| --- | --- |
| `target_ids` | Adds the unique target ID at one stable registry position. |
| `target_is_known` and `target_registry_position` | Recognize the same ID and preserve deterministic normalization. |
| `target_supports_user` / `target_supports_project` | Declare scope support without silently skipping an unsupported scope. |
| `target_ownership` | Select exactly one ownership mode: `managed-block` or `owned-file`. |
| `target_project_relative_path` | Return one safe relative project destination when project scope is supported. |
| `default_targets` | Include the target only after its approved support level makes it a default. |

A user-scope adapter also needs a user allowed root and relative suffix in
[lib/paths.sh](../../lib/paths.sh). Paths must be derived beneath a validated
root; do not concatenate unchecked CLI input into a destination. The target ID,
scope declarations, path mappings, ownership, renderer dispatch, and tests must
all agree. Partial registry entries are internal contract failures, not a
fallback opportunity.

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

Add a **pure renderer** to [lib/render.sh](../../lib/render.sh) and route the
new scope/target combination through `render_target_content`. It may use only
validated inputs and must write prospective bytes to standard output or a
caller-provided temporary file. It must not read, create, chmod, rename, or
delete the final destination; transaction code owns those effects.

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

Test through `install.sh`, not by calling renderer or state helpers directly.
Every fixture uses an isolated `HOME`, `XDG_CONFIG_HOME`, `XDG_STATE_HOME`, and
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
complete shell suite under Bash 3.2. Review the final diff for unrelated files,
hardcoded credentials, unsafe expansion, and English/Chinese semantic drift.

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
