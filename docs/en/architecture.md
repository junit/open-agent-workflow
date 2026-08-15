# Architecture

OAW has one product authority: a static, model-readable Canonical Policy Set.
The Go binary installs and checks that set; it does not decide or execute an
engineering method.

## Product Core

The Policy Set contains:

~~~text
POLICY.md                  portable rules and boundaries
cooperative-protocol.md    model-led activation and progress procedure
profiles/*.md              built-in and user-composed methods
adapters/<host>-policy.md  host-specific loading and discovery guidance
~~~

Policy defines activation, Profile selection, Responsibilities, defaults,
Skill resolution, safety, and the physical authority boundary. A Profile is
Markdown, not a compiled graph. Go diagnostics and optional evidence can
inspect it but cannot replace it.

## Ownership

The Agent Host owns model calls, Skills, Agents, tools, credentials, plugins,
MCP, hooks, sandboxing, approvals, and every physical effect. OAW never starts
a model process and never turns a logical rule into a sandbox.

Installation management owns only OAW-managed files and private Install State.
A project Policy Set is self-contained and takes precedence over a user set;
sets are not merged. Custom Profiles remain user- or project-owned.

Machine Assurance and the Bridge are optional components. They may add exact
content, Skill identity, or Host-observation evidence. They cannot select a
Profile, invoke a Skill, change Profile semantics, or veto a Policy-valid
workflow. Profile selection and physical execution permission remain separate.

## Normal Flow

1. The user explicitly activates OAW for one deliverable.
2. The model loads one Policy Set and selects a Profile, asking only when a
   genuine Profile ambiguity exists.
3. The model reads each declared Skill as its Responsibility becomes current.
4. The model performs the work through normal Host tools and records progress
   in conversation or an optional Markdown Progress Note.
5. The model reviews, verifies, and closes the deliverable according to the
   selected Profile and the Policy defaults.

Removing the binary after installation does not remove the rules from the
project or host instructions. Removing optional machine components removes only
their evidence claims.

## Extension Boundary

Host adapters may document paths, indices, native invocation names, and reload
behavior. They must not define Profile selection, progress control, or
Host-specific engineering ownership. See the extending-adapters guide and the
current architecture decisions.
