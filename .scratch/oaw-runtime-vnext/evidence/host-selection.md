# First Production Runtime Host Selection Evidence

**Audit date:** 2026-08-03

**Selection status:** `selected`

```yaml
selected_host: codex
selected_integration_id: oaw/codex-runner
selection_source: user-explicit
selected_at: 2026-08-03
```

This artifact records the pre-selection capability evidence and, after an
explicit user decision, the first production Runtime Host choice required by
Ticket 09. Discovery, local installation, an official automation interface, or
an OAW recommendation never selects or promotes a Host by itself.

## Admission Boundary

Before Ticket 09, every built-in Host integration was `instruction-only` with an
`audit.status` of `pending`. The selected Host is promoted only after its exact
Manifest, official capability references, adapter audit, and OAW conformance
report are pinned in one trusted integration record. All non-Codex integrations
remain `instruction-only` and continue to make no Runtime claims.

The official CLI sources below prove automation transport features only. They
do not prove the full OAW feature set. OAW-specific features remain pending
until the selected adapter passes the Ticket 08 conformance harness and its
real-process integration tests.

## Candidate Evidence

| Candidate | Locally observed version | Official automation evidence | Current OAW conclusion |
| --- | --- | --- | --- |
| Codex CLI | `codex-cli 0.146.0` | `codex exec` is non-interactive; `--json` emits JSONL events; progress is on stderr and the final message is on stdout; `--output-schema` constrains final structured output; `--ephemeral` avoids persisted rollout files; explicit read-only/workspace-write/danger-full-access sandboxes are documented. | Recommended first `runner-managed` candidate. It has the clearest machine stream and sandbox contract, but OAW Binding delivery, deduplication, pause/cancellation, Bundle inheritance, and observation normalization still require adapter implementation and conformance. |
| Claude Code | `2.1.212` | Print mode supports text, JSON, and stream-JSON input/output; `--json-schema` constrains final output; permission modes are explicit per invocation. | Viable alternate candidate. Official structured transport is strong, but physical isolation and the complete OAW control feature set are not established by the CLI reference alone. |
| Gemini CLI | `0.46.0` | Headless mode supports JSON and streaming JSON events for session, messages, tool use/results, errors, and final results, with documented exit codes. | Viable later candidate. Its basic transport is sufficient for adapter prototyping, but the first promotion would carry more error-normalization risk and still requires the same OAW conformance work. |

## Selected Host

The user explicitly selected **Codex CLI** as the first production Runtime Host
on 2026-08-03. OAW records it as the `runner-managed` integration
`oaw/codex-runner`, not `native-managed`. The reference `oaw run` process
remains the Runtime Protocol authority and invokes an exact Codex CLI process
binding. Codex output is untrusted Host output and must be bounded, strictly
parsed, normalized into OAW's closed observation vocabulary, and kept out of
authoritative state except through digest-pinned evidence references.

Ticket 09 completed the promotion gates at implementation fixed point `0cb396d`.
The Codex Runtime claims are limited to the pinned `runner-managed`
`oaw/codex-runner` record. `oaw run --host codex` checks that exact record at
startup; no discovery result, project configuration, or unsupported Host can
enable a Runtime entrypoint.

## Pinned Runtime Identities

| Asset | Digest |
| --- | --- |
| Manifest | `1cdbf2d09620d585486000418f1770fc604ae323a9cd8c27bd3e0bdef5c30d5d` |
| Audit evidence | `2e0e3c9e74bae4a8d249507ac1573596f5ad06964c19b8c739b2ef1e093052ec` |
| Conformance report | `7ea7026fda4146cbd6a19db8ea25168a9c02bf2f81f67a29ba37ab3fac419e4b` |
| Integration record | `bea2b3a7caee2062e7b058a8fbfe1187adfc5c60ac7c033275d02e251393d303` |

## Official Sources

Retrieved 2026-08-03:

- OpenAI, [Codex non-interactive mode](https://developers.openai.com/codex/non-interactive-mode).
- OpenAI, [Codex agent approvals and security](https://developers.openai.com/codex/agent-approvals-security).
- Anthropic, [Claude Code CLI reference](https://code.claude.com/docs/en/cli-reference).
- Google, [Gemini CLI headless mode reference](https://github.com/google-gemini/gemini-cli/blob/bc75a6198298560d2ab533c8b3f5404c40536bcc/docs/cli/headless.md).

## Selection Record Procedure

After the user chooses a Host:

1. Record the exact Host ID, integration ID, `user-explicit` selection source,
   and decision date in the fields above.
2. Record the user's exact choice in Ticket 09 and the workflow tracker.
3. Produce the executable Ticket 09 implementation plan for only that Host.
4. Keep every other built-in integration `instruction-only`.
5. Promote the selected integration only after official audit references and a
   passing, digest-pinned conformance report exist at the implementation fixed
   point.
