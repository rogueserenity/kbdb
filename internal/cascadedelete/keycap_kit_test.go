package cascadedelete_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/cascadedelete"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type DeleteKeycapKitSuite struct {
	suite.Suite

	mockKeycapSets *mocks.MockKeycapSetRepository
	mockBuilds     *mocks.MockBuildRepository
	mockImages     *mocks.MockBuildImageStore
	ctx            context.Context
}

func TestDeleteKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(DeleteKeycapKitSuite))
}

func (s *DeleteKeycapKitSuite) SetupTest() {
	s.mockKeycapSets = mocks.NewMockKeycapSetRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
	s.ctx = s.T().Context()
}

func (s *DeleteKeycapKitSuite) TestBlock_NoReferencingBuilds_DeletesKit() {
	imgKey := repository.KeycapKitImageKey("img-1")
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(s.ctx, "alice", "ks1", "kit1").
		Return(nil, nil)
	s.mockKeycapSets.EXPECT().
		DeleteKit(s.ctx, "ks1", "kit1").
		Return(&imgKey, nil)

	result, err := cascadedelete.DeleteKeycapKit(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", "kit1", cascadedelete.OnDeleteBlock,
	)

	s.Require().NoError(err)
	s.Require().NotNil(result.ImageKey)
	s.Equal(imgKey, *result.ImageKey)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteKeycapKitSuite) TestBlock_ReferencingBuilds_ReturnsBlockedError_DeletesNothing() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(s.ctx, "alice", "ks1", "kit1").
		Return([]string{"build-1", "build-2"}, nil)

	_, err := cascadedelete.DeleteKeycapKit(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", "kit1", cascadedelete.OnDeleteBlock,
	)

	s.Require().ErrorIs(err, cascadedelete.ErrBlocked)

	var blocked *cascadedelete.BlockedError
	s.Require().ErrorAs(err, &blocked)
	s.ElementsMatch([]string{"build-1", "build-2"}, blocked.BuildIDs)

	// mockKeycapSets has no .EXPECT() calls set up - mockery's mock.Mock
	// fails the test if DeleteKit is called unexpectedly, verifying nothing
	// was deleted.
}

func (s *DeleteKeycapKitSuite) TestDetach_ReferencingBuilds_DeletesKitAnyway() {
	// detach must NOT call FindBuildsReferencingKeycapKit at all - it
	// doesn't care about references.
	s.mockKeycapSets.EXPECT().
		DeleteKit(s.ctx, "ks1", "kit1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteKeycapKit(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", "kit1", cascadedelete.OnDeleteDetach,
	)

	s.Require().NoError(err)
	s.Nil(result.ImageKey)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteKeycapKitSuite) TestCascade_NoReferencingBuilds_DeletesOnlyKit() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(s.ctx, "alice", "ks1", "kit1").
		Return(nil, nil)
	s.mockKeycapSets.EXPECT().
		DeleteKit(s.ctx, "ks1", "kit1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteKeycapKit(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", "kit1", cascadedelete.OnDeleteCascade,
	)

	s.Require().NoError(err)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteKeycapKitSuite) TestCascade_ReferencingBuilds_DeletesEachBuildThenKit() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(s.ctx, "alice", "ks1", "kit1").
		Return([]string{"build-1", "build-2"}, nil)
	s.mockBuilds.EXPECT().
		Delete(s.ctx, "build-1").
		Return([]repository.BuildImageKey{"img-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(s.ctx, "build-2").
		Return(nil, nil)
	s.mockImages.EXPECT().
		BestEffortDelete(s.ctx, []repository.BuildImageKey{"img-1"}).
		Return()
	s.mockImages.EXPECT().
		BestEffortDelete(s.ctx, mock.MatchedBy(func(keys []repository.BuildImageKey) bool { return len(keys) == 0 })).
		Return()
	s.mockKeycapSets.EXPECT().
		DeleteKit(s.ctx, "ks1", "kit1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteKeycapKit(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", "kit1", cascadedelete.OnDeleteCascade,
	)

	s.Require().NoError(err)
	s.ElementsMatch([]string{"build-1", "build-2"}, result.DeletedBuildIDs)
}

func (s *DeleteKeycapKitSuite) TestCascade_BuildDeleteFails_ReturnsErrorWithoutDeletingKit() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(s.ctx, "alice", "ks1", "kit1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(s.ctx, "build-1").
		Return(nil, errors.New("dynamo unavailable"))

	_, err := cascadedelete.DeleteKeycapKit(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", "kit1", cascadedelete.OnDeleteCascade,
	)

	s.Require().Error(err)
	// mockKeycapSets has no .EXPECT() - verifies DeleteKit was never
	// reached.
}

func (s *DeleteKeycapKitSuite) TestUnknownOnDelete_ReturnsError_CallsNothing() {
	_, err := cascadedelete.DeleteKeycapKit(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", "kit1", cascadedelete.OnDelete("bogus"),
	)

	s.Require().Error(err)
	// mockKeycapSets and mockBuilds have no .EXPECT() calls set up -
	// verifies the unknown value is rejected before any repository call.
}

func (s *DeleteKeycapKitSuite) TestFindReferencingBuildsFails_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(s.ctx, "alice", "ks1", "kit1").
		Return(nil, errors.New("query failed"))

	_, err := cascadedelete.DeleteKeycapKit(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", "kit1", cascadedelete.OnDeleteBlock,
	)

	s.Require().Error(err)
	s.Require().NotErrorIs(err, cascadedelete.ErrBlocked)
}

func (s *DeleteKeycapKitSuite) TestBlock_DeleteKitNotFound_PropagatesError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(s.ctx, "alice", "ks1", "kit1").
		Return(nil, nil)
	s.mockKeycapSets.EXPECT().
		DeleteKit(s.ctx, "ks1", "kit1").
		Return(nil, repository.ErrNotFound)

	_, err := cascadedelete.DeleteKeycapKit(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", "kit1", cascadedelete.OnDeleteBlock,
	)

	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *DeleteKeycapKitSuite) TestBlock_DeleteKitMutationConflict_PropagatesError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(s.ctx, "alice", "ks1", "kit1").
		Return(nil, nil)
	s.mockKeycapSets.EXPECT().
		DeleteKit(s.ctx, "ks1", "kit1").
		Return(nil, repository.ErrMutationConflict)

	_, err := cascadedelete.DeleteKeycapKit(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", "kit1", cascadedelete.OnDeleteBlock,
	)

	s.Require().ErrorIs(err, repository.ErrMutationConflict)
}
