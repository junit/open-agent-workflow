# Open Agent Workflow Verification Evidence

## Ticket 03 - Core User Adapters

Verified on 2026-07-30 in Bash 3.2.57 on branch
`feature/oaw-ticket-02`.

- `bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh`: exit 0.
- `shellcheck -s bash -e SC2016 -x -P . install.sh lib/*.sh lib/commands/*.sh tests/*.sh`: exit 0.
- `bash tests/02-check-test.sh`: 4/4 cases passed.
- `bash tests/04-core-adapters-test.sh`: 9/9 cases passed.
- `bash tests/run.sh`: 31/31 test cases passed; suite summary passed.
- Repeated default `install`: policy, four target files, and state all reported
  `unchanged`; path, mtime, and inode fingerprints were identical.
- Duplicate target-state probe: exit 65 with no policy, target, or state
  mutation.

TDD evidence retained in the task history:

- Initial Codex, Gemini, and OpenCode installs failed at their previous
  Claude-only path before each renderer slice was implemented.
- Multi-target install failed with exit 69 before normalized record merging.
- Codex update failed with exit 69 before lifecycle generalization.
- Installed-health assertions failed before `check` gained state diagnostics.

## Ticket 04 - Project and Extension Adapters

Verified on 2026-07-31 in Bash 3.2.57 on branch
`feature/oaw-ticket-02` through commit `e1ea03e`.

- `bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh`: exit 0.
- `shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh`: exit 0.
- `bash tests/05-project-adapters-test.sh`: 17/17 behavior cases passed.
- `bash tests/05-policy-coordination-test.sh`: 8/8 behavior cases passed.
- `bash tests/run.sh`: 56/56 behavior cases passed; suite summary passed.
- `git diff --check`: exit 0.
- Secret-pattern scan across changed implementation and tests: no matches.
- Repeated default project install retained identical path, checksum, inode,
  mtime, and size fingerprints for all ten managed artifacts.
- A three-state probe (user plus two projects) updated one project and left all
  three checks clean with the new version recorded in all three states.

TDD and remediation evidence retained in the task history:

- Sequential Codex then OpenCode initially exited 65 on the tracked shared
  marker before shared-destination join support was added.
- Project update initially left user state on the old version and reported
  `installed codex: drift`; project final uninstall then removed the shared
  policy.
- Path-aware retention initially removed the policy when another syntactically
  valid state referenced the same path with an older checksum.
- A tampered cross-scope project root initially allowed update to exit 0 and
  rewrite policy plus both states; the binding regression now exits 65 before
  any managed fingerprint changes.
