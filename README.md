# iSaber

A personal blog for journals, technical articles, and selected work. React and Go live in one repository and build independently.

## Current state

The PRD and logical API/data model are approved. This foundation adds a runnable Next.js/React shell, a Go HTTP service, and separate CI jobs. The visual direction, owner authentication, database, Markdown import, and publishing features are not implemented yet. No cloud deployment is configured.

## Local development

Prerequisites: Node.js **24.22.2** with npm, and Go **1.27.3**. Version pins are recorded in `frontend/.nvmrc` and `backend/go.mod`. On WSL, use Linux Node.js/npm rather than Windows npm from a mounted drive.

Start Go in one terminal:

```bash
cd backend
go run ./cmd/api
```

Start React in another:

```bash
cd frontend
npm ci
npm run dev
```

Open `http://127.0.0.1:3000`. The public shell and `/admin` shell contain no real article or owner data. Go listens on `127.0.0.1:8080` by default. Next.js forwards `/api/*` requests to Go, keeping browser requests on one origin.

| Endpoint | Current behavior |
| --- | --- |
| Frontend `/` | Neutral public shell; final visual design is pending |
| Frontend `/admin` | Nonfunctional owner-access shell; no sign-in form or management data |
| Frontend `/healthz` | Frontend process liveness |
| Go `/healthz` | Go process liveness |
| Go `/readyz` | Readiness with no external dependencies yet |
| `/api/v1/*` | Problem-format `404` until business endpoints are implemented |

Configure Go's bind address with `HTTP_ADDR`. Configure the frontend's proxy target with `BACKEND_ORIGIN` (see `frontend/.env.example`). Next.js evaluates the proxy target at build/dev startup; rebuild production after changing it. Do not put credentials into this URL or browser-visible environment variables.

Both development servers bind to loopback. There is no production authentication or TLS setup yet; these commands are for local development, not public server deployment.

## Checks

Run from `frontend/`:

```bash
npm run typecheck
npm run lint
npm run build
npm run test:smoke
```

The smoke test requires a successful production build and temporarily uses port 4317. It verifies server-rendered pages, uncached responses, the admin shell, liveness, and missing-page behavior without requiring Go. It is not browser-based visual or accessibility verification.

Run from `backend/`:

```bash
gofmt -l .
go vet ./...
go test -race ./...
go build -trimpath -o bin/isaber-api ./cmd/api
```

`gofmt -l` should print nothing. Go has no third-party modules yet, so `go.sum` is intentionally absent. See the [contract guide](contracts/README.md) for separate API specification checks.

GitHub Actions runs **API contract**, **Frontend**, and **Backend** independently. Add the latter two as required checks in branch protection after their first successful remote run. Nothing deploys from this workflow.

## Project documents

- [Product requirements](docs/product/requirements.md)
- [Development plan](DEVELOPMENT_PLAN.md)
- [Implementation backlog](docs/product/backlog.md)
- [Data model](docs/architecture/data-model.md)
- [API design](docs/architecture/api-design.md)
- [Foundation technology decision](docs/architecture/decisions/0003-application-foundation.md)
- [Guided GitHub CI/CD setup](docs/operations/github-cicd.md)
