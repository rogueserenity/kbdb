#!/usr/bin/env bash
set -uo pipefail
if [ -f .sam-local-api.pid ]; then
  kill "$(cat .sam-local-api.pid)" 2>/dev/null || true
  rm -f .sam-local-api.pid .sam-local-api.log
fi
# Fallbacks for a process the PID file no longer tracks: still bound to
# 3000, or an orphan that already lost the port to a replacement.
lsof -tiTCP:3000 -sTCP:LISTEN 2>/dev/null | xargs -r kill 2>/dev/null || true
pkill -f 'sam local start-api' 2>/dev/null || true
docker compose down
