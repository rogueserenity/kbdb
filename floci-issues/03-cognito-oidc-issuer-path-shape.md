# Cognito OIDC discovery is not served at the real AWS issuer path

**Severity:** blocker — any OIDC client configured the way AWS documents
fails to initialize
**Component:** `services/cognito`
**Found against:** `rogueserenity/floci` @ `fix/sam-httpapi-transform`

## Symptom

A Lambda that verifies Cognito JWTs cannot start. It resolves the issuer
URL exactly as AWS documents, fetches the discovery document, and gets a
404:

```
{"level":"INFO","msg":"initializing token verifier: auth: fetching OIDC provider metadata:
 404 Not Found: {\"message\":\"User pool us-east-2_874a40c50 does not exist.\"}"}
```

The pool does exist — floci created it, and the control plane returns it:

```
$ aws --endpoint-url http://localhost:4566 cognito-idp describe-user-pool \
    --user-pool-id us-east-2_874a40c50 --query UserPool.Id --output text
us-east-2_874a40c50
```

## Root cause

Real Cognito serves OIDC discovery at

```
https://cognito-idp.{region}.amazonaws.com/{poolId}/.well-known/openid-configuration
```

and that same URL is the `iss` claim of every token it mints. AWS's own
docs tell you to configure clients with it, and SAM templates conventionally
build it with

```yaml
OIDC_ISSUER_URL: !Sub https://cognito-idp.${AWS::Region}.amazonaws.com/${UserPool}
```

floci serves the document only at `/{poolId}/...`:

```
$ curl -o /dev/null -w '%{http_code}\n' \
    http://localhost:4566/us-east-2_874a40c50/.well-known/openid-configuration
200

$ curl -o /dev/null -w '%{http_code}\n' \
    http://localhost:4566/cognito-idp/us-east-2_874a40c50/.well-known/openid-configuration
404
```

and advertises a correspondingly different issuer:

```json
{
  "issuer":   "http://localhost:4566/us-east-2_874a40c50",
  "jwks_uri": "http://localhost:4566/us-east-2_874a40c50/.well-known/jwks.json"
}
```

## Why the shape matters, not just the host

This is not merely "point the client somewhere else". OIDC discovery
requires the `issuer` in the document to match the URL the client was
configured with — `go-oidc`'s `oidc.NewProvider` enforces this, as do most
conforming libraries. So a client cannot simply be pointed at
`http://localhost:4566/{poolId}` while tokens carry a different `iss`, nor
vice versa; the path shape and the advertised issuer have to agree with
what the client was told.

The practical consequence is that a template written for real AWS cannot be
deployed unmodified. Every consumer has to introduce a
floci-specific issuer override, which defeats the purpose of deploying the
real template.

## Suggested fix

Serve the discovery document (and JWKS) additionally at the AWS-shaped
path:

```
/cognito-idp/{poolId}/.well-known/openid-configuration
/cognito-idp/{poolId}/.well-known/jwks.json
```

and set `issuer` to match whichever host the request arrived on, the way
the S3 presigned-URL support already adapts to the requesting host. That
would let a stock `!Sub https://cognito-idp.${AWS::Region}.amazonaws.com/${UserPool}`
template work unchanged when DNS for that host is pointed at floci — which
is already how floci's embedded DNS handles `localhost.localstack.cloud`
and `localhost.floci.io`.

## Note

This is the same class of problem as the S3 presigned-URL host issue floci
already solves architecturally with its embedded DNS server. Applying the
same approach to the Cognito issuer would close it consistently.
