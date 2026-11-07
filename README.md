# iSaber

A personal blog for journals, technical articles, and selected work. React and Go live in one repository and build independently.

## Current state

The PRD and logical API/data model are approved. React/Next.js, the Go service, PostgreSQL migrations, and CI are merged. The owner-authentication branch adds real owner setup/recovery, sign-in/out, sessions, and an authenticated `/admin` entry page. Markdown import, article management, object storage, publishing, and the final visual design remain subsequent work. No cloud deployment is configured.

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

Owner setup is a first-time step and refuses to replace an existing account. It prompts for a password without echoing it. See the [owner guide](docs/operations/owner-account.md) for recovery and production configuration.

Start React in another:

```bash
cd frontend
npm ci
npm run dev
```

Open `http://127.0.0.1:3000`, then `/admin` to sign in. Article content is not implemented yet. Go listens on `127.0.0.1:8080` by default. Next.js forwards `/api/*` requests to Go, keeping browser requests on one origin.

| Endpoint | Current behavior |
| --- | --- |
| Frontend `/` | Neutral public shell; final visual design is pending |
| Frontend `/admin` | Owner sign-in, authenticated identity, and sign-out; article tools are not implemented yet |
| Frontend `/healthz` | Frontend process liveness |
| Go `/healthz` | Go process liveness |
| Go `/readyz` | Database connectivity and required schema readiness; returns `503` when unavailable or unmigrated |
| `/api/v1/auth/*` | Owner login, session, and logout |
| `/api/v1/admin/*` | Requires an owner session and CSRF/origin checks on mutations, then returns `404` for unimplemented tools |
| Other `/api/v1/*` routes | Problem-format `404` until implemented |

Configure Go's bind address with `HTTP_ADDR`. Configure the frontend's proxy target with `BACKEND_ORIGIN` (see `frontend/.env.example`). Next.js evaluates the proxy target at build/dev startup; rebuild production after changing it. Do not put credentials into this URL or browser-visible environment variables.

Both development servers bind to loopback. Development authentication uses an isolated local HTTP cookie. Production authentication requires an HTTPS site origin, secure cookies, and an injected CSRF key; TLS hosting and cloud deployment are not configured by these local commands.

## Checks

Run from `frontend/`:

```bash
npm run typecheck
npm run lint
npm run build
npm run test:smoke
```

The smoke test requires a successful production build and temporarily uses port 4317. It verifies server-rendered pages, uncached responses, the visitor sign-in page, liveness, and missing-page behavior without requiring Go. It is not browser-based visual or accessibility verification.

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
- [Guided GitHub CI/CD setup](docs/operations/github-cicd.md)
