# Lambda proxy response Content-Type lost when the header name isn't exact-case

**Status:** fixed upstream — merged as [floci-io/floci#2150](https://github.com/floci-io/floci/pull/2150)
**Severity:** high — every error response (and any handler that lowercases
header names) loses its real Content-Type
**Component:** `services/apigateway/ApiGatewayExecuteController#buildProxyResponse`
**Found against:** `rogueserenity/floci` @ `fix/sam-httpapi-transform`, while
verifying the fix for floci-issues/01

## Symptom

A Lambda proxy integration (v1 REST or v2 HTTP API, `AWS_PROXY`) that
returns a body with `Content-Type: application/problem+json` gets
`application/json` back from floci instead — silently, no error, wrong
header only:

```
$ curl -s -D - "http://localhost:4566/execute-api/.../v1/users/x/keyboards/nope" \
    -H "Authorization: Bearer $TOKEN"
HTTP/1.1 404 Not Found
content-type: application/json          <- wrong

{"type":"https://mykeebs.info/errors/not-found","title":"Not Found","status":404,...}
```

The body is exactly right; only the header is wrong. This broke 19 of 36
specs in kbdb's `keyboards` suite alone, and reproduced identically across
every other suite once actually run (`--keep-going` showed all 7 suites
failing, not just the one `set -euo pipefail` happened to stop on first).

## Root cause

`buildProxyResponse` reads the Content-Type back out of the Lambda's JSON
response with an exact-case field lookup:

```java
JsonNode ctNode = node.path("headers").path("Content-Type");
if (!ctNode.isMissingNode() && !ctNode.isNull()) ct = ctNode.asText();
```

Confirmed via temporary debug logging what the Lambda actually returns for
kbdb (fronted by the AWS Lambda Web Adapter sidecar, which translates a real
`net/http` response back into proxy-response JSON):

```json
{"statusCode":404,"headers":{"content-type":"application/problem+json", ...
```

Lowercase `content-type`. HTTP header names are case-insensitive on the wire
(RFC 7230 §3.2) — the adapter is not doing anything wrong. `JsonNode#path`
is an exact-case JSON field lookup, so `.path("Content-Type")` misses it and
falls through to the `application/json` default. Meanwhile the *other* code
path just above it, which copies every response header onto the JAX-RS
builder (`respHeaders.fields().forEachRemaining(e -> builder.header(...))`),
copies `content-type` (lowercase) correctly — and then `builder.type(ct)`
unconditionally overwrites it with the wrong-cased lookup's `application/json`
fallback.

## Fix

Case-insensitive scan instead of an exact-case JSON path lookup:

```java
String ct = findHeaderIgnoreCase(respHeaders, "Content-Type").orElse(MediaType.APPLICATION_JSON);
builder.entity(bytes).type(ct);
```

`findHeaderIgnoreCase` iterates `respHeaders`' fields and matches with
`equalsIgnoreCase`. Verified: full kbdb functional suite (7 suites, 211
specs) passes cleanly after this change, having failed with exactly this
symptom before it.

## Scope note

Only `buildProxyResponse`'s Content-Type re-derivation was exact-case; the
bulk header-copy loops (both single- and multi-value) already iterate all
keys and are inherently case-preserving/insensitive-safe. No other header
name is special-cased the same way, so this was the only site with the bug.
