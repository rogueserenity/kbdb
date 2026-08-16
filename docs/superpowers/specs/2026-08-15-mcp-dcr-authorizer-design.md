# Cognito DCR + Unified Authorizer

Date: 2026-08-15

**Superseded 2026-08-16 by `2026-08-16-workos-auth-migration-design.md`.**
Live testing found a second Cognito blocker beyond DCR (no RFC 8252
loopback wildcard-port redirect URI support), and a provider comparison
concluded WorkOS AuthKit meets both requirements natively. Retained
unmodified as the research trail — the Cognito-shim architecture below is
not being built.

## Problem

AWS Cognito user pools do not support RFC 7591 Dynamic Client Registration
(DCR): app clients (`client_id`s) can only be created via
`CreateUserPoolClient`, which is not exposed as a public, unauthenticated,
spec-compliant registration endpoint. Any service that wants OAuth clients
(e.g. an MCP server, per the MCP Authorization spec, which expects DCR
support) to self-register against Cognito — rather than requiring a human
to manually provision each one — needs a shim in front of Cognito to
provide that.

Motivating context (not part of this component's own scope): kbdb, a
separate project, wants its MCP endpoint easy for community members —
including non-technical ones — to add to their own AI client (Claude Code,
ChatGPT connectors, other MCP hosts), which requires exactly this. How
kbdb integrates with this component is out of scope for this document;
see kbdb's own integration plan once one exists.

## Decision summary

Build the DCR shim + a unified request authorizer as a standalone,
public, MIT-licensed GitHub repository — a general-purpose component
usable by any Cognito-backed service, not tied to any one consumer's
domain. Configuration surface is limited to a Cognito User Pool ID and a
redirect-URI allowlist policy; the component has no awareness of what
routes or resources sit behind it.

Not announced or promoted anywhere at launch — public purely so CI runs on
free GitHub Actions minutes, and so this stays cleanly extractable/reusable
without ever having accreted consumer-specific coupling in the first
place.

Four phases, described in priority order:

1. **Registration + last-seen tracking** (build now)
2. **Unified authorizer** (build now, same phase as #1 — they share a data
   model)
3. **Purger** (design only, deferred)
4. **floci migration for local dev/CI**, including an upstream floci
   contribution (design only, deferred, sequenced after #1/#2 are proven
   against real AWS)

## Prior art considered

Researched before designing from scratch (see conversation for full
detail); summary:

- **`empires-security/mcp-oauth2-aws-cognito`** — closest architectural
  match (DCR Lambda + authorizer + DynamoDB + PKCE + RFC 8414/9728
  metadata). Not adoptable as code: Node/Express + raw CloudFormation, no
  tests/CI, single-author, last commit ~4.5 months before this spec was
  written, no releases/tags. More importantly, its own Issue #12 (Mar
  2026, never fixed before the repo went cold) surfaced the exact two
  flaws this design avoids: unbounded Cognito app-client creation with no
  dedup/TTL (hits Cognito's 1,000-client ceiling), and issuing
  `client_secret`s to what should be public/PKCE clients. MIT-licensed;
  used as a design reference only (particularly its discovery of the
  PKCE-advertisement gotcha below). Its 68 stars and unresolved
  "this doesn't scale" issue are evidence this is a recognized, recurring
  pain point for Cognito-backed MCP servers generally, not a one-off need.
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
Client (any OAuth/DCR-capable client)
   |
   |  1. GET <consumer's protected-resource metadata>
   v
[Consuming service]  --serves--> registration_endpoint: <this repo>/register
   |
   |  2. POST /register  (RFC 7591)
   v
[Registration Lambda]
   - validates redirect_uris against allowlist (loopback only, v1)
   - normalizes + dedupes on (client_name, redirect_uris)
   - GetItem on dedupe_key:
       hit  -> return existing cognito_client_id
       miss -> CreateUserPoolClient (public, no secret, PKCE-required)
               -> PutItem -> return new cognito_client_id
   v
[Cognito UserPool]  (owned by the consuming service, not this repo)
   |
   |  3. Standard OAuth code + PKCE flow using the new client_id,
   |     against Cognito's real Hosted UI
   v
   issues JWT (aud / client_id claim = the DCR-issued client_id)
   |
   v
[Consuming service's API Gateway]
   |
   |  4. Configured to invoke this repo's Authorizer Lambda
   |     (REQUEST type, payload format 2.0)
   v
[Authorizer Lambda]
   - verifies JWT (self-contained OIDC/go-oidc code; no consumer
     dependency)
   - extracts client_id from claims (aud on ID tokens / client_id claim
     on access tokens)
   - branches on event.routeKey against a deploy-time-configured list of
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
[Consuming service's own backend]
   - reads identity/anonymous status from the authorizer's returned
     context, decides its own request handling from there
```

The per-route "is anonymous OK here" table, and which routes this
authorizer fronts at all vs. bypass it entirely, are entirely the
consuming service's deploy-time configuration — this repo has no opinion
on route names or resource semantics.

### Data model — `OAuthClientTable`

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

`POST /register` (RFC 7591), unauthenticated:

1. Validate `redirect_uris`: every entry must match `http://localhost:*/*`
   or `http://127.0.0.1:*/*` (v1 allowlist — covers CLI/desktop clients
   spinning up a local callback server on an ephemeral port). Reject
   anything else with `400 invalid_client_metadata`.
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
the code flow, so MCP clients in particular should support it regardless.

This repo also serves a `registration_endpoint` value (for a consuming
service to reference from its own RFC 9728 protected-resource metadata)
and hardcodes `code_challenge_methods_supported: ["S256"]` in its own
metadata output, per the gotcha noted above.

## Phase 2: Unified Authorizer Lambda

Described in full under Architecture above. Key points not to lose:

- Self-contained JWT/OIDC verification — its own `go-oidc`-based code,
  with no dependency on any consuming service's code. This is what makes
  the authorizer genuinely reusable across consumers.
- Validation + last-seen tracking happen in **one** DynamoDB round-trip: a
  conditional `UpdateItem` (must already exist) with `ReturnValues:
  ALL_NEW`, not two separate reads/writes.
- Fails closed: an unrecognized `client_id` (deregistered, purged, or
  simply never registered) causes the conditional update to fail, which
  the authorizer treats as Deny.
- Returns an identity-or-anonymous context to the consuming service on
  Allow; the consuming service is entirely responsible for what it does
  with that context (ownership checks, per-item visibility, etc.) — this
  repo has no opinion on and no visibility into consumer resource models.

## Phase 3: Purger (design only, deferred)

Not implemented in this pass — captured here so `OAuthClientTable`'s shape
never needs to change when this is built, and so idle-client data exists
from day one rather than being backfilled blind.

Daily EventBridge-scheduled Lambda scans `OAuthClientTable` for
`last_used_at` older than a configurable retention window (default
proposal: 90 days idle), calls `DeleteUserPoolClient` for each, then
removes the table row. Sized this way specifically because `last_used_at`
is populated from real authenticated traffic (phase 2) from day one, not
retrofitted later — the moment retention policy is decided, there is
immediately real data to act on, including bulk-purging anything that was
already idle before the sweep existed.

Follow-up: file a GitHub issue in this repo linking back to this spec,
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
the consuming service's feature isn't blocked on the floci PR merging, and
the floci PR isn't rushed to unblock anything.

## Testing

- **Registration Lambda**: `testify/suite` unit tests with a mocked
  Cognito client wrapper (`mockery`-generated, one method:
  `CreateUserPoolClient`) and a mocked `OAuthClientRepository`.
- **Authorizer Lambda**: unit tests for the routeKey-branching logic and
  the conditional-update outcome mapping (found/not-found/error →
  Allow/Deny), with a mocked repository; integration-level tests using
  floci for real JWKS-backed JWT verification (client CRUD + JWT issuance
  is exactly what floci already supports today, no need to wait for phase
  4's authorize-flow work for this level of test).
- **Functional/end-to-end**: Ginkgo, Given/When/Then structure. Golden
  path (register → complete a real PKCE login → call a protected route →
  confirm Allow) and the redirect_uri-rejection case. Until phase 4 lands,
  the login step of this suite runs against `mockoidc` or a real AWS dev
  account during initial development, per the phase 4 sequencing decision
  above; once phase 4's floci PR lands, this becomes floci-backed like the
  rest.

## Open items for the implementation plan

- Repo name and initial scaffolding (go.mod, SAM template, CI).
- `OAuthClientTable`'s indexing (GSI on `cognito_client_id` vs. swapping
  primary/secondary key roles — noted above, needs resolving before
  writing table DDL).
- How a consuming service is expected to deploy/wire this in (nested
  stack vs. standalone prerequisite stack) — likely belongs in this repo's
  own README/deployment docs once written, not this spec.

## Explicitly out of scope

Any consumer-side integration work (e.g. kbdb wiring its API Gateway to
this authorizer, kbdb's `template.yaml` changes, kbdb deleting its own
in-process JWT verification code) belongs to that consumer's own
integration plan, not this spec.
