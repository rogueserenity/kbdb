package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

type CreateProfileSuite struct {
	suite.Suite

	mockRepo   *mocks.MockProfileRepository
	mockImages *mocks.MockProfileImageStore
	handler    http.HandlerFunc
}

func TestCreateProfileSuite(t *testing.T) {
	suite.Run(t, new(CreateProfileSuite))
}

func (s *CreateProfileSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
	s.mockImages = mocks.NewMockProfileImageStore(s.T())
	s.handler = CreateProfile(s.mockRepo, s.mockImages)
}

// profileUserID is the {userId} path value every CreateProfile test posts
// to; the owner/not-owner distinction is made by the caller identity on
// ctx, not by varying this.
const profileUserID = "user-alice"

// post builds a POST /v1/profile/{userId} request with body as JSON and the
// caller identity on ctx set to caller (empty for anonymous).
func (s *CreateProfileSuite) post(caller string, body any) *httptest.ResponseRecorder {
	s.T().Helper()

	var raw []byte
	switch b := body.(type) {
	case string:
		raw = []byte(b)
	default:
		var err error
		raw, err = json.Marshal(b)
		s.Require().NoError(err)
	}

	ctx := s.T().Context()
	if caller != "" {
		ctx = kbdbctx.WithUserID(ctx, caller)
	}

	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/profile/"+profileUserID, strings.NewReader(string(raw)))
	req.SetPathValue("identifier", profileUserID)

	rec := httptest.NewRecorder()
	s.handler(rec, req)
	return rec
}

func validInput() api.ProfileInput {
	return api.ProfileInput{Username: "alice"}
}

func (s *CreateProfileSuite) TestValidInput_201() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(p repository.Profile) bool {
		return p.Username == "alice"
	})).Return(&repository.Profile{StytchUserID: "user-alice", Username: "alice"}, nil)

	rec := s.post("user-alice", validInput())

	s.Equal(http.StatusCreated, rec.Code)
	var body api.Profile
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("alice", body.Username)
}

func (s *CreateProfileSuite) TestNotOwner_404_NoRepoCall() {
	rec := s.post("user-bob", validInput())

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateProfileSuite) TestAnonymous_404() {
	rec := s.post("", validInput())

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *CreateProfileSuite) TestMalformedBody_400() {
	rec := s.post("user-alice", "{not json")

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *CreateProfileSuite) TestInvalidUsername_400_InvalidParams() {
	rec := s.post("user-alice", api.ProfileInput{Username: "AB"})

	s.Equal(http.StatusBadRequest, rec.Code)
	var body struct {
		InvalidParams []struct {
			Name string `json:"name"`
		} `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Require().Len(body.InvalidParams, 1)
	s.Equal("username", body.InvalidParams[0].Name)
}

func (s *CreateProfileSuite) TestUserPrefixUsername_400() {
	rec := s.post("user-alice", api.ProfileInput{Username: "user-alice"})

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *CreateProfileSuite) TestLinkHTTPURL_400_IndexedParam() {
	in := validInput()
	in.Links = &[]api.ProfileLink{{Name: "site", Url: "http://x.example"}}
	rec := s.post("user-alice", in)

	s.Equal(http.StatusBadRequest, rec.Code)
	var body struct {
		InvalidParams []struct {
			Name string `json:"name"`
		} `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("links[0].url", body.InvalidParams[0].Name)
}

func (s *CreateProfileSuite) TestAlreadyHasProfile_409_ConflictType() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	rec := s.post("user-alice", validInput())

	s.Equal(http.StatusConflict, rec.Code)
	var body struct {
		Type string `json:"type"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("https://mykeebs.info/errors/conflict", body.Type)
}

func (s *CreateProfileSuite) TestUsernameTaken_409_UsernameUnavailableType() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrUsernameTaken)

	rec := s.post("user-alice", validInput())

	s.Equal(http.StatusConflict, rec.Code)
	var body struct {
		Type   string `json:"type"`
		Detail string `json:"detail"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("https://mykeebs.info/errors/username-unavailable", body.Type)
	s.Equal(`the username "alice" is already taken`, body.Detail)
}

func (s *CreateProfileSuite) TestRepoError_500() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamo down"))

	rec := s.post("user-alice", validInput())

	s.Equal(http.StatusInternalServerError, rec.Code)
}

type UpdateProfileSuite struct {
	suite.Suite

	mockRepo   *mocks.MockProfileRepository
	mockImages *mocks.MockProfileImageStore
	handler    http.HandlerFunc
}

func TestUpdateProfileSuite(t *testing.T) {
	suite.Run(t, new(UpdateProfileSuite))
}

func (s *UpdateProfileSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
	s.mockImages = mocks.NewMockProfileImageStore(s.T())
	s.handler = UpdateProfile(s.mockRepo, s.mockImages)
}

// put builds a PUT /v1/profile/{identifier} request with body as JSON and
// the caller identity on ctx set to caller (empty for anonymous).
func (s *UpdateProfileSuite) put(caller string, body any) *httptest.ResponseRecorder {
	s.T().Helper()

	var raw []byte
	switch b := body.(type) {
	case string:
		raw = []byte(b)
	default:
		var err error
		raw, err = json.Marshal(b)
		s.Require().NoError(err)
	}

	ctx := s.T().Context()
	if caller != "" {
		ctx = kbdbctx.WithUserID(ctx, caller)
	}

	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/profile/"+profileUserID, strings.NewReader(string(raw)))
	req.SetPathValue("identifier", profileUserID)

	rec := httptest.NewRecorder()
	s.handler(rec, req)
	return rec
}

func (s *UpdateProfileSuite) TestValidInput_200() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.MatchedBy(func(p repository.Profile) bool {
		return p.Username == "alice"
	})).Return(&repository.Profile{StytchUserID: "user-alice", Username: "alice"}, nil)

	rec := s.put("user-alice", api.ProfileInput{Username: "alice"})

	s.Equal(http.StatusOK, rec.Code)
	var body api.Profile
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("alice", body.Username)
	s.NotContains(rec.Body.String(), "user-alice") // IdP subject never leaked
}

func (s *UpdateProfileSuite) TestNotOwner_404_NoRepoCall() {
	rec := s.put("user-bob", api.ProfileInput{Username: "alice"})

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateProfileSuite) TestAnonymous_404() {
	rec := s.put("", api.ProfileInput{Username: "alice"})

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *UpdateProfileSuite) TestMalformedBody_400() {
	rec := s.put("user-alice", "{not json")

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *UpdateProfileSuite) TestInvalidUsername_400() {
	rec := s.put("user-alice", api.ProfileInput{Username: "AB"})

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *UpdateProfileSuite) TestNoProfile_404() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	rec := s.put("user-alice", api.ProfileInput{Username: "alice"})

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *UpdateProfileSuite) TestUsernameTaken_409_UsernameUnavailableType() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrUsernameTaken)

	rec := s.put("user-alice", api.ProfileInput{Username: "taken"})

	s.Equal(http.StatusConflict, rec.Code)
	var body struct {
		Type   string `json:"type"`
		Detail string `json:"detail"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("https://mykeebs.info/errors/username-unavailable", body.Type)
	s.Equal(`the username "taken" is already taken`, body.Detail)
}

func (s *UpdateProfileSuite) TestRepoError_500() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamo down"))

	rec := s.put("user-alice", api.ProfileInput{Username: "alice"})

	s.Equal(http.StatusInternalServerError, rec.Code)
}
