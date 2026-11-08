import base64
import importlib.util
import json
from pathlib import Path
import tempfile
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
