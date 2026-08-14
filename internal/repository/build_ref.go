package repository

// OnDelete controls how a Delete/DeleteKit call on an entity referenced by
// one or more Builds behaves.
type OnDelete string

const (
	// OnDeleteBlock is the default: Delete fails with ErrStillReferenced if
	// the item is still referenced by any Build.
	OnDeleteBlock OnDelete = "block"
	// OnDeleteCascade deletes the item and deletes every Build that
	// references it, in full.
	OnDeleteCascade OnDelete = "cascade"
	// OnDeleteDetach deletes the item regardless of references, leaving any
	// referencing Build with a dangling reference.
	OnDeleteDetach OnDelete = "detach"
)

// CascadeResult reports what a cascade delete affected. A Delete/DeleteKit
// call returns a nil *CascadeResult when onDelete is not OnDeleteCascade, or
// when cascade mode found nothing to delete.
type CascadeResult struct {
	DeletedBuildIDs []string
}

// RefType identifies which of a Build's reference fields a refMarker (see
// internal/repository/dynamo/build_ref.go) points at. Used internally to
// build marker items and reverse-lookup queries - never appears in
// BuildRepository's exported FindBuildsReferencingX method signatures,
// which are typed per reference kind instead.
type RefType string

const (
	// RefTypeKeyboard marks a marker for a Build's Keyboard field.
	RefTypeKeyboard RefType = "keyboard"
	// RefTypeSwitch marks a marker for one of a Build's Switches[] entries.
	RefTypeSwitch RefType = "switch"
	// RefTypeKeycapKit marks a marker for one of a Build's KeycapKits[]
	// entries.
	RefTypeKeycapKit RefType = "keycap_kit"
)
