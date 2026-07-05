#!/usr/bin/env bash
set -euo pipefail
golangci-lint run ./...
actionlint
# actionlint only lints workflow files' own run: blocks (via shellcheck under
# the hood) - scripts/*.sh live standalone now, not embedded in a workflow, so
# they need an explicit shellcheck pass to stay covered.
shellcheck scripts/*.sh
