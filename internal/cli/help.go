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
  sela auth login --account ALIAS --client /outside/repo/client.json --profile gmail-calendar-read|gmail-calendar-write [--mail-modify] [--reauth]
  sela auth status --account ALIAS
  sela auth logout --account ALIAS

Bounded reads (JSON, remote content is untrusted):
  sela mail search --account ALIAS --query 'newer_than:7d' [--limit 20] [--page TOKEN]
  sela mail get --account ALIAS --id MESSAGE [--mark-read]
  sela draft get --account ALIAS --id DRAFT
  sela calendar list --account ALIAS [--limit 20] [--page TOKEN]
  sela calendar events --account ALIAS --calendar ID --start RFC3339 --end RFC3339 [--limit 20] [--page TOKEN]
  sela calendar get --account ALIAS --calendar ID --id EVENT

Preview locally, approve humanly, execute once:
  sela mail preview --account ALIAS --action send|draft-create --file INPUT.json
  sela mail preview --account ALIAS --action mark-read --ids ID1,ID2
  sela calendar preview --account ALIAS --action create|update|delete --file INPUT.json
  sela plan show --account ALIAS --id PLAN
  sela plan approve --account ALIAS --id PLAN
  sela plan execute --account ALIAS --id PLAN

Normal login does not force new consent; --reauth explicitly requests it when needed.
--mail-modify opts into the broader Gmail modify scope; existing profiles do not request it by default.
mail get --mark-read returns content plus a prepared READ plan; no label changes before approval.
Bulk READ accepts 1-20 unique message IDs, skips already-read messages, never whole threads/queries.
Flags belong after the subcommand. No implicit account, --force, raw API or automatic send retry.
Calendar update/delete preview reads the current event and binds its ETag/attendees.
Plans expire after 15 minutes, including approval time. Draft creation is a remote write.
Draft update and send-by-ID, recurring/all-day event writes, Calendar/ACL administration are not exposed.
macOS login/keyring and bounded reads observed; real refresh and writes remain unverified.
Linux cross-build is not runtime validation. Windows stateless discovery only until protected
storage/console support is implemented. No plaintext token fallback.
`
