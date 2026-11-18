# Friends directory and link requests

Friends is available at `/friends`; its owner controls are in `/admin` → Friends. The owner approved the Friends release on November 18, 2026. Feature 5 (statistics) remains deferred. Deployment follows the standard PR/main workflow; database migration is a separate operator step before the new API starts.

The approved design preserves the owner’s latest circle colors, stationary breathing, hover attraction, optional image URLs, enlarged card icons, 42% vertical icon positioning, reduced card-header spacing, and supplied 256px profile image. The standalone study remains at `docs/design/friends-prototype.html` as a visual reference; it never submits requests. The React page submits real requests to the configured API.

## Owner workflow

- Create an entry with website name, HTTPS URL, introduction, and optional HTTPS icon URL. It starts as a hidden draft.
- Review the entry, check Published, and save to make it public. Clear Published and save to hide it while retaining its information.
- Move entries up/down to change the public order. Hidden entries keep their places in the owner list. Reordering checks the versions of the complete list and commits atomically.
- Review pending friend-link requests. Approval creates a hidden draft; publication remains a separate step. Rejection retains the private request. Reviewed requests cannot be processed again.
- Optional reply email addresses are visible only in the owner review queue. The app sends no automatic email.

Website names allow 80 characters, introductions 300 characters, and URLs 2048 characters. Blank icon URLs use initials. Images are loaded directly in the visitor’s browser without a referrer and fall back to initials when unavailable. The API does not fetch icons, crawl websites, or accept image uploads.

## API and persistence

Migration `00010_friends.sql` adds `friends` and `friend_requests`. It does not change articles, categories, owner credentials, or existing sessions. The API minimum schema version is 10. Local schema 10 was applied after a private local backup; production migration and release remain separate work.

Public `GET /api/v1/friends` returns only published entries and their public display fields. This rule also applies when an owner is signed in. Public `POST /api/v1/friend-requests` accepts same-origin JSON with a UUID Idempotency-Key. Replaying the same key and normalized payload returns the original acceptance for 24 hours; changed payloads conflict. Only committed submissions consume the key. The request acknowledgment returns `pending`, with no email or request data.

Admission limits are global: five new requests per minute, 1000 pending requests, and two concurrent submission operations. There is no visitor-IP storage. Quota responses include Retry-After. The directory holds up to 200 entries. A full directory prevents approval without changing the pending request; rejection remains available.

Owner endpoints require the existing owner session, Origin, and CSRF protections. Every update/review uses an expected version. Hidden directory entries, request status, and reply emails are excluded from the public directory. Public and owner responses remain no-store. See `contracts/openapi.yaml` for request/response definitions.

For a future production release, apply migration 10 using the operator database role, then rerun the existing `infra/grant-runtime.sql` process so the runtime role can access the two new tables before starting the new API. Do not deploy this API against schema 9. Existing binaries remain compatible with the additive tables.

## Local verification

- Go unit/race and full database integration suites passed, including additive migrations and existing article privacy behavior.
- Friends integration checks cover public/owner separation, same-origin/CSRF enforcement, safe URL validation, request quota/replay, optimistic concurrency, atomic reorder rollback, concurrent approval, capacity rollback, and rejection.
- Frontend production build and smoke checks passed. Browser acceptance used a separate temporary schema and synthetic owner, friends, and requests; no production data or local owner was used.
- The actual browser flow covers submit → pending → approve as hidden draft → publish → reorder → hide, private reply-email exclusion, supplied profile, 320–1900px layouts, hover attraction, and light/dark/mobile automated WCAG A/AA checks. Automated checks supplement owner and device review.

Lost-response browser checks additionally confirmed that retrying an already-accepted request or draft creation produces no duplicate. Unconfirmed draft fields stay read-only until the retry resolves. Explicit discard resets unsaved fields. Temporary acceptance services and schema were removed after verification; the normal local Friends directory and request queue remain empty.
