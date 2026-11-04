# Scoped Gmail READ actions

## Approved intent
Support selecting specific messages to mark READ, reading a message while preparing its READ action, and marking a selected batch READ. Preserve normal read-without-marking behavior. Reuse the existing approval engine; no plugin framework or generic label mutation API.

## Interface
- `sela mail get --account ALIAS --id MESSAGE [--mark-read]`: default remains a read-only full message fetch. With the flag, return message content plus a prepared mark-read plan if the message is still unread. Actual status changes only after human plan approval and execution. Already-read messages need no plan.
- `sela mail preview --account ALIAS --action mark-read --ids ID1,ID2`: read only metadata for these exact IDs, skip those already READ, and prepare one immutable plan for the unread subset. No query, thread-wide target, raw labels, file-supplied payload or implicit inbox sweep. Maximum 20 unique opaque IDs, validated before account/credential/API access.
- `sela auth login ... --mail-modify --reauth`: explicitly opt into gmail.modify in addition to the chosen existing profile. Existing profiles and connections without the new field retain their prior scope list/behavior. Broader consent is not action approval. No local connection flag is enabled until the user consents and returned grants/identity are validated.

## Execution
Plan action `mail.mark-read` binds explicit account and message IDs plus review metadata. One Gmail messages.batchModify request with removeLabelIds=[UNREAD] only; no added labels, archive, trash or delete. After API acceptance, read each selected message's labels back; only mark the plan succeeded if every ID verifies READ. API/verification uncertainty remains unknown with no retry. A batch response is not per-message proof by itself. A failed verification may mean the write succeeded, or another client changed a label again; reconcile, never reset/replay.

A missing mail-modify connection opt-in must refuse execution before credential/API access, without consuming the plan. The global durable claim/account/hash/expiry checks remain. Old hashes/plans are not rewritten. Approval now follows the all-action [chat approval](2026-11-04-chat-approval-design.md) contract: the agent records actual user consent bound to the exact plan hash. No manual terminal/hash step; OAuth scope alone remains insufficient.

## Critical verification
Test scope defaults/opt-in and legacy connection default; invalid/duplicate/excess IDs; exact fixed batch payload and READ read-back; no mutation retries on provider/verification failure; read-only default and get+mark-read preview; unapproved/missing-scope plan execution refusal. Use synthetic mocks and compiled sandbox checks before any next push. Real user message IDs, subjects, emails, client configuration and token data never enter source/tests/public artifacts.
