# Machine Assurance

[简体中文](../zh/machine-assurance.md) | [README](../../README.md)

`oaw-assurance` is an optional, separately built command for attaching exact
Provider and Host Binding identities to references already declared by one
selected Markdown Policy Profile. It does not select a Profile, run engineering
work, maintain progress, or decide whether a Profile is usable.

The default `oaw` binary and installer do not import, build, install, start, or
manage this component. From a trusted source checkout, build or install it
explicitly:

```bash
go build -o ./oaw-assurance ./cmd/oaw-assurance

GOBIN="$HOME/.local/bin" go install ./cmd/oaw-assurance
```

The default release archive intentionally contains only `oaw`. Packaging or
publishing `oaw-assurance` requires a separate release decision.

## Inspect Profile References

Assurance uses the same read-only Profile reader as `oaw profile`. Built-in,
project, and user source qualifiers follow the same selection rules:

```bash
oaw-assurance overlay inspect --profile project:team-delivery
```

The command returns the exact Profile content digest and an ordered list of
opaque occurrence references with their declared binding references. It does
not copy Responsibilities, Rules, ownership, or workflow order into an Overlay.

```json
{
  "schema_version": "oaw.assurance-reference-index/v1",
  "profile": {
    "source": "project",
    "id": "team-delivery",
    "content_digest": "sha256:..."
  },
  "occurrences": [
    {
      "occurrence_ref": "profile-occurrence:sha256:...",
      "binding_reference": "test-skill"
    }
  ]
}
```

Free-form Profile content that cannot produce an unambiguous occurrence index
remains valid Policy content. Only the optional machine claim is unavailable.

## Issue an Overlay

An issuing integration supplies exact Binding identities and secret-free
evidence references. The `binding_reference` must equal the reference at the
selected occurrence. Digests use lowercase `sha256:<64 hex>` form.

```json
{
  "schema_version": "oaw.assurance-issue-request/v1",
  "issuer": "team-ci",
  "claims": [
    {
      "occurrence_ref": "profile-occurrence:sha256:...",
      "provider_id": "team/provider",
      "distribution_id": "provider",
      "distribution_revision": "0123456789abcdef0123456789abcdef01234567",
      "distribution_tree_digest": "sha256:...",
      "host_id": "codex",
      "surface": "codex-plugin",
      "binding_id": "test-skill",
      "binding_kind": "skill",
      "binding_reference": "test-skill",
      "invocation": "model",
      "binding_content_digest": "sha256:...",
      "evidence": [
        {
          "kind": "host-observation",
          "reference": "evidence://team-ci/test-skill",
          "digest": "sha256:..."
        }
      ]
    }
  ]
}
```

Issue from standard input or a regular input file:

```bash
oaw-assurance overlay issue \
  --profile project:team-delivery \
  --input request.json > overlay.json
```

Issuance reparses the original Markdown, pins its full-document digest, rejects
unknown or duplicate occurrences, requires exact binding-reference equality,
and canonicalizes claims into Profile occurrence order. Unknown JSON fields are
rejected. The resulting Overlay has no fields for Responsibilities, Skill
composition, order, Rules, Add-ons, Risk, Request Mode, topology, progress, or
completion.

## Verify an Overlay

Verification requires the same source-qualified Profile:

```bash
oaw-assurance overlay verify \
  --profile project:team-delivery \
  --input overlay.json
```

Verification fails if the Profile ID, source, full Markdown content, occurrence
mapping, Binding reference, claim order, evidence order, or artifact digest has
changed. It writes no Profile, Install State, Workflow State, lock, or receipt.

An Overlay is a content-addressed identity claim. It is not a signature, an
invocation receipt, completion evidence, a Host permission, or a sandbox. The
issuer remains responsible for the evidence it supplies. The separately
installed Host Bridge may provide current Host observations, but the dependency
remains one-way:

```text
oaw-bridge -> oaw-assurance -> read-only Profile reader -> Markdown Profile
```

If `oaw-assurance` is absent or any operation fails, only the machine claim is
absent. The Agent can still select and follow the Policy Profile using its
installed Skills and Host-native abilities.
