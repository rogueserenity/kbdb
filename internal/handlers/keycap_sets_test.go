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
