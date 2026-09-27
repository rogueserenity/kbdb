// Package ownerprefs carries the item owner's
// [github.com/rogueserenity/kbdb/internal/repository.ProfilePreferences] on
// a request's context, independent of any transport (REST, MCP). Each
// transport installs a loader once it knows whose collection a request
// targets - REST from the {userId} path segment (see
// [github.com/rogueserenity/kbdb/internal/middleware.OwnerPreferences]),
// MCP from the tool's user_id argument - and handlers read the result with
// [Get]. The lookup runs on the first [Get] only, so a request that never
// needs preferences never reads the profile.
package ownerprefs
