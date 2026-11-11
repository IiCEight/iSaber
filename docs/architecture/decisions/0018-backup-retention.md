# ADR 0018: Operator-only retention with recovery pins and quarantine

Status: Historical. Superseded by [ADR 0019](0019-simple-delivery.md) on November 11, 2026. Retained to explain the former implementation; do not use it for current setup.

## Decision

Use independent root-controlled retention jobs on Tencent and JD. Keep seven days of recent copies, plus 30 daily/eight weekly/12 monthly recovery buckets on Tencent; retain at least three vault captures and ten local/outbox captures on JD. Preserve explicit restored recovery pins, pending/last-success work, and available current/previous release points. Use China calendar boundaries.

Require verified integrity, a fresh offsite capture, restored recovery pins, and healthy JD backup state before cleanup. Plan under the existing backup/vault lock and serialize vault exports against cleanup. Move eligible copies into a seven-day quarantine before any permanent purge; count quarantined ciphertext against quota. Retain tombstones so upload keys cannot reuse retired IDs.

The upload and monitoring identities gain no cleanup operation or policy authority. Their only operations remain the existing forced commands. Only the operator approves policy/activation and pins. Store a safe audit trail and refuse cleanup on ambiguity, stale backups, changed pins, or unsafe paths.

## Consequences

Quarantine permits operator recovery during its grace period but does not free space immediately. Retention cannot guarantee fitting arbitrarily large backups within current capacity. Existing backup alerts remain necessary. The initial actual plans keep all ten local snapshots and both encrypted copies, with no quarantine or deletion.

The successfully decrypted/restored snapshot is pinned on both servers. Current code does not remove that pin automatically or replace restore acceptance with a ciphertext checksum. Extend hourly health checks with safe enabled-policy timer/result/freshness alerts, using the existing verification reply for Tencent; no new upload operation is added. Audit/tombstone maintenance and production recovery objectives remain separate work. See the [retention runbook](https://github.com/IiCEight/iSaber/blob/174450fe5fa6097727b68b0b55353f2e6de73596/docs/operations/backup-retention.md).
