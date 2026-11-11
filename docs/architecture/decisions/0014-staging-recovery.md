# ADR 0014: Coordinated staging snapshots and schema-preserving application releases

Status: Historical. Superseded by [ADR 0019](0019-simple-delivery.md) on November 11, 2026. Retained to explain the former implementation; do not use it for current setup.

## Decision

Before changing a staging application release, stop its writers briefly and snapshot the database, media, secrets, certificates, and deployment metadata together. Keep PostgreSQL running for a custom-format logical dump. Record file/media receipts plus deterministic database row-content hashes. Resume previously running blog services after capture, including on ordinary command failures.

Restore into a new randomly named database and upload volume; compare contents and remove only those disposable targets. A valid snapshot inventory is checked before restore resources are created. Operations share an exclusive operator lock. Local snapshots and their private recovery proofs remain on JD; they contain secrets and are not GitHub artifacts.

Promote only verified immutable application image references with the same reviewed schema, API contract hash, and database image as the current release. Pull and verify candidate configuration identities before taking/restoring the recovery point. Candidate migration status must show only the already applied reviewed versions. The promotion command never runs schema migrations.

Journal the prior image environment and release/history metadata atomically before switching application references. A failed candidate health gate restores the prior application images and rechecks HTTPS while keeping current database and media state. Record current/previous release metadata only after success. An unfinished image-update journal blocks another promotion and can be recovered explicitly. File and directory flushes support recovery from interrupted metadata writes.

## Consequences and acceptance

The shared 2 GiB server uses a single application instance with brief maintenance interruptions, rather than promising zero downtime. Application rollback never rewinds database/media writes or downgrades schema. A schema, contract, database-baseline, proxy, or origin change requires a separate compatibility review.

Cloud acceptance includes nonempty database/media restore probes, a clean restored snapshot, application promotion between two existing verified releases, explicit rollback, and recovery from an injected failing candidate health gate. The intended final application release remains `8cca5a4310627f9585bef4c845db278007729754`.

This is operator-driven private staging. Offsite encrypted recovery, scheduling/retention, full-host rebuild acceptance, schema-evolution rollout policy, a restricted deployment identity, and unattended GitHub delivery remain subsequent work. See the [snapshot runbook](https://github.com/IiCEight/iSaber/blob/174450fe5fa6097727b68b0b55353f2e6de73596/docs/operations/staging-backups.md) and [release runbook](https://github.com/IiCEight/iSaber/blob/174450fe5fa6097727b68b0b55353f2e6de73596/docs/operations/staging-releases.md).
