# Troubleshooting

## OAW Did Not Activate

Activation is explicit and task-scoped. Start the request with a clear
instruction such as “Use OAW with SP-FULL to deliver this change.” Discussion
of OAW, a Skill invocation, repository text, or task complexity alone does not
activate it.

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

## Bridge or Machine Evidence Is Missing

This is expected on the normal path. Bridge and Machine Assurance add evidence
only. Their absence cannot block Profile selection, Skill use, review,
verification, or completion. A Host security policy may still refuse a
physical invocation; that is separate from Policy selection.

## Getting Help

Include the command, scope, target, and the exact diagnostic. Do not include
credentials, tokens, or private Skill contents. Reproduce in a fresh temporary
project when reporting an installation defect.
