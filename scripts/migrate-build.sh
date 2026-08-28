#!/usr/bin/env bash
# Builds the kbdb-migrate CLI into bin/. sam build only packages the Lambda, so
# this standalone operational tool needs its own build step.
set -euo pipefail

version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
mkdir -p bin
go build -ldflags "-X main.Version=${version}" -o bin/kbdb-migrate ./cmd/kbdb-migrate
echo "built bin/kbdb-migrate (${version})"
