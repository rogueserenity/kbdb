#!/usr/bin/env bash
set -euo pipefail

mockery
go tool oapi-codegen -config .oapi-codegen.yml api/openapi.yaml
go mod tidy
