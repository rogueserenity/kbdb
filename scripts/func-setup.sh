#!/usr/bin/env bash
set -euo pipefail
docker compose up -d --build
sam build
nohup sam local start-api > .sam-local-api.log 2>&1 &
echo $! > .sam-local-api.pid
echo "sam local start-api started (pid $(cat .sam-local-api.pid)); logs: .sam-local-api.log"
