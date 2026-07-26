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

	mockRepo *mocks.MockSwitchRepository
	handler  http.HandlerFunc
}

func TestListSwitchesSuite(t *testing.T) {
	suite.Run(t, new(ListSwitchesSuite))
}

func (s *ListSwitchesSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
	s.handler = ListSwitches(s.mockRepo)
}

func (s *ListSwitchesSuite) newRequest(ctx context.Context, query string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/switches?"+query, nil)
	req.SetPathValue("userId", "alice")
	return req
}

func (s *ListSwitchesSuite) TestListSwitches_Owner_RequestsAllVisibilities() {
	ctx := kbdbctx.WithUserID(context.Background(), "alice")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 3
		}), defaultSwitchListLimit, "").
		Return([]repository.Switch{{ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear"}}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, ""))

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
		List(mock.Anything, "alice", []repository.Visibility{repository.VisibilityPublic}, defaultSwitchListLimit, "").
		Return([]repository.Switch{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), ""))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListSwitchesSuite) TestListSwitches_OtherUser_RequestsPublicAndAuthenticated() {
	ctx := kbdbctx.WithUserID(context.Background(), "bob")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 2
		}), defaultSwitchListLimit, "").
		Return([]repository.Switch{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, ""))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListSwitchesSuite) TestListSwitches_PassesLimitAndCursor() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 5, "abc").
		Return([]repository.Switch{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), "limit=5&cursor=abc"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListSwitchesSuite) TestListSwitches_ReturnsNextCursor_WhenPresent() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, defaultSwitchListLimit, "").
		Return([]repository.Switch{}, "next-page-token", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), ""))

	var got api.SwitchListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.NextCursor)
	s.Equal("next-page-token", *got.NextCursor)
}

func (s *ListSwitchesSuite) TestListSwitches_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, defaultSwitchListLimit, "").
		Return(nil, "", errors.New("query failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), ""))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type GetSwitchSuite struct {
	suite.Suite

	mockRepo *mocks.MockSwitchRepository
	handler  http.HandlerFunc
}

func TestGetSwitchSuite(t *testing.T) {
	suite.Run(t, new(GetSwitchSuite))
}

func (s *GetSwitchSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
	s.handler = GetSwitch(s.mockRepo)
}

func (s *GetSwitchSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/switches/sw1", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("id", "sw1")
	return req
}

func (s *GetSwitchSuite) TestGetSwitch_Owner_Succeeds() {
	ctx := kbdbctx.WithUserID(context.Background(), "alice")

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
	s.handler(rec, s.newRequest(context.Background()))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetSwitchSuite) TestGetSwitch_AnonymousReadingPrivateSwitch_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetSwitchSuite) TestGetSwitch_OtherUserReadingAuthenticatedSwitch_Succeeds() {
	ctx := kbdbctx.WithUserID(context.Background(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", Visibility: repository.VisibilityAuthenticated}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetSwitchSuite) TestGetSwitch_OtherUserReadingPrivateSwitch_Returns404() {
	ctx := kbdbctx.WithUserID(context.Background(), "bob")

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
	s.handler(rec, s.newRequest(context.Background()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetSwitchSuite) TestGetSwitch_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(nil, errors.New("get item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type CreateSwitchSuite struct {
	suite.Suite

	mockSwitchRepo *mocks.MockSwitchRepository
	mockLookupRepo *mocks.MockLookupRepository
	handler        http.HandlerFunc
}

func TestCreateSwitchSuite(t *testing.T) {
	suite.Run(t, new(CreateSwitchSuite))
}

func (s *CreateSwitchSuite) SetupTest() {
	s.mockSwitchRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockLookupRepo = mocks.NewMockLookupRepository(s.T())
	s.handler = CreateSwitch(s.mockSwitchRepo, s.mockLookupRepo)
}

func (s *CreateSwitchSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/switches", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	return req
}

func (s *CreateSwitchSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(context.Background(), "alice")
}

// expectValidType mocks switch_type lookup approval for "Linear" - every
// test below that sends type:"Linear" and expects validation to proceed
// past it needs this, since type moved from a hardcoded enum check to the
// switch_type lookup.
func (s *CreateSwitchSuite) expectValidType() {
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "switch_type").
		Return(&repository.Lookup{Category: "switch_type", Values: []any{"Linear"}}, nil)
}

func (s *CreateSwitchSuite) TestCreateSwitch_Succeeds() {
	s.expectValidType()
	s.mockSwitchRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(sw repository.Switch) bool {
			return sw.UserID == "alice" && sw.ID != "" && sw.Brand == "Gateron" &&
				sw.Visibility == repository.VisibilityPrivate
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
	s.expectValidType()
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
		name     string
		category string
		body     string
	}{
		{"type", "switch_type", `{"brand":"Gateron","name":"Yellow","type":"POM"}`},
		{"material.top_housing", "switch_material", `{"brand":"Gateron","name":"Yellow","type":"Linear","material":{"top_housing":"POM"}}`},
		{"material.bottom_housing", "switch_material", `{"brand":"Gateron","name":"Yellow","type":"Linear","material":{"bottom_housing":"POM"}}`},
		{"material.stem", "switch_material", `{"brand":"Gateron","name":"Yellow","type":"Linear","material":{"stem":"POM"}}`},
		{"spring.material", "switch_spring_material", `{"brand":"Gateron","name":"Yellow","type":"Linear","spring":{"material":"POM"}}`},
		{"purchase.vendor", "vendor", `{"brand":"Gateron","name":"Yellow","type":"Linear","purchase":{"vendor":"POM"}}`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			if tt.category != "switch_type" {
				s.expectValidType()
			}
			s.mockLookupRepo.EXPECT().
				GetCategory(mock.Anything, tt.category).
				Return(&repository.Lookup{Category: tt.category, Values: []any{"POM"}}, nil)
			s.mockSwitchRepo.EXPECT().
				Create(mock.Anything, mock.Anything).
				Return(&repository.Switch{UserID: "alice", ID: "generated-id"}, nil)

			req := s.newRequest(s.ownerCtx(), tt.body)
			rec := httptest.NewRecorder()
			s.handler(rec, req)

			s.Equal(http.StatusCreated, rec.Code)
		})
	}
}

func (s *CreateSwitchSuite) TestCreateSwitch_MultipleInvalidFields_NamesAll() {
	// type and material.top_housing are both invalid here - the response
	// must report both via invalid_params, not just the first one checked.
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "switch_type").
		Return(&repository.Lookup{Category: "switch_type", Values: []any{"Linear"}}, nil)
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "switch_material").
		Return(&repository.Lookup{Category: "switch_material", Values: []any{"POM"}}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Gateron","name":"Yellow","type":"NotApproved",`+
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
	ctx := kbdbctx.WithUserID(context.Background(), "bob")

	req := s.newRequest(ctx, `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_Anonymous_Returns404() {
	req := s.newRequest(context.Background(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
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
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "switch_type").
		Return(&repository.Lookup{Category: "switch_type", Values: []any{"Linear"}}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Bogus"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_UnapprovedLookupValue_Returns400() {
	s.expectValidType()
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(&repository.Lookup{Category: "vendor", Values: []any{"Amazon"}}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Gateron","name":"Yellow","type":"Linear","purchase":{"vendor":"NotARealVendor"}}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_NonStringLookupValue_Returns500() {
	s.expectValidType()
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(&repository.Lookup{
			Category: "vendor",
			Values:   []any{map[string]any{"name": "Amazon"}, "CannonKeys"},
		}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Gateron","name":"Yellow","type":"Linear","purchase":{"vendor":"CannonKeys"}}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_LookupCategoryMissing_Returns400() {
	s.expectValidType()
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Gateron","name":"Yellow","type":"Linear","purchase":{"vendor":"Amazon"}}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_LookupRepositoryError_Returns500() {
	s.expectValidType()
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(nil, errors.New("get item failed"))

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Gateron","name":"Yellow","type":"Linear","purchase":{"vendor":"Amazon"}}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_AlreadyExists_Returns409() {
	s.expectValidType()
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
	s.expectValidType()
	s.mockSwitchRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}
