# kbdb

kbdb is a keyboard collection database — keyboards, switches, keycap sets, and the builds assembled from them. It's multi-user, with each user's data private to them, and designed to eventually support sharing specific items.

The primary interface is an MCP server, so you can manage your collection from an AI chat client; REST is a secondary interface for everything else. Both share the same service/repository layer, so they never drift out of sync with each other.

## Architecture

**One Lambda function for everything.** `ApiFunction` handles both REST routes and the MCP endpoint, across every entity (keyboards, switches, keycap sets, builds) — deliberately not split by protocol or by entity. Splitting would duplicate no code (REST and MCP already share the same service/repository layer, auth, and logging) while adding multiple cold-start profiles and deployment packages to keep in sync, with no isolation benefit at this project's traffic scale.

**Lambda packaging is a container image via [`aws-lambda-web-adapter`](https://github.com/awslabs/aws-lambda-web-adapter), not a zip with an in-process Lambda SDK adapter.** `aws-lambda-web-adapter` is a Lambda extension (sidecar) that translates API Gateway events into real HTTP requests against a plain `net/http` server — the Go application has zero AWS/Lambda SDK imports; the entrypoint just calls `http.ListenAndServe`. That's a genuine portability property: the same binary that runs in Lambda would run unmodified behind any other HTTP-speaking host.

**Auth verification happens twice, deliberately.** API Gateway's WorkOS JWT authorizer rejects invalid tokens before the Lambda runs, as a coarse pre-filter. The application also independently verifies every token itself, rather than trusting API-Gateway-injected claims implicitly. This is defense-in-depth, and it's required for MCP regardless: MCP clients won't necessarily go through API Gateway's authorizer the same way, so the MCP tool layer needs its own verification call into the same underlying verifier.

**DynamoDB over PostgreSQL**, chosen after evaluating the actual access patterns: every real query (list/get a user's own items, hydrate a build by reading its linked keyboard/switch/keycap set) is a shallow, fixed-fan-out lookup scoped to an already-known `user_id` — never a cross-user query or open-ended search. That means no secondary indexes are needed, and DynamoDB's permanent free tier is a better cost fit than RDS's non-permanent one.

**Multi-tenancy is baked in from day one**, even though nothing is currently shared: every entity table is partitioned by `user_id` (WorkOS's immutable user ID, not the mutable email), so a later "share/view specific items" feature doesn't require a schema rewrite.

**API versioning**: REST routes are prefixed `/v1/...` from day one. MCP has no formal version number (the protocol has no standardized tool-versioning layer) — instead, MCP tool schemas and descriptions follow additive-only evolution, since tool descriptions affect which tool an LLM client selects, not just the schema shape.

**This whole stack runs at effectively $0** on AWS free-tier usage by design — a cost-budget tripwire is wired into every environment so a real bill appearing is itself a signal something is wrong.

## Getting started

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for local setup, running the app against a local dev loop, deploying to your own AWS/WorkOS accounts, and the full command reference.

## License

MIT — see [`LICENSE`](LICENSE).
