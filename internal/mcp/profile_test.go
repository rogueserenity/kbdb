package mcp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	ctxpkg "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type HandleGetProfileSuite struct {
	suite.Suite

	mockRepo *mocks.MockProfileRepository
}

func TestHandleGetProfileSuite(t *testing.T) {
	suite.Run(t, new(HandleGetProfileSuite))
}

func (s *HandleGetProfileSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
}

func (s *HandleGetProfileSuite) TestBlankIdentifier_Errors() {
	handler := handleGetProfile(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetProfileInput{Identifier: "  "})

	s.Require().Error(err)
}

func (s *HandleGetProfileSuite) TestDiscoverableByID_Returned() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{StytchUserID: "user-alice", Username: "alice", Discoverable: true, Bio: sp("keebs")}, nil)

	handler := handleGetProfile(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetProfileInput{Identifier: "user-alice"})

	s.Require().NoError(err)
	s.Equal("alice", out.Profile.Username)
	s.Require().NotNil(out.Profile.Bio)
	s.Equal("keebs", *out.Profile.Bio)
	s.False(out.Profile.HasAvatar)
}

func (s *HandleGetProfileSuite) TestDiscoverableByUsername_Returned() {
	s.mockRepo.EXPECT().Get(mock.Anything, "alice").Return(nil, repository.ErrNotFound)
	s.mockRepo.EXPECT().ResolveUsername(mock.Anything, "alice").Return("user-alice", nil)
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{StytchUserID: "user-alice", Username: "alice", Discoverable: true}, nil)

	handler := handleGetProfile(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetProfileInput{Identifier: "alice"})

	s.Require().NoError(err)
	s.Equal("alice", out.Profile.Username)
}

func (s *HandleGetProfileSuite) TestNonDiscoverable_Owner_Returned() {
	ctx := ctxpkg.WithUserID(s.T().Context(), "user-alice")
	s.mockRepo.EXPECT().Get(ctx, "user-alice").
		Return(&repository.Profile{StytchUserID: "user-alice", Username: "alice", Discoverable: false}, nil)

	handler := handleGetProfile(s.mockRepo)
	_, out, err := handler(ctx, nil, schema.GetProfileInput{Identifier: "user-alice"})

	s.Require().NoError(err)
	s.Equal("alice", out.Profile.Username)
}

func (s *HandleGetProfileSuite) TestNonDiscoverable_OtherCaller_NotFoundError() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{StytchUserID: "user-alice", Username: "alice", Discoverable: false}, nil)

	handler := handleGetProfile(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetProfileInput{Identifier: "user-alice"})

	s.Require().ErrorIs(err, errProfileNotFound)
}

func (s *HandleGetProfileSuite) TestNoSuchProfile_NotFoundError() {
	s.mockRepo.EXPECT().Get(mock.Anything, "ghost").Return(nil, repository.ErrNotFound)
	s.mockRepo.EXPECT().ResolveUsername(mock.Anything, "ghost").Return("", repository.ErrNotFound)

	handler := handleGetProfile(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetProfileInput{Identifier: "ghost"})

	s.Require().ErrorIs(err, errProfileNotFound)
}

func (s *HandleGetProfileSuite) TestStoreError_GenericError() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").Return(nil, errors.New("dynamo down"))

	handler := handleGetProfile(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetProfileInput{Identifier: "user-alice"})

	s.Require().Error(err)
	s.NotErrorIs(err, errProfileNotFound)
}

func sp(v string) *string { return &v }

type HandleCreateProfileSuite struct {
	suite.Suite

	mockRepo *mocks.MockProfileRepository
}

func TestHandleCreateProfileSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateProfileSuite))
}

func (s *HandleCreateProfileSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
}

func (s *HandleCreateProfileSuite) call(in schema.CreateProfileInput) (schema.CreateProfileOutput, error) {
	_, out, err := handleCreateProfile(s.mockRepo)(callerContext(s.T()), nil, in)
	return out, err
}

func (s *HandleCreateProfileSuite) TestValid_Created() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(p repository.Profile) bool {
		return p.Username == "alice"
	})).Return(&repository.Profile{StytchUserID: callerID, Username: "alice"}, nil)

	out, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{Username: "alice"}})

	s.Require().NoError(err)
	s.Equal("alice", out.Profile.Username)
}

func (s *HandleCreateProfileSuite) TestInvalidUsername_ErrorNoRepoCall() {
	_, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{Username: "AB"}})

	s.Require().Error(err)
	s.Contains(err.Error(), "username")
}

func (s *HandleCreateProfileSuite) TestUserPrefixUsername_Error() {
	_, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{Username: "user-alice"}})

	s.Require().Error(err)
}

func (s *HandleCreateProfileSuite) TestLinkHTTPURL_Error() {
	_, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{
		Username: "alice",
		Links:    []schema.ProfileLink{{Name: "site", URL: "http://x.example"}},
	}})

	s.Require().Error(err)
	s.Contains(err.Error(), "links[0].url")
}

func (s *HandleCreateProfileSuite) TestAlreadyExists_Error() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	_, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{Username: "alice"}})

	s.Require().ErrorIs(err, errProfileAlreadyExists)
}

func (s *HandleCreateProfileSuite) TestUsernameTaken_Error() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrUsernameTaken)

	_, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{Username: "alice"}})

	s.Require().Error(err)
	s.Equal(`username "alice" is already taken`, err.Error())
}

func (s *HandleCreateProfileSuite) TestRepoError_GenericError() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamo down"))

	_, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{Username: "alice"}})

	s.Require().Error(err)
	s.NotErrorIs(err, errProfileAlreadyExists)
}
