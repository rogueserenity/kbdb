# HTTP API (v2) execute-api returns "Invalid API id specified" for unsigned requests

**Status:** fixed upstream — merged as [floci-io/floci#2146](https://github.com/floci-io/floci/pull/2146) and [#2286](https://github.com/floci-io/floci/pull/2286)
**Severity:** blocker — no HTTP API can be invoked unless it happens to live in `floci.default-region`
**Component:** `services/apigateway/ApiGatewayExecuteController`, `services/apigatewayv2/ApiGatewayV2Service`
**Found against:** `rogueserenity/floci` @ `fix/sam-httpapi-transform`

## Confirmed on re-verification

Re-tested from scratch (fresh floci, fresh `sam deploy`, region `us-east-2` vs
`floci.default-region`'s default of `us-east-1`) after two earlier issues
filed in this same pass (03, 04) turned out to be misdiagnoses. This one held
up: unsigned requests reproduced 404 consistently across repeated fresh runs,
and — critically — **so did requests carrying a real Cognito bearer JWT**,
which is the actual shape of every request kbdb's own functional suite sends.
The original write-up below only tested a fake SigV4-shaped header; a real
bearer token also fails to match `RegionResolver`'s `Credential=` pattern and
silently falls back to the default region exactly like a blank header does.
`RegionResolver.isRegionUnresolved` (in the fix) checks for that case too.

## Symptom

A deployed `AWS::Serverless::HttpApi` cannot be invoked. Every documented
execute path returns 404:

```
$ curl -s http://localhost:4566/execute-api/1bb284cc19/\$default/v1/lookups
{"message":"Invalid API id specified"}
```

…even though the API, its `$default` stage, its routes, and its authorizer
all exist and are readable via the control plane:

```
$ aws --endpoint-url http://localhost:4566 apigatewayv2 get-stages --api-id 1bb284cc19
$default
$ aws --endpoint-url http://localhost:4566 apigatewayv2 get-routes --api-id 1bb284cc19
GET /v1/users/{userId}/switches/{switchId}
... 28 routes ...
```

## Root cause

`ApiGatewayExecuteController.dispatch` decides v1-vs-v2 by attempting a v2
lookup, using a region resolved from request headers:

```java
String region = regionResolver.resolveRegion(headers);

boolean isV2 = false;
try {
    apiGatewayV2Service.getApi(region, apiId);
    isV2 = true;
} catch (AwsException ignored) {
    // Not a v2 API — fall through to v1 handling
}
```

An unsigned data-plane request (a browser, `curl`, or any HTTP client that
isn't SigV4-signing) carries no region. `resolveRegion` falls back to a
default that need not match where the API was created, the v2 lookup
misses, and control falls through to v1 — which also misses, producing the
misleading "Invalid API id specified".

The v1 path already compensates for exactly this, a few lines below:

```java
String auth = headers.getHeaderString("Authorization");
if (auth == null || auth.isBlank()) {
    region = apiGatewayService.resolveRestApiRegion(region, apiId);
}
```

There is no v2 equivalent, so v2 never gets the same second chance.

## Proof

Supplying a SigV4-shaped `Authorization` header that names the region
changes the outcome — the API is found and dispatched (the 502 is a
separate, unrelated failure, see issue 03):

```
$ curl -o /dev/null -w '%{http_code}\n' \
    "http://localhost:4566/execute-api/1bb284cc19/\$default/v1/lookups"
404

$ curl -o /dev/null -w '%{http_code}\n' \
    -H "Authorization: AWS4-HMAC-SHA256 Credential=test/20260806/us-east-2/execute-api/aws4_request, SignedHeaders=host, Signature=x" \
    "http://localhost:4566/execute-api/1bb284cc19/\$default/v1/lookups"
502
```

## Fix

Implemented and verified end-to-end (full kbdb functional suite — 7 Ginkgo
suites, 211 specs — passes against a real `sam deploy` onto this build):

1. `ApiGatewayV2Service.resolveHttpApiRegion(preferredRegion, apiId)` —
   mirrors `ApiGatewayService#resolveRestApiRegion` exactly: try the
   preferred region first, else scan stored keys for one ending in
   `::apiId` (the same `region::apiId` key shape v1 already relies on).
2. `RegionResolver.isRegionUnresolved(headers)` — the actual signal
   `dispatch` needs is not "was this request unsigned" but "did region
   resolution find a real region or silently fall back to the default."
   A blank/missing `Authorization` header and a present-but-non-SigV4 one
   (e.g. a bearer JWT) both hit the same silent fallback in
   `resolveRegionFromAuth`, so both need the same escape hatch.
3. `ApiGatewayExecuteController.dispatch` — uses `isRegionUnresolved` (not
   header-blankness) to decide whether to call `resolveHttpApiRegion`
   before the v2 `getApi` lookup, and the same flag for the existing v1
   fallback below it (previously its own separate blank-header check).

See the diff on this branch for the full change (3 files, ~50 lines).

## Why it matters

This makes HTTP APIs effectively unusable from any normal HTTP client.
Real API Gateway serves `https://{apiId}.execute-api.{region}.amazonaws.com`
without requiring the caller to sign anything, so any test suite or browser
hitting a deployed HTTP API hits this.
