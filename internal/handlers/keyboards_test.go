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

type ListKeyboardsSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeyboardRepository
	handler  http.HandlerFunc
}

func TestListKeyboardsSuite(t *testing.T) {
	suite.Run(t, new(ListKeyboardsSuite))
}

func (s *ListKeyboardsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeyboardRepository(s.T())
	s.handler = ListKeyboards(s.mockRepo)
}

func (s *ListKeyboardsSuite) newRequest(ctx context.Context, query string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/keyboards?"+query, nil)
	req.SetPathValue("userId", "alice")
	return req
}

func (s *ListKeyboardsSuite) TestListKeyboards_Owner_RequestsAllVisibilities() {
	ctx := kbdbctx.WithUserID(context.Background(), "alice")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 3
		}), 20, "").
		Return([]repository.Keyboard{{ID: "kb1", Brand: "Keychron", Name: "Q1"}}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.KeyboardListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	id, brand, name := "kb1", "Keychron", "Q1"
	s.Equal(&[]api.KeyboardSummary{{Id: &id, Brand: &brand, Name: &name}}, got.Items)
	s.Nil(got.NextCursor)
}

func (s *ListKeyboardsSuite) TestListKeyboards_Anonymous_RequestsPublicOnly() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", []repository.Visibility{repository.VisibilityPublic}, 20, "").
		Return([]repository.Keyboard{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeyboardsSuite) TestListKeyboards_OtherUser_RequestsPublicAndAuthenticated() {
	ctx := kbdbctx.WithUserID(context.Background(), "bob")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 2
		}), 20, "").
		Return([]repository.Keyboard{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeyboardsSuite) TestListKeyboards_PassesLimitAndCursor() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 5, "abc").
		Return([]repository.Keyboard{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), "limit=5&cursor=abc"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeyboardsSuite) TestListKeyboards_ReturnsNextCursor_WhenPresent() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Keyboard{}, "next-page-token", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), "limit=20"))

	var got api.KeyboardListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.NextCursor)
	s.Equal("next-page-token", *got.NextCursor)
}

func (s *ListKeyboardsSuite) TestListKeyboards_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return(nil, "", errors.New("query failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), "limit=20"))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type GetKeyboardSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeyboardRepository
	handler  http.HandlerFunc
}

func TestGetKeyboardSuite(t *testing.T) {
	suite.Run(t, new(GetKeyboardSuite))
}

func (s *GetKeyboardSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeyboardRepository(s.T())
	s.handler = GetKeyboard(s.mockRepo)
}

func (s *GetKeyboardSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/keyboards/kb1", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("id", "kb1")
	return req
}

func (s *GetKeyboardSuite) TestGetKeyboard_Owner_Succeeds() {
	ctx := kbdbctx.WithUserID(context.Background(), "alice")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Brand: "Keychron", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Keyboard
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("kb1", got.ID)
	s.Equal("Keychron", got.Brand)
}

func (s *GetKeyboardSuite) TestGetKeyboard_AnonymousReadingPublicKeyboard_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Visibility: repository.VisibilityPublic}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background()))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetKeyboardSuite) TestGetKeyboard_AnonymousReadingPrivateKeyboard_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeyboardSuite) TestGetKeyboard_OtherUserReadingAuthenticatedKeyboard_Succeeds() {
	ctx := kbdbctx.WithUserID(context.Background(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Visibility: repository.VisibilityAuthenticated}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetKeyboardSuite) TestGetKeyboard_OtherUserReadingPrivateKeyboard_Returns404() {
	ctx := kbdbctx.WithUserID(context.Background(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeyboardSuite) TestGetKeyboard_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeyboardSuite) TestGetKeyboard_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(nil, errors.New("get item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type CreateKeyboardSuite struct {
	suite.Suite

	mockKeyboardRepo *mocks.MockKeyboardRepository
	mockLookupRepo   *mocks.MockLookupRepository
	handler          http.HandlerFunc
}

func TestCreateKeyboardSuite(t *testing.T) {
	suite.Run(t, new(CreateKeyboardSuite))
}

func (s *CreateKeyboardSuite) SetupTest() {
	s.mockKeyboardRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockLookupRepo = mocks.NewMockLookupRepository(s.T())
	s.handler = CreateKeyboard(s.mockKeyboardRepo, s.mockLookupRepo)
}

func (s *CreateKeyboardSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/keyboards", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	return req
}

func (s *CreateKeyboardSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(context.Background(), "alice")
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_Succeeds() {
	s.mockKeyboardRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(kb repository.Keyboard) bool {
			return kb.ID != "" && kb.Brand == "Keychron" && kb.Visibility == repository.VisibilityPrivate
		})).
		Return(&repository.Keyboard{UserID: "alice", ID: "generated-id", Brand: "Keychron", Name: "Q1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Keyboard
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("generated-id", got.ID)
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_Visibility_Preserved() {
	s.mockKeyboardRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(kb repository.Keyboard) bool {
			return kb.Visibility == repository.VisibilityPublic
		})).
		Return(&repository.Keyboard{UserID: "alice", ID: "generated-id"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"public"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name     string
		category string
		body     string
	}{
		{"size", "keyboard_size", `{"brand":"Keychron","name":"Q1","visibility":"private","size":"NotApproved"}`},
		{"design.top_case.material", "keyboard_case_material", `{"brand":"Keychron","name":"Q1","visibility":"private","design":{"top_case":{"material":"NotApproved"}}}`},
		{"design.bottom_case.material", "keyboard_case_material", `{"brand":"Keychron","name":"Q1","visibility":"private","design":{"bottom_case":{"material":"NotApproved"}}}`},
		{"design.weight.material", "keyboard_weight_material", `{"brand":"Keychron","name":"Q1","visibility":"private","design":{"weight":{"material":"NotApproved"}}}`},
		{"pcb.firmware", "keyboard_pcb_firmware", `{"brand":"Keychron","name":"Q1","visibility":"private","pcb":{"firmware":"NotApproved"}}`},
		{"pcb.assembly", "keyboard_pcb_assembly_type", `{"brand":"Keychron","name":"Q1","visibility":"private","pcb":{"assembly":"NotApproved"}}`},
		{"pcb.connectivity", "keyboard_pcb_connectivity_type", `{"brand":"Keychron","name":"Q1","visibility":"private","pcb":{"connectivity":"NotApproved"}}`},
		{"purchase.vendor", "vendor", `{"brand":"Keychron","name":"Q1","visibility":"private","purchase":{"vendor":"NotApproved"}}`},
		{"purchase.order_status", "order_status", `{"brand":"Keychron","name":"Q1","visibility":"private","purchase":{"order_status":"NotApproved"}}`},
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
		})
	}
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_ValidatesPlateMaterials() {
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_plate_material").
		Return(&repository.Lookup{Category: "keyboard_plate_material", Values: []any{"FR4"}}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","design":{"plates":["NotApproved"]}}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("design.plates[0]", got.InvalidParams[0].Name)
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_LayoutValidForSize_Succeeds() {
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_size").
		Return(&repository.Lookup{Category: "keyboard_size", Values: []any{"60%", "65%"}}, nil)
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_layout").
		Return(&repository.Lookup{
			Category: "keyboard_layout",
			Values:   []any{map[string]any{"name": "WK", "sizes": []any{"60%", "65%"}}},
		}, nil)
	s.mockKeyboardRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(&repository.Keyboard{UserID: "alice", ID: "generated-id"}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","size":"60%","layout":"WK"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_LayoutInvalidForSize_Returns400() {
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_size").
		Return(&repository.Lookup{Category: "keyboard_size", Values: []any{"40%", "60%", "65%"}}, nil)
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_layout").
		Return(&repository.Lookup{
			Category: "keyboard_layout",
			Values:   []any{map[string]any{"name": "WK", "sizes": []any{"60%", "65%"}}},
		}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","size":"40%","layout":"WK"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("layout", got.InvalidParams[0].Name)
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_UnrecognizedLayout_Returns400() {
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_layout").
		Return(&repository.Lookup{
			Category: "keyboard_layout",
			Values:   []any{map[string]any{"name": "WK", "sizes": []any{"60%"}}},
		}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","layout":"NotALayout"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_LayoutWithoutSize_SkipsSizeCheck() {
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_layout").
		Return(&repository.Lookup{
			Category: "keyboard_layout",
			Values:   []any{map[string]any{"name": "WK", "sizes": []any{"60%"}}},
		}, nil)
	s.mockKeyboardRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(&repository.Keyboard{UserID: "alice", ID: "generated-id"}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","layout":"WK"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_LayoutCategoryMissing_Returns400() {
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_layout").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","layout":"WK"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_LayoutLookupRepositoryError_Returns500() {
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_layout").
		Return(nil, errors.New("get item failed"))

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","layout":"WK"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_MultipleInvalidFields_NamesAll() {
	// size and pcb.firmware are both invalid here - the response must
	// report both via invalid_params, not just the first one checked.
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_size").
		Return(&repository.Lookup{Category: "keyboard_size", Values: []any{"60%"}}, nil)
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_pcb_firmware").
		Return(&repository.Lookup{Category: "keyboard_pcb_firmware", Values: []any{"QMK/VIA"}}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","size":"NotApproved",`+
			`"pcb":{"firmware":"AlsoNotApproved"}}`)
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
	s.Contains(names, "size")
	s.Contains(names, "pcb.firmware")
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(context.Background(), "bob")

	req := s.newRequest(ctx, `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_Anonymous_Returns404() {
	req := s.newRequest(context.Background(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_NonStringLookupValue_Returns500() {
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(&repository.Lookup{
			Category: "vendor",
			Values:   []any{map[string]any{"name": "Amazon"}, "CannonKeys"},
		}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","purchase":{"vendor":"CannonKeys"}}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_LookupCategoryMissing_Returns400() {
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","purchase":{"vendor":"Amazon"}}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_LookupRepositoryError_Returns500() {
	s.mockLookupRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(nil, errors.New("get item failed"))

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","purchase":{"vendor":"Amazon"}}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_AlreadyExists_Returns409() {
	s.mockKeyboardRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_RepositoryError_Returns500() {
	s.mockKeyboardRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}
