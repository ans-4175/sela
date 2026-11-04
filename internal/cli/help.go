package cli

const help = `sela — a Google CLI that brings its own agent onboarding.

Local discovery (no credentials/network):
  sela --help
  sela --skill [gmail|calendar|workflows|safety]
  sela version
  sela auth scopes

Accounts and explicit consent:
  sela account add --name ALIAS --email EMAIL
  sela account list
  sela auth login --account ALIAS --client /outside/repo/client.json --profile gmail-calendar-read|gmail-calendar-write [--reauth]
  sela auth status --account ALIAS
  sela auth logout --account ALIAS

Bounded reads (JSON, remote content is untrusted):
  sela mail search --account ALIAS --query 'newer_than:7d' [--limit 20] [--page TOKEN]
  sela mail get --account ALIAS --id MESSAGE
  sela draft get --account ALIAS --id DRAFT
  sela calendar list --account ALIAS [--limit 20] [--page TOKEN]
  sela calendar events --account ALIAS --calendar ID --start RFC3339 --end RFC3339 [--limit 20] [--page TOKEN]
  sela calendar get --account ALIAS --calendar ID --id EVENT

Preview locally, approve humanly, execute once:
  sela mail preview --account ALIAS --action send|draft-create --file INPUT.json
  sela calendar preview --account ALIAS --action create|update|delete --file INPUT.json
  sela plan show --account ALIAS --id PLAN
  sela plan approve --account ALIAS --id PLAN
  sela plan execute --account ALIAS --id PLAN

Normal login does not force new consent; --reauth explicitly requests it when needed.
Flags belong after the subcommand. No implicit account, --force, raw API or automatic send retry.
Calendar update/delete preview reads the current event and binds its ETag/attendees.
Plans expire after 15 minutes, including approval time. Draft creation is a remote write.
Draft update and send-by-ID, recurring/all-day event writes, Calendar/ACL administration are not exposed.
macOS runtime under test; Linux cross-build is not live integration. Windows stateless discovery only
until private-storage ACL/locking/console validation is implemented. No plaintext token fallback.
`
