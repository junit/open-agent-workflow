# 01 - Installer foundation and check command

**What to build:** A portable, read-only OAW command surface that parses the
supported CLI contract, resolves isolated configuration and state locations,
detects workflow providers and agent tools, and reports actionable readiness
without changing the filesystem.

**Blocked by:** None - can start immediately.

**Status:** done

- [x] Running `install.sh` with no command or with `--help` prints usage and
      performs no mutation.
- [x] Unknown commands, unknown flags, missing flag values, invalid targets,
      and unsupported target/scope combinations fail before mutation.
- [x] `check` supports `--target`, `--project`, and isolated `HOME`,
      `XDG_CONFIG_HOME`, and `XDG_STATE_HOME` environments.
- [x] Provider detection reports Superpowers, Matt, and ECC independently and
      never installs, updates, or removes them.
- [x] Tool detection distinguishes supported, missing, and unavailable-scope
      targets without choosing a lifecycle profile for the user.
- [x] A black-box shell test harness executes the real `install.sh` in
      disposable directories and proves that read-only commands do not touch
      the operator's actual configuration.
- [x] The shell entrypoint parses under Bash 3.2-compatible syntax and has no
      required Node.js, Python, jq, or package-manager dependency.
