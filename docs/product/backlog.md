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
- [ ] Review the authentication branch and observe its real GitHub checks before merging.

The next implementation slice is the first Markdown import, preview, and publishing flow. Article management and private content must reuse the owner authorization boundary and receive their own access tests.
