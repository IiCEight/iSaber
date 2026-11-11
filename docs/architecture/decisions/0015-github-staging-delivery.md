# ADR 0015: Restricted GitHub delivery with authenticated metadata and a file cache

Status: Historical. Superseded by [ADR 0019](0019-simple-delivery.md) on November 11, 2026. Retained to explain the former implementation; do not use it for current setup.

## Context and decision

The workstation relay established reliable publication, but required the owner's machine. JD artifact downloads were slow, and GitHub-to-JD SSH measured between 19.88 and 62.31 seconds for 8 MiB. Keep builds/tests on GitHub, reuse verified server content, and send only missing files over SSH. The full 124.3 MiB dry run required 43.3 MiB of wire data and passed full reconstruction/verification on JD in a 304.1-second transfer.

Use a dedicated fixed-command SSH identity with no shell, forwarding, or Docker-group membership. The private key is an environment secret restricted to `main`. The receiver is installed root-owned and runs with isolated Python. Uploaded programs are never executed, even when their hashes match a source artifact.

The receiver independently authenticates the small metadata artifact through GitHub's SHA-256 receipt and a temporary read-only job token. It validates successful source/preparation runs and the current main ref. File size/hash receipts authenticate reconstructed content from the transfer and cache; complete image/config/layer verification follows. The runner independently verifies the full original ZIP. This avoids requiring the complete ZIP to cross the slow link while retaining source/integrity binding.

The job token is memory-only and restricted to the fixed repository API. Server registry/runtime credentials remain server-side. This deliberately differs from ADR 0012's operator-only credential arrangement; it does not install a permanent GitHub credential or self-hosted runner.

## Deployment boundary

After image verification/publication through JD's existing TCR login, invoke the already verified snapshot/restore/promotion tool. Keep the unchanged schema/API/database policy and pending-update recovery. Check main again immediately before the application switch. Source commits that become stale are not promoted.

GitHub staging jobs are serialized and not canceled mid-operation. Source artifacts are bounded, paths/tar records are restricted, cached bytes are rehashed, and no transport-provided path becomes a control-program path. Automatic delivery cannot update its own root-owned control tools.

A real dry run leaves the current application unchanged. Temporary probe credentials are revoked after testing. The first fully merged workflow run provides final automatic publish/promotion acceptance. Manual operator delivery and recovery remain available; offsite recovery, cache/log retention, schema evolution, and production delivery are subsequent work.
