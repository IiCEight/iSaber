import copy
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import prepare_release as preparation
import release_bundle
import test_release_bundle as fixtures


class PreparationTests(unittest.TestCase):
    def setUp(self):
        self.fixture = fixtures.BundleTests()
        self.fixture.setUp()
        self.addCleanup(self.fixture.doCleanups)
        self.fixture.use_external_database()
        self.run = {"path": ".github/workflows/ci.yml", "event": "push", "head_branch": "main",
                    "head_sha": "a" * 40, "conclusion": "success", "run_attempt": 1,
                    "head_repository": {"full_name": "IiCEight/iSaber"}}
        self.jobs = [{"name": "Containers", "conclusion": "success"}]
        self.artifact = {"id": 10, "name": "ci-images-" + "a" * 40 + "-1", "expired": False,
                         "digest": "sha256:" + "b" * 64, "size_in_bytes": 100}
        self.environment = {"RELEASE_SHA": "a" * 40, "VERIFIED_CI_RUN_ID": "1",
                            "GITHUB_RUN_ID": "3", "GITHUB_RUN_ATTEMPT": "2"}
        # The CI exporter writes its own run/attempt until preparation rebinds it.
        self.fixture.metadata.update(bundle_run_id="1", bundle_run_attempt="1")
        self.fixture.save()

    def select(self, run=None, jobs=None, artifacts=None):
        return preparation.select_ci_artifact(self.run if run is None else run,
                                             self.jobs if jobs is None else jobs,
                                             [self.artifact] if artifacts is None else artifacts, "a" * 40)

    def test_exact_successful_main_ci_attempt_is_selected(self):
        self.assertEqual(self.select(), self.artifact)

    def test_fork_pr_wrong_commit_failed_or_wrong_workflow_ci_is_rejected(self):
        for change in ({"head_repository": {"full_name": "fork/repo"}}, {"event": "pull_request"},
                       {"head_sha": "c" * 40}, {"conclusion": "failure"}, {"head_branch": "feature"},
                       {"path": ".github/workflows/other.yml"}, {"run_attempt": True}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                self.select(run={**self.run, **change})

    def test_missing_failed_container_rehearsal_is_rejected(self):
        for jobs in ([], [{"name": "Containers", "conclusion": "failure"}]):
            with self.subTest(jobs=jobs), self.assertRaises(ValueError):
                self.select(jobs=jobs)

    def test_expired_missing_duplicate_or_other_attempt_artifacts_are_rejected(self):
        for artifacts in ([], [self.artifact, self.artifact], [{**self.artifact, "expired": True}],
                          [{**self.artifact, "name": "ci-images-" + "a" * 40 + "-2"}]):
            with self.subTest(artifacts=artifacts), self.assertRaises(ValueError):
                self.select(artifacts=artifacts)

    def test_artifact_without_bounded_digest_receipt_is_rejected(self):
        for change in ({"digest": ""}, {"size_in_bytes": True}, {"size_in_bytes": 513 * 1024**2}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                self.select(artifacts=[{**self.artifact, **change}])

    def test_rebinding_keeps_every_tested_file_and_image_digest_unchanged(self):
        before = copy.deepcopy(self.fixture.metadata)
        with patch.object(preparation, "ROOT", self.fixture.root):
            result = preparation.rebind(self.fixture.root, self.environment, 1, self.artifact)
        self.assertEqual(result["images"], before["images"])
        self.assertEqual(result["files"], before["files"])
        self.assertEqual(result["ci_run_id"], "1")
        self.assertEqual(result["ci_run_attempt"], 1)
        self.assertEqual((result["bundle_run_id"], result["bundle_run_attempt"]), ("3", "2"))
        self.assertEqual(result["ci_artifact_digest"], self.artifact["digest"])
        release_bundle.verify(self.fixture.root, "a" * 40)

    def test_bundle_from_other_ci_run_attempt_or_source_is_rejected(self):
        original = copy.deepcopy(self.fixture.metadata)
        for change in ({"ci_run_id": "7"}, {"bundle_run_id": "7"}, {"bundle_run_attempt": "7"},
                       {"source_commit": "c" * 40}):
            self.fixture.metadata = {**original, **change}; self.fixture.save()
            with self.subTest(change=change), patch.object(preparation, "ROOT", self.fixture.root), self.assertRaises(ValueError):
                preparation.rebind(self.fixture.root, self.environment, 1, self.artifact)

    def test_trusted_checkout_drift_and_tampered_images_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(preparation, "ROOT", Path(directory)):
            relative = release_bundle.METADATA_FILES[0]
            path = Path(directory) / relative; path.parent.mkdir(parents=True); path.write_text("changed source")
            with self.assertRaisesRegex(ValueError, "trusted source checkout"):
                preparation.rebind(self.fixture.root, self.environment, 1, self.artifact)
        path = next(path for path in (self.fixture.root / "images/frontend").iterdir() if path.name != "manifest.json")
        path.write_bytes(b"tampered")
        with patch.object(preparation, "ROOT", self.fixture.root), self.assertRaises(ValueError):
            preparation.rebind(self.fixture.root, self.environment, 1, self.artifact)

    def test_preparation_downloads_receipt_verified_bundle_without_building(self):
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "prepared"
            def download(root, payload, **kwargs):
                self.assertEqual(payload["digest"], self.artifact["digest"])
                self.assertEqual(payload["size_in_bytes"], self.artifact["size_in_bytes"])
                import shutil
                shutil.copytree(self.fixture.root, root / "content")
            with patch.object(preparation, "github", side_effect=[self.run, {"jobs": self.jobs}, {"artifacts": [self.artifact]}]), patch.object(preparation, "artifact_url", return_value="https://fixture.blob.core.windows.net/artifact"), patch.object(preparation, "receive", side_effect=download), patch.object(preparation, "ROOT", self.fixture.root), patch.object(preparation, "docker") as docker:
                preparation.prepare(destination, self.environment)
            result = release_bundle.verify(destination, "a" * 40)
            self.assertEqual(result["images"], self.fixture.metadata["images"])
            docker.assert_not_called()

    def test_direct_publication_uses_reused_bundle_and_ephemeral_credentials(self):
        environment = {**self.environment, "TCR_USERNAME": "fixture-user", "TCR_PASSWORD": "fixture-secret"}
        def login(arguments, *, environment, password):
            self.assertEqual(password, "fixture-secret")
            self.assertNotIn(password, " ".join(arguments))
            self.assertNotIn("TCR_PASSWORD", environment)
            auth = Path(environment["DOCKER_CONFIG"]) / "config.json"
            auth.write_text("{}"); auth.chmod(0o600)
        with patch.object(preparation, "docker", side_effect=login), patch.object(preparation, "publish") as publish:
            preparation.publish_prepared(self.fixture.root, "release.json", environment)
        self.assertEqual(publish.call_args.args[:3], (self.fixture.root, "release.json", "a" * 40))
        self.assertFalse(publish.call_args.args[3].exists())
