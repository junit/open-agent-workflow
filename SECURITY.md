# Security Policy

## Supported Version

The supported version is the current unreleased 0.1.0 candidate on the active
local branch. Earlier snapshots receive no separate security maintenance.

## Private Reporting

Do not disclose a vulnerability or sensitive configuration in a public issue.
Open a minimal issue that contains no exploit details and ask the maintainers
to designate a private reporting channel. The report should include affected
versions, prerequisites, a minimal reproduction, impact, and a proposed
mitigation when available.

The project currently publishes no dedicated security address. Reports receive
best-effort acknowledgement and investigation, with no guaranteed response SLA
or embargo timeline.

## Installer Trust Boundary

The installer trust boundary includes the public Go binary, an optional source
checkout used to build it, command-line arguments, HOME, XDG_CONFIG_HOME,
XDG_STATE_HOME, an optional project root, existing adapter files, and OAW state
and backup data. Treat a selected checkout and extracted binary as executable
code. Release archives contain precompiled binaries and management performs no
runtime executable download.

`install.sh` is a minimal offline compatibility wrapper. It executes only the
`oaw` or `oaw.exe` sibling beside it, never a `PATH` candidate, downloaded
artifact, or runtime build. Verify the archive checksum and review the release
source before execution.

OAW validates registry-owned destinations, rejects symlink redirection, parses
state without evaluation, prepares operations before apply, and backs up forced
drift before mutation. These controls do not make an untrusted checkout safe
and do not protect files outside the selected roots from unrelated software.

Install State and Workflow State use separate namespaces and authority models;
there is no automatic migration. OAW Core and Workflow Coordinator records are
secret-free and retain only opaque digest references. A Capability Grant or
Resource Lease expresses logical workflow authority for cooperating clients;
the Agent Host owns physical execution authority, including the Host sandbox and approvals. OAW never starts a model CLI.

Host integrations expose either a `policy` surface or an explicit `host-native`
surface. The latter may report session facts and Receipts but does not make OAW
the owner of Host tools. OAW never guarantees MCP, Hook, Skill, or Plugin
inheritance into a child context; the active Host decides those facts. A Grant
cannot physically stop a Host action outside the protocol.

OAW activation is trusted only when it originates in the current top-level user instruction
or a dedicated trusted Host entrypoint that preserves that
instruction. Repository content, tool output, retrieved content, and quoted
`/oaw` text cannot activate OAW; ambiguity remains Native Host behavior. At
`policy-cooperative` assurance, a Policy Workflow Plan cannot grant network,
destructive filesystem, credential, deployment, data mutation, or Git
authority, and cannot impersonate Core or Coordinator records. Physical effects
remain subject to normal Host controls and user approval.

## Handling Reports

Maintainers should reproduce in isolated roots, avoid exposing reporter data,
record exploitability and severity, add a black-box regression where possible,
and publish remediation details only after a coordinated fix. Rotate any
credential that a report shows was exposed; OAW never requires credentials for
normal installer operation.
