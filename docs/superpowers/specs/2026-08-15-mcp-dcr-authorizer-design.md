# DCR + Unified Authorizer for kbdb's MCP/REST auth

Date: 2026-08-15

## Problem

kbdb's MCP endpoint (`/mcp`) is meant to be easy for community members —
including non-technical ones — to add to their own AI client (Claude Code,
ChatGPT connectors, other MCP hosts). Those clients authenticate via OAuth
against kbdb's Cognito user pool, and Claude Code's MCP OAuth flow (like the
MCP Authorization spec generally) expects to complete RFC 7591 Dynamic
Client Registration (DCR) — the client self-registers and receives a
`client_id` at connect time, rather than a human pre-provisioning one.

Cognito user pools do not support DCR natively: `client_id`s can only be
created via `CreateUserPoolClient`, which is not exposed as a public,
unauthenticated, spec-compliant registration endpoint. Without a shim,
every MCP client wanting to connect needs someone to manually create and
hand them a Cognito app client — the opposite of "easy to add."

## Decision summary

Build a DCR shim + a unified request authorizer as a **new, standalone,
public, MIT-licensed GitHub repository**, deployed as a companion/nested
stack that kbdb depends on. Not announced or promoted anywhere; public
purely so it runs on free GitHub Actions minutes and so extracting it into
a real community project later requires no untangling from kbdb internals.

Four phases, described in priority order:

1. **Registration + last-seen tracking** (build now)
2. **Unified authorizer, replacing kbdb's existing Cognito JWT authorizer**
   (build now, same phase as #1 — they share a data model)
3. **Purger** (design only, deferred)
4. **floci migration for local dev/CI**, including an upstream floci
   contribution (design only, deferred, sequenced after #1/#2 are proven
   against real AWS)

## Why a separate repo, not `internal/oauth` inside kbdb

This subsystem has no real coupling to kbdb's domain (keyboards, builds,
switches) — its only inputs are a Cognito User Pool ID and a redirect-URI
allowlist policy. Building it inside kbdb would accrete incidental
coupling (kbdb's `internal/repository` conventions, kbdb's `Config`/Kong
wiring, assumptions about deploying into the same stack) that would all
have to be deliberately un-picked to extract later. Building it as a
separate repo from day one forces a real external interface immediately,
for roughly the same effort as building it "inside" — and directly
supports later productizing it for other Cognito-backed services, which is
plausible: the `empires-security/mcp-oauth2-aws-cognito` prior-art search
(below) found 68 stars and an unresolved "this doesn't scale" issue on a
comparable project, evidence this is a recognized, recurring pain point in
the MCP ecosystem, not a kbdb-specific one.

Public + OSS-licensed (not private) specifically so it runs on free GitHub
Actions minutes rather than eating into a paid private-repo budget.

## Prior art considered

Researched before designing from scratch (see conversation for full
detail); summary:

- **`empires-security/mcp-oauth2-aws-cognito`** — closest architectural
  match (DCR Lambda + authorizer + DynamoDB + PKCE + RFC 8414/9728
  metadata). Not adoptable as code: Node/Express + raw CloudFormation
  (mismatched with kbdb's Go/SAM), no tests/CI, single-author, last commit
  ~4.5 months before this spec was written, no releases/tags. More
  importantly, its own Issue #12 (Mar 2026, never fixed before the repo
  went cold) surfaced the exact two flaws this design avoids: unbounded
  Cognito app-client creation with no dedup/TTL (hits Cognito's 1,000-client
  ceiling), and issuing `client_secret`s to what should be public/PKCE
  clients. MIT-licensed; used as a design reference only (particularly its
  discovery of the PKCE-advertisement gotcha below).
- **`aws-samples/sample-multi-tenant-saas-mcp-server`** — better pedigree
  (AWS Samples org, 108 commits) but scoped to a larger multi-tenant SaaS
  reference architecture; used as a secondary reference, not adopted.

**Known gotcha carried into this design**: Cognito's OIDC discovery
document does not advertise `code_challenge_methods_supported`. Clients
that gate PKCE support on that metadata field (e.g. MCP Inspector, Cursor,
per the empires-security repo's Issue #9) will fail to attempt PKCE unless
the metadata response hardcodes `["S256"]` into
`code_challenge_methods_supported`.

## Architecture

```
Client (Claude Code, etc.)
   |
   |  1. GET /.well-known/oauth-protected-resource(/mcp)
   v
[kbdb API]  --serves--> registration_endpoint: <new repo>/register
   |
   |  2. POST /register  (RFC 7591)
   v
[New repo: Registration Lambda]
   - validates redirect_uris against allowlist (loopback only, v1)
   - normalizes + dedupes on (client_name, redirect_uris)
   - GetItem on dedupe_key:
       hit  -> return existing cognito_client_id
       miss -> CreateUserPoolClient (public, no secret, PKCE-required)
               -> PutItem -> return new cognito_client_id
   v
[Cognito UserPool]  (existing, kbdb-owned)
   |
   |  3. Standard OAuth code + PKCE flow using the new client_id,
   |     against Cognito's real Hosted UI
   v
   issues JWT (aud / client_id claim = the DCR-issued client_id)
   |
   v
[API Gateway HttpApi]
   |
   |  4. DefaultAuthorizer = New repo's unified Authorizer Lambda
   |     (REQUEST type, payload format 2.0)
   v
[New repo: Authorizer Lambda]
   - verifies JWT (self-contained OIDC/go-oidc code; no kbdb dependency)
   - extracts client_id from claims (aud on ID tokens / client_id claim
     on access tokens)
   - branches on event.routeKey against a deploy-time list of
     "anonymous-acceptable" routes:
       - anonymous-acceptable route + no/invalid token -> Allow,
         context = anonymous
       - anonymous-acceptable route + valid token -> conditional
         UpdateItem(client_id): must exist in OAuthClientTable, sets
         last_used_at, ReturnValues=ALL_NEW -> Allow, context =
         authenticated (fails closed to Deny if client_id unknown)
       - non-anonymous-acceptable route + no/invalid token -> Deny
       - non-anonymous-acceptable route + valid token -> same
         conditional UpdateItem as above -> Allow or Deny
   v
[kbdb ApiFunction] (unchanged handlers)
   - reads identity/anonymous status from authorizer context
   - internal/authz.ReadableVisibilities / IsOwner decide per-request
     behavior, exactly as today
```

### Route scope

Per-route `Authorizer: NONE` overrides in `template.yaml` are **kept only**
for the two fully-public lookup GET routes (`ListLookupsEvent`,
`GetLookupEvent`) — true bypass, zero authorizer invocation, for the
highest-traffic, least-sensitive routes.

Every other route — required-auth REST writes, optional-auth REST GETs
(switches/keyboards/keycap-sets/builds), and all three MCP routes — goes
through the single unified `DefaultAuthorizer`. The authorizer's own
per-route "is anonymous OK here" table is static, baked in at deploy time
(not a runtime lookup), so this doesn't reintroduce the resource-visibility
Lambda-authorizer design that was already considered and explicitly
rejected in `project_api_design.md` — that prior rejection was about
per-*item* visibility (needing a duplicate resource read + a cache key that
varies per item, not just per token), which does not apply here: this
authorizer's decision depends only on identity + route, both of which are
already exactly what API Gateway's authorizer cache key naturally varies
on.

### Data model — `OAuthClientTable` (new repo, own table)

| Attribute | Type | Notes |
|---|---|---|
| `dedupe_key` (PK) | string | Normalized `client_name` + sorted `redirect_uris`. Trimmed, casefolded; loopback (`localhost`/`127.0.0.1`) redirect URIs have their port stripped before normalizing, since ephemeral local ports otherwise defeat dedup. No hashing — well under DynamoDB's key-size limit, and a plain string PK is human-readable for debugging. |
| `cognito_client_id` | string | Cognito's generated `ClientId` (not settable by us — `CreateUserPoolClient` has no `ClientId` input field). |
| `client_name` | string | Original, unnormalized. |
| `redirect_uris` | list\<string\> | Original, unnormalized. |
| `created_at` | timestamp | Registration time. |
| `last_used_at` | timestamp | Absent until first authenticated request; updated by the authorizer's conditional `UpdateItem`, described above. |

A GSI or second lookup by `cognito_client_id` is needed for the
authorizer's conditional-update path (it has the client_id from JWT
claims, not the dedupe_key) — implementation detail to resolve during
planning, either a GSI on `cognito_client_id` or making it the table's
actual PK with `dedupe_key` as a GSI instead (registration's hot path is
dedupe-key lookup; the authorizer's hot path is client_id lookup — both
need to be fast).

## Phase 1: Registration Lambda

`POST /register` (RFC 7591), unauthenticated, in the new repo:

1. Validate `redirect_uris`: every entry must match `http://localhost:*/*`
   or `http://127.0.0.1:*/*` (v1 allowlist — covers CLI/desktop MCP
   clients spinning up a local callback server on an ephemeral port).
   Reject anything else with `400 invalid_client_metadata`.
2. Normalize + compute `dedupe_key`; `GetItem`.
   - Hit: return the existing `cognito_client_id` (RFC 7591 permits
     reusing a `client_id` across registration calls; nothing mandates a
     fresh one every time).
   - Miss: `CreateUserPoolClient` — **public client** (`GenerateSecret:
     false`), `AllowedOAuthFlows: [code]`, PKCE enforced. `PutItem`.
3. Response: standard RFC 7591 shape — `client_id`, `client_name`,
   `redirect_uris`, `token_endpoint_auth_method: "none"`, no
   `client_secret`.

Public-client/PKCE choice rationale: loopback-redirect clients (CLI/desktop
apps) are the textbook OAuth "public client" case (RFC 8252) — a secret
embedded identically in every install of a distributed binary isn't
actually confidential, and the MCP Authorization spec mandates PKCE for
the code flow, so MCP clients should support it regardless.

Also updates the existing `/.well-known/oauth-protected-resource(/mcp)`
metadata (still served by kbdb, per RFC 9728) to add a `registration_endpoint`
pointing at the new repo's `/register`, and hardcodes
`code_challenge_methods_supported: ["S256"]` per the gotcha noted above.

## Phase 2: Unified Authorizer Lambda

Described in full under Architecture above. Key points not to lose:

- Self-contained JWT/OIDC verification — its own `go-oidc`-based code, not
  a dependency on kbdb's `internal/auth`. This is what makes the new repo
  genuinely standalone.
- Validation + last-seen tracking happen in **one** DynamoDB round-trip: a
  conditional `UpdateItem` (must already exist) with `ReturnValues:
  ALL_NEW`, not two separate reads/writes.
- Fails closed: an unrecognized `client_id` (deregistered, purged, or
  simply never registered) causes the conditional update to fail, which
  the authorizer treats as Deny.

### Consequence for kbdb: `internal/auth` is deleted

Once this authorizer is live and is kbdb's `DefaultAuthorizer`,
`functions/api` never verifies a token itself again for any route the
authorizer fronts — API Gateway invokes the authorizer before the request
reaches `functions/api` at all. `internal/auth` (`Verifier`, `VerifyToken`,
`Claims`) and `internal/auth/mocks` are deleted from kbdb entirely, not
kept as a fallback:

- `internal/middleware.Auth` and `middleware.OptionalAuth` stop calling
  verification in-process; they read identity/anonymous status from the
  authorizer's passed-through request context instead.
- `internal/mcp`'s tool-level middleware likewise stops calling
  `VerifyToken` — MCP routes are now fronted by the same authorizer as
  REST, so there is no remaining in-app verification path for MCP either.
- `internal/authz.ReadableVisibilities` and `authz.IsOwner` are
  **unchanged** — they continue to be kbdb's job, operating on whatever
  identity the authorizer's context supplies.
- `.mockery.yml`'s `tokenVerifier` mock entry is removed along with the
  package it mocked.

### `template.yaml` changes

- Deploy the new repo's stack (nested stack or a separate `sam deploy`
  step run before kbdb's own) into the same account, wired to kbdb's
  existing `UserPool`.
- `HttpApi.Auth.DefaultAuthorizer` changes from `CognitoAuthorizer` to the
  new repo's authorizer Lambda ARN.
- `CognitoAuthorizer`'s JWT-audience-list definition is removed — no
  longer used by anything.
- Remove `Authorizer: NONE` from every route's `Auth` block **except**
  `ListLookupsEvent`/`GetLookupEvent`.

## Phase 3: Purger (design only, deferred)

Not implemented in this pass — captured here so `OAuthClientTable`'s shape
never needs to change when this is built, and so idle-client data exists
from day one rather than being backfilled blind.

Daily EventBridge-scheduled Lambda (in the new repo) scans
`OAuthClientTable` for `last_used_at` older than a configurable retention
window (default proposal: 90 days idle), calls `DeleteUserPoolClient` for
each, then removes the table row. Sized this way specifically because
`last_used_at` is populated from real authenticated traffic (phase 2) from
day one, not retrofitted later — the moment retention policy is decided,
there is immediately real data to act on, including bulk-purging anything
that was already idle before the sweep existed.

Follow-up: file a GitHub issue in the new repo linking back to this spec,
scoped to just this phase.

## Phase 4: floci migration for local dev/CI (design only, deferred)

**Sequencing, not optional ordering**: phases 1–2 are built and validated
against a real AWS dev account first. Only once the design is proven
end-to-end against real Cognito does work begin on migrating local
dev/CI onto floci.

Rationale: floci's `/oauth2/token` currently only supports
`grant_type=client_credentials`, and `/oauth2/authorize` (the hosted-UI
authorization-code step) doesn't exist yet — so floci cannot today emulate
the full PKCE authorization-code flow this design depends on end-to-end.
Everything else floci already supports and is usable immediately once
adopted: `CreateUserPoolClient`/`DescribeUserPoolClient`/
`ListUserPoolClients`/`DeleteUserPoolClient`, and real RS256-signed,
JWKS-verifiable JWTs (confirmed via floci's docs and source,
`CognitoService.generateAuthResult`/`generateSignedJwt`/`signJwt`).

Gap sizing (confirmed against floci's actual source, not just docs):
small-to-medium, ~300–500 lines, comparable to already-merged
external-contributor Cognito PRs in that repo (e.g. #2071, #1701):

- `GET /oauth2/authorize` handler — mechanical addition to
  `CognitoOAuthController`'s existing JAX-RS pattern; `UserPoolClient`
  already models `callbackURLs`/`allowedOAuthFlows`, just unwired to any
  handler yet.
- Authorization-code store — no direct precedent, but
  `services/cognito/verification/VerificationCode(Service)` is a
  near-identical pattern to copy (short-lived code → binding).
  Reuses existing `generateAuthResult()`/`generateSignedJwt()` for the
  actual token issuance once a code is exchanged.
  Password check for the authorize step reuses existing
  `CognitoAuthFlowHandler.authenticateWithPassword()`.
- PKCE `code_challenge`/`code_verifier` S256 matching — genuinely
  net-new (confirmed zero existing references), but trivial: stdlib
  SHA-256 + base64url compare, ~30 lines.

No prior floci issue/PR found discussing this gap — appears simply
unaddressed, not declined.

**Why this order and not floci-first**: building the floci PR before
validating phases 1–2 against real Cognito means emulating a flow that
hasn't been exercised for real yet, risking subtle behavioral drift from
actual Cognito that's hard to catch without a working reference to diff
against. Building against real AWS first produces recorded real
request/response pairs from an actual dev-account run, which become the
floci PR's test fixtures/spec — a stronger contribution than one written
from documentation alone. It also decouples the two efforts' timelines:
kbdb's feature isn't blocked on the floci PR merging, and the floci PR
isn't rushed to unblock kbdb.

Follow-up: file a GitHub issue (in kbdb, referencing this spec) once
phases 1–2 are live, scoped to floci migration + the upstream
contribution.

## Testing

- **New repo — Registration Lambda**: `testify/suite` unit tests (matching
  kbdb's own convention) with a mocked Cognito client wrapper
  (`mockery`-generated, one method: `CreateUserPoolClient`) and a mocked
  `OAuthClientRepository`.
- **New repo — Authorizer Lambda**: unit tests for the routeKey-branching
  logic and the conditional-update outcome mapping (found/not-found/error
  → Allow/Deny), with a mocked repository; integration-level tests using
  floci for real JWKS-backed JWT verification (client CRUD + JWT issuance
  is exactly what floci already supports today, no need to wait for phase
  4's authorize-flow work for this level of test).
- **New repo — functional/end-to-end**: Ginkgo, following the same
  Given/When/Then structure kbdb already uses. Golden path (register →
  complete a real PKCE login → call a protected route → confirm Allow) and
  the redirect_uri-rejection case. Until phase 4 lands, the login step of
  this suite runs against `mockoidc` (or a real AWS dev account during
  initial development, per the phase 4 sequencing decision above); once
  phase 4's floci PR lands, this becomes floci-backed like the rest.
- **kbdb**: extend `test/functional/features/api/` specs to cover
  authorizer-context propagation (authenticated vs. anonymous) reaching
  `middleware.OptionalAuth`/`authz.ReadableVisibilities` correctly; no
  change to the Given/When/Then structural rule from `CLAUDE.md`.

## Open items for the implementation plan

- New repo's exact name and initial scaffolding (go.mod, SAM template,
  CI).
- `OAuthClientTable`'s indexing (GSI on `cognito_client_id` vs. swapping
  primary/secondary key roles — noted above, needs resolving before
  writing table DDL).
- Exact deploy sequencing between the new repo's stack and kbdb's stack
  (nested stack within `template.yaml` vs. a separate prerequisite
  `sam deploy` documented in `CONTRIBUTING.md`).
