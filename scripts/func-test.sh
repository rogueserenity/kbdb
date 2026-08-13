#!/usr/bin/env bash
set -euo pipefail
# Defaults for local runs against LocalStack; ci.yml sets these itself for
# a real deployed stack, so don't override an already-set value.
export KBDB_SWITCH_TABLE_NAME="${KBDB_SWITCH_TABLE_NAME:-kbdb-local-switch}"
export KBDB_KEYBOARD_TABLE_NAME="${KBDB_KEYBOARD_TABLE_NAME:-kbdb-local-keyboard}"
export KBDB_KEYCAP_SET_TABLE_NAME="${KBDB_KEYCAP_SET_TABLE_NAME:-kbdb-local-keycap-set}"
export KBDB_BUILD_TABLE_NAME="${KBDB_BUILD_TABLE_NAME:-kbdb-local-build}"
export KBDB_DYNAMODB_ENDPOINT_URL="${KBDB_DYNAMODB_ENDPOINT_URL-http://localhost:4566}"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-2}"
go tool ginkgo run ./test/functional/...
