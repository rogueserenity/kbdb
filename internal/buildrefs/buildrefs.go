package buildrefs

import (
	"context"
	"errors"
	"fmt"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// FieldError reports that Field's Value doesn't reference a real resource
// owned by the caller. Shaped like lookup.FieldError (Field/Value, no
// Category) so REST/MCP callers can render it the same way they already
// render lookup validation failures.
type FieldError struct {
	Field  string
	Value  string
	Reason string
}

// ValidateReferences checks that b's Keyboard, each Switches[].Switch, and
// each KeycapKits[].KeycapSet+Kit reference resources that exist and are
// owned by ownerID, returning one FieldError per invalid reference. An
// unset/empty Keyboard or an empty Switches/KeycapKits slice is not
// reported here - internal/lookup.ValidateBuild and api/openapi.yaml's
// required/minLength constraints already guard those, so this function
// assumes b has already passed that layer and only checks references that
// are actually present.
//
// "Doesn't exist" and "exists but owned by someone else" are reported
// identically (repository.ErrNotFound is scoped to ownerID's collection
// already, by every Get method's own contract), matching how the rest of
// this API treats unowned/nonexistent resources as indistinguishable 404s
// (see internal/authz) - a caller can't use this validation to probe
// whether an id exists under another account.
func ValidateReferences(
	ctx context.Context,
	ownerID string,
	b repository.Build,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	keycapSetRepo repository.KeycapSetRepository,
) ([]FieldError, error) {
	var fieldErrs []FieldError

	if b.Keyboard != "" {
		_, err := keyboardRepo.Get(ctx, ownerID, b.Keyboard)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			fieldErrs = append(fieldErrs, FieldError{
				Field: "keyboard", Value: b.Keyboard,
				Reason: "does not reference a keyboard in your collection",
			})
		case err != nil:
			return nil, fmt.Errorf("checking keyboard %q: %w", b.Keyboard, err)
		}
	}

	for i, entry := range b.Switches {
		if entry.Switch == "" {
			continue
		}

		_, err := switchRepo.Get(ctx, ownerID, entry.Switch)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			fieldErrs = append(fieldErrs, FieldError{
				Field: fmt.Sprintf("switches[%d].switch", i), Value: entry.Switch,
				Reason: "does not reference a switch in your collection",
			})
		case err != nil:
			return nil, fmt.Errorf("checking switch %q: %w", entry.Switch, err)
		}
	}

	for i, entry := range b.KeycapKits {
		if entry.KeycapSet == "" {
			continue
		}

		ks, err := keycapSetRepo.Get(ctx, ownerID, entry.KeycapSet)
		if errors.Is(err, repository.ErrNotFound) {
			fieldErrs = append(fieldErrs, FieldError{
				Field: fmt.Sprintf("keycap_kits[%d].keycap_set", i), Value: entry.KeycapSet,
				Reason: "does not reference a keycap set in your collection",
			})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("checking keycap set %q: %w", entry.KeycapSet, err)
		}

		if entry.Kit == "" {
			continue
		}

		if !hasKit(ks.Kits, entry.Kit) {
			fieldErrs = append(fieldErrs, FieldError{
				Field: fmt.Sprintf("keycap_kits[%d].kit", i), Value: entry.Kit,
				Reason: fmt.Sprintf("does not reference a kit in keycap set %q", entry.KeycapSet),
			})
		}
	}

	return fieldErrs, nil
}

func hasKit(kits []repository.KeycapKit, kitID string) bool {
	for _, k := range kits {
		if k.KitID == kitID {
			return true
		}
	}
	return false
}
