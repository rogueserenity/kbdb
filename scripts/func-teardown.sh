#!/usr/bin/env bash
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT/.env.floci"

if [[ -f "$ENV_FILE" ]]; then
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  [[ -n "${KBDB_OIDC_GEN_DIR:-}" ]] && rm -rf "$KBDB_OIDC_GEN_DIR"
fi

docker compose -f docker-compose.floci.yml down
rm -f "$ENV_FILE"
