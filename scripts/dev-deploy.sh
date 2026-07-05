#!/usr/bin/env bash
set -euo pipefail

if [ "${AWS_PROFILE:-}" != "kbdb-dev-admin" ]; then
  echo "AWS_PROFILE must be set to kbdb-dev-admin (got: '${AWS_PROFILE:-<unset>}')" >&2
  exit 1
fi

DEV_NAME="${KBDB_DEV_NAME:-$(whoami)}"
STACK_NAME="kbdb-dev-${DEV_NAME}"
REPO_NAME="kbdb-api-${STACK_NAME}"
ACCOUNT_ID="992234857260"
REGION="us-east-2"

sam build --template-file template.yaml
sam deploy --stack-name "$STACK_NAME" \
  --s3-bucket "kbdb-sam-artifacts-${ACCOUNT_ID}" \
  --region "$REGION" \
  --image-repositories "ApiFunction=${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${REPO_NAME}" \
  --capabilities CAPABILITY_IAM \
  --no-fail-on-empty-changeset
