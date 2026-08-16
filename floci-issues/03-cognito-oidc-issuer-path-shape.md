# Cognito OIDC issuer: reachable by config, but not over the AWS-shaped HTTPS URL

**Severity:** low for floci — mostly a consumer configuration matter
**Component:** `services/cognito` / docs
**Found against:** `rogueserenity/floci` @ `fix/sam-httpapi-transform`

## Correction to the first version of this issue

The original write-up claimed floci served OIDC discovery at the wrong path
and that a template using the AWS-documented issuer URL could not work.
That was wrong, and it understated floci's existing support.

floci already has the machinery: `FLOCI_HOSTNAME` rewrites the host in
returned URLs (equivalent to `LOCALSTACK_HOSTNAME`), and
`FLOCI_DNS_EXTRA_SUFFIXES` adds hostname suffixes to the embedded DNS
resolver. Together those solve the dual-endpoint problem — one hostname
that resolves correctly from both the host and sibling containers.

## What works today, with config only

Setting `FLOCI_HOSTNAME=localhost.floci.io` makes the issuer resolvable
from both sides — verified, 200 from each:

```
issuer: http://localhost.floci.io:4566/us-east-2_516997a00

from the HOST               discovery -> 200
from a SIBLING container    discovery -> 200
```

Adding `FLOCI_DNS_EXTRA_SUFFIXES=amazonaws.com` goes further and points the
real AWS Cognito hostname at floci, so a template hardcoding

```yaml
OIDC_ISSUER_URL: !Sub https://cognito-idp.${AWS::Region}.amazonaws.com/${UserPool}
```

reaches floci rather than the internet. Confirmed by the error changing
from a 404 to a connection attempt against floci's own IP.

## The one genuine remaining gap

That AWS-shaped URL is `https://`, and floci serves plain HTTP on 4566:

```
initializing token verifier: auth: fetching OIDC provider metadata:
Get "https://cognito-idp.us-east-2.amazonaws.com/us-east-2_58565e500/.well-known/openid-configuration":
dial tcp 172.18.0.3:443: connect: connection refused
```

So an unmodified AWS template still cannot initialize an OIDC client.
`FLOCI_TLS_ENABLED=true` with `FLOCI_TLS_SELF_SIGNED=true` would open 443,
but then the client has to trust a self-signed certificate — Go's
`go-oidc`/`crypto/tls` will reject it unless the CA is installed in the
Lambda container's trust store. That is a consumer-side problem floci
cannot fully solve.

## What would actually help

Not a bug fix so much as two smaller things:

1. **Serve TLS on 443 as well as the configured port when TLS is enabled**,
   so `https://cognito-idp.{region}.amazonaws.com/...` works without a port
   suffix. Today TLS applies to the single configured port, so the AWS
   hostname's implicit 443 is unreachable.
2. **Document the combination.** `FLOCI_HOSTNAME` +
   `FLOCI_DNS_EXTRA_SUFFIXES` is the answer to "how do I use an issuer URL
   from both the host and a Lambda container", and neither option's docs
   mention OIDC or the pairing. Both were found by reading
   `EmulatorConfig.java`, not the docs.

## Consumer-side note (kbdb)

The practical fix on our side is a template parameter overriding
`OIDC_ISSUER_URL` for local runs — which is exactly what the stashed
`OidcIssuerBaseUrl` work did. With `FLOCI_HOSTNAME=localhost.floci.io` set,
pointing that at `http://localhost.floci.io:4566` needs no floci change at
all.

So this is **not a floci blocker**. It is a kbdb template change plus two
documentation/TLS improvements worth suggesting upstream.
