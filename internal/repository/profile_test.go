package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type NewProfileImageKeySuite struct {
	suite.Suite
}

func TestNewProfileImageKeySuite(t *testing.T) {
	suite.Run(t, new(NewProfileImageKeySuite))
}

func (s *NewProfileImageKeySuite) TestSucceeds() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "user-alice")

	key, err := repository.NewProfileImageKey(ctx)

	s.Require().NoError(err)
	s.Equal(repository.ProfileImageKey("profiles/user-alice/avatar"), key)
}

func (s *NewProfileImageKeySuite) TestNoUserIDInContext_ReturnsError() {
	key, err := repository.NewProfileImageKey(context.Background())

	s.Require().ErrorIs(err, repository.ErrNoUserID)
	s.Empty(key)
}
