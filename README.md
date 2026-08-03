# Open Agent Workflow

[简体中文](README-zh.md)

Open Agent Workflow (OAW) arbitrates independently installed workflow providers across agent tools. It gives one engineering deliverable one explicit
lifecycle owner or one conflict-free stage map, then installs the same
governance policy into the instruction surfaces used by supported coding
agents.

OAW is a provider-neutral policy, adapter layer, and Go runtime. It does not
redistribute workflow families or replace the native configuration of an agent
tool.

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

- Classifies a top-level engineering request as `DIRECT`, `BOUNDED`, or
  `WORKFLOW` before a family-specific lifecycle starts.
- Presents every eligible built-in and user-defined lifecycle Profile for
  Workflow Mode and waits for the user's explicit choice.
- Locks the selected bundle across follow-ups, context compaction, tickets, and
  delegated agents.
- Supports full-family profiles, a predefined Matt-Superpowers hybrid, bounded
  specialist add-ons, and user-defined conflict-free stage maps.
- Detects Superpowers, Matt Pocock skills, and Everything Claude Code (ECC)
  independently without selecting among them.
- Installs user- or project-scoped adapters for nine agent tools.
- Provides idempotent `check`, `install`, `update`, and `uninstall` lifecycles
  with target selection, dry runs, drift checks, and recoverable force.
- Preserves unrelated user content and removes only OAW-owned artifacts.

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
```

`check` reports provider detection, target readiness, and installation health
without mutation. A plain `install` uses user scope and the four core targets.
`--project` selects one existing repository and defaults to all nine targets.
Use `--target claude,codex` (or another comma-separated set of IDs) to narrow a
command. Run `./oaw --help` or `./install.sh --help` for the management CLI
surface.

### Cutover and Runtime Boundaries

Public installation management is Go-authoritative.

`install.sh` is an offline sibling-binary compatibility wrapper.

Release archives contain precompiled binaries and perform no runtime executable download.

Install State and Runtime State are disjoint; no automatic migration occurs.

Existing Policy-only tasks and profile locks remain Policy-only unless explicitly adopted at a Stable Boundary.

Only the pinned Codex runner is currently Runtime-managed.

Other installed adapters remain Policy-only and provide no Runtime admission, Capability Grant, Resource Lease, transition enforcement, or physical isolation guarantee.

Available native and Docker smoke tests must pass; unavailable platform checks return 77 and do not block release readiness. On macOS, use `scripts/smoke-docker.sh` for the native Linux archive when Docker Desktop is available. WSL-specific checks are optional and a `SKIP` is recorded, never reported as a pass.

```bash
docker_arch=$(docker version --format '{{.Server.Arch}}')
bash scripts/smoke-docker.sh \
  "$PWD/dist/open-agent-workflow_0.1.0_linux_${docker_arch}.tar.gz"
```

## Task Gate

OAW performs enough read-only inspection to classify each top-level engineering
request as `DIRECT`, `BOUNDED`, or `WORKFLOW`. Direct Mode covers small, clear,
recoverable changes executed by the Main Agent. Bounded Mode admits one exact
Provider Capability for one observable deliverable. Neither mode selects a
lifecycle.

Only Workflow Mode runs the Startup Gate. OAW then shows every eligible
built-in and user-defined Profile, a recommendation, and any proposed bounded
add-ons. The user must choose explicitly. There is no timeout or silent default.
The compiled Lifecycle Bundle remains locked to the deliverable. Only the user
may switch it, and only at a stable boundary such as an approved specification,
a completed ticket, debugging cycle, review, or verification.

## Lifecycle Profiles

| Profile | Lifecycle ownership |
| --- | --- |
| `SP-FULL` | Superpowers owns the complete lifecycle. |
| `MATT-FULL` | Matt owns the complete lifecycle. |
| `ECC-FULL` | ECC owns the complete `oaw/ecc-engineering` lifecycle. |
| `MATT-SP-HYBRID` | Matt and Superpowers own the explicit stages below; declared ECC specialists remain bounded add-ons. |
| `USER-DEFINED` | Select a configured, versioned user-defined Profile Recipe; this is not a fifth built-in alias. |

A recommendation never becomes a default. Missing provider capability stops
Workflow selection; it is never silently omitted or replaced. Superpowers,
Matt, ECC, and third-party Providers use the same extensible Provider and
Capability model. A delegated agent inherits the exact locked bundle and does
not reopen family arbitration. A bounded add-on may produce only its declared
specialist deliverable and cannot take over the lifecycle.

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
give each responsibility exactly one owner.

## Supported Targets

Target IDs are stable CLI inputs and must be written exactly as shown. Core
adapters support both user and project scope. Extension adapters are officially
supported at project scope because their global surfaces are GUI-managed,
platform-specific, experimental, or less stable.

| Target ID | Agent tool | User scope | Project scope | Support level |
| --- | --- | --- | --- | --- |
| `claude` | Claude Code | Yes | Yes | Core |
| `codex` | Codex CLI | Yes | Yes | Core |
| `gemini` | Gemini CLI | Yes | Yes | Core |
| `opencode` | OpenCode | Yes | Yes | Core |
| `cursor` | Cursor | No | Yes | Project extension |
| `windsurf` | Windsurf / Devin rules | No | Yes | Project extension |
| `cline` | Cline | No | Yes | Project extension |
| `roo` | Roo Code | No | Yes | Project extension |
| `copilot` | GitHub Copilot | No | Yes | Project extension |

User scope defaults to `claude,codex,gemini,opencode`. Project scope defaults
to all rows in registry order. An unsupported target/scope combination or
unknown ID fails before mutation. Provider detection and target readiness are
diagnostics; neither chooses a lifecycle profile.

## Safety Model

- OAW does not install Superpowers, Matt Pocock skills, or ECC. Providers stay
  independently licensed, installed, configured, and updated.
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
| Adapters | [English](docs/en/adapters.md) | [中文](docs/zh/adapters.md) |
| Extending adapters | [English](docs/en/extending-adapters.md) | [中文](docs/zh/extending-adapters.md) |
| Security model | [English](docs/en/security.md) | [中文](docs/zh/security.md) |
| Troubleshooting | [English](docs/en/troubleshooting.md) | [中文](docs/zh/troubleshooting.md) |

The normative workflow is [policy/ENGINEERING.md](policy/ENGINEERING.md). The
detailed guides explain that policy and the implementation; they do not replace
it.

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

This repository is an unreleased, local-only v0.1 candidate. Cross-platform
archives can be built locally and release readiness follows the available
native/Docker verification matrix. The project does not claim a published remote repository, package,
release, domain, or globally reserved name. Machine-readable management status
is reserved for a post-v0.1 extension; v0.1 management output is human-readable
only. Remote publication requires separate owner approval.
