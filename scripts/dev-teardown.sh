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
REGION="${KBDB_DEV_REGION:-$(aws configure get region)}"

# ECR refuses to delete a repository that still contains images (a real
# AWS safety guard, confirmed via a live DELETE_FAILED error) - the stack
# delete below would otherwise fail on ApiRepository every time, since a
# real developer's repo almost always has at least one pushed image.
IMAGE_IDS=$(aws ecr list-images --repository-name "$REPO_NAME" --region "$REGION" --query "imageIds" --output json 2>/dev/null || echo "[]")
if [ "$IMAGE_IDS" != "[]" ]; then
  echo "Deleting images in $REPO_NAME..."
  aws ecr batch-delete-image --repository-name "$REPO_NAME" --region "$REGION" \
    --image-ids "$IMAGE_IDS" >/dev/null
fi

echo "Deleting $STACK_NAME (including its owned ECR repo)..."
aws cloudformation delete-stack --stack-name "$STACK_NAME" --region "$REGION"
aws cloudformation wait stack-delete-complete --stack-name "$STACK_NAME" --region "$REGION"
echo "$STACK_NAME deleted."
