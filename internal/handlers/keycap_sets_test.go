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
	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")

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
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_OtherUser_RequestsPublicAndAuthenticated() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

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
	s.handler(rec, s.newRequest(s.T().Context(), "limit=5&cursor=abc"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_ReturnsNextCursor_WhenPresent() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.KeycapSet{}, "next-page-token", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

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
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type GetKeycapSetSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
	handler    http.HandlerFunc
}

func TestGetKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(GetKeycapSetSuite))
}

func (s *GetKeycapSetSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = GetKeycapSet(s.mockRepo, s.mockImages)
}

func (s *GetKeycapSetSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/keycap-sets/ks1", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	return req
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_Owner_Succeeds() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")

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

func (s *GetKeycapSetSuite) TestGetKeycapSet_KitWithImagePath_IncludesPresignedURL() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imagePath := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			ID:         "ks1",
			Visibility: repository.VisibilityPrivate,
			Kits:       []repository.KeycapKit{{KitID: "kit1", Name: "Base", ImagePath: &imagePath}},
		}, nil)
	s.mockImages.EXPECT().
		PresignGet(mock.Anything, imagePath).
		Return("https://example.com/presigned-get", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)

	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Kits)
	s.Require().Len(*got.Kits, 1)
	s.Require().NotNil((*got.Kits)[0].Image)
	s.Equal("https://example.com/presigned-get", (*got.Kits)[0].Image.Url)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_AnonymousReadingPublicKeycapSet_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityPublic}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_AnonymousReadingPrivateKeycapSet_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_OtherUserReadingAuthenticatedKeycapSet_Succeeds() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityAuthenticated}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_OtherUserReadingPrivateKeycapSet_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

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
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(nil, errors.New("get item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_MalformedKitPurchaseDate_Returns500NotPanic() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	malformedDate := "not-a-date"
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			ID:         "ks1",
			Visibility: repository.VisibilityPrivate,
			Kits: []repository.KeycapKit{
				{KitID: "kit1", Purchase: repository.KeycapKitPurchase{OrderDate: &malformedDate}},
			},
		}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type CreateKeycapSetSuite struct {
	suite.Suite

	mockKeycapSetRepo *mocks.MockKeycapSetRepository
	mockImages        *mocks.MockKeycapKitImageStore
	handler           http.HandlerFunc
}

func TestCreateKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(CreateKeycapSetSuite))
}

func (s *CreateKeycapSetSuite) SetupTest() {
	s.mockKeycapSetRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = CreateKeycapSet(s.mockKeycapSetRepo, s.mockImages)
}

func (s *CreateKeycapSetSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/keycap-sets", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	return req
}

func (s *CreateKeycapSetSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
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
		name string
		body string
	}{
		{"profile", `{"brand":"GMK","name":"Laser","visibility":"private","profile":"NotApproved"}`},
		{"material", `{"brand":"GMK","name":"Laser","visibility":"private","material":"NotApproved"}`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

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
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
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

type UpdateKeycapSetSuite struct {
	suite.Suite

	mockKeycapSetRepo *mocks.MockKeycapSetRepository
	mockImages        *mocks.MockKeycapKitImageStore
	handler           http.HandlerFunc
}

func TestUpdateKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(UpdateKeycapSetSuite))
}

func (s *UpdateKeycapSetSuite) SetupTest() {
	s.mockKeycapSetRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = UpdateKeycapSet(s.mockKeycapSetRepo, s.mockImages)
}

func (s *UpdateKeycapSetSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/users/alice/keycap-sets/ks1", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	return req
}

func (s *UpdateKeycapSetSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_Succeeds() {
	s.mockKeycapSetRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(ks repository.KeycapSet) bool {
			return ks.ID == "ks1" && ks.Brand == "GMK" && ks.Visibility == repository.VisibilityPrivate
		})).
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Laser"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("ks1", got.Id)
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_Visibility_Preserved() {
	s.mockKeycapSetRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(ks repository.KeycapSet) bool {
			return ks.Visibility == repository.VisibilityPublic
		})).
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"public"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"profile", `{"brand":"GMK","name":"Laser","visibility":"private","profile":"NotApproved"}`},
		{"material", `{"brand":"GMK","name":"Laser","visibility":"private","material":"NotApproved"}`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

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

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_MultipleInvalidFields_NamesAll() {
	// profile and material are both invalid here - the response must
	// report both via invalid_params, not just the first one checked.
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

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_NotFound_Returns404() {
	s.mockKeycapSetRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_RepositoryError_Returns500() {
	s.mockKeycapSetRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_MutationConflict_Returns409() {
	s.mockKeycapSetRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteKeycapSetSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
	handler    http.HandlerFunc
}

func TestDeleteKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(DeleteKeycapSetSuite))
}

func (s *DeleteKeycapSetSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = DeleteKeycapSet(s.mockRepo, s.mockImages)
}

func (s *DeleteKeycapSetSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/users/alice/keycap-sets/ks1", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	return req
}

func (s *DeleteKeycapSetSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_Owner_Succeeds() {
	s.mockRepo.EXPECT().
		Delete(mock.Anything, "ks1").
		Return(nil, nil)
	s.mockImages.EXPECT().
		BestEffortDelete(mock.Anything, []repository.KeycapKitImageKey(nil)).
		Return()

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_KitsWithImages_BestEffortDeletesThem() {
	keys := []repository.KeycapKitImageKey{"keycap-sets/alice/ks1/kits/kit1/image", "keycap-sets/alice/ks1/kits/kit2/image"}
	s.mockRepo.EXPECT().
		Delete(mock.Anything, "ks1").
		Return(keys, nil)
	s.mockImages.EXPECT().
		BestEffortDelete(mock.Anything, keys).
		Return()

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_RepositoryNotFound_StillReturns204() {
	s.mockRepo.EXPECT().
		Delete(mock.Anything, "ks1").
		Return(nil, repository.ErrNotFound)
	s.mockImages.EXPECT().
		BestEffortDelete(mock.Anything, []repository.KeycapKitImageKey(nil)).
		Return()

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_Anonymous_Returns404() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		Delete(mock.Anything, "ks1").
		Return(nil, errors.New("delete item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type CreateKeycapKitSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
	handler    http.HandlerFunc
}

func TestCreateKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(CreateKeycapKitSuite))
}

func (s *CreateKeycapKitSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = CreateKeycapKit(s.mockRepo, s.mockImages)
}

func (s *CreateKeycapKitSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/keycap-sets/ks1/kits", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	return req
}

func (s *CreateKeycapKitSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_Succeeds() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks1", mock.MatchedBy(func(k repository.KeycapKit) bool {
			return k.KitID != "" && k.Name == "Base"
		})).
		Return(&repository.KeycapKit{KitID: "kit1", Name: "Base"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"name":"Base"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.KeycapKit
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("kit1", got.KitId)
	s.Equal("Base", got.Name)
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"purchase.vendor", `{"name":"Base","purchase":{"vendor":"NotApproved"}}`},
		{"purchase.order_status", `{"name":"Base","purchase":{"order_status":"NotApproved"}}`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

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

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"name":"Base"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"name":"Base"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_ParentSetNotFound_Returns404() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks1", mock.Anything).
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"name":"Base"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks1", mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"name":"Base"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_MutationConflict_Returns409() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks1", mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	req := s.newRequest(s.ownerCtx(), `{"name":"Base"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type UpdateKeycapKitSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
	handler    http.HandlerFunc
}

func TestUpdateKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(UpdateKeycapKitSuite))
}

func (s *UpdateKeycapKitSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = UpdateKeycapKit(s.mockRepo, s.mockImages)
}

func (s *UpdateKeycapKitSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/users/alice/keycap-sets/ks1/kits/kit1", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	req.SetPathValue("kitId", "kit1")
	return req
}

func (s *UpdateKeycapKitSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_Succeeds() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks1", mock.MatchedBy(func(k repository.KeycapKit) bool {
			return k.KitID == "kit1" && k.Name == "Extension"
		})).
		Return(&repository.KeycapKit{KitID: "kit1", Name: "Extension"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"name":"Extension"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.KeycapKit
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("kit1", got.KitId)
	s.Equal("Extension", got.Name)
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"purchase.vendor", `{"name":"Extension","purchase":{"vendor":"NotApproved"}}`},
		{"purchase.order_status", `{"name":"Extension","purchase":{"order_status":"NotApproved"}}`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

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

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"name":"Extension"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"name":"Extension"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks1", mock.Anything).
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"name":"Extension"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks1", mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"name":"Extension"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_MutationConflict_Returns409() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks1", mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	req := s.newRequest(s.ownerCtx(), `{"name":"Extension"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteKeycapKitSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
	handler    http.HandlerFunc
}

func TestDeleteKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(DeleteKeycapKitSuite))
}

func (s *DeleteKeycapKitSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = DeleteKeycapKit(s.mockRepo, s.mockImages)
}

func (s *DeleteKeycapKitSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/users/alice/keycap-sets/ks1/kits/kit1", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	req.SetPathValue("kitId", "kit1")
	return req
}

func (s *DeleteKeycapKitSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_Succeeds() {
	s.mockRepo.EXPECT().
		DeleteKit(mock.Anything, "ks1", "kit1").
		Return(nil, nil)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_KitHadImage_DeletesFromImages() {
	key := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image")
	s.mockRepo.EXPECT().
		DeleteKit(mock.Anything, "ks1", "kit1").
		Return(&key, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, key).
		Return(nil)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_ImageDeleteFails_StillReturns204() {
	key := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image")
	s.mockRepo.EXPECT().
		DeleteKit(mock.Anything, "ks1", "kit1").
		Return(&key, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, key).
		Return(errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_ParentSetNotFound_Returns404() {
	s.mockRepo.EXPECT().
		DeleteKit(mock.Anything, "ks1", "kit1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		DeleteKit(mock.Anything, "ks1", "kit1").
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_MutationConflict_Returns409() {
	s.mockRepo.EXPECT().
		DeleteKit(mock.Anything, "ks1", "kit1").
		Return(nil, repository.ErrMutationConflict)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type SetKeycapKitImageSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
	handler    http.HandlerFunc
}

func TestSetKeycapKitImageSuite(t *testing.T) {
	suite.Run(t, new(SetKeycapKitImageSuite))
}

func (s *SetKeycapKitImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = SetKeycapKitImage(s.mockRepo, s.mockImages)
}

func (s *SetKeycapKitImageSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/keycap-sets/ks1/kits/kit1/image", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	req.SetPathValue("kitId", "kit1")
	return req
}

func (s *SetKeycapKitImageSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

const setKeycapKitImageTestKey = repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image")

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_Succeeds() {
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, setKeycapKitImageTestKey, "image/png").
		Return("https://example.com/presigned-put", nil)
	s.mockRepo.EXPECT().
		SetKitImagePath(mock.Anything, "ks1", "kit1", setKeycapKitImageTestKey).
		Return(&repository.KeycapKit{KitID: "kit1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got struct {
		UploadURL string `json:"upload_url"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("https://example.com/presigned-put", got.UploadURL)
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_UnapprovedContentType_Returns400() {
	req := s.newRequest(s.ownerCtx(), `{"content_type":"application/pdf"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("content_type", got.InvalidParams[0].Name)
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_PresignError_Returns500() {
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, setKeycapKitImageTestKey, "image/png").
		Return("", errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_NotFound_Returns404() {
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, setKeycapKitImageTestKey, "image/png").
		Return("https://example.com/presigned-put", nil)
	s.mockRepo.EXPECT().
		SetKitImagePath(mock.Anything, "ks1", "kit1", setKeycapKitImageTestKey).
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_RepositoryError_Returns500() {
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, setKeycapKitImageTestKey, "image/png").
		Return("https://example.com/presigned-put", nil)
	s.mockRepo.EXPECT().
		SetKitImagePath(mock.Anything, "ks1", "kit1", setKeycapKitImageTestKey).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_MutationConflict_Returns409() {
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, setKeycapKitImageTestKey, "image/png").
		Return("https://example.com/presigned-put", nil)
	s.mockRepo.EXPECT().
		SetKitImagePath(mock.Anything, "ks1", "kit1", setKeycapKitImageTestKey).
		Return(nil, repository.ErrMutationConflict)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteKeycapKitImageSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
	handler    http.HandlerFunc
}

func TestDeleteKeycapKitImageSuite(t *testing.T) {
	suite.Run(t, new(DeleteKeycapKitImageSuite))
}

func (s *DeleteKeycapKitImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = DeleteKeycapKitImage(s.mockRepo, s.mockImages)
}

func (s *DeleteKeycapKitImageSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/users/alice/keycap-sets/ks1/kits/kit1/image", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	req.SetPathValue("kitId", "kit1")
	return req
}

func (s *DeleteKeycapKitImageSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

var deleteKeycapKitImageTestKey = repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image")

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_Succeeds() {
	s.mockRepo.EXPECT().
		ClearKitImagePath(mock.Anything, "ks1", "kit1").
		Return(&deleteKeycapKitImageTestKey, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, deleteKeycapKitImageTestKey).
		Return(nil)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_AlreadyAbsent_SucceedsWithoutS3Call() {
	s.mockRepo.EXPECT().
		ClearKitImagePath(mock.Anything, "ks1", "kit1").
		Return(nil, nil)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		ClearKitImagePath(mock.Anything, "ks1", "kit1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_MutationConflict_Returns409() {
	s.mockRepo.EXPECT().
		ClearKitImagePath(mock.Anything, "ks1", "kit1").
		Return(nil, repository.ErrMutationConflict)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		ClearKitImagePath(mock.Anything, "ks1", "kit1").
		Return(nil, errors.New("get item failed"))

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_S3DeleteError_Returns500() {
	s.mockRepo.EXPECT().
		ClearKitImagePath(mock.Anything, "ks1", "kit1").
		Return(&deleteKeycapKitImageTestKey, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, deleteKeycapKitImageTestKey).
		Return(errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}
