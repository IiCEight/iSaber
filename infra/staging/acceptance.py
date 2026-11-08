"""First-deployment acceptance on empty staging; removes its temporary owner."""
import argparse
import http.cookiejar
import json
from pathlib import Path
import secrets
import ssl
import urllib.error
import urllib.request

from staging import compose, sql, verify_https


def accept(root):
    if sql(root, "SELECT count(*) FROM owners;").strip() != b"0":
        raise ValueError("Initial acceptance requires an empty owner table; existing accounts are never changed")
    rights = sql(root, """SELECT
        has_column_privilege('isaber_runtime','owners','password_hash','UPDATE'),
        has_table_privilege('isaber_runtime','owners','INSERT'),
        has_table_privilege('isaber_runtime','goose_db_version','UPDATE'),
        has_schema_privilege('isaber_runtime','public','CREATE');""")
    if rights.strip() != b"f|f|f|f":
        raise ValueError("Runtime role has unexpected operator privileges")
    username = "staging-acceptance-" + secrets.token_hex(6)
    password = secrets.token_urlsafe(32)
    context = ssl.create_default_context(cafile=str(root / "staging-root.crt"))
    cookies = http.cookiejar.CookieJar()
    opener = urllib.request.build_opener(urllib.request.HTTPSHandler(context=context), urllib.request.HTTPCookieProcessor(cookies))

    def request(path, expected=200, body=None, headers=None):
        payload = json.dumps(body).encode() if body is not None else None
        options = {"Content-Type": "application/json", **(headers or {})}
        call = urllib.request.Request("https://localhost:3443" + path, data=payload, headers=options)
        try:
            response = opener.open(call, timeout=20)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            if response.status != expected:
                raise ValueError("HTTPS acceptance status mismatch at " + path)
            return response.read(), response.headers

    request("/api/v1/auth/session", 401)
    created = False
    try:
        compose(root, "run", "--rm", "--no-deps", "-T", "owner", "setup", "--username", username, "--password-stdin", data=(password + "\n").encode())
        created = True
        request("/api/v1/auth/login", 403, {"username": username, "password": password}, {"Origin": "https://untrusted.example"})
        body, headers = request("/api/v1/auth/login", body={"username": username, "password": password}, headers={"Origin": "https://localhost:3443"})
        session = json.loads(body)
        cookie = headers.get("Set-Cookie", "")
        if not cookie.startswith("__Host-isaber_session=") or not all(flag in cookie for flag in ("Secure", "HttpOnly", "Path=/")):
            raise ValueError("Production session cookie attributes missing")
        request("/api/v1/auth/session")
        request("/api/v1/admin/articles")
        page, _ = request("/admin")
        if username.encode() not in page:
            raise ValueError("Authenticated server rendering failed")
        request("/api/v1/auth/logout", 403, {}, {"Origin": "https://localhost:3443"})
        ca = (root / "staging-root.crt").read_bytes()
        print("HTTPS login, owner rendering, operator/runtime separation, and origin/CSRF checks passed", flush=True)
        compose(root, "restart", "postgres", "api", "frontend", "edge")
        compose(root, "up", "-d", "--wait", "postgres", "api", "frontend", "edge")
        verify_https(root)
        if ca != (root / "staging-root.crt").read_bytes():
            raise ValueError("Staging certificate authority changed across restart")
        request("/api/v1/auth/session")
        request("/api/v1/auth/logout", 204, {}, {"Origin": "https://localhost:3443", "X-CSRF-Token": session["csrf_token"]})
        request("/api/v1/auth/session", 401)
        print("Database/session/CA persistence across restart and logout verified", flush=True)
    finally:
        if created:
            sql(root, f"BEGIN; DELETE FROM sessions WHERE owner_id IN (SELECT id FROM owners WHERE username='{username}'); DELETE FROM owners WHERE username='{username}'; COMMIT;")
            print("Temporary acceptance owner removed; owner setup is ready for you", flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, default=Path("/opt/isaber-staging"))
    options = parser.parse_args()
    try:
        accept(options.root)
    except Exception as error:
        print(f"Staging acceptance failed ({type(error).__name__}); credential values were omitted")
        raise SystemExit(1)
