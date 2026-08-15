#!/usr/bin/env bash
# Experimental: bring up floci and deploy the real template.yaml against it,
# instead of localstack + sam local start-api.
#
# The full functional suite passes against this setup - see floci-issues/.
# Requires a local floci build from upstream/main until floci-io/floci cuts
# a release tag containing #2146 and #2150 (see floci-issues/README.md).
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

# sam deploy --resolve-image-repos fabricates the standard AWS ECR hostname
# itself rather than reading floci's returned repositoryUri, so it always
# tries to push to a real AWS host (floci-issues/04, retracted as a floci
# bug). Every real deploy path here (dev-deploy.sh, CI) already avoids
# --resolve-image-repos for the equivalent real-AWS reason - mirror that:
# create the repo explicitly and pass --image-repositories.
ECR_REPO="kbdb-floci"
aws --endpoint-url "$ENDPOINT" ecr create-repository \
  --repository-name "$ECR_REPO" >/dev/null 2>&1 || true
REPO_URI=$(aws --endpoint-url "$ENDPOINT" ecr describe-repositories \
  --repository-names "$ECR_REPO" --query 'repositories[0].repositoryUri' --output text)

sam deploy \
  --stack-name "$STACK" \
  --no-confirm-changeset --no-fail-on-empty-changeset \
  --resolve-s3 \
  --image-repositories "ApiFunction=$REPO_URI" \
  --capabilities CAPABILITY_IAM \
  --parameter-overrides \
    SkipApiRepository=true \
    "OidcIssuerBaseUrl=http://localhost.floci.io:4566"

out() {
  aws --endpoint-url "$ENDPOINT" cloudformation describe-stacks --stack-name "$STACK" \
    --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text
}

POOL_ID=$(out UserPoolId)
CLIENT_ID=$(out UserPoolClientId)

# floci stubs AWS::Cognito::UserPoolGroup - the stack reports CREATE_COMPLETE
# but no group exists, so AdminAddUserToGroup fails and admin tokens carry no
# cognito:groups claim. CreateGroup works at runtime; see floci-issues/02.
aws --endpoint-url "$ENDPOINT" cognito-idp create-group \
  --user-pool-id "$POOL_ID" --group-name admins >/dev/null 2>&1 || true

# Mint the same three identities CI does, via the same script, and export
# them through the overrides token.go already honors.
PASSWORD="$(openssl rand -base64 24)Aa1!"
KBDB_AUTH_TOKEN=$(scripts/ci-create-test-user.sh "$POOL_ID" "$CLIENT_ID" \
  "ci-test-user@rogueserenity.dev" "$PASSWORD")
KBDB_ADMIN_AUTH_TOKEN=$(scripts/ci-create-test-user.sh "$POOL_ID" "$CLIENT_ID" \
  "ci-test-admin@rogueserenity.dev" "$PASSWORD" admins)
KBDB_SECOND_USER_AUTH_TOKEN=$(scripts/ci-create-test-user.sh "$POOL_ID" "$CLIENT_ID" \
  "ci-test-user-2@rogueserenity.dev" "$PASSWORD")

API_ID=$(aws --endpoint-url "$ENDPOINT" apigatewayv2 get-apis --query 'Items[0].ApiId' --output text)

cat <<ENVEOF

Deployed. Export these to run the suite:

  export KBDB_API_BASE_URL='$ENDPOINT/execute-api/$API_ID/\$default'
  export KBDB_DYNAMODB_ENDPOINT_URL=$ENDPOINT
  export KBDB_LOOKUP_TABLE_NAME=$(out LookupTableName)
  export KBDB_SWITCH_TABLE_NAME=$(out SwitchTableName)
  export KBDB_KEYBOARD_TABLE_NAME=$(out KeyboardTableName)
  export KBDB_KEYCAP_SET_TABLE_NAME=$(out KeycapSetTableName)
  export KBDB_AUTH_TOKEN=$KBDB_AUTH_TOKEN
  export KBDB_ADMIN_AUTH_TOKEN=$KBDB_ADMIN_AUTH_TOKEN
  export KBDB_SECOND_USER_AUTH_TOKEN=$KBDB_SECOND_USER_AUTH_TOKEN

ENVEOF
