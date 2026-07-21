#!/usr/bin/env bash
set -euo pipefail
export KBDB_LOOKUP_TABLE_NAME=kbdb-local-lookup
export KBDB_DYNAMODB_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-2
go tool ginkgo run ./test/functional/...
