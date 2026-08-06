# Installer Reference

[简体中文](../zh/installer.md) | [README](../../README.md) |
[Architecture](architecture.md)

Public installation management is Go-authoritative. A release archive already
contains a precompiled `oaw` or `oaw.exe`; after verifying its checksum, use the
binary directly:

```text
./oaw check
./oaw install
./oaw update
./oaw uninstall
```

From a source checkout, build the binary whose embedded policy and version you
intend to use before running a command:

```text
go build -o ./oaw ./cmd/oaw
./oaw check
```

`install.sh` is an offline sibling-binary compatibility wrapper. It executes
only `oaw` or `oaw.exe` from its own directory; it does not search `PATH`, build
a binary, fetch a release, or download executable code. The compatibility forms
are:

```text
./install.sh check
./install.sh install
./install.sh update
./install.sh uninstall
```

Release archives contain precompiled binaries and perform no runtime executable
download. Commands emit human-readable output. Machine-readable management
status is outside the v0.1 contract. Running either entrypoint without arguments
or with `help`, `-h`, or `--help` prints help and exits 0. A command-scoped help
request, such as `./install.sh install --help`, does the same without mutation.
If the wrapper's sibling binary is missing or not executable, it exits 70.

## Syntax and Options

```text
./oaw <check|install|update|uninstall> [options]
./install.sh <check|install|update|uninstall> [options]

--target <ids>       comma-separated target IDs
--target=<ids>        equivalent inline form
--project <path>     operate on one physical project root
--project=<path>      equivalent inline form
--dry-run             prepare and report without persistent writes
--force               recover eligible recorded drift after backup
-h, --help            show help
```

`--target` accepts IDs separated by commas. Empty entries and unknown IDs are
usage errors. Duplicates collapse, and the selected IDs are normalized to
**registry order**, regardless of input order.

Without `--project`, the command uses user scope. With `--project`, OAW resolves
the path to a physical root and uses isolated project state named
`<crc>-<bytes>.state`. A project target must remain contained by that root.

`check` is read-only and rejects `--dry-run` and `--force`. The three mutation
commands accept `--dry-run`. `--force` can recover eligible recorded drift on
`update` or `uninstall`; it never adopts an untracked file as OAW-owned.

## Target Defaults

When `--target` is omitted, defaults depend on scope:

| Scope | Default target IDs |
| --- | --- |
| User | `claude,codex,gemini,opencode` |
| Project | `claude,codex,gemini,opencode,cursor,windsurf,cline,roo,copilot` |

Only Claude, Codex, Gemini, and OpenCode have user destinations. Asking for a
project-only target in user scope is rejected instead of silently skipped. The
[adapter matrix](adapters.md) lists every destination and ownership mode.

## Command Behavior

### `check`

`check` validates the embedded management source, selected scope, Install State,
policy, and target ownership without writing. It reports whether installation
is absent, clean, drifted, invalid, or otherwise unsafe. `check` exits 0 after
reporting these human-readable statuses, including drift or invalid state. That
status does not authorize mutation: a later mutation still validates ownership
and exits 65 when the reported problem has not been resolved or forced safely.

#### Public Go management boundary

`oaw check` and the three mutation commands use the same public production binary.
The compatibility wrapper reaches that exact implementation by `exec`;
there is no second Bash management implementation in a release. Pre-cutover
Bash behavior remains only as an independent test oracle in the repository.

### `install`

`install` renders policy adapters from the running binary's embedded source,
prepares the complete operation, and creates the scope's state record after
applying the targets. Existing foreign content at an owned-file destination or
conflicting managed ownership is refused, including with `--force`.

A normal `install` creates no operation backup, including when `--force` is
present. Extending or coordinating valid Install State preserves any existing valid `backup` reference, while a rejected install changes neither state nor the backup tree.

### `update`

`update` requires an existing valid installation record. The binary embeds the
policy, version, registry metadata, and renderer behavior built from the
**current checkout**. Rebuild `./oaw` after changing source checkout artifacts;
a release archive already contains the release Policy and Version. There is no network
fetch or hidden release selection. `--target` limits which
already-installed targets are refreshed; `update` does not add or remove
targets. Selecting a target that is not installed exits 65. Use `install` to
add and `uninstall` to remove targets.

### `uninstall`

With valid installation state, `uninstall` **removes only clean OAW ownership**:
a clean managed block is removed from its host file and a clean OAW-owned file
is deleted. It prunes **only OAW-created empty directories** recorded in state.
It does not delete surrounding user content, non-empty directories, provider
installations, or files whose ownership has drifted. An `uninstall` without
state is a successful no-op after confirming that selected managed-block
destinations contain no untracked OAW markers.

If a managed destination differs from the recorded checksum or expected OAW
fragment, **drift exits 65** before normal mutation. `--force` does not erase
that history silently: every affected artifact is placed in a verified backup
before apply.

For `update` and `uninstall`, the Go mutation journal records an inverse after
each applied effect. An apply failure attempts to restore changed destinations
and removed owned directories in reverse order. A rollback failure is reported
as status 74 and requires manual recovery. Forced operations also complete a
verified operation backup before the first mutation, so the backup remains the
auditable recovery source even when automatic rollback cannot finish.

## Dry Run

For `install`, `update`, and `uninstall`, `--dry-run` executes argument parsing,
path derivation, rendering, state validation, drift detection, and operation
preparation. It **writes no managed content, state, backups, or directories**.
The report describes the actions that a real run would attempt.

A dry run is not a reservation. A later real invocation repeats validation,
and concurrent filesystem changes can make that invocation fail.

## State and Backups

User and project installations never share a state file. Different physical
project roots also receive different state files. The installed policy is
stored under the XDG config root, while state and operation backups are stored
under the XDG state root; see the [architecture guide](architecture.md) for
exact paths and the record schema.

Install State and Workflow State are disjoint; no automatic migration occurs.
Management commands neither create Workflow State nor import existing
policy-only tasks or Profile locks. Starting coordination or switching at a
Stable Boundary is an explicit Workflow action, not a side effect of `install`,
`update`, or `uninstall`.

A normal `install` creates no operation backup. Clean `update` and `uninstall`
need not create one. A forced `update` or `uninstall` creates an
operation-scoped, verified backup before any prepared destination is changed.
Each destination uses atomic replacement. A later apply failure triggers the
Go mutation journal's best-effort whole-operation rollback; a rollback failure
is explicit and leaves the verified backup available for manual recovery.

## Exit Codes

The complete v0.1 exit set is **0, 64, 65, 66, 69, 70, 73, and 74**:

| Code | Meaning |
| --- | --- |
| `0` | Success, help, or a successful no-op/check result. |
| `64` | Invalid command, option, scope, root, or target selection. |
| `65` | Drift, invalid state, containment or symlink failure, or unsafe ownership. |
| `66` | `update` was requested without installation state. |
| `69` | Unsupported/internal target or renderer-contract failure. |
| `70` | Required local source such as `VERSION` or the checkout policy is unreadable or invalid. |
| `73` | Temporary workspace or filesystem creation failed. |
| `74` | Backup creation or verification encountered an I/O failure. |

Scripts should treat every undocumented nonzero result as failure and retain
stderr for diagnosis. Exit codes identify the failure class; they are not a
machine-readable state schema.

## Examples

```bash
# Inspect the default user installation.
./install.sh check

# Preview a project installation without creating files or state.
./install.sh install --project /path/to/repository --dry-run

# Install two user targets; output is normalized to registry order.
./install.sh install --target=opencode,claude

# Update an existing project installation from the running binary.
./install.sh update --project=/path/to/repository

# Preserve drifted artifacts in a backup before forced removal.
./install.sh uninstall --project /path/to/repository --force
```
