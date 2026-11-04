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
# Human approves plan after reviewing account/calendar/timezone/all attendees/notification effects.
sela plan approve --account personal --id PLAN_ID
sela plan execute --account personal --id PLAN_ID
```
For update add `event_id` and replacement event fields; description/location/attendees are explicit replacement values, so omission clears them. For delete supply calendar_id, event_id, send_updates and no event. Update/delete preview reads current state, captures old attendees + exact ETag and refuses a supplied stale `expected_etag`. Execution uses If-Match. An event changed by another client requires new preview/approval. `send_updates` must be explicit `all` or `none`. Attendees may receive invitations/updates/cancellations; none is not a guarantee of no notifications. Response states may be affected. Review BOTH previous and new recipients.

Timed single events only (<=31-day duration), explicit IANA timezone on start/end. Each timestamp offset must match that zone on that date (including daylight-saving changes); conflicting offsets/zones are rejected rather than guessed. No all-day/recurring writes, calendar deletion, ACL changes, conference creation or default silent notifications. Read access does not grant writes; write capability does not grant action approval.
