package repomcp

import "github.com/rogueserenity/kbdb/internal/repository"

// ownerVisibility returns v for the item's owner and nil for anyone else.
func ownerVisibility(v repository.Visibility, isOwner bool) *string {
	if !isOwner {
		return nil
	}
	out := string(v)

	return &out
}
