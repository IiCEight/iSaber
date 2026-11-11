# ADR 0019: One workflow and one blog deployment

Status: Owner approved the simplification plan on November 11, 2026; implementation and full-image acceptance are under review. Live cutover is not activated by this branch.

## Decision

Keep one GitHub Actions workflow. PRs validate the contract, frontend, backend, and containers. Successful main checks build/test the application images once and push those exact local images to GHCR. One Tencent Compose deployment pulls public images through NJU using the CI-published digests. SSH carries two image references, not image data or executable programs. Credentials and article/media data remain on the host.

Keep small shell commands for publication and deployment, a production Compose file, explicit SQL role grants, HTTPS health checks, and previous-image rollback. Do not apply schema migrations during deployment. Keep migrations as explicit reviewed operator actions.

Retire the custom image-bundle format, inter-workflow artifact provenance/repacking, SSH relay/cache/resume protocol, TCR republishing, separate JD staging delivery, and scheduled backup/offsite/retention/monitoring framework. These operational features grew beyond the blog's intended scope. This supersedes ADRs 0012–0018; application packaging and business/privacy requirements remain.

## Consequences

Image availability depends on GHCR and NJU; private upstream images do not fit the anonymous mirror route. Docker verifies the requested digest. Public package visibility is an owner setup step. Source-only build contexts exclude `.env`, runtime uploads, and local article data.

Rollback changes application images and does not restore data or reverse migrations. Persistent volumes survive application updates; the owner explicitly removed automated backup/recovery from scope. Host/domain setup and any one-time data migration precede activation. Existing installed timers and services require the documented retirement steps; source deletion alone does not disable them.

See [deployment](../../operations/deployment.md) and the [mirror evaluation](../../operations/ghcr-mirror-evaluation.md).
