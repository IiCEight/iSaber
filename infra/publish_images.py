"""Publish already-tested local images; never read a server's Docker credentials."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

REGISTRY = "ccr.ccs.tencentyun.com"
NAMESPACE = "isaber"
DIGEST = re.compile(r"sha256:[0-9a-f]{64}\Z")


def docker(arguments, *, environment, password=None):
    result = subprocess.run(
        ["docker", *arguments], env=environment, input=password,
        text=True, capture_output=True, check=False,
    )
    if result.returncode:
        # Do not echo credential-file contents, request headers, or subprocess output.
        raise RuntimeError(f"Docker {arguments[0]} failed (exit {result.returncode})")
    return result.stdout.strip()


def publish(destination, environment, *, root=None, invoke=docker):
    root = root or Path(__file__).resolve().parents[1]
    commit = environment.get("RELEASE_SHA", "")
    username = environment.get("TCR_USERNAME", "")
    password = environment.get("TCR_PASSWORD", "")
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("RELEASE_SHA must be a full source commit")
    if not username or not password:
        raise ValueError("TCR_USERNAME and TCR_PASSWORD secrets are required")
    version = re.search(r"const RequiredVersion int64 = ([0-9]+)", (root / "backend/internal/migrations/migrations.go").read_text())
    if not version:
        raise ValueError("Could not determine the migration version")
    metadata = {
        "format_version": 1, "source_commit": commit, "platform": "linux/amd64",
        "migration_version": int(version[1]),
        "contract_sha256": hashlib.sha256((root / "contracts/openapi.yaml").read_bytes()).hexdigest(),
        "ci_run_id": environment.get("VERIFIED_CI_RUN_ID", ""),
        "images": {},
    }
    child_env = {key: value for key, value in environment.items() if key not in {"TCR_PASSWORD", "TCR_USERNAME"}}
    with tempfile.TemporaryDirectory(prefix="isaber-registry-auth-") as auth:
        child_env["DOCKER_CONFIG"] = auth
        sources = {"frontend": "isaber-frontend:rehearsal", "backend": "isaber-api:rehearsal", "postgres": "postgres:16"}
        source_ids = {}
        for name, image in sources.items():
            identity = invoke(["image", "inspect", image, "--format", "{{.Id}}"], environment=child_env)
            if not DIGEST.fullmatch(identity):
                raise ValueError("Invalid local image identity")
            source_ids[name] = identity
        postgres_sources = json.loads(invoke(["image", "inspect", sources["postgres"], "--format", "{{json .RepoDigests}}"], environment=child_env))
        upstream = next((value for value in (postgres_sources or []) if value.startswith(("postgres@", "docker.io/library/postgres@")) and DIGEST.fullmatch(value.split("@", 1)[1])), None)
        if not upstream:
            raise ValueError("PostgreSQL must have a verified official upstream digest")
        invoke(["login", REGISTRY, "--username", username, "--password-stdin"], environment=child_env, password=password)
        for name, source in sources.items():
            repository = f"{REGISTRY}/{NAMESPACE}/{name}"
            tag = f"{repository}:sha-{commit}"
            invoke(["tag", source, tag], environment=child_env)
            invoke(["push", tag], environment=child_env)
            if invoke(["image", "inspect", tag, "--format", "{{.Id}}"], environment=child_env) != source_ids[name]:
                raise ValueError("Published tag no longer identifies the tested image")
            digests = json.loads(invoke(["image", "inspect", tag, "--format", "{{json .RepoDigests}}"], environment=child_env))
            matching = [value for value in (digests or []) if value.startswith(repository + "@") and DIGEST.fullmatch(value.split("@", 1)[1])]
            if len(matching) != 1:
                raise ValueError("Expected one digest for the published repository")
            metadata["images"][name] = {"reference": matching[0], "image_id": source_ids[name]}
            print(f"Published {name}: {matching[0]}")
        metadata["images"]["postgres"]["upstream_reference"] = upstream
    destination = Path(destination)
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(mode="w", dir=destination.parent, delete=False) as output:
        temporary = Path(output.name)
        json.dump(metadata, output, indent=2)
        output.write("\n")
    try:
        os.replace(temporary, destination)
    finally:
        temporary.unlink(missing_ok=True)
    return metadata


if __name__ == "__main__":
    try:
        if len(sys.argv) != 2:
            raise ValueError("Usage: publish_images.py OUTPUT_MANIFEST")
        publish(sys.argv[1], os.environ)
    except (ValueError, RuntimeError, OSError) as error:
        print(f"Publishing failed: {error}", file=sys.stderr)
        sys.exit(1)
