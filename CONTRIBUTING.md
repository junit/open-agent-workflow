# Contributing to Open Agent Workflow

OAW is a rule-driven product. Changes must preserve the static Policy Set as
the normal product core and keep optional machine components additive.

## Development Contract

- Keep Go installation management authoritative for check, install, update,
  uninstall, and advisory Profile inspection.
- Keep install.sh as a Bash 3.2-compatible offline sibling-binary wrapper.
- Do not install, update, patch, or vendor an external Skill provider.
- Exercise CLI behavior in isolated HOME, XDG_CONFIG_HOME, XDG_STATE_HOME,
  and project roots.
- Keep English and Chinese user documentation semantically equivalent.
- Keep Host-specific paths in policy/adapters and portable semantics in Policy
  and Profiles.
- Do not add a scanner, state machine, or machine evidence requirement to the
  Policy path.

## Testing

Focused tests should cover Policy Set validation, installation ownership,
Profile discovery, optional-component isolation, and no-binary operation.
Run:

    go test ./... -count=1
    go test -race ./... -count=1
    go vet ./...
    bash -n install.sh tests/*.sh scripts/*.sh
    bash tests/run.sh
    bash scripts/check-docs.sh

Review the diff for secrets, unrelated generated files, unsafe path expansion,
dead compatibility code, and missing documentation parity.
