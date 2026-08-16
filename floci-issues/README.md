# floci issues blocking kbdb's functional suite

Found by deploying kbdb's real `template.yaml` to a local floci build, then
running the Ginkgo functional suite against it.

Each file is scoped to one PR against floci.

| # | issue | status | blocks |
|---|---|---|---|
| 01 | [HTTP API execute-api region resolution](01-httpapi-execute-region-resolution.md) | fixed upstream — merged in [floci-io/floci#2146](https://github.com/floci-io/floci/pull/2146) and [#2286](https://github.com/floci-io/floci/pull/2286) | every request whose region can't be read from a SigV4 header — includes all bearer-JWT requests |
| 02 | [UserPoolGroup silently stubbed](02-cognito-userpoolgroup-silently-stubbed.md) | real gap, but moot for kbdb — no admin-group requirement anymore | nothing currently; would block admin-group specs if kbdb re-adds one |
| 03 | [Cognito OIDC issuer over HTTPS](03-cognito-oidc-issuer-path-shape.md) | not a floci blocker | nothing — solved with `FLOCI_HOSTNAME` + a kbdb template parameter |
| 04 | [RETRACTED — ECR push](04-ecr-image-push-hostname-not-routed.md) | n/a | not a floci bug; `sam deploy --resolve-image-repos` ignores floci's repositoryUri |
| 05 | [UNCONFIRMED — GetTemplateSummary](05-cloudformation-gettemplatesummary-unsupported.md) | n/a | observed once, not reproducible on retest — not filed upstream |
| 06 | [Lambda proxy Content-Type case sensitivity](06-lambda-proxy-response-content-type-case-sensitive.md) | fixed upstream — merged in [floci-io/floci#2150](https://github.com/floci-io/floci/pull/2150) | every error response and any handler returning lowercase header names |
| 07 | [HTTP API JWT authorizer claims not propagated](07-httpapi-jwt-authorizer-claims-not-propagated.md) | fixed on `floci-fork`, not yet pushed/PR'd upstream | every `RequireAuthorizerIdentity`-guarded write route — floci verified the JWT correctly but never attached its claims to the Lambda's `requestContext.authorizer` |

## Current state: full functional suite passes, with a local floci-fork patch for issue 07

01, 06, and (locally) 07 are all resolved. kbdb's required-auth routes rely
entirely on `requestContext.authorizer.jwt.claims` (no in-process
re-verification any more - see the WorkOS migration note below), which
issue 07 showed floci never populated for native `JWT`-type authorizers.
That's fixed on `floci-fork`'s `fix/httpapi-jwt-authorizer-claims-propagation`
branch (not yet pushed/opened as a PR) - **building floci locally from that
branch is currently required** for the full suite to pass; the public
`floci/floci:nightly` image does not yet contain this fix.

```
REST Builds Suite       - 49/49 specs   PASS
REST Keyboards Suite    - 41/41 specs   PASS
REST Keycap Sets Suite  - 79/79 specs   PASS
REST Lookups Suite      -  3/3  specs   PASS
REST Switches Suite     - 41/41 specs   PASS
MCP Builds Suite        - 40/40 specs   PASS
MCP Keyboards Suite     - 29/29 specs   PASS
MCP Keycap Sets Suite   - 69/69 specs   PASS
MCP Lookups Suite       -  6/6  specs   PASS
MCP Switches Suite      - 28/28 specs   PASS
```

(Verified 2026-08-16 against a local `docker/Dockerfile` build of
`floci-fork` @ `4b2fedac` (`fix/httpapi-jwt-authorizer-claims-propagation`),
run against kbdb's real WorkOS-based `template.yaml` - 385/385 specs across
all 10 suites. Before the issue-07 patch, the same run passed 17/49 on
Builds alone, with every write-path spec 500ing.)

01 and 06 were genuine floci bugs, fixed and merged upstream. 07 is fixed
on the fork, pending upstream push/PR. 02 is a genuine floci gap too, but
kbdb no longer has an admin-group requirement to expose it — `template.yaml`
doesn't declare `AWS::Cognito::UserPoolGroup` and no functional spec
exercises an admin token, so it's left filed upstream-only, not worked
around. 03 was a misdiagnosis in its original form — no floci change
needed, just a kbdb template parameter plus `FLOCI_HOSTNAME`. 04 and 05
were misdiagnoses, retracted rather than deleted.

As of 2026-08-15, neither fix has shipped in a tagged floci release yet
(latest is `1.6.0`, from 2026-08-06 — see `floci-io/floci`'s release list).
`floci/floci:latest` on Docker Hub therefore does *not* yet contain either
fix. floci also publishes `floci/floci:nightly`, built from `upstream/main`
on a schedule, which does contain both fixes — use that until a versioned
release tag is cut, instead of building floci locally.

## What already works

- `sam deploy` (not just `aws cloudformation deploy`) against floci, image
  push included, once `--image-repositories` + `FLOCI_SERVICES_ECR_URI_STYLE=path`
  are used instead of `--resolve-image-repos` (issue 04's retraction).
- DynamoDB tables, S3 bucket, ECR repo, log group, IAM role all create for
  real (at the time of the run above, so did a Cognito user pool and
  client - kbdb's template no longer declares those, see the WorkOS
  migration note above).
- The Lambda **container image** is pulled, launched, and invoked, with the
  `aws-lambda-web-adapter` sidecar registering and forwarding correctly.
- S3 presigned GET/PUT support exists (`PreSignedUrlFilter`,
  `PreSignedUrlGenerator`).
- The embedded DNS server plus `FLOCI_HOSTNAME` genuinely solve the
  dual-endpoint problem: with `FLOCI_HOSTNAME=localhost.floci.io` an issuer
  hosted on floci's own S3 emulation returns 200 from both the host and a
  sibling container (originally verified against the Cognito issuer; the
  same DNS/hostname mechanism is provider-agnostic).
- DynamoDB `ConditionExpression`, `ConditionalCheckFailed`,
  `FilterExpression`, `ExclusiveStartKey` are all present.

## Reproducing (current, working setup)

Until issue 07's fix is pushed upstream and lands in a public floci image,
build floci locally from the fork branch first:

```
cd floci-fork
git checkout fix/httpapi-jwt-authorizer-claims-propagation
docker build -f docker/Dockerfile -t floci/floci:nightly .
```

(Tagging the local build as `floci/floci:nightly` is what lets
`docker-compose.floci.yml`'s existing `image: floci/floci:nightly` pick it
up without any compose file changes - Docker resolves an already-present
local tag without hitting the registry.)

Then, from kbdb:

```
cd kbdb
mise run func-setup   # deploys, mints tokens, prints exports
# export the printed KBDB_* vars, then:
mise run func-test
```

Once issue 07's fix is pushed to `floci-fork`'s `origin` and/or merged
upstream and included in a public `floci/floci:nightly` build, the manual
local build step above goes away and `docker compose -f
docker-compose.floci.yml pull floci` (to refresh the moving `nightly` tag)
is enough again - `func-setup.sh` doesn't need to change either way, since
it doesn't build or pull the image itself.
