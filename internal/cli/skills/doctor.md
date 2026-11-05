# Authentication lifecycle and local diagnosis

Start with `sela doctor`. Without flags this is credential-free local platform discovery: no account read, keyring, network, refresh, repair or directory creation. Windows can use this discovery but still cannot open protected account state.

For one user-selected account:
```sh
sela doctor --account personal
```
This checks existing account/private connection configuration and reports configured scopes, not a live grant/identity check. Missing or unsafe setup is a diagnostic status, not permission to fix chmod/paths automatically. It does not open the keyring by default.

Only when the user authorizes local credential inspection:
```sh
sela doctor --account personal --credential-check
```
This can trigger a native keyring prompt. It returns only credential presence, expiry and local status; no token/client values. It does not refresh, save, delete or contact Google. Read the JSON status: command success means the diagnostic ran, not that the account works online. `valid_locally_not_provider_verified` is not proof the grant is still valid; zero expiry is unknown, not perpetual validity. A refresh-token's presence is not proof that it remains usable.

## Lifecycle

Normal authorized reads refresh on demand under the existing per-account process lock. Refreshed credentials must persist before use; an omitted replacement refresh token preserves the previous one. Persistence failure stops use. Revoked/expired grants reporting invalid_grant require explicit fresh browser reauthentication, never a blind write retry. Logout removes a local credential and does not claim remote revocation.

Synthetic tests cover valid/expired access credentials, refresh rotation/omission, invalid_grant, persistence failure, concurrent sources and cancelled requests. **Actual natural expiry/refresh on the user's connection remains unverified.** Do not edit a real token's expiry, export a refresh token, revoke a grant or force live tests to manufacture evidence. Ask for a bounded live test separately.

Google Testing grants may have limited lifetime; doctor cannot infer a project's publishing status, exact grant expiry or production verification. Never print credentials, auth URLs/callback codes, or private client configuration for debugging.

`--mail-modify` and `--calendar-freebusy` are independent login opt-ins. Real scope changes use human-controlled browser consent with an explicitly selected profile and wanted flags, commonly with --reauth. Do not flip connection booleans to simulate consent, silently broaden an existing connection, or treat OAuth capability as action approval.
