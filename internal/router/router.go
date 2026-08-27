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
	"github.com/rogueserenity/kbdb/internal/consent"
	"github.com/rogueserenity/kbdb/internal/handlers"
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/mcp"
	"github.com/rogueserenity/kbdb/internal/middleware"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// New builds the application's http.Handler. verifier authenticates every
// REST request and /mcp. Additional entities/routes are added here in
// later issues, on this same handler.
//
// issuerURL configures the MCP endpoint's RFC 9728 Protected Resource
// Metadata (the OIDC issuer MCP clients should authenticate against); the
// metadata's "resource" field is derived per-request rather than passed in
// statically — see [github.com/rogueserenity/kbdb/internal/mcp.Handlers]
// for why. stytchPublicToken configures the GET /authorize consent page and
// GET /logout (see internal/consent); logoutReturnOrigins restricts
// /logout's return_to param. version is advertised to MCP clients in the
// server's initialize handshake.
func New(
	verifier *auth.Verifier,
	switchRepo repository.SwitchRepository,
	switchImageStore repository.SwitchImageStore,
	keyboardRepo repository.KeyboardRepository,
	keyboardImageStore repository.KeyboardImageStore,
	keycapSetRepo repository.KeycapSetRepository,
	imageStore repository.KeycapKitImageStore,
	buildRepo repository.BuildRepository,
	buildImageStore repository.BuildImageStore,
	profileRepo repository.ProfileRepository,
	profileImageStore repository.ProfileImageStore,
	issuerURL, stytchPublicToken, version string,
	logoutReturnOrigins []string,
) http.Handler {
	validate := restOpenAPIValidator()

	mux := http.NewServeMux()

	// Not part of api/openapi.yaml, so not wrapped in validate - see
	// internal/consent.
	mux.Handle("GET /authorize", consent.Handler(stytchPublicToken, issuerURL))
	mux.Handle("GET /logout", consent.LogoutHandler(stytchPublicToken, issuerURL, logoutReturnOrigins))

	// security: [] in api/openapi.yaml - always anonymous. No PUT/DELETE:
	// lookup categories are static, deploy-time data (internal/lookup),
	// not writable at runtime.
	mux.Handle("GET /v1/lookups", validate(http.HandlerFunc(handlers.ListLookups)))
	mux.Handle("GET /v1/lookups/{category}", validate(http.HandlerFunc(handlers.GetLookup)))

	// security: [{}, CognitoAuth] in api/openapi.yaml - anonymous callers see
	// only public switches (see [github.com/rogueserenity/kbdb/internal/authz.ReadableVisibilities]).
	mux.Handle("GET /v1/users/{userId}/switches",
		middleware.OptionalAuth(verifier)(validate(handlers.ListSwitches(switchRepo, switchImageStore))))
	mux.Handle("GET /v1/users/{userId}/switches/{switchId}",
		middleware.OptionalAuth(verifier)(validate(handlers.GetSwitch(switchRepo, switchImageStore))))
	mux.Handle("POST /v1/users/{userId}/switches",
		middleware.RequireAuthorizerIdentity(validate(handlers.CreateSwitch(switchRepo, switchImageStore))))
	mux.Handle("PUT /v1/users/{userId}/switches/{switchId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.UpdateSwitch(switchRepo, switchImageStore))))
	mux.Handle("DELETE /v1/users/{userId}/switches/{switchId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteSwitch(switchRepo, buildRepo, buildImageStore, switchImageStore))))
	mux.Handle("POST /v1/users/{userId}/switches/{switchId}/image",
		middleware.RequireAuthorizerIdentity(validate(handlers.SetSwitchImage(switchRepo, switchImageStore))))
	mux.Handle("DELETE /v1/users/{userId}/switches/{switchId}/image",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteSwitchImage(switchRepo, switchImageStore))))

	// security: [{}, CognitoAuth] in api/openapi.yaml - anonymous callers see
	// only public keyboards (see [github.com/rogueserenity/kbdb/internal/authz.ReadableVisibilities]).
	mux.Handle("GET /v1/users/{userId}/keyboards",
		middleware.OptionalAuth(verifier)(validate(handlers.ListKeyboards(keyboardRepo, keyboardImageStore))))
	mux.Handle("GET /v1/users/{userId}/keyboards/{keyboardId}",
		middleware.OptionalAuth(verifier)(validate(handlers.GetKeyboard(keyboardRepo, keyboardImageStore))))
	mux.Handle("POST /v1/users/{userId}/keyboards",
		middleware.RequireAuthorizerIdentity(validate(handlers.CreateKeyboard(keyboardRepo, keyboardImageStore))))
	mux.Handle("PUT /v1/users/{userId}/keyboards/{keyboardId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.UpdateKeyboard(keyboardRepo, keyboardImageStore))))
	mux.Handle("DELETE /v1/users/{userId}/keyboards/{keyboardId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteKeyboard(keyboardRepo, buildRepo, buildImageStore, keyboardImageStore))))
	mux.Handle("POST /v1/users/{userId}/keyboards/{keyboardId}/images",
		middleware.RequireAuthorizerIdentity(validate(handlers.AddKeyboardImage(keyboardRepo, keyboardImageStore))))
	mux.Handle("DELETE /v1/users/{userId}/keyboards/{keyboardId}/images/{imageId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteKeyboardImage(keyboardRepo, keyboardImageStore))))

	// security: [{}, CognitoAuth] in api/openapi.yaml - anonymous callers see
	// only public keycap sets (see [github.com/rogueserenity/kbdb/internal/authz.ReadableVisibilities]).
	mux.Handle("GET /v1/users/{userId}/keycap-sets",
		middleware.OptionalAuth(verifier)(validate(handlers.ListKeycapSets(keycapSetRepo, imageStore))))
	mux.Handle("GET /v1/users/{userId}/keycap-sets/{keycapSetId}",
		middleware.OptionalAuth(verifier)(validate(handlers.GetKeycapSet(keycapSetRepo, imageStore))))
	mux.Handle("POST /v1/users/{userId}/keycap-sets",
		middleware.RequireAuthorizerIdentity(validate(handlers.CreateKeycapSet(keycapSetRepo, imageStore))))
	mux.Handle("PUT /v1/users/{userId}/keycap-sets/{keycapSetId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.UpdateKeycapSet(keycapSetRepo, imageStore))))
	mux.Handle("DELETE /v1/users/{userId}/keycap-sets/{keycapSetId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteKeycapSet(keycapSetRepo, buildRepo, buildImageStore, imageStore))))
	mux.Handle("POST /v1/users/{userId}/keycap-sets/{keycapSetId}/kits",
		middleware.RequireAuthorizerIdentity(validate(handlers.CreateKeycapKit(keycapSetRepo, imageStore))))
	mux.Handle("PUT /v1/users/{userId}/keycap-sets/{keycapSetId}/kits/{kitId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.UpdateKeycapKit(keycapSetRepo, imageStore))))
	mux.Handle("DELETE /v1/users/{userId}/keycap-sets/{keycapSetId}/kits/{kitId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteKeycapKit(keycapSetRepo, buildRepo, buildImageStore, imageStore))))
	mux.Handle("POST /v1/users/{userId}/keycap-sets/{keycapSetId}/kits/{kitId}/image",
		middleware.RequireAuthorizerIdentity(validate(handlers.SetKeycapKitImage(keycapSetRepo, imageStore))))
	mux.Handle("DELETE /v1/users/{userId}/keycap-sets/{keycapSetId}/kits/{kitId}/image",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteKeycapKitImage(keycapSetRepo, imageStore))))

	// security: [{}, CognitoAuth] in api/openapi.yaml - anonymous callers see
	// only public builds (see [github.com/rogueserenity/kbdb/internal/authz.ReadableVisibilities]).
	mux.Handle("GET /v1/users/{userId}/builds",
		middleware.OptionalAuth(verifier)(validate(handlers.ListBuilds(buildRepo, keyboardRepo, switchRepo, keycapSetRepo, buildImageStore))))
	mux.Handle("POST /v1/users/{userId}/builds",
		middleware.RequireAuthorizerIdentity(validate(handlers.CreateBuild(buildRepo, buildImageStore, imageStore, keyboardImageStore, switchImageStore, keyboardRepo, switchRepo, keycapSetRepo))))
	mux.Handle("GET /v1/users/{userId}/builds/{buildId}",
		middleware.OptionalAuth(verifier)(validate(handlers.GetBuild(buildRepo, buildImageStore, imageStore, keyboardImageStore, switchImageStore, keyboardRepo, switchRepo, keycapSetRepo))))
	mux.Handle("PUT /v1/users/{userId}/builds/{buildId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.UpdateBuild(buildRepo, buildImageStore, imageStore, keyboardImageStore, switchImageStore, keyboardRepo, switchRepo, keycapSetRepo))))
	mux.Handle("DELETE /v1/users/{userId}/builds/{buildId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteBuild(buildRepo, buildImageStore))))
	mux.Handle("POST /v1/users/{userId}/builds/{buildId}/images",
		middleware.RequireAuthorizerIdentity(validate(handlers.AddBuildImage(buildRepo, buildImageStore))))
	mux.Handle("DELETE /v1/users/{userId}/builds/{buildId}/images/{imageId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteBuildImage(buildRepo, buildImageStore))))

	// security: [{}, CognitoAuth] in api/openapi.yaml - anonymous callers may
	// read discoverable profiles; the handler applies the
	// discoverable-or-owner rule itself (see internal/profileread).
	mux.Handle("GET /v1/profile/{identifier}",
		middleware.OptionalAuth(verifier)(validate(handlers.GetProfile(profileRepo, profileImageStore))))

	// MCP: /mcp verifies auth in-process (middleware.RequireAuth), not via
	// API Gateway's native authorizer - see that middleware's doc comment
	// for why (a spec-compliant 401 needs a WWW-Authenticate header naming
	// the RFC 9728 metadata URL, which the gateway's own fixed rejection
	// response can't carry). template.yaml's McpEvent is Authorizer: NONE
	// accordingly. Not wrapped in validate: api/openapi.yaml only covers
	// the REST surface.
	mcpHandlers := mcp.New(switchRepo, switchImageStore, keyboardRepo, keyboardImageStore, keycapSetRepo, imageStore, buildRepo, buildImageStore, profileRepo, verifier, issuerURL, version)
	mux.Handle("/mcp", mcpHandlers.Streamable)
	mux.Handle(mcpHandlers.MetadataPath, mcpHandlers.Metadata)
	mux.Handle(mcpHandlers.RootMetadataPath, mcpHandlers.Metadata)

	return middleware.Logging(middleware.Recover(mux))
}

// restOpenAPIValidator validates incoming REST requests' shape against
// api/openapi.yaml. Auth stays enforced by
// [github.com/rogueserenity/kbdb/internal/middleware], not here (see
// [openapi3filter.NoopAuthenticationFunc]); open-vocabulary lookup values
// stay hand-validated per handler (e.g.
// [github.com/rogueserenity/kbdb/internal/handlers.validateSwitchLookups]),
// since that isn't expressible as a static schema.
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
// everywhere else in the API (see
// [github.com/rogueserenity/kbdb/internal/problem]), rather than the
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
