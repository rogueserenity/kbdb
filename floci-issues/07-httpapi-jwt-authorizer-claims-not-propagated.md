# HTTP API native JWT authorizer verifies the token but never propagates its claims to the Lambda

**Status:** confirmed against `floci-fork` @ `6b14999` (2026-08-15); fixed on
`floci-fork`'s `fix/httpapi-jwt-authorizer-claims-propagation` branch
(commit `4b2fedac`), verified end-to-end against kbdb's real Ginkgo
functional suite (385/385 specs, was 17/49 on Builds alone before the fix)
- not yet pushed/opened as a PR upstream
**Severity:** high — every route relying on `requestContext.authorizer.jwt.claims`
(e.g. to read the verified caller's `sub`) gets nothing, even though the
token itself was correctly verified and the request let through
**Component:** `services/apigateway/ApiGatewayExecuteController` -
`enforceJwtAuthorizer` / `buildV2ProxyEvent`
**Found against:** kbdb's real `template.yaml` deployed to floci via
`sam deploy`, with an `AWS::Serverless::HttpApi` `JWT`-type authorizer
(`WorkOSAuthorizer`) as `DefaultAuthorizer` on every route - see
`docs/superpowers/plans/2026-08-16-workos-auth-migration.md`. kbdb's
required-auth routes dropped in-process token re-verification entirely in
favor of trusting this authorizer's output alone
(`internal/middleware.RequireAuthorizerIdentity`), which is what surfaced
this - the earlier `floci-issues/README.md` "full suite passes" validation
predates that change and never actually read authorizer-populated claims
back out.

## Symptom

A request with a valid bearer token, routed through an `HttpApi` route
whose `AuthorizationType: JWT`, is let through to the Lambda (not
401/403'd) - but the Lambda's `requestContext.authorizer` is entirely
absent from the invocation event. Any handler expecting
`requestContext.authorizer.jwt.claims.sub` (the AWS-documented shape for a
native JWT authorizer - see
https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-jwt-authorizer.html)
gets nothing:

```
$ curl -s -X POST "http://localhost.floci.io:4566/execute-api/.../\$default/v1/users/<uid>/switches" \
    -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{...}' -i
HTTP/1.1 500 Internal Server Error
{"type":"...","title":"Internal Server Error","status":500,"detail":"server misconfiguration"}
```

The Lambda's own logs confirm it never saw an identity:

```
{"level":"ERROR","msg":"required-auth route reached with no verified identity from API Gateway authorizer", ...}
```

floci's own logs show zero JWT-authorizer log lines for the request at
all - no rejection, no claims extraction, nothing. The token was accepted
silently.

## Root cause (confirmed by reading the source, not just observed behavior)

`ApiGatewayExecuteController#enforceJwtAuthorizer` (used for `HttpApi` v2
routes with `AuthorizationType: JWT`) does real verification work - it
extracts the bearer token, parses its claims, and checks `exp`/`iss`/`aud`
against the authorizer's configured `JwtConfiguration`, returning a 401
`Response` if any check fails. On success it returns `null` ("authorized,
proceed") - **but the parsed claims are discarded at that point.** Nothing
downstream ever sees them.

Compare with the sibling `Lambda REQUEST` authorizer path
(`enforceRequestAuthorizerV2`, invoked for `AuthorizationType: CUSTOM`),
which explicitly builds a `principalId`/`context` map from the authorizer
Lambda's response and threads it through `AuthorizerResult` so it later
gets written into the outgoing Lambda event's
`requestContext.authorizer` node (see `invokeAuthorizer`'s
`extractAuthorizerContext` and `authorizerNode.put(...)` calls, used by the
v1/REST dispatch path). The v2 native-JWT path has no equivalent - `null`
is the only signal `enforceJwtAuthorizer` returns, so there is no claims
payload for anything to attach.

Confirmed by reading `buildV2ProxyEvent` (the function that actually
serializes the event sent to the Lambda for `HttpApi` v2 routes): its
`requestContext` object (`ctx`) never gets an `authorizer` sub-object
added anywhere in the function, unconditionally - even in the success
case.

Also worth noting: even if the claims *were* threaded through,
`JwtClaims` (the record `parseJwtClaims` returns) only captures
`iss`/`aud`/`client_id`/`exp` - the fields needed for the authorizer's own
verification, not `sub` or the full claim set a downstream Lambda would
need. A real fix needs both: retain the full parsed claims (or at least
`sub`), and actually attach them to the v2 proxy event's
`requestContext.authorizer.jwt.claims`.

## What already works

- Issuer/audience/expiry verification itself is correct - a token with a
  wrong issuer or audience, or an expired one, is correctly rejected with
  401 by `enforceJwtAuthorizer`. Only the success path silently drops
  information.
- The `Authorizer` CloudFormation resource (`AWS::ApiGatewayV2::Authorizer`
  / SAM's `HttpApi.Auth.Authorizers`) is created and stored correctly -
  `WorkOSAuthorizer` shows up in floci's logs at deploy time
  (`Created authorizer: WorkOSAuthorizer (...) for API ...`), and its
  `JwtConfiguration` (issuer/audience) is exactly what
  `enforceJwtAuthorizer` reads back and enforces.
- The `Lambda REQUEST`/`CUSTOM` authorizer path (`enforceRequestAuthorizerV2`)
  does correctly build and propagate a context - this is specifically a
  gap in the native `JWT` authorizer type, not authorizer context
  propagation in general.

## Consumer-side impact (kbdb)

Every `RequireAuthorizerIdentity`-guarded write route (create/update/delete
on switches, keyboards, keycap sets, builds, plus build images) 500s
locally against an unpatched floci - `scripts/func-setup.sh`'s deploy and
token-minting work correctly, but `scripts/func-test.sh`'s Builds/Switches/
Keyboards/KeycapSets suites all fail on every write-path spec. Read-only
and `OptionalAuth` routes (which don't depend on
`requestContext.authorizer.jwt.claims`) pass normally.

## The fix

`enforceJwtAuthorizer` now returns a `JwtAuthorizerResult` record (mirroring
the existing `AuthorizerResult` used by the v1/REST `CUSTOM`-authorizer
path) carrying the parsed claims - flattened to strings, every claim
retained (not just `iss`/`aud`/`client_id`/`exp`) - instead of a bare
`Response`. `dispatchV2` threads those claims through `buildV2ProxyEvent`'s
new `Map<String, String> jwtClaims` parameter, which attaches
`requestContext.authorizer.jwt.claims` when non-empty. A new
`BuildV2ProxyEventJwtClaimsTest` covers the populated/null/empty cases; the
existing `buildV2ProxyEvent` call sites keep working unchanged via a
backward-compatible overload.

Verified by building the patched `docker/Dockerfile` image locally,
deploying kbdb's real `template.yaml` to it via `sam deploy`
(`scripts/func-setup.sh`), and running the full Ginkgo suite
(`scripts/func-test.sh`): **385/385 specs pass across all 10 suites** - the
same suites that were failing on every write-path spec before the patch.

Not yet pushed to `origin` or opened as a PR against `floci-io/floci`
upstream - the branch (`fix/httpapi-jwt-authorizer-claims-propagation`)
exists locally on the fork only, pending review.
