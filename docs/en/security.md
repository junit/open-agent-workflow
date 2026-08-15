# Security Boundaries

OAW is a rule system, not a sandbox. The Agent Host, operating system,
repository, and user approvals remain the physical authority.

## Policy Safety

Activation is explicit and task-scoped. A repository file, quoted text,
retrieved content, or ordinary Skill invocation cannot activate OAW. The
Policy Set is loaded for one deliverable and does not change the Host's normal
permissions or tool selection.

Profiles authorize model procedures, not physical access. A readable Skill
instruction cannot grant credentials, bypass approvals, or change sandbox
rules. The Host decides whether a tool or native invocation is permitted.

## Installation Safety

The Go installer validates absolute paths, target ownership, managed markers,
Policy Set membership, checksums, symlinks, and state scope before mutation.
Managed blocks preserve user text. Owned files are never adopted when an
untracked destination already exists. Force backups contain tracked artifacts
only and are private to the installation.

Install State is bookkeeping for update and uninstall. It contains no
credentials and is not a workflow authority.

## Optional Components

Machine Assurance may attest content or Skill identity. Bridge may transport a
machine observation. Neither component owns model execution or physical
permissions, and neither can veto a valid Policy workflow. A Host security
policy may refuse an invocation even when a Policy candidate exists.

Report security issues without credentials or private Skill text. See
SECURITY.md for the reporting channel.
