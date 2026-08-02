package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type NewKeycapKitImageKeySuite struct {
	suite.Suite
}

func TestNewKeycapKitImageKeySuite(t *testing.T) {
	suite.Run(t, new(NewKeycapKitImageKeySuite))
}

func (s *NewKeycapKitImageKeySuite) TestSucceeds() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")

	key, err := repository.NewKeycapKitImageKey(ctx, "ks1", "kit1")

	s.Require().NoError(err)
	s.Equal(repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image"), key)
}

func (s *NewKeycapKitImageKeySuite) TestNoUserIDInContext_ReturnsError() {
	key, err := repository.NewKeycapKitImageKey(context.Background(), "ks1", "kit1")

	s.Require().ErrorIs(err, repository.ErrNoUserID)
	s.Empty(key)
}
