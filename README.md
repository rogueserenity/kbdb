# kbdb

kbdb is a keyboard collection database — keyboards, switches, keycap sets, and assembled builds. The primary interface is an MCP server for AI chat clients, with REST as a secondary interface; both share the same service/repository layer. It's built to be multi-user and eventually community-facing, though the current phase ships fully private, siloed-per-user data.

The project is being built issue-by-issue against a fixed architecture. It's currently in Phase 0: the scaffolding — auth, routing, the MCP layer, CI/CD, the local dev loop — is in place and proven end-to-end against a real deployed stack. The real keyboard/switch/keycap/build data model comes in Phase 1.

## Stack

Go, AWS Lambda (container image), API Gateway (HTTP API), Cognito, DynamoDB (Phase 1), AWS SAM for infrastructure-as-code. [`mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go) for the MCP layer.

## Architecture

**One Lambda function for everything.** `ApiFunction` handles both REST routes and the MCP endpoint, and will handle every entity (keyboards, switches, keycap sets, builds) once Phase 1 lands — deliberately not split by protocol or by entity. Splitting would duplicate no code (REST and MCP already share the same service/repository layer, auth, and logging) while adding multiple cold-start profiles and deployment packages to keep in sync, with no isolation benefit at this project's traffic scale.

**Lambda packaging is a container image via [`aws-lambda-web-adapter`](https://github.com/awslabs/aws-lambda-web-adapter), not a zip with an in-process Lambda SDK adapter.** `aws-lambda-web-adapter` is a Lambda extension (sidecar) that translates API Gateway events into real HTTP requests against a plain `net/http` server — the Go application has zero AWS/Lambda SDK imports; the entrypoint just calls `http.ListenAndServe`. That's a genuine portability property: the same binary that runs in Lambda would run unmodified behind any other HTTP-speaking host.

**Auth verification happens twice, deliberately.** API Gateway's Cognito JWT authorizer rejects invalid tokens before the Lambda runs, as a coarse pre-filter. The application also independently verifies every token itself, rather than trusting API-Gateway-injected claims implicitly. This is defense-in-depth, and it's required for MCP regardless: MCP clients won't necessarily go through API Gateway's authorizer the same way, so the MCP tool layer needs its own verification call into the same underlying verifier.

**DynamoDB (Phase 1) over PostgreSQL**, chosen after evaluating the actual access patterns: every real query (list/get a user's own items, hydrate a build by reading its linked keyboard/switch/keycap set) is a shallow, fixed-fan-out lookup scoped to an already-known `user_id` — never a cross-user query or open-ended search. That means no secondary indexes are needed, and DynamoDB's permanent free tier is a better cost fit than RDS's non-permanent one.

**Multi-tenancy is baked in from day one**, even though nothing is currently shared: every entity table (Phase 1) will be partitioned by `user_id` (Cognito's immutable `sub` claim, not the mutable email), so a later "share/view specific items" feature doesn't require a schema rewrite.

**API versioning**: REST routes are prefixed `/v1/...` from day one. MCP has no formal version number (the protocol has no standardized tool-versioning layer) — instead, MCP tool schemas and descriptions follow additive-only evolution, since tool descriptions affect which tool an LLM client selects, not just the schema shape.

**This whole stack runs at effectively $0** on AWS free-tier usage by design — a cost-budget tripwire is wired into every environment so a real bill appearing is itself a signal something is wrong.

## Getting started

Tool versions are pinned via [mise](https://mise.jdx.dev/):

```sh
mise install
```

Common tasks (see `mise.toml` for the full list):

```sh
mise run lint          # golangci-lint + actionlint + shellcheck
mise run test          # unit tests
mise run func-setup    # bring up a local dev loop (LocalStack + mockoidc + sam local start-api)
mise run func-test     # run functional tests against it
mise run func-teardown # tear it down
```

Deploying to AWS uses per-developer stacks (`mise run dev-setup`/`dev-deploy`/`dev-teardown`) rather than one shared environment. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the full command reference and AWS account/deploy model.

## Status

GitHub issues in this repo are the authoritative source of what's built, in progress, or deferred.

## License

MIT — see [`LICENSE`](LICENSE).
