# Codex Bridge

The Codex Bridge is an optional Machine Assurance transport. It is not
required for SP-FULL, MATT-FULL, ECC-FULL, or MATT-SP-HYBRID.

## Scope

The Bridge observes the active Codex session and reports secret-free current
Host facts to the Machine Assurance path. Machine Assurance, not the Bridge,
issues or verifies an identity Overlay for an already selected Markdown
Profile. The Bridge does not select a Profile, read or rewrite project files,
invoke a Skill, manage progress, review, verification, or completion, and it
does not own permissions.

Bridge absence or failure therefore removes only an evidence overlay. The
normal Policy conversation continues with readable Skills and Host-native
tools. A Host security policy may independently refuse physical invocation.

## Boundary

Keep Bridge protocol and cache discovery in this document and its
implementation package. Do not copy those details into `POLICY.md`, a Profile,
or the default `oaw` command. A Host observation is evidence input, not Profile
semantics or physical execution permission.
