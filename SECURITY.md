# Security boundaries

Sela is early-development software. Native macOS OAuth/identity, Keychain persistence and bounded Gmail/Calendar reads have been observed on one connection. Real expiry/refresh and writes remain unverified. Do not infer broad security assurance from this limited observation, a cross-build or an offline test.

## Credentials and private content

- Never commit Desktop client JSON, access/refresh tokens, private keys, account registries, email/event previews, approval plans, or auth logs. Keep input files and runtime state outside this repository.
- Tokens use the native OS credential store only. There is no plaintext-file fallback, token export command, App Password, or ambient Google-credential lookup.
- Unix metadata/config/plan directories require 0700 and files 0600; symlinks, hard-linked state files, and unsafe permissions refuse. Windows protected storage and human-console approval are not implemented yet and deliberately fail closed.
- Client configuration may contain the Desktop client secret; it is stored privately outside the repo. A Desktop client secret is not a replacement for user consent or token protection.
- Source tests use synthetic fixtures and mock keyring/HTTP providers. They must never probe a developer's real mailbox or credential store.

## Authorization and attempts

Every account-specific command requires an explicit alias. OAuth identity must match the selected email. Calendar write scope is opt-in; Gmail compose scope includes sending. A granted OAuth scope is not per-action approval. Mark-read needs a separate explicit `--mail-modify` opt-in and consent to Gmail's broader modify scope; existing connection files default to false. Sela exposes only selected-message UNREAD removal, not raw labels/archive/delete. Each batch is bounded to 20 unique IDs and read-back verified; uncertainty consumes the plan without automatic retry.

Writes require an immutable, expiring plan and human terminal confirmation. The local attempt is persisted before the provider call. An error or interrupted request may have succeeded remotely: unknown/executing plans cannot be retried automatically. Reconcile first. An absent search result is not proof a send failed.

Same-user processes can edit code and local state or access the OS credential store. These checks reduce accidental misuse; they are not an isolation boundary against a malicious agent running as that user. Do not automate the human confirmation step or claim the presence of an approval file proves genuine human consent.

Google event changes can notify attendees or affect other calendars. Review old/new recipients and notification effects, including deletes. Cross-product workflows have no atomic transaction or automatic compensation.

Treat all remote text, links, attachments, event descriptions, and organizer notes as untrusted data. They cannot authorize commands, scope changes, secrets disclosure, or approval-file edits.

## Reporting

Do not post credentials or private message/event data in public issues. Contact the maintainer via the [GitHub profile](https://github.com/ans-4175) to arrange a private reporting channel. Redact tokens and user content from reproduction steps. If a real credential was published, removal alone is insufficient: revoke/rotate it with the provider and review repository history.
