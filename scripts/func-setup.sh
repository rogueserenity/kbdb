#!/usr/bin/env bash
set -euo pipefail
docker compose up -d --build

# sam local start-api never runs a real CloudFormation deploy, so
# template.yaml's LookupTable is never provisioned locally - create it by
# hand. KEEP THIS IN SYNC with template.yaml's LookupTable resource
# (AttributeDefinitions/KeySchema/BillingMode) - there's no automated check
# for drift between the two. Table name/region must also match
# test/functional/support/env.local.json and samconfig.toml's region. This
# whole block goes away once local dev deploys the real template against
# floci instead of sam local start-api (see project memory on the floci
# migration).
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-2

# Up to 15s waiting for LocalStack to accept connections; one extra
# describe-table check if the table already exists from a prior run.
for _ in $(seq 1 15); do
  aws dynamodb describe-table --endpoint-url http://localhost:4566 \
    --table-name kbdb-local-lookup >/dev/null 2>&1 && break
  aws dynamodb create-table \
    --endpoint-url http://localhost:4566 \
    --table-name kbdb-local-lookup \
    --attribute-definitions AttributeName=category,AttributeType=S \
    --key-schema AttributeName=category,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    >/dev/null 2>&1 && break
  sleep 1
done

# KEEP THIS IN SYNC with template.yaml's SwitchTable resource - same caveat
# as the LookupTable block above.
for _ in $(seq 1 15); do
  aws dynamodb describe-table --endpoint-url http://localhost:4566 \
    --table-name kbdb-local-switch >/dev/null 2>&1 && break
  aws dynamodb create-table \
    --endpoint-url http://localhost:4566 \
    --table-name kbdb-local-switch \
    --attribute-definitions AttributeName=user_id,AttributeType=S AttributeName=id,AttributeType=S \
    --key-schema AttributeName=user_id,KeyType=HASH AttributeName=id,KeyType=RANGE \
    --billing-mode PAY_PER_REQUEST \
    >/dev/null 2>&1 && break
  sleep 1
done

# KEEP THIS IN SYNC with template.yaml's KeyboardTable resource - same
# caveat as the LookupTable block above.
for _ in $(seq 1 15); do
  aws dynamodb describe-table --endpoint-url http://localhost:4566 \
    --table-name kbdb-local-keyboard >/dev/null 2>&1 && break
  aws dynamodb create-table \
    --endpoint-url http://localhost:4566 \
    --table-name kbdb-local-keyboard \
    --attribute-definitions AttributeName=user_id,AttributeType=S AttributeName=id,AttributeType=S \
    --key-schema AttributeName=user_id,KeyType=HASH AttributeName=id,KeyType=RANGE \
    --billing-mode PAY_PER_REQUEST \
    >/dev/null 2>&1 && break
  sleep 1
done

# KEEP THIS IN SYNC with template.yaml's KeycapSetTable resource - same
# caveat as the LookupTable block above.
for _ in $(seq 1 15); do
  aws dynamodb describe-table --endpoint-url http://localhost:4566 \
    --table-name kbdb-local-keycap-set >/dev/null 2>&1 && break
  aws dynamodb create-table \
    --endpoint-url http://localhost:4566 \
    --table-name kbdb-local-keycap-set \
    --attribute-definitions AttributeName=user_id,AttributeType=S AttributeName=id,AttributeType=S \
    --key-schema AttributeName=user_id,KeyType=HASH AttributeName=id,KeyType=RANGE \
    --billing-mode PAY_PER_REQUEST \
    >/dev/null 2>&1 && break
  sleep 1
done

sam build
nohup sam local start-api > .sam-local-api.log 2>&1 &
echo $! > .sam-local-api.pid
echo "sam local start-api started (pid $(cat .sam-local-api.pid)); logs: .sam-local-api.log"
