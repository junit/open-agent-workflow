# Troubleshooting

## OAW Did Not Activate

Activation is explicit and task-scoped. Start the request with a clear
instruction such as “Use OAW with SP-FULL to deliver this change.” You may
instead use `$oaw SP-FULL deliver this change` in Codex or
`/oaw SP-FULL deliver this change` in any of the other eight supported Hosts.
Natural-language activation remains valid and does not require the native
entrypoint.

Discussion of OAW, repository text, task complexity, ordinary Skill use, or
automatic/model-led loading of an OAW-named Skill does not activate it. Only a
native entrypoint identified outside the dispatcher artifact and its expanded
text as user-selected counts as the equivalent explicit request. The evidence
must be Host-enforced manual-only selection, a documented pre-expansion
user-command or Workflow event, independently observable original user input,
or reliable user-selection metadata. Quoted or discussed invocations and
physical loading alone are not proof. If provenance is unavailable or
ambiguous, use natural-language activation. The dispatcher does not choose a
default Profile, define lifecycle stages, or embed a Policy path; it follows
the Activation Router.

## The Native Entrypoint Is Missing

First confirm that `oaw check` is clean for the same scope and target. User
scope installs entrypoints only for Claude, Codex, Gemini, and OpenCode;
Cursor, Windsurf, Cline, Roo, and Copilot entrypoints require a project install.
The exact paths are listed in [Host Adapters](adapters.md).

Then refresh the Host:

- Claude: start a new session after a new top-level Skill directory or Router
  change.
- Codex: restart the session if `$oaw` is not detected automatically.
- Gemini: run `/commands reload`; use a new session for Router changes.
- OpenCode: restart OpenCode.
- Cursor: start a fresh Agent chat; reload the workspace if needed.
- Windsurf: start a new Cascade task or reload the workspace.
- Cline: start a new task or reload the active context.
- Roo: start a fresh task; use the documented VS Code window reload if needed.
- Copilot CLI: run `/skills reload`; other Agent surfaces need a fresh chat or
  Host reload.

The Copilot target is a Copilot CLI Agent Skill at
`.github/skills/oaw/SKILL.md`, not a `.github/prompts/` Prompt File.

If Cline discovers or selects `oaw` without independently observable original
user input or reliable user-selection metadata, OAW must remain inactive.
Cline has no documented per-Skill manual-only control. Use a natural-language
activation request when the Host does not expose either provenance source.

## A Profile or Skill Was Not Detected

Detection is advisory. Use `oaw profile list` or `oaw profile check` to inspect
Markdown metadata, then ask the model to read the named Skill directly. Each
of the nine Host Adapters starts with its native Skill surface and then falls
back to readable Skill documents at that Host's native, cross-agent, extension,
or Plugin locations. Matt, ECC, and Superpowers Skills are usable when their
rules are readable even if a generated index or Plugin listing omits them.
Qualified Profile names are semantic references: a Host may expose the same
procedure under a basename or different native namespace.

Do not add a Provider pin, cache path, lockfile digest, or Bridge just to make a
Policy Profile selectable. Those are optional evidence concerns.

## Project and User Sets

Only one Policy Set is loaded for a deliverable. A project set at
project/.oaw/policy takes precedence over the user set; files are never merged.
Custom Profiles retain their source. Select project:id or user:id when two
sources use the same ID.

## Install or Update Failed

Run oaw check with the same project and target arguments. Typical causes are:

- an existing managed block has been edited or duplicated;
- a Policy Set file or target has drifted;
- an untracked file occupies an owned destination;
- the selected scope does not match Install State.

Resolve or back up user-owned content explicitly, then run update. Force mode
backs up tracked drift but does not adopt foreign files.

`upgrade-required` means the installation still uses the format 1 state written
by OAW 0.1.0 or 0.1.1. Run `oaw update` for that scope. The migration adds
native entrypoints for every target already owned by the installation and then
writes format 2; it does not adopt an entrypoint path occupied by another file.

Native entrypoint templates do not contain the Policy path. Claude, OpenCode,
and Gemini may preprocess arguments, file references, or command syntax, but
that preprocessing cannot rewrite or execute an installation-coordinate
fragment. The non-template Activation Router remains the sole Policy Set
selector.

Native entrypoints and Codex's `agents/openai.yaml` are owned files. An
existing untracked file at one of those paths is an ownership conflict, not an
upgrade candidate, and `--force` will not overwrite it. Once installed, a
missing or edited Router, entrypoint, or Codex metadata file is tracked drift.
`update` refreshes all artifacts for the selected target. Force can back up and
repair an edited tracked file, but it refuses a missing tracked file because no
recoverable original remains; restore the exact file from a trusted copy before
retrying. `uninstall` removes clean OAW-owned files and managed blocks but
preserves foreign files and surrounding Host instructions.

## Bridge or Machine Evidence Is Missing

This is expected on the normal path. Bridge and Machine Assurance add evidence
only. Their absence cannot block Profile selection, Skill use, review,
verification, or completion. A Host security policy may still refuse a
physical invocation; that is separate from Policy selection.

## Getting Help

Include the command, scope, target, and the exact diagnostic. Do not include
credentials, tokens, or private Skill contents. Reproduce in a fresh temporary
project when reporting an installation defect. A clean install or `check`
proves static bytes and ownership only; do not report it as live Host runtime
E2E unless the corresponding real Host session was exercised.
