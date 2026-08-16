#!/usr/bin/env bash
set -euo pipefail
# Generates a throwaway RSA keypair, derives the corresponding static
# OIDC discovery document + JWKS, and publishes both to S3 at a stable
# URL - so a real deployed AWS stack's native JWT authorizer has a
# publicly-reachable issuer/jwks_uri to fetch from, without needing to
# reach back into the ephemeral GitHub Actions runner (which has no
# stable public IP/DNS - see docs/superpowers/specs/2026-08-16-workos-auth-migration-design.md's
# Testing section for the full reachability rationale).
#
# Usage: ci-publish-emulator-jwks.sh <s3-bucket> <s3-prefix> <s3-region>
# Prints: <issuer-url> <private-key-path> <kid> to stdout, space-separated,
# for the caller to capture and pass to `sam deploy` / the emulator. The
# private key file at <private-key-path> deliberately survives past this
# script's own exit (it lives outside WORKDIR, which is the only thing
# the EXIT trap removes) - the caller needs it to still exist afterward
# to mount into the emulator container; it's the caller's/CI job's
# responsibility to clean it up once it's no longer needed.

S3_BUCKET="$1"
S3_PREFIX="$2"
S3_REGION="$3"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

KEY_PATH="$(mktemp)"

openssl genrsa -out "$KEY_PATH" 2048 >/dev/null 2>&1
openssl rsa -in "$KEY_PATH" -pubout -out "$WORKDIR/key.pub.pem" >/dev/null 2>&1

# The generic https://<bucket>.s3.amazonaws.com host 307-redirects to the
# region-specific one for any bucket outside us-east-1 (confirmed live
# against the real kbdb-jwks bucket) - API Gateway's JWT authorizer won't
# follow that redirect when fetching JWKS, so the region-specific host
# must be used directly, not the generic one.
ISSUER_URL="https://${S3_BUCKET}.s3.${S3_REGION}.amazonaws.com/${S3_PREFIX}"

# Derive kid (a short, stable identifier for this run's key) and the
# JWK's n/e (RSA modulus/exponent, base64url-no-padding) from the public
# key via openssl + python3 (both already required by this repo's CI/dev
# environment).
python3 - "$WORKDIR/key.pub.pem" "$ISSUER_URL" "$WORKDIR" <<'PYEOF'
import sys, base64, json
from cryptography.hazmat.primitives import serialization

pub_path, issuer, workdir = sys.argv[1], sys.argv[2], sys.argv[3]

with open(pub_path, "rb") as f:
    pub = serialization.load_pem_public_key(f.read())
numbers = pub.public_numbers()

def b64url(i: int) -> str:
    b = i.to_bytes((i.bit_length() + 7) // 8, "big")
    return base64.urlsafe_b64encode(b).rstrip(b"=").decode()

kid = "ci-" + base64.urlsafe_b64encode(str(numbers.n)[:16].encode()).rstrip(b"=").decode()[:16]

jwks = {
    "keys": [{
        "kty": "RSA",
        "use": "sig",
        "alg": "RS256",
        "kid": kid,
        "n": b64url(numbers.n),
        "e": b64url(numbers.e),
    }]
}

discovery = {
    "issuer": issuer,
    "jwks_uri": issuer + "/jwks.json",
    "authorization_endpoint": issuer + "/authorize",
    "token_endpoint": issuer + "/token",
    "response_types_supported": ["code"],
    "subject_types_supported": ["public"],
    "id_token_signing_alg_values_supported": ["RS256"],
}

with open(f"{workdir}/jwks.json", "w") as f:
    json.dump(jwks, f)
with open(f"{workdir}/openid-configuration", "w") as f:
    json.dump(discovery, f)
with open(f"{workdir}/kid", "w") as f:
    f.write(kid)
PYEOF

KID="$(cat "$WORKDIR/kid")"

# No --acl public-read: the bucket (bootstrap/jwks-bucket.yaml) is
# BucketOwnerEnforced, which disables ACLs entirely - passing one would
# fail the call outright. Public read access comes from that bucket's own
# bucket policy, scoped to the jwks/ prefix, not per-object ACLs.
aws s3 cp "$WORKDIR/jwks.json" "s3://${S3_BUCKET}/${S3_PREFIX}/jwks.json" \
  --region "$S3_REGION" --content-type application/json >/dev/null
aws s3 cp "$WORKDIR/openid-configuration" "s3://${S3_BUCKET}/${S3_PREFIX}/.well-known/openid-configuration" \
  --region "$S3_REGION" --content-type application/json >/dev/null

echo "${ISSUER_URL} ${KEY_PATH} ${KID}"
