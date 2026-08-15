# Installer

The installer manages one Canonical Policy Set at user or project scope. It
does not install Skills, inspect their contents, invoke a model, or create
workflow execution state.

## Commands

~~~text
oaw check [--project PATH] [--target IDS]
oaw install [--project PATH] [--target IDS] [--dry-run] [--force]
oaw update [--project PATH] [--target IDS] [--dry-run] [--force]
oaw uninstall [--project PATH] [--target IDS] [--dry-run] [--force]
oaw profile list
oaw profile show SOURCE:ID
oaw profile check SOURCE:ID
~~~

User installation defaults to the user Host instruction files. Project
installation writes a self-contained set under PATH/.oaw/policy and project
adapter files. A project set takes precedence over a user set without merging.

## Ownership

Managed blocks preserve surrounding Host instructions. Owned files are created
only when the destination is absent. Install State records the exact Policy Set
files, target records, checksums, scope, and directories owned by that
installation. Install State is private bookkeeping for safe update and
uninstall; it is not workflow progress.

Install, update, and uninstall validate every path and source before writing.
Force mode may create a private backup when tracked content drifted. It never
adopts untracked files and never changes another scope's installation.

## Wrapper and Releases

install.sh is an offline wrapper that resolves only a sibling oaw or oaw.exe.
It never searches PATH for a different executable, downloads a release, or
builds code. Release archives contain the precompiled binary, wrapper, Policy
documentation, and checksums.

Build from source with go build -o ./oaw ./cmd/oaw. Verify a release with the
published SHA256SUMS before executing it.
