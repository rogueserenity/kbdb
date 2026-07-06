#!/usr/bin/env bash
set -euo pipefail

DEV_NAME="${KBDB_DEV_NAME:-$(whoami)}"
STACK_NAME="kbdb-dev-${DEV_NAME}"
REPO_NAME="kbdb-api-${STACK_NAME}"
ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
REGION="${KBDB_DEV_REGION:-$(aws configure get region)}"
REPO_URI="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${REPO_NAME}"
S3_BUCKET="kbdb-sam-artifacts-${ACCOUNT_ID}"

if aws cloudformation describe-stacks --stack-name "$STACK_NAME" --region "$REGION" >/dev/null 2>&1; then
  echo "Stack $STACK_NAME already exists - run 'mise run dev-teardown' first if you want to start over." >&2
  exit 1
fi

echo "Bootstrapping $STACK_NAME (repo: $REPO_NAME)..."

# a. Standalone repo (DeletionPolicy: Retain baked in - required before import).
aws cloudformation deploy --template-file bootstrap/ecr-repo.yaml \
  --stack-name "kbdb-ecr-bootstrap-${STACK_NAME}" \
  --region "$REGION" \
  --parameter-overrides "RepositoryName=${REPO_NAME}"

# b. Build and push an initial image - sam deploy can't resolve ApiFunction's
# ImageUri from a repo that doesn't hold an image yet.
sam build --template-file template.yaml
PACKAGED_TEMPLATE="packaged-${STACK_NAME}.yaml"
sam package --s3-bucket "$S3_BUCKET" \
  --image-repository "$REPO_URI" \
  --output-template-file "$PACKAGED_TEMPLATE"

# c. Deploy everything except ApiRepository (it already exists standalone -
# CloudFormation can't create a second, colliding repo of the same name).
# Strip it from the PACKAGED template (produced by sam package above), not
# the raw source template - the packaged one already has ApiFunction's
# ImageUri resolved to the just-pushed image's real digest; stripping from
# the unpackaged template.yaml would deploy with the local build's
# apifunction:latest tag instead, which doesn't exist in ECR. sam deploy
# still requires --image-repositories even though ImageUri is already fully
# resolved (confirmed via a real "Missing option" error without it) - SAM's
# own CLI validation doesn't distinguish "already resolved" ahead of time.
NO_REPO_TEMPLATE="template-no-repo-${STACK_NAME}.yaml"
awk '
  /^  ApiRepository:$/ { skip = 1; next }
  skip && /^  [A-Za-z]/ { skip = 0 }
  !skip
' "$PACKAGED_TEMPLATE" > "$NO_REPO_TEMPLATE"

sam deploy --template-file "$NO_REPO_TEMPLATE" \
  --stack-name "$STACK_NAME" \
  --s3-bucket "$S3_BUCKET" \
  --region "$REGION" \
  --image-repositories "ApiFunction=${REPO_URI}" \
  --capabilities CAPABILITY_IAM \
  --no-confirm-changeset

# d. Import the standalone repo into the now-existing stack. First delete
# the standalone bootstrap stack from step (a) - CloudFormation refuses to
# import a resource still tracked/owned by another stack (confirmed via a
# real "already exists in stack ..." changeset failure); DeletionPolicy:
# Retain (already on this resource, baked into bootstrap/ecr-repo.yaml)
# means deleting that stack does NOT delete the actual repo, only
# CloudFormation's ownership record of it - the repo itself survives,
# confirmed via a real describe-repositories check after the delete.
aws cloudformation delete-stack --stack-name "kbdb-ecr-bootstrap-${STACK_NAME}" --region "$REGION"
aws cloudformation wait stack-delete-complete --stack-name "kbdb-ecr-bootstrap-${STACK_NAME}" --region "$REGION"

# DeletionPolicy: Retain must already be present in the template submitted
# for import - add it to the packaged template (which still has
# ApiRepository) before diffing.
sed -i.bak $'/^  ApiRepository:$/a\\
    DeletionPolicy: Retain' "$PACKAGED_TEMPLATE"
rm -f "${PACKAGED_TEMPLATE}.bak"

RESOURCES_TO_IMPORT="resources-to-import-${STACK_NAME}.json"
cat > "$RESOURCES_TO_IMPORT" <<EOF
[{"ResourceType":"AWS::ECR::Repository","LogicalResourceId":"ApiRepository","ResourceIdentifier":{"RepositoryName":"${REPO_NAME}"}}]
EOF

aws cloudformation create-change-set --stack-name "$STACK_NAME" \
  --region "$REGION" \
  --change-set-name import-ecr-repo \
  --change-set-type IMPORT \
  --template-body "file://${PACKAGED_TEMPLATE}" \
  --capabilities CAPABILITY_IAM \
  --resources-to-import "file://${RESOURCES_TO_IMPORT}"

aws cloudformation wait change-set-create-complete \
  --stack-name "$STACK_NAME" --change-set-name import-ecr-repo --region "$REGION"
aws cloudformation execute-change-set --stack-name "$STACK_NAME" \
  --change-set-name import-ecr-repo --region "$REGION"
aws cloudformation wait stack-import-complete --stack-name "$STACK_NAME" --region "$REGION"

# e. Reconcile: the import step above can't include Outputs, and DeletionPolicy:
# Retain isn't wanted long-term (template.yaml's tracked ApiRepository has none -
# it deletes normally as part of dev-teardown's stack delete). A plain sam deploy
# with the real, unmodified template reconciles both in one pass.
sam deploy --stack-name "$STACK_NAME" \
  --s3-bucket "$S3_BUCKET" \
  --region "$REGION" \
  --image-repositories "ApiFunction=${REPO_URI}" \
  --capabilities CAPABILITY_IAM \
  --no-confirm-changeset --no-fail-on-empty-changeset

rm -f "$PACKAGED_TEMPLATE" "$NO_REPO_TEMPLATE" "$RESOURCES_TO_IMPORT"
echo "$STACK_NAME is ready. Use 'mise run dev-deploy' for subsequent deploys."
