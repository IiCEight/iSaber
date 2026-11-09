import hashlib
import io
import json
from pathlib import Path
import signal
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch
from contextlib import nullcontext

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "staging"))
import offsite_recover
import offsite_restore
import offsite
import vault
from vault_protocol import canonical, validate_manifest


def manifest(cipher=b"age-encryption.org/v1\nfixture", plain=b"fixture"):
    return {"version": 1, "id": "20261109T033000Z-1234abcd", "cipher_bytes": len(cipher), "cipher_sha256": hashlib.sha256(cipher).hexdigest(), "plain_bytes": len(plain), "plain_sha256": hashlib.sha256(plain).hexdigest(), "backup_sha256": "c" * 64, "source_commit": "d" * 40, "recipient": "age1" + "q" * 58}


class ProtocolTests(unittest.TestCase):
    def test_failed_upload_retries_pending_snapshot_without_pausing_again(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "offsite").mkdir()
            pending = root / "offsite/pending.json"
            pending.write_text(json.dumps({"id": manifest()["id"]}))
            with patch.object(offsite, "lock", return_value=nullcontext()), patch.object(offsite, "snapshot") as snapshot, patch.object(offsite, "prepare", return_value=root / "cipher") as prepare, patch.object(offsite, "transfer", side_effect=RuntimeError("offline")):
                with self.assertRaises(RuntimeError): offsite.daily(root)
                snapshot.assert_not_called()
                prepare.assert_called_once_with(root, root / "backups" / manifest()["id"])
            self.assertTrue(pending.exists())
            with patch.object(offsite, "lock", return_value=nullcontext()), patch.object(offsite, "snapshot") as snapshot, patch.object(offsite, "prepare", return_value=root / "cipher"), patch.object(offsite, "transfer"):
                offsite.daily(root)
                snapshot.assert_not_called()
            self.assertFalse(pending.exists())

    def test_reject_unknown_fields_boolean_sizes_and_invalid_ids(self):
        for changes in ({"unknown": 1}, {"cipher_bytes": True}, {"plain_bytes": 5 * 1024**3}, {"id": "20261309T033000Z-1234abcd"}, {"id": "../outside"}):
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                validate_manifest({**manifest(), **changes})

    def test_unpack_rejects_links_extra_members_and_oversized_headers(self):
        for name, kind, size in (("../outside", tarfile.REGTYPE, 1), ("manifest.json", tarfile.SYMTYPE, 0), ("manifest.json", tarfile.XHDTYPE, 1), ("manifest.json", tarfile.REGTYPE, 4097)):
            info = tarfile.TarInfo(name); info.type = kind; info.size = size
            with tempfile.TemporaryDirectory() as temporary, self.assertRaises(ValueError):
                offsite_recover.unpack(io.BytesIO(info.tobuf(format=tarfile.USTAR_FORMAT)), Path(temporary))

    def test_plain_archive_digest_and_traversal(self):
        for name in ("backup.json", "../outside"):
            output = io.BytesIO()
            with tarfile.open(fileobj=output, mode="w", format=tarfile.USTAR_FORMAT) as archive:
                info = tarfile.TarInfo(name); info.size = 2; archive.addfile(info, io.BytesIO(b"{}"))
            plain = output.getvalue(); record = manifest(plain=plain); record["backup_sha256"] = hashlib.sha256(b"{}").hexdigest()
            with tempfile.TemporaryDirectory() as temporary:
                if name == "backup.json":
                    offsite_restore.extract_plain(io.BytesIO(plain), Path(temporary), record)
                    record["plain_sha256"] = "0" * 64
                    with tempfile.TemporaryDirectory() as other, self.assertRaises(ValueError):
                        offsite_restore.extract_plain(io.BytesIO(plain), Path(other), record)
                else:
                    with self.assertRaises(ValueError): offsite_restore.extract_plain(io.BytesIO(plain), Path(temporary), record)


class VaultTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.namespace = self.root / "jd-staging"; self.namespace.mkdir()
        self.key = self.root / "sender"
        subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(self.key)], check=True, capture_output=True)
        (self.root / "allowed_signers").write_text("isaber-jd-staging " + self.key.with_suffix(".pub").read_text())
        (self.root / "policy.json").write_text(json.dumps({"recipient": manifest()["recipient"]}))
        self.patches = [patch.object(vault, "ROOT", self.root), patch.object(vault, "NAMESPACE", self.namespace), patch.object(vault, "MIN_FREE", 0)]
        for mock in self.patches: mock.start()

    def tearDown(self):
        signal.alarm(0)
        for mock in reversed(self.patches): mock.stop()
        self.temporary.cleanup()

    def header(self, record=None):
        record = record or manifest()
        path = self.root / "manifest-to-sign"
        path.write_bytes(canonical(record)); path.with_suffix(".sig").unlink(missing_ok=True)
        subprocess.run(["ssh-keygen", "-Y", "sign", "-f", str(self.key), "-n", "isaber-backup", str(path)], check=True, capture_output=True)
        return {"operation": "put", "manifest": record, "signature": path.with_suffix(".sig").read_text()}

    def store(self, header, cipher=b"age-encryption.org/v1\nfixture"):
        with tempfile.TemporaryDirectory(prefix=".incoming-", dir=self.namespace) as temporary:
            return vault.store(header, io.BytesIO(cipher), Path(temporary))

    def test_signed_round_trip_retry_and_no_overwrite(self):
        header = self.header()
        self.assertEqual(self.store(header)["status"], "stored")
        self.assertEqual(self.store({**header, "operation": "verify"})["status"], "exists")
        changed = self.header({**manifest(), "source_commit": "e" * 40})
        with self.assertRaises(ValueError): self.store(changed)
        self.assertEqual(json.loads((self.namespace / manifest()["id"] / "manifest.json").read_text()), manifest())

    def test_signature_tamper_missing_and_cipher_tamper(self):
        header = self.header()
        with self.assertRaises(ValueError): self.store({**header, "manifest": {**manifest(), "source_commit": "e" * 40}})
        with self.assertRaises(FileNotFoundError): self.store({**header, "operation": "verify"})
        with self.assertRaises(ValueError): self.store(header, b"age-encryption.org/v1\ntampered")
        self.assertFalse((self.namespace / manifest()["id"]).exists())

    def test_capacity_preserves_existing_objects(self):
        header = self.header()
        with patch.object(vault, "MAX_STORED", 1), self.assertRaises(OSError): self.store(header)
        self.assertFalse((self.namespace / manifest()["id"]).exists())


if __name__ == "__main__": unittest.main()
