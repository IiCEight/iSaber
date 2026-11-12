# Implementation backlog

Status: Reconciled with merged PRs and live deployment evidence on November 11, 2026. Checked feature items record implementation/automated verification; live acceptance is identified separately. Browser and final design acceptance remain open.

Current operational scope (owner decision, November 11): replace the relay/staging/backup framework with one workflow, public GHCR images through NJU, and one Tencent Compose deployment. PR #26 is merged and the Tencent blog is live over trusted IP HTTPS, preserving the JD account/article. Ongoing automatic main delivery is enabled with explicit owner approval; the full CI-to-Tencent pipeline has passed. See [ADR 0019](../architecture/decisions/0019-simple-delivery.md).

## Next priorities

| Priority | Remaining work | Current evidence |
| --- | --- | --- |
| 1 | Require Frontend, Backend and Containers in the main ruleset | Pull requests and API contract are already required; all four checks run and gate deployment |
| 2 | Rehearse explicit live application rollback | Failure-path tests pass and the first automatic release retained previous image references |
| 3 | Complete real browser publishing/reading acceptance | HTTP/SSR, database and privacy tests pass; owner browser/mobile/keyboard review is pending |
| 4 | Finish public page acceptance and owner content | Archive, Article and About are integrated on the design branch; [local cross-page review](../design/public-page-review.md) records two reading fixes, the no-JavaScript 404 issue, and remaining live-content/device checks. Biography/contact copy is deferred. |
| 5 | Add a domain when chosen | Trusted public-IP HTTPS works; private persistent storage is accepted, with COS optional |

## Implementation sequence and evidence

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
| 9 | Simple Tencent delivery and launch | Operational acceptance | Tested images reused through GHCR/NJU, persistent storage, public HTTPS, application rollback |

The owner selected and positively reviewed the Vivid Home direction and motion. Remaining page design can continue alongside browser acceptance. Privacy checks begin with the first authenticated/data endpoints and are expanded in every subsequent slice.

## Foundation progress

- [x] Independent source directories and toolchain version pins.
- [x] Merge the initial React/Next.js public and owner-access shells; Vivid Home later replaced the neutral homepage in PRs #22–23.
- [x] Go process health endpoints, problem responses, HTTP timeouts, and shutdown handling.
- [x] Frontend/backend CI jobs added alongside the existing contract check.
- [x] Complete local build, static analysis, and smoke verification for the foundation branch, including real Next.js-to-Go forwarding and clean Go shutdown.
- [x] Merge the application foundation in PR #3 and observe all three successful GitHub checks.
- [ ] Add Frontend, Backend and Containers as required merge checks; the existing main ruleset requires pull requests and API contract only.
- [x] Implement PostgreSQL/pgx and Goose migrations in PR #4. Its initial COS proposal remains a future option; the accepted Tencent deployment uses the private filesystem adapter and persistent uploads volume.
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
- [x] Integrate the owner-approved conventional admin design locally, preserving the existing authenticated API workflows and shared Markdown preview. See the [admin integration review](../design/admin-integration.md); real Go/database acceptance and remote delivery of this UI remain separate work.
- [ ] Perform interactive browser, keyboard, and visual acceptance.

See the [publishing guide](../operations/publishing.md). Local image/ZIP imports and authorized delivery are merged. Published version browsing/restoration is merged. Tencent uses a private persistent uploads volume; COS is optional future storage work. Vivid Home is integrated, while final page design and browser acceptance remain open.


## Typora image import progress

- [x] Validate ZIP paths/counts/expansion, supported image bytes/extensions, and decoded pixel bounds.
- [x] Rewrite inline/reference Markdown image destinations and attach article-owned image records atomically.
- [x] Preserve image memberships through metadata edits and enforce current public versus owner-retained access.
- [x] Verify Go/PostgreSQL privacy tests, production builds, and real Next.js/Go ZIP/image HTTP acceptance.
- [x] Merge images in PR #7 and observe successful checks on both the PR and main.
- [ ] Perform interactive browser acceptance for image imports and reading.
- [x] Accept separate private persistent database/uploads volumes on Tencent and verify the JD content cutover. COS remains optional; automated backups are outside the revised delivery scope.


## Published revision history progress

- [x] Implement owner-only published snapshot pagination/detail, excluding draft-only and cross-article access.
- [x] Restore historical Markdown/metadata/image memberships into a new draft with source provenance and version checks.
- [x] Preserve live content, controls, URLs, and article publication/update dates; require explicit publication.
- [x] Connect dashboard version reading/restoration and verify Go/PostgreSQL privacy, pagination ties, retry safety, and the real Next.js/Go proxy flow.
- [x] Merge history in PR #8 and observe successful checks on both the PR and main.
- [ ] Perform interactive browser acceptance for history reading/restoration.

Category/tag management, public browsing and public search merged in PR #9 and are deployed. Private persistent storage and automatic CI/CD are accepted; browser review and further visual refinement remain.


## Category and tag progress

- [x] Add owner-only label creation/rename/list operations and public lists restricted to current public published memberships.
- [x] Add deterministic name/ID cursors scoped by label kind and public/admin audience.
- [x] Coordinate renames with article writes, preserving identities/associations and transactionally updating affected versions/dates.
- [x] Add complete dashboard selectors/management, public filters, and article label links.
- [x] Verify backend privacy/concurrency tests, frontend builds, and the actual Next.js/Go label/browsing flow against OpenAPI.
- [x] Merge content discovery in PR #9 and observe successful PR/main checks.
- [ ] Perform interactive browser acceptance for content discovery.

Comments merged in PR #10 and are deployed. The owner approved the Vivid Home direction; remaining public pages and browser moderation acceptance still need review.


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

Publishing, organization, search and moderated feedback are implemented and deployed. Portfolio content, final page design and interactive browser acceptance still need owner input/review. Tencent storage, trusted IP HTTPS and automatic delivery have passed operational acceptance.


## Simplified delivery milestone

- [x] Inspect git history and shared Tencent/JD workloads; identify the custom transfer/backup framework as superseded scope.
- [x] Verify anonymous NJU pulls of upstream images and four own-account probe versions by original digest/content. DaoCloud does not allow our application namespace.
- [x] Implement one workflow with four existing checks, one image build/test phase, main-only GHCR publication, and an opt-in Tencent deployment.
- [x] Replace custom Python delivery/backup services with shell, Compose, and explicit runtime-role SQL.
- [x] Test invalid input, rollback, failed pull/start, interruption, and locking; pass full container/SQL permissions acceptance in CI.
- [x] Complete and record full frontend/backend mirror pulls by exact digest, initial timeouts, successful retries, cached timings, and official dependency pulls.
- [x] Merge replacement PR #26 and verify main CI/image publication; pull the resulting frontend/backend digests on Tencent in 17.13s/31.41s.
- [x] Owner selects direct public-IP access; obtain trusted IP HTTPS, accept private persistent storage and verify shared Tencent workloads remain running.
- [x] Install reviewed host files/Compose plugin, private stable secrets, a restricted deployment key and a main-only GitHub production environment.
- [x] Owner selects JD preservation; copy account/article/database/uploads once, compare row fingerprints and file checksums, and verify existing article visibility over public HTTPS.
- [x] Enable ongoing delivery with owner approval and verify full main CI → GHCR → NJU → Tencent in [run 43019383308](https://github.com/IiCEight/iSaber/actions/runs/43019383308), with unchanged database/content and retained previous release references.
- [ ] Verify an explicit live application rollback; rollback input/failure paths already pass automated tests.
- [x] Disable obsolete JD delivery/backup/retention schedules and stop old staging containers; preserve original data/archives.

Historical milestones PR #11–24 established containers, private JD staging, the relay, restricted delivery, encrypted backups, monitoring, retention, and CI image reuse. PR #24 merged and its main CI → preparation → staging release succeeded. The first scheduled backup succeeded on November 10; retention merged but was never activated pending policy approval. The owner subsequently removed staging and automated backups from scope, so replacement-server recovery and retention activation are no longer pending delivery requirements. Historical implementation/runbooks remain in [git history at the last legacy main revision](https://github.com/IiCEight/iSaber/tree/174450fe5fa6097727b68b0b55353f2e6de73596/docs/operations).

See [deployment](../operations/deployment.md), [mirror evidence](../operations/ghcr-mirror-evaluation.md), and [local container acceptance](../operations/containers.md). Removing tracked code does not alter installed server copies.

## Vivid Home and public navigation

- [x] Agree on Home, Archive, Article, About, and Search, with one article model organized by categories/tags.
- [x] Review the full-screen hero composition and horizontal top navigation.
- [x] Select A · Vivid and positively review its GSAP motion prototype.
- [x] Integrate the Home into React with public article data, scoped motion, reduced motion, and useful data states.
- [x] Preserve existing browsing/search/filter/pagination behavior in Archive/Search and retain legacy homepage-query redirects.
- [x] Provide actual navigation destinations and preserve existing owner and reading paths.
- [x] Merge Vivid Home integration (#22) and continuous scroll/lens interactions (#23).
- [ ] Review the live integrated site with representative English/Chinese articles, mobile layouts, keyboard navigation and reduced motion.
- [ ] Refine final owner copy/fonts and separately design Archive, Article, and About.
