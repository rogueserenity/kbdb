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

type DeleteKeyboardSuite struct {
	suite.Suite

	mockKeyboards *mocks.MockKeyboardRepository
	mockBuilds    *mocks.MockBuildRepository
	mockImages    *mocks.MockBuildImageStore
	ctx           context.Context
}

func TestDeleteKeyboardSuite(t *testing.T) {
	suite.Run(t, new(DeleteKeyboardSuite))
}

func (s *DeleteKeyboardSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
	s.ctx = s.T().Context()
}

func (s *DeleteKeyboardSuite) TestBlock_NoReferencingBuilds_DeletesKeyboard() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(s.ctx, "alice", "kb1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Delete(s.ctx, "kb1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteKeyboard(
		s.ctx, s.mockKeyboards, s.mockBuilds, s.mockImages,
		"alice", "kb1", cascadedelete.OnDeleteBlock,
	)

	s.Require().NoError(err)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteKeyboardSuite) TestBlock_KeyboardHadImages_ReturnsTheirImageKeys() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(s.ctx, "alice", "kb1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Delete(s.ctx, "kb1").
		Return([]repository.KeyboardImageKey{"keyboards/alice/kb1/images/img1"}, nil)

	result, err := cascadedelete.DeleteKeyboard(
		s.ctx, s.mockKeyboards, s.mockBuilds, s.mockImages,
		"alice", "kb1", cascadedelete.OnDeleteBlock,
	)

	s.Require().NoError(err)
	s.Equal([]repository.KeyboardImageKey{"keyboards/alice/kb1/images/img1"}, result.ImageKeys)
}

func (s *DeleteKeyboardSuite) TestBlock_ReferencingBuilds_ReturnsBlockedError_DeletesNothing() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(s.ctx, "alice", "kb1").
		Return([]string{"build-1", "build-2"}, nil)

	_, err := cascadedelete.DeleteKeyboard(
		s.ctx, s.mockKeyboards, s.mockBuilds, s.mockImages,
		"alice", "kb1", cascadedelete.OnDeleteBlock,
	)

	s.Require().ErrorIs(err, cascadedelete.ErrBlocked)

	var blocked *cascadedelete.BlockedError
	s.Require().ErrorAs(err, &blocked)
	s.ElementsMatch([]string{"build-1", "build-2"}, blocked.BuildIDs)

	// mockKeyboards has no .EXPECT() calls set up - mockery's mock.Mock
	// fails the test if Delete is called unexpectedly, verifying nothing
	// was deleted.
}

func (s *DeleteKeyboardSuite) TestDetach_ReferencingBuilds_DeletesKeyboardAnyway() {
	// detach must NOT call FindBuildsReferencingKeyboard at all - it
	// doesn't care about references.
	s.mockKeyboards.EXPECT().
		Delete(s.ctx, "kb1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteKeyboard(
		s.ctx, s.mockKeyboards, s.mockBuilds, s.mockImages,
		"alice", "kb1", cascadedelete.OnDeleteDetach,
	)

	s.Require().NoError(err)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteKeyboardSuite) TestCascade_NoReferencingBuilds_DeletesOnlyKeyboard() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(s.ctx, "alice", "kb1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Delete(s.ctx, "kb1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteKeyboard(
		s.ctx, s.mockKeyboards, s.mockBuilds, s.mockImages,
		"alice", "kb1", cascadedelete.OnDeleteCascade,
	)

	s.Require().NoError(err)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteKeyboardSuite) TestCascade_ReferencingBuilds_DeletesEachBuildThenKeyboard() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(s.ctx, "alice", "kb1").
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
	s.mockKeyboards.EXPECT().
		Delete(s.ctx, "kb1").
		Return(nil, nil)

	result, err := cascadedelete.DeleteKeyboard(
		s.ctx, s.mockKeyboards, s.mockBuilds, s.mockImages,
		"alice", "kb1", cascadedelete.OnDeleteCascade,
	)

	s.Require().NoError(err)
	s.ElementsMatch([]string{"build-1", "build-2"}, result.DeletedBuildIDs)
}

func (s *DeleteKeyboardSuite) TestCascade_BuildDeleteFails_ReturnsErrorWithoutDeletingKeyboard() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(s.ctx, "alice", "kb1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(s.ctx, "build-1").
		Return(nil, errors.New("dynamo unavailable"))

	_, err := cascadedelete.DeleteKeyboard(
		s.ctx, s.mockKeyboards, s.mockBuilds, s.mockImages,
		"alice", "kb1", cascadedelete.OnDeleteCascade,
	)

	s.Require().Error(err)
	// mockKeyboards has no .EXPECT() - verifies Delete was never reached.
}

func (s *DeleteKeyboardSuite) TestUnknownOnDelete_ReturnsError_CallsNothing() {
	_, err := cascadedelete.DeleteKeyboard(
		s.ctx, s.mockKeyboards, s.mockBuilds, s.mockImages,
		"alice", "kb1", cascadedelete.OnDelete("bogus"),
	)

	s.Require().Error(err)
	// mockKeyboards and mockBuilds have no .EXPECT() calls set up -
	// verifies the unknown value is rejected before any repository call.
}

func (s *DeleteKeyboardSuite) TestFindReferencingBuildsFails_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(s.ctx, "alice", "kb1").
		Return(nil, errors.New("query failed"))

	_, err := cascadedelete.DeleteKeyboard(
		s.ctx, s.mockKeyboards, s.mockBuilds, s.mockImages,
		"alice", "kb1", cascadedelete.OnDeleteBlock,
	)

	s.Require().Error(err)
	s.Require().NotErrorIs(err, cascadedelete.ErrBlocked)
}
