# MCP Lookup Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two MCP tools, `list_lookups` and `get_lookup`, mirroring REST's `GET /v1/lookups` and `GET /v1/lookups/{category}`, while establishing the `internal/mcp/schema` + `internal/repomcp` package pattern future entity tools will reuse.

**Architecture:** Three new/extended packages: `internal/mcp/schema` (hand-written, jsonschema-tagged tool input/output structs — no generation step, since REST's generated `internal/handlers/api` types lack `jsonschema` tags), `internal/repomcp` (repository→schema mapping, mirroring `internal/repoapi`), and `internal/mcp` (tool registration/handlers, gains a `repository.LookupRepository` dependency). `mcp.AddTool[In, Out]`'s generic schema inference (confirmed in `go-sdk` v1.7.0) drives the wire schema from the Go structs directly.

**Tech Stack:** Go, `github.com/modelcontextprotocol/go-sdk` v1.7.0 (`mcp.AddTool`), `testify/suite` (unit tests), Ginkgo/Gomega (functional tests).

## Global Constraints

- Unit tests under `internal/` use `testify/suite`, not bare `func TestX(t *testing.T)` + `t.Run` tables (CLAUDE.md).
- `test/functional/` specs use Ginkgo, with strict Given/When/Then nesting — one `Context` per precondition, one `When` per action, never multiple `When`s differing only in precondition under the same `Context` (CLAUDE.md).
- Generated mocks (`internal/repository/mocks/`) are regenerated via `mise exec -- mockery`, never hand-edited.
- Default to no comments; only add one when the WHY is non-obvious. Non-exported identifiers get comments only when genuinely non-obvious — most need none.
- Package docs live in their own `doc.go` file (`// Package x ...` + `package x`, nothing else), not stapled onto whichever source file happens to be first.
- Run `mise run lint` / `mise run test` / `mise run check-generated` before considering any task done; `mise run func-setup` → `mise run func-test` → `mise run func-teardown` for the functional suite.

---

### Task 1: `internal/mcp/schema` package — lookup input/output types

**Files:**
- Create: `internal/mcp/schema/doc.go`
- Create: `internal/mcp/schema/lookup.go`

**Interfaces:**
- Produces: `schema.ListLookupsInput` (empty struct), `schema.ListLookupsOutput{ Categories []string }`, `schema.GetLookupInput{ Category string }`, `schema.GetLookupOutput{ Category string; Values []any }` — all with `json` tags; `Category`/`Values` fields additionally carry `jsonschema` description tags. These are consumed by Task 3 (`internal/repomcp`) and Task 4 (`internal/mcp` tool handlers).

This task has no test of its own — it's a pure type declaration, verified by the compiler once Task 3/4 use it.

- [ ] **Step 1: Write the package doc**

```go
// internal/mcp/schema/doc.go

// Package schema holds MCP tool input/output types. Kept separate from
// internal/handlers/api's generated REST types since mcp.AddTool infers
// tool schemas from jsonschema struct tags, which the generated types don't
// carry.
package schema
```

- [ ] **Step 2: Write the types**

```go
// internal/mcp/schema/lookup.go
package schema

type ListLookupsInput struct{}

type ListLookupsOutput struct {
	Categories []string `json:"categories" jsonschema:"every lookup category name"`
}

type GetLookupInput struct {
	Category string `json:"category" jsonschema:"the lookup category name, e.g. switch_type or vendor"`
}

type GetLookupOutput struct {
	Category string `json:"category" jsonschema:"the lookup category name"`
	Values   []any  `json:"values" jsonschema:"the category's approved values"`
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/mcp/schema/...`
Expected: no output (success).

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/schema/doc.go internal/mcp/schema/lookup.go
git commit -m "feat(mcp): add schema.ListLookups/GetLookup input/output types"
```

---

### Task 2: `internal/repomcp` package — `LookupToMCP` mapping + tests

**Files:**
- Create: `internal/repomcp/doc.go`
- Create: `internal/repomcp/lookup.go`
- Test: `internal/repomcp/lookup_test.go`

**Interfaces:**
- Consumes: `schema.GetLookupOutput` (Task 1); `repository.Lookup`, `repository.Lookup.LayoutValues()`, `repository.Lookup.CaseMountTypeValues()`, `repository.CategoryKeyboardLayout`, `repository.CategoryBuildCaseMountType` (all existing, `internal/repository/lookup.go`).
- Produces: `repomcp.LookupToMCP(l repository.Lookup) (schema.GetLookupOutput, error)` — consumed by Task 4's `get_lookup` handler.

This mirrors `internal/repoapi/lookup.go`'s `LookupToAPI` exactly (same typed-decode-then-map shape), just targeting `schema.GetLookupOutput` instead of `api.Lookup`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/repomcp/lookup_test.go
package repomcp_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type LookupToMCPSuite struct {
	suite.Suite
}

func TestLookupToMCPSuite(t *testing.T) {
	suite.Run(t, new(LookupToMCPSuite))
}

func (s *LookupToMCPSuite) TestPlainStringCategory_PassesValuesThrough() {
	out, err := repomcp.LookupToMCP(repository.Lookup{Category: "vendor", Values: []any{"a", "b"}})

	s.Require().NoError(err)
	s.Equal(schema.GetLookupOutput{Category: "vendor", Values: []any{"a", "b"}}, out)
}

func (s *LookupToMCPSuite) TestKeyboardLayout_DecodesTypedValues() {
	in := repository.Lookup{
		Category: repository.CategoryKeyboardLayout,
		Values:   []any{map[string]any{"name": "WK", "sizes": []any{"60%", "65%"}}},
	}

	out, err := repomcp.LookupToMCP(in)

	s.Require().NoError(err)
	s.Equal(repository.CategoryKeyboardLayout, out.Category)
	s.Equal([]any{repository.LayoutValue{Name: "WK", Sizes: []string{"60%", "65%"}}}, out.Values)
}

func (s *LookupToMCPSuite) TestKeyboardLayout_WrongShape_Errors() {
	in := repository.Lookup{Category: repository.CategoryKeyboardLayout, Values: []any{"WK"}}

	_, err := repomcp.LookupToMCP(in)

	s.Require().Error(err)
}

func (s *LookupToMCPSuite) TestBuildCaseMountType_DecodesTypedValues() {
	in := repository.Lookup{
		Category: repository.CategoryBuildCaseMountType,
		Values:   []any{map[string]any{"name": "Top Mount", "supports_durometer": true}},
	}

	out, err := repomcp.LookupToMCP(in)

	s.Require().NoError(err)
	s.Equal(repository.CategoryBuildCaseMountType, out.Category)
	s.Equal([]any{repository.CaseMountTypeValue{Name: "Top Mount", SupportsDurometer: true}}, out.Values)
}

func (s *LookupToMCPSuite) TestBuildCaseMountType_WrongShape_Errors() {
	in := repository.Lookup{Category: repository.CategoryBuildCaseMountType, Values: []any{"Top Mount"}}

	_, err := repomcp.LookupToMCP(in)

	s.Require().Error(err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/repomcp/... -v`
Expected: FAIL — `repomcp` package doesn't exist yet (build failure).

- [ ] **Step 3: Write the package doc**

```go
// internal/repomcp/doc.go

// Package repomcp maps internal/repository's DB-shaped types to
// internal/mcp/schema's MCP tool types, mirroring internal/repoapi's
// relationship to internal/handlers/api for the MCP transport instead of REST.
package repomcp
```

- [ ] **Step 4: Write the implementation**

```go
// internal/repomcp/lookup.go
package repomcp

import (
	"fmt"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// LookupToMCP decodes CategoryKeyboardLayout/CategoryBuildCaseMountType into
// their typed shape first, so a mismatch in stored data errors here instead
// of silently returning whatever was actually stored.
func LookupToMCP(l repository.Lookup) (schema.GetLookupOutput, error) {
	values := l.Values

	switch l.Category {
	case repository.CategoryKeyboardLayout:
		layouts, err := l.LayoutValues()
		if err != nil {
			return schema.GetLookupOutput{}, fmt.Errorf("decoding %s values: %w", l.Category, err)
		}
		values = toAnySlice(layouts)
	case repository.CategoryBuildCaseMountType:
		mountTypes, err := l.CaseMountTypeValues()
		if err != nil {
			return schema.GetLookupOutput{}, fmt.Errorf("decoding %s values: %w", l.Category, err)
		}
		values = toAnySlice(mountTypes)
	}

	return schema.GetLookupOutput{
		Category: l.Category,
		Values:   values,
	}, nil
}

func toAnySlice[T any](items []T) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}

	return out
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/repomcp/... -v`
Expected: PASS, all 5 subtests.

- [ ] **Step 6: Commit**

```bash
git add internal/repomcp/doc.go internal/repomcp/lookup.go internal/repomcp/lookup_test.go
git commit -m "feat(mcp): add repomcp.LookupToMCP mapping"
```

---

### Task 3: `internal/mcp` — wire `LookupRepository` into `New`/`registerTools`

**Files:**
- Modify: `internal/mcp/server.go` (the `New` function, around line 69)
- Modify: `internal/mcp/ping.go` (the `registerTools` function)
- Modify: `internal/router/router.go:110` (the `mcp.New(...)` call site)

**Interfaces:**
- Consumes: `repository.LookupRepository` (existing interface, `internal/repository/lookup.go`). `router.New` already receives `lookupRepo repository.LookupRepository` as a parameter (`internal/router/router.go:35`) — this task just threads that existing value one level further, into `mcp.New`.
- Produces: `mcp.New(verifier *auth.Verifier, lookupRepo repository.LookupRepository, issuerURL, version string) Handlers` (signature change) and `registerTools(s *sdkmcp.Server, lookupRepo repository.LookupRepository)` (signature change) — consumed by Task 4, which adds the two new `mcp.AddTool` calls inside `registerTools`.

No new test in this task — `internal/mcp/server_test.go` doesn't exist yet (no MCP unit tests currently; `ping.go` has none either), and this is a pure signature-threading change with no new behavior. Verified by `go build` and the existing MCP functional `ping` suite still passing (confirms `New`'s behavior for the existing tool is unchanged).

- [ ] **Step 1: Update `internal/mcp/server.go`**

Change the `New` function signature and its call to `registerTools`:

```go
// New builds the MCP server. issuerURL is the OIDC issuer MCP clients should
// authenticate against, advertised via RFC 9728 Protected Resource Metadata.
// version is advertised to MCP clients on connect.
func New(verifier *auth.Verifier, lookupRepo repository.LookupRepository, issuerURL, version string) Handlers {
	mcpServer := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "kbdb", Version: version}, nil)
	mcpServer.AddReceivingMiddleware(identityMiddleware())

	registerTools(mcpServer, lookupRepo)

	streamable := sdkmcp.NewStreamableHTTPHandler(
		func(*http.Request) *sdkmcp.Server { return mcpServer },
		&sdkmcp.StreamableHTTPOptions{
			Stateless:                  true,
			DisableLocalhostProtection: true,
		},
	)

	return Handlers{
		Streamable:       requireBearerToken(verifier, streamable),
		MetadataPath:     MetadataPath,
		RootMetadataPath: RootMetadataPath,
		Metadata:         metadataHandler(issuerURL),
	}
}
```

Add the import:

```go
"github.com/rogueserenity/kbdb/internal/repository"
```

- [ ] **Step 2: Update `internal/mcp/ping.go`'s `registerTools`**

```go
func registerTools(s *mcp.Server, lookupRepo repository.LookupRepository) {
	mcp.AddTool(s, pingTool, handlePing)
}
```

Add the import: `"github.com/rogueserenity/kbdb/internal/repository"`. (`lookupRepo` is unused until Task 4 adds the lookup tools — this will fail `go vet`/lint as an unused parameter; that's expected and resolved by Task 4, which is the very next task. Do not add a `_ = lookupRepo` workaround.)

- [ ] **Step 3: Update `internal/router/router.go:110`**

```go
mcpHandlers := mcp.New(verifier, lookupRepo, issuerURL, version)
```

- [ ] **Step 4: Verify it builds (lint will flag the unused param — expected, see Task 4)**

Run: `go build ./...`
Expected: no output (success) — `go build` doesn't flag unused function parameters, only unused local variables/imports, so this succeeds even though `lookupRepo` isn't read yet.

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/ping.go internal/router/router.go
git commit -m "feat(mcp): thread LookupRepository into mcp.New/registerTools"
```

---

### Task 4: `internal/mcp/lookups.go` — `list_lookups`/`get_lookup` tools + unit tests

**Files:**
- Create: `internal/mcp/lookups.go`
- Test: `internal/mcp/lookups_test.go`
- Modify: `internal/mcp/ping.go` (`registerTools` — add the two new `mcp.AddTool` calls)

**Interfaces:**
- Consumes: `schema.ListLookupsInput/Output`, `schema.GetLookupInput/Output` (Task 1); `repomcp.LookupToMCP` (Task 2); `repository.LookupRepository.ListCategories(ctx)`, `.GetCategory(ctx, category)`, `repository.ErrNotFound` (existing); `log.FromContext(ctx)`, `log.Error`, `log.LookupCategory` (existing, `internal/log/log.go`); `internal/repository/mocks.MockLookupRepository` (existing generated mock, same one `internal/handlers/lookups_test.go` uses).
- Produces: `listLookupsTool`, `getLookupTool` (`*sdkmcp.Tool`), `handleListLookups(repo) sdkmcp.ToolHandlerFor[schema.ListLookupsInput, schema.ListLookupsOutput]`, `handleGetLookup(repo) sdkmcp.ToolHandlerFor[schema.GetLookupInput, schema.GetLookupOutput]` — registered into `registerTools` in this same task (no later task depends on these names further).

- [ ] **Step 1: Write the failing tests**

```go
// internal/mcp/lookups_test.go
package mcp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type HandleListLookupsSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
}

func TestHandleListLookupsSuite(t *testing.T) {
	suite.Run(t, new(HandleListLookupsSuite))
}

func (s *HandleListLookupsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
}

func (s *HandleListLookupsSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		ListCategories(mock.Anything).
		Return([]string{"vendor", "switch_type"}, nil)

	handler := handleListLookups(s.mockRepo)
	result, out, err := handler(s.T().Context(), nil, schema.ListLookupsInput{})

	s.Require().NoError(err)
	s.Nil(result)
	s.Equal(schema.ListLookupsOutput{Categories: []string{"vendor", "switch_type"}}, out)
}

func (s *HandleListLookupsSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		ListCategories(mock.Anything).
		Return(nil, errors.New("scan failed"))

	handler := handleListLookups(s.mockRepo)
	_, _, err := handler(s.T().Context(), nil, schema.ListLookupsInput{})

	s.Require().Error(err)
}

type HandleGetLookupSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
}

func TestHandleGetLookupSuite(t *testing.T) {
	suite.Run(t, new(HandleGetLookupSuite))
}

func (s *HandleGetLookupSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
}

func (s *HandleGetLookupSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(&repository.Lookup{Category: "vendor", Values: []any{"a", "b"}}, nil)

	handler := handleGetLookup(s.mockRepo)
	result, out, err := handler(s.T().Context(), nil, schema.GetLookupInput{Category: "vendor"})

	s.Require().NoError(err)
	s.Nil(result)
	s.Equal(schema.GetLookupOutput{Category: "vendor", Values: []any{"a", "b"}}, out)
}

func (s *HandleGetLookupSuite) TestNotFound_ReturnsError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleGetLookup(s.mockRepo)
	_, _, err := handler(s.T().Context(), nil, schema.GetLookupInput{Category: "missing"})

	s.Require().Error(err)
}

func (s *HandleGetLookupSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(nil, errors.New("get item failed"))

	handler := handleGetLookup(s.mockRepo)
	_, _, err := handler(s.T().Context(), nil, schema.GetLookupInput{Category: "vendor"})

	s.Require().Error(err)
}

func (s *HandleGetLookupSuite) TestKeyboardLayout_CorruptStoredShape_ReturnsError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardLayout).
		Return(&repository.Lookup{Category: repository.CategoryKeyboardLayout, Values: []any{"WK"}}, nil)

	handler := handleGetLookup(s.mockRepo)
	_, _, err := handler(s.T().Context(), nil, schema.GetLookupInput{Category: repository.CategoryKeyboardLayout})

	s.Require().Error(err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/mcp/... -run 'HandleListLookups|HandleGetLookup' -v`
Expected: FAIL — `handleListLookups`/`handleGetLookup` undefined.

- [ ] **Step 3: Write the implementation**

```go
// internal/mcp/lookups.go
package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

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

No doc.go changes here: `internal/mcp` already exists with its package doc in `server.go`.

- [ ] **Step 4: Register the tools in `internal/mcp/ping.go`'s `registerTools`**

```go
func registerTools(s *mcp.Server, lookupRepo repository.LookupRepository) {
	mcp.AddTool(s, pingTool, handlePing)
	mcp.AddTool(s, listLookupsTool, handleListLookups(lookupRepo))
	mcp.AddTool(s, getLookupTool, handleGetLookup(lookupRepo))
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/mcp/... -v`
Expected: PASS, all subtests (existing + new).

- [ ] **Step 6: Run full verification**

Run: `mise run lint && mise run test`
Expected: both clean/passing.

- [ ] **Step 7: Commit**

```bash
git add internal/mcp/lookups.go internal/mcp/lookups_test.go internal/mcp/ping.go
git commit -m "feat(mcp): add list_lookups and get_lookup tools"
```

---

### Task 5: Functional tests — `list_lookups`/`get_lookup` over a live MCP server

**Files:**
- Create: `test/functional/features/mcp/lookups/lookups_suite_test.go`
- Create: `test/functional/features/mcp/lookups/lookups_list_test.go`
- Create: `test/functional/features/mcp/lookups/lookups_get_test.go`

**Interfaces:**
- Consumes: `support.BaseURL()`, `api.NewMCPClient(endpoint, token)`, `api.MCPClient.CallTool(ctx, name, args) (*sdkmcp.CallToolResult, error)`, `api.AuthToken(ctx)` (all existing, `test/functional/support/...`); `db.SeedLookupCategory(ctx, category, values)`, `db.DeleteLookupCategory(ctx, category)` (existing, `test/functional/support/db`, already used by `test/functional/features/api/lookups/lookups_get_test.go`).
- Produces: nothing consumed by later tasks — this is the terminal verification task.

Tool call results decode via `result.StructuredContent` (populated automatically by `mcp.AddTool` from the handler's typed `Out` return) — re-marshal/unmarshal it into the expected Go struct, the same way REST functional tests decode `resp.Body` into a struct.

- [ ] **Step 1: Write the suite runner**

```go
// test/functional/features/mcp/lookups/lookups_suite_test.go
package lookups_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLookups(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP Lookups Suite")
}
```

- [ ] **Step 2: Write `list_lookups` functional spec**

```go
// test/functional/features/mcp/lookups/lookups_list_test.go
package lookups_test

import (
	"encoding/json"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Listing lookup categories", func() {
	var (
		client   *api.MCPClient
		result   *sdkmcp.CallToolResult
		err      error
		category string
	)

	BeforeEach(func() {
		result = nil
		err = nil
		category = "functional-test-category-" + uuid.NewString()
	})

	AfterEach(func(ctx SpecContext) {
		Expect(db.DeleteLookupCategory(ctx, category)).To(Succeed())
	})

	Context("given a valid bearer token and the lookup table has a category", func() {
		BeforeEach(func(ctx SpecContext) {
			token, err := api.AuthToken(ctx)
			Expect(err).NotTo(HaveOccurred())
			client = api.NewMCPClient(support.BaseURL()+"/mcp", token)

			Expect(db.SeedLookupCategory(ctx, category, []any{"a", "b"})).To(Succeed())
		})

		When("the list_lookups tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_lookups", map[string]any{})
			})

			It("succeeds and includes the seeded category", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())

				raw, err := json.Marshal(result.StructuredContent)
				Expect(err).NotTo(HaveOccurred())

				var out struct {
					Categories []string `json:"categories"`
				}
				Expect(json.Unmarshal(raw, &out)).To(Succeed())
				Expect(out.Categories).To(ContainElement(category))
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the list_lookups tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_lookups", map[string]any{})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
```

- [ ] **Step 3: Write `get_lookup` functional spec**

```go
// test/functional/features/mcp/lookups/lookups_get_test.go
package lookups_test

import (
	"encoding/json"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Getting a lookup category", func() {
	var (
		client   *api.MCPClient
		result   *sdkmcp.CallToolResult
		err      error
		category string
	)

	BeforeEach(func(ctx SpecContext) {
		result = nil
		err = nil
		category = "functional-test-category-" + uuid.NewString()

		token, err := api.AuthToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		client = api.NewMCPClient(support.BaseURL()+"/mcp", token)
	})

	AfterEach(func(ctx SpecContext) {
		Expect(db.DeleteLookupCategory(ctx, category)).To(Succeed())
	})

	Context("given the category exists", func() {
		BeforeEach(func(ctx SpecContext) {
			Expect(db.SeedLookupCategory(ctx, category, []any{"a", "b"})).To(Succeed())
		})

		When("the get_lookup tool is called with that category", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "get_lookup", map[string]any{"category": category})
			})

			It("succeeds and returns the category's values", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())

				raw, err := json.Marshal(result.StructuredContent)
				Expect(err).NotTo(HaveOccurred())

				var out struct {
					Category string   `json:"category"`
					Values   []string `json:"values"`
				}
				Expect(json.Unmarshal(raw, &out)).To(Succeed())
				Expect(out.Category).To(Equal(category))
				Expect(out.Values).To(Equal([]string{"a", "b"}))
			})
		})
	})

	Context("given the category does not exist", func() {
		When("the get_lookup tool is called with that category", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "get_lookup", map[string]any{"category": "functional-test-category-missing-" + uuid.NewString()})
			})

			It("returns an MCP tool error result, not a transport failure", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeTrue())
			})
		})
	})
})
```

- [ ] **Step 4: Bring up the local functional stack**

Run: `mise run func-setup`
Expected: LocalStack + mockoidc + `sam local start-api` come up cleanly (same as any other functional-test session).

- [ ] **Step 5: Run the new suite**

Run: `mise run func-test -- --focus="Lookups"` (or run the full suite: `mise run func-test`)
Expected: all specs in `test/functional/features/mcp/lookups/` PASS, and the pre-existing `test/functional/features/mcp/ping` and `test/functional/features/api/lookups` suites remain PASS (no regression).

- [ ] **Step 6: Tear down**

Run: `mise run func-teardown`

- [ ] **Step 7: Commit**

```bash
git add test/functional/features/mcp/lookups/
git commit -m "test(mcp): add functional coverage for list_lookups/get_lookup"
```

---

### Task 6: Final verification sweep

**Files:** none (verification only).

- [ ] **Step 1: Full lint/unit/generated-check pass**

Run: `mise run lint && mise run test && mise run check-generated`
Expected: all clean. (`check-generated` should report no drift — this plan makes no changes to any mockery-covered interface, so no mock regeneration is expected. If it does report drift, investigate before proceeding — don't blindly regenerate.)

- [ ] **Step 2: SAM template validation**

Run: `sam validate --lint`
Expected: passes (no `template.yaml` changes in this plan).

- [ ] **Step 3: Full functional suite, one more time, end to end**

Run: `mise run func-setup && mise run func-test && mise run func-teardown`
Expected: all suites pass, including the new MCP lookups specs and the untouched `ping`/REST lookups suites.

- [ ] **Step 4: Review diff for scope creep**

Run: `git diff main --stat` (or `git log --stat` over this branch's commits)
Expected: touches only `internal/mcp/schema/`, `internal/repomcp/`, `internal/mcp/`, `internal/router/router.go`, `test/functional/features/mcp/lookups/`. No changes to `internal/handlers`, `internal/repoapi`, `api/openapi.yaml`, or `template.yaml`.
