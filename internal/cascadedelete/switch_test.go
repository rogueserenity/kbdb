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

type DeleteSwitchSuite struct {
	suite.Suite

	mockSwitches *mocks.MockSwitchRepository
	mockBuilds   *mocks.MockBuildRepository
	mockImages   *mocks.MockBuildImageStore
	ctx          context.Context
}

func TestDeleteSwitchSuite(t *testing.T) {
	suite.Run(t, new(DeleteSwitchSuite))
}

func (s *DeleteSwitchSuite) SetupTest() {
	s.mockSwitches = mocks.NewMockSwitchRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
	s.ctx = s.T().Context()
}

func (s *DeleteSwitchSuite) TestBlock_NoReferencingBuilds_DeletesSwitch() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(s.ctx, "alice", "sw1").
		Return(nil, nil)
	s.mockSwitches.EXPECT().
		Delete(s.ctx, "sw1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteSwitch(
		s.ctx, s.mockSwitches, s.mockBuilds, s.mockImages,
		"alice", "sw1", cascadedelete.OnDeleteBlock,
	)

	s.Require().NoError(err)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteSwitchSuite) TestBlock_SwitchHadImage_ReturnsImageKey() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(s.ctx, "alice", "sw1").
		Return(nil, nil)
	key := repository.SwitchImageKey("switches/alice/sw1/image")
	s.mockSwitches.EXPECT().
		Delete(s.ctx, "sw1").
		Return(&key, nil)

	result, err := cascadedelete.DeleteSwitch(
		s.ctx, s.mockSwitches, s.mockBuilds, s.mockImages,
		"alice", "sw1", cascadedelete.OnDeleteBlock,
	)

	s.Require().NoError(err)
	s.Require().NotNil(result.ImageKey)
	s.Equal(key, *result.ImageKey)
}

func (s *DeleteSwitchSuite) TestBlock_ReferencingBuilds_ReturnsBlockedError_DeletesNothing() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(s.ctx, "alice", "sw1").
		Return([]string{"build-1", "build-2"}, nil)

	_, err := cascadedelete.DeleteSwitch(
		s.ctx, s.mockSwitches, s.mockBuilds, s.mockImages,
		"alice", "sw1", cascadedelete.OnDeleteBlock,
	)

	s.Require().ErrorIs(err, cascadedelete.ErrBlocked)

	var blocked *cascadedelete.BlockedError
	s.Require().ErrorAs(err, &blocked)
	s.ElementsMatch([]string{"build-1", "build-2"}, blocked.BuildIDs)

	// mockSwitches has no .EXPECT() calls set up - mockery's mock.Mock
	// fails the test if Delete is called unexpectedly, verifying nothing
	// was deleted.
}

func (s *DeleteSwitchSuite) TestDetach_ReferencingBuilds_DeletesSwitchAnyway() {
	// detach must NOT call FindBuildsReferencingSwitch at all - it doesn't
	// care about references.
	s.mockSwitches.EXPECT().
		Delete(s.ctx, "sw1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteSwitch(
		s.ctx, s.mockSwitches, s.mockBuilds, s.mockImages,
		"alice", "sw1", cascadedelete.OnDeleteDetach,
	)

	s.Require().NoError(err)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteSwitchSuite) TestCascade_NoReferencingBuilds_DeletesOnlySwitch() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(s.ctx, "alice", "sw1").
		Return(nil, nil)
	s.mockSwitches.EXPECT().
		Delete(s.ctx, "sw1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteSwitch(
		s.ctx, s.mockSwitches, s.mockBuilds, s.mockImages,
		"alice", "sw1", cascadedelete.OnDeleteCascade,
	)

	s.Require().NoError(err)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteSwitchSuite) TestCascade_ReferencingBuilds_DeletesEachBuildThenSwitch() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(s.ctx, "alice", "sw1").
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
	s.mockSwitches.EXPECT().
		Delete(s.ctx, "sw1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteSwitch(
		s.ctx, s.mockSwitches, s.mockBuilds, s.mockImages,
		"alice", "sw1", cascadedelete.OnDeleteCascade,
	)

	s.Require().NoError(err)
	s.ElementsMatch([]string{"build-1", "build-2"}, result.DeletedBuildIDs)
}

func (s *DeleteSwitchSuite) TestCascade_BuildDeleteFails_ReturnsErrorWithoutDeletingSwitch() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(s.ctx, "alice", "sw1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(s.ctx, "build-1").
		Return(nil, errors.New("dynamo unavailable"))

	_, err := cascadedelete.DeleteSwitch(
		s.ctx, s.mockSwitches, s.mockBuilds, s.mockImages,
		"alice", "sw1", cascadedelete.OnDeleteCascade,
	)

	s.Require().Error(err)
	// mockSwitches has no .EXPECT() - verifies Delete was never reached.
}

func (s *DeleteSwitchSuite) TestUnknownOnDelete_ReturnsError_CallsNothing() {
	_, err := cascadedelete.DeleteSwitch(
		s.ctx, s.mockSwitches, s.mockBuilds, s.mockImages,
		"alice", "sw1", cascadedelete.OnDelete("bogus"),
	)

	s.Require().Error(err)
	// mockSwitches and mockBuilds have no .EXPECT() calls set up - verifies
	// the unknown value is rejected before any repository call.
}

func (s *DeleteSwitchSuite) TestFindReferencingBuildsFails_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(s.ctx, "alice", "sw1").
		Return(nil, errors.New("query failed"))

	_, err := cascadedelete.DeleteSwitch(
		s.ctx, s.mockSwitches, s.mockBuilds, s.mockImages,
		"alice", "sw1", cascadedelete.OnDeleteBlock,
	)

	s.Require().Error(err)
	s.Require().NotErrorIs(err, cascadedelete.ErrBlocked)
}
