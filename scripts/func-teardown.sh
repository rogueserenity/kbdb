#!/usr/bin/env bash
set -uo pipefail
if [ -f .sam-local-api.pid ]; then
  kill "$(cat .sam-local-api.pid)" 2>/dev/null || true
  rm -f .sam-local-api.pid .sam-local-api.log
fi
# Fallback: the tracked PID doesn't always match the actual listener (e.g.
# if a prior teardown was interrupted mid-startup), leaving an orphan bound
# to port 3000 that fails the next func-setup with "Address already in use".
lsof -tiTCP:3000 -sTCP:LISTEN 2>/dev/null | xargs -r kill 2>/dev/null || true
docker compose down
