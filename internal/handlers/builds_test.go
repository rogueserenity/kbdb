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

type CreateBuildSuite struct {
	suite.Suite

	mockBuildRepo *mocks.MockBuildRepository
	mockImages    *mocks.MockBuildImageStore
	handler       http.HandlerFunc
}

func TestCreateBuildSuite(t *testing.T) {
	suite.Run(t, new(CreateBuildSuite))
}

func (s *CreateBuildSuite) SetupTest() {
	s.mockBuildRepo = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
	s.handler = CreateBuild(s.mockBuildRepo, s.mockImages)
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
	s.mockBuildRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateBuildSuite) TestCreateBuild_RepositoryError_Returns500() {
	s.mockBuildRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"keyboard":"kb1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}
