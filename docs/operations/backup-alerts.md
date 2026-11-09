# Offsite backup health and GitHub alerts

The owner selected GitHub Actions notifications and confirmed Actions email notifications with “Only notify for failed workflows” enabled in their personal account.

## What runs

After this workflow is merged to `main`, **Offsite backup health** runs hourly at minute 17 UTC (minute 17 in China too). It is independent of JD's 03:30 Asia/Shanghai backup timer: the check can fail even when JD is down. A manual **Run workflow** is also available on the default branch. The workflow uses the main-only `staging` environment, read-only repository permission, and the dedicated `BACKUP_MONITOR_SSH_KEY` environment secret.

The monitor fails for:

- An unavailable JD host, rejected SSH credential, invalid host pin, invalid response, or monitor timeout.
- A disabled/inactive backup timer or failed backup service/attempt.
- No successful offsite capture, or a capture more than **26 hours** old.
- A Tencent copy whose signed manifest/ciphertext checksum cannot be verified, including unreachable Tencent.
- Less than **5 GiB** free on JD.
- Pending backup work while its service is not running, or a significant clock mismatch.
- Enabled retention policies with a stopped timer, failed apply, or no successful apply within 26 hours, on JD or Tencent.

Freshness uses the UTC capture timestamp in the immutable snapshot ID, not the time an upload was retried. `last-success.json` preserves the last successful capture when a newer attempt fails; `status.json` continues to describe the latest attempt. The monitor supports the original uploaded `status.json` during upgrade. A pending capture is allowed while the service is running, but an earlier recorded failure remains visible until success.

JD queries systemd and safe metadata, then asks Tencent to verify the existing signed object and its actual encrypted checksum. The vault check retries once for brief contention. Monitoring does not take a new backup, resume/pause applications, change timer state, upload ciphertext, overwrite status, or run a restore. Full restoration still requires the owner's offline decryption identity; this check does not replace a restore rehearsal.

## Restricted access

`isaber-monitor` has a root-owned home and authorized key, no Docker-group membership, and no general shell/forwarding access. Its forced command accepts exactly `backup-health`. One exact sudo command invokes the root-owned checker with isolated Python and a 150-second outer deadline. The checker has its own 130-second deadline; the SSH client has a 180-second deadline and the job a five-minute limit.

The monitoring key cannot invoke deployment, upload, read arbitrary files, or select another root/path/command. It receives only allowlisted health measurements/codes. It never receives article text, database credentials, registry credentials, private keys, or the decryption identity. GitHub logs and summaries contain validated snapshot identifiers, disk space, and fixed diagnostic messages; raw SSH/systemd/private exception output is suppressed.

The private SSH key is stored only in the protected GitHub environment after setup. Temporary local copies and the branch-only acceptance environment are removed after verification. The public key remains in `/var/lib/isaber-monitor/.ssh/authorized_keys`. Existing deployment and vault upload credentials/policies are unchanged.

## Receiving alerts

Visit [GitHub notification settings](https://github.com/settings/notifications) and enable Actions email notifications with **Only notify for failed workflows**. GitHub sends native failed-workflow notifications; we do not install an SMTP server or store an email password. See [GitHub's notification instructions](https://docs.github.com/en/subscriptions-and-notifications/how-tos/managing-github-actions-notifications).

For scheduled workflows, GitHub directs notifications to the user who last modified the cron schedule, or the user who re-enabled a disabled schedule. Retain an active owner as that user when editing/re-enabling this workflow. Watching a repository alone is not proof of delivery. See [GitHub's workflow-notification rules](https://docs.github.com/en/actions/concepts/workflows-and-actions/notifications-for-workflow-runs).

Each unhealthy run fails and can produce another notification; there is no incident deduplication or separate recovery email with the failures-only setting. A subsequent healthy run turns green. The workflow summary and error annotations identify the checks that failed.

## Diagnose and recover

Open the failed **Offsite backup health** run and read its summary. From the workstation:

```bash
ssh jdcloud 'systemctl list-timers isaber-offsite.timer --all --no-pager'
ssh jdcloud 'systemctl show isaber-offsite.service -p ActiveState -p Result -p ExecMainStatus'
ssh jdcloud 'cat /opt/isaber-staging/offsite/status.json'
ssh jdcloud 'cat /opt/isaber-staging/offsite/last-success.json'
ssh jdcloud 'journalctl -u isaber-offsite.service -n 20 --no-pager'
ssh jdcloud 'python3 -I /opt/isaber-staging/config/backup_health.py'
```

The last command prints the safe report and returns nonzero when unhealthy. A monitor connectivity failure needs an SSH/host/key check; it is not evidence that old backups were deleted. A vault-verification failure needs an operator inspection on Tencent before trusting the copy. Do not disable host-key checking or overwrite/delete backups to make an alert disappear.

After fixing the underlying issue, run a backup/retry if needed:

```bash
ssh jdcloud 'systemctl start isaber-offsite.service'
gh workflow run backup-health.yml --ref main
```

A retry of old pending work can successfully upload an old snapshot and remain overdue. If no pending work remains and that capture is still old, start the service again to capture current data. Confirm the new snapshot and the next health check. Consult the [backup/recovery runbook](offsite-backups.md) for decryption and isolated restore commands. Disk retention/deletion policy remains separate work; no cleanup is performed by the monitor.

## Verified acceptance

On November 9, 2026, the real GitHub-hosted monitoring job succeeded through the restricted identity and verified the Tencent copy. In the same [acceptance run](https://github.com/IiCEight/iSaber/actions/runs/42583045310), a separate job intentionally failed using synthetic backup data and no server credentials. The owner confirmed receipt of GitHub's failure notification. That red job was the intended notification drill, not a failed real backup.

An isolated JD metadata fixture with an altered signed receipt was rejected by the actual Tencent verifier. A separate failed-attempt fixture alerted while retaining the prior successful capture. The real monitoring key rejected shell, deployment, upload, path overrides, and arbitrary sudo commands. An unreachable-host check failed with a safe annotation. No live backup or application data was changed by these drills. The branch-only acceptance workflow/environment and local temporary key copies were removed afterward; the final workflow retains only the main-only monitoring credential.

## Installation and rotation

Through the existing root operator SSH connection, install reviewed `backup_health.py` and its matching dependencies (`vault_protocol.py`, `offsite.py`, and the existing backup/staging helpers) into `/opt/isaber-staging/config/`, root-owned and private. Install the updated sender so future success writes both status records. Seed `last-success.json` from an existing `status.json` only when its state is `uploaded`.

Generate a dedicated Ed25519 monitoring key in private temporary operator storage. Transfer only its public key and the reviewed `install_monitor.py`/`monitor_entry.py` to a root-only temporary directory on JD. Run `install_monitor.py --public-key PATH`, validate sudoers, and remove installer staging files. Store the private key with `gh secret set BACKUP_MONITOR_SSH_KEY --env staging` via stdin, retaining the existing pinned destination variables. Test the restricted client, then remove local temporary key copies. Replacing this monitor's key does not require rotating deployment or backup encryption keys.

For emergency revocation, remove the monitor key from its dedicated authorized-keys file and delete `BACKUP_MONITOR_SSH_KEY` from the staging environment. The check will then alert as unavailable; backups/deployment continue independently. Disable the GitHub workflow separately if stopping monitoring intentionally.

## Coverage boundary

This is hourly best-effort monitoring for private staging. GitHub can delay scheduled runs and automatically disables schedules in public repositories after 60 days without repository activity. GitHub outages or a disabled monitor schedule cannot be detected by that same workflow. See [schedule behavior](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#schedule) and [schedule disabling](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/disable-and-enable-workflows).

The 26-hour freshness threshold allows the daily capture time plus a two-hour grace period. Detection can occur at the next hourly run, later if GitHub delays it. Failed attempts can be detected sooner because their status fails independently of age. This is not an immediate paging or production availability guarantee. An independent heartbeat/paging service, retention policy, restore cadence, and full-host recovery acceptance remain separate production-readiness work.
