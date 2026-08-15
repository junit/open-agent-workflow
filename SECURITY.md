# Security Policy

## Supported Version

The supported version is the latest tagged release and the current `main`
branch. Earlier snapshots receive no separate security maintenance.

## Private Reporting

Do not disclose vulnerability details or sensitive configuration in a public
issue. Open a minimal issue without exploit details and ask maintainers for a
private reporting channel. Include the affected version, prerequisites,
minimal reproduction, impact, and mitigation when available.

The project has no dedicated security address and offers no response SLA.

## Trust Boundary

The Agent Host, operating system, repository, credentials, sandbox, approvals,
and user remain the physical authority. OAW Policy rules do not grant
permissions and OAW never starts a model process.

The Go installer validates owned destinations, rejects symlink redirection,
prepares mutations before apply, and keeps Install State private. These
controls do not make an untrusted checkout safe and do not protect files
outside the selected roots.

install.sh executes only a sibling oaw or oaw.exe. It never searches PATH,
downloads code, or builds at runtime. Verify release checksums before use.

Machine Assurance and Bridge are optional evidence components. They cannot
choose or veto a Policy Profile and cannot enforce a sandbox. A Host security
policy may independently refuse physical invocation.
