// Package buildrefs validates that a Build's cross-entity references
// (Keyboard, Switches, KeycapKits) name real resources owned by the caller,
// independent of any transport (REST, MCP) — mirroring internal/lookup's
// shape as a transport-agnostic validation package importable by both
// internal/handlers and internal/mcp, except this checks live repository
// data instead of the static lookup category system, so it can't live in
// internal/lookup itself (see that package's ValidateBuild doc comment).
package buildrefs
