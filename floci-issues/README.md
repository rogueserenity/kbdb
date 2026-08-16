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

## Current state: full functional suite passes

With 01 and 06 fixed upstream (both merged into `floci-io/floci`'s `main`,
both verified against a real `sam deploy` of kbdb's actual `template.yaml`)
and 02 no longer applicable (kbdb has no admin-group requirement), kbdb's
entire Ginkgo functional suite passes:

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

(Verified 2026-08-15 against floci built from `floci-io/floci`'s `upstream/main` —
385/385 specs across all 10 suites. Counts have grown since the original pass
above the table was written; kbdb has since added Builds, REST and MCP.)

01 and 06 were genuine floci bugs, both fixed and merged upstream. 02 is a
genuine floci gap too, but kbdb no longer has an admin-group requirement to
expose it — `template.yaml` doesn't declare `AWS::Cognito::UserPoolGroup`
and no functional spec exercises an admin token, so it's left filed
upstream-only, not worked around. 03 was a misdiagnosis in its original
form — no floci change needed, just a kbdb template parameter plus
`FLOCI_HOSTNAME`. 04 and 05 were misdiagnoses, retracted rather than deleted.

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
- DynamoDB tables, S3 bucket, ECR repo, log group, IAM role, Cognito user
  pool and client all create for real.
- The Lambda **container image** is pulled, launched, and invoked, with the
  `aws-lambda-web-adapter` sidecar registering and forwarding correctly.
- S3 presigned GET/PUT support exists (`PreSignedUrlFilter`,
  `PreSignedUrlGenerator`).
- The embedded DNS server plus `FLOCI_HOSTNAME` genuinely solve the
  dual-endpoint problem: with `FLOCI_HOSTNAME=localhost.floci.io` the
  Cognito issuer returns 200 from both the host and a sibling container.
- DynamoDB `ConditionExpression`, `ConditionalCheckFailed`,
  `FilterExpression`, `ExclusiveStartKey` are all present.
- Real Cognito admin APIs (`admin-create-user`, `admin-set-user-password`,
  `admin-initiate-auth`) work well enough that kbdb's own
  `scripts/ci-create-test-user.sh` runs unmodified against floci.

## Reproducing (current, working setup)

```
cd kbdb
docker compose -f docker-compose.floci.yml pull floci   # refresh the nightly tag
docker compose -f docker-compose.floci.yml up -d
bash scripts/func-setup-floci.sh   # deploys, mints tokens, prints exports
# export the printed KBDB_* vars, then:
bash scripts/func-test.sh
```

`docker-compose.floci.yml` pulls `floci/floci:nightly`, which floci builds
from `upstream/main` on a schedule and already contains both #2146 and
#2150. Because it's a moving tag, `docker compose pull` before `up -d` to
avoid running against a stale cached image. Once floci-io/floci cuts a
versioned release tag containing both fixes, switch to that pinned tag
instead of `nightly`.
