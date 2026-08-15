# Open Agent Workflow Domain

Open Agent Workflow (OAW) is a rule-driven engineering workflow for Agent
Hosts. The static Policy Set is the complete product path. Optional executables
may add machine evidence, but normal engineering work does not depend on them.

## Policy Product

**Policy**: Portable rules for activation, Profile selection, engineering
defaults, safety, review, verification, and physical authority boundaries.

**Canonical Policy Set**: One versioned set containing `POLICY.md`,
`cooperative-protocol.md`, Built-in Profiles, and Host Adapter guidance.

**Activation Router**: A small Host instruction that loads OAW only when the
user explicitly asks OAW to govern a deliverable.

**Deliverable**: One coherent engineering outcome handled under one selected
Profile.

**Profile**: A Markdown engineering method that maps Responsibilities to
Skills, Host-native actions, and task-specific rules.

**Built-in Profile**: A Profile shipped in the Canonical Policy Set. The current
set contains `SP-FULL`, `MATT-FULL`, `ECC-FULL`, and `MATT-SP-HYBRID`.

**Custom Profile**: A project- or user-owned Markdown Profile composed from
currently available Skills. It uses the same contract as a Built-in Profile.

**Responsibility**: A stable outcome area: problem framing, specification,
planning, workspace preparation, implementation, testing, review,
verification, or closeout.

**Policy Default**: Model-native behavior supplied by Policy when a Profile
does not assign a Skill or special rule to a Responsibility.

**Skill**: An independently installed, model-readable procedure. A selected
Profile assigns a Skill only the Responsibilities named by that Profile.

**Skill Availability**: The model's present ability to read a Skill document
or use a Host-native invocation. Index and scanner output are advisory.

**Profile Selection**: The user's explicit choice, or the model's stated choice
when no real ambiguity requires a question.

**Add-on**: A task-scoped specialist Skill that does not take ownership of the
Profile's core Responsibilities.

**Progress Note**: An optional Markdown continuity aid. Conversation remains
the primary progress record; a Progress Note is not control state.

**Complexity**: A qualitative model judgment that scales decomposition,
planning, and continuity detail.

**Risk**: A qualitative model judgment that scales safeguards, approval,
review, and verification.

## Host And Installation

**Host**: The Agent environment that owns model execution, Skills, Agents,
tools, credentials, plugins, sandboxing, approvals, and physical effects.

**Host Adapter**: Host-specific loading, discovery, and installation guidance.
It cannot change portable Policy or Profile semantics.

**Project Policy Set**: A self-contained Canonical Policy Set under
`.oaw/policy/`. It takes precedence over a User Policy Set without merging.

**User Policy Set**: A Canonical Policy Set under the user's OAW configuration
root. It is used only when the current project has no Project Policy Set.

**Managed Block**: A delimited OAW-owned section in a Host instruction file.
It points to the selected Policy Set while preserving surrounding user content.

**Owned File**: An installer-created file whose complete contents belong to one
OAW installation. Existing untracked files are never adopted.

**Install State**: Private bookkeeping for safe check, update, backup, and
uninstall operations. It contains no workflow progress or credentials.

**Physical Authority Boundary**: The boundary at which the Host, operating
system, repository, and user approval remain authoritative.

## Optional Machine Evidence

**Machine Assurance**: The standalone `oaw-assurance` component. It issues and
verifies machine-readable claims for one exact Policy Profile snapshot.

**Assurance Overlay**: A content-addressed record that binds exact Profile
occurrences to exact Provider and Host Binding evidence. It cannot select a
Profile, change its rules, or claim execution or completion.

**Provider**: An independently installed collection of Skills or tools whose
identity may be described by Machine Assurance.

**Binding**: A machine claim connecting one Profile occurrence to an exact
Provider and Host invocation surface.

**Bridge**: The standalone `oaw-bridge` Codex integration. It observes
secret-free current Host facts for Assurance. It does not invoke Skills,
manage progress, or enforce permissions.

## Invariants

1. A selected Policy Set, Profile, readable Skills, and Host-native abilities
   are sufficient for normal delivery without an OAW executable.
2. Optional components may add evidence but cannot make a Policy-valid workflow
   unavailable.
3. Markdown Profiles are the only engineering-method authority.
4. OAW never starts a model process or expands Host permissions.
5. Project and User Policy Sets are selected by precedence and never merged.
6. Custom Profiles and Built-in Profiles use one extension contract.
