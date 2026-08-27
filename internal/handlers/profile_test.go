package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type GetProfileSuite struct {
	suite.Suite

	mockRepo   *mocks.MockProfileRepository
	mockImages *mocks.MockProfileImageStore
	handler    http.HandlerFunc
}

func TestGetProfileSuite(t *testing.T) {
	suite.Run(t, new(GetProfileSuite))
}

func (s *GetProfileSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
	s.mockImages = mocks.NewMockProfileImageStore(s.T())
	s.handler = GetProfile(s.mockRepo, s.mockImages)
}

func (s *GetProfileSuite) newRequest(ctx context.Context, identifier string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/profile/"+identifier, nil)
	req.SetPathValue("identifier", identifier)
	return req
}

func (s *GetProfileSuite) TestDiscoverableByID_200() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{StytchUserID: "user-alice", Username: "alice", Discoverable: true, Bio: strp("keebs")}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "user-alice"))

	s.Equal(http.StatusOK, rec.Code)
	var body api.Profile
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("alice", body.Username)
	s.Require().NotNil(body.Bio)
	s.Equal("keebs", *body.Bio)
}

func (s *GetProfileSuite) TestDiscoverableByUsername_200() {
	s.mockRepo.EXPECT().Get(mock.Anything, "alice").Return(nil, repository.ErrNotFound)
	s.mockRepo.EXPECT().ResolveUsername(mock.Anything, "alice").Return("user-alice", nil)
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{StytchUserID: "user-alice", Username: "alice", Discoverable: true}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "alice"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetProfileSuite) TestNonDiscoverable_Owner_200() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "user-alice")
	s.mockRepo.EXPECT().Get(ctx, "user-alice").
		Return(&repository.Profile{StytchUserID: "user-alice", Username: "alice", Discoverable: false}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "user-alice"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetProfileSuite) TestNonDiscoverable_OtherCaller_404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "user-bob")
	s.mockRepo.EXPECT().Get(ctx, "user-alice").
		Return(&repository.Profile{StytchUserID: "user-alice", Username: "alice", Discoverable: false}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "user-alice"))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetProfileSuite) TestNonDiscoverable_Anonymous_404() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{StytchUserID: "user-alice", Username: "alice", Discoverable: false}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "user-alice"))

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *GetProfileSuite) TestNoSuchProfile_404() {
	s.mockRepo.EXPECT().Get(mock.Anything, "ghost").Return(nil, repository.ErrNotFound)
	s.mockRepo.EXPECT().ResolveUsername(mock.Anything, "ghost").Return("", repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "ghost"))

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *GetProfileSuite) TestStoreError_500() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").Return(nil, errors.New("dynamo down"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "user-alice"))

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func strp(v string) *string { return &v }
