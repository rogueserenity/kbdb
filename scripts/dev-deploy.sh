#!/usr/bin/env bash
set -euo pipefail

# scripts/env/dev.env is a gitignored symlink to your own committed
# scripts/env/<name>-dev.env (see CONTRIBUTING.md). Fills in unset vars
# only - an existing shell export always wins.
ENV_FILE="$(dirname "$0")/env/dev.env"
if [ -f "$ENV_FILE" ]; then
  while IFS='=' read -r key value; do
    case "$key" in ''|'#'*) continue ;; esac
    if [ -z "${!key:-}" ]; then
      export "$key=$value"
    fi
  done < "$ENV_FILE"
fi

DEV_NAME="${KBDB_DEV_NAME:-$(whoami)}"
STACK_NAME="kbdb-dev-${DEV_NAME}"
REPO_NAME="kbdb-api-${STACK_NAME}"
ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
REGION="${KBDB_DEV_REGION:-$(aws configure get region)}"
OIDC_ISSUER_BASE_URL="${KBDB_OIDC_ISSUER_BASE_URL:?set KBDB_OIDC_ISSUER_BASE_URL - see CONTRIBUTING.md}"
OIDC_AUDIENCE="${KBDB_OIDC_AUDIENCE:?set KBDB_OIDC_AUDIENCE - see CONTRIBUTING.md}"

sam build --template-file template.yaml --region "$REGION"
sam deploy --stack-name "$STACK_NAME" \
  --s3-bucket "kbdb-sam-artifacts-${ACCOUNT_ID}" \
  --region "$REGION" \
  --image-repositories "ApiFunction=${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${REPO_NAME}" \
  --parameter-overrides \
    "OidcIssuerBaseUrl=${OIDC_ISSUER_BASE_URL}" \
    "OidcAudience=${OIDC_AUDIENCE}" \
  --capabilities CAPABILITY_IAM \
  --no-fail-on-empty-changeset \
  --no-confirm-changeset
