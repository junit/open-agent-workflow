# Open Agent Workflow Domain Glossary

## Core Terms

### Open Agent Workflow (OAW)

The provider-neutral governance layer that selects and locks an engineering
workflow for a task, then adapts that decision to supported agent tools.

### Workflow Family

A coherent engineering methodology with its own discovery, planning,
implementation, testing, debugging, review, and completion procedures.
Superpowers, Matt Pocock skills, and Everything Claude Code are the initial
families compared by OAW.

### Provider

An independently installed source of workflow skills or agents. OAW detects
providers and routes to them but does not distribute or update them.

### Lifecycle Profile

A complete, user-selected ownership policy for one engineering deliverable.
A profile assigns exactly one owner to every applicable lifecycle stage.

### Lifecycle Lock

The task-scoped commitment to a selected lifecycle profile. It persists across
follow-ups, context compaction, and delegated work until the deliverable ends
or the user explicitly changes it at a stable boundary.

### Lifecycle Owner

The workflow family responsible for coordinating a complete task lifecycle in
a full-family profile.

### Stage Owner

The single workflow family responsible for one lifecycle responsibility in a
hybrid profile.

### Bundle

The fully named lifecycle profile plus its exact bounded specialist add-ons.
A bundle is inherited by delegated agents without re-running profile
selection.

### Bounded Add-on

An explicitly selected specialist capability from a non-owning provider. It
may produce only its declared deliverable and never becomes a lifecycle owner.

### Adapter

A translation from OAW's canonical governance policy into the instruction
surface understood by one agent tool.

### Core Adapter

An adapter whose user-level instruction surface is stable enough for OAW to
support as a default global installation target.

### Extension Adapter

An adapter supported primarily at project scope because its global surface is
GUI-managed, platform-specific, experimental, or less stable.

### Target

One supported agent tool selected for an install lifecycle operation.

### Scope

The boundary at which an adapter is installed. User scope applies across the
user's projects; project scope applies only to one repository.

### Canonical Rule Source

The single OAW-owned policy artifact from which all adapter entrypoints derive
their lifecycle behavior.

### Thin Entrypoint

A small target-native instruction that imports, references, or directs the
agent to the canonical rule source without duplicating the complete policy.

### Managed Block

A mechanically delimited section owned by OAW inside a user-owned instruction
file. Delimiters support safe updates and removal; they are not semantic
precedence controls for the agent model.

### Install State

The OAW-owned record of installed version, targets, scopes, destinations,
checksums, and recoverable backups.

### Drift

A difference between recorded OAW-managed content and the content currently on
disk. Drift is reported and blocks mutation unless the user explicitly forces
a backed-up operation.

### Stable Boundary

A task transition at which the user may safely change a lifecycle profile,
such as an approved specification, a completed ticket, a completed debugging
bundle, or a completed review loop.

