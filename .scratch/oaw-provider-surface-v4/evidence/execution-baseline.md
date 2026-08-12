# OAW Provider Surface v4 Execution Baseline

implementation_base: 2a4e9fb189e01d5c3f6fe7242d3f65bf500656e7
source_branch: main
implementation_branch: feat/oaw-provider-surface-v4
original_workspace_status_digest: f376f612c86287b65667c2cd424cf9e251a9dd651cf75b91d00e8fb8b6eb0cd5
selected_lifecycle: SP-FULL / CURRENT / no Add-on
recorded_at: 2026-08-10T00:00:00+08:00

baseline_verification:
- `rtk go test ./... -count=1`: 1425 passed, 1 failed, 1 skipped; the existing
  `internal/codexbridge/appserver/TestCodexLauncherOutlivesOpeningRequestContext`
  failed with `context deadline exceeded`.
- `rtk go test ./internal/codexbridge/appserver -run '^TestCodexLauncherOutlivesOpeningRequestContext$' -count=5`: 5 passed.
- The focused pass confirms the known timing/load flake; the test remains
  unchanged and is not excluded from later verification.

The source checkout contained pre-existing user changes. This worktree starts
clean at the immutable implementation base; those unrelated changes are not
part of this implementation.
