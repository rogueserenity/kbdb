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

**Migrate kbdb's identity provider from Cognito to WorkOS AuthKit, and
route every OAuth client — REST and MCP alike — through WorkOS's Connect
product.** WorkOS owns registration, client lifecycle, and the
login/signup UI entirely — kbdb no longer needs to build or operate any
DCR shim, dedupe table, purger, or (as established below) even a custom
authorizer Lambda. What kbdb still owns:

1. **WorkOS configuration**: one User Management application (kbdb's real
   identity directory, providing the hosted login/signup UI every flow
   ultimately redirects to) plus one first-party Connect OAuth application
   (the single OAuth entry point both REST and MCP clients use) — build
   now, as WorkOS dashboard/API configuration, not code.
2. **A native, config-only API Gateway JWT authorizer** (`AuthorizerType:
   JWT`, the same mechanism `CognitoAuthorizer` already used) pointed at
   Connect's issuer and kbdb's `aud` value — no Lambda, no custom
   verification code, for every required-auth route on both REST and MCP.

That's the entire scope. No registration Lambda, no `OAuthClientTable`, no
dedupe logic, no purger, no floci-migration phase, and — confirmed via
live testing below — no authorizer Lambda either. All of that existed
solely to compensate for gaps (Cognito's lack of native DCR, or an
initially-assumed need for custom per-route logic) that turned out not to
apply once the actual WorkOS flows were tested end-to-end.

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

## WorkOS architecture: everything through first-party Connect

WorkOS separates two concerns that are easy to conflate:

- **User Management** (`api.workos.com/user_management/*`) is the actual
  identity system — the one real user directory, signup, login, password
  reset, social login, and the hosted login page every flow below
  ultimately shows the user.
- **Connect** (`https://<workspace>.authkit.app/oauth2/*`) is an OAuth
  surface *layered on top of* the same User Management directory — not a
  separate identity system. It's the product WorkOS built for
  MCP/third-party-application OAuth: DCR, CIMD, PKCE-enforced public
  clients, and a real `aud` claim (which plain User Management tokens
  lack entirely — confirmed by direct comparison of live tokens).

**Initially assumed, then corrected by live testing**: that Connect
requires bringing your own login UI (based on reading the *Standalone*
Connect mode's docs, which do redirect out to your own login mechanism),
which would have forced REST to stay on User Management directly while
only MCP used Connect. **This is wrong for plain (non-Standalone) Connect
first-party OAuth applications** — confirmed live by opening a plain
`/oauth2/authorize` URL against a first-party Connect app and observing
the actual browser output: it shows WorkOS's own hosted AuthKit login
page, identical UX to User Management's own login, with zero custom code.
There is no meaningful UX cost to routing REST through Connect instead of
User Management directly — both land the user on the same hosted login
page.

Given that, and that only Connect-issued tokens carry a checkable `aud`,
the design routes **both REST and MCP through the same first-party
Connect application**. Every client — a REST web app, a REST mobile app,
an MCP client — registers/authenticates against Connect; Connect redirects
to WorkOS's hosted login (backed by User Management) for the actual
credential check; every resulting token has the same issuer and the same
`aud`.

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
tokens obtained — the User Management access token, and multiple Connect
access/ID tokens obtained across two separate live spikes). **One
account, one OAuth entry point (Connect), consistent identity across every
client** — confirmed, not assumed.

### `aud` claim behavior (confirmed via live token, corrects an initial
### misreading of WorkOS's docs)

WorkOS's MCP docs describe access tokens as carrying an `aud` claim
"matching the requested `resource`" — read at first as meaning the literal
URI string passed in the `resource=` authorize parameter would be echoed
back. Live testing showed this is not quite right: the Connect access
token's `aud` came back as the **User Management application's
`client_id`** (`client_01M03Y9NBQB88HBSR8F50R1B2B`) in every test,
including a repeat run with no `resource=` parameter passed at all — `aud`
is deterministic per target application, not derived from a client-
supplied `resource` string.

This is correct, not a bug: in WorkOS's data model, "the resource being
protected" is a real, concrete WorkOS application object (kbdb's User
Management app), not an arbitrary self-declared string a client can
freely choose. `aud` therefore asserts "this token is scoped to access
kbdb," anchored to an object WorkOS itself controls — a stronger,
non-spoofable security property than validating against a client-supplied
URI would have been, and the exact value a native JWT authorizer's
`audience` config should be set to.

(The Connect *ID* token's `aud`, separately, is the Connect client's own
`client_id` — standard OIDC behavior, ID tokens are always audienced to
the requesting client. Only the access token matters for authorization.)

## What kbdb builds

### 1. A native, config-only API Gateway JWT authorizer (no Lambda)

**Initially assumed a custom Lambda authorizer would be needed** — to
verify WorkOS JWTs and branch per-route (required-auth vs. optional-auth
vs. anonymous-acceptable). Re-examined once the `aud` behavior above was
confirmed: since every Connect-issued token (REST or MCP) shares one
issuer and one `aud` value, this is exactly what API Gateway's built-in
`AuthorizerType: JWT` mechanism handles natively — the same mechanism
`CognitoAuthorizer` already used, requiring only a `JwtConfiguration`
block (`issuer`, `audience`) in `template.yaml`. No Lambda, no custom
verification code, no code to test or maintain for this piece at all.

Configuration:
- `issuer`: kbdb's WorkOS Connect issuer — the per-workspace
  `*.authkit.app` domain (or a configured custom domain in production).
- `audience`: kbdb's User Management application's `client_id` (the value
  confirmed above).
- Applied as `HttpApi.Auth.DefaultAuthorizer`, replacing Cognito's
  `CognitoAuthorizer` definition entirely.

Per-route scoping is otherwise unchanged from kbdb's existing pattern:
the two fully-public lookup GET routes keep `Authorizer: NONE` (true
bypass); required-auth REST writes and all three MCP routes pick up the
new default authorizer automatically (no per-route override needed, same
as they did under `CognitoAuthorizer` today).

**One gap a native JWT authorizer cannot cover**: optional-auth REST GET
routes (list/get switches, keyboards, etc.) need "allow anonymous, but use
identity if a valid token is present" — a native JWT authorizer is
strictly allow-if-valid/deny-otherwise, it cannot pass through an
anonymous-but-allowed request. These routes keep `Authorizer: NONE` and
their own in-process verification exactly as today, which means a slice
of `internal/auth`'s verification logic survives for this one case rather
than being deleted outright (see below).

No client_id/dedupe/last-used-at tracking of any kind is needed anywhere
in this design — WorkOS owns all of that internally (visible via its own
`environmentApplications` API/dashboard if ever needed, but not kbdb's
concern).

Consequence: `internal/auth`'s `Verifier`/`VerifyToken` keep exactly one
caller — `middleware.OptionalAuth`, for the routes above — reconfigured to
verify against WorkOS's issuer/JWKS instead of Cognito's. Every other
caller goes away: `internal/middleware.Auth` (required-auth REST) no
longer calls it at all, since the native authorizer now gates those
routes before the request reaches `functions/api`.

**Explicitly re-examined, not silently dropped**: `middleware.Auth`'s
existing doc comment describes its in-process verification as deliberate
defense-in-depth, "independently of any upstream API Gateway authorizer."
That reasoning predates this design and was re-considered on its own
merits (not just carried over) — decided to drop it: the native JWT
authorizer is the same class of mechanism `CognitoAuthorizer` already was
(AWS-verified signature/issuer/audience/expiry before a request ever
reaches `functions/api`), so re-verifying the identical token in-process
adds cost without covering a materially different failure mode.
`internal/mcp`'s tool-level middleware likewise drops its own
`VerifyToken` call, since MCP routes are now fronted by the same native
authorizer as REST writes. `internal/authz.ReadableVisibilities`/
`authz.IsOwner` are unchanged — still kbdb's job, operating on whatever
identity is available (from the
native authorizer's context on gated routes, or from the surviving
in-process check on optional-auth routes).

### 2. WorkOS configuration (dashboard/API, not code)

- One **User Management** application per environment (dev/CI/prod) —
  this is kbdb's real identity directory and the source of the hosted
  login/signup page. Not talked to directly by any client; exists to
  back Connect's login step and to supply the `client_id` used as the
  native authorizer's `audience` value.
- One **first-party Connect OAuth application** per environment
  (`clientConfidentiality: Public`, `type: OAuth`, no `organizationId`) —
  the single OAuth entry point every client (REST and MCP alike)
  registers/authenticates against. DCR/PKCE-capable, confirmed to show
  WorkOS's normal hosted login UI with no custom code required.
- Redirect URIs on the Connect application: a default, non-wildcard
  loopback URI plus `http://localhost:*/callback` as an additional
  (non-default) wildcard entry, for MCP/CLI clients — confirmed live that
  WorkOS accepts the wildcard form but rejects it as the sole/default URI,
  so both must be registered. A REST client (browser/mobile) would
  register its own real (non-loopback) redirect URI(s) the same way.
- A `.well-known/oauth-protected-resource` metadata route (kbdb still
  serves this itself, per RFC 9728) pointing clients at Connect's
  `registration_endpoint`/`authorization_endpoint`/`token_endpoint`,
  discoverable at `https://<workspace-authkit-domain>/.well-known/oauth-authorization-server`.

## Testing

- **REST/MCP handler tests**: unchanged — `internal/handlers`,
  `internal/mcp` tests don't need to know or care that auth moved, since
  they never called `internal/auth` directly.
- **`middleware.OptionalAuth` unit tests**: `testify/suite`, mocked
  JWKS/verification (this is a reconfiguration of `internal/auth`'s
  existing test structure — verifying against WorkOS's issuer instead of
  Cognito's — not new test design).
- **Functional/end-to-end**: Ginkgo, Given/When/Then. Golden path
  (register a Connect client via DCR → complete a real PKCE login →
  call a protected REST route and a protected MCP route → confirm both
  Allow via the native authorizer, correct `sub` available to the
  handler) and the redirect_uri-rejection case (WorkOS-side, but worth a
  black-box confirmation). Also covers the optional-auth-route case
  (anonymous request succeeds with public-only visibility; authenticated
  request succeeds with full visibility) against the surviving in-process
  path.

### Local and CI both replace `mockoidc`, but with different mechanisms —
### deliberately, not an inconsistency

`mockoidc` (`test/functional/support/mockoidc/`) is retired entirely —
both its role as the OIDC issuer and its `cognito:groups`-claim shim
(Cognito-specific, no WorkOS equivalent needed). What replaces it differs
by environment, because the two environments have a materially different
constraint:

**Local (`func-setup`/`sam local start-api`)**: run **WorkOS's own
official emulator**, `@workos/emulate` (npm package; MIT-licensed;
verified live — real RS256-signed, JWKS-verifiable JWTs; real
`user_management/authorize`, `.../authenticate`, `.../users` HTTP
surface), as a **Docker container** — `ghcr.io/workos/emulate`, published
by WorkOS with every release, alongside `docker-compose.yml`'s existing
services. Confirmed via the image's own docs: it binds `0.0.0.0` (reachable
from sibling containers, same requirement `mockoidc`'s own code comments
document for itself today) and supports a mounted seed config
(`workos-emulate.config.yaml`) that can pre-declare a stable Connect OAuth
application (fixed `client_id`, `audience`) — avoiding any "look up the
dynamically-generated client_id" step in local dev, same convenience
`mockoidc`'s fixed `TestClientID` currently provides. Test users can
additionally be created ad hoc via a plain `POST /user_management/users`
call against the running container (confirmed live — this is exactly how
`fixtures.TestUserSubject`-style dynamic identities work today), so
`QueueUser`/`MintToken` in `test/functional/support/api/token.go` port
over as "call the emulator's real HTTP API" instead of driving
`mockoidc`'s bespoke `/test/queue-user` control endpoint. There is no
reachability problem locally — the emulator container and
`sam local start-api` share the same Docker network, exactly like
`mockoidc` does today — so the *live, full* emulator is used, not a
static/pre-baked shortcut. This also unlocks `--interactive` mode (real
hosted login pages) for manual dev-loop testing, something `mockoidc`
never offered.

**CI (`functional-test` job)**: cannot use the live emulator the same
way, because CI's structure is fundamentally different from local dev —
`sam deploy` puts the API behind a **real, publicly-reachable AWS API
Gateway** (confirmed by reading `.github/workflows/ci.yml`: CI deploys a
real `kbdb-pr-<N>` stack and calls *into* it from the runner, the reverse
of local dev's same-machine setup). API Gateway's native JWT authorizer
needs to fetch JWKS via an outbound HTTPS call to the configured
`issuer`/`jwks_uri` — and a GitHub-hosted runner has no stable public
IP/DNS a deployed AWS resource can reach back into. Investigated and
rejected: tunneling (ngrok/Cloudflare Tunnel) works but hands back a
fresh random URL every run, forcing the deploy step to depend on a
started-tunnel's dynamic output and adding an external, non-AWS service
as a flake source on every PR; self-hosted runners solve reachability
trivially but trade it for real ongoing cost/maintenance disproportionate
to this one problem.

**Adopted instead**: `@workos/emulate` supports a **pinned signing key**
(`--signing-key <pem>` / `WORKOS_EMULATE_SIGNING_KEY`, confirmed via
`--help` and the image's own docs) and a **pinned issuer**
(`--issuer <url>` / `WORKOS_EMULATE_ISSUER`) — meaning its JWKS is fully
deterministic and computable *without running the emulator at all*, from
just the public half of a keypair. CI generates (or uses a checked-in
test) RSA keypair, derives the corresponding static JWKS JSON, and
publishes it (plus a static `openid-configuration` document) to a
public/pre-signed S3 location at a stable, known URL *before* `sam
deploy` runs — so the stack's `JwtConfiguration.issuer`/discovery can be
set at deploy time, no dynamic-URL ordering problem. This is a real,
AWS-community-precedented pattern (not novel to kbdb): API Gateway's JWT
authorizer only requires `jwks_uri` to be *some* static, publicly
reachable HTTPS endpoint serving standard JWKS JSON — it has no concept
of "live OIDC issuer" beyond that. The CI runner itself still runs
`@workos/emulate` locally (same pinned key) purely to *mint* tokens for
test users via its HTTP API — that direction of traffic (runner calling
out to a process on the same runner) has no reachability problem, only
the reverse direction (AWS calling into the runner) did, which this
approach avoids needing entirely.

If this proves more complex in practice than it looks on paper, the
documented fallback (a recognized, common testing-pyramid tradeoff, not
unique to kbdb) is to not exercise the real authorizer/API-Gateway config
on every PR — verify it on a slower, less frequent tier (e.g.
merge-to-main only) while PR-level functional tests exercise the
Lambda/business logic through a path that doesn't require real API
Gateway JWT verification. Not adopted as the primary plan, noted as a
pressure-release valve if the S3-JWKS approach turns out to be more
trouble than it's worth.

## Explicitly out of scope / retired

Everything in `2026-08-15-mcp-dcr-authorizer-design.md`'s phases 1, 3, and
4 (Registration Lambda, Purger, floci migration) — moot, WorkOS owns all
of it. That spec is retained only as the research trail explaining *why*
Cognito was rejected and *why* WorkOS's Connect product (not a
custom-built equivalent) is the right fit.
