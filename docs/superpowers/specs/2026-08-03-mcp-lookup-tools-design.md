# MCP lookup tools: `list_lookups` / `get_lookup`

## Context

`internal/mcp` currently exposes exactly one tool, `ping` (`internal/mcp/ping.go`), which was a proof-of-concept to confirm the MCP transport, auth middleware, and identity wiring worked end-to-end — not a pattern to replicate. `ping` takes no input, calls no repository, and returns a hardcoded string. It doesn't answer any of the questions real tools need to answer: how a tool handler gets a repository, what shape typed input/output takes, how errors surface, or how a tool that reads another user's data enforces the same visibility rules REST does.

This design covers two tools — `list_lookups` and `get_lookup`, mirroring REST's unauthenticated `GET /v1/lookups` and `GET /v1/lookups/{category}` — but its real purpose is to establish the pattern every future entity tool (switches, keyboards, keycap sets, builds) will follow. That broader context is captured here even though only the two lookup tools are built in this PR, so the next tool doesn't have to re-derive these decisions.

## Package layout

Three packages, mirroring the existing `repository` / `repoapi` / `handlers/api` split used by REST:

```
internal/repository/          (existing) DB-shaped types: repository.Lookup, repository.Switch, ...
internal/mcp/schema/          (NEW) hand-written MCP wire-shaped types, jsonschema-tagged
internal/repomcp/             (NEW) repository.X -> schema.X mapping functions
internal/mcp/                 (existing) tool registration + handlers; imports schema + repomcp
```

- **`internal/mcp/schema`** holds every tool's input/output struct (`GetLookupInput`, `GetLookupOutput`, `ListLookupsInput`, `ListLookupsOutput`, and later `ListSwitchesInput`, etc.), one file per entity (`lookup.go`, later `switch.go`, `keycap_set.go`...). These are **not** generated — REST's `internal/handlers/api` types come from `api/openapi.yaml` via oapi-codegen and only carry `json` tags, no `jsonschema` tags, so they can't be reused directly as MCP tool schemas without either hand-editing generated code (not allowed) or losing per-field descriptions (bad tool UX — tool schemas are what an LLM client reads to decide how to call a tool). A separate hand-written package is the same trade a REST-generated-vs-repository-shaped split already makes elsewhere in this codebase. `schema` depends on nothing internal — plain structs and tags only.
- **`internal/repomcp`** is `internal/repoapi`'s direct counterpart: one file per entity, pure mapping functions (`LookupToMCP(repository.Lookup) (schema.GetLookupOutput, error)`), same reasoning as `repoapi.LookupToAPI` — decode typed categories (`keyboard_layout`, `case_mount_type`) via `Lookup.LayoutValues()`/`CaseMountTypeValues()` here too, so a shape mismatch in stored data errors in the mapping layer, not silently later. Depends on `repository` and `mcp/schema`; nothing depends on it except `internal/mcp`.
- **`internal/mcp`** keeps tool registration and handler logic only — no wire-shape struct definitions live here anymore (this is the one departure from `ping.go`'s current all-in-one-file shape, made deliberately non-scalable-by-example since `ping` predates this decision).

## Repo wiring

`mcp.New` and `registerTools` gain a `repository.LookupRepository` parameter, following the same "handler takes the repos it needs" shape as `internal/handlers` (e.g. `handlers.ListSwitches(repo) http.HandlerFunc`):

```go
func New(verifier *auth.Verifier, lookupRepo repository.LookupRepository, issuerURL, version string) Handlers { ... }

func registerTools(s *mcp.Server, lookupRepo repository.LookupRepository) {
    mcp.AddTool(s, pingTool, handlePing)
    mcp.AddTool(s, listLookupsTool, handleListLookups(lookupRepo))
    mcp.AddTool(s, getLookupTool, handleGetLookup(lookupRepo))
}
```

`internal/router/router.go`'s call site (`mcp.New(verifier, issuerURL, version)`) updates to pass `lookupRepo`, which is already an existing parameter to `router.New`.

## Typed tool handlers

Uses the SDK's `mcp.AddTool[In, Out any]` (confirmed in `go-sdk v1.7.0`'s `mcp/server.go`), which infers each tool's JSON input/output schema from the Go struct passed as the type parameter — including per-field descriptions from `jsonschema` struct tags. This is why `schema` structs need to be hand-written rather than reused from `api`.

```go
// internal/mcp/schema/lookup.go
type GetLookupInput struct {
    Category string `json:"category" jsonschema:"the lookup category name, e.g. switch_type or vendor"`
}

type GetLookupOutput struct {
    Category string `json:"category"`
    Values   []any  `json:"values"`
}

type ListLookupsOutput struct {
    Categories []string `json:"categories"`
}
```

`ListLookupsInput` is an empty struct (no arguments) — `list_lookups` takes nothing, matching REST's `GET /v1/lookups`.

```go
// internal/mcp/lookups.go
var listLookupsTool = &mcp.Tool{
    Name:        "list_lookups",
    Description: "Lists every lookup category name (e.g. switch_type, vendor, keyboard_size). Call get_lookup with a category name to see its approved values.",
}

var getLookupTool = &mcp.Tool{
    Name:        "get_lookup",
    Description: "Returns the approved values for one lookup category.",
}

func handleListLookups(repo repository.LookupRepository) mcp.ToolHandlerFor[schema.ListLookupsInput, schema.ListLookupsOutput] {
    return func(ctx context.Context, _ *mcp.CallToolRequest, _ schema.ListLookupsInput) (*mcp.CallToolResult, schema.ListLookupsOutput, error) {
        categories, err := repo.ListCategories(ctx)
        if err != nil {
            log.FromContext(ctx).Error("listing lookup categories", log.Error, err)
            return nil, schema.ListLookupsOutput{}, errors.New("failed to list lookup categories")
        }
        return nil, schema.ListLookupsOutput{Categories: categories}, nil
    }
}

func handleGetLookup(repo repository.LookupRepository) mcp.ToolHandlerFor[schema.GetLookupInput, schema.GetLookupOutput] {
    return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetLookupInput) (*mcp.CallToolResult, schema.GetLookupOutput, error) {
        lookup, err := repo.GetCategory(ctx, in.Category)
        if errors.Is(err, repository.ErrNotFound) {
            return nil, schema.GetLookupOutput{}, fmt.Errorf("lookup category %q not found", in.Category)
        }
        if err != nil {
            log.FromContext(ctx).Error("getting lookup category", log.Error, err, log.LookupCategory, in.Category)
            return nil, schema.GetLookupOutput{}, errors.New("failed to get lookup category")
        }

        out, err := repomcp.LookupToMCP(*lookup)
        if err != nil {
            log.FromContext(ctx).Error("mapping lookup category to MCP shape", log.LookupCategory, in.Category, log.Error, err)
            return nil, schema.GetLookupOutput{}, errors.New("failed to get lookup category")
        }
        return nil, out, nil
    }
}
```

Error convention: confirmed via `go-sdk`'s `mcp/tool.go` that a non-nil `error` return from a `ToolHandlerFor` automatically sets `CallToolResult.IsError = true` — an in-band MCP tool error, not a transport-level failure. This is the correct layer for "category not found" / "internal failure" the same way `problem.NotFound`/`problem.Internal` are for REST; it's distinct from `requireBearerToken`'s real HTTP 401 for auth failures, which happens before any tool handler runs. Error message text returned to the client should stay generic for internal failures (mirroring `problem.Internal`'s "failed to X" bodies) — the detailed error goes to the log, not the client.

## Forward-looking: entity tools with a target user

Lookups have no owner concept, so `list_lookups`/`get_lookup` need no caller-identity handling beyond what `identityMiddleware` already puts on `ctx`. Future tools over owned, visibility-scoped entities (switches, keyboards, keycap sets) are different, and worth deciding now so the pattern is consistent when they're built:

- **List/get tools take a `user_id` field in their input struct**, mirroring REST's `{userId}` path parameter — an MCP caller must be able to ask about another user's public/shared items, the same as an anonymous or cross-user REST caller can. Example: `schema.ListSwitchesInput{ UserID string, Cursor string }`.
- **These handlers reuse `internal/authz` directly** — `authz.ReadableVisibilities(ctx, ownerID)` for list, `authz.CanReadVisibility(ctx, ownerID, item.Visibility)` for get (404-shaped-as-not-found in the tool error, not a distinguishable "exists but forbidden" response, matching REST's not-403 masking) — so an MCP caller gets identical visibility rules to the equivalent REST call. No new authz logic gets built for MCP; it's the same package, same functions, same ctx-derived caller identity (`ctxpkg.UserID`, set by `identityMiddleware` from the verified bearer token).
- **Write tools (create/update/delete) take no `user_id` field** — they're implicitly self-scoped, matching REST's `authz.IsOwner(ctx, ownerID)` check on `CreateSwitch`/`UpdateSwitch`/`DeleteSwitch`. There's no legitimate case for an MCP client to write into another user's collection, so the schema doesn't even expose a field that could express that request.

This section is guidance for the next tool PR, not implemented here.

## Testing

`internal/mcp/lookups_test.go`, `internal/repomcp/lookup_test.go` — `testify/suite`, per CLAUDE.md, mirroring the existing `internal/handlers/lookups_test.go` coverage (found/not-found/repo-error cases) using `internal/repository/mocks.MockLookupRepository`, the same mock REST handler tests already use.

## Verification

- `mise run lint` / `mise run test` / `mise run check-generated`
- `sam validate --lint` (no template changes expected)
- Full local functional run: `mise run func-setup` → `mise run func-test` → `mise run func-teardown` — add MCP-layer Ginkgo coverage for `list_lookups`/`get_lookup` alongside the existing `ping` MCP functional specs, following the same Given/Context/When/Then structure CLAUDE.md requires.

## Out of scope this PR

- Any switches/keyboards/keycap-sets/builds MCP tools themselves.
- Write (create/update/delete) lookup MCP tools — REST's admin-only `POST/PUT/DELETE /v1/lookups/{category}` has no MCP equivalent requested; only list/get.
