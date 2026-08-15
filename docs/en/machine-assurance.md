# Machine Assurance

`oaw-assurance` is an optional executable for users who need an exact,
machine-readable identity claim for one selected Markdown Profile.

## Assurance Overlay

An Overlay pins the full Profile digest and maps deterministic Skill or
Host-action occurrences to exact Provider, distribution, Host, Binding,
invocation, content, and evidence identities. Issue and verify operations fail
closed when a requested occurrence or Binding cannot be established.

An Overlay contains no engineering method, Responsibility ownership, ordering,
progress, approval, execution result, or completion claim. It cannot select or
modify a Profile and cannot authorize a Host action.

## Commands

```text
oaw-assurance overlay inspect --profile SOURCE:ID
oaw-assurance overlay issue --profile SOURCE:ID --input INPUT.json
oaw-assurance overlay verify --profile SOURCE:ID --input OVERLAY.json
```

The component reads Profiles through the shared read-only Profile inspector.
It is separately built and installed; the default `oaw` executable does not
depend on it.

Assurance failure means only that the requested machine claim is unavailable.
The selected Profile and normal Policy workflow remain unchanged.
