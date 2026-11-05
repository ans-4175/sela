# Give Sela a Google connection

One Cloud project, one Desktop OAuth client, and one account you explicitly choose. Coffee optional. Consent is not.

This guide sets up **Gmail + Google Calendar** for your own use. It does not authorize Sela or an agent to read your mailbox, inspect events, or perform writes just because login succeeded. Those are separate task decisions.

## Before you start

- Build Sela as described in the [README](../README.md#build-from-source). Examples below assume `sela` is on PATH; otherwise use `./bin/sela` from the source directory.
- Use a Google account with Gmail and Calendar available. Workspace administrators may restrict third-party apps or scopes.
- Current native testing is on macOS. Linux integration needs runtime validation; Windows protected storage/locking is not implemented yet. See the README platform table.
- Keep client files and runtime state **outside the repository**. Never upload client JSON, tokens, real account metadata or approval plans to issues or chat.

## 1. Create your Cloud project

Open [Google Cloud Console](https://console.cloud.google.com/) and create/select a project, for example **Sela Personal**. Make sure the intended project stays selected while configuring the following pages.

Under **APIs & Services → Library**, enable both:

- **Gmail API**
- **Google Calendar API**

Enabling an API does not grant Sela access to your account. That happens later through OAuth consent.

## 2. Configure the consent screen

Open **Google Auth Platform → Branding**. If prompted, choose **Get Started**.

- App name: **Sela Personal** (or a name you recognize).
- Support email and developer/contact email: your own contact address.
- Audience: **External** for a personal Google account. Internal is only appropriate for eligible Workspace organization deployments.

Review Google's policies and save the configuration.

### Testing versus Production

For an initial test, **Testing** is usable: under **Audience → Test users**, add the Google email you intend to connect. With the Gmail/Calendar scopes below, Testing authorizations and offline refresh tokens expire after **seven days**.

For ongoing personal use, review whether you qualify for Google's [personal-use verification exception](https://support.google.com/cloud/answer/13464323?hl=en), and use **In production** when appropriate. Publishing status and verification status are different. Unverified-app warnings, user caps and account/admin restrictions may still apply. Production does not make refresh tokens immortal; revocation, account policy and other lifecycle limits remain.

If Google blocks your app or requires further review, stop and resolve the actual configuration/policy issue. Do not change application type or use someone else's client to bypass it.

## 3. Declare the scopes

Open **Google Auth Platform → Data Access → Add or remove scopes**. Sela's write-enabled profile requests:

```text
openid
email
https://www.googleapis.com/auth/gmail.readonly
https://www.googleapis.com/auth/gmail.compose
https://www.googleapis.com/auth/calendar.events
https://www.googleapis.com/auth/calendar.calendarlist.readonly
```

`openid`/`email` identify the authenticated Google account. The Console may show the email identity scope as `https://www.googleapis.com/auth/userinfo.email`.

**`gmail.compose` includes sending**, not just drafts. `calendar.events` permits event changes. Neither means an agent may perform an arbitrary write: Sela's per-action preview/approval gate still applies.

If you only want Calendar reads, use the `gmail-calendar-read` profile and declare `calendar.events.readonly` instead of `calendar.events`. That profile **still includes Gmail compose/send permission**; its name is not a Gmail read-only guarantee.

### Optional: selected-message READ changes

To use mark-read, additionally declare `https://www.googleapis.com/auth/gmail.modify`, then repeat your chosen login command with **`--mail-modify --reauth`**. You must explicitly review and consent to the added access. Existing profiles/connections do not silently gain it.

Google's modify scope is broader than marking READ. Sela exposes only fixed removal of UNREAD on explicitly selected message IDs, not arbitrary labels, archive or deletion. OAuth consent is not per-action approval: actual label changes still need an exact user-approved plan recorded by the agent from chat consent. Omitting the flag later disables this CLI capability but does not necessarily revoke Google's previously granted permissions; use Google Account access controls if you need provider-side revocation.

### Optional: selected-calendar free/busy reads

Availability uses the separate `https://www.googleapis.com/auth/calendar.events.freebusy` scope. Declare it only if wanted, then deliberately repeat your selected login profile with `--calendar-freebusy --reauth`, preserving any other opt-ins you still want. This requires fresh human-controlled browser consent; existing profiles/connections are not widened automatically. It enables the bounded read-only availability command, not meeting creation or standing scheduling authority. Brief and thread reads do not require this new scope. See `sela --skill availability`.

You can inspect the exact profiles without accessing an account:

```sh
sela auth scopes
```

## 4. Create a Desktop client

Open **Google Auth Platform → Clients → Create client**:

1. Application type: **Desktop app**.
2. Name: **Sela Desktop** (the name is for your Console).
3. Create the client and download its JSON file.

Some Console versions offer **Use this client for an AI-powered agent**. Google documents this as optional and **immutable after client creation**, and recommends separate clients for agent and standard-user workflows if you support both. Review that choice for your deployment before creating the client. Sela's live behavior with agent-specific client settings has not been validated; the setting is not an authorization shortcut. See [Google's Desktop client instructions](https://developers.google.com/workspace/gmail/api/quickstart/go#authorize_credentials_for_a_desktop_application).

Sela expects an `installed` Desktop client, not a Web client, API key, service account or App Password. The CLI handles its own temporary loopback redirect; you do not paste an authorization code or token into the terminal.

Store the download privately. For example, replacing the source path with your actual downloaded file:

```sh
mkdir -p "$HOME/.config/sela"
chmod 700 "$HOME/.config/sela"
mv "/path/to/downloaded-client.json" "$HOME/.config/sela/google-desktop.json"
chmod 600 "$HOME/.config/sela/google-desktop.json"
```

The Desktop client is not your user token, but keep its configuration private anyway. Do not copy it into this public repository, even temporarily.

## 5. Register the account and log in

Pick an explicit alias. Use your actual Google email in place of this example:

```sh
sela account add --name personal --email you@example.com
sela auth login --account personal \
  --client "$HOME/.config/sela/google-desktop.json" \
  --profile gmail-calendar-write
```

Use `--profile gmail-calendar-read` if that is the Calendar access you deliberately chose. All flags go after the subcommand. If the alias already exists for the same account, skip `account add`; Sela refuses alias rebinding rather than quietly changing identities.

Open the printed Google consent URL in your own browser. Check the app, account and requested permissions, then grant only the scopes you intend to use. Sela verifies that Google's authenticated, verified email matches the selected account before saving credentials.

The callback listens temporarily on `127.0.0.1` on this machine and expires after five minutes. Browser and CLI must reach the same machine's loopback address. For this first test, run them on the same desktop; remote SSH/headless login is not documented as supported. Never send the callback URL/code to an agent or public issue.

If successful, the command returns `authenticated: true` for the selected account. Then:

```sh
sela auth status --account personal
```

`credential_present: true` confirms local storage presence, **not** a new live Google check or proof that every scope/API operation works. Login verifies identity; it does not fetch mail or Calendar contents.

## 6. Know where things live

On the current Unix implementation:

- Tokens: native OS keyring, **no plaintext token fallback**. macOS uses Keychain; Linux requires a working supported keyring service.
- Private metadata, client configuration and plans: `$XDG_CONFIG_HOME/sela` or `~/.config/sela`.
- State directories: 0700. State files: 0600. Unsafe permissions or linked state files are refused rather than silently followed.

Access tokens refresh on demand. Refresh reloads the account's latest saved credential under a process lock and persists changes before returning them to an API client. No daemon keeps tokens alive, and no test proves a token will remain valid indefinitely.

To remove the locally stored token:

```sh
sela auth logout --account personal
```

Logout is a credential mutation; run it only when you mean to disconnect that local account. It does not claim to revoke all Google's previously issued tokens. Use your Google Account's third-party access controls for provider-side revocation.

## Local diagnosis before changing authentication

`sela doctor` is credential-free discovery. `sela doctor --account ALIAS` checks existing local setup without Google or keyring access. Only the optional `--credential-check` opens the local keyring and reports presence/expiry; it never prints token values or refreshes/saves/deletes them. Read the diagnostic status, not just the exit code. Local validity/presence is not evidence of a current provider grant; real natural expiry/refresh remains unverified. See `sela --skill doctor`.

## If login refuses

| Symptom | Check |
| --- | --- |
| Invalid Desktop client | Download a Desktop app client, not a Web/service credential; don't edit endpoints to make validation pass. |
| Access denied / app blocked | Correct project, enabled APIs, audience/test user, granted scopes and Workspace policy. |
| Wrong account | Retry with the intended Google account; don't rebind the alias to bypass the check. |
| Missing refresh token | With your explicit consent, repeat the same login command with `--reauth`. This requests fresh consent; it is not the default. |
| Missing granted scope | Review the chosen profile and consent screen. Sela will not silently downgrade the connection. |
| Callback timeout | Browser/CLI on the same machine; complete consent within five minutes. Start a new login instead of reusing an old callback. |
| Unsafe local storage path/permissions | Use a private real directory you own, without symlink ancestors; `0700` directories / `0600` files. On macOS `/var` and `/tmp` can be symlinks: choose their canonical paths for custom config/test roots. Never chmod system directories or random ancestors to silence this check. |
| Keyring unavailable / persistence failed | Fix native keyring availability or access. There is no token-file fallback. Don't post raw keyring/OAuth output. |
| `invalid_grant` later | Explicit reauthentication is needed; do not loop refresh requests or promise never-expiring tokens. |

## After login

Start with a separately authorized, bounded read task and the [embedded agent guides](../README.md#the-manual-is-a-command):

```sh
sela --skill gmail
sela --skill calendar
sela --skill workflows
sela --skill safety
```

A write needs an exact preview matched to the user's chat consent → agent-recorded approval → execution. Users do not run plan approval commands or copy hashes; agents must not fabricate consent. Google OAuth browser consent remains a separate setup requirement. A working OAuth connection is not a standing instruction to become everyone's calendar manager.

## Official references

- [Google's Gmail Go quickstart](https://developers.google.com/workspace/gmail/api/quickstart/go) for Console setup. Its sample token-file/code-paste flow is **not** Sela's storage/login design.
- [Configure OAuth consent and scopes](https://developers.google.com/workspace/guides/configure-oauth-consent).
- [Audience, Testing expiry and Production](https://support.google.com/cloud/answer/15549945?hl=en).
- [When verification is not needed](https://support.google.com/cloud/answer/13464323?hl=en).

Console labels and policies can change. Reviewed against these sources on 2026-11-04.
