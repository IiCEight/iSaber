import hashlib
import io
import json
from pathlib import Path
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "staging"))
import backup


def release():
    return {"format_version": 1, "source_commit": "a" * 40, "platform": "linux/amd64", "migration_version": 5, "contract_sha256": "b" * 64,
            "images": {name: {"reference": "ccr.ccs.tencentyun.com/isaber/" + name + "@sha256:" + "c" * 64, "image_id": "sha256:" + "d" * 64,
            **({"upstream_reference": "postgres@sha256:" + "e" * 64} if name == "postgres" else {})} for name in ("frontend", "backend", "postgres")}}


class BackupTests(unittest.TestCase):
    def archive(self, path, name="./fixture", kind=tarfile.REGTYPE):
        with tarfile.open(path, "w") as output:
            member = tarfile.TarInfo(name)
            member.type = kind
            if kind == tarfile.REGTYPE:
                member.size = 7
                output.addfile(member, io.BytesIO(b"fixture"))
            else:
                member.linkname = "outside"
                output.addfile(member)

    def fixture(self, root):
        directory = root / "backups/fixture"
        directory.mkdir(parents=True)
        (directory / "database.pgdump").write_bytes(b"database")
        self.archive(directory / "uploads.tar")
        self.archive(directory / "certificates.tar")
        metadata = {"format_version": 1, "release": release(), "row_counts": {}, "table_hashes": {}, "uploads": backup.tar_inventory(directory / "uploads.tar"),
                    "files": {path.name: {"size": path.stat().st_size, "sha256": backup.checksum(path)} for path in directory.iterdir()}}
        (directory / "backup.json").write_text(json.dumps(metadata))
        return directory

    def test_snapshot_integrity_is_verified_before_creating_restore_resources(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            snapshot = self.fixture(root)
            backup.verify_snapshot(root, snapshot)
            (snapshot / "database.pgdump").write_bytes(b"modified")
            with patch.object(backup, "sql") as sql, patch.object(backup, "run") as command:
                with self.assertRaisesRegex(ValueError, "integrity"):
                    backup.restore_check(root, snapshot)
            sql.assert_not_called()
            command.assert_not_called()

    def test_restore_refuses_a_directory_outside_the_backup_tree(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "elsewhere").mkdir()
            with self.assertRaises(ValueError):
                backup.verify_snapshot(root, root / "elsewhere")

    def test_archive_inventory_rejects_links_and_traversal(self):
        for name, kind in [("../escape", tarfile.REGTYPE), ("/absolute", tarfile.REGTYPE), ("link", tarfile.SYMTYPE), ("hardlink", tarfile.LNKTYPE)]:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                path = Path(directory) / "fixture.tar"
                self.archive(path, name, kind)
                with self.assertRaises(ValueError):
                    backup.tar_inventory(path)

    def test_archive_records_actual_media_hashes(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "fixture.tar"
            self.archive(path)
            self.assertEqual(backup.tar_inventory(path), {"fixture": {"size": 7, "sha256": hashlib.sha256(b"fixture").hexdigest()}})

    def test_failed_snapshot_resumes_only_previously_running_blog_services(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "current-release.json").write_text(json.dumps(release()))
            def compose(_root, *arguments):
                return b"postgres\napi\nfrontend\nedge\n" if arguments[0] == "ps" else b""
            with patch.object(backup, "compose", side_effect=compose) as command, patch.object(backup, "row_counts", return_value={}), patch.object(backup, "table_hashes", return_value={}), patch.object(backup, "stream", side_effect=RuntimeError("fixture failure")), patch.object(backup, "verify_https") as https:
                with self.assertRaises(RuntimeError):
                    backup.snapshot(root)
            calls = [call.args[1:] for call in command.call_args_list]
            self.assertIn(("stop", "edge", "frontend", "api"), calls)
            self.assertIn(("up", "-d", "--wait", "api", "frontend", "edge"), calls)
            https.assert_called_once_with(root)
            self.assertEqual(list((root / "backups").iterdir()), [])

    def test_volume_archive_has_no_network_and_mounts_live_data_readonly(self):
        arguments = backup.volume_command("fixture-image", "isaber-staging_uploads", "/source", "tar", "-cf", "-", ".")
        self.assertIn("--network=none", arguments)
        self.assertIn("--read-only", arguments)
        self.assertIn("--pull=never", arguments)
        self.assertIn("-i", arguments)
        self.assertIn("type=volume,source=isaber-staging_uploads,target=/source,readonly", arguments)

    def test_failed_restore_drops_only_its_new_disposable_database(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            snapshot = self.fixture(root)
            metadata = {"release": release(), "row_counts": {}, "table_hashes": {}, "uploads": {}}
            with patch.object(backup, "verify_snapshot", return_value=metadata), patch.object(backup, "sql") as sql, patch.object(backup, "stream", side_effect=RuntimeError("fixture restore failure")):
                with self.assertRaises(RuntimeError):
                    backup.restore_check(root, snapshot)
            statements = [call.args[1] for call in sql.call_args_list]
            self.assertEqual(len(statements), 2)
            self.assertRegex(statements[0], r'^CREATE DATABASE "isaber_restore_[0-9a-f]{16}";$')
            name = statements[0].split('"')[1]
            self.assertEqual(statements[1], f'DROP DATABASE "{name}" WITH (FORCE);')
            self.assertNotEqual(name, "isaber_staging")

    def test_concurrent_backup_operations_are_refused(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(backup, "credentials", return_value={}):
            root = Path(directory)
            with backup.lock(root):
                with self.assertRaises(BlockingIOError):
                    with backup.lock(root):
                        self.fail("A second snapshot operation acquired the same lock")
