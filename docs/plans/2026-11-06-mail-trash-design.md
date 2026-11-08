# Selected Gmail Trash

Implementation approved for offline development only. Live Gmail, login/reauth, installation and release require separate authorization. This document contains no user mail or credentials.

## Contract

`mail preview --account ALIAS --action trash --id MESSAGE` fetches bounded metadata for exactly one message and creates one immutable 15-minute plan. The review shows sender, subject, date, message/thread identifiers and full labels; metadata is untrusted. Incomplete, duplicate or oversized metadata refuses preview. Only INBOX messages without TRASH, SPAM, DRAFT, SENT, STARRED or IMPORTANT are eligible. There is no override, thread operation, bulk/query mutation, generic label mutation, permanent deletion or restore. For multiple selected messages, each is a separately previewed/approved/executed plan; there is no batch executor or automatic continuation. Existing `mail search` remains read-only, paginated and explicit; category results are candidate discovery, not a mutation scope or proof of complete mailbox coverage.

The existing connection must have explicit `--mail-modify` opt-in and granted Gmail modify scope. Existing connections are never silently widened. OAuth consent is distinct from actual human chat approval bound by account, plan, hash, expiry and exact message. Consent records are agent attestations, not independent chat authentication.

Execution claims the plan durably before a fresh bounded metadata read. Full reviewed metadata and sorted label snapshot must match and still pass the eligibility policy, or no Trash request is sent. One `messages.trash` request follows, then an independent bounded read must observe the same ID with TRASH and without INBOX. Gmail provides no atomic conditional check in this flow: concurrent clients can change the message between read and write, and readback cannot prove causation. Trash is not permanent delete but Gmail retention/account policy may later delete it permanently.

All errors after plan claim, including a stale pre-write check where no write was sent, conservatively leave the plan unknown; a crash may leave it executing. Both consume the one attempt. Stop and reconcile with a separately authorized read; no automatic retry, replacement, compensation or continuing a batch. No exactly-once guarantee is claimed across clients. Successful plan result includes the exact message ID and verified_trashed observed postcondition.

## Compatibility and verification

Existing READ plan semantics/hash/default scopes are unchanged; new action is additive. `go test ./...` exercises synthetic providers; no real account data is used. Live behavior, retention effects, crash recovery across real providers and all platform environments remain unverified. A tiny separately approved real-account test is a future decision, not included in feature approval.
