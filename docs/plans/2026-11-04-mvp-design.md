# Sela: portable personal-assistant toolkit

## Product contract

An agent should discover Sela's capabilities, learn their boundaries, and complete a user-selected assistant task using the CLI and its embedded instructions. The toolkit belongs to no particular model or agent harness. A shell-capable harness supplies execution and permissions; the model supplies reasoning. Sela does not bundle a model, autonomous scheduler, or claim that every agent will follow its instructions correctly.

Prove this first with Gmail and Google Calendar. The initial workflows are invitation-to-event-and-reply, morning briefing, meeting rescheduling, mail follow-up, and focus-time planning. Workflows describe prerequisites, clarification questions, approvals and partial-success recovery. Other Google products and personal tools remain future scope.

## Package boundaries

- `cmd/sela`: executable entry point.
- `internal/cli`: strict routing, explicit accounts, provider setup, plan commands; embedded help/skill.
- `internal/accounts`: nonsecret Google-account metadata, explicit resolution and rebinding refusal.
- `internal/auth`: official Google Desktop OAuth, PKCE/state/loopback callback, scope checking and serialized refresh.
- `internal/credentials`: OS keyring adapter, no plaintext-token fallback.
- `internal/actions`: typed, reviewable mail/calendar payloads and validation.
- `internal/plans`: expiring approval scope and durable one-attempt state machine.
- `internal/googleapi`: narrow operations over the official Gmail/Calendar SDKs.
- `internal/localstore`: private local files, atomic persistence and platform locking boundary.

No plugin framework or provider registry for the first two services. The v1 account registry is intentionally Google-only. Before adding another provider, introduce an explicit provider/account namespace and migration rather than guessing a provider from an alias. New capabilities need help, skill/workflows and scoped authority alongside their adapter.

## Auth and accounts

Use an explicitly selected alias/email and one of two profiles: `gmail-calendar-read` or `gmail-calendar-write`. Both include Gmail readonly/compose and identity; compose includes sending. Only the latter requests Calendar event write scope. No Gmail modify/full-mailbox scope, Calendar/ACL administration, ADC, App Password or ambient credential fallback.

Desktop installed-client JSON and official Google endpoints only. Loopback callback binds 127.0.0.1, random state and PKCE S256. The authenticated, verified Google email must match the selected account before credentials are stored. Missing/partial granted scopes refuse. Normal login never forces fresh consent; `--reauth` explicitly requests it if an offline refresh token is absent. Production consent configuration is distinct from verification and never promises permanent tokens.

Refresh reloads current keyring credentials under a per-account process lock, retains an omitted refresh token and persists changes before returning access credentials. A pending or changed local client/profile configuration fails closed. Logout deletes a local credential, not a claimed remote revocation.

## Payloads and writes

The public command flow is preview → human approval → execute. Plans bind account, action, exact payload, random ID and creation/expiry times with SHA256. They expire 15 minutes after creation; approval does not extend the deadline. Human approval requires the exact hash in a controlling terminal. No force/yes flag, piped approval, raw API command or classifier-based authorization.

State: prepared → approved → executing → succeeded/unknown. Persist the attempt before transport. Crashes, provider/transport errors and uncertain result persistence consume the attempt. Never reset/retry automatically. This provides one local attempt per plan, not distributed exactly-once delivery. Multi-product workflows can partially succeed; each compensating action would need a new explicit approval.

Mail uses immutable text/plain MIME, fixed account sender, exact To/Cc/Bcc/subject/body, attachment-byte snapshots and hashes, and a stable Message-ID. Attachment paths are not re-read at execute time. Exposed writes are draft creation and direct exact-snapshot sending. Draft update/send-by-ID is unavailable until concurrent external-change semantics are deliberately addressed; there is no hidden adapter branch for it.

Calendar create/update/delete targets one explicit calendar and timed single event. The preview includes attendees and explicit `send_updates` all/none; none is not a guarantee of zero notifications. Updates/deletes capture current event recipients and ETag; execute uses If-Match. Changed state needs new preview/approval. Date offsets must match the chosen IANA zone on that date, including DST. No recurring/all-day writes, conference/ACL/calendar administration or background schedule.

Invalid read arguments and invalid event-preview input must fail before credential/identity/API access. Validation stays shared where it prevents drift; do not replace a handful of typed operations with a generic API escape hatch.

## Portability and verification

Go 1.26+; official OAuth/Google SDK and native OS keyring libraries. No toolchain auto-install. macOS is the current native offline-test host. Linux has the Unix storage implementation; cross-compilation alone does not validate Linux runtime/keyring behavior. Windows currently supports stateless discovery only: private-storage ACL/locking and human-console support remain to be implemented and validated, not silently approximated with Unix file modes.

Test critical logic and interfaces: account isolation, secure persistence/locks, refresh retention, callback/identity checks, expiry/hash/replay, exact mail bytes, and Calendar ETag/notification contract. Do not chase coverage or build abstractions just to mock every getter. No test may touch real Google accounts or native keyring credentials; live validation requires separate user consent.

## Threat boundary

Private files and terminal approval guard against accidental misuse, not malicious same-user processes that can edit code/state or use the keyring. Local approval files are not cryptographic proof of human consent. Keep runtime data outside the public repository. Remote message/event contents are untrusted data, never commands or approval. No private data goes to secondary models/providers without explicit permission.
