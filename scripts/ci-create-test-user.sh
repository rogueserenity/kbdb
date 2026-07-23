#!/usr/bin/env bash
set -euo pipefail

# Creates (or recreates) a throwaway Cognito user against the given user
# pool, optionally adds it to a group, and prints its minted ID token to
# stdout (nothing else - callers capture this via $(...), so no other
# command here may write to stdout). Used by ci.yml's functional-test job
# to provision a plain and an admin test identity against a real deployed
# stack.
#
# Usage: ci-create-test-user.sh <user-pool-id> <client-id> <username> <password> [group]

USER_POOL_ID="$1"
CLIENT_ID="$2"
USERNAME="$3"
PASSWORD="$4"
GROUP="${5:-}"

# synchronize (a later push to the same PR) redeploys the same
# stack/UserPool rather than a fresh one, so this fixed username already
# exists from the prior run - delete first. Only swallow the specific
# "didn't exist yet" case (a genuinely first run against a fresh stack);
# any other failure (throttling, a real permissions regression, etc.)
# should fail loudly here rather than surface later as a confusing
# UsernameExistsException from admin-create-user below.
if ! DELETE_ERROR=$(aws cognito-idp admin-delete-user \
  --user-pool-id "$USER_POOL_ID" \
  --username "$USERNAME" 2>&1); then
  if ! grep -q "UserNotFoundException" <<< "$DELETE_ERROR"; then
    echo "$DELETE_ERROR" >&2
    exit 1
  fi
fi

aws cognito-idp admin-create-user \
  --user-pool-id "$USER_POOL_ID" \
  --username "$USERNAME" \
  --user-attributes Name=email,Value="$USERNAME" Name=email_verified,Value=true \
  --message-action SUPPRESS >/dev/null

aws cognito-idp admin-set-user-password \
  --user-pool-id "$USER_POOL_ID" \
  --username "$USERNAME" \
  --password "$PASSWORD" \
  --permanent >/dev/null

if [ -n "$GROUP" ]; then
  aws cognito-idp admin-add-user-to-group \
    --user-pool-id "$USER_POOL_ID" \
    --username "$USERNAME" \
    --group-name "$GROUP" >/dev/null
fi

aws cognito-idp admin-initiate-auth \
  --user-pool-id "$USER_POOL_ID" \
  --client-id "$CLIENT_ID" \
  --auth-flow ADMIN_USER_PASSWORD_AUTH \
  --auth-parameters USERNAME="$USERNAME",PASSWORD="$PASSWORD" \
  --query "AuthenticationResult.IdToken" \
  --output text
