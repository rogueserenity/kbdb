#!/usr/bin/env bash
set -euo pipefail
# Table names/API URL/auth tokens are real, stack-derived values from a
# floci or CI deploy (not fixed local names) - func-setup.sh prints the
# exports to run; ci.yml sets them itself for its real deployed stack.
: "${KBDB_SWITCH_TABLE_NAME:?run scripts/func-setup.sh and export its printed KBDB_* vars first}"
: "${KBDB_KEYBOARD_TABLE_NAME:?run scripts/func-setup.sh and export its printed KBDB_* vars first}"
: "${KBDB_KEYCAP_SET_TABLE_NAME:?run scripts/func-setup.sh and export its printed KBDB_* vars first}"
: "${KBDB_BUILD_TABLE_NAME:?run scripts/func-setup.sh and export its printed KBDB_* vars first}"
: "${KBDB_API_BASE_URL:?run scripts/func-setup.sh and export its printed KBDB_* vars first}"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-2}"
go tool ginkgo run ./test/functional/...
