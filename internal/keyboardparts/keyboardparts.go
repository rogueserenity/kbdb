package keyboardparts

import (
	"fmt"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// FieldError reports a part id the request can't use. Shaped like
// [github.com/rogueserenity/kbdb/internal/buildrefs.FieldError].
type FieldError struct {
	Field  string
	Value  string
	Reason string
}

// AssignIDs gives every plate and PCB in kb without an id a fresh one from
// newID. A sent id must be one of existing's parts of the same kind, and may
// appear only once; anything else is reported. existing is nil on create,
// where no id may be sent.
func AssignIDs(kb *repository.Keyboard, existing *repository.Keyboard, newID func() string) []FieldError {
	var fieldErrs []FieldError

	known := map[string]bool{}
	if existing != nil {
		for _, p := range existing.Plates {
			known[p.ID] = true
		}
	}
	seen := map[string]bool{}
	for i := range kb.Plates {
		fieldErrs = assign(fieldErrs, &kb.Plates[i].ID, fmt.Sprintf("plates[%d].id", i), "plate", known, seen, newID)
	}

	known = map[string]bool{}
	if existing != nil {
		for _, p := range existing.PCBs {
			known[p.ID] = true
		}
	}
	seen = map[string]bool{}
	for i := range kb.PCBs {
		fieldErrs = assign(fieldErrs, &kb.PCBs[i].ID, fmt.Sprintf("pcbs[%d].id", i), "PCB", known, seen, newID)
	}

	return fieldErrs
}

func assign(
	fieldErrs []FieldError, id *string, field, kind string, known, seen map[string]bool, newID func() string,
) []FieldError {
	switch {
	case *id == "":
		*id = newID()
	case !known[*id]:
		fieldErrs = append(fieldErrs, FieldError{
			Field: field, Value: *id,
			Reason: fmt.Sprintf("is not the id of one of this keyboard's %ss; omit it to add a new %s", kind, kind),
		})
	case seen[*id]:
		fieldErrs = append(fieldErrs, FieldError{
			Field: field, Value: *id,
			Reason: fmt.Sprintf("is used by more than one %s", kind),
		})
	}
	seen[*id] = true

	return fieldErrs
}
