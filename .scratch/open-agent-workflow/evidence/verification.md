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
