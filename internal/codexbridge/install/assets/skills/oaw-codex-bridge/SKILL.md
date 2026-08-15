---
name: oaw-codex-bridge
description: Use only when an explicitly selected Markdown Policy Profile needs optional current Codex Binding assurance.
---

# OAW Codex Assurance Bridge

Call `observe_profile` with one source-qualified Profile selector when the user
or an audit integration explicitly requests machine assurance. The operation
uses current Codex `skills/list` metadata, exact Provider distribution and
Binding content evidence, and the standalone Assurance module to return one
Assurance Overlay bound to that Profile.

The Overlay identifies the exact Profile snapshot and covered Bindings. It
does not select or run the Profile, classify work, grant permission, coordinate
progress, prove invocation or completion, or constrain the Agent Host.

Bridge installation, Plugin files, and `oaw-bridge check` prove installation
integrity only. The PreToolUse Hook supplies current session and project context
to `observe_profile`; hand-authored Hook JSON is still cooperative Host input,
not a signature or operating-system attestation.

Accept only Bridge v3 and Hook Context v3. Do not infer missing Bindings, change
Profile Responsibilities or Rules, or substitute another Skill. A missing,
revoked, failed, or incomplete Bridge removes only the optional machine claim.
The Markdown Profile remains usable through the normal rule-driven path.
