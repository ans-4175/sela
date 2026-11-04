# Create a Calendar event with a new Google Meet

Scope approved as a bounded exception to the capability freeze. No new providers, OAuth scopes, conferencing framework or automatic repair. Implementation approval is not approval to create an actual event or send invitations.

## Contract

- Optional Calendar create input: `"conference":{"type":"google_meet"}`. Only this provider/type and only create are supported. No supplied conference URL, copy/reuse, conference removal or conference-changing update.
- Preview generates a fresh cryptographic request ID, stores it under `conference.request_id`, and binds it into the same immutable account/calendar/event/attendees/notification plan. Approval UX now follows [chat approval](2026-11-04-chat-approval-design.md); no manual TTY/hash step. User input must not supply that ID. Ordinary plans omit conference data entirely.
- Execution uses the official pinned Google Calendar SDK: exactly one insert, `conferenceDataVersion=1`, `conferenceData.createRequest` with the approved request ID and fixed `hangoutsMeet` solution key. Existing calendar.events authorization suffices; no new OAuth work is part of implementation.
- Perform one read-only event get after a successful insert with a nonempty event ID. Expose a Meet URL only if event ID, conference request ID, successful status, Meet solution and HTTPS meet.google.com video URI agree. No loop, second insert, conference retry or cancellation.

## Partial success is not meeting readiness

Conference generation is asynchronous. Event creation and guest invitations can happen before the Meet URL is ready. Neither `send_updates=none` nor this implementation guarantees atomic creation of an event with a ready link.

A successful insert returns a durable receipt with `event_created:true`, event ID, ETag, `conference_status`, `meet_ready` and `workflow_complete`. Pending, failure, malformed/mismatched conference data or a failed read-back leave `workflow_complete:false` and no purported ready URL. The generic plan state `succeeded` means the event-create action returned successfully, not that all user requirements were fulfilled. Read the result fields. Later use read-only `calendar get` on that event; never create a replacement automatically.

An insert transport/provider error or missing event ID remains unknown and consumes the existing one-attempt plan. A later persistence failure can also lose the receipt: reconcile Calendar before any further action. Request IDs are conference request correlation, NOT event-insert idempotency keys or permission to retry.

## Compatibility and critical tests

Non-Meet create keeps its existing output/request shape. Ordinary update omits conferenceData and conferenceDataVersion, preserving the provider's existing conference rather than clearing/replacing it. Update/delete cannot accept a conference intent. Existing old plan hashes remain unchanged by omission.

Tests cover create-only validation, generated unique IDs and hash binding/expiry, local preview without credentials, exact SDK conference/notification parameters, successful verified link, asynchronous pending/failure, post-insert read failure/ID mismatch/unsafe URLs, one-attempt unknown behavior and ordinary create/update omission. Fixtures are synthetic; compiled sandbox tests make no real Google/Keychain calls. One native Calendar create+Meet subsequently passed live read-back using the earlier terminal gate. This does not prove notification delivery or other Calendar write paths.
