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

The installer trust boundary includes the current local checkout, command-line
arguments, HOME, XDG_CONFIG_HOME, XDG_STATE_HOME, an optional project root,
existing adapter files, and OAW state and backup data. Treat the selected local
checkout as executable code. OAW does not fetch or execute remote code.

OAW validates registry-owned destinations, rejects symlink redirection, parses
state without evaluation, prepares operations before apply, and backs up forced
drift before mutation. These controls do not make an untrusted checkout safe
and do not protect files outside the selected roots from unrelated software.

## Handling Reports

Maintainers should reproduce in isolated roots, avoid exposing reporter data,
record exploitability and severity, add a black-box regression where possible,
and publish remediation details only after a coordinated fix. Rotate any
credential that a report shows was exposed; OAW never requires credentials for
normal installer operation.
