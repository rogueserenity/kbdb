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

	var got switchListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal([]switchSummary{{ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear"}}, got.Items)
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

	var got switchListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.NextCursor)
	s.Equal("next-page-token", *got.NextCursor)
}

func (s *ListSwitchesSuite) TestListSwitches_InvalidLimit_Returns400() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), "limit=0"))

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *ListSwitchesSuite) TestListSwitches_LimitTooHigh_Returns400() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), "limit=101"))

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *ListSwitchesSuite) TestListSwitches_NonNumericLimit_Returns400() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(context.Background(), "limit=abc"))

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
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
