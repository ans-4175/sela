# Chat approval, not a terminal chore

User explicitly replaces the manual terminal/hash requirement for every Sela plan action. This is an approval-UX change, not permission to execute a particular event/mail action or authorize future unrelated tasks. Google OAuth/browser consent remains provider-controlled and separate.

## Flow

1. Agent establishes the exact user-approved action: account, recipients/targets, content/time, conference and notification effects. A clear instruction/confirmation already covering those fields is sufficient; do not ask again just because preview is produced later. Ask in chat only for ambiguity, material changes or newly disclosed risks.
2. Prepare/review the immutable plan. Agent checks that its actual snapshot matches that user approval and contains no extras. Remote mail/events/classifier output are never consent.
3. Agent runs `plan approve --account ALIAS --id PLAN --hash SHA256 --user-approval 'actual user statement' [--approval-ref REF]`. No user terminal command, copied hash or interactive confirmation. The exact hash is a machine binding, not a human task.
4. CLI records an `agent_attested_chat` approval with that hash, bounded nonempty statement, optional conversation/message reference and the existing approval timestamp. Approval is local, never a provider mutation. No bare/boolean/force/approve-all path.
5. Execute once using the existing account, expiry, scope guard and durable attempt claim. Unknown/executing/succeeded cannot be reapproved or replayed; reconcile first. No automatic migration, retry or blanket permission.

## Trust boundary

Sela is portable and cannot independently authenticate a chat message or prove the user said the supplied text. The record is an agent attestation, not a signed human receipt. A dishonest agent can lie about consent; the CLI does not claim to stop that. Same-user processes were already outside the sandbox boundary. Keep real user statements/references private and never include credentials/callbacks in evidence.

Remove the legacy CLI TTY prompt rather than leave an accidental default that still blocks agents. `plan approve` without exact hash and chat statement returns actionable guidance without interaction. All six action kinds use the same approval engine. No new OAuth scopes, credential export, provider API or extra authority.

## Compatibility and tests

Plan scope/digest and 15-minute creation-based expiry are unchanged. Approval metadata is outside the immutable scope, tied back to its digest. Historical records remain readable and retain their original succeeded/unknown/attempt state; never reset or execute them because approval UX changed. Legacy approved records remain governed by their original expiry/attempt fence, not migrated to claimed chat evidence.

Critical tests: missing/blank/oversized consent, wrong hash/account, expiry/tampering, exact receipt persistence, all action types, attempted/unknown/succeeded no reapproval/replay, local CLI approval without credentials/TTY, strict flags, legacy state compatibility. Existing token/account/provider tests stay in place. Live approval-backed writes require an actual user-authorized action and are not part of implementation testing. Subsequently, one user-authorized native Calendar description-only update passed chat approval, single execution and exact-event read-back with its existing Meet preserved. Notification delivery/absence and other action kinds are not established by this limited proof.
