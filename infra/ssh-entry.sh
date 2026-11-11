#!/usr/bin/env bash
# Root-owned forced command for a dedicated deployment key in ubuntu's authorized_keys.
set -euo pipefail
case "${SSH_ORIGINAL_COMMAND:-}" in
  deploy|rollback)
    exec sudo -n env -u ISABER_ROOT /opt/isaber/deploy.sh "$SSH_ORIGINAL_COMMAND"
    ;;
  *) printf 'This key permits only deploy or rollback.\n' >&2; exit 1 ;;
esac
