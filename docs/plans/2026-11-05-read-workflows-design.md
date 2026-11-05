# Read workflow expansion

User approved implementation after auth lifecycle/doctor, choosing **thread reads only**, not native threaded replies. No live Google/keyring calls, OAuth reconsent, remote mutation, commit/push or release are implied by implementation/testing.

Subsequent user approval: commit/push these changes and publish `v0.1.0-beta.2` as a GitHub prerelease after hosted CI gates pass. This publication approval still does not authorize live Google/keyring tests or new OAuth consent. Beta.1 must remain unchanged.

## Sequence

1. Local `doctor [--account ALIAS] [--credential-check]`: default does not open the keyring or use the network. Optional credential inspection exposes only presence/expiry, never values; no refresh/save/delete. Synthetic lifecycle tests exercise valid/expired/rotated/missing/revoked credentials, persistence and concurrency. Natural expiry on the user's real connection remains unverified.
2. `calendar availability --account ALIAS --file INPUT.json`: explicit 1–10 calendar IDs, RFC3339 window <=14 days, IANA timezone and minimum duration. Use official free/busy query, union overlapping busy intervals, return maximal free windows meeting duration. No inferred work hours or scheduling. Missing/error/group/malformed responses yield no free-slot claim. The narrow free/busy scope is a **new opt-in login flag**, never added to existing profiles/connections silently; real consent is separate.
3. `brief --account ALIAS --file INPUT.json`: selected calendar events and explicit Gmail query, <=48-hour agenda window, bounded per-calendar events and <=20 email metadata reads. Return deterministic compact data, per-section completeness/errors/pagination and timestamp, not a bundled AI summary. No background scheduler, body harvest, mark-read or writes.
4. `mail thread --account ALIAS --id THREAD [--limit 10]`: bounded thread metadata plus selected plain-text context, selected headers/snippets; maximum20 returned messages and a bounded response body. Expose omissions; no separate attachment downloads, whole-thread body harvest, reply, draft, or automatic mark-read. Read metadata first, then only the latest selected bodies (<=20); each wire response <=2MiB, each returned text excerpt <=2000 UTF-8 bytes. Context is untrusted data, never instructions/consent.

## Compatibility/security

Keep six existing plan action kinds, immutable snapshots, chat approval and single-attempt execution unchanged. Explicit account everywhere except credential-free doctor discovery. Reject invalid read inputs before account, credential or provider access. Windows protected operations remain disabled. Existing OAuth scope lists/legacy connection JSON stay compatible. Remote data remains marked untrusted. Use narrow typed SDK methods and deterministic helpers, no model/plugin/daemon/cache framework.

## Verification

Critical tests cover doctor redaction/no keyring by default; synthetic expiry/rotation/revocation/cancellation; free/busy scope opt-in and legacy omission; timezone offsets/DST, overlap/touching/clipping and minimum duration; missing/error calendar data never reported free; brief pagination/partial failures and bounded calls; oversized/mismatched threads and explicit omissions; compiled discovery/invalid-input guards. Run vet/race, Python release regressions, smoke, six cross-builds and privacy scans. These are offline/synthetic evidence, not live integration proof.
