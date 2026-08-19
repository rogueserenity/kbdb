#!/usr/bin/env bash
set -euo pipefail

# Dev-only: generates a TypeScript client from api/openapi.yaml into a
# gitignored scratch dir, so generator output/config can be sanity-checked
# by hand. Not wired into CI - see scripts/publish-client.sh (added
# separately) for the version-stamped, published equivalent.

cd "$(dirname "${BASH_SOURCE[0]}")/.."

OUT_DIR="client"
GENERATOR_IMAGE="openapitools/openapi-generator-cli:v7.24.0"

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

docker run --rm \
  -v "$PWD:/local" \
  "$GENERATOR_IMAGE" generate \
  -i "/local/api/openapi.yaml" \
  -g typescript-fetch \
  -o "/local/$OUT_DIR" \
  --additional-properties=npmName=@rogueserenity/kbdb-api-client,supportsES6=true

echo "Generated TypeScript client at $OUT_DIR/"
