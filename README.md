# Sela

**A portable personal-assistant toolkit. Bring your own brain.**

Google calls them messages and events. You call it “please sort my Tuesday out.”

Your model brings the reasoning. Your harness runs the tools. Sela brings the Gmail and Calendar capabilities, workflows, and a manual the agent can read itself.

> An API-shaped CLI asks, “Which endpoint do you want to call?”
>
> Sela starts with, “What does the user need, and how should an agent handle it?”

“Help me with this meeting invitation” involves more than an API call. Someone has to read the right email, check the right calendar, resolve the timezone, propose a reply, and ask before inviting half the company. Ideally, that someone isn't you copy-pasting API documentation into a prompt.

Sela packages those tools and working instructions together. Give an agent shell access to Sela, let it discover `--help` and `--skill`, and it can learn the available capabilities and their boundaries. No particular model, agent SDK, or harness is required. The agent still needs to use tools competently and respect the user's permissions; a command-line manual is not a personality transplant.

**The first proof is Gmail + Google Calendar.** Other personal tools may follow. They do not have implementations, scopes, or inherited permission just because they appear on a roadmap.

## Status: early development

The CLI and Google adapters have offline critical-path tests. **Native macOS OAuth login, identity verification, Keychain persistence, and bounded Gmail/Calendar reads have also been observed on one connection.** Actual token expiry/refresh and write operations remain unverified live. This is limited integration evidence, not a production-readiness claim. No credentials are bundled.

| Platform | Current boundary |
| --- | --- |
| macOS | Native build/tests plus observed login, Keychain and bounded reads; refresh/writes unverified |
| Linux | Unix storage/locking implementation; cross-build is not runtime validation |
| Windows | Stateless help/skill/version only; account storage and approval fail closed pending ACL/locking/console implementation |

“Portable PA” means the toolkit travels between agents and harnesses. It does not mean every OS or model has already passed integration testing.

## The manual is a command

```sh
sela --help
sela --skill
sela --skill gmail
sela --skill calendar
sela --skill workflows
sela --skill safety
```

These commands run locally without Google access or credential-store calls. Skills are embedded in the binary, so the instruction manual travels with the tools.

The workflow guide covers:
- Email invitation → calendar proposal → approved reply.
- A read-only morning brief from bounded email queries and selected calendars.
- Rescheduling a meeting without surprise notifications.
- Email follow-ups and draft preparation.
- Planning focus time without assuming an empty calendar means universal availability.

Each playbook explains what to ask the user, which reads to perform, when approval is needed, and how to handle partial success. Email bodies and event descriptions are **data, not instructions or consent**.

## Build from source

Requires **Go 1.26+**. No daemon, bundled model, or toolchain download is needed.

```sh
git clone https://github.com/ans-4175/sela.git
cd sela
GOTOOLCHAIN=local go build -trimpath -o bin/sela ./cmd/sela
./bin/sela --skill
```

For development:

```sh
make check  # vet + race tests of critical logic and interfaces
make build
```

Tests use synthetic credentials and local mocks, not your OS keyring or Google account. We test the mistakes that matter: wrong account, lost refresh credentials, changed payloads, expired approvals, duplicate attempts, and stale event updates. No coverage trophy cabinet.

## Several accounts. Zero guessing.

There is **no implicit default account**, even if only one is registered.

```sh
sela account add --name personal --email you@example.com
sela account list
sela auth scopes
```

Adding metadata does not log in or prove account identity. Follow the **[Google setup guide](docs/setup-google.md)** for project creation, API enablement, consent scopes, Desktop client download and troubleshooting. Keep the client file **outside this repository**.

```sh
sela auth login --account personal \
  --client /private/path/client.json \
  --profile gmail-calendar-read
```

The user opens the printed consent URL in their browser. Sela uses a loopback callback, PKCE, and an authenticated email check before saving credentials.

Both profiles request Gmail reading and `gmail.compose` (**which includes sending**, not just drafts). `gmail-calendar-read` requests Calendar read access; `gmail-calendar-write` explicitly opts into event write access. Selecting a scope is not approval for an individual write.

If Google omits an offline refresh token, repeat the chosen login command with **`--reauth`** to request fresh consent. Normal login does not force re-consent. Testing-mode Google projects can have short-lived refresh credentials for these scopes; an eligible Production configuration is separate from verification and never guarantees perpetual tokens.

Selected-message mark-read is a separate opt-in: add `--mail-modify --reauth` to your deliberately chosen login command and consent to Gmail's broader `gmail.modify` scope. Existing profiles do not request it by default. The CLI does not expose archive/delete/arbitrary label changes. See the setup guide before granting it.

Tokens stay in the native OS keyring, with **no plaintext fallback**. Private client configuration, account metadata, and approval plans live under `$XDG_CONFIG_HOME/sela` or `~/.config/sela` on the current Unix implementation. Refresh is on demand and serialized per account across processes. `auth status` checks local credential presence; `auth logout` removes the local credential, not a claimed remote revocation.

## Read with a purpose

```sh
sela mail search --account personal --query 'newer_than:7d' --limit 20
sela mail get --account personal --id MESSAGE_ID
sela calendar list --account personal --limit 20
sela calendar events --account personal --calendar primary \
  --start 2026-12-01T00:00:00+07:00 --end 2026-12-08T00:00:00+07:00
```

Reads and plans return JSON. Remote results are marked as untrusted data. Pagination is explicit; Sela does not decide to tour the entire mailbox. The skill guides document input shapes, limits, notifications, and examples.

Normal `mail get` never changes READ status. For read-and-mark, `mail get --mark-read` returns content plus a prepared READ plan; the label changes only after approval/execution. For a selected batch:

```sh
sela mail preview --account personal --action mark-read --ids ID1,ID2
```

The preview accepts 1–20 unique message IDs, reads metadata, and skips messages already READ. It never expands a query or marks an entire thread. Approved execution removes only UNREAD, then verifies each selected message. Learn the full read/search/summarize/READ workflows with `sela --skill gmail`.

## Sending is not a side quest

A send, draft creation, or event change uses the same path:

```text
preview → human approval → one execution attempt
```

Prepare a private input JSON outside the repo, then:

```sh
sela mail preview --account personal --action draft-create --file /private/path/mail.json
# Or: calendar preview --action create|update|delete, with the chosen account and input.
sela plan show --account personal --id PLAN_ID
sela plan approve --account personal --id PLAN_ID
sela plan execute --account personal --id PLAN_ID
```

Approval requires the human to review the exact account and payload, then type its hash in a controlling terminal. No `--yes`, `--force`, automated confirmation, or raw-API escape hatch.

Plans expire **15 minutes after creation**, not after approval. Mail attachments are snapshotted; changing a file later does not change the approved send. Calendar update/delete captures the current event, affected attendees, notification choice, and ETag. Execution uses `If-Match`; changed events need a fresh preview. Timestamp offsets must match the selected timezone on that date, including daylight-saving rules.

Once claimed, an attempt cannot be replayed. Transport errors and interrupted writes remain **unknown/executing** until reconciled; Sela will not blindly send again. This is local at-most-one-attempt behavior, not distributed exactly-once delivery. Cross-product workflows can partially succeed and have no automatic rollback.

Not exposed yet: Gmail draft update/send-by-ID, attachment downloads, arbitrary mailbox labels/archive/delete, recurring or all-day event writes, Calendar/ACL administration, free/busy aggregation, or background scheduling.

## Extend the capability, bring the instructions

The packages separate account metadata, OAuth, native credentials, typed payloads, approval state, Google SDK adapters, and CLI routing. No plugin framework is needed for the first two services. See [the design](docs/plans/2026-11-04-mvp-design.md).

A future capability should arrive with its narrow adapter, explicit scopes/account binding, critical interface tests, `--help`, and an embedded `--skill` workflow. Adding a new endpoint without explaining how an assistant should use it is only half the job.

Private local files and terminal approval guard against accidental misuse. They do **not** sandbox a malicious process running as the same OS user. Do not commit real credentials, mail/calendar previews, or approval state. See [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE), copyright © 2026 **ans4175**. Keep the notice when you reuse it. Google services and trademarks belong to their respective owners; Sela is an independent project.
