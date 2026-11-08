# iSaber

A personal blog for journals, technical articles, and selected work. React and Go live in one repository and build independently.

## Current state

The PRD and logical API/data model are approved. The React/Go foundation, PostgreSQL migrations, CI, and owner authentication are merged. Markdown upload → private draft → preview → publish → public reading is merged. Typora ZIP/image imports and authorized image delivery are merged. Published revision history/restoration is merged. Category/tag management, public browsing, and English/Chinese search are merged in PR #9. Guest comments, owner moderation, and article comment controls are merged in PR #10. Separate application images and the container CI rehearsal are merged in PR #11. Guided staging setup is in progress. Direct registry upload timed out; the [image bundle relay](docs/operations/image-relay.md) is prepared for review. See the [publishing guide](docs/operations/publishing.md). Private COS/cloud delivery and the final artistic design remain subsequent work.

## Local development

Prerequisites: Node.js **24.22.2** with npm, Go **1.27.3**, and PostgreSQL **16** (Docker Compose is provided). Version pins are recorded in `frontend/.nvmrc` and `backend/go.mod`. On WSL, use Linux Node.js/npm rather than Windows npm from a mounted drive.

Start the development database from the repository root:

```bash
docker compose -f infra/compose.dev.yml up -d --wait
```

Configure, migrate, and start Go in one terminal:

```bash
cd backend
set -a
. ./.env.example
set +a
go run ./cmd/migrate up
go run ./cmd/owner setup --username owner
go run ./cmd/api
```

The comments API requires migration **00005**. Existing checkouts must run `go run ./cmd/migrate up` before starting the API. It adds a rate-check index and preserves existing data.

Owner setup is a first-time step and refuses to replace an existing account. It prompts for a password without echoing it. See the [owner guide](docs/operations/owner-account.md) for recovery and production configuration.

Start React in another:

```bash
cd frontend
npm ci
npm run dev
```

Open `http://127.0.0.1:3000`, then `/admin` to sign in. Upload a `.md` file or article/image ZIP and follow the [publishing walkthrough](docs/operations/publishing.md). Go listens on `127.0.0.1:8080` by default. Next.js forwards `/api/*` requests to Go, keeping browser requests on one origin.

| Endpoint | Current behavior |
| --- | --- |
| Frontend `/` | Public articles with English/Chinese search and category/tag/language filters; final visual design is pending |
| Frontend `/admin` | Owner sign-in/out, imports, metadata, preview, publishing, history/restoration, category/tag management, comment moderation/controls, and private reading |
| Frontend `/articles/{slug}` | Published public Markdown, code, math, and dates; private/missing articles return 404 |
| Frontend `/healthz` | Frontend process liveness |
| Go `/healthz` | Go process liveness |
| Go `/readyz` | Database connectivity and required schema readiness; returns `503` when unavailable or unmigrated |
| `/api/v1/auth/*` | Owner login, session, and logout |
| `/api/v1/admin/*` | Owner-protected article/import tools; origin/CSRF checks on mutations |
| `/api/v1/articles*` | Public-only list and article detail |
| `/api/v1/articles/{article_id}/assets/{asset_id}` | Current public revision images for visitors; retained images for authenticated owner; uncached private delivery |
| `/api/v1/categories`, `/api/v1/tags` | Labels used by current public published revisions only |
| `/api/v1/articles/search` | Ranked public-only title, summary, tag, and article-text search |
| `/api/v1/articles/{article_id}/comments` | Approved public comments and moderated guest submission; private/unpublished articles return 404 |
| `/api/v1/admin/comments*` | Owner-only moderation and filtered queue |
| Other `/api/v1/*` routes | Problem-format `404` until implemented |

Configure Go's bind address with `HTTP_ADDR`. Configure the frontend's proxy target with `BACKEND_ORIGIN` (see `frontend/.env.example`). Next.js evaluates the proxy target at build/dev startup; rebuild production after changing it. Do not put credentials into this URL or browser-visible environment variables.

Both development servers bind to loopback. Development authentication uses an isolated local HTTP cookie. Production authentication requires an HTTPS site origin, secure cookies, and an injected CSRF key; TLS hosting and cloud deployment are not configured by these local commands.

## Container rehearsal

Separate application images, explicit migration commands, private persistent volumes, and an isolated CI stack are described in the [container guide](docs/operations/containers.md). Cloud deployment remains pending. The next owner exercise is in the [staging preparation guide](docs/operations/staging-preparation.md).

## Checks

Run from `frontend/`:

```bash
npm run typecheck
npm run lint
npm run build
npm run test:smoke
```

The smoke test requires a successful production build and temporarily uses port 4317. It verifies server-rendered pages, uncached responses, the visitor sign-in page, liveness, and missing-page behavior using a small synthetic older-API fixture rather than requiring Go. It also verifies that missing comment routes do not hide an existing article. It is not browser-based visual or accessibility verification.

Run from `backend/`:

```bash
gofmt -l .
go vet ./...
go test -race ./...
go build -trimpath -o bin/isaber-api ./cmd/api
go build -trimpath -o bin/isaber-migrate ./cmd/migrate
go build -trimpath -o bin/isaber-owner ./cmd/owner
```

`gofmt -l` should print nothing. Go dependency versions/checksums are committed in `go.mod` and `go.sum`. See the [database guide](docs/operations/database.md) for real PostgreSQL integration tests and migration operations, and the [contract guide](contracts/README.md) for API specification checks.

GitHub Actions runs **API contract**, **Frontend**, and **Backend** independently. Add the latter two as required checks in branch protection after their first successful remote run. Nothing deploys from this workflow.

## Project documents

- [Product requirements](docs/product/requirements.md)
- [Development plan](DEVELOPMENT_PLAN.md)
- [Implementation backlog](docs/product/backlog.md)
- [Data model](docs/architecture/data-model.md)
- [API design](docs/architecture/api-design.md)
- [Foundation technology decision](docs/architecture/decisions/0003-application-foundation.md)
- [PostgreSQL and private storage decision](docs/architecture/decisions/0004-postgresql-and-private-storage.md)
- [Database setup and migrations](docs/operations/database.md)
- [Owner setup, sign-in, and recovery](docs/operations/owner-account.md)
- [Markdown publishing and recovery](docs/operations/publishing.md)
- [Publishing architecture decision](docs/architecture/decisions/0006-first-markdown-publishing.md)
- [Typora image import decision](docs/architecture/decisions/0007-typora-images.md)
- [Category/tag rename coordination](docs/architecture/decisions/0008-label-renames.md)
- [English/Chinese public search](docs/architecture/decisions/0009-public-search.md)
- [Guest comments and moderation](docs/architecture/decisions/0010-guest-comments.md)
- [Guided GitHub CI/CD setup](docs/operations/github-cicd.md)
