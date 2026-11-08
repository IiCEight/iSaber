import contextlib
import importlib.util
import io
import hashlib
import json
from pathlib import Path
import tempfile
import tarfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("publish_images", Path(__file__).resolve().parents[1] / "publish_images.py")
publisher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publisher)


class PublisherTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.output = Path(self.directory.name) / "release.json"
        self.environment = {"RELEASE_SHA": "a" * 40, "TCR_USERNAME": "fixture-user", "TCR_PASSWORD": "fixture-secret", "VERIFIED_CI_RUN_ID": "123"}
        self.calls = []
        self.upstream = "postgres@sha256:" + "b" * 64
        self.fail_push = False
        self.wrong_repository = False
        self.changed_identity = False
        self.manifest_index = False

    def invoke(self, arguments, *, environment, password=None):
        self.calls.append((arguments, dict(environment), password))
        self.assertNotIn("TCR_PASSWORD", environment)
        self.assertNotIn("TCR_USERNAME", environment)
        self.assertNotIn("fixture-secret", " ".join(arguments))
        self.assertEqual(Path(environment["DOCKER_CONFIG"]).stat().st_mode & 0o777, 0o700)
        if arguments[0] == "login":
            self.assertEqual(password, "fixture-secret")
        else:
            self.assertIsNone(password)
        if arguments[:2] == ["image", "save"]:
            self.assertIn("--platform", arguments)
            self.assertEqual(arguments[arguments.index("--platform") + 1], "linux/amd64")
            config = b'{"architecture":"amd64","rootfs":{"type":"layers","diff_ids":[]}}'
            if self.changed_identity and arguments[2].startswith("ccr."):
                config = b'{"architecture":"amd64","config":{"Cmd":["changed"]}}'
            config_path = "blobs/sha256/fixture" if arguments[2].startswith("ccr.") else "config.json"
            manifest = json.dumps([{"Config": config_path}]).encode()
            with tarfile.open(arguments[arguments.index("--output") + 1], "w") as archive:
                for name, body in [("manifest.json", manifest), (config_path, config)]:
                    member = tarfile.TarInfo(name)
                    member.size = len(body)
                    archive.addfile(member, io.BytesIO(body))
            return ""
        if arguments[:2] == ["image", "inspect"]:
            image = arguments[2]
            if image == "postgres:16":
                return json.dumps([self.upstream])
            repository = image.rsplit(":", 1)[0]
            if self.wrong_repository:
                repository = "unrelated.example/other/image"
            return json.dumps([repository + "@sha256:" + "d" * 64])
        if arguments[:2] == ["manifest", "inspect"]:
            if self.manifest_index and arguments[2].endswith("d" * 64):
                return json.dumps({"manifests": [{"digest": "sha256:" + "f" * 64, "platform": {"os": "linux", "architecture": "amd64"}}, {"digest": "sha256:" + "e" * 64, "platform": {"os": "unknown", "architecture": "unknown"}}]})
            config = b'{"architecture":"amd64","rootfs":{"type":"layers","diff_ids":[]}}'
            identity = "sha256:" + ("e" * 64 if self.changed_identity else hashlib.sha256(config).hexdigest())
            return json.dumps({"config": {"digest": identity}})
        if arguments[0] == "push" and self.fail_push:
            raise RuntimeError("Docker push failed")
        if arguments[0] == "push":
            tag = "unrelated-tag" if self.wrong_repository else arguments[1].rsplit(":", 1)[1]
            return tag + ": digest: sha256:" + "d" * 64 + " size: 1234\n"
        return ""

    def publish(self):
        with contextlib.redirect_stdout(io.StringIO()):
            return publisher.publish(self.output, self.environment, invoke=self.invoke)

    def test_success_records_all_three_bound_digests_and_cleans_credentials(self):
        result = self.publish()
        self.assertEqual(json.loads(self.output.read_text()), result)
        self.assertEqual(set(result["images"]), {"frontend", "backend", "postgres"})
        self.assertEqual(result["images"]["postgres"]["upstream_reference"], self.upstream)
        self.assertEqual(result["source_commit"], "a" * 40)
        self.assertEqual(len(result["contract_sha256"]), 64)
        for name, image in result["images"].items():
            self.assertTrue(image["reference"].startswith(f"ccr.ccs.tencentyun.com/isaber/{name}@sha256:"))
        self.assertFalse(Path(self.calls[0][1]["DOCKER_CONFIG"]).exists())

    def test_missing_secret_or_bad_commit_never_calls_docker(self):
        for key, value in [("TCR_PASSWORD", ""), ("RELEASE_SHA", "main")]:
            with self.subTest(key=key):
                environment = {**self.environment, key: value}
                with self.assertRaises(ValueError):
                    publisher.publish(self.output, environment, invoke=self.invoke)
                self.assertFalse(self.calls)
                self.assertFalse(self.output.exists())

    def test_push_failure_does_not_produce_a_completed_manifest(self):
        self.fail_push = True
        with self.assertRaises(RuntimeError):
            self.publish()
        self.assertFalse(self.output.exists())
        self.assertFalse(Path(self.calls[0][1]["DOCKER_CONFIG"]).exists())

    def test_unrelated_push_receipt_cannot_enter_manifest(self):
        self.wrong_repository = True
        with self.assertRaises(ValueError):
            self.publish()
        self.assertFalse(self.output.exists())

    def test_index_resolves_tested_platform_and_records_root_digest(self):
        self.manifest_index = True
        result = self.publish()
        self.assertTrue(result["images"]["frontend"]["reference"].endswith("d" * 64))
        self.assertTrue(any(call[0][:2] == ["manifest", "inspect"] and call[0][2].endswith("f" * 64) for call in self.calls))

    def test_unverified_postgres_source_is_rejected_before_login(self):
        self.upstream = "unrelated.example/postgres@sha256:" + "b" * 64
        with self.assertRaises(ValueError):
            self.publish()
        self.assertFalse(any(call[0][0] == "login" for call in self.calls))
        self.assertFalse(self.output.exists())

    def test_changed_image_identity_cannot_be_reported_as_tested(self):
        self.changed_identity = True
        with self.assertRaises(ValueError):
            self.publish()
        self.assertFalse(self.output.exists())

    def test_timeout_does_not_expose_partial_subprocess_output(self):
        error = publisher.subprocess.TimeoutExpired(["docker", "push"], 1200, output="fixture-secret", stderr="fixture-secret")
        with patch.object(publisher.subprocess, "run", side_effect=error) as command:
            with self.assertRaisesRegex(RuntimeError, "exceeded 1200s") as result:
                publisher.docker(["push", "fixture-image"], environment={})
        self.assertNotIn("fixture-secret", str(result.exception))
        self.assertEqual(command.call_args.kwargs["timeout"], 1200)

    def test_safe_error_classification_does_not_echo_credentials(self):
        failure = publisher.subprocess.CompletedProcess([], 1, stdout="", stderr="unauthorized: fixture-secret")
        with patch.object(publisher.subprocess, "run", return_value=failure) as command:
            with self.assertRaisesRegex(RuntimeError, "authorization rejected") as result:
                publisher.docker(["login", "fixture-registry"], environment={}, password="fixture-secret")
        self.assertNotIn("fixture-secret", str(result.exception))
        self.assertEqual(command.call_args.kwargs["timeout"], 60)


if __name__ == "__main__":
    unittest.main()
