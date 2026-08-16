# Migrate kbdb auth from Cognito to WorkOS

Date: 2026-08-16 (supersedes `2026-08-15-mcp-dcr-authorizer-design.md`)

## Problem

kbdb's MCP endpoint (`/mcp`) needs to be easy for community members —
including non-technical ones — to add to their own AI client (Claude Code,
ChatGPT connectors, other MCP hosts). Those clients authenticate via OAuth,
which for a public MCP server means supporting RFC 7591 Dynamic Client
Registration (DCR): the client self-registers and receives a `client_id`
at connect time, rather than a human pre-provisioning one.

kbdb was on AWS Cognito. Investigation (see prior spec, retained for its
research trail) found Cognito fails this outright — no DCR endpoint at
all — and a second, independently blocking problem: Cognito's
`CallbackURLs` requires every redirect URI to be pre-registered by exact
string match, with no wildcard/any-port support. RFC 8252 §7.3 (OAuth for
native apps) requires the authorization server to accept loopback redirect
URIs on *any* port, specifically to accommodate CLI/desktop clients that
bind an ephemeral port per launch — which is confirmed, empirically, to be
Claude Code's actual default behavior (GitHub issue
anthropics/claude-code#35740; `--callback-port` exists specifically as an
opt-in override for providers like Cognito that can't handle this).
Neither gap is something a shim can safely paper over without either
requiring every user to pass a manual flag (defeating "easy to add") or
building non-trivial registration/dedup/lifecycle machinery ourselves.

A broad provider comparison (Auth0, Ory Hydra, Keycloak, WorkOS, Zitadel,
Authentik, Supabase Auth, Firebase Auth, Cognito) found only a few
providers meet both hard requirements natively. **WorkOS AuthKit** was
selected: confirmed via live testing (not just docs) to support RFC 7591
DCR and RFC 8252 loopback wildcard-port redirect URIs, free up to 1M MAU,
with no separate cost or documented limit for the DCR/Connect features
this design uses.

## Decision summary

**Migrate kbdb's identity provider from Cognito to WorkOS AuthKit.**
WorkOS owns registration, client lifecycle, and the login/signup UI
entirely — kbdb no longer needs to build or operate any of the DCR shim,
dedupe table, or purger infrastructure the original Cognito-based design
required. What kbdb still owns:

1. **A custom Lambda authorizer** verifying WorkOS-issued JWTs and fronting
   API Gateway (both REST and MCP routes) — build now.
2. **WorkOS configuration**: one User Management application (kbdb's real
   identity directory) plus one first-party Connect OAuth application
   (the DCR/PKCE-capable entry point MCP clients register against) — build
   now, as WorkOS dashboard/API configuration, not code.

That's the entire scope. No registration Lambda, no `OAuthClientTable`, no
dedupe logic, no purger, no floci-migration phase — all of that existed
solely to compensate for Cognito's lack of native DCR/lifecycle management
and is now moot.

## Why WorkOS, not a Cognito shim (recap)

Full reasoning and the discarded Cognito-shim architecture are preserved
in `2026-08-15-mcp-dcr-authorizer-design.md` for the research trail
(prior art review of `empires-security/mcp-oauth2-aws-cognito` and
`aws-samples/sample-multi-tenant-saas-mcp-server`, the floci gap-sizing
work, etc.) — none of it is being built. The short version: every piece
of that design existed to compensate for something Cognito doesn't do
natively (DCR, wildcard-port loopback redirects, client lifecycle
visibility) that WorkOS does out of the box, confirmed live rather than
assumed from docs.

## WorkOS architecture: User Management + first-party Connect

WorkOS separates two concerns that are easy to conflate:

- **User Management** (`api.workos.com/user_management/*`) is the actual
  identity system — the one real user directory, signup, login, password
  reset, social login. This is what a kbdb REST client (a browser/mobile
  app doing a normal login) talks to.
- **Connect** (`https://<workspace>.authkit.app/oauth2/*`) is an
  additional OAuth surface *layered on top of* the same User Management
  directory — not a separate identity system. It's the product WorkOS
  built specifically for MCP/third-party-application OAuth: DCR, CIMD,
  PKCE-enforced public clients, and RFC 8707 resource-indicator-style
  `aud` claims. Confirmed live and empirically (not just from docs):
  Connect has no login/signup UI of its own — the Standalone Connect flow
  explicitly redirects out to your own login mechanism (i.e., User
  Management) and back, meaning Connect cannot replace User Management as
  kbdb's REST login path. It also cannot be skipped for MCP, since MCP
  clients need DCR, which only Connect provides.

**First-party vs. third-party is the critical Connect configuration.**
Third-party Connect applications are WorkOS's B2B/partner-integration
model (require an `organizationId`, add an `org_id` claim, show an
explicit "authorize this third party" consent screen) — not what kbdb
wants. First-party (created by omitting `organizationId`) is documented
and empirically confirmed as sharing the same user identities as User
Management with no separate consent-screen/org layer. This was verified
end-to-end in a live spike against a real WorkOS Staging environment: a
user logged into User Management, then the same user completed a
first-party Connect OAuth flow, and the resulting tokens' `sub` claims
were byte-for-byte identical (`user_01M03ZBSZJB6FQEPFE7T25T2DC` in all
three tokens obtained — the User Management access token, and the Connect
access token and ID token). **One account, two OAuth entry points** —
confirmed, not assumed.

### `aud` claim behavior (confirmed via live token, corrects an initial
### misreading of WorkOS's docs)

WorkOS's MCP docs describe access tokens as carrying an `aud` claim
"matching the requested `resource`" — read at first as meaning the literal
URI string passed in the `resource=` authorize parameter would be echoed
back. The live spike showed this is not quite right: the Connect access
token's `aud` came back as the **User Management application's
`client_id`** (`client_01M03Y9NBQB88HBSR8F50R1B2B`), not the
`https://api.mykeebs.info` URI passed as `resource=`.

This is correct, not a bug: in WorkOS's data model, "the resource being
protected" is a real, concrete WorkOS application object (kbdb's User
Management app), not an arbitrary self-declared string a client can
freely choose. `aud` therefore asserts "this token is scoped to access
kbdb," anchored to an object WorkOS itself controls — a stronger,
non-spoofable security property than validating against a client-supplied
URI would have been. The authorizer should check `aud` equals kbdb's
User Management application's `client_id`, not any URI.

(The Connect *ID* token's `aud`, separately, is the Connect client's own
`client_id` — standard OIDC behavior, ID tokens are always audienced to
the requesting client. The authorizer only needs the access token.)

## What kbdb builds

### 1. Lambda Authorizer

A `REQUEST`-type API Gateway authorizer (payload format 2.0), replacing
Cognito's `CognitoAuthorizer`, fronting both REST and MCP routes via
`HttpApi.Auth.DefaultAuthorizer`:

- Verifies the JWT: RS256 signature via WorkOS's JWKS (standard
  `go-oidc`/OIDC verification, issuer = kbdb's WorkOS AuthKit domain —
  confirmed to be a real, resolvable per-workspace `*.authkit.app`
  subdomain, or a configured custom domain in production), expiry, and
  `aud` equals kbdb's User Management application's `client_id`.
- Extracts `sub` (the WorkOS user ID) as the identity to pass through to
  kbdb's handlers via the authorizer's returned context.
- Branches on `event.routeKey` against a deploy-time-configured list of
  "anonymous-acceptable" routes (optional-auth REST GETs): Allow with
  anonymous context if no/invalid token there, Deny elsewhere without a
  valid token.
- Per-route scoping: unchanged from kbdb's existing pattern — the two
  fully-public lookup GET routes keep `Authorizer: NONE` (true bypass, no
  authorizer invocation at all); every other route (required-auth REST
  writes, optional-auth REST GETs, all three MCP routes) goes through this
  authorizer.
- No client_id/dedupe/last-used-at tracking of any kind — WorkOS owns all
  of that internally (visible via its own `environmentApplications`
  API/dashboard if ever needed, but not kbdb's concern).

Consequence: `internal/auth` (`Verifier`, `VerifyToken`, `Claims`) and
`internal/auth/mocks` are deleted from kbdb entirely — `functions/api`
never verifies a token itself again, since API Gateway invokes the
authorizer before any request reaches it. `internal/middleware.Auth` and
`middleware.OptionalAuth` stop calling verification in-process; they read
identity/anonymous status from the authorizer's passed-through context.
`internal/mcp`'s tool-level middleware likewise drops its own
`VerifyToken` call. `internal/authz.ReadableVisibilities`/`authz.IsOwner`
are unchanged — still kbdb's job, operating on whatever identity the
authorizer's context supplies.

### 2. WorkOS configuration (dashboard/API, not code)

- One **User Management** application per environment (dev/CI/prod) —
  this is kbdb's real identity directory.
- One **first-party Connect OAuth application** per environment
  (`clientConfidentiality: Public`, `type: OAuth`, no `organizationId`) —
  the DCR/PKCE-capable entry point MCP clients register against.
- Redirect URIs on the Connect application: a default, non-wildcard
  loopback URI plus `http://localhost:*/callback` as an additional
  (non-default) wildcard entry — confirmed live that WorkOS accepts the
  wildcard form but rejects it as the sole/default URI, so both must be
  registered.
- A `.well-known/oauth-protected-resource` metadata route (kbdb still
  serves this itself, per RFC 9728) pointing MCP clients at WorkOS's
  Connect `registration_endpoint`/`authorization_endpoint`/`token_endpoint`,
  discoverable at `https://<workspace-authkit-domain>/.well-known/oauth-authorization-server`.

## Testing

- **Authorizer unit tests**: `testify/suite`, mocked JWKS/verification
  (mirrors the structure `internal/auth`'s existing tests already use, so
  this is largely a port rather than new test design), covering the
  routeKey-branching and Allow/Deny/anonymous-context logic.
- **Functional/end-to-end**: Ginkgo, Given/When/Then. Golden path
  (register a Connect client via DCR → complete a real PKCE login →
  call a protected REST/MCP route → confirm Allow, correct `sub` in
  context) and the redirect_uri-rejection case (WorkOS-side, but worth a
  black-box confirmation).
- No `mockoidc`/floci dependency for any of this — WorkOS's real Staging
  environment (a second, free WorkOS environment, same as kbdb already
  does with a personal dev AWS stack) plays the role LocalStack/floci
  played for Cognito. No local emulation needed since there's no AWS
  service being emulated here at all — WorkOS is Auth-as-a-Service.

## Explicitly out of scope / retired

Everything in `2026-08-15-mcp-dcr-authorizer-design.md`'s phases 1, 3, and
4 (Registration Lambda, Purger, floci migration) — moot, WorkOS owns all
of it. That spec is retained only as the research trail explaining *why*
Cognito was rejected and *why* WorkOS's Connect product (not a
custom-built equivalent) is the right fit.
