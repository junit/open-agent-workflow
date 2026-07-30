# Open Agent Workflow v0.1 Specification

## Problem Statement

Developers increasingly combine multiple agent engineering methodologies and
multiple coding-agent clients. Each methodology may automatically claim
planning, implementation, testing, debugging, review, and completion. When
several are installed together, automatic triggers can start overlapping
lifecycles, duplicate artifacts, change methods mid-task, or silently bypass a
user's preferred workflow.

The same user may also work in Claude Code, Codex CLI, Gemini CLI, OpenCode,
Cursor, Windsurf, Cline, Roo Code, and GitHub Copilot. These tools use different
instruction filenames, scopes, precedence rules, and reload behavior. Manually
maintaining equivalent governance rules across them creates drift and makes
safe upgrades or removal difficult.

Users need one open, provider-neutral policy that classifies an engineering
task, presents explicit lifecycle choices, locks the selected profile for the
deliverable, and installs safely into multiple agent tools without overwriting
unrelated configuration.

## Solution

Open Agent Workflow provides a canonical engineering-workflow arbitration
policy, target-specific thin entrypoints, and a zero-dependency shell
installer. The policy supports full-family lifecycle profiles, an explicit
quality-oriented hybrid profile, and custom locked ownership maps. Providers
remain independently installed and versioned.

The installer detects providers and supported agent tools, explains missing
capabilities, and manages user- or project-scoped entrypoints through
recoverable, checksummed operations. It supports check, install, update, and
uninstall lifecycles; repeated operations are idempotent. User-owned content is
preserved, drift blocks mutation by default, and forced mutation always creates
a backup first.

The repository documents the motivation, the initial comparison of
Superpowers, Matt Pocock skills, and Everything Claude Code, the lifecycle
model, the adapter architecture, extension contracts, security boundaries, and
operational troubleshooting in English and Chinese.

## User Stories

1. As a developer with several workflow providers installed, I want to choose
   one lifecycle profile before engineering work starts, so that providers do
   not compete for the same responsibility.
2. As a developer, I want ordinary and complex tasks classified consistently,
   so that the profile recommendation matches the task's uncertainty and
   scope.
3. As a developer, I want every available profile shown before selection, so
   that recommendations never become silent defaults.
4. As a developer, I want my selected profile locked to the deliverable, so
   that follow-up requests do not change methodology unexpectedly.
5. As a developer, I want delegated agents to inherit the exact bundle, so that
   they do not reopen arbitration or add a second owner.
6. As a developer, I want full Superpowers, Matt, and ECC profiles, so that I
   can use one family for an entire task when appropriate.
7. As a developer, I want a predefined Matt-Superpowers hybrid, so that each
   lifecycle stage uses its selected strongest owner without ambiguity.
8. As a developer, I want exact specialist add-ons named in the bundle, so that
   a specialist cannot take over the lifecycle.
9. As a developer, I want missing provider capabilities reported before work,
   so that required stages are never silently omitted.
10. As a developer, I want provider installation kept separate from OAW, so
    that upstream licenses, versions, and configuration remain under my
    control.
11. As a Claude Code user, I want OAW available in my user instructions, so
    that the same selection gate applies across projects.
12. As a Codex CLI user, I want OAW available in my global AGENTS instructions,
    so that automatic skills remain subordinate to the locked profile.
13. As a Gemini CLI user, I want OAW available through its supported context
    mechanism, so that I can use the same lifecycle policy.
14. As an OpenCode user, I want OAW installed through its supported global rule
    surface, so that no undocumented import behavior is required.
15. As a Cursor user, I want a valid project rule adapter, so that OAW can be
    enabled without editing GUI-only user rules.
16. As a Windsurf user, I want a current project rule adapter with legacy
    behavior documented, so that I do not depend on deprecated filenames.
17. As a Cline user, I want a project rule adapter, so that OAW combines with
    my workspace instructions predictably.
18. As a Roo Code user, I want a project rule adapter in its preferred rule
    directory, so that OAW loads automatically without owning custom modes.
19. As a GitHub Copilot user, I want a repository instruction adapter, so that
    OAW participates in supported project instruction loading.
20. As a user, I want one canonical rule source, so that target entrypoints do
    not drift into separate policies.
21. As a user, I want OAW configuration to follow XDG conventions, so that OAW
    does not claim a provider-owned namespace.
22. As a user, I want a dry run, so that I can inspect every planned mutation
    before it happens.
23. As a user, I want target selection, so that I can install only the clients
    I use.
24. As a user, I want a project path option, so that I can generate scoped
    adapters without changing global configuration.
25. As a user, I want installation to be idempotent, so that rerunning the same
    version leaves the filesystem unchanged.
26. As a user, I want updates to use my current checkout, so that upgrades are
    reproducible and do not execute unreviewed remote code.
27. As a user, I want drift reported, so that local customizations are never
    overwritten silently.
28. As a user, I want forced mutation to create a backup first, so that I can
    recover from an intentional replacement.
29. As a user, I want uninstall to remove only OAW-owned artifacts, so that my
    unrelated instructions survive.
30. As a user, I want created files removed only when they contain no unrelated
    content, so that uninstall cannot destroy later additions.
31. As a user, I want machine-readable status to remain a future-compatible
    option, so that editors and package managers can integrate later.
32. As a contributor, I want an adapter contract and fixtures, so that I can
    add a new agent tool without changing lifecycle semantics.
33. As a contributor, I want deterministic tests under an isolated home, so
    that adapter changes cannot touch my real configuration.
34. As a security reviewer, I want path containment and symlink behavior
    tested, so that crafted destinations cannot escape the selected scope.
35. As an open-source evaluator, I want the three-family comparison labeled
    with its criteria and limitations, so that experience-based scores are not
    presented as universal benchmarks.
36. As a Chinese-speaking user, I want a complete Chinese guide, so that I can
    understand the same product and safety model as English readers.
37. As an English-speaking user, I want a complete English guide, so that the
    project is accessible to the wider open-source community.
38. As a maintainer, I want explicit support levels, so that experimental
    adapters are not mistaken for stable global integrations.
39. As a maintainer, I want provider detection separated from profile
    selection, so that detection never changes a user's lifecycle choice.
40. As a maintainer, I want release and remote publication outside the local
    build workflow, so that no external repository is created without owner
    approval.

## Implementation Decisions

- The project is a provider-neutral arbitration and adapter layer. It does not
  redistribute, install, update, or patch workflow providers.
- The canonical policy defines task classification, a blocking selection gate,
  full-family profiles, a predefined hybrid, bounded specialist add-ons,
  lifecycle persistence, subagent inheritance, and stable switching
  boundaries.
- The initial provider comparison covers planning, implementation, TDD,
  debugging, review, and completion verification. Numeric scores are an
  experience-based design input with documented criteria, not an empirical
  universal benchmark.
- Configuration follows XDG conventions. Configuration artifacts and mutable
  install state are separated.
- Core adapters support user and project scope. Extension adapters are
  officially supported at project scope and may document best-effort global
  paths without mutating them by default.
- Claude and Gemini adapters may use their documented import behavior. Codex
  and OpenCode adapters use model-visible bootstrap instructions rather than
  relying on undocumented Markdown imports.
- Marker delimiters are mechanical ownership boundaries only. They do not
  establish model precedence.
- Installer commands are explicit subcommands: check, install, update, and
  uninstall. Invoking the script without a command prints help and performs no
  mutation.
- Installer selection includes target, project scope, dry-run, and force
  controls. Unknown arguments and unsupported target/scope combinations fail
  before mutation.
- The installer is compatible with Bash 3.2 and common macOS, Linux, and WSL
  utilities. It requires no Node.js, Python, jq, or package manager.
- Every mutating operation resolves and validates all destinations before the
  first write.
- Existing target files are merged through one uniquely named OAW block.
  Invalid, duplicate, or out-of-order markers are treated as drift.
- State records the source version, policy checksum, selected scope, targets,
  destinations, managed checksums, and backup references without evaluating
  state content as shell code.
- A repeated install or update with identical inputs performs no content
  writes and reports an unchanged result.
- Update reads artifacts only from the current checkout. Network fetching and
  self-update are extension points, not v0.1 behavior.
- Uninstall removes managed blocks and OAW-owned files. It preserves
  user-owned files and refuses drifted removals unless forced after backup.
- Backups are timestamped, operation-scoped, and stored separately from
  configuration. Backup creation precedes forced mutation.
- The repository is initialized locally with an Apache-2.0 license. Creating a
  remote repository, pushing, publishing releases, and claiming global package
  names require separate approval.

## Testing Decisions

- The primary and only functional test seam is black-box execution of the real
  installer CLI.
- Every test supplies isolated HOME, XDG configuration, XDG state, and project
  directories. Tests never access the operator's real agent configuration.
- Tests assert externally visible exit codes, output, installed content,
  checksums, state, backups, and preserved user content.
- Contract cases cover help and argument validation, detection, fresh install,
  repeated install, local update, clean uninstall, forced drift handling, dry
  run, target filtering, user scope, project scope, and every adapter path.
- Security cases cover untrusted project paths, path containment, symlinked
  destinations, malformed markers, duplicate markers, hostile filenames, and
  state parsing without shell evaluation.
- Static shell syntax and lint checks supplement the black-box tests but do not
  create another functional testing seam.
- Tests do not source installer internals or assert private function names.

## Out of Scope

- Installing, vendoring, upgrading, or licensing Superpowers, Matt Pocock
  skills, ECC, or another workflow provider.
- Native Windows PowerShell support. Windows users use WSL in v0.1.
- Automatic GitHub Release downloads, remote main-branch updates, or executing
  code fetched by the installer.
- Creating or pushing a GitHub repository, publishing a package, or reserving
  a domain name.
- A GUI, hosted service, telemetry, accounts, synchronization server, or
  central policy registry.
- Automatically selecting a lifecycle profile without user input.
- Treating HTML comments or target-specific marker syntax as semantic model
  precedence.
- Mutating GUI-only global rules for extension adapters.
- Installing project-specific workflow providers or resolving provider license
  incompatibilities.

## Further Notes

- `open-agent-workflow` is available as a local directory and was not found as
  an npm package at discovery time. Other GitHub users may use the same
  repository name; GitHub names are owner-scoped.
- Official adapter behavior must be documented with source URLs and retrieval
  dates because client instruction surfaces evolve quickly.
- Claude strips block-level HTML comments before model injection. OAW markers
  therefore exist for deterministic file management, not for model-visible
  policy boundaries.
- Codex has a global instruction size limit. Thin entrypoints keep the
  canonical policy out of duplicated target files while still directing the
  agent to read it before workflow selection.

