package buildrefs

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// FieldError reports that Field's Value doesn't reference a real resource
// owned by the caller. Shaped like
// [github.com/rogueserenity/kbdb/internal/lookup.FieldError] (Field/Value,
// no Category) so REST/MCP callers can render it the same way they
// already render lookup validation failures.
type FieldError struct {
	Field  string
	Value  string
	Reason string
}

// ValidateReferences doesn't report an unset/empty Keyboard or empty
// Switches/KeycapKits entry - required/minLength constraints upstream
// already guard those.
//
// "Doesn't exist" and "exists but owned by someone else" are reported
// identically (every Get's ErrNotFound is already scoped to ownerID's
// collection), so this can't be used to probe whether an id exists under
// another account.
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

	keycapSets := make(map[string]*repository.KeycapSet)
	notFoundKeycapSets := make(map[string]bool)

	for i, entry := range b.KeycapKits {
		if entry.KeycapSet == "" {
			continue
		}

		ks, ok := keycapSets[entry.KeycapSet]
		if !ok {
			if notFoundKeycapSets[entry.KeycapSet] {
				fieldErrs = append(fieldErrs, FieldError{
					Field: fmt.Sprintf("keycap_kits[%d].keycap_set", i), Value: entry.KeycapSet,
					Reason: "does not reference a keycap set in your collection",
				})
				continue
			}

			var err error
			ks, err = keycapSetRepo.Get(ctx, ownerID, entry.KeycapSet)
			if errors.Is(err, repository.ErrNotFound) {
				notFoundKeycapSets[entry.KeycapSet] = true
				fieldErrs = append(fieldErrs, FieldError{
					Field: fmt.Sprintf("keycap_kits[%d].keycap_set", i), Value: entry.KeycapSet,
					Reason: "does not reference a keycap set in your collection",
				})
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("checking keycap set %q: %w", entry.KeycapSet, err)
			}
			keycapSets[entry.KeycapSet] = ks
		}

		if entry.Kit == "" {
			continue
		}

		if !slices.ContainsFunc(ks.Kits, func(k repository.KeycapKit) bool { return k.KitID == entry.Kit }) {
			fieldErrs = append(fieldErrs, FieldError{
				Field: fmt.Sprintf("keycap_kits[%d].kit", i), Value: entry.Kit,
				Reason: fmt.Sprintf("does not reference a kit in keycap set %q", entry.KeycapSet),
			})
		}
	}

	return fieldErrs, nil
}
