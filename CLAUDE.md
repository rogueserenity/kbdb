# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

kbdb is a keyboard collection database (keyboards, switches, keycap sets, assembled builds), migrating from a set of linked Notion databases. The primary interface is an MCP server (AI chat), with REST as a secondary interface — both share the same service/repository layer. A web UI may come later. It's built to be multi-user and eventually community-facing, though the current phase ships fully private, siloed-per-user data.

The project is being built issue-by-issue against a fixed architecture (see below); GitHub issues in this repo are the authoritative source of what's scoped/in-progress/deferred. Check open issues before assuming something is unimplemented or undecided.

## Commands

```sh
# Build / test / vet (mise-managed Go toolchain — run `mise install` once per machine)
go build ./...
go vet ./...
go test ./...
go test ./... -run TestVerifyTokenSuite -v   # single suite
go test ./internal/auth/... -v               # single package

# Regenerate mocks after changing an interface in internal/auth (or adding new ones to .mockery.yml)
mockery

# SAM (validate / build / deploy the Lambda + API Gateway + Cognito + Budget stack)
sam validate --lint
sam build
AWS_PROFILE=kbdb-admin sam deploy --image-repositories ApiFunction=<account>.dkr.ecr.<region>.amazonaws.com/kbdb-api
```

Deploys use the `kbdb-admin` AWS IAM Identity Center (SSO) profile, not default credentials — set up once via `aws configure sso`, refreshed via `aws sso login --profile kbdb-admin` when the session expires. There is no default/implicit profile wired up; commands against real AWS will fail with `NoCredentials` without `AWS_PROFILE=kbdb-admin` (or `--profile kbdb-admin`).

`sam build` requires Docker running locally — `ApiFunction` is packaged as a container image, not a zip (see Architecture below). On macOS with Docker Desktop, `docker-credential-desktop` must be on `PATH` or `sam build`/`docker push` fail with a credential-store error; add `/Applications/Docker.app/Contents/Resources/bin` to `PATH` if so.

**First deploy to a new AWS account/stack**: the ECR repo (`ApiRepository`) must exist before `sam deploy` can push an image to it, but the same template also declares that repo as a CloudFormation resource — a chicken-and-egg problem with no first-party SAM fix. Bootstrap once via `aws ecr create-repository --repository-name kbdb-api`, push an initial image, then import the repo into the stack via a CloudFormation `IMPORT` changeset (`aws cloudformation create-change-set --change-set-type IMPORT --resources-to-import ...`) before running a normal `sam deploy`. After that one-time bootstrap, ordinary `sam deploy` calls work.

## Architecture

**Stack**: Go, AWS Lambda (container image), API Gateway (HTTP API), Cognito, DynamoDB (Phase 1, not yet added), AWS SAM for IaC. mcp-go for the MCP layer (Phase 0.5, not yet added — issue #6).

**`template.yaml` also declares an `AWS::Budgets::Budget`** (`CostBudget`) as a cost tripwire — this whole stack is designed to run at effectively $0 on AWS free-tier usage, so a real bill appearing is itself a signal something is wrong (e.g. runaway Lambda invocations, an accidental non-free resource). It emails `aws-budget@rogueserenity.dev` at 80% and 100% of a small monthly threshold. Don't remove it when touching the template, and raise the threshold deliberately if the project's real usage/cost profile changes rather than deleting it.

**DynamoDB (Phase 1) was chosen over PostgreSQL** after evaluating the actual access patterns: every real query (list/get a user's own items, hydrate a `build` by reading its linked keyboard/switch/keycap_set, a future "view this user's public items" feature) is a shallow, fixed-fan-out lookup scoped to an already-known `user_id` — never a cross-user query or open-ended search. That means no GSIs are needed (a base-table `Query(PK=user_id)` plus a `FilterExpression` covers everything), and DynamoDB's permanent free tier is a better cost fit than RDS's non-permanent one. If a Phase 2 feature ever needs real cross-user search/aggregation, that's the point to revisit this — not before.

**One Lambda function for everything.** `ApiFunction` will handle both REST routes and the MCP endpoint, and every entity (keyboards, switches, keycap_sets, builds) once Phase 1 lands — deliberately not split by protocol or by entity. Splitting would duplicate no code (REST and MCP already share the same service/repository layer, auth, and logging) while adding multiple cold-start profiles and deployment packages to keep in sync, with no isolation benefit at this project's traffic scale. New routes are added as additional `Events` on the same function in `template.yaml`, not new functions.

**Lambda packaging is a container image via `aws-lambda-web-adapter`, not a zip with an in-process Lambda SDK adapter.** The originally-planned `awslabs/aws-lambda-go-api-proxy` library was archived by AWS in May 2025. `aws-lambda-web-adapter` is a Lambda extension (sidecar) that translates API Gateway events into real HTTP requests against a plain `net/http` server — the Go application has **zero AWS/Lambda SDK imports**; `functions/api/main.go` just calls `http.ListenAndServe`. This is a stronger version of the project's portability goal than the old adapter library would have given. The tradeoff is container-image packaging (an `AWS::ECR::Repository` with a lifecycle policy expiring `pr-*`-tagged images after 2 days, plus a Dockerfile) instead of a simpler zip build.

**Auth verification happens twice, deliberately.** API Gateway's HTTP API has a Cognito JWT authorizer (`template.yaml`'s `HttpApi.Auth`) that rejects invalid tokens before the Lambda runs — a coarse, optional pre-filter. The Go application *also* independently verifies every token itself (`internal/auth.Verifier.VerifyToken`, called from `internal/middleware.Auth`), rather than trusting API-Gateway-injected claims implicitly. This is defense-in-depth, and it's also required for the MCP side once added: MCP clients won't necessarily go through API Gateway's authorizer the same way, and the MCP tool layer needs its own verification call — into the *same* `Verifier`, via a different attachment point (`mcp-go`'s `WithToolHandlerMiddleware` instead of `net/http` middleware). One verification implementation, multiple thin protocol-specific adapters calling it — not one auth implementation per protocol.

**`internal/auth` is intentionally not `net/http`-aware.** `Verifier`/`VerifyToken` take and return a raw token string and claims; they know nothing about HTTP, Lambda, or MCP. This is what lets the same verifier be reused by both the REST middleware (`internal/middleware.Auth`) and the future MCP tool middleware without duplicating verification logic per protocol.

**`internal/auth.Verifier` depends on an unexported `tokenVerifier` interface, not the concrete `*oidc.IDTokenVerifier` type**, specifically so unit tests can inject a mock (`internal/auth/mocks`, generated by `mockery` — see `.mockery.yml`) instead of standing up a real OIDC discovery round-trip or hand-signing JWTs. This is deliberate: correctness of `go-oidc`'s own signature/expiry/issuer/audience validation is not this project's concern to unit-test (that's testing a well-tested third-party library, not our code) — those tests were removed in favor of ones that verify *our* wiring calls the verifier and propagates its result correctly. Real end-to-end rejection-path coverage (expired tokens, bad signatures, etc., actually being rejected) belongs at the functional/black-box test layer, against a real or mock OIDC issuer (`mockoidc` — issue #7), not here. This dependency-injection-over-mocking pattern (define a narrow interface for the one method actually used, inject it, generate a `mockery` mock rather than hand-writing one) is the preferred approach generally in this codebase, not specific to auth.

**Generated mocks live in a `mocks/` subpackage next to the code they mock (e.g. `internal/auth/mocks/`), never alongside the source file.** Add new interfaces to mock under `.mockery.yml`'s `packages` entry rather than hand-writing a mock; give the generated struct an exported name (e.g. `structname: 'MockTokenVerifier'`) since an unexported interface still needs an importable mock type from the separate `mocks` package.

**Package layout under `internal/`**:
- `auth/` — token verification logic only (`Verifier`, `VerifyToken`, `Claims`). No `net/http`.
- `auth/mocks/` — generated by `mockery` (`mise exec -- mockery` or just `mockery` if mise-activated); do not hand-edit. Regenerate after changing `tokenVerifier`'s method set or adding new interfaces to `.mockery.yml`.
- `middleware/` — `net/http` middleware: `Auth` (wraps `auth.Verifier`) and `Logging` (structured request logging, correlation IDs).
- `handlers/` — route handler functions (e.g. `Ping`). Kept separate from middleware and from router wiring.
- `router/` — wires handlers + middleware into the application's `http.Handler`. No logic of its own.

Follow this same split when adding Phase 1 entities: a handler per route in `handlers/`, business/data-access logic in its own package (a `KeyboardRepository`-style interface per entity, per the plan — not yet added), never inline in `router.go`.

**`functions/api/` deliberately does not follow the `golang-standards/project-layout` convention of `cmd/<binary-name>/` for entrypoints** — this was raised and considered explicitly; `functions/api/` was kept because it more directly signals "this is a Lambda function" in this AWS-specific codebase. Don't "fix" this to `cmd/api/` — it's an intentional, discussed deviation, not an oversight. `internal/` and (planned) `test/` do follow the convention.

**Config is env-var-only, read via Kong (`functions/api/config.go`)**, not raw `os.Getenv` calls — Kong gives struct-tag-driven required-field validation and defaults even though there are no CLI flags to parse (this is a Lambda entrypoint, not a CLI tool). Add new config by adding a field to `Config`, not by reaching for `os.Getenv` elsewhere.

**Multi-tenancy is baked into the design from day one even though nothing is currently shared.** Every entity table (Phase 1, not yet added) will be partitioned by `user_id` (Cognito's immutable `sub` claim — not email, which is mutable) so that a later "let users share/view specific items" feature doesn't require a schema rewrite. `Claims.Subject` (populated from `idToken.Subject`) is what becomes `user_id` downstream.

**API versioning**: REST routes are prefixed `/v1/...` from day one (see `template.yaml` / `internal/router`). MCP has no formal version number (the protocol has no standardized tool-versioning layer) — instead, MCP tool schemas/descriptions follow additive-only evolution (never remove/rename/required-ify a field) once the MCP layer exists, since tool *descriptions* affect which tool an LLM client selects, not just the schema.

## Testing strategy

Two frameworks, split by test type — not interchangeable, don't mix them:
- **Unit tests**: `testify/suite` + `mockery`-generated mocks, colocated with the code (`foo_test.go` next to `foo.go`). For function-level logic with dependencies injected via interfaces — no real infra, no network calls.
- **Functional/black-box tests** (not yet added — issue #7/#8): Ginkgo v2 + Gomega, living under a centralized `test/functional/features/{api,mcp}/` tree (not colocated), driving real HTTP/MCP clients against a real deployed stack (locally via `sam local start-api` + LocalStack + `mockoidc`, or a real ephemeral per-PR AWS stack in CI). Gomega's `ghttp` (HTTP mocking) is explicitly not used here — the point of this layer is exercising the real deployed thing, not mocks.

Test data isolation (once functional tests exist): a fresh synthetic `user_id` (UUID) generated per spec, not table truncation — every table is partitioned by `user_id`, so parallel specs can never collide even sharing the same DynamoDB table.

## Toolchain

Go, `aws-cli`, `aws-sam-cli`, and `mockery` versions are pinned in `mise.toml` — run `mise install` once, then `mise activate` (already wired into most shells via `.zshrc`/`.bashrc`) makes `go`/`sam`/`aws`/`mockery` resolve directly without a `mise exec --` prefix. If a Bash tool session doesn't have `mise activate` in effect (e.g. non-interactive subshells), fall back to `mise exec -- <command>`.
