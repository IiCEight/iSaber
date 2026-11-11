# ADR 0005: Owner credentials, sessions, and local development

Status: Accepted and merged in PR #5.

## Decision

- Provision the single owner with `owner setup`; replace credentials and revoke sessions atomically with `owner recover`. Both are operator commands requiring `OWNER_DATABASE_URL`. There is no registration or public password-reset API.
- Read passwords through a hidden terminal prompt with confirmation, or an explicitly requested stdin pipe. Password values are never command-line flags or printed output.
- Accept passphrases of 15–1024 Unicode characters, preserving spaces and case, without composition requirements. Reject entirely blank passwords. Store only salted Argon2id hashes with 19 MiB memory, two iterations, one lane, 16-byte salts, and 32-byte output.
- Use cryptographically random 32-byte opaque session tokens. Persist only SHA-256 token hashes and CSRF verification hashes. JSON never includes the bearer token.
- Enforce an eight-hour absolute lifetime and 30-minute idle lifetime in PostgreSQL. Every authenticated request checks expiry/revocation and updates last-seen time. New logins use fresh tokens.
- Derive CSRF tokens using HMAC-SHA-256 over the session token with a separate 32-byte secret. The production `CSRF_SECRET` is base64 encoded and must remain stable across restarts/replicas. Rotating it invalidates prior sessions.
- Use the production `__Host-isaber_session` cookie with `Secure`, `HttpOnly`, `SameSite=Strict`, `Path=/`, and no Domain. Require an exact configured HTTPS `SITE_ORIGIN` for production.
- Require exact same-origin headers for login and unsafe owner operations, JSON-only bounded login requests, and session-bound `X-CSRF-Token` on logout and management mutations. Unknown future admin endpoints are authenticated before returning 404.
- For explicit `APP_ENV=development`, require loopback site origins and API bind addresses, use the separate `isaber_dev_session` cookie, and permit local HTTP. Without a supplied CSRF key, generate a new ephemeral key at startup; restarting then requires signing in again.
- Bound expensive hashing to two concurrent HTTP logins and five validated login attempts per minute per API process, counting successful attempts too. Do not trust client-supplied forwarded IP headers. Return `429` with `Retry-After` when limited.

## Consistency and availability

Login verifies the password before acquiring a shared owner-row lock, then rechecks the stored hash before inserting the session. Recovery updates the owner row and revokes sessions in the same transaction. This prevents a concurrent login with the previous credential hash from creating an unrevoked session after recovery.

Unknown users and incorrect passwords produce the same generic unauthorized response. An unknown-user check runs the same password KDF against a dummy hash. Database errors are generic; no submitted passwords, hashes, cookies, CSRF tokens, connection strings, or private body text are logged.

The rate limiter is intentionally process-local. It resets on restart and is not a distributed ingress control. Its global budget can temporarily affect the legitimate owner under attack. Before public production deployment, review edge-level abuse protection, aggregate limits across replicas, and the operator access/recovery path. No trusted-proxy IP policy is silently assumed here.

Expired and revoked session records are inaccessible; automated retention cleanup remains operational follow-up work before long-term production use. Server-side revocation does not erase previously downloaded content from a browser.

## Frontend scope at PR #5

`/admin` checks the current session on the server without caching, renders sign-in for visitors, and includes owner identity only after successful backend authorization. The browser performs login/logout through the same-origin API rewrite. It keeps the CSRF token in memory, while the cookie stays HttpOnly. No session tokens are stored in localStorage.

This original slice provided identity confirmation and sign-out. Article/image/revision management, labels/search and moderation subsequently merged in PRs #6–10; see the [publishing guide](../../operations/publishing.md). Vivid Home is integrated, while final page design and browser acceptance remain.

## References

- [OWASP password storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [OWASP session management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [Go Argon2 implementation](https://pkg.go.dev/golang.org/x/crypto/argon2)

The hash parameters follow the cited minimum recommendation. Session lifetimes and process limits are project defaults requiring deployment review against actual usage and capacity.
