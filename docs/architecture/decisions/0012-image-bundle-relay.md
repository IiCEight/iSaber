# ADR 0012: Verified image bundles and an operator relay

Status: Historical. Superseded by [ADR 0019](0019-simple-delivery.md) on November 11, 2026. Retained to explain the former implementation; do not use it for current setup.

## Evidence

GitHub-hosted builds, container acceptance, TCR HTTPS, and registry authentication succeeded. The frontend push from GitHub exceeded its 1,200-second limit after the earlier publishing run hit the overall 30-minute limit. A public fixture pushed successfully from JD to TCR, while a bounded GitHub release download from JD received no bytes in 25 seconds. The data path from GitHub to the registry is not established as reliable.

## Decision

Keep builds and acceptance on GitHub. Export the tested frontend/backend images and pinned PostgreSQL identity into a verified bundle before attempting registry delivery. Default to an operator relay through the existing authenticated workspace:

```text
successful main CI
  → GitHub builds and tests exact images
  → GitHub retains verified bundle
  → operator workspace downloads through gh
  → existing authenticated SSH copies bundle to JD
  → JD verifies and copies images to TCR, preserving digests
  → operator retains verified release.json
```

The operator command validates the workflow identity, successful bundle job, source main CI, run attempt, inventory hashes, platform, image configuration, layer digests, and PostgreSQL provenance. This binds a bundle to its verified source rather than the default branch's possibly newer tip. It refuses PR/fork/failed-source artifacts.

Skopeo copies compressed image content through its directory transport and uses `--preserve-digests` for publication. Docker archives isolate export from Ubuntu Skopeo's older Docker-daemon API client. The destination registry configuration is checked against the tested config digest before a completed release manifest is written.

Use the workspace's existing `gh` and SSH authentication and JD's existing root-only Docker registry login. The default route copies neither GitHub nor registry credentials between machines. No runner, poller, deployment key, inbound port rule, or service is added. JD receives only verified images and the reviewed bundle tool, not the full repository. Temporary files are removed after success/failure; release metadata stays in ignored local storage.

## Operational consequences

This is an operator-triggered recovery path, not unattended registry publishing. A green **Prepare release images** run means a tested bundle exists; it does not mean images are published or services deployed. Actual publication is complete only when the relay returns a verified `release.json`. Separate staging TLS, deployment identities, backups, and acceptance remain required.

Bundle artifacts expire after one day and use GitHub artifact storage allowance. Retain the published manifest and registry digests for rollback; rerun preparation if the bundle expires before use. Direct publishing is an explicit opt-in repository variable for future network testing, not the default. Do not treat a failed direct run as a successful preparation; switch back to relay mode and rerun before using the conservative operator gate.

## Validation

Local checks include trusted-run and integrity failure tests, a real 284 MiB export, and SSH transfer/extraction/verification on JD in about 185 seconds with publication omitted. A separate fixture confirmed Skopeo can preserve its exact manifest/config digest when copying through JD's existing TCR login. The merged release at commit `87aa23a1e77571cee9b9c6b682c52821caa25a6c` published all three real images; JD pulled their exact digests and verified their configurations. Node and PostgreSQL executable checks passed, with existing Docker/SSH/relay services active.


## Performance update

Version 2 bundles omit PostgreSQL layers and carry a reviewed upstream/configuration/TCR digest pin. GitHub still tests the real pinned upstream image; export refuses configuration drift, and JD verifies the existing TCR configuration before completing a manifest. A baseline upgrade requires explicit mirroring, verification, and a reviewed pin update. The real bundle shrank from approximately 283 MiB to 124.3 MiB. Version 1 verification remains supported for retained older runs.

An optional `--transport server` route lets JD download a GitHub artifact using a short-lived signed URL sent through SSH stdin. The GitHub token stays local. The archive is bound to GitHub's digest/size receipt and the separately verified metadata before bundle verification/publication. ZIP extraction rejects traversal, duplicates, and symlinks. The URL is restricted to GitHub HTTPS storage endpoints and is omitted from errors and process arguments.

Keep the workspace route as default: a live signed-artifact probe on JD received only 2 MiB in 72 seconds, materially slower than the measured workspace relay. Direct receipt is an option for future network improvement, with a ten-minute download deadline. This change does not remove the duplicate build across main CI/preparation or add unattended deployment.
