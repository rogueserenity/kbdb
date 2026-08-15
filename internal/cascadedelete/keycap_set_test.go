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

type DeleteKeycapSetSuite struct {
	suite.Suite

	mockKeycapSets *mocks.MockKeycapSetRepository
	mockBuilds     *mocks.MockBuildRepository
	mockImages     *mocks.MockBuildImageStore
	ctx            context.Context
}

func TestDeleteKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(DeleteKeycapSetSuite))
}

func (s *DeleteKeycapSetSuite) SetupTest() {
	s.mockKeycapSets = mocks.NewMockKeycapSetRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
	s.ctx = s.T().Context()
}

func (s *DeleteKeycapSetSuite) TestBlock_NoReferencingBuilds_DeletesSet() {
	imgKeys := []repository.KeycapKitImageKey{"img-1", "img-2"}
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(s.ctx, "alice", "ks1").
		Return(nil, nil)
	s.mockKeycapSets.EXPECT().
		Delete(s.ctx, "ks1").
		Return(imgKeys, nil)

	result, err := cascadedelete.DeleteKeycapSet(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", cascadedelete.OnDeleteBlock,
	)

	s.Require().NoError(err)
	s.ElementsMatch(imgKeys, result.ImageKeys)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteKeycapSetSuite) TestBlock_ReferencingBuilds_ReturnsBlockedError_DeletesNothing() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(s.ctx, "alice", "ks1").
		Return([]string{"build-1", "build-2"}, nil)

	_, err := cascadedelete.DeleteKeycapSet(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", cascadedelete.OnDeleteBlock,
	)

	s.Require().ErrorIs(err, cascadedelete.ErrBlocked)

	var blocked *cascadedelete.BlockedError
	s.Require().ErrorAs(err, &blocked)
	s.ElementsMatch([]string{"build-1", "build-2"}, blocked.BuildIDs)

	// mockKeycapSets has no .EXPECT() calls set up - mockery's mock.Mock
	// fails the test if Delete is called unexpectedly, verifying nothing
	// was deleted.
}

func (s *DeleteKeycapSetSuite) TestDetach_ReferencingBuilds_DeletesSetAnyway() {
	// detach must NOT call FindBuildsReferencingKeycapSet at all - it
	// doesn't care about references.
	s.mockKeycapSets.EXPECT().
		Delete(s.ctx, "ks1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteKeycapSet(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", cascadedelete.OnDeleteDetach,
	)

	s.Require().NoError(err)
	s.Empty(result.ImageKeys)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteKeycapSetSuite) TestCascade_NoReferencingBuilds_DeletesOnlySet() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(s.ctx, "alice", "ks1").
		Return(nil, nil)
	s.mockKeycapSets.EXPECT().
		Delete(s.ctx, "ks1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteKeycapSet(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", cascadedelete.OnDeleteCascade,
	)

	s.Require().NoError(err)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteKeycapSetSuite) TestCascade_ReferencingBuilds_DeletesEachBuildThenSet() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(s.ctx, "alice", "ks1").
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
		Delete(s.ctx, "ks1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteKeycapSet(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", cascadedelete.OnDeleteCascade,
	)

	s.Require().NoError(err)
	s.ElementsMatch([]string{"build-1", "build-2"}, result.DeletedBuildIDs)
}

func (s *DeleteKeycapSetSuite) TestCascade_BuildDeleteFails_ReturnsErrorWithoutDeletingSet() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(s.ctx, "alice", "ks1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(s.ctx, "build-1").
		Return(nil, errors.New("dynamo unavailable"))

	_, err := cascadedelete.DeleteKeycapSet(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", cascadedelete.OnDeleteCascade,
	)

	s.Require().Error(err)
	// mockKeycapSets has no .EXPECT() - verifies Delete was never reached.
}

func (s *DeleteKeycapSetSuite) TestUnknownOnDelete_ReturnsError_CallsNothing() {
	_, err := cascadedelete.DeleteKeycapSet(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", cascadedelete.OnDelete("bogus"),
	)

	s.Require().Error(err)
	// mockKeycapSets and mockBuilds have no .EXPECT() calls set up -
	// verifies the unknown value is rejected before any repository call.
}

func (s *DeleteKeycapSetSuite) TestFindReferencingBuildsFails_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(s.ctx, "alice", "ks1").
		Return(nil, errors.New("query failed"))

	_, err := cascadedelete.DeleteKeycapSet(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", cascadedelete.OnDeleteBlock,
	)

	s.Require().Error(err)
	s.Require().NotErrorIs(err, cascadedelete.ErrBlocked)
}

func (s *DeleteKeycapSetSuite) TestBlock_DeleteSetNotFound_PropagatesError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(s.ctx, "alice", "ks1").
		Return(nil, nil)
	s.mockKeycapSets.EXPECT().
		Delete(s.ctx, "ks1").
		Return(nil, repository.ErrNotFound)

	_, err := cascadedelete.DeleteKeycapSet(
		s.ctx, s.mockKeycapSets, s.mockBuilds, s.mockImages,
		"alice", "ks1", cascadedelete.OnDeleteBlock,
	)

	s.Require().ErrorIs(err, repository.ErrNotFound)
}
