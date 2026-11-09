import base64
import importlib.util
import json
from pathlib import Path
import tempfile
from contextlib import nullcontext
import unittest
from unittest.mock import patch, Mock

spec = importlib.util.spec_from_file_location("staging_tools", Path(__file__).resolve().parents[1] / "staging/staging.py")
staging = importlib.util.module_from_spec(spec)
spec.loader.exec_module(staging)


class StagingTests(unittest.TestCase):
    def release(self):
        return {"format_version": 1, "source_commit": "a" * 40, "platform": "linux/amd64", "migration_version": 5,
                "contract_sha256": "b" * 64, "images": {name: {"reference": "ccr.ccs.tencentyun.com/isaber/" + name + "@sha256:" + "c" * 64, "image_id": "sha256:" + "d" * 64,
                **({"upstream_reference": "postgres@sha256:" + "e" * 64} if name == "postgres" else {})} for name in ("frontend", "backend", "postgres")}}

    def test_only_reviewed_schema_platform_and_repository_digests_are_accepted(self):
        staging.validate_release(self.release())
        for key, value in [("platform", "linux/arm64"), ("migration_version", 6), ("source_commit", "main")]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                staging.validate_release({**self.release(), key: value})
        for reference in ("ccr.ccs.tencentyun.com/isaber/frontend:latest", "other.example/frontend@sha256:" + "c" * 64):
            release = self.release()
            release["images"]["frontend"]["reference"] = reference
            with self.assertRaises(ValueError):
                staging.validate_release(release)

    def test_secrets_are_private_generated_once_and_not_regenerated(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with patch.object(staging.subprocess, "run", return_value=Mock(returncode=1)), patch.object(staging, "run", return_value=b"29.1.3"):
                staging.initialize(root)
            first = (root / "secrets.env").read_bytes()
            values = staging.credentials(root)
            self.assertEqual(len(base64.b64decode(values["CSRF_SECRET"])), 32)
            self.assertNotEqual(values["POSTGRES_PASSWORD"], values["RUNTIME_PASSWORD"])
            self.assertEqual((root / "secrets.env").stat().st_mode & 0o777, 0o600)
            with patch.object(staging, "run") as command:
                staging.initialize(root)
            command.assert_not_called()
            self.assertEqual((root / "secrets.env").read_bytes(), first)

    def test_existing_database_without_secrets_cannot_get_new_credentials(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(staging.subprocess, "run", return_value=Mock(returncode=0)):
            with self.assertRaises(ValueError):
                staging.initialize(Path(directory))
            self.assertFalse((Path(directory) / "secrets.env").exists())

    def test_data_bearing_staging_cannot_be_overwritten_by_initial_deployer(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            staging.write_private(root / "secrets.env", "RUNTIME_PASSWORD=" + "a" * 64 + "\n")
            staging.write_private(root / "current-release.json", json.dumps(self.release()))
            staging.write_private(root / "candidate.json", json.dumps(self.release()))
            with patch.object(staging, "run", return_value=b"sha256:" + b"a" * 64), patch.object(staging, "image_config_id", return_value=json.loads((staging.CONFIG / "proxy-image.json").read_text())["image_id"]), patch.object(staging, "compose") as command:
                with self.assertRaisesRegex(ValueError, "backup"):
                    staging.deploy(root, root / "candidate.json")
            command.assert_not_called()
            self.assertFalse((root / "release.env").exists())


class PromotionTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.current = StagingTests().release()
        self.candidate = {**self.current, "source_commit": "f" * 40}
        self.environment = "CADDY_IMAGE=sha256:" + "e" * 64 + "\n"
        (self.root / "current-release.json").write_text(json.dumps(self.current))
        (self.root / "candidate.json").write_text(json.dumps(self.candidate))
        (self.root / "release.env").write_text(self.environment)
        import backup
        for target, value in [("lock", nullcontext()), ("snapshot", self.root / "backups/fixture"), ("restore_check", None)]:
            mock = patch.object(backup, target, return_value=value)
            setattr(self, target, mock.start())
            self.addCleanup(mock.stop)
        for target, value in [("sql", b"5"), ("run", b""), ("image_config_id", "sha256:" + "d" * 64), ("verify_https", None)]:
            mock = patch.object(staging, target, return_value=value)
            setattr(self, target, mock.start())
            self.addCleanup(mock.stop)
        mock = patch.object(staging, "compose", side_effect=lambda root, *arguments: b"1 applied\n2 applied\n3 applied\n4 applied\n5 applied\n" if arguments[0] == "run" else b"")
        self.compose = mock.start()
        self.addCleanup(mock.stop)

    def test_promotion_requires_a_verified_snapshot_then_records_previous_release(self):
        staging.promote(self.root, self.root / "candidate.json")
        self.snapshot.assert_called_once_with(self.root)
        self.restore_check.assert_called_once_with(self.root, self.root / "backups/fixture")
        self.assertEqual(json.loads((self.root / "current-release.json").read_text()), self.candidate)
        self.assertEqual(json.loads((self.root / "previous-release.json").read_text()), self.current)
        self.assertFalse((self.root / "pending-promotion.json").exists())
        self.assertTrue(all("up" not in call.args[1:] for call in self.compose.call_args_list if call.args[1] == "run"))

    def test_failed_candidate_restores_previous_images_and_metadata(self):
        self.verify_https.side_effect = [RuntimeError("fixture failed health gate"), None]
        with self.assertRaisesRegex(RuntimeError, "previous release recovered"):
            staging.promote(self.root, self.root / "candidate.json")
        self.assertEqual((self.root / "release.env").read_text(), self.environment)
        self.assertEqual(json.loads((self.root / "current-release.json").read_text()), self.current)
        self.assertEqual(json.loads((self.root / "last-promotion.json").read_text())["outcome"], "recovered_previous")
        self.assertFalse((self.root / "pending-promotion.json").exists())

    def test_failed_restore_check_prevents_any_candidate_environment_or_service_change(self):
        self.restore_check.side_effect = RuntimeError("fixture recovery failure")
        with self.assertRaises(RuntimeError):
            staging.promote(self.root, self.root / "candidate.json")
        self.compose.assert_not_called()
        self.assertEqual((self.root / "release.env").read_text(), self.environment)
        self.assertFalse((self.root / "pending-promotion.json").exists())

    def test_final_source_gate_prevents_a_stale_switch_after_backup(self):
        gate = Mock(side_effect=RuntimeError("a newer main commit exists"))
        with self.assertRaises(RuntimeError):
            staging.promote(self.root, self.root / "candidate.json", before_switch=gate)
        gate.assert_called_once_with()
        self.compose.assert_not_called()
        self.assertFalse((self.root / "pending-promotion.json").exists())
        self.assertEqual((self.root / "release.env").read_text(), self.environment)

    def test_contract_database_and_live_schema_changes_are_rejected(self):
        for field in ("contract", "database", "schema"):
            candidate = json.loads(json.dumps(self.candidate))
            if field == "contract": candidate["contract_sha256"] = "a" * 64
            if field == "database": candidate["images"]["postgres"]["reference"] = "ccr.ccs.tencentyun.com/isaber/postgres@sha256:" + "b" * 64
            if field == "schema": self.sql.return_value = b"6"
            (self.root / "candidate.json").write_text(json.dumps(candidate))
            with self.subTest(field=field), self.assertRaises(ValueError):
                staging.promote(self.root, self.root / "candidate.json")
        self.snapshot.assert_not_called()
        self.compose.assert_not_called()

    def test_pending_journal_blocks_another_promotion(self):
        (self.root / "pending-promotion.json").write_text("{}")
        with self.assertRaisesRegex(ValueError, "Recover"):
            staging.promote(self.root, self.root / "candidate.json")
        self.run.assert_not_called()
        self.snapshot.assert_not_called()

    def test_an_unready_candidate_migration_is_never_applied(self):
        self.compose.side_effect = lambda root, *arguments: b"1 applied\n2 applied\n3 applied\n4 applied\n5 applied\n6 pending\n" if arguments[0] == "run" else b""
        with self.assertRaises(RuntimeError):
            staging.promote(self.root, self.root / "candidate.json")
        self.assertEqual(json.loads((self.root / "current-release.json").read_text()), self.current)
        self.assertTrue(all("up" not in call.args[1:] for call in self.compose.call_args_list if call.args[1] == "run"))

    def test_interrupted_promotion_restores_release_and_earlier_history(self):
        history = {**self.current, "source_commit": "1" * 40}
        journal = {"previous_release": self.current, "candidate_release": self.candidate, "previous_release_history": history,
                   "previous_image_environment": self.environment, "snapshot": str(self.root / "backups/fixture")}
        (self.root / "pending-promotion.json").write_text(json.dumps(journal))
        (self.root / "current-release.json").write_text(json.dumps(self.candidate))
        (self.root / "previous-release.json").write_text(json.dumps(self.current))
        staging.recover_promotion(self.root)
        self.assertEqual(json.loads((self.root / "current-release.json").read_text()), self.current)
        self.assertEqual(json.loads((self.root / "previous-release.json").read_text()), history)
        self.assertEqual((self.root / "release.env").read_text(), self.environment)
        self.assertFalse((self.root / "pending-promotion.json").exists())
