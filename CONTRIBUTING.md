# Contributing to Open Agent Workflow

Open Agent Workflow (OAW) accepts issue-sized vertical changes that preserve
its provider-neutral lifecycle contract. Discuss behavior changes in an issue
before implementation. Keep one change responsible for one observable outcome.

## Development Contract

- Treat the public Go `oaw` binary as the authoritative implementation of
  `check`, `install`, `update`, and `uninstall`.
- Maintain Bash 3.2 compatibility for the minimal `install.sh` wrapper. It must
  execute only a precompiled sibling binary and must not search `PATH`, build,
  download, or add a package-manager runtime.
- Do not vendor, install, update, or patch any workflow provider.
- Exercise behavior through the real CLI black-box seam. Tests must use an
  isolated HOME, XDG_CONFIG_HOME, XDG_STATE_HOME, and project root.
- Add the focused test to the numbered suite. Documentation contracts live in
  tests/10-docs-test.sh and the complete suite runs through tests/run.sh.
- Keep user-visible English/Chinese documentation equivalent when behavior,
  safety guidance, support levels, or commands change.
- Treat remote publication, pushing, releases, credentials, and third-party
  resource changes as owner-approved operations outside installer code.
- Keep Install State and Runtime State disjoint. Management changes must not
  import existing Policy-only tasks or profile locks, and adapter installation
  must not imply Runtime admission.

## Adapter Evidence

Every adapter change must include an adapter evidence packet:

1. Official primary source URLs and a retrieval date.
2. Exact user and project destinations with declared support levels.
3. Loader, import/reference, precedence, and reload behavior.
4. Destination ownership, pure rendering, and shared-path collision rules.
5. Black-box fixtures plus hostile-path, symlink, and inert-data checks.

An adapter must not change lifecycle semantics or depend on undocumented
provider installation. Experimental behavior stays labeled experimental until
the evidence and fixtures support graduation.

## Before Submitting

Run these commands:

    go test ./... -count=1
    go test -race ./... -count=1
    go vet ./...
    bash -n install.sh tests/*.sh scripts/*.sh
    shellcheck -S warning -x install.sh tests/*.sh scripts/*.sh
    bash tests/run.sh
    bash scripts/check-docs.sh

For a release candidate, build the six offline archives with
`scripts/build-release.sh` and verify their checksums and exact contents. An
actual WSL smoke pass is required before publishing a release; status 77 from
`scripts/smoke-wsl.sh` is a skip, never a pass.

Review the diff for secrets, unrelated generated files, unsafe path expansion,
and missing English/Chinese parity. Use a conventional commit message and
describe the test evidence. Do not perform remote publication from a local
contribution workflow. Release archives contain precompiled binaries and
perform no runtime executable download.
