# Owner setup, sign-in, and recovery

Status: Authentication merged in PR #5. This guide covers local commands and owner credential recovery. See [deployment](deployment.md) for the current production Compose/operator commands.

## Local development

First follow the [database guide](database.md) to start PostgreSQL and apply migrations. The remembered-login release requires schema version 7; apply migration `00007_remembered_sessions.sql` before starting its API binary.

From `backend/`, load the synthetic local configuration and provision your owner:

```bash
set -a
. ./.env.example
set +a
go run ./cmd/owner setup --username owner
```

The command prompts twice without echoing the password. Use a passphrase of at least 15 characters, up to 1024. It is not trimmed or normalized. Setup refuses to replace an existing account, including if you supply a different username.

This is your blog login account. It is separate from the PostgreSQL connection user (`isaber_dev` in the local example).

Then start the API:

```bash
go run ./cmd/api
```

In another terminal, start the frontend from `frontend/`:

```bash
npm run dev
```

Visit `http://127.0.0.1:3000/admin`. Use that exact origin with the example configuration; `localhost` is a different origin and would need a matching `SITE_ORIGIN`. Article-management tools are merged; see the [publishing walkthrough](publishing.md).

Five validated login attempts are allowed per minute per API process. If you see a rate-limit message, wait a minute before retrying. Repeated incorrect attempts and successful logins both count. Invalid cross-origin or malformed requests do not consume the password-check budget.

The sign-in form selects **Keep me signed in for 30 days** by default. This creates a persistent HttpOnly session cookie and a server-side session with a fixed 30-day expiry, so closing the browser or returning after days away does not require signing in again. Uncheck it to use the previous eight-hour maximum and 30-minute idle limit. Existing sessions keep their previous policy; sign in once with the option selected to create a remembered session. Signing out or recovering the password revokes remembered sessions immediately. The password is not stored by the application in the browser.

The development CSRF key is ephemeral unless you set `CSRF_SECRET`. Sessions therefore require sign-in again after an API restart by default. A changed cookie/key does not remove article or owner data.

## Password recovery

Recovery requires operator database access, not a public web page:

```bash
go run ./cmd/owner recover --username owner
```

Enter and confirm the replacement passphrase at the hidden prompts. The command updates the existing owner and revokes all their previous sessions atomically. Unknown usernames are not created automatically. Open `/admin` and sign in with the new password afterward.

For automation, `--password-stdin` reads exactly one password line from a secure pipe or password-manager integration. Do not put the password in an argument, an inline shell command, a committed file, a PR comment, or chat. If you use this mode, ensure the supplying process does not log it. The command does not read a password from environment variables.

## Environment variables

| Variable | Purpose |
| --- | --- |
| `DATABASE_URL` | Runtime database connection for API sessions and application data |
| `OWNER_DATABASE_URL` | Explicit operator connection for setup and recovery; no automatic fallback |
| `MIGRATION_DATABASE_URL` | Separate operator connection for migrations |
| `APP_ENV` | Defaults to `production`; use `development` explicitly for local HTTP |
| `SITE_ORIGIN` | Exact browser origin; production requires HTTPS, development requires loopback |
| `CSRF_SECRET` | Base64-encoded 32-byte key; required in production and optional for development |

Production uses secure `__Host-isaber_session` cookies. Development uses a distinct `isaber_dev_session` cookie, so switching between modes does not trust an insecure local cookie as a production session.

Generate the production key inside your secret-management workflow and inject it into the process environment. Never commit or print it in CI logs. Keep it stable across API replicas; cookies issued under a different key fail authentication without renewing their idle timers. Do not restore a retired key; use owner recovery to persistently revoke earlier sessions during a security-related rotation. Operators should use a role permitted to update owner credentials and revoke sessions; the normal runtime role does not need owner credential-update or migration privileges.

The frontend must use the same `BACKEND_ORIGIN` for its production rewrite build and server-side session checks. Its browser requests stay on the website origin. The reverse proxy must preserve the browser's original Origin header. Cross-origin access is not enabled.

## Verification and remaining work

Backend integration tests cover setup refusal, password verification, cookie attributes, stored token hashes, sessions, CSRF/origin rejection, logout revocation, recovery, idle/absolute expiry, key rotation, service restart, and process rate limits. Remembered-session checks include three days of inactivity, unchanged absolute expiry, explicit short-session login, strict boolean validation, restart/key behavior, and revocation after logout/recovery. Migration tests verify existing sessions retain their credentials, timestamps, and original policy. Tests use isolated PostgreSQL schemas and no real credentials.

The hidden terminal setup prompt and stdin recovery were also exercised with synthetic local accounts; password input was not echoed or included in command output.

Frontend HTTP checks verify the rendered sign-in page, POST form fallback, uncached responses, and absence of owner data for visitors. A full local proxy check verifies cookie forwarding and server-rendered authenticated state. Interactive browser and final visual/accessibility review remain separate verification steps.

The November 15, 2026 remembered-login update was additionally checked in Chromium against the real local API: a persistent browser profile was closed and reopened after simulating three days of session inactivity, and the session survived an API restart using the same local key. Cookie expiry, request preference, fixed server expiry, logout revocation, and unchecked-mode idle expiry were verified. The sign-in layout was checked at 1440, 768, 390, and 320 CSS pixels. [Desktop preview](../design/previews/admin-remember-login-desktop.png) and [mobile preview](../design/previews/admin-remember-login-mobile.png) use only synthetic local credentials. Migration 00007 was also applied to production on November 15, 2026 before the application rollout; existing session policies and credentials were preserved.

Article management, private article/media delivery, and comment moderation are merged and have their own access tests. Automated session cleanup and production ingress abuse controls remain separate work.
