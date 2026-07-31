# Architecture

[简体中文](../zh/architecture.md) | [README](../../README.md)

Open Agent Workflow (OAW) installs one canonical engineering policy into the
instruction surfaces of supported agent tools. It does not install those tools
or their workflow providers. The installer owns only the policy, its recorded
adapter output, and directories that it actually created.

## Components and Boundaries

The repository contains five cooperating layers:

1. The checkout supplies `VERSION`, `policy/ENGINEERING.md`, the target
   registry, and pure renderer functions.
2. The CLI parses a command and selects user or project scope.
3. Path and state code derives canonical destinations and validates existing
   installation records as inert data.
4. Transaction code prepares every change, creates any required backup, and
   then applies the prepared files.
5. Adapter targets make the installed policy visible through each tool's own
   instruction mechanism.

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

`<crc>-<bytes>` is the `cksum` result for the physical project-root path bytes.
This gives each resolved project root an isolated state record without placing
installer metadata inside that repository. State paths are metadata locations;
they do not change which repository files a project installation may own.

## Data Flow

The mutation pipeline is:

```text
checkout policy -> pure renderer -> preflight/prepare -> required backup -> apply -> state/targets
```

The arrows describe data and control flow, not a promise of operation-wide
atomicity. A renderer receives validated values and writes prospective content
only to a caller-provided temporary path. Because it is a **pure renderer**, it
does not inspect or mutate the eventual destination.

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
place, providing **atomic replacement per destination**. OAW deliberately does
not claim a global transaction, operation-wide rollback, or atomic replacement
across several destinations. A later apply failure can therefore leave an
earlier destination changed; verified backups are the recovery boundary for a
forced operation.

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

State files are tab-separated records. They are parsed as inert text and are
never sourced or evaluated by the shell.

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
