# ECC-FULL Selection Evidence

## Isolated run

- Temporary project: `/tmp/oaw-ecc-full.5w98kd/project`
- Isolated `HOME`, `XDG_CONFIG_HOME`, and `XDG_STATE_HOME`: under
  `/tmp/oaw-ecc-full.5w98kd/`
- Installation: project scope, Codex target only
- Assurance: `policy-cooperative`; no Assurance or Bridge component was
  installed
- Topology: current Host session only

The activated request was:

```text
Use OAW with ECC-FULL to deliver a deterministic rollout cohort command.
Keep the work in this project and use the ECC rules readable in the current
Host without requiring Provider or route metadata.
```

## Semantic Skill resolution

The managed `.oaw/policy/profiles/ECC-FULL.md` assigned the Responsibilities to
the ECC rules listed in `host-skills-index.txt`. Each Skill document was read
when its Responsibility became current. The selection depended on the declared
Responsibility and the Skill's natural-language purpose, not its Provider,
revision, physical cache path, route contract, or digest.

`ecc:intent-driven-development` produced bounded acceptance criteria;
`ecc:product-capability` separated the operator promise, stable hash contract,
and non-goals; `ecc:tdd-workflow` owned both RED/GREEN cycles; and
`ecc:verification-loop` guided final checks. `ecc:git-workflow` supplied the
checkpoint convention.

Semantic reading also caught that `ecc:blueprint` explicitly excludes a small
single-delivery task. Regression commit `2c76ebc` and Policy fix `6c0e8ff` made
the ECC-FULL planning assignment explicit: Policy Default owns ordinary work,
while `ecc:blueprint` is used only for its complex multi-session trigger. The
installed Profile contained that rule, so the dogfood did not fake a Blueprint
invocation or silently change planning ownership.

## Missing machine metadata did not block

With the isolated environment intentionally containing no Provider metadata,
`oaw check` reported `provider ecc: missing` (and the other two Providers
missing) while reporting the project Codex installation clean. The model-led
Policy workflow proceeded immediately from the readable Profile and Skill
rules. No eligibility form, Bridge evidence, lifecycle reducer, or route
admission was consulted.
