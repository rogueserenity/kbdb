#!/usr/bin/env bash
# Experimental: bring up floci and deploy the real template.yaml against it,
# instead of localstack + sam local start-api.
#
# BLOCKED - see floci-issues/. The stack deploys and DynamoDB works, but no
# request reaches the app. Kept so the next attempt starts here rather than
# rediscovering the setup.
set -euo pipefail

export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-2
# sam deploy honors this (botocore reads it); there is no --endpoint-url
# flag, which is all samlocal wraps.
export AWS_ENDPOINT_URL="${KBDB_FLOCI_ENDPOINT:-http://localhost:4566}"

STACK="${KBDB_FLOCI_STACK:-kbdb-floci}"
ENDPOINT="${KBDB_FLOCI_ENDPOINT:-http://localhost:4566}"

docker compose -f docker-compose.floci.yml up -d

for _ in $(seq 1 30); do
  curl -sf -o /dev/null "$ENDPOINT/_floci/health" && break
  sleep 1
done

sam build

# sam deploy works against floci via AWS_ENDPOINT_URL, but its ECR image
# push fails (see floci-issues/04). Deploying the built artifact directly
# sidesteps the push, since floci can run the local image by tag.
aws --endpoint-url "$ENDPOINT" cloudformation deploy \
  --template-file .aws-sam/build/template.yaml \
  --stack-name "$STACK" \
  --parameter-overrides SkipApiRepository=true \
  --capabilities CAPABILITY_IAM \
  --no-fail-on-empty-changeset

out() {
  aws --endpoint-url "$ENDPOINT" cloudformation describe-stacks --stack-name "$STACK" \
    --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text
}

API_ID=$(aws --endpoint-url "$ENDPOINT" apigatewayv2 get-apis --query 'Items[0].ApiId' --output text)

cat <<ENVEOF

Deployed. Export these to run the suite:

  export KBDB_API_BASE_URL='$ENDPOINT/execute-api/$API_ID/\$default'
  export KBDB_DYNAMODB_ENDPOINT_URL=$ENDPOINT
  export KBDB_LOOKUP_TABLE_NAME=$(out LookupTableName)
  export KBDB_SWITCH_TABLE_NAME=$(out SwitchTableName)
  export KBDB_KEYBOARD_TABLE_NAME=$(out KeyboardTableName)
  export KBDB_KEYCAP_SET_TABLE_NAME=$(out KeycapSetTableName)

ENVEOF
