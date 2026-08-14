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
