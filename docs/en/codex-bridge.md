# Codex Bridge

The Codex Bridge is an optional Machine Assurance transport. It is not
required for SP-FULL, MATT-FULL, ECC-FULL, or MATT-SP-HYBRID.

## Scope

The Bridge may observe the active Codex session and attach exact content or
Skill identity to an already selected Markdown Profile. It does not select a
Profile, read or rewrite project files, invoke a Skill, manage progress,
review, verification, or completion, and it does not own permissions.

Bridge absence or failure therefore removes only an evidence overlay. The
normal Policy conversation continues with readable Skills and Host-native
tools. A Host security policy may independently refuse physical invocation.

## Boundary

Keep Bridge protocol and cache discovery in this document and its implementation
package. Do not copy those details into POLICY.md or a Profile. A route name is
not Provider provenance; any optional contract identifier describes only the
adapter-facing semantic interface.
