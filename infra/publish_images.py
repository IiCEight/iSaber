"""Publish already-tested local images; never read a server's Docker credentials."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time
import tarfile

REGISTRY = "ccr.ccs.tencentyun.com"
NAMESPACE = "isaber"
DIGEST = re.compile(r"sha256:[0-9a-f]{64}\Z")
SOURCES = {"frontend": "isaber-frontend:rehearsal", "backend": "isaber-api:rehearsal", "postgres": "postgres:16"}


def docker(arguments, *, environment, password=None):
    timeout = 1200 if arguments[0] == "push" else 60
    try:
        result = subprocess.run(
            ["docker", *arguments], env=environment, input=password,
            text=True, capture_output=True, check=False, timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        raise RuntimeError(f"Docker {arguments[0]} exceeded {timeout}s; check the registry network path") from None
    if result.returncode:
        # Do not echo credential-file contents, request headers, or subprocess output.
        detail = ""
        error = (result.stderr or "").lower()
        if any(value in error for value in ("unauthorized", "denied", "authentication required")):
            detail = "; registry authorization rejected: check credentials and repository permissions"
        elif any(value in error for value in ("timeout", "timed out", "deadline exceeded", "connection reset", "tls handshake")):
            detail = "; registry network connection failed"
        elif any(value in error for value in ("manifest invalid", "unsupported media type")):
            detail = "; registry rejected the image manifest format"
        raise RuntimeError(f"Docker {arguments[0]} failed (exit {result.returncode}){detail}")
    return result.stdout.strip()


def image_config_id(image, *, environment, invoke=docker):
    # Docker 29's containerd store may expose an index digest as .Id; Docker 28
    # exposes the config digest. A platform-specific export provides the actual
    # config bytes on both engines, including runtime settings and rootfs hashes.
    with tempfile.TemporaryDirectory(prefix="isaber-image-identity-") as directory:
        archive = Path(directory) / "image.tar"
        invoke(["image", "save", image, "--platform", "linux/amd64", "--output", str(archive)], environment=environment)
        try:
            with tarfile.open(archive) as saved:
                manifest_member = saved.getmember("manifest.json")
                if not manifest_member.isfile() or manifest_member.size > 1024 * 1024:
                    raise ValueError("Invalid saved image manifest")
                with saved.extractfile(manifest_member) as source:
                    manifest = json.load(source)
                if not isinstance(manifest, list) or len(manifest) != 1:
                    raise ValueError("Expected one exported platform image")
                config_member = saved.getmember(manifest[0]["Config"])
                if not config_member.isfile() or config_member.size > 4 * 1024 * 1024:
                    raise ValueError("Invalid saved image configuration")
                with saved.extractfile(config_member) as source:
                    return "sha256:" + hashlib.sha256(source.read()).hexdigest()
        except (tarfile.TarError, KeyError, TypeError):
            raise ValueError("Could not verify the exported image configuration") from None


def published_config_id(reference, *, environment, invoke=docker):
    manifest = json.loads(invoke(["manifest", "inspect", reference], environment=environment))
    if "manifests" in manifest:
        candidates = [item for item in manifest["manifests"] if item.get("platform", {}).get("os") == "linux" and item.get("platform", {}).get("architecture") == "amd64"]
        if len(candidates) != 1 or not DIGEST.fullmatch(candidates[0].get("digest", "")):
            raise ValueError("Expected one published linux/amd64 manifest")
        child = reference.split("@", 1)[0] + "@" + candidates[0]["digest"]
        manifest = json.loads(invoke(["manifest", "inspect", child], environment=environment))
    identity = manifest.get("config", {}).get("digest", "")
    if not DIGEST.fullmatch(identity):
        raise ValueError("Invalid published image configuration digest")
    return identity


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
        source_ids = {}
        for name, image in SOURCES.items():
            identity = image_config_id(image, environment=child_env, invoke=invoke)
            if not DIGEST.fullmatch(identity):
                raise ValueError("Invalid local image identity")
            source_ids[name] = identity
        postgres_sources = json.loads(invoke(["image", "inspect", SOURCES["postgres"], "--format", "{{json .RepoDigests}}"], environment=child_env))
        upstream = next((value for value in (postgres_sources or []) if value.startswith(("postgres@", "docker.io/library/postgres@")) and DIGEST.fullmatch(value.split("@", 1)[1])), None)
        if not upstream:
            raise ValueError("PostgreSQL must have a verified official upstream digest")
        print(f"Authenticating to {REGISTRY}", flush=True)
        invoke(["login", REGISTRY, "--username", username, "--password-stdin"], environment=child_env, password=password)
        print("Registry authentication succeeded", flush=True)
        for name, source in SOURCES.items():
            repository = f"{REGISTRY}/{NAMESPACE}/{name}"
            tag = f"{repository}:sha-{commit}"
            invoke(["tag", source, tag], environment=child_env)
            print(f"Uploading {name}: {tag} (command limit: 1200s)", flush=True)
            started = time.monotonic()
            summary = invoke(["push", tag], environment=child_env)
            # Docker 29 can retain a local RepoDigest for an incomplete index,
            # even when it pushed only the available platform. Use the actual
            # push receipt, then verify that exact digest against the registry.
            matching = re.findall(r"(?m)^" + re.escape(tag.rsplit(":", 1)[1]) + r": digest: (sha256:[0-9a-f]{64}) size: [0-9]+\s*$", summary)
            if len(matching) != 1:
                raise ValueError("Expected one digest receipt for the pushed tag")
            reference = repository + "@" + matching[0]
            if published_config_id(reference, environment=child_env, invoke=invoke) != source_ids[name]:
                raise ValueError("Published platform no longer identifies the tested image")
            metadata["images"][name] = {"reference": reference, "image_id": source_ids[name]}
            print(f"Published {name}: {reference} ({time.monotonic() - started:.1f}s)", flush=True)
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
        if len(sys.argv) == 2 and sys.argv[1] == "--check-images":
            for name, image in SOURCES.items():
                print(f"{name}: {image_config_id(image, environment=os.environ)}", flush=True)
        elif len(sys.argv) == 2:
            publish(sys.argv[1], os.environ)
        else:
            raise ValueError("Usage: publish_images.py OUTPUT_MANIFEST | --check-images")
    except (ValueError, RuntimeError, OSError) as error:
        print(f"Publishing failed: {error}", file=sys.stderr)
        sys.exit(1)
