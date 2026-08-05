package mcp

import (
	"context"
	"errors"
	"strings"

	ctxpkg "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/log"
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

func clampListLimit(limit int) int {
	if limit < 1 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}

	return limit
}
