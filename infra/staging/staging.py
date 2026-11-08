"""Operate only the dedicated private isaber-staging Compose project on JD."""
import argparse
import base64
import json
import os
from pathlib import Path
import re
import secrets
import ssl
import subprocess
import sys
import time
import urllib.request

CONFIG = Path(__file__).resolve().parent
sys.path.insert(0, str(CONFIG.parent))
from publish_images import image_config_id
PROJECT = "isaber-staging"
DIGEST = r"sha256:[0-9a-f]{64}"


def run(arguments, *, data=None, timeout=180):
    result = subprocess.run(arguments, input=data, capture_output=True, timeout=timeout)
    if result.returncode:
        # Database and Compose errors can include interpolated credentials.
        raise RuntimeError(f"{arguments[0]} operation failed (exit {result.returncode}); inspect the protected host configuration")
    return result.stdout


def write_private(path, data):
    temporary = path.with_suffix(path.suffix + ".pending")
    descriptor = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(descriptor, "w") as output:
        output.write(data)
    temporary.chmod(0o600)
    os.replace(temporary, path)


def validate_release(release):
    if release.get("format_version") != 1 or release.get("platform") != "linux/amd64" or not re.fullmatch(r"[0-9a-f]{40}", release.get("source_commit", "")):
        raise ValueError("Require a verified linux/amd64 release manifest")
    if release.get("migration_version") != 5 or not re.fullmatch(r"[0-9a-f]{64}", release.get("contract_sha256", "")):
        raise ValueError("This staging configuration requires the reviewed version 5 schema")
    if set(release.get("images", {})) != {"frontend", "backend", "postgres"}:
        raise ValueError("Unexpected release image set")
    for name, image in release["images"].items():
        if not re.fullmatch(r"ccr\.ccs\.tencentyun\.com/isaber/" + name + "@" + DIGEST, image.get("reference", "")) or not re.fullmatch(DIGEST, image.get("image_id", "")):
            raise ValueError("Release image must use its repository-bound immutable digest")
    if not re.fullmatch(r"(?:docker.io/library/)?postgres@" + DIGEST, release["images"]["postgres"].get("upstream_reference", "")):
        raise ValueError("Database provenance is required")


def credentials(root):
    path = root / "secrets.env"
    if path.is_symlink() or path.stat().st_mode & 0o077:
        raise ValueError("Staging secrets must be a private regular file")
    return dict(line.split("=", 1) for line in path.read_text().splitlines() if line)


def initialize(root):
    root.mkdir(parents=True, exist_ok=True, mode=0o700)
    root.chmod(0o700)
    if (root / "secrets.env").exists():
        credentials(root)
        print("Existing staging secrets retained")
        return
    existing = subprocess.run(["docker", "volume", "inspect", PROJECT + "_database"], capture_output=True)
    if existing.returncode == 0:
        raise ValueError("Database volume already exists; restore its original staging secrets before continuing")
    if run(["docker", "info", "--format", "{{.ServerVersion}}"]):
        password, runtime = secrets.token_hex(32), secrets.token_hex(32)
        values = {
            "POSTGRES_PASSWORD": password, "RUNTIME_PASSWORD": runtime,
            "DATABASE_URL": f"postgres://isaber_runtime:{runtime}@postgres:5432/isaber_staging?sslmode=disable",
            "OPERATOR_DATABASE_URL": f"postgres://isaber_operator:{password}@postgres:5432/isaber_staging?sslmode=disable",
            "CSRF_SECRET": base64.b64encode(secrets.token_bytes(32)).decode(),
        }
        write_private(root / "secrets.env", "".join(f"{key}={value}\n" for key, value in values.items()))
        print("Generated dedicated staging secrets on the server; no values printed")


def compose(root, *arguments, data=None, timeout=180):
    return run(["docker", "compose", "--project-name", PROJECT, "--env-file", str(root / "secrets.env"), "--env-file", str(root / "release.env"), "--file", str(CONFIG / "compose.yml"), *arguments], data=data, timeout=timeout)


def sql(root, statement):
    return compose(root, "exec", "-T", "postgres", "psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", "isaber_operator", "-d", "isaber_staging", "-At", data=statement.encode())


def grant_runtime(root):
    password = credentials(root)["RUNTIME_PASSWORD"]
    if not re.fullmatch(r"[0-9a-f]{64}", password):
        raise ValueError("Invalid generated runtime credential")
    if sql(root, "SELECT count(*) FROM pg_roles WHERE rolname='isaber_runtime';").strip() == b"0":
        sql(root, "CREATE ROLE isaber_runtime LOGIN;")
    sql(root, f"""BEGIN;
ALTER ROLE isaber_runtime PASSWORD '{password}';
REVOKE ALL ON DATABASE isaber_staging FROM PUBLIC;
GRANT CONNECT ON DATABASE isaber_staging TO isaber_runtime;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO isaber_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO isaber_runtime;
REVOKE ALL ON owners, goose_db_version FROM isaber_runtime;
GRANT SELECT ON owners, goose_db_version TO isaber_runtime;
-- Login uses SELECT FOR SHARE. A column UPDATE privilege permits row locking;
-- CHECK(singleton) forces this column to stay true, and credentials stay protected.
GRANT UPDATE(singleton) ON owners TO isaber_runtime;
COMMIT;""")


def verify_https(root):
    certificate = root / "staging-root.crt"
    last_error = None
    for _ in range(30):
        try:
            compose(root, "cp", "edge:/data/caddy/pki/authorities/local/root.crt", str(certificate), timeout=10)
            certificate.chmod(0o600)
            context = ssl.create_default_context(cafile=str(certificate))
            for path in ("/healthz", "/", "/admin", "/api/v1/articles"):
                with urllib.request.urlopen("https://localhost:3443" + path, context=context, timeout=10) as response:
                    if response.status != 200:
                        raise ValueError("Private HTTPS acceptance failed")
            print("Private HTTPS certificate, public routes, and owner sign-in page verified")
            return
        except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
            last_error = type(error).__name__
            time.sleep(1)
    raise RuntimeError("Private HTTPS did not become ready: " + str(last_error))


def deploy(root, manifest):
    credentials(root)
    release = json.loads(Path(manifest).read_text())
    validate_release(release)
    proxy = json.loads((CONFIG / "proxy-image.json").read_text())
    if not re.fullmatch(DIGEST, proxy["image_id"]):
        raise ValueError("Invalid proxy configuration identity")
    tag = "isaber-staging-caddy:" + proxy["version"].removesuffix("-alpine")
    if image_config_id(tag, environment=os.environ) != proxy["image_id"]:
        raise ValueError("Offline proxy configuration differs from its verified upstream")
    engine_id = run(["docker", "image", "inspect", tag, "--format", "{{.Id}}"]).decode().strip()
    if not re.fullmatch(DIGEST, engine_id):
        raise ValueError("Invalid local proxy image ID")
    previous = (root / "release.env").read_text() if (root / "release.env").exists() else None
    # Updating a data-bearing staging instance requires a verified backup first.
    if (root / "current-release.json").exists():
        raise ValueError("Staging is already deployed; take a coordinated backup and review promotion/rollback before updating")
    values = {name.upper() + "_IMAGE": image["reference"] for name, image in release["images"].items()}
    values["CADDY_IMAGE"] = engine_id
    write_private(root / "release.env", "".join(f"{key}={value}\n" for key, value in values.items()))
    try:
        compose(root, "config", "--quiet")
        print("Pulling the three immutable release images", flush=True)
        compose(root, "pull", "postgres", "api", "frontend", timeout=900)
        print("Starting the dedicated database and applying explicit migrations", flush=True)
        compose(root, "up", "-d", "--wait", "postgres")
        compose(root, "run", "--rm", "--no-deps", "migrate", "up")
        grant_runtime(root)
        print("Starting private HTTPS, frontend, and API", flush=True)
        compose(root, "up", "-d", "--wait", "api", "frontend", "edge")
        verify_https(root)
        write_private(root / "current-release.json", json.dumps(release, indent=2) + "\n")
        print("Private staging deployed at https://localhost:3443 through SSH; public web ports were not used")
    except Exception:
        compose(root, "stop", "edge", "frontend", "api")
        if previous is not None:
            write_private(root / "release.env", previous)
        raise


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=("init", "deploy", "verify", "status"))
    parser.add_argument("--root", type=Path, default=Path("/opt/isaber-staging"))
    parser.add_argument("--release", type=Path)
    arguments = parser.parse_args()
    try:
        if arguments.command == "init":
            initialize(arguments.root)
        elif arguments.command == "deploy":
            if not arguments.release:
                raise ValueError("A verified --release manifest is required")
            deploy(arguments.root, arguments.release)
        elif arguments.command == "verify":
            verify_https(arguments.root)
        else:
            print(compose(arguments.root, "ps").decode())
    except Exception as error:
        # Never echo a subprocess command, environment, secret, or raw DB error.
        print(f"Staging operation failed ({type(error).__name__}): check the protected host configuration")
        raise SystemExit(1)
