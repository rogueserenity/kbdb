# `sam deploy` can't push an image: the ECR hostname isn't routed to the registry

**Severity:** medium — blocks `sam deploy` for image-backed functions; a
workaround exists
**Component:** `services/ecr`
**Found against:** `rogueserenity/floci` @ `fix/sam-httpapi-transform`

## Symptom

`sam deploy` against floci authenticates to ECR, creates the repository,
and begins pushing layers — then fails on the first blob:

```
$ AWS_ENDPOINT_URL=http://localhost:4566 sam deploy \
    --stack-name kbdb-floci --resolve-s3 --resolve-image-repos \
    --parameter-overrides SkipApiRepository=true

Error: Unable to upload artifact apifunction:latest referenced by ImageUri
parameter of ApiFunction resource.
unknown: unexpected status from HEAD request to
https://000000000000.dkr.ecr.us-east-2.amazonaws.com/v2/kbdbflocisamdeploy9720acff/apifunction51503098repo/blobs/sha256:d0e1...:
400 Bad Request
```

The control plane worked — floci logs show the repo created and the auth
token issued:

```
INFO [EcrService] Created ECR repository us-east-2/000000000000/kbdbflocisamdeploy9720acff/apifunction51503098repo
INFO [AwsJson11Controller] AwsJson11Controller ecr action: GetAuthorizationToken
```

## Root cause

floci runs the OCI registry as a **sibling container**
(`floci-ecr-registry`, `registry:2`), published on host port 5100:

```
$ docker ps --format '{{.Names}}\t{{.Ports}}'
floci-ecr-registry   0.0.0.0:5100->5000/tcp
kbdb-floci-1         0.0.0.0:4566->4566/tcp
```

The registry is healthy and reachable there:

```
$ curl -o /dev/null -w '%{http_code}\n' http://localhost:5100/v2/
200
```

But the main endpoint does not serve the registry API:

```
$ curl -o /dev/null -w '%{http_code}\n' http://localhost:4566/v2/
404
```

`GetAuthorizationToken` returns a `proxyEndpoint` of
`https://{account}.dkr.ecr.{region}.amazonaws.com`, and the Docker client
dutifully sends blob requests there. Nothing routes that hostname to
`localhost:5100`, so the push dies on the first blob HEAD.

The registry logs confirm blob traffic never arrives — only the initial
`GET /v2/` ping:

```
$ docker logs floci-ecr-registry | grep -E 'blobs|PATCH|PUT'
(nothing)
```

## Why this is a real gap

The README advertises this as supported:

| ECR | In-process with real registry | Repositories, **docker push / pull**, image-backed Lambda functions |

Pushing works if you address `localhost:5100` directly. It does not work
through the ECR hostname the service's own `GetAuthorizationToken` hands
back, which is the path every standard tool takes — `sam deploy`, `docker
push` after `aws ecr get-login-password`, and CI image builds alike.

## Suggested fix

Either:

1. **Proxy `/v2/*` on the main endpoint** to the registry container, so the
   advertised `proxyEndpoint` resolves somewhere real when DNS points the
   ECR hostname at floci (the same approach the embedded DNS server already
   takes for `localhost.localstack.cloud` and S3 presigned URLs), or
2. **Return a reachable `proxyEndpoint`** from `GetAuthorizationToken` —
   e.g. `localhost:5100` — so clients are told where the registry actually
   is rather than an AWS hostname that goes nowhere.

Option 2 is smaller; option 1 keeps templates and tooling unmodified, which
is more consistent with how floci handles S3.

## Workaround

Skip the push. Build the image locally and deploy the SAM-built artifact
directly with `aws cloudformation deploy` — floci runs an image-backed
Lambda from a local tag without needing it in the registry (verified: the
container launched and the handler executed).

```
sam build
aws --endpoint-url http://localhost:4566 cloudformation deploy \
  --template-file .aws-sam/build/template.yaml --stack-name kbdb-floci \
  --parameter-overrides SkipApiRepository=true --capabilities CAPABILITY_IAM
```

## Correction to an earlier assumption

`sam deploy` has no `--endpoint-url` flag, which initially looked like it
couldn't target floci at all. That is wrong: it honors `AWS_ENDPOINT_URL`
via botocore, which is the whole of what LocalStack's `samlocal` wrapper
sets. `sam deploy` reaches floci fine — this issue is the next thing it
hits.
