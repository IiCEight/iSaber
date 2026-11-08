import hashlib
import json
from pathlib import Path
import sys
import tempfile
import tarfile
import io
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import release_bundle as bundle
import relay_release as relay


class BundleTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        for name in bundle.METADATA_FILES:
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("const RequiredVersion int64 = 5" if name.endswith("migrations.go") else "fixture")
        self.metadata = {
            "format_version": 1, "kind": "isaber-image-bundle", "source_commit": "a" * 40,
            "ci_run_id": "1", "bundle_run_id": "2", "bundle_run_attempt": "1",
            "platform": "linux/amd64", "migration_version": 5,
            "contract_sha256": bundle.digest(self.root / "contracts/openapi.yaml"), "images": {},
        }
        for name in bundle.SOURCES:
            directory = self.root / "images" / name
            directory.mkdir(parents=True)
            config = json.dumps({"os": "linux", "architecture": "amd64"}).encode()
            config_hash = hashlib.sha256(config).hexdigest()
            layer = b"fixture-layer"
            layer_hash = hashlib.sha256(layer).hexdigest()
            (directory / config_hash).write_bytes(config)
            (directory / layer_hash).write_bytes(layer)
            manifest = {"schemaVersion": 2, "config": {"digest": "sha256:" + config_hash}, "layers": [{"digest": "sha256:" + layer_hash}]}
            (directory / "manifest.json").write_text(json.dumps(manifest))
            self.metadata["images"][name] = {"image_id": "sha256:" + config_hash, "manifest_digest": "sha256:" + bundle.digest(directory / "manifest.json")}
        self.metadata["images"]["postgres"]["upstream_reference"] = "postgres@sha256:" + "b" * 64
        self.inventory()

    def inventory(self):
        self.metadata["files"] = {path.relative_to(self.root).as_posix(): {"size": path.stat().st_size, "sha256": bundle.digest(path)} for path in self.root.rglob("*") if path.is_file() and path.name != "bundle.json"}
        self.save()

    def save(self):
        (self.root / "bundle.json").write_text(json.dumps(self.metadata))

    def test_complete_bundle_is_verified(self):
        result = bundle.verify(self.root, "a" * 40)
        self.assertEqual(result["images"], self.metadata["images"])

    def test_tampered_file_is_rejected_before_any_publish_command(self):
        (self.root / "infra/publish_images.py").write_text("changed")
        with patch.object(bundle, "run") as execute:
            with self.assertRaises(ValueError):
                bundle.publish(self.root, self.root / "output.json", "a" * 40, "unused-auth")
        execute.assert_not_called()

    def test_extra_file_and_symlink_are_rejected(self):
        extra = self.root / "unexpected"
        extra.write_text("unlisted")
        with self.assertRaises(ValueError):
            bundle.verify(self.root)
        extra.unlink()
        source = self.root / "infra/publish_images.py"
        source.unlink()
        source.symlink_to(self.root / "contracts/openapi.yaml")
        with self.assertRaises(ValueError):
            bundle.verify(self.root)

    def test_layer_cannot_be_changed_by_rewriting_only_the_inventory(self):
        path = self.root / "images/frontend" / hashlib.sha256(b"fixture-layer").hexdigest()
        path.write_bytes(b"different-layer")
        self.inventory()
        with self.assertRaises(ValueError):
            bundle.verify(self.root)

    def test_commit_and_platform_are_bound(self):
        with self.assertRaises(ValueError):
            bundle.verify(self.root, "f" * 40)
        self.metadata["platform"] = "linux/arm64"
        self.save()
        with self.assertRaises(ValueError):
            bundle.verify(self.root)

    def test_missing_database_provenance_is_rejected(self):
        del self.metadata["images"]["postgres"]["upstream_reference"]
        self.save()
        with self.assertRaises(ValueError):
            bundle.verify(self.root)


class TrustedRunTests(unittest.TestCase):
    def setUp(self):
        self.run = {"path": ".github/workflows/publish.yml", "event": "workflow_run", "head_branch": "main", "head_sha": "a" * 40, "conclusion": "success", "head_repository": {"full_name": "IiCEight/iSaber"}}
        self.jobs = [{"name": "Prepare tested image bundle", "conclusion": "success"}]
        self.ci = {**self.run, "path": ".github/workflows/ci.yml", "event": "push"}

    def test_trusted_main_with_matching_successful_ci_is_allowed(self):
        relay.check_run(self.run, self.jobs, self.ci)

    def test_fork_and_pr_and_wrong_commit_are_rejected(self):
        for changed in [{"head_repository": {"full_name": "another/repo"}}, {"event": "pull_request"}]:
            with self.subTest(changed=changed), self.assertRaises(ValueError):
                relay.check_run({**self.run, **changed}, self.jobs, self.ci)
        with self.assertRaises(ValueError):
            relay.check_run(self.run, self.jobs, {**self.ci, "head_sha": "f" * 40})

    def test_failed_job_or_failed_ci_is_rejected(self):
        with self.assertRaises(ValueError):
            relay.check_run(self.run, [{"name": "Prepare tested image bundle", "conclusion": "failure"}], self.ci)
        with self.assertRaises(ValueError):
            relay.check_run(self.run, self.jobs, {**self.ci, "conclusion": "failure"})

    def test_default_branch_advancing_does_not_change_bundle_source(self):
        relay.check_run({**self.run, "head_sha": "f" * 40}, self.jobs, self.ci, "a" * 40)
        with self.assertRaises(ValueError):
            relay.check_run(self.run, self.jobs, self.ci, "f" * 40)


class ExtractionTests(unittest.TestCase):
    def test_remote_bootstrap_rejects_unsafe_entries_before_running_tools(self):
        for name, kind in [("../outside", tarfile.REGTYPE), ("bad\\name", tarfile.REGTYPE), ("link", tarfile.SYMTYPE), ("/absolute", tarfile.REGTYPE)]:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                with tarfile.open(root / "bundle.tar", "w") as archive:
                    member = tarfile.TarInfo(name)
                    member.type = kind
                    if kind == tarfile.SYMTYPE:
                        member.linkname = "../outside"
                    archive.addfile(member, io.BytesIO(b""))
                arguments = ["bootstrap", str(root), bundle.digest(root / "bundle.tar"), "a" * 40]
                with patch.object(sys, "argv", arguments), patch("shutil.which", return_value="fixture-skopeo"), patch("subprocess.run") as execute:
                    with self.assertRaises(SystemExit):
                        exec(compile(relay.BOOTSTRAP, "relay-bootstrap", "exec"), {})
                execute.assert_not_called()

    def test_transfer_checksum_mismatch_stops_before_extraction(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "bundle.tar").write_bytes(b"changed-transfer")
            with patch.object(sys, "argv", ["bootstrap", str(root), "0" * 64, "a" * 40]), patch("subprocess.run") as execute:
                with self.assertRaises(SystemExit):
                    exec(compile(relay.BOOTSTRAP, "relay-bootstrap", "exec"), {})
            execute.assert_not_called()
            self.assertFalse((root / "content").exists())


if __name__ == "__main__":
    unittest.main()
