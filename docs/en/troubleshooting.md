# Troubleshooting

[简体中文](../zh/troubleshooting.md) | [Installer reference](installer.md) |
[Security model](security.md)

Use the same checkout, scope, and target selection throughout diagnosis. OAW
does not fetch releases or repair workflow providers, and v0.1 output is
human-readable. Preserve the complete command and stderr when asking for help.

## Safe Diagnostic Sequence

Start read-only:

```bash
./install.sh check
```

For project scope, repeat the exact scope you intend to mutate:

```bash
./install.sh check --project /absolute/path --target claude
```

`check exits 0` after reporting `clean`, `drift`, `invalid-state`, or
`not-installed`. Read the `installed <target>:` lines; exit 0 means the check
completed, not that a later mutation is authorized.

Next preview an existing installation update:

```bash
./install.sh update --dry-run
./install.sh update --project /absolute/path --target claude --dry-run
```

`./install.sh update --dry-run` performs the same state, ownership, source, and
path preparation as update but writes no managed content, state, backup, or
directory. If no installation state exists, `update exits 66`; preview
`install --dry-run` instead, then use `install` to create the missing target.

Do not add `--force` until you have identified the changed files, understood
which bytes OAW owns, and reviewed the preview. For eligible recorded drift,
use one explicitly scoped command:

```bash
./install.sh update --project /absolute/path --target claude --force
```

The exact example above is intentionally narrow. Do not replace the project or
target with a broad guess. A normal drifted **mutation exits 65** before writes;
force is justified only when the recorded ownership and backup are acceptable.

## Read `check` Output

| Output | Meaning | Next action |
| --- | --- | --- |
| `provider <name>: missing` | A **missing provider** was not detected at its expected instruction root. | Install or repair that provider independently, or select a lifecycle bundle that does not require it. OAW never installs providers. |
| `target <id>: detected` | The target tool's instruction root was found. | This is readiness information, not proof that the running agent loaded the adapter. |
| `installed <id>: not-installed` | No valid state row owns this target. | Use `install --dry-run`, then `install`. Do not use update to add it. |
| `installed <id>: clean` | Recorded policy and target ownership match disk. | If behavior is stale, inspect provider loading and restart the target agent. |
| `installed <id>: drift` | Managed bytes, policy, or a recorded file no longer match. | Compare the destination with the state-backed expectation; preview update or uninstall before deciding on force. |
| `installed <id>: invalid-state` | State shape, binding, registry metadata, or shared ownership cannot be trusted. | Do not force. Preserve state and files for manual diagnosis. |

A **missing provider** does not necessarily stop adapter file installation, but
the selected workflow capability will remain unavailable. Provider detection
cannot choose a lifecycle profile or substitute another family.

## Clean Files but Stale Agent Behavior

Agent tools differ in precedence and reload behavior. First confirm the exact
path and loader in the [adapter matrix](adapters.md). Check whether another
user, project, nested, team, or organization instruction has higher documented
precedence.

When the provider does not document live rule reload, **restart the target agent**
or application after an install or update. For tools with a documented refresh
command, use that command and inspect the loaded context. OAW marker comments
do not force reload and do not establish model precedence.

For bootstrap adapters, confirm the running agent can read the absolute
canonical policy path. For documented-import adapters, use the provider's
context inspection facility where available. A clean OAW check cannot prove
that a provider or model honored the instruction.

## Drift and Invalid State

Drift normally means a user, tool, or checkout changed recorded OAW bytes.
Preserve the current file before editing. Run the dry-run with the same scope
and selected targets, and inspect whether it proposes `would-update`,
`would-remove`, or `would-backup`.

`--force` can repair only recoverable drift tied to valid state. It cannot:

- adopt an untracked owned file;
- repair malformed or forged state;
- follow or replace a symlink;
- escape the project or XDG containment root;
- choose between duplicate, nested, or otherwise ambiguous markers.

An ambiguous marker case may create a backup and then stop with `manual
recovery required`. That is a refusal, not a partial success. Compare the
original, expected OAW fragment, and backup before editing anything.

## Inspect and Restore Backups

A successful forced mutation prints `backup: <directory>`. A forced dry-run
prints `would-backup: <directory>` but does not create it. Under the reported
directory, open `manifest.tsv` as text. Its header records format, operation,
and scope; each `artifact` row records:

```text
artifact<TAB>original-absolute-path<TAB>backup-path<TAB>checksum
```

Verify that every original path belongs to the intended scope, that each
backup file exists, and that its checksum matches the manifest. Never source or
execute `manifest.tsv`. Stop the affected agent/tool before restoration, then
**restore backups manually**, one reviewed artifact at a time, from the listed
backup path to the listed original path. Preserve modes and re-run `check`
afterward.

There is **no operation-wide rollback**. Replacement is atomic per destination,
so a late apply failure can leave an earlier destination updated. Use the
manifest and command output to identify which artifacts require manual restore;
do not copy the entire backup directory over `HOME`, an XDG root, or a project.

## Update Problems

- `no installation state; run install first`: update exits 66. Run a scoped
  install dry-run and then install the target.
- `selected target is not installed`: update cannot add targets. Use install.
- `installed content differs from this checkout`: install found a different
  source version or policy. Run update from the checkout you intend to trust.
- Exit 70 for `VERSION` or policy: the current checkout is incomplete,
  unreadable, or invalid. OAW updates only from that checkout and never fetches
  a replacement.
- A path, containment, control-character, or symlink diagnostic: correct the
  root or filesystem layout. `--force` is not an override.

## Uninstall Refusal

An **uninstall refusal** protects content whose ownership cannot be proved.
Common causes are drift, invalid state, a changed or missing recorded target, a
symlink swap, inconsistent shared-destination checksums, or ambiguous markers.
Run `check`, then the corresponding scoped `uninstall --dry-run`. If state is
valid and drift is intentional, review an explicitly scoped forced uninstall;
otherwise use manual recovery.

Uninstall without state is a guarded successful no-op, but untracked OAW
markers still cause exit 65. A non-empty OAW-created directory is retained and
reported unchanged rather than recursively deleted. Clean uninstall never
removes surrounding user instructions or independently installed providers.

If evidence is still unclear, stop mutation and retain the checkout version,
full output, state file, destination bytes, and any backup path for a private
report under the [security policy](../../SECURITY.md).
