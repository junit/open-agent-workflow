# Release Manifest Check Plan

The `release-check` Custom Profile owns framing, executable planning, TDD, and
review. Specification, workspace safety, fresh verification, and closeout are
intentionally omitted and therefore use Policy Defaults.

1. Add failing validator tests, implement default `version`/`commit` checks,
   and commit the first real `manifestcheck` artifact.
2. Start a fresh task that explicitly reuses `project:release-check`; add
   failing tests for `ValidateRequired` and `--require`, implement the smallest
   extension, and keep default behavior unchanged.
3. Use the readable security-review Skill when its native route is absent,
   record that fallback, run fresh verification after removing `oaw`, and
   confirm project/user Profile conflict visibility.
