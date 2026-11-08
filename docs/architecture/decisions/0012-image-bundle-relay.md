# ADR 0012: Verified image bundles and an operator relay

Status: Implemented for review after two failed direct publishing runs. Actual release-bundle publication remains pending the merged workflow and relay command.

## Evidence

GitHub-hosted builds, container acceptance, TCR HTTPS, and registry authentication succeeded. The frontend push from GitHub exceeded its 1,200-second limit after the earlier publishing run hit the overall 30-minute limit. A public fixture pushed successfully from JD to TCR, while a bounded GitHub release download from JD received no bytes in 25 seconds. The data path from GitHub to the registry is not established as reliable.

## Decision

Keep builds and acceptance on GitHub. Export the tested frontend, backend, and PostgreSQL images into a verified bundle before attempting registry delivery. Default to an operator relay through the existing authenticated workspace:

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

Use the workspace's existing `gh` and SSH authentication and JD's existing root-only Docker registry login. No credential values are extracted or sent to the other machine. No runner, poller, deployment key, inbound port rule, or service is added. JD receives only verified images and the reviewed bundle tool, not the full repository. Temporary files are removed after success/failure; release metadata stays in ignored local storage.

## Operational consequences

This is an operator-triggered recovery path, not unattended registry publishing. A green **Prepare release images** run means a tested bundle exists; it does not mean images are published or services deployed. Actual publication is complete only when the relay returns a verified `release.json`. Separate staging TLS, deployment identities, backups, and acceptance remain required.

Bundle artifacts expire after one day and use GitHub artifact storage allowance. Retain the published manifest and registry digests for rollback; rerun preparation if the bundle expires before use. Direct publishing is an explicit opt-in repository variable for future network testing, not the default. Do not treat a failed direct run as a successful preparation; switch back to relay mode and rerun before using the conservative operator gate.

## Validation

Local checks include trusted-run and integrity failure tests, a real 284 MiB export, and SSH transfer/extraction/verification on JD in about 185 seconds with publication omitted. A separate fixture confirmed Skopeo can preserve its exact manifest/config digest when copying through JD's existing TCR login. Whole-application GitHub artifact download and publication are not yet accepted.
