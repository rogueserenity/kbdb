package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	ctxpkg "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

const (
	callerID = "caller-0001"
	otherID  = "other-0002"
)

// callerContext mimics what identityMiddleware puts on ctx for a verified
// caller, since these handlers run downstream of it.
func callerContext(t *testing.T) context.Context {
	t.Helper()

	return ctxpkg.WithUserID(t.Context(), callerID)
}

type HandleListSwitchesSuite struct {
	suite.Suite

	mockRepo *mocks.MockSwitchRepository
}

func TestHandleListSwitchesSuite(t *testing.T) {
	suite.Run(t, new(HandleListSwitchesSuite))
}

func (s *HandleListSwitchesSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
}

func (s *HandleListSwitchesSuite) TestBlankUserID_DefaultsToCaller() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, defaultListLimit, "").
		Return([]repository.Switch{{ID: "sw-1", Brand: "Gateron", Name: "Oil King", Type: "linear"}}, "", nil)

	handler := handleListSwitches(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Switches, 1)
	s.Equal("sw-1", out.Switches[0].ID)
	s.Empty(out.NextCursor)
}

// The caller reading their own collection may see every tier; reading
// someone else's is capped at public + authenticated.
func (s *HandleListSwitchesSuite) TestOwnCollection_ReadsAllVisibilityTiers() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
			repository.VisibilityPrivate,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)

	handler := handleListSwitches(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{})

	s.Require().NoError(err)
}

func (s *HandleListSwitchesSuite) TestOtherUsersCollection_ExcludesPrivate() {
	s.mockRepo.EXPECT().
		List(mock.Anything, otherID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)

	handler := handleListSwitches(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{UserID: otherID})

	s.Require().NoError(err)
}

func (s *HandleListSwitchesSuite) TestLimitAboveMax_IsClamped() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, maxListLimit, mock.Anything).
		Return(nil, "", nil)

	handler := handleListSwitches(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{Limit: 5000})

	s.Require().NoError(err)
}

func (s *HandleListSwitchesSuite) TestCursorIsPropagated() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, "page-2").
		Return(nil, "page-3", nil)

	handler := handleListSwitches(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{Cursor: "page-2"})

	s.Require().NoError(err)
	s.Equal("page-3", out.NextCursor)
}

func (s *HandleListSwitchesSuite) TestNoCallerIdentity_ReturnsError() {
	handler := handleListSwitches(s.mockRepo)
	_, _, err := handler(s.T().Context(), nil, schema.ListSwitchesInput{})

	s.Require().ErrorIs(err, errNoCallerIdentity)
}

func (s *HandleListSwitchesSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, "", errors.New("query failed"))

	handler := handleListSwitches(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{})

	s.Require().ErrorContains(err, "failed to list switches")
}

type HandleGetSwitchSuite struct {
	suite.Suite

	mockRepo *mocks.MockSwitchRepository
}

func TestHandleGetSwitchSuite(t *testing.T) {
	suite.Run(t, new(HandleGetSwitchSuite))
}

func (s *HandleGetSwitchSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
}

func (s *HandleGetSwitchSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "sw-1").
		Return(&repository.Switch{
			ID:         "sw-1",
			Brand:      "Gateron",
			Name:       "Oil King",
			Type:       "linear",
			Visibility: repository.VisibilityPrivate,
		}, nil)

	handler := handleGetSwitch(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "sw-1"})

	s.Require().NoError(err)
	s.Equal("sw-1", out.Switch.ID)
	s.Equal("Gateron", out.Switch.Brand)
}

func (s *HandleGetSwitchSuite) TestBlankSwitchID_ReturnsError() {
	handler := handleGetSwitch(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "  "})

	s.Require().ErrorContains(err, "switch_id must not be blank")
}

func (s *HandleGetSwitchSuite) TestNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleGetSwitch(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "missing"})

	s.Require().ErrorIs(err, errSwitchNotFound)
}

// An unreadable switch must be indistinguishable from a missing one, so a
// caller can't probe for the existence of another user's private items.
func (s *HandleGetSwitchSuite) TestOtherUsersPrivateSwitch_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "sw-1").
		Return(&repository.Switch{
			ID:         "sw-1",
			Visibility: repository.VisibilityPrivate,
		}, nil)

	handler := handleGetSwitch(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "sw-1", UserID: otherID})

	s.Require().ErrorIs(err, errSwitchNotFound)
}

func (s *HandleGetSwitchSuite) TestOtherUsersPublicSwitch_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "sw-1").
		Return(&repository.Switch{
			ID:         "sw-1",
			Brand:      "Gateron",
			Visibility: repository.VisibilityPublic,
		}, nil)

	handler := handleGetSwitch(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "sw-1", UserID: otherID})

	s.Require().NoError(err)
	s.Equal("sw-1", out.Switch.ID)
}

func (s *HandleGetSwitchSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("get failed"))

	handler := handleGetSwitch(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "sw-1"})

	s.Require().ErrorContains(err, "failed to get switch")
}
