# Open Agent Workflow

[简体中文](README-zh.md)

Open Agent Workflow (OAW) is a rule-driven engineering workflow for agent
tools. It installs one portable Policy Set whose Markdown Profiles compose
independently installed Skills into a clear engineering method.

The Policy, selected Profile, readable or invokable Skills, and the Agent
Host's native abilities form the complete normal product. No OAW runtime,
Bridge, route scanner, reducer, or state database is required after
installation. Optional machine components may add exact evidence without
changing or vetoing the rule-driven path.

## Why OAW

Engineering workflow providers can each trigger planning, implementation, TDD,
debugging, review, and completion procedures. When several providers are
available to one agent, those automatic triggers can compete for the same
responsibility. The result may be duplicate plans, conflicting test methods,
or a workflow that silently changes during a follow-up.

The same developer may also use several agent tools. Each tool has different
instruction files, scopes, precedence rules, and reload behavior. Hand-copying
governance across those surfaces creates cross-client drift. OAW keeps one
canonical policy and renders thin target-native entrypoints around it.

## Problems It Solves

- Overlapping automatic triggers from independently installed workflow
  providers.
- Silent profile selection or methodology changes during one deliverable.
- Ambiguous ownership between planning, implementation, tests, debugging,
  review, and completion.
- Divergent workflow instructions across user and repository agent settings.
- Unsafe upgrades or removal of content mixed with user-owned instructions.
- Provider detection being mistaken for permission to select a workflow.

## Capabilities

- Activates only when the user explicitly asks OAW to govern a deliverable.
- Selects built-in or Custom Markdown Profiles through natural language and
  follows their Skills through the Host's normal invocation or readable rules.
- Supports Superpowers, Matt Pocock Skills, Everything Claude Code (ECC), a
  predefined Matt-Superpowers hybrid, and user-defined Skill combinations.
- Uses model-led Skill resolution: an advisory index may help discovery but
  cannot make an otherwise readable Skill or Profile unavailable.
- Scales planning, review, approval, and verification from qualitative task
  complexity and risk without turning them into CLI fields or startup gates.
- Keeps progress in the Host's normal conversation and planning state, with an
  optional Markdown Progress Note for long-running work.
- Installs user- or project-scoped adapters for nine agent tools.
- Provides idempotent `check`, `install`, `update`, and `uninstall` lifecycles
  with target selection, dry runs, drift checks, and recoverable force.
- Provides read-only `profile list`, `profile show`, and `profile check`
  commands for advisory Markdown Profile inspection.
- Preserves unrelated user content and removes only OAW-owned artifacts.

## Host-Scoped Provider Authority

Provider authority follows one exact identity chain:

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

Codex and Claude Code are independent Hosts. Even when they reference the same
physical files, OAW derives separate Host Installation identities. Provider
Descriptor bindings and configured installation hints are declarations only;
they cannot create Host Binding Evidence. A `policy` Host may report
Candidates, but a Candidate cannot satisfy Profile compilation without a
verified Provider Instance. Foreign-Host diagnostics never become a pin,
Registry input, Profile owner, Capability Grant, or Workflow authority.

The active Provider Descriptor is `oaw.provider-descriptor/v4` and user
configuration remains v3; older inputs are rejected rather than upgraded. An
ambiguous current-Host candidate can be pinned only with the
exact identity fields below; `location` and `version` are optional readable
assertions:

```toml
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-<sha256>"
evidence_digest = "<sha256>"
# location = "/exact/physical/path"
# version = "6.1.1"
```

Stable Host-scope diagnostics include `HOST_BINDING_EVIDENCE_REQUIRED`,
`PROVIDER_BINDING_UNAVAILABLE`, `PROVIDER_FOREIGN_HOST_ONLY`,
`PROVIDER_PIN_INCOMPATIBLE`, and `HOST_PROVIDER_SCOPE_MISMATCH`. Exact physical
evidence belongs to the separately built Machine Assurance component. The
default `oaw` CLI does not expose Provider inspection or use machine evidence
to decide whether a Profile can be followed.

## Quick Start

Release archives contain the correct precompiled `oaw` or `oaw.exe` binary.
After verifying `SHA256SUMS`, extract the archive for your platform and invoke
the binary directly:

```bash
./oaw check
./oaw install
./oaw install --project /path/to/repository
./oaw update --dry-run
./oaw uninstall
```

The bundled `install.sh` is a Bash 3.2-compatible wrapper for scripts that use
the earlier entrypoint. These compatibility forms execute the sibling binary:

```bash
./install.sh check
./install.sh install
./install.sh install --project /path/to/repository
./install.sh update --dry-run
./install.sh uninstall
```

From a source checkout, build the binary before using either entrypoint:

```bash
go build -o ./oaw ./cmd/oaw
./oaw check
./oaw profile list
```

`check` reports provider detection, target readiness, and installation health
without mutation. A plain `install` uses user scope and the four core targets.
`--project` selects one existing repository and defaults to all nine targets.
Use `--target claude,codex` (or another comma-separated set of IDs) to narrow a
command. Run `./oaw --help` or `./install.sh --help` for the management CLI
surface.

Profile inspection is optional and read-only:

```bash
./oaw profile list
./oaw profile show MATT-FULL
./oaw profile show project:team-delivery
./oaw profile check .oaw/profiles/team-delivery.md
```

`profile list` reports the built-ins embedded in the current binary plus direct
Markdown Custom Profiles from the current project and user config roots. A
project/user ID conflict remains two entries and requires `project:<id>` or
`user:<id>` for `show` and `check`. Duplicate IDs inside one scope, malformed
required `id`/`name` metadata, and Custom Profiles that claim a reserved
built-in ID are reported explicitly. Partial Profiles remain valid; body and
Responsibility diagnostics are warnings. These commands do not inspect Skill
availability, choose a Profile, create workflow state, or decide whether the
model can use a Profile.

### Core, Coordination, and Host Boundaries

Public installation management is Go-authoritative.

`install.sh` is an offline sibling-binary compatibility wrapper.

Release archives contain precompiled binaries and perform no runtime executable download.

Installation management distributes the canonical Policy and target-native instruction entrypoints; it does not execute engineering work.

The cooperative Policy path does not require OAW Core. On machine-backed paths, OAW Core is required and stateless. The Workflow Coordinator is optional and stores only Workflow State for `WORKFLOW`; Install State and Workflow State are disjoint, with no migration or implicit adoption.

The Agent Host owns Agents, model calls, MCP, Hooks, Skills, Plugins, authentication, tools, sandbox, approvals, and every physical effect. OAW never starts a model process.

`CURRENT` uses the active session unchanged. `SUBAGENT` is eligible only when the active Host provides a native Subagent facility; there is no process fallback. Codex has a policy integration by default. The separately built `oaw-bridge` is an optional Assurance integration: its only MCP operation, `observe_profile`, can attach current exact Skill Binding identities to a source-qualified Markdown Profile. It does not select or run a Profile, attest delegation, call Core or the Coordinator, or change the no-Bridge Policy path.

Available native and Docker smoke tests must pass; unavailable platform checks return 77 and do not block release readiness. On macOS, use `scripts/smoke-docker.sh` for the native Linux archive when Docker Desktop is available. WSL-specific checks are optional and a `SKIP` is recorded, never reported as a pass.

```bash
docker_arch=$(docker version --format '{{.Server.Arch}}')
bash scripts/smoke-docker.sh \
  "$PWD/dist/open-agent-workflow_0.1.0_linux_${docker_arch}.tar.gz"
```

## Explicit Activation

Installation distributes a lazy Activation Router; it does not enroll ordinary
work into OAW. Until the current top-level user request explicitly asks OAW to
govern a deliverable, the Host remains Native Host behavior, as if OAW were not
installed. An ordinary bug fix, Host-selected Skill, or direct invocation of an
ordinary Skill follows the Host's normal routing with no OAW mode, gate,
recommendation, or state.

`/oaw <task>` or `Use OAW to handle <task>` creates one task-scoped
`OAW Engagement`. Related follow-ups inherit it; unrelated deliverables remain Native
Host behavior. OAW then performs Assurance Preflight before classifying the
activated task:

```text
Native Host unless explicitly activated
    -> Assurance Preflight
    -> DIRECT / BOUNDED / WORKFLOW
    -> cooperative or machine-backed execution
```

Assurance is separate from Request Mode. Instruction-only Hosts use
`policy-cooperative`; a current Host-native integration may support
`core-backed` or `coordinator-backed`. Activated `DIRECT` handles one small,
recoverable change. Activated `BOUNDED` handles one user-selected Capability
and one named deliverable; it is not normal Host Skill routing. Activated
`WORKFLOW` runs a selection gate. There is no timeout or silent default.
Machine-backed selection may compile a Lifecycle Bundle; policy-cooperative
selection uses an explicit Profile candidate, `CURRENT`, a Policy Workflow
Plan, and a Progress Tracker without claiming machine guarantees.

## No-Bridge Policy Workflow

After installation, activate OAW and select a Profile in the same request:

```text
Use OAW with MATT-SP-HYBRID to deliver the editor.
```

If no Profile is named, the Agent states a reasonable choice and proceeds
unless materially different methods create genuine ambiguity. Selection does
not require a topology choice, Add-on sentinel, Complexity or Risk field,
limitation acknowledgement, Provider proof, or a second confirmation for each
declared Skill. Terms such as `policy_selectable` and `host_routable` may appear
in older diagnostic records, but they are not admission gates for the static
Policy product.

Codex uses its native Skill index first and lazily reads relevant Skill
instructions when the index is incomplete. This keeps `SP-FULL`, `MATT-FULL`,
`ECC-FULL`, and `MATT-SP-HYBRID` usable without Bridge whenever their rules can
be read or invoked. The model owns progress through normal conversation and
planning; a Markdown Progress Note is optional and can never block work.

Bridge and Machine Assurance are optional evidence components. Their absence
or failure removes only their machine claim, not Profile selection, Skill use,
review, verification, or completion.

## Lifecycle Profiles

| Profile | Lifecycle contract |
| --- | --- |
| `MATT-FULL` | Matt-led `oaw/domain-engineering`; neutral Host actions fill exact workspace, broad verification, and closeout gaps. |
| `SP-FULL` | Inline Superpowers `oaw/delivery` with its exact planning, TDD, debugging, review, verification, and finish skills. |
| `ECC-FULL` | ECC-led `oaw/ecc-engineering`; only exact Host-observed Skill, Agent, Role, or Instruction alternatives compile. |
| `MATT-SP-HYBRID` | Preserved Matt/Superpowers composition; ECC remains an explicitly selected typed Add-on only. |
| `USER-DEFINED` | Select a configured, versioned user-defined Profile Recipe; this is not a fifth built-in alias. |

A recommendation never becomes a default. Missing provider capability stops
Workflow selection; it is never silently omitted or replaced. Superpowers,
Matt, ECC, and third-party Providers use the same extensible Provider and
Capability model. A Host-native Subagent inherits the exact locked bundle and
does not reopen family arbitration. A bounded add-on may produce only its
declared specialist deliverable and cannot take over the lifecycle. `DIRECT`
and `BOUNDED` do not create Workflow State.

The common ordered lifecycle is `problem-framing` ->
`solution-specification` -> `delivery-planning` -> `workspace-preparation` ->
`implementation` -> `implementation-tdd` -> conditional `incident-recovery` ->
`review-remediation` -> `fresh-verification` -> `closeout`. Every Recipe must
resolve one outcome owner per applicable slot plus its neutral Host actions and
gates. `FULL` never transfers Host ownership to a Provider.

## Matt-Superpowers Hybrid

The initial three-family scores are experience-based design inputs, not
universal benchmarks. They are version-sensitive judgments using procedure
completeness, correctness discipline, ambiguity handling, review closure,
verification strength, and operational overhead. Values below are shown in
**Superpowers / Matt / ECC** order; the detailed comparison records the limits.

| Stage | Scores (SP / Matt / ECC) | `MATT-SP-HYBRID` owner |
| --- | --- | --- |
| Planning | 4.8 / 5.0 / 3.8 | Matt for requirements, domain modeling, specification, and complex-work decomposition; Superpowers for per-ticket executable plans |
| Implementation | 5.0 / 4.2 / 3.7 | Superpowers |
| TDD | 4.8 / 4.9 / 4.1 | Matt `tdd` |
| Debugging | 4.7 / 5.0 / 2.8 | Matt `diagnosing-bugs` |
| Review | 5.0 / 4.8 / 4.4 | Superpowers |
| Completion | 5.0 / 3.6 / 4.0 | Superpowers |

Workspace and Git setup belong to Superpowers. Build, dependency, or type
repair belongs only to an explicitly selected ECC resolver, or to no ECC
resolver. Specialist checks run only as exact bounded add-ons. These assignments
give each responsibility exactly one owner. The precise sequence is Matt
`grill-with-docs` -> `to-spec` -> `to-tickets`, Superpowers
`superpowers:writing-plans` -> `superpowers:using-git-worktrees` -> inline
`superpowers:executing-plans`, Matt `tdd` and `diagnosing-bugs`, then
Superpowers review/remediation, `superpowers:verification-before-completion`,
and `superpowers:finishing-a-development-branch`.

## Supported Targets

Target IDs are stable CLI inputs and must be written exactly as shown. Core
adapters support both user and project scope. Extension adapters are officially
supported at project scope because their global surfaces are GUI-managed,
platform-specific, experimental, or less stable.

| Target ID | Agent tool | User scope | Project scope | Control surface |
| --- | --- | --- | --- | --- |
| `claude` | Claude Code | Yes | Yes | `policy` |
| `codex` | Codex CLI | Yes | Yes | `policy` |
| `gemini` | Gemini CLI | Yes | Yes | `policy` |
| `opencode` | OpenCode | Yes | Yes | `policy` |
| `cursor` | Cursor | No | Yes | `policy` |
| `windsurf` | Windsurf / Devin rules | No | Yes | `policy` |
| `cline` | Cline | No | Yes | `policy` |
| `roo` | Roo Code | No | Yes | `policy` |
| `copilot` | GitHub Copilot | No | Yes | `policy` |

User scope defaults to `claude,codex,gemini,opencode`. Project scope defaults
to all rows in registry order. An unsupported target/scope combination or
unknown ID fails before mutation. Provider detection and target readiness are
diagnostics; neither chooses a lifecycle profile.

## Safety Model

- OAW does not install Superpowers, Matt Pocock skills, or ECC. Providers stay
  independently licensed, installed, configured, and updated.
- The Agent Host retains physical authority. OAW Grants and Resource Leases
  coordinate cooperating clients and never replace the Host sandbox and
  approvals.
- A selected local checkout or extracted release binary is executable code and
  must be reviewed and trusted. Management never downloads an executable at
  runtime.
- All destinations are prepared and validated before the first managed write.
- Existing instruction files receive one checksummed OAW block; owned extension
  files are kept separate. Marker comments are ownership boundaries, not model
  precedence controls.
- Paths are constrained to the selected user or project roots, and symlink
  redirection is rejected. State is parsed as inert data.
- Drift fails closed before mutation.
- `--force` backs up every affected artifact before mutation. It never bypasses
  invalid state or path-containment checks.
- `--dry-run` previews planned mutations without writing managed content, state,
  backups, or target directories.

See [SECURITY.md](SECURITY.md) or [SECURITY-zh.md](SECURITY-zh.md) for the
reporting contract and installer trust boundary.

## Update and Uninstall

Updates use the Policy, Version, registry, and rendering behavior embedded in
the running binary. Rebuild `./oaw` after changing a source checkout; a release
archive already contains the intended binary. There is no v0.1 self-update,
remote main-branch fetch, package-manager update, or provider update. A clean
repeated install or update is idempotent.

By default, changed OAW-managed content is reported as drift and blocks the
entire operation before a write. Inspect the diagnostic and use a dry run. Use
`--force` only when replacing or removing the drift is intentional; a complete
operation-scoped backup is created first.

Uninstall removes managed blocks and OAW-owned files, retains unrelated user
bytes, and prunes only empty directories that OAW actually created. The
canonical policy remains until the final valid installation reference is
removed. Target selection supports partial uninstall.

## Provider Prerequisites

Install and maintain workflow providers through their own trusted channels
before choosing a profile that requires them. OAW's `check` command reports
Superpowers, Matt, and ECC independently; it reports a provider as `missing`
when its required indicators are incomplete. It does not download, vendor,
patch, update, remove, license, or silently substitute provider content. Agent
tools themselves are also installed separately.

The default CLI intentionally has no Provider catalog or resolution command.
Use `profile list`, `profile show`, and `profile check` only to inspect Markdown
Profile identity and structure. The Agent determines Skill availability from
the Host's native surfaces and readable instructions. Exact Provider and Host
Binding claims belong to the separately built `oaw-assurance` component and do
not control normal Policy execution. See the [lifecycle guide](docs/en/lifecycle.md)
and [troubleshooting guide](docs/en/troubleshooting.md) for details.

## Documentation

Each detailed guide has equivalent English and Chinese entrypoints. These
guides are being completed as part of the local v0.1 documentation ticket:

| Topic | English | 简体中文 |
| --- | --- | --- |
| Background | [English](docs/en/background.md) | [中文](docs/zh/background.md) |
| Comparison | [English](docs/en/comparison.md) | [中文](docs/zh/comparison.md) |
| Lifecycle | [English](docs/en/lifecycle.md) | [中文](docs/zh/lifecycle.md) |
| Architecture | [English](docs/en/architecture.md) | [中文](docs/zh/architecture.md) |
| Installer | [English](docs/en/installer.md) | [中文](docs/zh/installer.md) |
| Machine Assurance | [English](docs/en/machine-assurance.md) | [中文](docs/zh/machine-assurance.md) |
| Codex Assurance Bridge | [English](docs/en/codex-bridge.md) | [中文](docs/zh/codex-bridge.md) |
| Adapters | [English](docs/en/adapters.md) | [中文](docs/zh/adapters.md) |
| Extending adapters | [English](docs/en/extending-adapters.md) | [中文](docs/zh/extending-adapters.md) |
| Security model | [English](docs/en/security.md) | [中文](docs/zh/security.md) |
| Troubleshooting | [English](docs/en/troubleshooting.md) | [中文](docs/zh/troubleshooting.md) |

The portable entry is [policy/POLICY.md](policy/POLICY.md), with natural-language
operation in [policy/cooperative-protocol.md](policy/cooperative-protocol.md).
The detailed guides explain that Policy Set and the implementation; they do not
replace it.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) or
[CONTRIBUTING-zh.md](CONTRIBUTING-zh.md). Contributions must preserve Bash 3.2
compatibility, the black-box CLI test seam, English/Chinese parity, adapter
evidence, provider independence, and the no-remote-publication boundary. The
community expectations are in [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## License

OAW is licensed under the [Apache License 2.0](LICENSE). Workflow providers and
agent tools remain governed by their own licenses.

## Project Status

The source baseline is fixed at v0.1.0 as of 2026-08-14. Cross-platform
archives can be built locally and release readiness follows the available
native/Docker verification matrix. This repository state is not a published
remote release and this change does not create a tag, package, domain, or
globally reserved name. Remote publication and tag creation require separate
owner approval.
