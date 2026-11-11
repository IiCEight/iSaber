#!/usr/bin/env bash
# Application images only. Database migrations and first setup are explicit operator steps.
set -euo pipefail
operation=${1:-deploy}
[[ $# -le 1 && "$operation" =~ ^(deploy|rollback)$ ]] || exit 1
if [[ "$operation" == deploy ]]; then
  IFS= read -r -t 15 -n 72 frontend
  IFS= read -r -t 15 -n 72 backend
  [[ "$frontend" =~ ^sha256:[0-9a-f]{64}$ && "$backend" =~ ^sha256:[0-9a-f]{64}$ ]] || exit 1
fi

# ISABER_ROOT is for operator tests; the SSH entry removes it before sudo.
root=${ISABER_ROOT:-/opt/isaber}
cd "$root"
umask 077
exec 9>.deploy.lock
flock -n 9 || { printf 'Another deployment is running.\n' >&2; exit 1; }
[[ -f .env && ! -L .env && -f compose.yml && -f Caddyfile ]]
domain=$(sed -n 's/^BLOG_DOMAIN=//p' .env)
bind=$(sed -n 's/^BLOG_BIND_ADDR=//p' .env)
[[ "$domain" =~ ^[a-zA-Z0-9.-]+$ && "$domain" == *.* && "$bind" =~ ^[0-9.]+$ ]]

# Docker/Compose errors can include configuration: retain them only on the host.
exec 3>&1
exec >>deploy.log 2>&1
compose() { docker compose --project-name isaber --env-file .env --env-file "$1" --file compose.yml "${@:2}"; }
healthy() {
  for _ in {1..12}; do
    if curl --fail --silent --show-error --noproxy '*' --max-time 10 \
      --resolve "$domain:443:$bind" "https://$domain/healthz" >/dev/null; then return 0; fi
    sleep 5
  done
  return 1
}
activate() {
  cp "$1" .release.env.pending || return 1
  mv .release.env.pending release.env || return 1
  compose release.env up -d --no-deps --pull never --wait --wait-timeout 120 api frontend edge && healthy
}
changed=0
restore=''
finish() {
  status=$?
  trap - EXIT
  trap '' HUP INT TERM
  if (( status != 0 && changed )); then
    if [[ -n "$restore" ]] && activate "$restore"; then
      printf 'Deployment failed; previous application images restored.\n' >&3 || true
    else
      if [[ -z "$restore" ]]; then compose release.env stop edge frontend api || true; fi
      printf 'Deployment/rollback failed; inspect /opt/isaber/deploy.log.\n' >&3 || true
    fi
  elif (( status != 0 )); then
    printf 'Deployment failed before switching images; inspect /opt/isaber/deploy.log.\n' >&3 || true
  fi
  rm -f .candidate.env .before.env .release.env.pending
  exit "$status"
}
trap finish EXIT
trap 'exit 130' HUP INT TERM
if [[ "$operation" == rollback ]]; then
  [[ -f previous.env && ! -L previous.env ]]
  cp previous.env .candidate.env
else
  printf 'FRONTEND_IMAGE=ghcr.nju.edu.cn/iiceight/isaber-frontend@%s\nBACKEND_IMAGE=ghcr.nju.edu.cn/iiceight/isaber-backend@%s\n' \
    "$frontend" "$backend" > .candidate.env
fi
# Files used for rollback are generated here, not uploaded programs or shell input.
mapfile -t refs < .candidate.env
[[ ${#refs[@]} == 2 ]]
[[ "${refs[0]}" =~ ^FRONTEND_IMAGE=ghcr\.nju\.edu\.cn/iiceight/isaber-frontend@sha256:[0-9a-f]{64}$ ]]
[[ "${refs[1]}" =~ ^BACKEND_IMAGE=ghcr\.nju\.edu\.cn/iiceight/isaber-backend@sha256:[0-9a-f]{64}$ ]]
timeout --kill-after=10s 8m bash -c 'exec docker compose --project-name isaber --env-file .env --env-file .candidate.env --file compose.yml pull api frontend'
if [[ -f release.env ]]; then
  cp release.env .before.env
  restore=.before.env
fi
changed=1
activate .candidate.env
if [[ -n "$restore" ]] && ! cmp -s "$restore" .candidate.env; then cp "$restore" previous.env; fi
changed=0
printf 'Application images deployed and HTTPS health check passed.\n' >&3
