#!/usr/bin/env bash
set -uo pipefail
docker compose -f docker-compose.floci.yml down
rm -f "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.env.floci"
