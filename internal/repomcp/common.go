package repomcp

import "github.com/rogueserenity/kbdb/internal/repository"

// currencyFor returns currency when price is set and nil otherwise, so a
// price and its currency always appear together.
func currencyFor(price *float64, currency string) *string {
	if price == nil {
		return nil
	}

	return &currency
}

// ownerVisibility returns v for the item's owner and nil for anyone else.
func ownerVisibility(v repository.Visibility, isOwner bool) *string {
	if !isOwner {
		return nil
	}
	out := string(v)

	return &out
}
