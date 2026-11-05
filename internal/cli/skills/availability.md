# Find candidate free time, not permission to book it

Ask or reuse user-confirmed account, selected calendar IDs, date/time window, timezone, meeting duration and any work-hour constraints. Never assume 09:00–17:00, a timezone, weekends, attendees' availability, all accounts, or all calendars. For several days with working hours, query separate explicit daily windows; one overnight-spanning window includes nights.

## Prerequisite: explicit free/busy scope

Existing read/write profiles do NOT implicitly request the official free/busy scope. Availability requires the independent `--calendar-freebusy` login opt-in (`calendar.events.freebusy`). Human browser consent is separate, with the user-selected profile and any other wanted opt-ins preserved. Agent must not silently reauthenticate or change flags in stored state.

## Input and command

Write private JSON outside the repository, using actual user-selected boundaries:
```json
{"calendars":["primary","work-calendar-id"],"start":"2026-12-01T09:00:00+07:00","end":"2026-12-01T17:00:00+07:00","time_zone":"Asia/Jakarta","duration_minutes":30}
```
```sh
sela calendar availability --account personal --file /private/path/availability.json
```
Supports 1–10 unique explicit calendar IDs, <=14-day RFC3339 window, IANA timezone matching each timestamp's offset (including DST), minimum duration 1–1440 minutes. These are calendars accessible through this ONE account, not cross-account aggregation. Uses the official free/busy query: its POST is a read, not an event mutation. No event details are harvested for this calculation.

## Interpret conservatively

`free_windows` are maximal gaps at least the requested duration, not a promise that every minute is appropriate or a list of booked slots. Busy intervals are clipped to the window and unioned across all selected calendars, including touching/overlapping intervals. Returned timestamps use the chosen zone. The wire response is bounded.

Missing calendars, provider errors/groups, missing busy data, wrong response window or malformed intervals fail closed: **do not claim availability**. Do not substitute an empty calendar-events page, exclude the failing calendar silently, or claim coverage of unselected calendars/accounts. Free/busy reflects provider semantics such as transparent events and can become stale immediately.

Present a few suitable candidate times and name coverage/constraints. Scheduling a chosen slot remains a separate create preview, exact chat approval and one execution attempt. Do not invite anyone, reserve time, regenerate Meet or create a recurring event merely because this read found a gap.
