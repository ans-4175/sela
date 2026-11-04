# Calendar: make time, not surprise invitations

## Bounded reads
```sh
sela calendar list --account personal --limit 20
sela calendar events --account personal --calendar CALENDAR_ID --start 2026-12-01T00:00:00+07:00 --end 2026-12-08T00:00:00+07:00 --limit 20
sela calendar get --account personal --calendar CALENDAR_ID --id EVENT_ID
```
Use a user-selected calendar and RFC3339 dates with offsets (range <=366 days). No implicit primary calendar; an explicit `--calendar primary` is fine if the user chose it. Empty results are not guaranteed availability: calendars may be missing, paging incomplete, events tentative/private or timezone assumptions wrong. No freebusy API/aggregation is exposed yet.

## Event write boilerplate
Requires separately consented `gmail-calendar-write` profile. Example private input:
```json
{"calendar_id":"primary","send_updates":"all","event":{"summary":"Catch-up","description":"Agenda approved by the user","location":"Video call","start":{"date_time":"2026-12-01T10:00:00+07:00","time_zone":"Asia/Jakarta"},"end":{"date_time":"2026-12-01T10:30:00+07:00","time_zone":"Asia/Jakarta"},"attendees":["person@example.com"]}}
```
```sh
sela calendar preview --account personal --action create --file /private/path/event.json
# After actual user chat consent, agent matches the exact fields/effects and records it:
sela plan approve --account personal --id PLAN_ID --hash SHA256 --user-approval 'USER STATEMENT'
sela plan execute --account personal --id PLAN_ID
```
The user's chat instruction/confirmation is enough when it covers the exact fields/effects: the agent runs approve/execute, not the user. Do not re-ask already-confirmed unchanged details. Replace USER STATEMENT with actual minimal consent evidence; see `--skill safety` for trust limitations.

For update add `event_id` and replacement event fields; description/location/attendees are explicit replacement values, so omission clears them. For delete supply calendar_id, event_id, send_updates and no event. Update/delete preview reads current state, captures old attendees + exact ETag and refuses a supplied stale `expected_etag`. Execution uses If-Match. An event changed by another client requires new preview/approval. `send_updates` must be explicit `all` or `none`. Attendees may receive invitations/updates/cancellations; none is not a guarantee of no notifications. Response states may be affected. Review BOTH previous and new recipients.

## New Google Meet (create only)

To request a new Meet, add this top-level field to create input:
```json
"conference": {"type": "google_meet"}
```
Preview generates a unique `conference.request_id` and binds it to the exact account/calendar/event/attendees/send_updates approval. Do not supply/reuse IDs, conference URLs or another provider. Uses the existing Calendar event-write profile; no additional OAuth scope is introduced. Ordinary update omits conference data and preserves the existing conference; conference-changing updates are rejected.

**Partial success matters:** Google generates conferences asynchronously. The event and guest invitations can exist before a Meet link is ready; this is not atomic event+ready-link creation. Preview explicitly discloses this risk before chat approval. Do not invent consent to a newly disclosed risk; ask in chat if needed, not for terminal commands/hashes. Execution inserts once, then reads the event once. Check `result.event_created`, `conference_status`, `meet_ready`, `workflow_complete` and `meet_url`. Generic plan `state:succeeded` means event creation was accepted, NOT that Meet is ready or invitations were delivered.

Only a successful matching read-back exposes a verified HTTPS meet.google.com video URL. Pending/failure/unverified results retain the created event ID with `workflow_complete:false`. Use `calendar get` on that existing ID to reconcile; compare conference request ID with the approved plan, status success, solution hangoutsMeet and its video entry point. Never repeat insert, replace the event, regenerate a conference or cancel automatically. Even a stable conference request ID does not make event insert replay-safe. Unknown insert/persistence outcomes still require reconciliation before any new write.

One native macOS create+Meet flow passed live read-back verification using the earlier terminal gate. One chat-approved description-only update also passed exact-event read-back with title/time/attendee and Meet unchanged. This does not prove absence of Google notifications for send_updates=none, or validate all update scenarios. Mail send/draft, Calendar delete and real token refresh remain unverified.

Timed single events only (<=31-day duration), explicit IANA timezone on start/end. Each timestamp offset must match that zone on that date (including daylight-saving changes); conflicting offsets/zones are rejected rather than guessed. No all-day/recurring writes, calendar deletion, ACL changes, conference-changing updates or default silent notifications. Read access does not grant writes; write capability does not grant action approval.
