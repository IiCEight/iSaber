# ADR 0001: One repository with independent frontend and backend applications

Status: Accepted by the owner.

## Context

The blog is one product with a React frontend and Go backend. Publishing, private content, search, and comments require coordinated API and UI changes. The frontend also needs an independent visual design process. Both applications must support independent builds, tests, and deployments.

## Decision

Keep the applications in `frontend/` and `backend/` within one repository. Keep the shared OpenAPI contract in `contracts/`, with documentation and infrastructure configuration alongside it.

- Maintain separate application dependencies, builds, tests, container images, and deployment pipelines.
- Communicate across the application boundary through the API.
- Review changes spanning the API and both applications in one pull request when appropriate.
- Validate both applications when the shared contract changes.
- Preserve API compatibility across independent releases and record deployed artifact versions.

## Consequences

One checkout supports local development and coordinated review. Shared contracts and documentation can evolve alongside implementation. Frontend design and release work remain independent of backend releases.

CI must select relevant checks without missing shared-contract or infrastructure changes. Required checks must report completion even when application-specific jobs are skipped. Application boundaries require explicit enforcement; directory separation alone is insufficient.

Repository readers can access both applications. Directory ownership can route reviews but does not provide separate source access permissions.

## Reconsider when

- Collaborators need access to only one application's source.
- Separate teams take independent ownership and repository governance.
- The backend serves multiple independently managed products and shared-contract distribution becomes necessary.

If repositories are split, define contract ownership, versioned API client distribution, cross-repository integration testing, and release compatibility before migration.
