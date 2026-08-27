#!/usr/bin/env bash
# Deploys the real template.yaml to floci (a local AWS emulator) and starts
# the WorkOS emulator against it, so the functional suite runs against a
# real CloudFormation deploy - including a real API Gateway JWT authorizer,
# which sam local start-api never emulated (write routes rely on it
# entirely now - see internal/middleware.RequireAuthorizerIdentity).
set -euo pipefail

export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-2
# sam deploy honors this (botocore reads it); there is no --endpoint-url
# flag, which is all samlocal wraps.
export AWS_ENDPOINT_URL="${KBDB_FLOCI_ENDPOINT:-http://localhost.floci.io:4566}"

STACK="${KBDB_FLOCI_STACK:-kbdb-floci}"
ENDPOINT="${KBDB_FLOCI_ENDPOINT:-http://localhost.floci.io:4566}"
OIDC_BUCKET="kbdb-floci-oidc"
CLIENT_ID="client_local_kbdb"
# The base must match docker-compose.floci.yml's WORKOS_EMULATE_ISSUER
# exactly. From emulator v0.10.0 the minted AuthKit access token's iss is
# "<base>/user_management/<client_id>" (as in production WorkOS), and go-oidc
# does an exact-string iss match against the discovery doc's "issuer" - so
# the discovery doc's "issuer", the URL go-oidc fetches, and OidcIssuerBaseUrl
# all carry the /user_management/<client_id> suffix, while WORKOS_EMULATE_ISSUER
# stays bare (the emulator appends the suffix itself).
ISSUER_BASE_URL="$ENDPOINT/$OIDC_BUCKET"
ISSUER_URL="$ISSUER_BASE_URL/user_management/$CLIENT_ID"

docker compose -f docker-compose.floci.yml up -d floci workos-emulate

for _ in $(seq 1 30); do
  curl -sf -o /dev/null "$ENDPOINT/_floci/health" && break
  sleep 1
done
for _ in $(seq 1 20); do
  curl -sf -o /dev/null "http://localhost:4100/health" && break
  sleep 0.5
done

sam build

# sam deploy --resolve-image-repos fabricates the standard AWS ECR hostname
# itself rather than reading floci's returned repositoryUri, so it always
# tries to push to a real AWS host. dev-deploy.sh/ci.yml already avoid
# --resolve-image-repos for the equivalent real-AWS reason - mirror that:
# create the repo explicitly and pass --image-repositories.
ECR_REPO="kbdb-floci"
aws ecr create-repository --repository-name "$ECR_REPO" >/dev/null 2>&1 || true
REPO_URI=$(aws ecr describe-repositories \
  --repository-names "$ECR_REPO" --query 'repositories[0].repositoryUri' --output text)

# The emulator does serve its own per-client discovery doc from v0.10.0, but
# its jwks_uri points at localhost:4100, which the deployed Lambda can't
# reach - so host a static doc here instead whose jwks_uri points at the
# emulator's real, live, sibling-reachable JWKS endpoint. The doc is just a
# pointer, never a JWKS snapshot, so it can't go stale and there's no signing
# key to generate or pin (the emulator mints its own at startup). It's served
# under the /user_management/<client_id> suffix so go-oidc finds it at
# "$ISSUER_URL/.well-known/openid-configuration".
aws s3api create-bucket --bucket "$OIDC_BUCKET" \
  --region "$AWS_DEFAULT_REGION" \
  --create-bucket-configuration LocationConstraint="$AWS_DEFAULT_REGION" \
  >/dev/null 2>&1 || true

cat <<EOF | aws s3 cp - "s3://$OIDC_BUCKET/user_management/$CLIENT_ID/.well-known/openid-configuration" \
  --content-type application/json >/dev/null
{
  "issuer": "$ISSUER_URL",
  "jwks_uri": "http://workos-emulate:4100/oauth2/jwks",
  "response_types_supported": ["code", "id_token"],
  "subject_types_supported": ["public"],
  "id_token_signing_alg_values_supported": ["RS256"]
}
EOF

sam deploy \
  --stack-name "$STACK" \
  --no-confirm-changeset --no-fail-on-empty-changeset \
  --resolve-s3 \
  --image-repositories "ApiFunction=$REPO_URI" \
  --capabilities CAPABILITY_IAM \
  --parameter-overrides \
    SkipApiRepository=true \
    "OidcIssuerBaseUrl=$ISSUER_URL" \
    "OidcAudience=client_local_kbdb" \
    "StytchPublicToken=public-token-test-local-kbdb" \
    "LogoutReturnOrigins=http://localhost:5173"

out() {
  aws cloudformation describe-stacks --stack-name "$STACK" \
    --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text
}

# scripts/workos-emulate-seed.yaml pre-seeds these two users plus the
# client_local_kbdb application - no need to create them per run like
# ci.yml does for its throwaway per-PR users.
create_and_mint() {
  local email="$1" password="$2"
  curl -sf -X POST http://localhost:4100/user_management/authenticate \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"client_local_kbdb\",\"client_secret\":\"sk_test_default\",\"grant_type\":\"password\",\"email\":\"$email\",\"password\":\"$password\"}" \
    | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])"
}

KBDB_AUTH_TOKEN=$(create_and_mint "kbdb-local-test-user@rogueserenity.dev" "kbdb-local-test-password-1")
KBDB_SECOND_USER_AUTH_TOKEN=$(create_and_mint "kbdb-local-second-user@rogueserenity.dev" "kbdb-local-test-password-2")

API_ID=$(aws apigatewayv2 get-apis --query 'Items[0].ApiId' --output text)

# func-test.sh sources this directly, so `mise run func-test` picks up a
# fresh deploy's stack-derived values without any manual export step.
ENV_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.env.floci"
cat <<ENVEOF > "$ENV_FILE"
export KBDB_API_BASE_URL='$ENDPOINT/execute-api/$API_ID/\$default'
export KBDB_DYNAMODB_ENDPOINT_URL=$ENDPOINT
export KBDB_SWITCH_TABLE_NAME=$(out SwitchTableName)
export KBDB_KEYBOARD_TABLE_NAME=$(out KeyboardTableName)
export KBDB_KEYCAP_SET_TABLE_NAME=$(out KeycapSetTableName)
export KBDB_BUILD_TABLE_NAME=$(out BuildTableName)
export KBDB_PROFILE_TABLE_NAME=$(out ProfileTableName)
export KBDB_PROFILE_USERNAME_TABLE_NAME=$(out ProfileUsernameTableName)
export KBDB_AUTH_TOKEN=$KBDB_AUTH_TOKEN
export KBDB_SECOND_USER_AUTH_TOKEN=$KBDB_SECOND_USER_AUTH_TOKEN
ENVEOF

echo "Deployed. Wrote $ENV_FILE - mise run func-test sources it automatically."
