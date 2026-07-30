# 05 - Drift, backups, force, and filesystem hardening

**What to build:** Make every mutating lifecycle operation fail closed on
unexpected content or unsafe paths, while providing an explicit, recoverable
force path and exact uninstall behavior.

**Blocked by:** 02 - Claude user-scope lifecycle; 03 - Remaining core user
adapters; 04 - Project-scope core and extension adapters.

**Status:** ready-for-agent

- [ ] Install state records version, policy checksum, scope, targets,
      destinations, managed checksums, ownership, and backup references in a
      data format that is parsed without shell evaluation.
- [ ] Modified managed content, malformed markers, duplicate markers,
      out-of-order markers, and mismatched state are reported as drift and
      block update or uninstall by default.
- [ ] `--force` creates a timestamped operation backup before the first
      mutation, records it in state or output, and then performs only the
      requested scoped replacement or removal.
- [ ] All destinations are resolved and validated before any write so an
      invalid later target cannot leave an earlier target partially changed.
- [ ] Project path containment rejects traversal, non-directory roots, and
      destinations that escape the selected project through symlinks.
- [ ] Existing symlinked target files and directories follow a documented,
      conservative policy and cannot redirect writes outside the selected
      scope.
- [ ] Hostile filenames and state values remain inert; no state or filesystem
      content is evaluated as shell code.
- [ ] Black-box security tests cover drift, forced backup and recovery data,
      marker corruption, containment, symlinks, hostile paths, partial-write
      prevention, and exact uninstall.

