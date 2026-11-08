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
import receive_artifact as receiver
import zipfile
import shutil


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

    def use_external_database(self):
        shutil.rmtree(self.root / "images/postgres")
        baseline = {"upstream_reference": "postgres@sha256:" + "b" * 64,
                    "reference": "ccr.ccs.tencentyun.com/isaber/postgres@sha256:" + "c" * 64,
                    "image_id": self.metadata["images"]["postgres"]["image_id"]}
        (self.root / bundle.BASELINE_FILE).write_text(json.dumps(baseline))
        self.metadata["format_version"] = 2
        self.metadata["images"]["postgres"] = {**baseline, "manifest_digest": "sha256:" + "c" * 64}
        self.inventory()

    def test_external_database_is_bound_to_the_inventory_and_manifest(self):
        self.use_external_database()
        bundle.verify(self.root)
        self.metadata["images"]["postgres"]["image_id"] = "sha256:" + "d" * 64
        self.save()
        with self.assertRaisesRegex(ValueError, "pinned"):
            bundle.verify(self.root)

    def test_external_database_rejects_layers_or_another_registry(self):
        self.use_external_database()
        extra = self.root / "images/postgres/layer"
        extra.parent.mkdir()
        extra.write_bytes(b"unexpected")
        self.inventory()
        with self.assertRaises(ValueError):
            bundle.verify(self.root)
        shutil.rmtree(extra.parent)
        path = self.root / bundle.BASELINE_FILE
        baseline = json.loads(path.read_text())
        baseline["reference"] = "other.example/postgres@sha256:" + "c" * 64
        path.write_text(json.dumps(baseline))
        self.inventory()
        with self.assertRaises(ValueError):
            bundle.verify(self.root)

    def test_database_reuse_verifies_remote_content_without_uploading_it(self):
        self.use_external_database()
        config = json.dumps({"os": "linux", "architecture": "amd64"}).encode()
        with tempfile.TemporaryDirectory() as directory:
            auth = Path(directory) / "auth.json"
            auth.touch(mode=0o600)
            output = Path(directory) / "release.json"
            with patch.object(bundle, "run", return_value=config) as invoke:
                bundle.publish(self.root, output, "a" * 40, auth)
            copies = [call.args[0] for call in invoke.call_args_list if call.args[0][1] == "copy"]
            self.assertEqual(len(copies), 2)
            self.assertTrue(all("postgres" not in " ".join(call) for call in copies))
            self.assertEqual(json.loads(output.read_text())["images"]["postgres"]["reference"], self.metadata["images"]["postgres"]["reference"])
            output.unlink()
            def wrong_database(arguments):
                return b"wrong-config" if "postgres@" in " ".join(arguments) else config
            with patch.object(bundle, "run", side_effect=wrong_database), self.assertRaises(ValueError):
                bundle.publish(self.root, output, "a" * 40, auth)
            self.assertFalse(output.exists())

    def test_compose_and_export_use_the_same_locked_upstream(self):
        root = Path(__file__).resolve().parents[2]
        baseline = bundle.postgres_baseline(root)
        self.assertTrue(bundle.SOURCES["postgres"].endswith("@" + baseline["upstream_reference"].split("@", 1)[1]))
        self.assertIn("image: " + bundle.SOURCES["postgres"], (root / "infra/compose.rehearsal.yml").read_text())


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

    def test_server_download_requires_matching_ci_before_getting_a_url(self):
        run = {**self.run, "run_attempt": 1}
        artifact = {"name": "bundle-" + "a" * 40 + "-1", "expired": False}
        metadata = {"name": "metadata-" + "a" * 40 + "-1", "expired": False}
        header = {"format_version": 2, "kind": "isaber-image-bundle", "source_commit": "a" * 40, "ci_run_id": "1", "bundle_run_id": "2", "bundle_run_attempt": "1", "platform": "linux/amd64", "images": {name: {} for name in bundle.SOURCES}}
        for conclusion in ("success", "failure"):
            with self.subTest(conclusion=conclusion), patch.object(relay, "github", side_effect=[run, {"artifacts": [artifact, metadata]}, {"jobs": self.jobs}, {**self.ci, "conclusion": conclusion}]), patch.object(relay, "command"), patch.object(relay, "parse_json", return_value=header), patch.object(relay, "relay_direct") as direct:
                if conclusion == "failure":
                    with self.assertRaises(ValueError):
                        relay.relay(2, "server")
                    direct.assert_not_called()
                else:
                    relay.relay(2, "server")
                    direct.assert_called_once_with(artifact, header, 2, 1, "a" * 40)

    def test_server_download_rejects_metadata_from_another_attempt(self):
        artifact = {"name": "bundle-" + "a" * 40 + "-1"}
        metadata = {"name": "metadata-" + "a" * 40 + "-1"}
        header = {"format_version": 2, "kind": "isaber-image-bundle", "source_commit": "a" * 40, "ci_run_id": "1", "bundle_run_id": "2", "bundle_run_attempt": "2", "platform": "linux/amd64", "images": {name: {} for name in bundle.SOURCES}}
        with patch.object(relay, "github", side_effect=[{**self.run, "run_attempt": 1}, {"artifacts": [artifact, metadata]}]), patch.object(relay, "command"), patch.object(relay, "parse_json", return_value=header), patch.object(relay, "relay_direct") as direct:
            with self.assertRaisesRegex(ValueError, "attempt"):
                relay.relay(2, "server")
            direct.assert_not_called()


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


class ArtifactReceiptTests(unittest.TestCase):
    def test_signed_url_rejects_credentials_http_and_other_hosts(self):
        for url in ["http://test.blob.core.windows.net/a", "https://github.com/a", "https://x.blob.core.windows.net.evil.example/a", "https://u:p@x.blob.core.windows.net/a", "https://x.blob.core.windows.net:444/a"]:
            with self.subTest(url=url), self.assertRaises(ValueError):
                receiver.validate_url(url)
        receiver.validate_url("https://x.blob.core.windows.net/a?signature=fixture")

    def test_zip_traversal_duplicates_and_symlinks_fail_before_writes(self):
        for names in [["../escape"], ["/absolute"], ["a\\b"], ["duplicate", "duplicate"], ["symlink"]]:
            with self.subTest(names=names), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                with zipfile.ZipFile(root / "artifact.zip", "w") as archive:
                    for name in names:
                        entry = zipfile.ZipInfo(name)
                        if name == "symlink":
                            entry.external_attr = 0o120777 << 16
                        archive.writestr(entry, b"fixture")
                with self.assertRaises(ValueError):
                    receiver.extract(root / "artifact.zip", root / "content")
                self.assertFalse((root / "content").exists())

    def test_archive_receipt_rejects_corruption_before_extracting(self):
        payload = {"url": "https://x.blob.core.windows.net/fixture", "digest": "sha256:" + "0" * 64, "size_in_bytes": 7}
        with tempfile.TemporaryDirectory() as directory, patch("urllib.request.build_opener") as opener:
            opener.return_value.open.return_value = io.BytesIO(b"changed")
            with self.assertRaisesRegex(ValueError, "checksum"):
                receiver.receive(directory, payload)
            self.assertFalse((Path(directory) / "content").exists())

    def test_complete_archive_receipt_and_metadata_match(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            content = io.BytesIO()
            metadata = {"source_commit": "a" * 40}
            with zipfile.ZipFile(content, "w") as archive:
                archive.writestr("bundle.json", json.dumps(metadata))
            data = content.getvalue()
            payload = {"url": "https://x.blob.core.windows.net/fixture", "digest": "sha256:" + hashlib.sha256(data).hexdigest(), "size_in_bytes": len(data), "bundle": metadata}
            with patch("urllib.request.build_opener") as opener:
                opener.return_value.open.return_value = io.BytesIO(data)
                receiver.receive(root, payload)
            self.assertEqual(json.loads((root / "content/bundle.json").read_text()), metadata)
            self.assertEqual((root / "bundle.zip").stat().st_mode & 0o777, 0o600)


if __name__ == "__main__":
    unittest.main()
