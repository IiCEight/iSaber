#!/usr/bin/env bash
# Runs after image acceptance on the same GitHub runner. Never rebuilds images.
set -euo pipefail
: "${GITHUB_REPOSITORY_OWNER:?}" "${GITHUB_REPOSITORY:?}" "${GITHUB_SHA:?}"
: "${GITHUB_ACTOR:?}" "${GH_TOKEN:?}" "${GITHUB_OUTPUT:?}"
namespace=${GITHUB_REPOSITORY_OWNER,,}
[[ "$namespace" =~ ^[a-z0-9-]+$ && "$GITHUB_SHA" =~ ^[0-9a-f]{40}$ ]]
trap 'docker logout ghcr.io >/dev/null 2>&1 || true' EXIT
printf '%s' "$GH_TOKEN" | docker login ghcr.io --username "$GITHUB_ACTOR" --password-stdin
for kind in frontend backend; do
  source=isaber-frontend:rehearsal
  [[ "$kind" != backend ]] || source=isaber-api:rehearsal
  repository="ghcr.io/$namespace/isaber-$kind"
  image="$repository:sha-$GITHUB_SHA"
  docker tag "$source" "$image"
  docker push "$image"
  reference=$(docker image inspect "$image" --format '{{range .RepoDigests}}{{println .}}{{end}}' |
    awk -v prefix="$repository@" 'index($0,prefix)==1 {print substr($0,length(prefix)+1); exit}')
  [[ "$reference" =~ ^sha256:[0-9a-f]{64}$ ]]
  printf '%s=%s\n' "$kind" "$reference" >> "$GITHUB_OUTPUT"
  printf '%s\n' "$repository@$reference" >> "${GITHUB_STEP_SUMMARY:-/dev/null}"
done
