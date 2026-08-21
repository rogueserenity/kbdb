# kbdb

kbdb is a keyboard collection database — keyboards, switches, keycap sets, and the builds assembled from them. It's multi-user: each item has a visibility of private (owner only), authenticated (any signed-in user), or public (no auth needed), set per item by its owner.

It exposes two first-class APIs — REST and an MCP server, so you can manage your collection from an AI chat client — backed by the same service/repository layer, so they never drift out of sync with each other.

## Architecture

**A single Lambda function serves everything** — every REST route and the MCP endpoint, across every entity. REST and MCP share one service/repository layer, one auth path, one logging setup: a fix or a new capability lands in both interfaces at once, with nothing to keep in sync by hand.

**The Lambda ships as a container image running a plain `net/http` server**, via [`aws-lambda-web-adapter`](https://github.com/awslabs/aws-lambda-web-adapter). The Go application itself has zero AWS/Lambda SDK imports — the entrypoint just calls `http.ListenAndServe`. The same binary runs unmodified behind any other HTTP-speaking host, not just Lambda.

**Auth is defense-in-depth.** API Gateway's native JWT authorizer rejects invalid tokens before the Lambda even runs. The application then independently verifies every token itself rather than trusting injected claims implicitly — the same verification path both REST and MCP tool calls go through, since MCP clients don't all reach the server via API Gateway's authorizer the same way.

**DynamoDB gives every query a predictable, flat-rate cost.** Every real access pattern — list/get a user's own items, hydrate a build from its linked keyboard/switch/keycap set — is a shallow lookup scoped to a known `user_id`, so there's nothing here a secondary index or a relational query planner would improve on.

**Multi-tenancy and per-item visibility are load-bearing from the schema up.** Every entity table is partitioned by `user_id` (the IdP's immutable subject claim, never the mutable email), and each item carries its own visibility — private, authenticated, or public — enforced independently of that partitioning.

**API versioning is explicit where it needs to be.** REST routes are prefixed `/v1/...`. MCP tool schemas and descriptions evolve additive-only instead, since tool descriptions steer which tool an LLM client picks, not just the schema shape a version number would cover.

**The whole stack runs at effectively $0** on AWS free-tier usage — a cost-budget tripwire is wired into every environment, so a real bill showing up is itself the alarm.

## Identity provider requirements

kbdb has no IdP-specific code — it's currently deployed against Stytch but doesn't depend on it. Any IdP works as long as it:

- Publishes standard OIDC discovery (`.well-known/openid-configuration`) and a JWKS endpoint, since both API Gateway's native JWT authorizer and the application's own `go-oidc`-based verifier resolve signing keys that way.
- Issues RS256-signed JWTs with standard `iss`, `aud`, and `exp` claims.
- Includes a `sub` claim that's a stable, immutable identifier for the user — it's used as the partition key for every entity table, so it must never change or be reused for a different person.
- Serves OAuth 2.0 Authorization Server Metadata (RFC 8414) at that issuer, so MCP clients doing OAuth discovery can find its authorization/token endpoints. The MCP endpoint advertises the issuer as its authorization server via RFC 9728 Protected Resource Metadata (`/.well-known/oauth-protected-resource`) — an IdP without RFC 8414 metadata breaks MCP client login even though REST and hand-issued tokens would still work fine.
- Supports RFC 7591 Dynamic Client Registration, so an MCP client like Claude Code can register itself against kbdb without a human manually provisioning it a client ID first. This is the reason this project doesn't use Cognito, which supports neither this nor the next requirement.
- Supports RFC 8252 loopback wildcard-port redirect URIs, since that's how a locally-running MCP client completes its OAuth redirect.

## Getting started

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for local setup, running the app against a local dev loop, deploying to your own AWS account and Stytch project, and the full command reference.

## License

MIT — see [`LICENSE`](LICENSE).
