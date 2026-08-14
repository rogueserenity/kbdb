// Package buildrefs validates that a Build's cross-entity references
// (Keyboard, Switches, KeycapKits) name real resources owned by the
// caller. Separate from [github.com/rogueserenity/kbdb/internal/lookup]
// since it checks live repository data, not the static lookup category
// system.
package buildrefs
