package router

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/rogueserenity/kbdb/internal/auth"
	"github.com/rogueserenity/kbdb/internal/handlers"
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/mcp"
	"github.com/rogueserenity/kbdb/internal/middleware"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// New builds the application's http.Handler. verifier authenticates every
// request; additional entities/routes are added here in later issues, on
// this same handler.
//
// issuerURL configures the MCP endpoint's RFC 9728 Protected Resource
// Metadata (the OIDC issuer MCP clients should authenticate against); the
// metadata's "resource" field is derived per-request rather than passed in
// statically — see internal/mcp.Handlers doc comment for why. version is
// advertised to MCP clients in the server's initialize handshake.
func New(
	verifier *auth.Verifier,
	switchRepo repository.SwitchRepository,
	keyboardRepo repository.KeyboardRepository,
	keycapSetRepo repository.KeycapSetRepository,
	imageStore repository.KeycapKitImageStore,
	buildRepo repository.BuildRepository,
	buildImageStore repository.BuildImageStore,
	issuerURL, version string,
) http.Handler {
	validate := restOpenAPIValidator()

	mux := http.NewServeMux()

	// No middleware.Auth: security: [] in api/openapi.yaml. No PUT/DELETE:
	// lookup categories are static, deploy-time data (internal/lookup),
	// not writable at runtime.
	mux.Handle("GET /v1/lookups", validate(http.HandlerFunc(handlers.ListLookups)))
	mux.Handle("GET /v1/lookups/{category}", validate(http.HandlerFunc(handlers.GetLookup)))

	// security: [{}, CognitoAuth] in api/openapi.yaml - anonymous callers see
	// only public switches (see internal/authz.ReadableVisibilities).
	mux.Handle("GET /v1/users/{userId}/switches",
		middleware.OptionalAuth(verifier)(validate(handlers.ListSwitches(switchRepo))))
	mux.Handle("GET /v1/users/{userId}/switches/{switchId}",
		middleware.OptionalAuth(verifier)(validate(handlers.GetSwitch(switchRepo))))
	mux.Handle("POST /v1/users/{userId}/switches",
		middleware.Auth(verifier)(validate(handlers.CreateSwitch(switchRepo))))
	mux.Handle("PUT /v1/users/{userId}/switches/{switchId}",
		middleware.Auth(verifier)(validate(handlers.UpdateSwitch(switchRepo))))
	mux.Handle("DELETE /v1/users/{userId}/switches/{switchId}",
		middleware.Auth(verifier)(validate(handlers.DeleteSwitch(switchRepo))))

	// security: [{}, CognitoAuth] in api/openapi.yaml - anonymous callers see
	// only public keyboards (see internal/authz.ReadableVisibilities).
	mux.Handle("GET /v1/users/{userId}/keyboards",
		middleware.OptionalAuth(verifier)(validate(handlers.ListKeyboards(keyboardRepo))))
	mux.Handle("GET /v1/users/{userId}/keyboards/{keyboardId}",
		middleware.OptionalAuth(verifier)(validate(handlers.GetKeyboard(keyboardRepo))))
	mux.Handle("POST /v1/users/{userId}/keyboards",
		middleware.Auth(verifier)(validate(handlers.CreateKeyboard(keyboardRepo))))
	mux.Handle("PUT /v1/users/{userId}/keyboards/{keyboardId}",
		middleware.Auth(verifier)(validate(handlers.UpdateKeyboard(keyboardRepo))))
	mux.Handle("DELETE /v1/users/{userId}/keyboards/{keyboardId}",
		middleware.Auth(verifier)(validate(handlers.DeleteKeyboard(keyboardRepo))))

	// security: [{}, CognitoAuth] in api/openapi.yaml - anonymous callers see
	// only public keycap sets (see internal/authz.ReadableVisibilities).
	mux.Handle("GET /v1/users/{userId}/keycap-sets",
		middleware.OptionalAuth(verifier)(validate(handlers.ListKeycapSets(keycapSetRepo))))
	mux.Handle("GET /v1/users/{userId}/keycap-sets/{keycapSetId}",
		middleware.OptionalAuth(verifier)(validate(handlers.GetKeycapSet(keycapSetRepo, imageStore))))
	mux.Handle("POST /v1/users/{userId}/keycap-sets",
		middleware.Auth(verifier)(validate(handlers.CreateKeycapSet(keycapSetRepo, imageStore))))
	mux.Handle("PUT /v1/users/{userId}/keycap-sets/{keycapSetId}",
		middleware.Auth(verifier)(validate(handlers.UpdateKeycapSet(keycapSetRepo, imageStore))))
	mux.Handle("DELETE /v1/users/{userId}/keycap-sets/{keycapSetId}",
		middleware.Auth(verifier)(validate(handlers.DeleteKeycapSet(keycapSetRepo, imageStore))))
	mux.Handle("POST /v1/users/{userId}/keycap-sets/{keycapSetId}/kits",
		middleware.Auth(verifier)(validate(handlers.CreateKeycapKit(keycapSetRepo, imageStore))))
	mux.Handle("PUT /v1/users/{userId}/keycap-sets/{keycapSetId}/kits/{kitId}",
		middleware.Auth(verifier)(validate(handlers.UpdateKeycapKit(keycapSetRepo, imageStore))))
	mux.Handle("DELETE /v1/users/{userId}/keycap-sets/{keycapSetId}/kits/{kitId}",
		middleware.Auth(verifier)(validate(handlers.DeleteKeycapKit(keycapSetRepo, imageStore))))
	mux.Handle("POST /v1/users/{userId}/keycap-sets/{keycapSetId}/kits/{kitId}/image",
		middleware.Auth(verifier)(validate(handlers.SetKeycapKitImage(keycapSetRepo, imageStore))))
	mux.Handle("DELETE /v1/users/{userId}/keycap-sets/{keycapSetId}/kits/{kitId}/image",
		middleware.Auth(verifier)(validate(handlers.DeleteKeycapKitImage(keycapSetRepo, imageStore))))

	// security: [{}, CognitoAuth] in api/openapi.yaml - anonymous callers see
	// only public builds (see internal/authz.ReadableVisibilities).
	mux.Handle("GET /v1/users/{userId}/builds",
		middleware.OptionalAuth(verifier)(validate(handlers.ListBuilds(buildRepo, keyboardRepo, buildImageStore))))
	// Default CognitoAuthorizer applies - same rationale as CreateKeyboardEvent.
	mux.Handle("POST /v1/users/{userId}/builds",
		middleware.Auth(verifier)(validate(handlers.CreateBuild(buildRepo, buildImageStore, keyboardRepo, switchRepo, keycapSetRepo))))
	// Auth: NONE at the gateway - same rationale as ListKeyboardsEvent.
	mux.Handle("GET /v1/users/{userId}/builds/{buildId}",
		middleware.OptionalAuth(verifier)(validate(handlers.GetBuild(buildRepo, buildImageStore))))
	mux.Handle("PUT /v1/users/{userId}/builds/{buildId}",
		middleware.Auth(verifier)(validate(handlers.UpdateBuild(buildRepo, buildImageStore, keyboardRepo, switchRepo, keycapSetRepo))))
	mux.Handle("DELETE /v1/users/{userId}/builds/{buildId}",
		middleware.Auth(verifier)(validate(handlers.DeleteBuild(buildRepo, buildImageStore))))

	// MCP: auth happens inside the MCP server itself, returning MCP-shaped
	// errors rather than a bare 401. Not wrapped in validate: api/openapi.yaml
	// only covers the REST surface.
	mcpHandlers := mcp.New(verifier, switchRepo, keyboardRepo, keycapSetRepo, imageStore, buildRepo, buildImageStore, issuerURL, version)
	mux.Handle("/mcp", mcpHandlers.Streamable)
	mux.Handle(mcpHandlers.MetadataPath, mcpHandlers.Metadata)
	mux.Handle(mcpHandlers.RootMetadataPath, mcpHandlers.Metadata)

	return middleware.Logging(middleware.Recover(mux))
}

// restOpenAPIValidator validates incoming REST requests' shape against
// api/openapi.yaml. Auth stays enforced by internal/middleware, not here
// (see NoopAuthenticationFunc); open-vocabulary lookup values stay
// hand-validated per handler (e.g. handlers.validateSwitchLookups), since
// that isn't expressible as a static schema.
func restOpenAPIValidator() func(http.Handler) http.Handler {
	spec, err := api.GetSpec()
	if err != nil {
		log.Fatalf("router: loading embedded OpenAPI spec: %v", err)
	}

	return nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
		// Deployed Host varies per stack; spec's servers URL is fixed.
		DoNotValidateServers: true,
		Options: openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			// Report every schema violation in a request, not just the
			// first, so validationErrorHandler can populate a complete
			// invalid_params list in one response.
			MultiError: true,
		},
		ErrorHandlerWithOpts: validationErrorHandler,
	})
}

// validationErrorHandler translates nethttp-middleware's OpenAPI validation
// failures into the same RFC 9457 application/problem+json shape used
// everywhere else in the API (see internal/problem), rather than the
// middleware's default plain-text body. Requests to paths/methods absent
// from api/openapi.yaml entirely (opts.MatchedRoute == nil) are reported as
// a plain 404, since there is no schema to name invalid_params against.
func validationErrorHandler(_ context.Context, err error, w http.ResponseWriter, _ *http.Request, opts nethttpmiddleware.ErrorHandlerOpts) {
	if opts.MatchedRoute == nil {
		problem.NotFound(w, "resource not found")
		return
	}

	invalidParams := collectInvalidParams(err)
	if len(invalidParams) == 0 {
		problem.BadRequest(w, err.Error())
		return
	}

	problem.ValidationFailed(w, "request failed schema validation", invalidParams)
}

// collectInvalidParams walks err - an openapi3.MultiError of
// *openapi3filter.RequestError, each possibly itself wrapping an
// openapi3.MultiError of *openapi3.SchemaError (one per violated field) -
// into one problem.InvalidParam per violation. Returns nil if err doesn't
// match this shape (e.g. a malformed-JSON body, which has no field path).
func collectInvalidParams(err error) []problem.InvalidParam {
	var params []problem.InvalidParam

	multi, ok := errors.AsType[openapi3.MultiError](err)
	if !ok {
		multi = openapi3.MultiError{err}
	}

	for _, e := range multi {
		reqErr, ok := errors.AsType[*openapi3filter.RequestError](e)
		if !ok || reqErr.Err == nil {
			continue
		}

		// A parameter-level failure (query/path/header/cookie) names itself
		// via Parameter.Name - reqErr.Err here is often a plain parse/
		// required error, not a *openapi3.SchemaError with a JSONPointer,
		// so schemaErrorsFrom below (body-only) can't name it and would
		// otherwise mislabel it "body".
		if reqErr.Parameter != nil {
			params = append(params, problem.InvalidParam{Name: reqErr.Parameter.Name, Reason: reqErr.Err.Error()})
			continue
		}

		params = append(params, schemaErrorsFrom(reqErr.Err)...)
	}

	return params
}

// schemaErrorsFrom flattens err - a *openapi3.SchemaError or an
// openapi3.MultiError of them - into invalid_params entries, naming each by
// its JSON pointer path (e.g. "material.stem").
func schemaErrorsFrom(err error) []problem.InvalidParam {
	if schemaErr, ok := errors.AsType[*openapi3.SchemaError](err); ok {
		name := strings.Join(schemaErr.JSONPointer(), ".")
		if name == "" {
			name = "body"
		}

		return []problem.InvalidParam{{Name: name, Reason: schemaErr.Reason}}
	}

	if multi, ok := errors.AsType[openapi3.MultiError](err); ok {
		var params []problem.InvalidParam
		for _, e := range multi {
			params = append(params, schemaErrorsFrom(e)...)
		}

		return params
	}

	return nil
}
