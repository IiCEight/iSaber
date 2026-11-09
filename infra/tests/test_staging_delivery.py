import hashlib
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import Mock, patch
import zipfile

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "staging"))
import gateway
from install_gateway import authorized_key
import test_release_bundle as bundle_fixtures


class DeliveryTests(unittest.TestCase):
    def setUp(self):
        self.fixture = bundle_fixtures.BundleTests()
        self.fixture.setUp()
        self.addCleanup(self.fixture.doCleanups)
        self.fixture.use_external_database()
        self.bundle = self.fixture.metadata
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        (self.root / "delivery").mkdir()
        self.incoming = self.root / "incoming"
        self.incoming.mkdir()
        self.expected = gateway.expected_release(self.bundle)
        self.current = {**self.expected, "source_commit": "b" * 40}
        (self.root / "current-release.json").write_text(json.dumps(self.current))
        content = io.BytesIO()
        with zipfile.ZipFile(content, "w") as archive:
            for path in self.fixture.root.rglob("*"):
                if path.is_file(): archive.write(path, path.relative_to(self.fixture.root).as_posix())
        self.data = content.getvalue()
        self.run = {"path": ".github/workflows/publish.yml", "event": "workflow_run", "head_branch": "main", "head_sha": "f" * 40, "conclusion": "success", "run_attempt": 1, "head_repository": {"full_name": "IiCEight/iSaber"}}
        self.artifact = {"name": "bundle-" + "a" * 40 + "-1", "size_in_bytes": len(self.data), "digest": "sha256:" + hashlib.sha256(self.data).hexdigest(), "expired": False}
        self.responses = {
            "actions/runs/2": self.run,
            "actions/runs/2/artifacts?per_page=100": {"artifacts": [self.artifact]},
            "git/ref/heads/main": {"object": {"sha": "a" * 40}},
            "actions/runs/2/attempts/1/jobs?per_page=100": {"jobs": [{"name": "Prepare tested image bundle", "conclusion": "success"}]},
            "actions/runs/1": {**self.run, "path": ".github/workflows/ci.yml", "event": "push", "head_sha": "a" * 40},
        }
        self.api = Mock()
        self.api.get.side_effect = lambda path: self.responses[path]
        for name, value in [("GitHub", self.api), ("ROOT", self.root)]:
            patcher = patch.object(gateway, name, return_value=value) if name == "GitHub" else patch.object(gateway, name, value)
            patcher.start()
            self.addCleanup(patcher.stop)
        for name in ("alarm", "signal"):
            patcher = patch.object(gateway.signal, name)
            patcher.start()
            self.addCleanup(patcher.stop)
        patcher = patch.object(gateway, "publish", side_effect=lambda directory, output, commit, auth: output.write_text(json.dumps(self.expected)))
        self.publish = patcher.start(); self.addCleanup(patcher.stop)
        patcher = patch.object(gateway, "promote")
        self.promote = patcher.start(); self.addCleanup(patcher.stop)

    def deliver(self, data=None):
        return gateway.deliver({"operation": "deliver", "run_id": 2, "token": "fixture-read-token"}, io.BytesIO(self.data if data is None else data), self.incoming)

    def test_verified_artifact_is_published_with_installed_tools_and_promoted(self):
        # Uploaded tools deliberately contain invalid Python ('fixture'). A
        # successful operation proves those uploaded programs are never executed.
        result = self.deliver()
        self.assertEqual(result["status"], "deployed")
        self.assertEqual(result["release"], self.expected)
        self.publish.assert_called_once()
        self.promote.assert_called_once()

    def test_ahead_main_is_rejected_before_reading_any_payload(self):
        self.responses["git/ref/heads/main"]["object"]["sha"] = "c" * 40
        source = io.BytesIO(self.data)
        with self.assertRaises(gateway.StaleRelease):
            gateway.deliver({"run_id": 2, "token": "fixture"}, source, self.incoming)
        self.assertEqual(source.tell(), 0)
        self.publish.assert_not_called(); self.promote.assert_not_called()

    def test_tampered_zip_is_rejected_before_publish_or_promotion(self):
        with self.assertRaises(ValueError): self.deliver(self.data[:-1] + b"x")
        self.publish.assert_not_called(); self.promote.assert_not_called()

    def test_fork_failed_ci_and_incompatible_contract_are_rejected(self):
        self.responses["actions/runs/1"]["conclusion"] = "failure"
        with self.assertRaises(ValueError): self.deliver()
        self.publish.assert_not_called(); self.promote.assert_not_called()
        self.responses["actions/runs/1"]["conclusion"] = "success"
        self.current["contract_sha256"] = "c" * 64
        (self.root / "current-release.json").write_text(json.dumps(self.current))
        # A fresh incoming directory represents a new SSH operation.
        self.incoming = self.root / "incoming-next"; self.incoming.mkdir()
        with self.assertRaises(ValueError): self.deliver()
        self.publish.assert_not_called(); self.promote.assert_not_called()
        with self.assertRaises(ValueError):
            gateway.select_artifact({**self.run, "head_repository": {"full_name": "other/repo"}}, [self.artifact])

    def test_probe_identity_cannot_request_delivery(self):
        with self.assertRaises(ValueError):
            gateway.handle({"operation": "deliver", "run_id": 2}, io.BytesIO(self.data), self.incoming, probe_only=True)
        gateway.GitHub.assert_not_called()
        self.publish.assert_not_called(); self.promote.assert_not_called()

    def test_probe_size_and_hash_are_enforced(self):
        for size in (0, True, 8 * 1024**2 + 1):
            with self.subTest(size=size), self.assertRaises(ValueError):
                gateway.handle({"operation": "probe", "size": size, "digest": "sha256:" + "a" * 64}, io.BytesIO(b"x"), self.incoming, True)
        with self.assertRaises(ValueError):
            gateway.handle({"operation": "probe", "size": 1, "digest": "sha256:" + "a" * 64}, io.BytesIO(b"x"), self.incoming, True)

    def test_unchanged_release_is_an_idempotent_noop(self):
        (self.root / "current-release.json").write_text(json.dumps(self.expected))
        self.assertEqual(self.deliver()["status"], "already_current")
        self.publish.assert_not_called(); self.promote.assert_not_called()

    def test_source_attempt_digest_and_expiry_are_enforced(self):
        for changed in ({"expired": True}, {"digest": ""}, {"name": "bundle-" + "a" * 40 + "-2"}):
            with self.subTest(changed=changed), self.assertRaises(ValueError):
                gateway.select_artifact(self.run, [{**self.artifact, **changed}])

    def test_authorized_keys_are_forced_and_cannot_inject_options(self):
        for probe in (False, True):
            key = authorized_key("ssh-ed25519 AAAA\ncommand=evil", probe)
            self.assertTrue(key.startswith('restrict,command="/usr/bin/python3 /usr/local/libexec/isaber-staging-ssh'))
            self.assertNotIn("evil", key)
            self.assertEqual(key.count("\n"), 1)
        with self.assertRaises(ValueError): authorized_key('command="evil" ssh-ed25519 AAAA')
