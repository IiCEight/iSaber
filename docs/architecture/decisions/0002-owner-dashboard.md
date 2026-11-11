# ADR 0002: Integrated owner dashboard and one owner account

Status: Accepted by the owner.

## Context

The owner writes articles in Typora and needs an interface to upload content, edit metadata, preview, publish, manage revisions, organize articles, and moderate comments. Reader accounts would add complexity without a current product need.

## Decision

- Provide one owner account, created through a documented setup command rather than a public registration page.
- Defer reader accounts, public registration, and selected-reader sharing. Private content is accessible only to the signed-in owner.
- Implement `/admin` as a protected section of the existing React application, alongside the public blog.
- Use the same Go application, database, and asset storage for public and owner-facing functionality.
- Require Go to authenticate and authorize every management request, including direct API requests.
- Prioritize efficient, responsive, accessible management in the dashboard while developing the public site's artistic design separately.

The subsequently approved [API design](../api-design.md) specifies username/password login, opaque owner sessions, CSRF handling, and operational credential recovery. Hashing parameters, session lifetimes and credential recovery are now implemented in [ADR 0005](0005-owner-authentication.md) and the [owner guide](../../operations/owner-account.md). This ADR itself does not approve unrelated PRD defaults.

## Consequences

The owner gets a purpose-built management interface without maintaining another service or frontend deployment. Public and management features share an application release, while the React frontend and Go backend remain independently deployable under ADR 0001.

Frontend route protection must not substitute for API authorization. Private and draft data must be excluded from unauthenticated responses and public caches. Setup must not create additional owners or silently overwrite credentials when repeated.

Dashboard work can proceed against API contracts without waiting for the public site's final visual direction.

## Reconsider when

- Multiple authors or selected readers require accounts and additional permission rules.
- A separately owned administration application needs independent source access or release management.
- Operational requirements justify separate management hosting or network access controls.
