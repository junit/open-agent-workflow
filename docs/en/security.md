# Installer Security Model

[简体中文](../zh/security.md) | [Security policy](../../SECURITY.md) |
[Architecture](architecture.md)

This guide describes the controls and limits of the local Open Agent Workflow
(OAW) installer. It is not a claim that an untrusted checkout, operating
system, agent tool, or workflow provider is safe.

## Trust Boundaries

The installer treats these values and artifacts as trust-boundary inputs:

- the current checkout, including executable shell code, `VERSION`, and
  `policy/ENGINEERING.md`;
- CLI target and project arguments;
- `HOME`, `XDG_CONFIG_HOME`, and `XDG_STATE_HOME`;
- the physical project root and every component beneath a selected destination;
- existing policy, adapter, state, directory, and backup artifacts.

Run only a checkout you trust. OAW **does not access the network**, download a
release, install a provider, or execute content from an instruction file or
state record. This removes a remote-fetch boundary but does not make the local
checkout non-executable.

## Root, Path, and Symlink Defenses

Consumed roots must be absolute and contain no **control characters**. Project
scope is resolved with physical-directory semantics before identity and
containment checks. Registry functions provide a fixed relative suffix for
each target; empty components, `.` or `..`, absolute suffixes, and unsafe
serialization fields are rejected.

OAW validates every intermediate component and the final destination. A
**symlink** is rejected whether it points inside or outside the allowed root.
The same checks cover policy, user targets, project targets, state, backup, and
recorded cross-scope references. Project destinations must satisfy physical
root **containment**; a matching filename elsewhere is never sufficient.

Validation is repeated while creating directories, before copying a backup,
before each replacement or removal, and before pruning a directory. This
reduces path-swap and time-of-check/time-of-use exposure. It cannot stop a
process running as the **same local account** from changing files after the
last check or after an operation has returned.

## State Is Data, Not Shell

Installation state is parsed as **inert tab-separated data** and is **never sourced or evaluated**.
The parser accepts only known record types and cardinalities, safe fields,
absolute recorded paths, numeric checksum pairs, registry-order target rows,
known ownership modes and origins, consistent shared destinations, and a scope
binding that matches the selected physical project.

A syntactically valid record is not sufficient authority. Before mutation,
OAW re-derives target destinations from the registry, verifies the installed
policy and target bytes, validates recorded OAW-created directories, and checks
other live state before retaining a shared policy. Forged, stale, malformed, or
executable-looking state fails closed with exit 65. `--force` cannot override
an invalid state schema.

State files and backup artifacts are installed with mode `600`. Operation
backup directories use mode `700`. These permissions reduce accidental
cross-user disclosure, but the backups can contain user instruction files and
must still be treated as sensitive local data.

## Prepare and Apply

During the **prepare phase**, OAW renders prospective content, parses all
relevant state, verifies drift and ownership, resolves shared destinations,
and builds every file and directory action before managed writes begin. A
failure in a later target therefore prevents an earlier target from being
written during preflight.

The apply path performs **apply revalidation** against the allowed root and
expected relative suffix. Replacements use a temporary file beside the target,
set the declared mode, revalidate again, and then `mv`, providing **atomic replacement per destination**.
This is **not operation-wide atomicity**: several destinations are not one
filesystem transaction, and OAW promises no automatic rollback after a later
apply failure.

Dry-run performs preparation and reports actions but creates no managed files,
state, backups, or directories. A dry-run is not a lock; the real command
repeats validation.

## Force and Backups

`--force` is a narrow recovery mechanism for drift whose prior ownership can
still be established. It does not adopt an untracked owned file, bypass a
symlink or containment failure, accept malformed state, or guess between
ambiguous marker layouts.

Before an eligible forced update or uninstall mutates anything, OAW collects
every affected existing policy, target, and state artifact. It creates an
operation-scoped backup, copies each artifact with mode `600`, compares source
and backup checksums, writes `manifest.tsv`, and rechecks source bytes before
apply. Each `artifact` row records the original absolute path, backup path, and
checksum. Apply also confirms a changing destination is present in the active
manifest and still matches the recorded pre-mutation checksum.

If marker ownership is ambiguous, OAW creates a recovery backup when possible
and exits 65 with **manual recovery** required. It does not choose which user
bytes to delete. Users restore from backups manually by reading `manifest.tsv`;
the manifest is data and must never be executed or sourced.

## Exact Uninstall Ownership

Uninstall removes only a clean recorded managed block or a clean recorded
owned file. It preserves surrounding user bytes and does not remove a drifted
artifact without an eligible forced operation. Directories are removable only
when state records that OAW actually created them, they still resolve beneath
the allowed root, and they are empty after planned file removals. A directory
that appeared after preparation is never claimed as OAW-owned.

## Out of Scope

The installer cannot protect against:

- malicious shell code in the selected checkout;
- an operating-system or **same local account** compromise;
- unrelated software modifying allowed roots after validation;
- a provider loader ignoring instructions, applying undocumented precedence,
  or retaining a stale session;
- a model failing to follow the installed policy;
- manual restoration to the wrong path or from an unverified backup.

Use isolated roots for testing, inspect every forced dry-run, retain stderr and
the reported backup path, and stop if ownership is unclear. Report suspected
vulnerabilities through the private process in the
[security policy](../../SECURITY.md), without putting exploit details or local
configuration in a public issue.
