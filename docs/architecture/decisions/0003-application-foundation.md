# ADR 0003: React and Go application foundation

Status: Accepted and merged in PR #3. Authentication/content features and container delivery followed in later milestones; [ADR 0019](0019-simple-delivery.md) describes the live deployment. The original foundation scope is recorded below.

## Context

The approved product requires a React frontend and Go backend, independently buildable and deployable. Public articles need server-rendered content, while the owner dashboard uses the same API. An artistic visual direction will be reviewed separately.

During the initial foundation validation, the local Linux Go installation was 1.22.2 and Linux Node.js was absent from PATH. Supported toolchains were downloaded into `/tmp` and checked against official SHA-256 metadata for validation; system installations were not changed.

## Decision

- Use Next.js 16.4.5 with React 19 and TypeScript, following the earlier server-rendering recommendation. Exact package versions and transitive resolutions are committed in the npm lockfile.
- Pin Node.js 24.22.2 in `.nvmrc`; use npm and `npm ci` for lockfile-driven installation.
- Use Biome for frontend linting, separately from TypeScript checking and production builds.
- Pin Go 1.27.3 and start with `net/http`, structured standard-library logging, bounded HTTP timeouts, and graceful shutdown. No HTTP framework is required for operational probes.
- Keep browser API calls same-origin through a Next.js `/api/*` rewrite to Go. This forwarding layer does not implement business rules.
- Use dynamic frontend rendering and uncached operational/error responses. Do not introduce shared caches for future private content.
- Keep the UI to a neutral public shell and an admin-access placeholder until design and authentication work land. Do not simulate working owner sign-in.
- Run contract, frontend, and backend CI jobs independently on GitHub-hosted runners, with official setup actions pinned to full commits.

## Consequences

Frontend server rendering requires a Node.js runtime alongside Go in eventual deployment. React can be built and tested without a running Go server, and Go can be tested without Node.js. The frontend lockfile and Go version pin must be updated deliberately as security and maintenance releases arrive.

At introduction, backend readiness was process-only because there were no external dependencies. [ADR 0004](0004-postgresql-and-private-storage.md) adds database-aware readiness and failure tests. Future object-storage integration must also review readiness. Health probes are operational endpoints outside the versioned business API contract.

Initial HTTP timeouts and request sizes are foundation defaults, not final upload configuration. Markdown upload work must reconcile them with the approved file limits and real server/network behavior.

At this milestone, database drivers/migrations, storage, credentials, containers and deployment were later work. PostgreSQL and the filesystem adapter subsequently shipped, and automatic Tencent delivery is now accepted; a COS adapter and generated-client consistency remain future work. This record preserves the scope of the original foundation change.

## References

- [Next.js installation](https://nextjs.org/docs/app/getting-started/installation)
- [Node.js official releases](https://nodejs.org/dist/index.json)
- [Go official releases](https://go.dev/dl/?mode=json)
