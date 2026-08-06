# floci issues blocking kbdb's functional suite

Found by deploying kbdb's real `template.yaml` to a local floci built from
`rogueserenity/floci` @ `fix/sam-httpapi-transform`, then attempting to run
the Ginkgo functional suite against it.

Each file is scoped to one PR against floci.

| # | issue | severity | blocks |
|---|---|---|---|
| 01 | [HTTP API execute-api region resolution](01-httpapi-execute-region-resolution.md) | blocker | every request to a deployed HTTP API |
| 02 | [UserPoolGroup silently stubbed](02-cognito-userpoolgroup-silently-stubbed.md) | high | the 3 admin lookups specs |
| 03 | [Cognito OIDC issuer path shape](03-cognito-oidc-issuer-path-shape.md) | blocker | Lambda cold start — the app can't build its verifier |
| 04 | [ECR image push hostname not routed](04-ecr-image-push-hostname-not-routed.md) | medium | `sam deploy` of an image-backed function (workaround exists) |

## What already works

Worth stating, because the list above is short and the rest is not:

- `aws cloudformation deploy` of the SAM-transformed template succeeds —
  the `fix/sam-httpapi-transform` branch does its job. 100+ resources
  including 28 routes, 28 integrations, the JWT authorizer, and the
  `$default` stage.
- DynamoDB tables, S3 bucket, ECR repo, log group, IAM role, Cognito user
  pool and client all create for real.
- The Lambda **container image** is pulled, launched, and invoked, with the
  `lambda-adapter` extension registering correctly.
- S3 presigned GET/PUT support exists (`PreSignedUrlFilter`,
  `PreSignedUrlGenerator`).
- DynamoDB `ConditionExpression`, `ConditionalCheckFailed`,
  `FilterExpression`, `ExclusiveStartKey` are all present.

The three issues are the entire distance between "deploys" and "functional
suite passes".

## Order to fix

01 and 03 are both hard blockers and independent of each other. 02 only
matters once requests are reaching the app. 04 has a workaround and is not
on the critical path, but it is what stops the standard `sam deploy` flow
from working end to end.

## Reproducing

```
cd floci-fork && ./mvnw -DskipTests package && docker build -f docker/Dockerfile -t floci-local:test .
cd kbdb && docker compose -f docker-compose.floci.yml up -d
sam build
aws --endpoint-url http://localhost:4566 cloudformation deploy \
  --template-file .aws-sam/build/template.yaml \
  --stack-name kbdb-floci --parameter-overrides SkipApiRepository=true \
  --capabilities CAPABILITY_IAM --no-fail-on-empty-changeset
```

`sam deploy` does reach floci — it honors `AWS_ENDPOINT_URL` (there is no
`--endpoint-url` flag; that env var is all LocalStack's `samlocal` wrapper
sets). It then fails pushing the container image, see issue 04. Deploying
the built artifact with `aws cloudformation deploy` sidesteps the push,
since floci runs an image-backed Lambda from a local tag.
