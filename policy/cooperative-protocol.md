# OAW Cooperative Protocol

This protocol operationalizes the [Open Agent Workflow Policy](POLICY.md)
through the Agent Host's normal conversation, planning, Skill, and tool
surfaces. It is a model-readable procedure, not a state machine or machine
authority record.

## Activation

When the user explicitly activates OAW for a deliverable:

1. Load the selected Canonical Policy Set and current Host Adapter.
2. If the user names a Profile, select it and begin without asking them to
   repeat the choice.
3. Otherwise inspect only the identity and Purpose needed to distinguish
   available Built-in and Custom Profiles, state a reasonable selection, and
   proceed unless materially different methods create genuine ambiguity.
4. Load the complete selected Profile.
5. Load each referenced Skill only when its Responsibility becomes current.

Do not present a startup form for topology, an Add-on sentinel, Complexity,
Risk, Policy limitations, Provider evidence, or assurance level. Mention a real
limitation when it affects the work, not as a ritual confirmation.

## Profile Sources

Load the Project Policy Set when the project contains one; otherwise load the
User Policy Set. Never merge their core Policy files.

Candidate Profiles consist of the selected Policy Set's Built-in Profiles plus
readable project and user Custom Profiles. Keep source qualifiers when two
Custom Profiles share an ID. Built-in IDs are reserved and cannot be shadowed.

The Built-in Profiles are:

- [SP-FULL](profiles/SP-FULL.md)
- [MATT-FULL](profiles/MATT-FULL.md)
- [ECC-FULL](profiles/ECC-FULL.md)
- [MATT-SP-HYBRID](profiles/MATT-SP-HYBRID.md)

## Natural-Language Selection

Examples of complete user intent include:

```text
Use MATT-FULL for this deliverable.
Use team-delivery and add e2e-testing.
Create a project Profile from the installed planning, TDD, review, and
verification Skills, then use it.
```

Selection authorizes every Skill declared by the Profile for the current
deliverable. It does not grant physical permissions. Use Host-native Skill
invocation when available; otherwise follow readable Skill instructions as
rules and describe that accurately.

## Model-Led Skill Resolution

For a declared Skill, evaluate these sources in order:

1. the current Host-provided Skill index or native invocation surface;
2. readable Skill instructions at locations documented by the Host Adapter;
3. alternatives explicitly declared by the Profile;
4. semantically equivalent installed Skills;
5. the corresponding Policy Default or Host-native behavior.

An index or diagnostic is advisory. A Skill is unavailable only when the Agent
cannot read its rules and the Host cannot invoke it.

Use a declared alternative directly and report it. An undeclared substitution
may proceed with an explanation only when it preserves the method and
Responsibility ownership. Obtain user confirmation before changing TDD, review,
verification, security, or ownership semantics. Do not substitute a Skill that
the Profile marks mandatory.

## Partial Profiles and Defaults

For each Responsibility omitted by a Custom Profile, apply the Policy Default.
Do not infer that review, verification, or safety disappeared. If a Profile says
a Responsibility is not applicable, evaluate that claim against the current
deliverable and explain any disagreement.

## Add-ons

An Add-on is a task-scoped specialist Skill named by the user or recommended by
the Agent. It contributes one bounded result and then returns control to the
selected Profile. There is no `NONE` selection and no startup Add-on form.

An Add-on does not acquire a core Responsibility. A persistent change to the
engineering method belongs in a new or edited Profile.

## Progress

Use the Host's native planning and conversation state for ordinary work. For a
long or cross-session deliverable, a Markdown Progress Note may record:

- deliverable and selected Profile with source;
- task-scoped Add-ons;
- completed Responsibilities and artifacts;
- current Responsibility;
- review and verification evidence;
- next step and unresolved decisions.

A Progress Note is a continuity aid, not authority. A missing, stale, or damaged
note cannot block work. Reconstruct only facts supported by current artifacts
and ask when a material decision is uncertain.

## Profile Switching

The user may switch Profiles at any time. Summarize completed Responsibilities
and artifacts, map them to the new Profile, reuse compatible outcomes, and
perform only missing or strengthened work.

If switching would reduce an important TDD, review, verification, security, or
safety guarantee, explain the reduction and obtain explicit confirmation. No
CLI command, stable-boundary state machine, or machine record grants permission
to switch.

## Review, Remediation, and Verification

Treat review as an outcome, not a ceremonial invocation. Compare the result
with the selected Profile, approved requirements, repository standards, and
relevant risks. Findings return to the declared implementation owner. After
remediation, review again and run fresh verification.

Fresh verification must observe current output after the final change. Select
checks proportionate to behavior and risk. Do not reuse stale output to claim
completion.

## Uncertainty and Host Refusal

When Skill meaning, prior progress, or a material decision cannot be recovered,
state the known facts and ask the narrow question needed to continue. Do not
reconstruct authority or claim work that cannot be evidenced.

When the Host refuses a physical invocation, distinguish the cause:

- try a valid readable-Skill or Host-native alternative when the Profile permits;
- request a required user action when only the user can perform it;
- explain the actual Host limitation when no valid path remains.

Machine Assurance may refuse a machine claim, but that does not make the
Profile unavailable. Host security may still refuse physical execution.

## Closeout

Close the deliverable only after required review findings are resolved and
fresh verification has run. Report:

- the delivered behavior and important files or artifacts;
- tests, review, and verification actually performed;
- substitutions or Add-ons used;
- residual risks or unavailable checks;
- repository or external actions actually authorized and completed.

Never report a native Skill invocation, machine attestation, receipt, lock, or
guarantee that did not occur.
