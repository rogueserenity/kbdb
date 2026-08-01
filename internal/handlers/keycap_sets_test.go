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
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type ListKeycapSetsSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeycapSetRepository
	handler  http.HandlerFunc
}

func TestListKeycapSetsSuite(t *testing.T) {
	suite.Run(t, new(ListKeycapSetsSuite))
}

func (s *ListKeycapSetsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.handler = ListKeycapSets(s.mockRepo)
}

func (s *ListKeycapSetsSuite) newRequest(ctx context.Context, query string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/keycap-sets?"+query, nil)
	req.SetPathValue("userId", "alice")
	return req
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_Owner_RequestsAllVisibilities() {
	ctx := kbdbctx.WithUserID(context.Background(), "alice")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 3
		}), 20, "").
		Return([]repository.KeycapSet{{ID: "ks1", Brand: "GMK", Name: "Laser"}}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.KeycapSetListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	id, brand, name := "ks1", "GMK", "Laser"
	s.Equal(&[]api.KeycapSetSummary{{Id: &id, Brand: &brand, Name: &name}}, got.Items)
	s.Nil(got.NextCursor)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_Anonymous_RequestsPublicOnly() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", []repository.Visibility{repository.VisibilityPublic}, 20, "").
		Return([]repository.KeycapSet{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_OtherUser_RequestsPublicAndAuthenticated() {
	ctx := kbdbctx.WithUserID(context.Background(), "bob")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 2
		}), 20, "").
		Return([]repository.KeycapSet{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_PassesLimitAndCursor() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 5, "abc").
		Return([]repository.KeycapSet{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), "limit=5&cursor=abc"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_ReturnsNextCursor_WhenPresent() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.KeycapSet{}, "next-page-token", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), "limit=20"))

	var got api.KeycapSetListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.NextCursor)
	s.Equal("next-page-token", *got.NextCursor)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return(nil, "", errors.New("query failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), "limit=20"))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type GetKeycapSetSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeycapSetRepository
	handler  http.HandlerFunc
}

func TestGetKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(GetKeycapSetSuite))
}

func (s *GetKeycapSetSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.handler = GetKeycapSet(s.mockRepo)
}

func (s *GetKeycapSetSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/keycap-sets/ks1", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("id", "ks1")
	return req
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_Owner_Succeeds() {
	ctx := kbdbctx.WithUserID(context.Background(), "alice")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Brand: "GMK", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("ks1", got.ID)
	s.Equal("GMK", got.Brand)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_AnonymousReadingPublicKeycapSet_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityPublic}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background()))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_AnonymousReadingPrivateKeycapSet_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_OtherUserReadingAuthenticatedKeycapSet_Succeeds() {
	ctx := kbdbctx.WithUserID(context.Background(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityAuthenticated}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_OtherUserReadingPrivateKeycapSet_Returns404() {
	ctx := kbdbctx.WithUserID(context.Background(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(nil, errors.New("get item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type CreateKeycapSetSuite struct {
	suite.Suite

	mockKeycapSetRepo *mocks.MockKeycapSetRepository
	mockLookupRepo    *mocks.MockLookupRepository
	handler           http.HandlerFunc
}

func TestCreateKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(CreateKeycapSetSuite))
}

func (s *CreateKeycapSetSuite) SetupTest() {
	s.mockKeycapSetRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockLookupRepo = mocks.NewMockLookupRepository(s.T())
	s.handler = CreateKeycapSet(s.mockKeycapSetRepo, s.mockLookupRepo)
}

func (s *CreateKeycapSetSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/keycap-sets", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	return req
}

func (s *CreateKeycapSetSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(context.Background(), "alice")
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_Succeeds() {
	s.mockKeycapSetRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(ks repository.KeycapSet) bool {
			return ks.ID != "" && ks.Brand == "GMK" && ks.Visibility == repository.VisibilityPrivate
		})).
		Return(&repository.KeycapSet{UserID: "alice", ID: "generated-id", Brand: "GMK", Name: "Laser"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("generated-id", got.Id)
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_Visibility_Preserved() {
	s.mockKeycapSetRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(ks repository.KeycapSet) bool {
			return ks.Visibility == repository.VisibilityPublic
		})).
		Return(&repository.KeycapSet{UserID: "alice", ID: "generated-id"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"public"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name     string
		category string
		body     string
	}{
		{"profile", "keycap_profile", `{"brand":"GMK","name":"Laser","visibility":"private","profile":"NotApproved"}`},
		{"material", "keycap_material", `{"brand":"GMK","name":"Laser","visibility":"private","material":"NotApproved"}`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			s.mockLookupRepo.EXPECT().
				GetCategory(mock.Anything, tt.category).
				Return(&repository.Lookup{Category: tt.category, Values: []any{"Approved"}}, nil)

			req := s.newRequest(s.ownerCtx(), tt.body)
			rec := httptest.NewRecorder()
			s.handler(rec, req)

			s.Equal(http.StatusBadRequest, rec.Code)
			s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

			var got struct {
				InvalidParams []problem.InvalidParam `json:"invalid_params"`
			}
			s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
			s.Require().Len(got.InvalidParams, 1)
			s.Equal(tt.name, got.InvalidParams[0].Name)
		})
	}
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_MultipleInvalidFields_NamesAll() {
	// profile and material are both invalid here - the response must
	// report both via invalid_params, not just the first one checked.
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keycap_profile").
		Return(&repository.Lookup{Category: "keycap_profile", Values: []any{"Cherry"}}, nil)
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keycap_material").
		Return(&repository.Lookup{Category: "keycap_material", Values: []any{"ABS"}}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"GMK","name":"Laser","visibility":"private","profile":"NotApproved",`+
			`"material":"AlsoNotApproved"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	names := make([]string, len(got.InvalidParams))
	for i, p := range got.InvalidParams {
		names[i] = p.Name
	}
	s.Contains(names, "profile")
	s.Contains(names, "material")
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(context.Background(), "bob")

	req := s.newRequest(ctx, `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_Anonymous_Returns404() {
	req := s.newRequest(context.Background(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_LookupCategoryMissing_Returns400() {
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keycap_profile").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"GMK","name":"Laser","visibility":"private","profile":"Cherry"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_LookupRepositoryError_Returns500() {
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keycap_profile").
		Return(nil, errors.New("get item failed"))

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"GMK","name":"Laser","visibility":"private","profile":"Cherry"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_AlreadyExists_Returns409() {
	s.mockKeycapSetRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_RepositoryError_Returns500() {
	s.mockKeycapSetRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}
