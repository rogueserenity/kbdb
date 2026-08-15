package mcp

import (
	"context"
	"errors"
	"strings"

	"github.com/rogueserenity/kbdb/internal/authz"
	ctxpkg "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/log"
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
// (e.g. [github.com/rogueserenity/kbdb/internal/handlers.GetKeyboard]).
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

func clampListLimit(limit int) int {
	if limit < 1 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}

	return limit
}
