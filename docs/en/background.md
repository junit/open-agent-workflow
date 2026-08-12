# Why Open Agent Workflow Exists

[简体中文](../zh/background.md) | [README](../../README.md)

Open Agent Workflow (OAW) is a provider-neutral governance layer for developers
who use more than one engineering workflow and more than one coding-agent
client. OAW Core answers which contract applies and who owns each responsibility;
the optional Workflow Coordinator records durable Workflow State; the Agent
Host answers how work is physically executed in the current session or a native
Subagent. These are separate authority boundaries.

## One Task, Several Automatic Triggers

A workflow family may bring its own discovery, planning, implementation, TDD,
debugging, review, and completion procedures. Superpowers, Matt Pocock skills,
and Everything Claude Code (ECC) each cover several of those responsibilities.
When they are installed together, overlapping automatic triggers can start more
than one lifecycle for the same deliverable.

The conflict is about ownership, not whether a provider is useful. Without an
arbitration gate, one family can draft a specification while another creates a
different plan; two TDD procedures can establish different seams; or a review
tool can open a second remediation loop after another family already owns
completion. A follow-up request can also retrigger selection and silently
change methods halfway through the work.

OAW does not arbitrate ordinary Host behavior. A normal request, including
automatic Host Skill selection or direct invocation of one ordinary Skill,
remains Native Host behavior. Only explicit activation creates a task-scoped
OAW Engagement. Inside that Engagement, OAW assesses the task, shows the
applicable choices, waits for explicit user selection, and binds that selection
to the deliverable. Detection can inform the gate, but it never makes the
choice.

## One Policy, Several Agent Tools

Workflow ownership must remain stable when the same developer moves among
Claude Code, Codex CLI, Gemini CLI, OpenCode, Cursor, Windsurf, Cline, Roo Code,
and GitHub Copilot. Those tools do not share one instruction filename or one
scope model. Some have user and project instruction surfaces; others expose a
reliable project surface while global configuration is GUI-managed,
platform-specific, experimental, or less stable.

Hand-maintaining a separate policy for every client creates cross-client drift.
One file may omit a newly added profile, retain an old hybrid map, or describe
different switching rules. That difference is especially hard to notice when
each file is valid for its own tool.

OAW instead keeps one canonical rule source in its own XDG namespace. The
installer renders a small target-native Activation Router. The Router leaves
ordinary requests alone and loads the complete Policy only after explicit
activation. Adapters translate the instruction surface; they do not fork
lifecycle semantics. Mechanical marker comments establish filesystem ownership
only and do not claim model precedence.

## Provider Independence

Workflow providers remain independently installed, licensed, versioned, and
configured. OAW detects known capability indicators and routes work after the
user selects a compatible profile. It does not install, update, or remove providers,
and it does not vendor or patch their skills.

This boundary matters for both trust and maintenance:

- Users choose provider versions and installation channels themselves.
- Upstream licenses and configuration remain under upstream and user control.
- OAW can report an unavailable profile without silently substituting another
  family or omitting a required stage.
- Updating OAW never becomes permission to download or execute provider code.
- A bounded specialist add-on can contribute one declared deliverable without
  becoming a second lifecycle owner.

Agent tools are independent too. OAW installs only its policy and adapters; it
does not install the clients or mutate GUI-only global rule stores.

## Core, Coordination, and Host Boundaries

OAW owns the arbitration policy, target-specific entrypoints, OAW Core
compilation, checksummed Install State, and recoverable backups. The optional
Workflow Coordinator owns only cooperating-client Workflow State. The Agent
Host owns physical execution authority. This boundary is deliberately narrow:

1. Leave the request as Native Host unless the user explicitly activates OAW.
2. Create one OAW Engagement for one deliverable and run Assurance Preflight.
3. Classify the activated task as `DIRECT`, `BOUNDED`, or `WORKFLOW`.
4. For Workflow Mode, present all supported choices and block for explicit selection.
5. Use `policy-cooperative`, `core-backed`, or `coordinator-backed` claims truthfully.
6. Fail closed on drift and back up before an explicitly forced mutation.
7. Remove only OAW-owned artifacts during uninstall.

`DIRECT` and `BOUNDED` do not create Workflow State. OAW never starts a model
process. `CURRENT` uses the active session unchanged, while `SUBAGENT` is
available only when the current Host can create a native child.

OAW does not decide which methodology is universally best. Its initial
three-family assessment is an experience-bounded design input, documented in
the [comparison](comparison.md). The normative ownership and switching rules
remain in [policy/ENGINEERING.md](../../policy/ENGINEERING.md); the
[lifecycle guide](lifecycle.md) explains how to apply them.

## Result

The practical result is Native Host behavior for ordinary work and one explicit
workflow decision for each activated deliverable. Providers can coexist without
competing inside an OAW Engagement, client configuration can change without
creating a second governance source, and installation lifecycle operations
remain local, reviewable, and recoverable.
