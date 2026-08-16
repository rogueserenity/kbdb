#!/usr/bin/env bash
set -euo pipefail

DEV_NAME="${KBDB_DEV_NAME:-$(whoami)}"
STACK_NAME="kbdb-dev-${DEV_NAME}"
REPO_NAME="kbdb-api-${STACK_NAME}"
ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
REGION="${KBDB_DEV_REGION:-$(aws configure get region)}"
WORKOS_AUTHKIT_DOMAIN="${KBDB_WORKOS_AUTHKIT_DOMAIN:?set KBDB_WORKOS_AUTHKIT_DOMAIN - see CONTRIBUTING.md}"
WORKOS_USER_MANAGEMENT_CLIENT_ID="${KBDB_WORKOS_USER_MANAGEMENT_CLIENT_ID:?set KBDB_WORKOS_USER_MANAGEMENT_CLIENT_ID - see CONTRIBUTING.md}"

sam build --template-file template.yaml --region "$REGION"
sam deploy --stack-name "$STACK_NAME" \
  --s3-bucket "kbdb-sam-artifacts-${ACCOUNT_ID}" \
  --region "$REGION" \
  --image-repositories "ApiFunction=${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${REPO_NAME}" \
  --parameter-overrides \
    "WorkOSAuthKitDomain=${WORKOS_AUTHKIT_DOMAIN}" \
    "WorkOSUserManagementClientId=${WORKOS_USER_MANAGEMENT_CLIENT_ID}" \
  --capabilities CAPABILITY_IAM \
  --no-fail-on-empty-changeset \
  --no-confirm-changeset
