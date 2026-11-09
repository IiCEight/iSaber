# ADR 0017: Independent backup checks through restricted SSH and GitHub notifications

Status: Implemented for milestone review.

## Decision

Use an hourly GitHub-hosted check, separate from the JD backup timer, with native failed-workflow notifications selected by the owner. Use a dedicated read-only SSH account/forced command and main-only environment secret; do not reuse the deployment key or introduce a general server shell. Personal Actions email notifications are an owner setting, confirmed enabled during setup.

Check systemd state, the latest attempt, preserved last successful capture, capture age (26 hours), disk reserve, pending work, and actual signed ciphertext on Tencent. Compare capture/server timestamps with the external runner's clock. Treat missing/invalid responses and unavailable monitoring as failures. Publish only bounded, allowlisted health information and fixed error messages.

Persist `last-success.json` separately from the latest attempt so a failed attempt cannot erase the last successful capture from diagnostics. Retrying old ciphertext never resets freshness. Health queries cannot take backups, mutate their status, change services, or restore SQL. Decryption keys remain offline.

## Consequences

The monitor can detect JD outages independently, with no new public listener or email credential. Native notifications follow the owner's GitHub settings and schedule actor rules. Hourly failed runs may repeat alerts; there is no deduplication or dedicated recovery email.

Scheduled Actions can be delayed or disabled after repository inactivity and cannot monitor their own absence. This is private-staging monitoring, not immediate production paging. Retention, an independent dead-man heartbeat, restore cadence, and replacement-host disaster recovery remain separate work. See the [alert runbook](../../operations/backup-alerts.md).
