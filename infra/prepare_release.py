"""Reuse the exact verified image bundle from successful push-to-main CI."""
import argparse
import json
import os
from pathlib import Path
import re
import sys
import tempfile

from receive_artifact import receive
from relay_release import REPOSITORY, artifact_url, github
from release_bundle import BASELINE_FILE, METADATA_FILES, ROOT, digest, publish, verify
from publish_images import docker, REGISTRY


def select_ci_artifact(run, jobs, artifacts, commit):
    if (run.get("path") != ".github/workflows/ci.yml" or run.get("event") != "push"
            or run.get("head_branch") != "main" or run.get("head_sha") != commit
            or run.get("conclusion") != "success"
            or run.get("head_repository", {}).get("full_name") != REPOSITORY):
        raise ValueError("Require successful push-to-main CI for this exact commit")
    attempt = run.get("run_attempt")
    if type(attempt) is not int or attempt < 1:
        raise ValueError("Invalid CI attempt")
    if not any(job.get("name") == "Containers" and job.get("conclusion") == "success" for job in jobs):
        raise ValueError("The source CI container rehearsal did not succeed")
    matches = [artifact for artifact in artifacts if not artifact.get("expired")
               and artifact.get("name") == f"ci-images-{commit}-{attempt}"]
    if len(matches) != 1:
        raise ValueError("Expected one unexpired tested-image bundle for the exact CI attempt; rerun CI instead of rebuilding in preparation")
    artifact = matches[0]
    if (not re.fullmatch(r"sha256:[0-9a-f]{64}", artifact.get("digest", ""))
            or type(artifact.get("size_in_bytes")) is not int
            or not 0 < artifact["size_in_bytes"] <= 512 * 1024**2):
        raise ValueError("CI artifact lacks a bounded integrity receipt")
    return artifact


def rebind(directory, environment, ci_attempt, artifact):
    commit = environment.get("RELEASE_SHA", "")
    ci_run_id = environment.get("VERIFIED_CI_RUN_ID", "")
    bundle = verify(directory, commit)
    if (bundle["format_version"] != 2 or str(bundle["ci_run_id"]) != ci_run_id
            or str(bundle["bundle_run_id"]) != ci_run_id
            or str(bundle["bundle_run_attempt"]) != str(ci_attempt)):
        raise ValueError("Tested images belong to another CI run or attempt")
    for relative in (*METADATA_FILES, BASELINE_FILE):
        if digest(ROOT / relative) != bundle["files"][relative]["sha256"]:
            raise ValueError("CI bundle metadata differs from the trusted source checkout")
    bundle.update(bundle_run_id=environment.get("GITHUB_RUN_ID", ""),
                  bundle_run_attempt=environment.get("GITHUB_RUN_ATTEMPT", ""),
                  ci_run_attempt=ci_attempt, ci_artifact_id=artifact["id"],
                  ci_artifact_digest=artifact["digest"])
    (Path(directory) / "bundle.json").write_text(json.dumps(bundle, indent=2) + "\n")
    verify(directory, commit)
    return bundle


def prepare(destination, environment):
    ci_run_id = environment.get("VERIFIED_CI_RUN_ID", "")
    commit = environment.get("RELEASE_SHA", "")
    if not re.fullmatch(r"[1-9][0-9]*", ci_run_id) or not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("An exact source CI run and commit are required")
    run = github(f"actions/runs/{ci_run_id}")
    attempt = run.get("run_attempt")
    jobs = github(f"actions/runs/{ci_run_id}/attempts/{attempt}/jobs?per_page=100")["jobs"]
    artifacts = github(f"actions/runs/{ci_run_id}/artifacts?per_page=100")["artifacts"]
    artifact = select_ci_artifact(run, jobs, artifacts, commit)
    destination = Path(destination)
    if destination.exists():
        raise ValueError("Preparation output must not already exist")
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="isaber-ci-images-", dir=destination.parent) as temporary:
        directory = Path(temporary)
        payload = {key: artifact[key] for key in ("digest", "size_in_bytes")}
        payload["url"] = artifact_url(artifact)
        receive(directory, payload, label="Preparation runner")
        rebind(directory / "content", environment, attempt, artifact)
        (directory / "content").rename(destination)
    print(f"Reused tested images from CI {ci_run_id}, attempt {attempt}, artifact {artifact['id']} ({artifact['digest']}); no image build or re-export", flush=True)


def publish_prepared(directory, output, environment):
    # The optional direct path also publishes the reused bundle, without a
    # daemon or a fresh build. Credentials never enter metadata or argv.
    verify(directory, environment.get("RELEASE_SHA", ""))
    username, password = environment.get("TCR_USERNAME"), environment.get("TCR_PASSWORD")
    if not username or not password:
        raise ValueError("Registry credentials are required for direct publication")
    child_env = {key: value for key, value in environment.items() if key not in ("TCR_USERNAME", "TCR_PASSWORD")}
    with tempfile.TemporaryDirectory(prefix="isaber-registry-auth-") as temporary:
        child_env["DOCKER_CONFIG"] = temporary
        docker(["login", REGISTRY, "--username", username, "--password-stdin"], environment=child_env, password=password)
        authfile = Path(temporary) / "config.json"
        authfile.chmod(0o600)
        publish(directory, output, environment["RELEASE_SHA"], authfile)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("operation", choices=("prepare", "publish"))
    parser.add_argument("directory", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    try:
        if args.operation == "prepare":
            prepare(args.directory, os.environ)
        elif args.output:
            publish_prepared(args.directory, args.output, os.environ)
        else:
            raise ValueError("Publication requires an output manifest")
    except Exception as error:
        # Signed URLs and credentials may occur in exception text; omit it.
        print(f"Tested-image preparation failed ({type(error).__name__}); require a successful matching CI attempt and unexpired artifact", file=sys.stderr)
        raise SystemExit(1)
