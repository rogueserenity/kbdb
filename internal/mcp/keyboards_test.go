package mcp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type HandleListKeyboardsSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeyboardRepository
}

func TestHandleListKeyboardsSuite(t *testing.T) {
	suite.Run(t, new(HandleListKeyboardsSuite))
}

func (s *HandleListKeyboardsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeyboardRepository(s.T())
}

func (s *HandleListKeyboardsSuite) TestBlankUserID_DefaultsToCaller() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, defaultListLimit, "").
		Return([]repository.Keyboard{{ID: "kb-1", Brand: "Mode", Name: "Sixty"}}, "", nil)

	handler := handleListKeyboards(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Keyboards, 1)
	s.Equal("kb-1", out.Keyboards[0].ID)
}

func (s *HandleListKeyboardsSuite) TestOwnCollection_ReadsAllVisibilityTiers() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
			repository.VisibilityPrivate,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)

	handler := handleListKeyboards(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{})

	s.Require().NoError(err)
}

func (s *HandleListKeyboardsSuite) TestOtherUsersCollection_ExcludesPrivate() {
	s.mockRepo.EXPECT().
		List(mock.Anything, otherID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)

	handler := handleListKeyboards(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{UserID: otherID})

	s.Require().NoError(err)
}

func (s *HandleListKeyboardsSuite) TestLimitAboveMax_IsClamped() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, maxListLimit, mock.Anything).
		Return(nil, "", nil)

	handler := handleListKeyboards(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{Limit: 5000})

	s.Require().NoError(err)
}

func (s *HandleListKeyboardsSuite) TestCursorIsPropagated() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, "page-2").
		Return(nil, "page-3", nil)

	handler := handleListKeyboards(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{Cursor: "page-2"})

	s.Require().NoError(err)
	s.Equal("page-3", out.NextCursor)
}

func (s *HandleListKeyboardsSuite) TestNoCallerIdentity_ReturnsError() {
	handler := handleListKeyboards(s.mockRepo)
	_, _, err := handler(s.T().Context(), nil, schema.ListKeyboardsInput{})

	s.Require().ErrorIs(err, errNoCallerIdentity)
}

func (s *HandleListKeyboardsSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, "", errors.New("query failed"))

	handler := handleListKeyboards(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{})

	s.Require().ErrorContains(err, "failed to list keyboards")
}

type HandleGetKeyboardSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeyboardRepository
}

func TestHandleGetKeyboardSuite(t *testing.T) {
	suite.Run(t, new(HandleGetKeyboardSuite))
}

func (s *HandleGetKeyboardSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeyboardRepository(s.T())
}

func (s *HandleGetKeyboardSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(&repository.Keyboard{
			ID:         "kb-1",
			Brand:      "Mode",
			Name:       "Sixty",
			Visibility: repository.VisibilityPrivate,
		}, nil)

	handler := handleGetKeyboard(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1"})

	s.Require().NoError(err)
	s.Equal("kb-1", out.Keyboard.ID)
	s.Equal("Mode", out.Keyboard.Brand)
}

func (s *HandleGetKeyboardSuite) TestBlankKeyboardID_ReturnsError() {
	handler := handleGetKeyboard(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "  "})

	s.Require().ErrorContains(err, "keyboard_id must not be blank")
}

func (s *HandleGetKeyboardSuite) TestNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleGetKeyboard(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "missing"})

	s.Require().ErrorIs(err, errKeyboardNotFound)
}

// An unreadable keyboard must be indistinguishable from a missing one, so a
// caller can't probe for the existence of another user's private items.
func (s *HandleGetKeyboardSuite) TestOtherUsersPrivateKeyboard_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Visibility: repository.VisibilityPrivate}, nil)

	handler := handleGetKeyboard(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1", UserID: otherID})

	s.Require().ErrorIs(err, errKeyboardNotFound)
}

func (s *HandleGetKeyboardSuite) TestOtherUsersPublicKeyboard_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "kb-1").
		Return(&repository.Keyboard{
			ID:         "kb-1",
			Brand:      "Mode",
			Visibility: repository.VisibilityPublic,
		}, nil)

	handler := handleGetKeyboard(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1", UserID: otherID})

	s.Require().NoError(err)
	s.Equal("kb-1", out.Keyboard.ID)
}

func (s *HandleGetKeyboardSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("get failed"))

	handler := handleGetKeyboard(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1"})

	s.Require().ErrorContains(err, "failed to get keyboard")
}
