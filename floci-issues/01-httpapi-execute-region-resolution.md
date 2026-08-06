# HTTP API (v2) execute-api returns "Invalid API id specified" for unsigned requests

**Severity:** blocker — no HTTP API can be invoked over an unsigned request
**Component:** `services/apigateway/ApiGatewayExecuteController`
**Found against:** `rogueserenity/floci` @ `fix/sam-httpapi-transform`

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

## Suggested fix

Mirror the v1 fallback for v2: when the request is unsigned, resolve the
region by searching for the API id across regions before concluding it
isn't a v2 API. Something equivalent to a `resolveHttpApiRegion` alongside
the existing `resolveRestApiRegion`.

A narrower alternative: attempt the v2 lookup across all known regions
rather than only the resolved one, since `apiId` is globally unique in
practice within a single floci instance.

## Why it matters

This makes HTTP APIs effectively unusable from any normal HTTP client.
Real API Gateway serves `https://{apiId}.execute-api.{region}.amazonaws.com`
without requiring the caller to sign anything, so any test suite or browser
hitting a deployed HTTP API hits this.
