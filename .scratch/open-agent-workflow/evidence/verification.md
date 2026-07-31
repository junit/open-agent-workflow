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

## Ticket 05 - Drift and Hardening (Tasks 1-2 Checkpoint)

Verified on 2026-07-31 in Bash 3.2.57 on branch
`feature/oaw-ticket-02` through commit `c5763b5`.

- `bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh`: exit 0.
- `shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh`: exit 0.
- `bash tests/03-claude-lifecycle-test.sh`: 9/9 behavior cases passed.
- `bash tests/05-policy-coordination-test.sh`: 8/8 behavior cases passed.
- `bash tests/06-security-test.sh`: 10/10 behavior cases passed.
- `bash tests/run.sh`: 66/66 behavior cases passed; suite summary passed with
  exit 0.
- `git diff --check`: exit 0.
- Secret-pattern scan across changed implementation and tests: no matches.

TDD and debugging evidence retained in the task history:

- Inconsistent shared-destination state initially reported an installed target
  as clean before the closed state schema rejected it.
- Managed block drift initially failed without the required target/path
  diagnostic.
- Update with untracked markers and no state initially stopped at the generic
  no-state error before inspecting the selected destination.
- Cross-scope update initially accepted and rewrote a drifted candidate state.
- Final uninstall initially accepted a drifted retention candidate and could
  delete the current scope while retaining policy.
- Multi-candidate retention initially returned after an early healthy state
  and failed to inspect a later drifted state.
- The full regression suite exposed a historical noncanonical `other.state`
  fixture. Replacing it with a public-CLI project installation restored the
  intended policy-retention test without weakening candidate validation.

## Ticket 05 - Drift and Hardening (Task 3 Re-review)

Verified on 2026-07-31 in Bash 3.2.57 on branch
`feature/oaw-ticket-02` through commit `74ff3ec`.

- `bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh`: exit 0.
- `shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh`: exit 0.
- `bash tests/06-security-test.sh`: all 17 behavior cases passed.
- `bash tests/07-containment-test.sh`: all 5 behavior cases passed.
- `bash tests/run.sh`: every implemented behavior case and the suite summary
  passed with exit 0.
- `git diff --check`: exit 0 before the remediation commit.
- Secret and dangerous-evaluation scans across the remediation implementation
  and test found no matches.

TDD and remediation evidence:

- A direct apply-boundary probe exited 65 and avoided the outside target file,
  but demonstrated the original bug by reporting `outside_rules=created`.
- After the black-box race fixture stopped pre-creating outside `rules/`,
  `bash tests/07-containment-test.sh` failed RED with
  `FAIL: apply-time parent swap created an outside directory`.
- Component-relative directory creation and per-component physical-coordinate
  verification made the same race fixture GREEN without weakening the symlink
  diagnostic or the no-state-mutation assertion.
