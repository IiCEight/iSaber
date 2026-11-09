from contextlib import redirect_stdout
from datetime import datetime, timedelta, timezone
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "staging"))
import backup_health as health
import backup_monitor
from install_monitor import authorized_key

NOW = datetime(2026, 11, 10, 4, tzinfo=timezone.utc)


def healthy():
    return {"version": 1, "checked_at": NOW.isoformat(), "timer_enabled": True, "timer_active": True, "service_active": "inactive", "service_result": "success", "last_attempt": "uploaded", "last_snapshot_id": "20261110T033000Z-1234abcd", "vault_verified": True, "disk_free_bytes": 8 * 1024**3, "pending": False, "errors": []}


class HealthTests(unittest.TestCase):
    def test_healthy_and_every_material_failure(self):
        self.assertEqual(health.evaluate(healthy(), NOW), [])
        cases = [({"timer_enabled": False}, "timer_disabled"), ({"timer_active": False}, "timer_inactive"), ({"service_result": "timeout"}, "backup_service_failed"), ({"last_attempt": "failed"}, "last_attempt_failed"), ({"last_snapshot_id": None}, "snapshot_missing"), ({"vault_verified": False}, "vault_unverified"), ({"disk_free_bytes": 0}, "low_disk_space"), ({"pending": True}, "pending_backup"), ({"checked_at": (NOW - timedelta(minutes=6)).isoformat()}, "clock_skew")]
        for changes, expected in cases:
            with self.subTest(expected=expected):
                self.assertIn(expected, health.evaluate({**healthy(), **changes}, NOW))

    def test_retrying_an_old_snapshot_does_not_reset_capture_freshness(self):
        report = {**healthy(), "last_snapshot_id": "20261108T033000Z-1234abcd"}
        self.assertIn("backup_overdue", health.evaluate(report, NOW))
        report["last_snapshot_id"] = "20261110T040600Z-1234abcd"
        self.assertIn("clock_skew", health.evaluate(report, NOW))

    def test_pending_work_is_allowed_only_while_the_service_runs(self):
        report = {**healthy(), "pending": True, "service_active": "activating"}
        self.assertNotIn("pending_backup", health.evaluate(report, NOW))
        report["service_active"] = "inactive"
        self.assertIn("pending_backup", health.evaluate(report, NOW))

    def test_strict_report_rejects_untrusted_log_content(self):
        for changes in ({"version": True}, {"disk_free_bytes": True}, {"timer_enabled": "yes"}, {"last_snapshot_id": "../private"}, {"errors": ["::error::private-content"]}, {"service_result": "secret-value"}, {"checked_at": "2026-11-10T04:00:00"}, {"extra": "private"}):
            with self.subTest(changes=changes), self.assertRaises((ValueError, TypeError)):
                health.evaluate({**healthy(), **changes}, NOW)

    def test_failed_latest_attempt_keeps_last_success_visible(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); (root / "offsite").mkdir()
            (root / "offsite/status.json").write_text(json.dumps({"state": "failed"}))
            (root / "offsite/last-success.json").write_text(json.dumps({"state": "uploaded", "snapshot_id": healthy()["last_snapshot_id"]}))
            unit = {"UnitFileState": "enabled", "ActiveState": "active", "Result": "success"}
            with patch.object(health, "unit_state", return_value=unit), patch.object(health, "verify_vault", return_value=True):
                report = health.collect(root)
            self.assertEqual(report["last_snapshot_id"], healthy()["last_snapshot_id"])
            self.assertEqual(report["last_attempt"], "failed")
            report["checked_at"] = NOW.isoformat()
            self.assertIn("last_attempt_failed", health.evaluate(report, NOW))

    def test_upgrade_accepts_existing_success_but_not_failed_status(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); (root / "offsite").mkdir()
            unit = {"UnitFileState": "enabled", "ActiveState": "active", "Result": "success"}
            with patch.object(health, "unit_state", return_value=unit), patch.object(health, "verify_vault", return_value=True):
                for state in ("uploaded", "failed"):
                    (root / "offsite/status.json").write_text(json.dumps({"state": state, "snapshot_id": healthy()["last_snapshot_id"]}))
                    report = health.collect(root)
                    self.assertEqual(report["last_snapshot_id"], healthy()["last_snapshot_id"] if state == "uploaded" else None)

    def test_vault_retention_failure_is_reported_without_faking_cipher_damage(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); (root / "offsite").mkdir()
            (root / "offsite/status.json").write_text(json.dumps({"state": "uploaded", "snapshot_id": healthy()["last_snapshot_id"]}))
            def verify(_root, _success, maintenance):
                maintenance["healthy"] = False
                return True
            unit = {"UnitFileState": "enabled", "ActiveState": "active", "Result": "success"}
            with patch.object(health, "unit_state", return_value=unit), patch.object(health, "verify_vault", side_effect=verify):
                report = health.collect(root)
            self.assertTrue(report["vault_verified"])
            self.assertIn("vault_retention_failed", report["errors"])

    def test_action_annotations_are_specific_and_fail_the_job(self):
        with redirect_stdout(io.StringIO()) as output, patch.dict("os.environ", {}, clear=True):
            self.assertEqual(backup_monitor.publish({**healthy(), "vault_verified": False}, NOW), 1)
        self.assertIn("::error title=Offsite backup: vault_unverified::", output.getvalue())
        with redirect_stdout(io.StringIO()), patch.dict("os.environ", {}, clear=True):
            self.assertEqual(backup_monitor.publish(healthy(), NOW), 0)

    def test_monitor_key_is_restricted_and_cannot_inject_another_command(self):
        key = authorized_key("ssh-ed25519 AAAA\ncommand=evil")
        self.assertEqual(key.count("\n"), 1)
        self.assertTrue(key.startswith('restrict,command="/usr/bin/python3 -I /usr/local/libexec/isaber-monitor-ssh" '))
        entry = Path(__file__).resolve().parents[1] / "staging/monitor_entry.py"
        for command in ("id", "deliver", "store", "backup-health --root=/tmp", ""):
            result = subprocess.run([sys.executable, "-I", str(entry)], env={"SSH_ORIGINAL_COMMAND": command}, capture_output=True)
            self.assertNotEqual(result.returncode, 0)


if __name__ == "__main__": unittest.main()
