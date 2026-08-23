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

type ListSwitchesSuite struct {
	suite.Suite

	mockRepo   *mocks.MockSwitchRepository
	mockImages *mocks.MockSwitchImageStore
	handler    http.HandlerFunc
}

func TestListSwitchesSuite(t *testing.T) {
	suite.Run(t, new(ListSwitchesSuite))
}

func (s *ListSwitchesSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
	s.handler = ListSwitches(s.mockRepo, s.mockImages)
}

func (s *ListSwitchesSuite) newRequest(ctx context.Context, query string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/switches?"+query, nil)
	req.SetPathValue("userId", "alice")
	return req
}

func (s *ListSwitchesSuite) TestListSwitches_Owner_RequestsAllVisibilities() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 3
		}), 20, "").
		Return([]repository.Switch{{ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear"}}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.SwitchListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	id, brand, name, typ := "sw1", "Gateron", "Yellow", "Linear"
	s.Equal(&[]api.SwitchSummary{{Id: &id, Brand: &brand, Name: &name, Type: &typ}}, got.Items)
	s.Nil(got.NextCursor)
}

func (s *ListSwitchesSuite) TestListSwitches_Anonymous_RequestsPublicOnly() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", []repository.Visibility{repository.VisibilityPublic}, 20, "").
		Return([]repository.Switch{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListSwitchesSuite) TestListSwitches_OtherUser_RequestsPublicAndAuthenticated() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 2
		}), 20, "").
		Return([]repository.Switch{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListSwitchesSuite) TestListSwitches_PassesLimitAndCursor() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 5, "abc").
		Return([]repository.Switch{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=5&cursor=abc"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListSwitchesSuite) TestListSwitches_ReturnsNextCursor_WhenPresent() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Switch{}, "next-page-token", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	var got api.SwitchListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.NextCursor)
	s.Equal("next-page-token", *got.NextCursor)
}

func (s *ListSwitchesSuite) TestListSwitches_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return(nil, "", errors.New("query failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type GetSwitchSuite struct {
	suite.Suite

	mockRepo   *mocks.MockSwitchRepository
	mockImages *mocks.MockSwitchImageStore
	handler    http.HandlerFunc
}

func TestGetSwitchSuite(t *testing.T) {
	suite.Run(t, new(GetSwitchSuite))
}

func (s *GetSwitchSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
	s.handler = GetSwitch(s.mockRepo, s.mockImages)
}

func (s *GetSwitchSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/switches/sw1", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("switchId", "sw1")
	return req
}

func (s *GetSwitchSuite) TestGetSwitch_Owner_Succeeds() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", Brand: "Gateron", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Switch
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("sw1", got.ID)
	s.Equal("Gateron", got.Brand)
}

func (s *GetSwitchSuite) TestGetSwitch_AnonymousReadingPublicSwitch_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", Visibility: repository.VisibilityPublic}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetSwitchSuite) TestGetSwitch_AnonymousReadingPrivateSwitch_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetSwitchSuite) TestGetSwitch_OtherUserReadingAuthenticatedSwitch_Succeeds() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", Visibility: repository.VisibilityAuthenticated}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetSwitchSuite) TestGetSwitch_OtherUserReadingPrivateSwitch_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetSwitchSuite) TestGetSwitch_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetSwitchSuite) TestGetSwitch_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(nil, errors.New("get item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetSwitchSuite) TestGetSwitch_MalformedStoredDate_Returns500NotPanic() {
	malformedDate := "not-a-date"
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			ID:         "sw1",
			Visibility: repository.VisibilityPublic,
			Purchase:   repository.SwitchPurchase{OrderDate: &malformedDate},
		}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type CreateSwitchSuite struct {
	suite.Suite

	mockSwitchRepo *mocks.MockSwitchRepository
	mockImages     *mocks.MockSwitchImageStore
	handler        http.HandlerFunc
}

func TestCreateSwitchSuite(t *testing.T) {
	suite.Run(t, new(CreateSwitchSuite))
}

func (s *CreateSwitchSuite) SetupTest() {
	s.mockSwitchRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
	s.handler = CreateSwitch(s.mockSwitchRepo, s.mockImages)
}

func (s *CreateSwitchSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/switches", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	return req
}

func (s *CreateSwitchSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

func (s *CreateSwitchSuite) TestCreateSwitch_Succeeds() {
	s.mockSwitchRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(sw repository.Switch) bool {
			return sw.ID != "" && sw.Brand == "Gateron" && sw.Visibility == repository.VisibilityPrivate
		})).
		Return(&repository.Switch{UserID: "alice", ID: "generated-id", Brand: "Gateron", Name: "Yellow", Type: "Linear"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Switch
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("generated-id", got.ID)
}

func (s *CreateSwitchSuite) TestCreateSwitch_Visibility_Preserved() {
	s.mockSwitchRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(sw repository.Switch) bool {
			return sw.Visibility == repository.VisibilityPublic
		})).
		Return(&repository.Switch{UserID: "alice", ID: "generated-id"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"public"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateSwitchSuite) TestCreateSwitch_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"type", `{"brand":"Gateron","name":"Yellow","type":"NotApproved","visibility":"private"}`},
		{"material.top_housing", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"top_housing":"NotApproved"}}`},
		{"material.bottom_housing", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"bottom_housing":"NotApproved"}}`},
		{"material.stem", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"stem":"NotApproved"}}`},
		{"spring.material", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","spring":{"material":"NotApproved"}}`},
		{"purchase.vendor", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","purchase":{"vendor":"NotApproved"}}`},
		{"purchase.order_status", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","purchase":{"order_status":"NotApproved"}}`},
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

func (s *CreateSwitchSuite) TestCreateSwitch_MultipleInvalidFields_NamesAll() {
	// type and material.top_housing are both invalid here - the response
	// must report both via invalid_params, not just the first one checked.
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Gateron","name":"Yellow","type":"NotApproved","visibility":"private",`+
			`"material":{"top_housing":"AlsoNotApproved"}}`)
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
	s.Contains(names, "type")
	s.Contains(names, "material.top_housing")
}

func (s *CreateSwitchSuite) TestCreateSwitch_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_UnapprovedType_Returns400() {
	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Bogus"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_UnapprovedLookupValue_Returns400() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Gateron","name":"Yellow","type":"Linear","purchase":{"vendor":"NotARealVendor"}}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_AlreadyExists_Returns409() {
	s.mockSwitchRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_RepositoryError_Returns500() {
	s.mockSwitchRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_MalformedStoredDate_Returns500NotPanic() {
	malformedDate := "not-a-date"
	s.mockSwitchRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(&repository.Switch{
			ID:         "sw1",
			Visibility: repository.VisibilityPrivate,
			Purchase:   repository.SwitchPurchase{OrderDate: &malformedDate},
		}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type UpdateSwitchSuite struct {
	suite.Suite

	mockSwitchRepo *mocks.MockSwitchRepository
	mockImages     *mocks.MockSwitchImageStore
	handler        http.HandlerFunc
}

func TestUpdateSwitchSuite(t *testing.T) {
	suite.Run(t, new(UpdateSwitchSuite))
}

func (s *UpdateSwitchSuite) SetupTest() {
	s.mockSwitchRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
	s.handler = UpdateSwitch(s.mockSwitchRepo, s.mockImages)
}

func (s *UpdateSwitchSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/users/alice/switches/sw1", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("switchId", "sw1")
	return req
}

func (s *UpdateSwitchSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_Succeeds() {
	s.mockSwitchRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(sw repository.Switch) bool {
			return sw.ID == "sw1" && sw.Brand == "Gateron" && sw.Visibility == repository.VisibilityPrivate
		})).
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Switch
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("sw1", got.ID)
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_Visibility_Preserved() {
	s.mockSwitchRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(sw repository.Switch) bool {
			return sw.Visibility == repository.VisibilityPublic
		})).
		Return(&repository.Switch{UserID: "alice", ID: "sw1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"public"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"type", `{"brand":"Gateron","name":"Yellow","type":"NotApproved","visibility":"private"}`},
		{"material.top_housing", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"top_housing":"NotApproved"}}`},
		{"material.bottom_housing", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"bottom_housing":"NotApproved"}}`},
		{"material.stem", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"stem":"NotApproved"}}`},
		{"spring.material", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","spring":{"material":"NotApproved"}}`},
		{"purchase.vendor", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","purchase":{"vendor":"NotApproved"}}`},
		{"purchase.order_status", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","purchase":{"order_status":"NotApproved"}}`},
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

func (s *UpdateSwitchSuite) TestUpdateSwitch_MultipleInvalidFields_NamesAll() {
	// type and material.top_housing are both invalid here - the response
	// must report both via invalid_params, not just the first one checked.
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Gateron","name":"Yellow","type":"NotApproved","visibility":"private",`+
			`"material":{"top_housing":"AlsoNotApproved"}}`)
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
	s.Contains(names, "type")
	s.Contains(names, "material.top_housing")
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_UnapprovedType_Returns400() {
	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Bogus"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_UnapprovedLookupValue_Returns400() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Gateron","name":"Yellow","type":"Linear","purchase":{"vendor":"NotARealVendor"}}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_NotFound_Returns404() {
	s.mockSwitchRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_RepositoryError_Returns500() {
	s.mockSwitchRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_MalformedStoredDate_Returns500NotPanic() {
	malformedDate := "not-a-date"
	s.mockSwitchRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(&repository.Switch{
			ID:         "sw1",
			Visibility: repository.VisibilityPrivate,
			Purchase:   repository.SwitchPurchase{OrderDate: &malformedDate},
		}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteSwitchSuite struct {
	suite.Suite

	mockSwitches     *mocks.MockSwitchRepository
	mockBuilds       *mocks.MockBuildRepository
	mockBuildImages  *mocks.MockBuildImageStore
	mockSwitchImages *mocks.MockSwitchImageStore
	handler          http.HandlerFunc
}

func TestDeleteSwitchSuite(t *testing.T) {
	suite.Run(t, new(DeleteSwitchSuite))
}

func (s *DeleteSwitchSuite) SetupTest() {
	s.mockSwitches = mocks.NewMockSwitchRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImages = mocks.NewMockBuildImageStore(s.T())
	s.mockSwitchImages = mocks.NewMockSwitchImageStore(s.T())
	s.handler = DeleteSwitch(s.mockSwitches, s.mockBuilds, s.mockBuildImages, s.mockSwitchImages)
}

func (s *DeleteSwitchSuite) newRequest(ctx context.Context, onDelete string) *http.Request {
	target := "/users/alice/switches/sw1"
	if onDelete != "" {
		target += "?on_delete=" + onDelete
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, target, nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("switchId", "sw1")
	return req
}

func (s *DeleteSwitchSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_Owner_DefaultOnDelete_NoReferences_Returns204() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, "alice", "sw1").
		Return(nil, nil)
	s.mockSwitches.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1"}, nil)
	s.mockSwitches.EXPECT().
		Delete(mock.Anything, "sw1").
		Return(nil, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_Owner_Block_Referenced_Returns409WithBlockingBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, "alice", "sw1").
		Return([]string{"build-1", "build-2"}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "block"))

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

	var body struct {
		BlockingBuildIDs []string `json:"blocking_build_ids"`
	}
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&body))
	s.ElementsMatch([]string{"build-1", "build-2"}, body.BlockingBuildIDs)
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_Owner_Detach_Referenced_Returns204_DoesNotCheckReferences() {
	s.mockSwitches.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1"}, nil)
	s.mockSwitches.EXPECT().
		Delete(mock.Anything, "sw1").
		Return(nil, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "detach"))

	s.Equal(http.StatusNoContent, rec.Code)
	// s.mockBuilds has no .EXPECT() - verifies FindBuildsReferencingSwitch
	// was never called in detach mode.
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_Owner_Cascade_Referenced_Returns200WithDeletedBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, "alice", "sw1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(mock.Anything, "alice", "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(mock.Anything, "build-1").
		Return(nil, nil)
	s.mockSwitches.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1"}, nil)
	s.mockSwitches.EXPECT().
		Delete(mock.Anything, "sw1").
		Return(nil, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "cascade"))

	s.Equal(http.StatusOK, rec.Code)

	var body struct {
		DeletedBuildIDs []string `json:"deleted_build_ids"`
	}
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&body))
	s.Equal([]string{"build-1"}, body.DeletedBuildIDs)
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_Owner_InvalidOnDelete_Returns400() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "bogus"))

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, ""))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_Anonymous_Returns404() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), ""))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_RepositoryError_Returns500() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, "alice", "sw1").
		Return(nil, nil)
	s.mockSwitches.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1"}, nil)
	s.mockSwitches.EXPECT().
		Delete(mock.Anything, "sw1").
		Return(nil, errors.New("delete item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_ImageDeleteFails_Returns500_DoesNotDeleteSwitch() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, "alice", "sw1").
		Return(nil, nil)
	key := repository.SwitchImageKey("switches/alice/sw1/image")
	s.mockSwitches.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", ImagePath: &key}, nil)
	s.mockSwitchImages.EXPECT().
		Delete(mock.Anything, key).
		Return(errors.New("s3 unavailable"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
	// mockSwitches has no .EXPECT() for Delete - verifies the DB record
	// was never touched, so a retry can safely re-attempt the S3 delete.
}

type SetSwitchImageSuite struct {
	suite.Suite

	mockRepo   *mocks.MockSwitchRepository
	mockImages *mocks.MockSwitchImageStore
	handler    http.HandlerFunc
}

func TestSetSwitchImageSuite(t *testing.T) {
	suite.Run(t, new(SetSwitchImageSuite))
}

func (s *SetSwitchImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
	s.handler = SetSwitchImage(s.mockRepo, s.mockImages)
}

func (s *SetSwitchImageSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/switches/sw1/image", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("switchId", "sw1")
	return req
}

func (s *SetSwitchImageSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

const setSwitchImageTestKey = repository.SwitchImageKey("switches/alice/sw1/image")

func (s *SetSwitchImageSuite) TestSetSwitchImage_Succeeds() {
	s.mockRepo.EXPECT().
		SetImagePath(mock.Anything, "sw1", setSwitchImageTestKey).
		Return(&repository.Switch{ID: "sw1"}, nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, setSwitchImageTestKey, "image/png").
		Return("https://example.com/presigned-put", nil)

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

func (s *SetSwitchImageSuite) TestSetSwitchImage_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_UnapprovedContentType_Returns400() {
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

func (s *SetSwitchImageSuite) TestSetSwitchImage_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		SetImagePath(mock.Anything, "sw1", setSwitchImageTestKey).
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_PresignError_Returns500() {
	s.mockRepo.EXPECT().
		SetImagePath(mock.Anything, "sw1", setSwitchImageTestKey).
		Return(&repository.Switch{ID: "sw1"}, nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, setSwitchImageTestKey, "image/png").
		Return("", errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		SetImagePath(mock.Anything, "sw1", setSwitchImageTestKey).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_MutationConflict_Returns409() {
	s.mockRepo.EXPECT().
		SetImagePath(mock.Anything, "sw1", setSwitchImageTestKey).
		Return(nil, repository.ErrMutationConflict)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteSwitchImageSuite struct {
	suite.Suite

	mockRepo   *mocks.MockSwitchRepository
	mockImages *mocks.MockSwitchImageStore
	handler    http.HandlerFunc
}

func TestDeleteSwitchImageSuite(t *testing.T) {
	suite.Run(t, new(DeleteSwitchImageSuite))
}

func (s *DeleteSwitchImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
	s.handler = DeleteSwitchImage(s.mockRepo, s.mockImages)
}

func (s *DeleteSwitchImageSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/users/alice/switches/sw1/image", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("switchId", "sw1")
	return req
}

func (s *DeleteSwitchImageSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

var deleteSwitchImageTestKey = repository.SwitchImageKey("switches/alice/sw1/image")

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", ImagePath: &deleteSwitchImageTestKey}, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, deleteSwitchImageTestKey).
		Return(nil)
	s.mockRepo.EXPECT().
		ClearImagePath(mock.Anything, "sw1").
		Return(&deleteSwitchImageTestKey, nil)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_AlreadyAbsent_SucceedsWithoutS3Call() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1"}, nil)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_MutationConflict_Returns409() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", ImagePath: &deleteSwitchImageTestKey}, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, deleteSwitchImageTestKey).
		Return(nil)
	s.mockRepo.EXPECT().
		ClearImagePath(mock.Anything, "sw1").
		Return(nil, repository.ErrMutationConflict)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(nil, errors.New("get item failed"))

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_S3DeleteError_Returns500_DoesNotDeleteDBRecord() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", ImagePath: &deleteSwitchImageTestKey}, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, deleteSwitchImageTestKey).
		Return(errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
	// mockRepo has no .EXPECT() for ClearImagePath - verifies the DB
	// record was never touched, so a retry can safely re-attempt the S3
	// delete.
}
