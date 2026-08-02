# Architecture

[简体中文](../zh/architecture.md) | [README](../../README.md)

Open Agent Workflow (OAW) installs one canonical engineering policy into the
instruction surfaces of supported agent tools. It does not install those tools
or their workflow providers. The installer owns only the policy, its recorded
adapter output, and directories that it actually created.

## Components and Boundaries

The repository contains six cooperating layers:

1. The checkout supplies `VERSION`, `policy/ENGINEERING.md`, the target
   registry, and pure renderer functions.
2. The public Go CLI parses management commands and selects user or project
   scope; `install.sh` only executes the precompiled sibling binary.
3. Path and state code derives canonical destinations and validates existing
   installation records as inert data.
4. Transaction code prepares every change, creates any required backup, and
   then applies the prepared files.
5. Adapter targets make the installed policy visible through each tool's own
   instruction mechanism.
6. The optional Runtime Plane provides the canonical Runtime Protocol,
   `oaw runtime exchange`, and the selected `oaw run --host codex` Host driver.

The policy is the normative workflow source. Adapter files are transport
layers, not independent policy copies. Agent tools, Superpowers, Matt Pocock
skills, and ECC remain independently installed and versioned.

## Canonical Storage

OAW follows the XDG base-directory convention while retaining explicit
defaults:

| Artifact | Canonical path |
| --- | --- |
| Installed policy | `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/ENGINEERING.md` |
| User installation state | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/installations/user.state` |
| Project installation state | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/installations/projects/<crc>-<bytes>.state` |
| Operation backups | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/backups` |
| Runtime State | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/runtime` |

`<crc>-<bytes>` is the `cksum` result for the physical project-root path bytes.
This gives each resolved project root an isolated state record without placing
installer metadata inside that repository. State paths are metadata locations;
they do not change which repository files a project installation may own.

## Data Flow

The mutation pipeline is:

```text
embedded checkout policy -> pure renderer -> preflight/prepare -> required backup -> apply -> Install State/targets
```

The Go binary embeds the Policy, Version, registry, and renderer behavior from
the checkout used to build it. Release archives already contain that binary;
a source checkout must build `./oaw` before execution. The arrows describe data
and control flow, not simultaneous operation-wide atomicity. A renderer
receives validated values and writes prospective content only to a
caller-provided temporary path. Because it is a **pure renderer**, it does not
inspect or mutate the eventual destination.

## Runtime Plane

The Runtime Plane is optional and does not replace the Policy Plane. The
canonical `oaw.runtime/v1` transport is available through `oaw runtime exchange`;
`oaw run --host codex` uses the same `runtime.Engine.Exchange` seam and adds the
ordered `GRANT_ISSUED`, `DISPATCH_PREPARED`, `DISPATCH_AUTHORIZED`, and
`CAPABILITY_OBSERVED` handshake around a bounded Codex process.

Only the pinned Codex runner is currently Runtime-managed. Runtime-aware execution is admitted
only for the pinned `runner-managed` integration `oaw/codex-runner`. Other
installed adapters remain Policy-only and provide no Runtime admission,
Capability Grant, Resource Lease, transition enforcement, or physical isolation
guarantee. Discovery, installation, and project configuration never promote
them.

Install State and Runtime State are disjoint; no automatic migration occurs.
Existing Policy-only tasks and profile locks remain Policy-only unless
explicitly adopted at a Stable Boundary. Management reads and writes only the
TSV installation namespace; Runtime reads and writes only the revisioned Run
namespace.

Host output is untrusted. The Codex driver bounds process output, keeps
diagnostics on stderr, normalizes JSONL into closed outcomes, and returns only
digest-pinned evidence references to Runtime state. A resumed run can provide
an explicit project root so the trusted project Configuration Snapshot remains
part of admission.

During the **prepare phase**, OAW resolves the source and destinations, renders
prospective files, checks containment and symlink rules, parses prior state,
detects drift, and verifies all planned actions before any managed destination
is written. Target selection and shared-destination collisions are resolved at
this point. Codex and OpenCode, for example, share one project `AGENTS.md`
managed block instead of racing to produce separate files.

If forced mutation would replace drifted or foreign content, OAW creates and
verifies an **operation-scoped backup** of every affected artifact before
applying any prepared action. The resulting backup reference can be recorded
in state.

During the **apply phase**, paths are validated again immediately before use.
Each file is written beside its destination as a temporary file and moved into
place, providing **atomic replacement per destination**. After every effect,
the Go manager records an inverse in its mutation journal. A reported apply
failure attempts reverse-order rollback; rollback failure is surfaced as an
error rather than hidden. OAW does not claim simultaneous atomic replacement
across destinations or automatic recovery from a process or machine crash.
Verified backups remain the auditable recovery boundary for forced operations.

## Ownership Modes

Every target declares one of two ownership modes:

- `managed-block` inserts one marker-delimited OAW block while preserving
  surrounding user content. Claude, Codex, Gemini, and OpenCode use this mode.
- `owned-file` reserves an adapter-specific file for OAW. Cursor, Windsurf,
  Cline, Roo Code, and Copilot use this mode.

Markers are an installer ownership boundary. **Marker comments do not establish model precedence**,
override a tool's documented instruction hierarchy, or force a running agent
session to reload. Each tool remains responsible for discovery, precedence,
merging, and cache or session behavior.

Drift means recorded OAW ownership no longer matches the current file. Without
`--force`, mutation fails closed. On uninstall, a clean managed block is
removed from its containing file, while a clean owned file is removed in full.
Only empty directories that state proves OAW created are eligible for pruning.

## State Schema

The records below describe Install State only. They are tab-separated inert
text and are never sourced or evaluated by the shell. Runtime State instead
uses immutable Run revisions, `HEAD`, Grants, Resource Leases, and evidence
references under its separate namespace; it never parses TSV Install State as
Runtime authority.

| Record | Cardinality | Meaning |
| --- | --- | --- |
| `format` | one | State serialization format. |
| `version` | one | Installed OAW version. |
| `scope` | one | `user` or `project`. |
| `project` | project only | Physical project root. |
| `policy` | one | Installed policy path and checksum metadata. |
| `backup` | optional | Backup manifest associated with the last relevant operation. |
| `directory` | zero or more | Directory created and therefore potentially removable by OAW. |
| `target` | one or more | Target ID, absolute path, ownership mode, checksum, and origin. |

Before an update or uninstall, OAW validates the record format, scope, project
identity, target registry membership, paths, ownership modes, and checksums.
Malformed or unsafe state is rejected rather than interpreted leniently.

## Trust Model

`HOME`, `XDG_CONFIG_HOME`, `XDG_STATE_HOME`, project roots, checkout artifacts,
existing target files, and state are all trust-boundary inputs. OAW requires
absolute safe roots, resolves project scope physically, rejects unsafe symlink
or containment paths, and rechecks destinations during apply. The
[installer guide](installer.md) documents commands and failures; the
[adapter guide](adapters.md) documents each client-facing surface.
