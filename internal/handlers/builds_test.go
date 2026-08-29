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

func floatPtr(f float64) *float64 { return &f }
func intPtr(i int) *int           { return &i }

type CreateBuildSuite struct {
	suite.Suite

	mockBuildRepo      *mocks.MockBuildRepository
	mockImages         *mocks.MockBuildImageStore
	mockKitImages      *mocks.MockKeycapKitImageStore
	mockKeyboardImages *mocks.MockKeyboardImageStore
	mockSwitchImages   *mocks.MockSwitchImageStore
	mockKeyboardRepo   *mocks.MockKeyboardRepository
	mockSwitchRepo     *mocks.MockSwitchRepository
	mockKeycapSetRepo  *mocks.MockKeycapSetRepository
	handler            http.HandlerFunc
}

func TestCreateBuildSuite(t *testing.T) {
	suite.Run(t, new(CreateBuildSuite))
}

func (s *CreateBuildSuite) SetupTest() {
	s.mockBuildRepo = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
	s.mockKitImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.mockKeyboardImages = mocks.NewMockKeyboardImageStore(s.T())
	s.mockSwitchImages = mocks.NewMockSwitchImageStore(s.T())
	s.mockKeyboardRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockSwitchRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockKeycapSetRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.handler = CreateBuild(s.mockBuildRepo, s.mockImages, s.mockKitImages, s.mockKeyboardImages, s.mockSwitchImages, s.mockKeyboardRepo, s.mockSwitchRepo, s.mockKeycapSetRepo)
}

// stubOwnedKeyboard arranges keyboardRepo.Get to report "kb1" as existing
// and owned by alice - the default reference-validation outcome most tests
// below want, so the create path under test can proceed to the behavior
// they're actually asserting on.
func (s *CreateBuildSuite) stubOwnedKeyboard() {
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1"}, nil).
		Maybe()
}

func (s *CreateBuildSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/builds", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	return req
}

func (s *CreateBuildSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

func (s *CreateBuildSuite) TestCreateBuild_Succeeds() {
	s.stubOwnedKeyboard()
	s.mockBuildRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(b repository.Build) bool {
			return b.ID != "" && b.Keyboard == "kb1" && b.Visibility == repository.VisibilityPrivate
		})).
		Return(&repository.Build{UserID: "alice", ID: "generated-id", Keyboard: "kb1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.Build
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("generated-id", got.Id)
}

func (s *CreateBuildSuite) TestCreateBuild_Visibility_Preserved() {
	s.stubOwnedKeyboard()
	s.mockBuildRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(b repository.Build) bool {
			return b.Visibility == repository.VisibilityPublic
		})).
		Return(&repository.Build{UserID: "alice", ID: "generated-id", Keyboard: "kb1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"public"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateBuildSuite) TestCreateBuild_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"stabs.name", `{"keyboard":"kb1","visibility":"private","stabs":{"name":"NotApproved"}}`},
		{"stabs.mount_type", `{"keyboard":"kb1","visibility":"private","stabs":{"mount_type":"NotApproved"}}`},
		{"case_mount_type.type", `{"keyboard":"kb1","visibility":"private","case_mount_type":{"type":"NotApproved"}}`},
		{"case_mount_type.durometer", `{"keyboard":"kb1","visibility":"private","case_mount_type":{"durometer":"NotApproved"}}`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			// Lookup validation runs (and short-circuits) before reference
			// validation, so keyboardRepo is never consulted here - no stub
			// needed/expected.
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

func (s *CreateBuildSuite) TestCreateBuild_MultipleInvalidFields_NamesAll() {
	req := s.newRequest(s.ownerCtx(),
		`{"keyboard":"kb1","visibility":"private",`+
			`"stabs":{"name":"NotApproved"},"case_mount_type":{"type":"AlsoNotApproved"}}`)
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
	s.Contains(names, "stabs.name")
	s.Contains(names, "case_mount_type.type")
}

func (s *CreateBuildSuite) TestCreateBuild_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateBuildSuite) TestCreateBuild_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateBuildSuite) TestCreateBuild_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateBuildSuite) TestCreateBuild_AlreadyExists_Returns409() {
	s.stubOwnedKeyboard()
	s.mockBuildRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateBuildSuite) TestCreateBuild_MutationConflict_Returns409() {
	s.stubOwnedKeyboard()
	s.mockBuildRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateBuildSuite) TestCreateBuild_RepositoryError_Returns500() {
	s.stubOwnedKeyboard()
	s.mockBuildRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateBuildSuite) TestCreateBuild_MissingKeyboard_Returns400() {
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("keyboard", got.InvalidParams[0].Name)
}

func (s *CreateBuildSuite) TestCreateBuild_MissingSwitch_Returns400() {
	s.stubOwnedKeyboard()
	s.mockSwitchRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(),
		`{"keyboard":"kb1","visibility":"private","switches":[{"switch":"sw1","count":4}]}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("switches[0].switch", got.InvalidParams[0].Name)
}

func (s *CreateBuildSuite) TestCreateBuild_MissingKeycapSet_Returns400() {
	s.stubOwnedKeyboard()
	s.mockKeycapSetRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(),
		`{"keyboard":"kb1","visibility":"private","keycap_kits":[{"keycap_set":"ks1","kit":"kit1"}]}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("keycap_kits[0].keycap_set", got.InvalidParams[0].Name)
}

func (s *CreateBuildSuite) TestCreateBuild_KeycapSetFoundButKitMissing_Returns400() {
	s.stubOwnedKeyboard()
	s.mockKeycapSetRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1",
			Kits: map[string]repository.KeycapKit{"other-kit": {KitID: "other-kit"}},
		}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"keyboard":"kb1","visibility":"private","keycap_kits":[{"keycap_set":"ks1","kit":"kit1"}]}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("keycap_kits[0].kit", got.InvalidParams[0].Name)
}

func (s *CreateBuildSuite) TestCreateBuild_ValidReferences_Succeeds() {
	s.stubOwnedKeyboard()
	s.mockSwitchRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1"}, nil)
	s.mockKeycapSetRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1",
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1"}},
		}, nil)
	s.mockBuildRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(&repository.Build{UserID: "alice", ID: "generated-id", Keyboard: "kb1"}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"keyboard":"kb1","visibility":"private",`+
			`"switches":[{"switch":"sw1","count":4}],`+
			`"keycap_kits":[{"keycap_set":"ks1","kit":"kit1"}]}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateBuildSuite) TestCreateBuild_ReferenceCheckRepositoryError_Returns500() {
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(nil, errors.New("dynamo unavailable"))

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type UpdateBuildSuite struct {
	suite.Suite

	mockBuildRepo      *mocks.MockBuildRepository
	mockImages         *mocks.MockBuildImageStore
	mockKitImages      *mocks.MockKeycapKitImageStore
	mockKeyboardImages *mocks.MockKeyboardImageStore
	mockSwitchImages   *mocks.MockSwitchImageStore
	mockKeyboardRepo   *mocks.MockKeyboardRepository
	mockSwitchRepo     *mocks.MockSwitchRepository
	mockKeycapSetRepo  *mocks.MockKeycapSetRepository
	handler            http.HandlerFunc
}

func TestUpdateBuildSuite(t *testing.T) {
	suite.Run(t, new(UpdateBuildSuite))
}

func (s *UpdateBuildSuite) SetupTest() {
	s.mockBuildRepo = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
	s.mockKitImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.mockKeyboardImages = mocks.NewMockKeyboardImageStore(s.T())
	s.mockSwitchImages = mocks.NewMockSwitchImageStore(s.T())
	s.mockKeyboardRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockSwitchRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockKeycapSetRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.handler = UpdateBuild(s.mockBuildRepo, s.mockImages, s.mockKitImages, s.mockKeyboardImages, s.mockSwitchImages, s.mockKeyboardRepo, s.mockSwitchRepo, s.mockKeycapSetRepo)
}

func (s *UpdateBuildSuite) stubOwnedKeyboard() {
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1"}, nil).
		Maybe()
}

func (s *UpdateBuildSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/users/alice/builds/b1", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("buildId", "b1")
	return req
}

func (s *UpdateBuildSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

func (s *UpdateBuildSuite) TestUpdateBuild_Succeeds() {
	s.stubOwnedKeyboard()
	s.mockBuildRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(b repository.Build) bool {
			return b.ID == "b1" && b.Keyboard == "kb1" && b.Visibility == repository.VisibilityPrivate
		})).
		Return(&repository.Build{UserID: "alice", ID: "b1", Keyboard: "kb1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.Build
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("b1", got.Id)
}

func (s *UpdateBuildSuite) TestUpdateBuild_Visibility_Preserved() {
	s.stubOwnedKeyboard()
	s.mockBuildRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(b repository.Build) bool {
			return b.Visibility == repository.VisibilityPublic
		})).
		Return(&repository.Build{UserID: "alice", ID: "b1", Keyboard: "kb1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"public"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *UpdateBuildSuite) TestUpdateBuild_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"stabs.name", `{"keyboard":"kb1","visibility":"private","stabs":{"name":"NotApproved"}}`},
		{"case_mount_type.type", `{"keyboard":"kb1","visibility":"private","case_mount_type":{"type":"NotApproved"}}`},
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

func (s *UpdateBuildSuite) TestUpdateBuild_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateBuildSuite) TestUpdateBuild_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateBuildSuite) TestUpdateBuild_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateBuildSuite) TestUpdateBuild_MissingKeyboard_Returns400() {
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("keyboard", got.InvalidParams[0].Name)
}

func (s *UpdateBuildSuite) TestUpdateBuild_NotFound_Returns404() {
	s.stubOwnedKeyboard()
	s.mockBuildRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateBuildSuite) TestUpdateBuild_MutationConflict_Returns409() {
	s.stubOwnedKeyboard()
	s.mockBuildRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateBuildSuite) TestUpdateBuild_RepositoryError_Returns500() {
	s.stubOwnedKeyboard()
	s.mockBuildRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type ListBuildsSuite struct {
	suite.Suite

	mockBuildRepo     *mocks.MockBuildRepository
	mockKeyboardRepo  *mocks.MockKeyboardRepository
	mockSwitchRepo    *mocks.MockSwitchRepository
	mockKeycapSetRepo *mocks.MockKeycapSetRepository
	mockImages        *mocks.MockBuildImageStore
	handler           http.HandlerFunc
}

func TestListBuildsSuite(t *testing.T) {
	suite.Run(t, new(ListBuildsSuite))
}

func (s *ListBuildsSuite) SetupTest() {
	s.mockBuildRepo = mocks.NewMockBuildRepository(s.T())
	s.mockKeyboardRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockSwitchRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockKeycapSetRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
	s.handler = ListBuilds(s.mockBuildRepo, s.mockKeyboardRepo, s.mockSwitchRepo, s.mockKeycapSetRepo, s.mockImages)
}

func (s *ListBuildsSuite) newRequest(ctx context.Context, query string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/builds?"+query, nil)
	req.SetPathValue("userId", "alice")
	return req
}

func (s *ListBuildsSuite) TestListBuilds_Empty_ReturnsEmptyItems() {
	s.mockBuildRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Build{}, "", nil)

	req := s.newRequest(kbdbctx.WithUserID(s.T().Context(), "alice"), "limit=20")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var got api.BuildListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Empty(*got.Items)
}

func (s *ListBuildsSuite) TestListBuilds_SingleBuild_ResolvableKeyboard_DenormalizesBrandName() {
	s.mockBuildRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Build{{UserID: "alice", ID: "build1", Keyboard: "kb1", Visibility: repository.VisibilityPublic}}, "", nil)
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)

	req := s.newRequest(s.T().Context(), "limit=20")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var got api.BuildListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	item := (*got.Items)[0]
	s.Require().NotNil(item.Keyboard)
	s.Require().NotNil(item.Keyboard.Brand)
	s.Equal("Keychron", *item.Keyboard.Brand)
	s.Require().NotNil(item.Keyboard.Name)
	s.Equal("Q1", *item.Keyboard.Name)
}

func (s *ListBuildsSuite) TestListBuilds_Owner_IncludesTotalCost() {
	s.mockBuildRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Build{{
			UserID: "alice", ID: "build1", Keyboard: "kb1", Visibility: repository.VisibilityPrivate,
			Switches:   []repository.BuildSwitchEntry{{Switch: "sw1", Count: 70}},
			KeycapKits: []repository.BuildKeycapKitEntry{{KeycapSet: "ks1", Kit: "kit1"}},
			Stabs:      &repository.BuildStabs{Price: floatPtr(12.5)},
		}}, "", nil)
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1",
			Purchase: repository.KeyboardPurchase{Price: floatPtr(200)},
		}, nil)
	s.mockSwitchRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear",
			Purchase: repository.SwitchPurchase{Price: floatPtr(45), Quantity: intPtr(90)},
		}, nil)
	s.mockKeycapSetRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: map[string]repository.KeycapKit{"kit1": {
				KitID: "kit1", Name: "Base",
				Purchase: repository.KeycapKitPurchase{Price: floatPtr(150)},
			}},
		}, nil)

	req := s.newRequest(kbdbctx.WithUserID(s.T().Context(), "alice"), "limit=20")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var got api.BuildListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	item := (*got.Items)[0]
	s.Require().NotNil(item.TotalCost)
	s.InDelta(200+35+150+12.5, *item.TotalCost, 0.0001)
}

func (s *ListBuildsSuite) TestListBuilds_NonOwner_OmitsTotalCost() {
	s.mockBuildRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Build{{
			UserID: "alice", ID: "build1", Keyboard: "kb1", Visibility: repository.VisibilityPublic,
			Switches:   []repository.BuildSwitchEntry{{Switch: "sw1", Count: 70}},
			KeycapKits: []repository.BuildKeycapKitEntry{{KeycapSet: "ks1", Kit: "kit1"}},
			Stabs:      &repository.BuildStabs{Price: floatPtr(12.5)},
		}}, "", nil)
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1",
			Purchase: repository.KeyboardPurchase{Price: floatPtr(200)},
		}, nil)

	req := s.newRequest(kbdbctx.WithUserID(s.T().Context(), "bob"), "limit=20")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var got api.BuildListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	s.Nil((*got.Items)[0].TotalCost)
}

func (s *ListBuildsSuite) TestListBuilds_BuildWithKeyboardThatNotFound_OmitsKeyboardStillReturns200() {
	s.mockBuildRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Build{{UserID: "alice", ID: "build1", Keyboard: "deleted-kb", Visibility: repository.VisibilityPublic}}, "", nil)
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "deleted-kb").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.T().Context(), "limit=20")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var got api.BuildListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	s.Nil((*got.Items)[0].Keyboard)
}

func (s *ListBuildsSuite) TestListBuilds_KeyboardRepositoryError_Returns500() {
	s.mockBuildRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Build{{UserID: "alice", ID: "build1", Keyboard: "kb1", Visibility: repository.VisibilityPublic}}, "", nil)
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(nil, errors.New("dynamo unavailable"))

	req := s.newRequest(s.T().Context(), "limit=20")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *ListBuildsSuite) TestListBuilds_PassesLimitAndCursor() {
	s.mockBuildRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 5, "abc").
		Return([]repository.Build{}, "", nil)

	req := s.newRequest(s.T().Context(), "limit=5&cursor=abc")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListBuildsSuite) TestListBuilds_ReturnsNextCursor_WhenPresent() {
	s.mockBuildRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Build{}, "next-page-token", nil)

	req := s.newRequest(s.T().Context(), "limit=20")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var got api.BuildListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.NextCursor)
	s.Equal("next-page-token", *got.NextCursor)
}

func (s *ListBuildsSuite) TestListBuilds_Anonymous_RequestsPublicOnly() {
	s.mockBuildRepo.EXPECT().
		List(mock.Anything, "alice", []repository.Visibility{repository.VisibilityPublic}, 20, "").
		Return([]repository.Build{}, "", nil)

	req := s.newRequest(s.T().Context(), "limit=20")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListBuildsSuite) TestListBuilds_OtherUser_RequestsPublicAndAuthenticated() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	s.mockBuildRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 2
		}), 20, "").
		Return([]repository.Build{}, "", nil)

	req := s.newRequest(ctx, "limit=20")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListBuildsSuite) TestListBuilds_RepositoryError_Returns500() {
	s.mockBuildRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return(nil, "", errors.New("query failed"))

	req := s.newRequest(s.T().Context(), "limit=20")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type GetBuildSuite struct {
	suite.Suite

	mockBuildRepo      *mocks.MockBuildRepository
	mockImages         *mocks.MockBuildImageStore
	mockKitImages      *mocks.MockKeycapKitImageStore
	mockKeyboardImages *mocks.MockKeyboardImageStore
	mockSwitchImages   *mocks.MockSwitchImageStore
	mockKeyboardRepo   *mocks.MockKeyboardRepository
	mockSwitchRepo     *mocks.MockSwitchRepository
	mockKeycapSetRepo  *mocks.MockKeycapSetRepository
	handler            http.HandlerFunc
}

func TestGetBuildSuite(t *testing.T) {
	suite.Run(t, new(GetBuildSuite))
}

func (s *GetBuildSuite) SetupTest() {
	s.mockBuildRepo = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
	s.mockKitImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.mockKeyboardImages = mocks.NewMockKeyboardImageStore(s.T())
	s.mockSwitchImages = mocks.NewMockSwitchImageStore(s.T())
	s.mockKeyboardRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockSwitchRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockKeycapSetRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.handler = GetBuild(s.mockBuildRepo, s.mockImages, s.mockKitImages, s.mockKeyboardImages, s.mockSwitchImages, s.mockKeyboardRepo, s.mockSwitchRepo, s.mockKeycapSetRepo)
}

// stubOwnedKeyboard arranges keyboardRepo.Get to report "kb1" as existing -
// the default BuildToAPI keyboard-resolution outcome most tests below want,
// so the get path under test can proceed to the behavior they're actually
// asserting on.
func (s *GetBuildSuite) stubOwnedKeyboard() {
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", mock.Anything).
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil).
		Maybe()
}

func (s *GetBuildSuite) newRequest(ctx context.Context, buildID string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/builds/"+buildID, nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("buildId", buildID)
	return req
}

func (s *GetBuildSuite) TestGetBuild_Found_ReturnsBuild() {
	s.stubOwnedKeyboard()
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(&repository.Build{UserID: "alice", ID: "build1", Keyboard: "kb1", Visibility: repository.VisibilityPublic}, nil)

	req := s.newRequest(s.T().Context(), "build1")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.Build
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("build1", got.Id)
}

func (s *GetBuildSuite) TestGetBuild_NotFound_Returns404() {
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "no-such-build").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.T().Context(), "no-such-build")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetBuildSuite) TestGetBuild_NotOwnedAndNotShared_Returns404() {
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(&repository.Build{UserID: "alice", ID: "build1", Visibility: repository.VisibilityPrivate}, nil)

	req := s.newRequest(s.T().Context(), "build1")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetBuildSuite) TestGetBuild_SharedVisibility_ReturnsBuild() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")
	s.stubOwnedKeyboard()
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(&repository.Build{UserID: "alice", ID: "build1", Visibility: repository.VisibilityAuthenticated}, nil)

	req := s.newRequest(ctx, "build1")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var got api.Build
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("build1", got.Id)
}

func (s *GetBuildSuite) TestGetBuild_RepositoryError_Returns500() {
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(nil, errors.New("dynamo unavailable"))

	req := s.newRequest(s.T().Context(), "build1")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteBuildSuite struct {
	suite.Suite

	mockBuildRepo *mocks.MockBuildRepository
	mockImages    *mocks.MockBuildImageStore
	handler       http.HandlerFunc
}

func TestDeleteBuildSuite(t *testing.T) {
	suite.Run(t, new(DeleteBuildSuite))
}

func (s *DeleteBuildSuite) SetupTest() {
	s.mockBuildRepo = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
	s.handler = DeleteBuild(s.mockBuildRepo, s.mockImages)
}

func (s *DeleteBuildSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/users/alice/builds/build1", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("buildId", "build1")
	return req
}

func (s *DeleteBuildSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

func (s *DeleteBuildSuite) TestDeleteBuild_Owner_Succeeds() {
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(&repository.Build{ID: "build1"}, nil)
	s.mockBuildRepo.EXPECT().
		Delete(mock.Anything, "build1").
		Return(nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteBuildSuite) TestDeleteBuild_ImagesPresent_DeletesEachFromS3BeforeDB() {
	key1 := repository.BuildImageKey("builds/alice/build1/images/img1")
	key2 := repository.BuildImageKey("builds/alice/build1/images/img2")
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(&repository.Build{ID: "build1", Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img1", Path: key1},
			{ImageID: "img2", Path: key2},
		})}, nil)
	s.mockImages.EXPECT().
		DeleteBuildImage(mock.Anything, key1).
		Return(nil)
	s.mockImages.EXPECT().
		DeleteBuildImage(mock.Anything, key2).
		Return(nil)
	s.mockBuildRepo.EXPECT().
		Delete(mock.Anything, "build1").
		Return(nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteBuildSuite) TestDeleteBuild_ImageDeleteFails_Returns500_DoesNotDeleteBuild() {
	key1 := repository.BuildImageKey("builds/alice/build1/images/img1")
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(&repository.Build{ID: "build1", Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img1", Path: key1},
		})}, nil)
	s.mockImages.EXPECT().
		DeleteBuildImage(mock.Anything, key1).
		Return(errors.New("s3 unavailable"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
	// mockBuildRepo has no .EXPECT() for Delete - verifies the DB record
	// was never touched, so a retry can safely re-attempt the S3 delete.
}

func (s *DeleteBuildSuite) TestDeleteBuild_RepositoryNotFound_StillReturns204() {
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteBuildSuite) TestDeleteBuild_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteBuildSuite) TestDeleteBuild_Anonymous_Returns404() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteBuildSuite) TestDeleteBuild_RepositoryError_Returns500() {
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(&repository.Build{ID: "build1"}, nil)
	s.mockBuildRepo.EXPECT().
		Delete(mock.Anything, "build1").
		Return(errors.New("delete item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type AddBuildImageSuite struct {
	suite.Suite

	mockBuildRepo *mocks.MockBuildRepository
	mockImages    *mocks.MockBuildImageStore
	handler       http.HandlerFunc
}

func TestAddBuildImageSuite(t *testing.T) {
	suite.Run(t, new(AddBuildImageSuite))
}

func (s *AddBuildImageSuite) SetupTest() {
	s.mockBuildRepo = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
	s.handler = AddBuildImage(s.mockBuildRepo, s.mockImages)
}

func (s *AddBuildImageSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/builds/build1/images", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("buildId", "build1")
	return req
}

func (s *AddBuildImageSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

func (s *AddBuildImageSuite) TestAddBuildImage_Succeeds() {
	s.mockBuildRepo.EXPECT().
		AddImage(mock.Anything, "build1", mock.MatchedBy(func(img repository.BuildImage) bool {
			return img.ImageID != ""
		})).
		Return(nil)
	s.mockImages.EXPECT().
		PresignPutBuildImage(mock.Anything, mock.Anything, "image/png").
		Return("https://example.com/presigned-put", nil)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got struct {
		ImageID   string `json:"image_id"`
		UploadURL string `json:"upload_url"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.NotEmpty(got.ImageID)
	s.Equal("https://example.com/presigned-put", got.UploadURL)
}

func (s *AddBuildImageSuite) TestAddBuildImage_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *AddBuildImageSuite) TestAddBuildImage_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *AddBuildImageSuite) TestAddBuildImage_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *AddBuildImageSuite) TestAddBuildImage_UnapprovedContentType_Returns400() {
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

func (s *AddBuildImageSuite) TestAddBuildImage_NotFound_Returns404() {
	s.mockBuildRepo.EXPECT().
		AddImage(mock.Anything, "build1", mock.Anything).
		Return(repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *AddBuildImageSuite) TestAddBuildImage_PresignError_Returns500() {
	s.mockBuildRepo.EXPECT().
		AddImage(mock.Anything, "build1", mock.Anything).
		Return(nil)
	s.mockImages.EXPECT().
		PresignPutBuildImage(mock.Anything, mock.Anything, "image/png").
		Return("", errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *AddBuildImageSuite) TestAddBuildImage_RepositoryError_Returns500() {
	s.mockBuildRepo.EXPECT().
		AddImage(mock.Anything, "build1", mock.Anything).
		Return(errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *AddBuildImageSuite) TestAddBuildImage_MutationConflict_Returns409() {
	s.mockBuildRepo.EXPECT().
		AddImage(mock.Anything, "build1", mock.Anything).
		Return(repository.ErrMutationConflict)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/png"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteBuildImageSuite struct {
	suite.Suite

	mockBuildRepo *mocks.MockBuildRepository
	mockImages    *mocks.MockBuildImageStore
	handler       http.HandlerFunc
}

func TestDeleteBuildImageSuite(t *testing.T) {
	suite.Run(t, new(DeleteBuildImageSuite))
}

func (s *DeleteBuildImageSuite) SetupTest() {
	s.mockBuildRepo = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
	s.handler = DeleteBuildImage(s.mockBuildRepo, s.mockImages)
}

func (s *DeleteBuildImageSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/users/alice/builds/build1/images/img1", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("buildId", "build1")
	req.SetPathValue("imageId", "img1")
	return req
}

func (s *DeleteBuildImageSuite) ownerCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "alice")
}

var deleteBuildImageTestKey = repository.BuildImageKey("builds/alice/build1/images/img1")

func (s *DeleteBuildImageSuite) TestDeleteBuildImage_Succeeds() {
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(&repository.Build{ID: "build1", Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img1", Path: deleteBuildImageTestKey},
		})}, nil)
	s.mockImages.EXPECT().
		DeleteBuildImage(mock.Anything, deleteBuildImageTestKey).
		Return(nil)
	s.mockBuildRepo.EXPECT().
		DeleteImage(mock.Anything, "build1", "img1").
		Return(&deleteBuildImageTestKey, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteBuildImageSuite) TestDeleteBuildImage_DeleteImageNotFound_Returns204() {
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(&repository.Build{ID: "build1", Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img1", Path: deleteBuildImageTestKey},
		})}, nil)
	s.mockImages.EXPECT().
		DeleteBuildImage(mock.Anything, deleteBuildImageTestKey).
		Return(nil)
	s.mockBuildRepo.EXPECT().
		DeleteImage(mock.Anything, "build1", "img1").
		Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteBuildImageSuite) TestDeleteBuildImage_AlreadyAbsent_SucceedsWithoutS3Call() {
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(&repository.Build{ID: "build1"}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteBuildImageSuite) TestDeleteBuildImage_NotOwner_Returns404() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "bob")

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteBuildImageSuite) TestDeleteBuildImage_Anonymous_Returns404() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteBuildImageSuite) TestDeleteBuildImage_NotFound_Returns404() {
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteBuildImageSuite) TestDeleteBuildImage_MutationConflict_Returns409() {
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(&repository.Build{ID: "build1", Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img1", Path: deleteBuildImageTestKey},
		})}, nil)
	s.mockImages.EXPECT().
		DeleteBuildImage(mock.Anything, deleteBuildImageTestKey).
		Return(nil)
	s.mockBuildRepo.EXPECT().
		DeleteImage(mock.Anything, "build1", "img1").
		Return(nil, repository.ErrMutationConflict)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteBuildImageSuite) TestDeleteBuildImage_RepositoryError_Returns500() {
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(nil, errors.New("get item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteBuildImageSuite) TestDeleteBuildImage_S3DeleteError_Returns500_DoesNotDeleteDBRecord() {
	s.mockBuildRepo.EXPECT().
		Get(mock.Anything, "alice", "build1").
		Return(&repository.Build{ID: "build1", Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img1", Path: deleteBuildImageTestKey},
		})}, nil)
	s.mockImages.EXPECT().
		DeleteBuildImage(mock.Anything, deleteBuildImageTestKey).
		Return(errors.New("s3: access denied"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
	// mockBuildRepo has no .EXPECT() for DeleteImage - verifies the DB
	// record was never touched.
}
