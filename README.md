# Open Agent Workflow

[简体中文](README-zh.md)

Open Agent Workflow (OAW) is a rule-driven engineering workflow for agent
hosts. It installs one readable Policy Set and lets the model select a Profile
and use the Skills already available in the host.

The complete product path is the Markdown Policy, its Profiles, the selected
Skills, and the Host's native abilities. Installation leaves readable rules in
the Host or project, so delivery does not require a running OAW process or
optional machine evidence.

## Quick Start

Use a release binary or build one from this checkout:

```bash
go build -o ./oaw ./cmd/oaw
./oaw check
./oaw install
```

The bundled `install.sh` is an offline wrapper for the sibling binary. It does
not download or build executable code:

```bash
./install.sh check
./install.sh install --project /path/to/repository
./install.sh update --dry-run
./install.sh uninstall
```

Installation management is Go-authoritative. `check` is read-only; `install`,
`update`, and `uninstall` manage Policy Set files, target-native instruction
entrypoints, and private Install State. They never execute engineering work.
The installed Policy Set includes full adapters for Claude, Codex, Gemini,
OpenCode, Cursor, Windsurf, Cline, Roo, and Copilot.

## Canonical Policy Set

The installed source is a single selected set:

```text
POLICY.md
cooperative-protocol.md
profiles/
  SP-FULL.md
  MATT-FULL.md
  ECC-FULL.md
  MATT-SP-HYBRID.md
adapters/
  claude-policy.md
  codex-policy.md
  gemini-policy.md
  opencode-policy.md
  cursor-policy.md
  windsurf-policy.md
  cline-policy.md
  roo-policy.md
  copilot-policy.md
```

A project Policy Set under `.oaw/policy/` takes precedence over a user Policy
Set. The two sets are never merged. Host instruction files contain only a
managed activation router that points to the selected set.

## Rule-Driven Use

OAW is opt-in. The current top-level request must explicitly ask the host to
use OAW for a deliverable. Otherwise the host behaves normally and OAW does
not inspect or change Skill, Agent, role, tool, or permission selection.

After activation, select a Profile in the same request when possible:

```text
Use OAW with MATT-SP-HYBRID to deliver the editor.
```

The model reads the selected Profile, then reads each declared Skill when its
Responsibility becomes current. A host Skill index is an optimization, not a
proof of availability. If an instruction is readable or a native invocation
is available, the model can use it without Bridge or Provider attestation.

The four built-in Profiles are ordinary Markdown and can be inspected with:

```bash
./oaw profile list
./oaw profile show built-in:MATT-SP-HYBRID
./oaw profile check built-in:SP-FULL
```

These commands are advisory and read-only. They do not select a Profile,
inspect Skill contents, create progress state, or run a workflow.

## Custom Profiles

Users can compose a new method from currently installed Skills without editing
Go code or registering a Provider. Create a Markdown file in one of these
locations:

```text
.oaw/profiles/team-delivery.md
$XDG_CONFIG_HOME/open-agent-workflow/profiles/team-delivery.md
```

Minimal metadata is enough:

```markdown
---
id: team-delivery
name: Team Delivery
---

# Team Delivery

## Responsibilities

| Responsibility | Skill or action |
| --- | --- |
| planning | `ecc:blueprint` |
| implementation | `matt:implementation` |
| verification | `ecc:verification` |
| closeout | Host-native closeout |
```

The Policy supplies defaults for responsibilities omitted by a Custom
Profile. Project and user Profiles with the same ID remain distinct; select
`project:team-delivery` or `user:team-delivery` when the source is
ambiguous. Built-in IDs are reserved and cannot be shadowed.

## Optional Assurance

Machine Assurance and the separately built `oaw-bridge` are optional evidence
components. Machine Assurance can issue or verify an exact identity mapping for
a selected Profile; the Bridge can supply current Codex observations to that
evidence path. They do not choose a Profile, invoke a Skill, own physical
permissions, manage delivery, or veto the Policy path. Removing them removes
only their machine-backed claim.

The Agent Host owns model calls, Agents, Skills, Plugins, MCP, Hooks,
credentials, tools, sandboxing, approvals, and every physical effect. OAW
never starts a model process or emulates a Host.

## Development

The source version is recorded in `VERSION`. Run the focused and full checks
from a checkout:

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
bash scripts/check-docs.sh
bash tests/run.sh
```

See [CONTEXT.md](CONTEXT.md), [the architecture decisions](docs/adr/README.md),
the [release operations manual](docs/en/releasing.md), and the
[Policy Set](policy/POLICY.md) for normative boundaries.
