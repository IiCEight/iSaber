from datetime import datetime, timedelta, timezone
import fcntl
import hashlib
import json
import shutil
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "staging"))
import retention
import vault
from vault_protocol import canonical

NOW = datetime(2026, 11, 10, 4, tzinfo=timezone.utc)


def record(at, suffix="1234abcd", received=None):
    return {"id": at.strftime("%Y%m%dT%H%M%SZ-") + suffix, "captured": at, "received": received or at}


class SelectionTests(unittest.TestCase):
    def test_daily_weekly_monthly_and_new_receipts(self):
        records = [record(NOW - timedelta(days=day)) for day in range(500)]
        keep = retention.select(records, "tencent", NOW, pins={records[-1]["id"]})
        self.assertTrue(all(item["id"] in keep for item in records[:30]))
        self.assertIn(records[-1]["id"], keep)
        self.assertNotIn(records[-2]["id"], keep)
        # The newest actual capture in each of the last 12 calendar months stays.
        for month in range(12):
            number = NOW.year * 12 + NOW.month - 1 - month
            matching = [item for item in records if item["captured"].year * 12 + item["captured"].month - 1 == number]
            self.assertIn(matching[0]["id"], keep)
        old_received_today = record(NOW - timedelta(days=490), "aaaaaaaa", NOW)
        self.assertIn(old_received_today["id"], retention.select([*records, old_received_today], "tencent", NOW, {}))

    def test_china_day_boundaries_select_newest_capture(self):
        # 15:59Z and 16:01Z fall on different days in China.
        first = datetime(2026, 10, 30, 15, 59, tzinfo=timezone.utc)
        records = [record(first), record(first + timedelta(minutes=2)), record(first + timedelta(minutes=3), "aaaaaaaa")]
        records += [record(NOW - timedelta(days=day)) for day in range(4)]
        keep = retention.select(records, "tencent", NOW, {})
        self.assertIn(records[0]["id"], keep)
        self.assertNotIn(records[1]["id"], keep)
        self.assertIn(records[2]["id"], keep)

    def test_jd_keeps_seven_days_latest_ten_and_protected(self):
        records = [record(NOW - timedelta(days=day)) for day in range(30)]
        protected = records[-2]["id"]
        keep = retention.select(records, "jd", NOW, pins={records[-1]["id"]}, protected={protected})
        self.assertTrue(all(item["id"] in keep for item in records[:10]))
        self.assertNotIn(records[10]["id"], keep)
        self.assertIn(protected, keep); self.assertIn(records[-1]["id"], keep)
        with self.assertRaises(ValueError): retention.select([record(NOW + timedelta(hours=1))], "jd", NOW, {})


class LifecycleTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(); self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        (self.root / "jd-staging").mkdir(); (self.root / "retention").mkdir()
        self.key = self.root / "signing"
        subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(self.key)], check=True, capture_output=True)
        (self.root / "allowed_signers").write_text("isaber-jd-staging " + self.key.with_suffix(".pub").read_text())
        self.recipient = "age1" + "q" * 58
        (self.root / "policy.json").write_text(json.dumps({"recipient": self.recipient}))
        self.pin = self.object(NOW)
        self.configuration = {"version": 1, "enabled": True, "pins": {self.pin: "b" * 64}}
        self.save_policy()
        self.old = [self.object(datetime(2024, 1, day, tzinfo=timezone.utc)) for day in (1, 2, 3)]

    def save_policy(self):
        (self.root / "retention/policy.json").write_text(json.dumps(self.configuration))

    def object(self, at):
        identifier = record(at)["id"]
        directory = self.root / "jd-staging" / identifier; directory.mkdir()
        cipher = b"age-encryption.org/v1\nfixture"
        manifest = {"version": 1, "id": identifier, "cipher_bytes": len(cipher), "cipher_sha256": hashlib.sha256(cipher).hexdigest(), "plain_bytes": 1, "plain_sha256": "a" * 64, "backup_sha256": "b" * 64, "source_commit": "c" * 40, "recipient": self.recipient}
        path = directory / "manifest.json"; path.write_bytes(canonical(manifest))
        subprocess.run(["ssh-keygen", "-Y", "sign", "-f", str(self.key), "-n", "isaber-backup", str(path)], check=True, capture_output=True)
        (directory / "manifest.json.sig").rename(directory / "signature")
        (directory / "snapshot.age").write_bytes(cipher)
        (directory / "receipt.json").write_text(json.dumps({"status": "stored", "manifest": manifest, "received_at": at.isoformat()}))
        return identifier

    def test_dry_run_quarantine_grace_period_and_eventual_purge(self):
        first = retention.plan(self.root, "tencent", NOW)
        self.assertEqual([item["id"] for item in first["quarantine"]], [self.old[0]])
        self.assertTrue((self.root / "jd-staging" / self.old[0]).exists())
        retention.apply(self.root, first)
        quarantined = retention.trash_directory(self.root, "vault", self.old[0])
        self.assertTrue((quarantined / "object/snapshot.age").exists())
        self.assertFalse(retention.plan(self.root, "tencent", NOW)["purge"])
        later = NOW + timedelta(days=8)
        self.object(later)
        second = retention.plan(self.root, "tencent", later)
        self.assertEqual([item["id"] for item in second["purge"]], [self.old[0]])
        retention.apply(self.root, second)
        self.assertFalse(quarantined.exists())
        self.assertTrue((self.root / "jd-staging" / self.pin).exists())

    def test_disabled_policy_and_missing_pin_refuse_mutation(self):
        self.configuration["enabled"] = False; self.save_policy()
        result = retention.plan(self.root, "tencent", NOW)
        with self.assertRaises(ValueError): retention.apply(self.root, result)
        self.configuration["pins"] = {record(NOW - timedelta(days=1))["id"]: "b" * 64}; self.save_policy()
        with self.assertRaises(ValueError): retention.plan(self.root, "tencent", NOW)

    def test_integrity_and_stale_backup_fail_closed(self):
        with self.assertRaises(ValueError): retention.plan(self.root, "tencent", NOW + timedelta(days=2))
        cipher = self.root / "jd-staging" / self.old[0] / "snapshot.age"
        cipher.write_bytes(b"tampered")
        with self.assertRaises(ValueError): retention.plan(self.root, "tencent", NOW)
        self.assertTrue(cipher.exists())

    def test_symlinks_are_never_traversed_or_deleted(self):
        outside = self.root / "outside"; outside.mkdir(); (outside / "valuable").write_text("keep")
        (self.root / "jd-staging" / self.old[0] / "escape").symlink_to(outside, target_is_directory=True)
        with self.assertRaises(ValueError): retention.plan(self.root, "tencent", NOW)
        self.assertEqual((outside / "valuable").read_text(), "keep")

    def test_retention_and_export_share_the_vault_lock(self):
        with retention.locked(self.root, "tencent"), (self.root / ".lock").open("a") as other:
            with self.assertRaises(BlockingIOError): fcntl.flock(other, fcntl.LOCK_EX | fcntl.LOCK_NB)
            with patch.object(vault, "ROOT", self.root), self.assertRaises(BlockingIOError): vault.export(self.pin)

    def test_jd_health_gate_and_pending_promotion_prevent_inventory_or_deletion(self):
        import backup_health
        with patch.object(backup_health, "collect", return_value={}), patch.object(backup_health, "evaluate", return_value=["backup_overdue"]), patch.object(retention, "jd_records") as inventory:
            with self.assertRaises(ValueError): retention.plan(self.root, "jd", NOW)
            inventory.assert_not_called()
        (self.root / "pending-promotion.json").write_text("{}")
        with patch.object(retention, "jd_records") as inventory:
            with self.assertRaises(ValueError): retention.plan(self.root, "jd", NOW)
            inventory.assert_not_called()

    def test_jd_retention_uses_existing_backup_and_promotion_lock(self):
        import backup
        with patch.object(backup, "credentials", return_value={}):
            with retention.locked(self.root, "jd"), self.assertRaises(BlockingIOError):
                with backup.lock(self.root): self.fail("Concurrent lock acquired")

    def test_jd_unknown_outbox_files_require_operator_review(self):
        source = self.root / "jd-staging" / self.pin
        target = self.root / "offsite/outbox" / self.pin
        shutil.copytree(source, target)
        self.assertEqual(len(retention.jd_records(self.root)[1]), 1)
        valuable = target / "operator-note.txt"; valuable.write_text("preserve")
        with self.assertRaises(ValueError): retention.jd_records(self.root)
        self.assertEqual(valuable.read_text(), "preserve")

    def test_enabled_maintenance_alerts_for_stale_failed_or_disabled_runs(self):
        timer = b"LoadState=loaded\nActiveState=active\nUnitFileState=enabled\n"
        service = b"LoadState=loaded\nActiveState=inactive\nResult=success\n"
        def units(*_args, **_kwargs):
            return subprocess.CompletedProcess([], 0, stdout=timer if _args[0][2].endswith(".timer") else service)
        applied = self.root / "retention/last-apply.json"
        applied.write_text(json.dumps({"state": "applied", "checked_at": NOW.isoformat()}))
        with patch.object(retention.subprocess, "run", side_effect=units):
            self.assertEqual(retention.maintenance_status(self.root, "tencent", NOW), {"enabled": True, "healthy": True})
            self.assertFalse(retention.maintenance_status(self.root, "tencent", NOW + timedelta(hours=27))["healthy"])
            applied.write_text(json.dumps({"state": "failed", "checked_at": NOW.isoformat()}))
            self.assertFalse(retention.maintenance_status(self.root, "tencent", NOW)["healthy"])
        self.configuration["enabled"] = False; self.save_policy()
        with patch.object(retention.subprocess, "run") as command:
            self.assertEqual(retention.maintenance_status(self.root, "tencent", NOW), {"enabled": False, "healthy": True})
            command.assert_not_called()


if __name__ == "__main__": unittest.main()
