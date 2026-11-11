#!/usr/bin/env bash
# Test observable release state and rollback; no cloud access or Docker is needed.
set -euo pipefail
script=$(cd "$(dirname "$0")/.." && pwd)/deploy.sh
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT
mkdir "$temporary/bin"
export PATH="$temporary/bin:$PATH"
cat > "$temporary/bin/docker" <<'DOCKER'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$ISABER_ROOT/calls"
if [[ "$*" == *' pull '* && -f "$ISABER_ROOT/fail-pull" ]]; then exit 1; fi
if [[ "$*" == *' up '* && -f "$ISABER_ROOT/fail-start" ]] && grep -q 'bbbbbbbb' "$ISABER_ROOT/release.env"; then exit 1; fi
if [[ "$*" == *' up '* && -f "$ISABER_ROOT/interrupt-start" ]]; then
  rm "$ISABER_ROOT/interrupt-start"
  kill -HUP "$PPID"
  exit 1
fi
DOCKER
cat > "$temporary/bin/curl" <<'CURL'
#!/usr/bin/env bash
exit 0
CURL
chmod +x "$temporary/bin/docker" "$temporary/bin/curl"
old="sha256:$(printf 'a%.0s' {1..64})"
new="sha256:$(printf 'b%.0s' {1..64})"
setup() {
  export ISABER_ROOT="$temporary/$1"
  mkdir "$ISABER_ROOT"
  printf 'BLOG_DOMAIN=blog.example.com\nBLOG_BIND_ADDR=10.0.0.6\nPOSTGRES_PASSWORD=private-fixture\n' > "$ISABER_ROOT/.env"
  touch "$ISABER_ROOT/compose.yml" "$ISABER_ROOT/Caddyfile"
  printf 'FRONTEND_IMAGE=ghcr.nju.edu.cn/iiceight/isaber-frontend@%s\nBACKEND_IMAGE=ghcr.nju.edu.cn/iiceight/isaber-backend@%s\n' "$old" "$old" > "$ISABER_ROOT/release.env"
  cp "$ISABER_ROOT/.env" "$ISABER_ROOT/original-secrets"
  cp "$ISABER_ROOT/release.env" "$ISABER_ROOT/original-release"
}
deploy() { printf '%s\n%s\n' "$new" "$new" | bash "$script" deploy; }
setup invalid
if printf 'untrusted/image:latest\n%s\n' "$new" | bash "$script" deploy; then exit 1; fi
test ! -e "$ISABER_ROOT/calls"
setup success
deploy
grep -q "$new" "$ISABER_ROOT/release.env"
cmp "$ISABER_ROOT/original-release" "$ISABER_ROOT/previous.env"
cmp "$ISABER_ROOT/original-secrets" "$ISABER_ROOT/.env"
bash "$script" rollback
cmp "$ISABER_ROOT/original-release" "$ISABER_ROOT/release.env"
for failure in fail-pull fail-start interrupt-start; do
  setup "$failure"
  touch "$ISABER_ROOT/$failure"
  if deploy; then printf 'Expected failure: %s\n' "$failure" >&2; exit 1; fi
  cmp "$ISABER_ROOT/original-release" "$ISABER_ROOT/release.env"
  cmp "$ISABER_ROOT/original-secrets" "$ISABER_ROOT/.env"
  ! grep -Eq 'migrate|down|volume|postgres' "$ISABER_ROOT/calls"
done
setup locked
if printf '%s\n%s\n' "$new" "$new" | flock "$ISABER_ROOT/.deploy.lock" bash "$script" deploy; then exit 1; fi
test ! -e "$ISABER_ROOT/calls"
printf 'Deployment input, success, rollback, pull/start failure, interruption, and locking checks passed.\n'
