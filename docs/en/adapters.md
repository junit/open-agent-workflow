# Host Adapters

An adapter is installation and loading guidance for one Agent Host. It is not
an engineering method and does not add Profile Responsibilities.

## Adapter Contract

Each adapter documents the instruction file paths, scope and precedence,
managed-block or owned-file rules, reload behavior, and native Skill
invocation surfaces. It may mention a readable cache location as a fallback,
but it must not require a particular cache, lockfile, revision, or digest for
Policy operation.

The current built-in adapter is the Codex adapter. Other hosts use the same
Policy semantics through their native instruction format. Add a new adapter
under policy/adapters/ and add its target coordinates to the installer registry.

## Target Ownership

Managed-block targets preserve surrounding instructions. Owned-file targets
are OAW-created files with no user content. An adapter must declare which model
owns each destination so update and uninstall can be conservative.

## Separation

Portable rules belong in POLICY.md, cooperative-protocol.md, and Profiles.
Host paths and invocation details belong here. Machine identity and attestation
belong in optional Machine Assurance. Keeping those layers separate prevents a
Host-specific scanner from becoming a Policy dependency.
