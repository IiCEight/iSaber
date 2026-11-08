# Implementation backlog

Status: Derived from the approved PRD. Ordered for incremental delivery; a checked item indicates local completion, not a production release.

| Order | Work item | PRD coverage | Completion evidence |
| --- | --- | --- | --- |
| 1 | Independent frontend/backend scaffold and CI | Foundation for all capabilities | Separate builds/checks, real HTTP smoke tests, documented local setup |
| 2 | Database/storage choices and migrations | AUTH-01, PUB-01–03, META-01, PRIV-01 | Decision records, migration apply/upgrade verification, constraints for revision ownership |
| 3 | Owner setup, sessions, CSRF, recovery | AUTH-01, ADMIN-01 | Documented commands, sign-in/out, unauthorized and expired-session tests |
| 4 | First Markdown import and publishing flow | PUB-01, PUB-02, READ-01, PRIV-01 | Owner uploads, previews, and publishes an article; no anonymous private access |
| 5 | Images, robust imports, revisions, metadata | PUB-01–03, META-01 | ZIP validation, failed-job recovery, version conflicts, safe retry and stable URLs |
| 6 | Public visual direction and implementation | DESIGN-01, READ-01, PORT-01 | Owner-approved reference analysis/prototype; representative English/Chinese content |
| 7 | Categories, tags, browsing, search | META-01, FIND-01 | Public-only results, label management, English/Chinese search fixtures |
| 8 | Guest comments and moderation | COMM-01 | Pending/approved visibility, owner moderation, disabled-comment and rate-limit tests |
| 9 | Staging delivery and production readiness | Operational acceptance | Registry access, guided setup, real deployments, backup restoration, rollback rehearsal |

Visual design discovery can run alongside backend development after the owner provides references. Privacy checks begin with the first authenticated/data endpoints and are expanded in every subsequent slice.

## Foundation progress

- [x] Independent source directories and toolchain version pins.
- [x] Neutral React/Next.js public and owner-access shells; final design remains open.
- [x] Go process health endpoints, problem responses, HTTP timeouts, and shutdown handling.
- [x] Frontend/backend CI jobs added alongside the existing contract check.
- [x] Complete local build, static analysis, and smoke verification for the foundation branch, including real Next.js-to-Go forwarding and clean Go shutdown.
- [x] Merge the application foundation in PR #3 and observe all three successful GitHub checks.
- [ ] Add Frontend and Backend as required checks after they report successfully.
- [x] Implement PostgreSQL/pgx and Goose migrations in the persistence branch; select private COS as the production object-storage boundary, with its adapter deferred to import work.
- [x] Validate migration upgrades/reruns, ownership constraints, the migration CLI, and readiness transitions against real local PostgreSQL; verify unit/race tests and builds.
- [x] Merge persistence in PR #4 and observe its successful PostgreSQL CI checks.

## Owner authentication progress

- [x] Implement hidden-prompt/stdin owner setup and atomic credential recovery with session revocation.
- [x] Implement password hashing, bounded login requests, session expiry, origin/CSRF checks, and production/development cookie isolation.
- [x] Connect the `/admin` sign-in page and sign-out to the real Go API; return owner identity only after a session check.
- [x] Verify unit/race tests and PostgreSQL integration coverage for sessions, recovery, expiry, cookies, and rate limits.
- [x] Verify the full local Next.js/Go proxy, setup/recovery CLI, cookie forwarding, protected SSR, and live OpenAPI response schemas.
- [x] Merge authentication in PR #5 and observe its successful GitHub checks.

## First publishing progress

- [x] Implement `.md` acceptance, private durable staging, queued imports, and metadata correction.
- [x] Implement immutable drafts, explicit publication/unpublication, stable URLs/dates, optimistic versions, and retry keys.
- [x] Connect owner upload/preview/management and public reading using a shared Markdown/code/math renderer.
- [x] Verify PostgreSQL integration/privacy tests and the actual Next.js/Go upload → preview → publish → read flow against OpenAPI.
- [x] Merge publishing in PR #6 and observe successful checks on the PR and main.
- [ ] Perform interactive browser, keyboard, and visual acceptance.

See the [publishing guide](../operations/publishing.md). Local image/ZIP imports and authorized delivery are merged. Published version browsing/restoration is merged; production private COS setup remains a separate guided deployment step. Final public design still requires the owner's references and approval.


## Typora image import progress

- [x] Validate ZIP paths/counts/expansion, supported image bytes/extensions, and decoded pixel bounds.
- [x] Rewrite inline/reference Markdown image destinations and attach article-owned image records atomically.
- [x] Preserve image memberships through metadata edits and enforce current public versus owner-retained access.
- [x] Verify Go/PostgreSQL privacy tests, production builds, and real Next.js/Go ZIP/image HTTP acceptance.
- [x] Merge images in PR #7 and observe successful checks on both the PR and main.
- [ ] Perform interactive browser acceptance for image imports and reading.
- [ ] Configure and verify private COS, storage retention/quotas, and coordinated backup/restore before cloud deployment.


## Published revision history progress

- [x] Implement owner-only published snapshot pagination/detail, excluding draft-only and cross-article access.
- [x] Restore historical Markdown/metadata/image memberships into a new draft with source provenance and version checks.
- [x] Preserve live content, controls, URLs, and article publication/update dates; require explicit publication.
- [x] Connect dashboard version reading/restoration and verify Go/PostgreSQL privacy, pagination ties, retry safety, and the real Next.js/Go proxy flow.
- [x] Merge history in PR #8 and observe successful checks on both the PR and main.
- [ ] Perform interactive browser acceptance for history reading/restoration.

Category/tag management, public browsing, and public search are merged together in PR #9 while the owner gathers artistic design references. Production COS and CI/CD deployment remain guided release work.


## Category and tag progress

- [x] Add owner-only label creation/rename/list operations and public lists restricted to current public published memberships.
- [x] Add deterministic name/ID cursors scoped by label kind and public/admin audience.
- [x] Coordinate renames with article writes, preserving identities/associations and transactionally updating affected versions/dates.
- [x] Add complete dashboard selectors/management, public filters, and article label links.
- [x] Verify backend privacy/concurrency tests, frontend builds, and the actual Next.js/Go label/browsing flow against OpenAPI.
- [x] Merge content discovery in PR #9 and observe successful PR/main checks.
- [ ] Perform interactive browser acceptance for content discovery.

The comments milestone is merged in PR #10. Final design still requires owner references; private COS and cloud deployment remain guided release work.


## Content-discovery milestone review

Owner preference: continue related implementation on one branch with focused commits, then open/update and merge one milestone PR. Categories/tags, public browsing, and English/Chinese search were bundled in PR #9; use the same approach for comments.

- [x] Implement public-only title/summary/tag/body search with word/literal matching and ranked query-bound cursors.
- [x] Add readable snippets, search/filter continuation, and distinct empty/no-result/error states.
- [x] Test matching, ranking ties, live changes, private/draft/history exclusions, and bounded search concurrency.
- [x] Verify the combined local Next.js/Go search/organization flow and OpenAPI payloads.
- [x] Merge one combined content-discovery PR (#9) and observe successful GitHub checks.


## Guest comments and moderation milestone

- [x] Add pending guest submission, exact-origin/bounded plain-text validation, and guarded idempotent receipts.
- [x] Add approved-only public reading, owner filtering/moderation, and article comment controls.
- [x] Persist sliding accepted-submission quotas across instances and add migration 00005's creation-time index.
- [x] Connect server-rendered approvals, retry-preserving visitor forms, and owner moderation UI.
- [x] Verify PostgreSQL privacy/quota/concurrency tests, builds, and the full local proxy flow with OpenAPI payloads.
- [x] Merge the combined comments PR (#10) and observe successful PR/main GitHub checks.
- [ ] Perform interactive browser/keyboard acceptance for comments.

Publishing, organization, search, and moderated feedback are implemented locally after this milestone. Portfolio content and final artistic design still need owner input and approval; private COS, CI/CD deployment, operational backups/restore, and release acceptance remain separate work.


## Container preparation milestone

- [x] Prepare separate non-root application images, explicit operator tools, and private persistent-volume wiring.
- [x] Add an isolated container CI rehearsal for migrations, readiness, SSR/static assets, proxy authorization, and restart.
- [x] Validate separate image builds and full Compose acceptance locally through Podman's Docker-compatible API, including database outage/restart and retained migration state.
- [x] Merge the container milestone in PR #11 and observe successful PR/main checks, including Containers on GitHub's Docker engine.
- [ ] Select registry/connectivity and prepare cloud TLS, deployment identities, backups, and private storage with owner guidance.

See the [container guide](../operations/containers.md). This is application packaging; no cloud deployment pipeline is active.


## Guided staging preparation

- [x] Inspect both shared servers and registry connectivity without modifying their configuration.
- [x] Record port conflicts, package/network constraints, and concrete owner exercise commands in the [staging preparation guide](../operations/staging-preparation.md).
- [ ] Owner refreshes JD package metadata; review Docker/Compose installation simulation.
- [ ] Agree staging HTTPS access and verify a full image pull before implementing deployment.

Continue related setup work on one milestone branch; no per-exercise PR cycle is needed.
