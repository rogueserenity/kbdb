package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rogueserenity/kbdb/internal/authz"
	ctxpkg "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// REST gets these bounds from api/openapi.yaml's Limit param, applied by
// the request validator before a handler runs. MCP has no equivalent
// validation layer, so the tool handlers apply them themselves.
const (
	defaultListLimit = 20
	maxListLimit     = 100
)

// errNoCallerIdentity is unreachable while requireBearerToken gates every
// MCP request (see server.go), but fails closed rather than defaulting to
// an empty owner ID if that wiring ever changes.
var errNoCallerIdentity = errors.New("no caller identity on context")

func resolveOwnerID(ctx context.Context, userID string) (string, error) {
	if id := strings.TrimSpace(userID); id != "" {
		return id, nil
	}

	subject, ok := ctxpkg.UserID(ctx)
	if !ok || subject == "" {
		log.FromContext(ctx).Error("MCP tool ran with no caller identity on context")
		return "", errNoCallerIdentity
	}

	return subject, nil
}

// ownedReadable resolves ownerID (defaulting to the caller when userID is
// blank), fetches the entity with id via get, and enforces the 404-not-403
// convention shared by every get_* MCP tool: notFoundErr both when the
// entity genuinely doesn't exist and when the caller can't read it, so a
// visibility-denied entity isn't distinguishable from a nonexistent one.
// Mirrors the equivalent per-handler logic in each entity's REST handler
// (e.g. handlers.GetKeyboard).
func ownedReadable[T any](
	ctx context.Context,
	get func(ctx context.Context, ownerID, id string) (*T, error),
	visibilityOf func(T) repository.Visibility,
	entityName string,
	notFoundErr error,
	idLogField string,
	userID, id string,
) (*T, error) {
	ownerID, err := resolveOwnerID(ctx, userID)
	if err != nil {
		return nil, err
	}

	item, err := get(ctx, ownerID, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, notFoundErr
	}
	if err != nil {
		log.FromContext(ctx).Error("getting "+entityName, idLogField, id, log.Error, err)
		return nil, errors.New("failed to get " + entityName)
	}

	if !authz.CanReadVisibility(ctx, ownerID, visibilityOf(*item)) {
		log.DeniedRead(ctx, entityName, ownerID, string(visibilityOf(*item)), idLogField, id)
		return nil, notFoundErr
	}

	return item, nil
}

// validatedWrite is the shared body of every create_*/update_* MCP tool's
// input validation: blank-check brand/name, map the tool input to its
// repository shape, check Visibility, then run the entity's lookup-field
// validation and format any failures into a single error. Mirrors the
// equivalent per-entity logic that used to be duplicated across
// validatedKeyboard, validatedSwitch, and validatedKeycapSet.
//
// extra runs after the blank-checks but before mapping, for validation that
// needs the raw tool input rather than the mapped entity (e.g. keyboards'
// purchase-date check) - nil if an entity has none. reasonFor formats one
// lookup.FieldError into its tool-facing message; entities that don't need
// keyboards' size/layout special case can pass defaultFieldErrorReason.
func validatedWrite[TIn any, TOut any](
	ctx context.Context,
	lookupRepo repository.LookupRepository,
	in TIn,
	brand, name string,
	visibility string,
	extra func() error,
	fromMCP func(TIn) TOut,
	visibilityOf func(*TOut) *repository.Visibility,
	validate func(context.Context, repository.LookupRepository, TOut) ([]lookup.FieldError, error),
	reasonFor func(lookup.FieldError) string,
) (TOut, error) {
	var zero TOut

	if strings.TrimSpace(brand) == "" {
		return zero, errors.New("brand must not be blank")
	}
	if strings.TrimSpace(name) == "" {
		return zero, errors.New("name must not be blank")
	}

	if extra != nil {
		if err := extra(); err != nil {
			return zero, err
		}
	}

	out := fromMCP(in)

	v := visibilityOf(&out)
	if !v.Valid() {
		return zero, fmt.Errorf("visibility %q must be one of: public, authenticated, private", visibility)
	}

	fieldErrs, err := validate(ctx, lookupRepo, out)
	if err != nil {
		log.FromContext(ctx).Error("validating lookup fields", log.Error, err)
		return zero, errors.New("failed to validate lookup fields")
	}
	if len(fieldErrs) > 0 {
		reasons := make([]string, len(fieldErrs))
		for i, fe := range fieldErrs {
			reasons[i] = reasonFor(fe)
		}

		return zero, errors.New(strings.Join(reasons, "; "))
	}

	return out, nil
}

// defaultFieldErrorReason is reasonFor for entities with no per-field
// special case, i.e. everything but keyboards' layout/size rule.
func defaultFieldErrorReason(fe lookup.FieldError) string {
	return fmt.Sprintf("%s: %q is not an approved %s value", fe.Field, fe.Value, fe.Category)
}

func clampListLimit(limit int) int {
	if limit < 1 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}

	return limit
}
