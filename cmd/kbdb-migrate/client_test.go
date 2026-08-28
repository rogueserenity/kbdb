package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ClientSuite struct {
	suite.Suite
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (s *ClientSuite) testClient(baseURL string) *apiClient {
	c, err := newAPIClient(baseURL, unsignedJWT(map[string]any{"sub": "user_test"}))
	s.Require().NoError(err)
	return c
}

func (s *ClientSuite) TestNewAPIClient_ExtractsSubject() {
	c := s.testClient("https://api.example.test")
	s.Equal("user_test", c.subject)
	s.Equal("https://api.example.test", c.baseURL)
}

func (s *ClientSuite) TestUserPath() {
	c := s.testClient("https://api.example.test")
	s.Equal("/v1/users/user_test/keyboards", c.userPath("keyboards"))
	s.Equal("/v1/users/user_test/keyboards/k1", c.userPath("keyboards/k1"))
}

func (s *ClientSuite) TestListAll_FollowsCursorAndStopsOnNull() {
	// Three pages: cursor "" -> items 1,2 next "p2"; "p2" -> 3,4 next "p3";
	// "p3" -> 5 next null.
	p2, p3 := "p2", "p3"
	pages := map[string]listPage{
		"": {
			Items:      rawItems(`{"id":"1"}`, `{"id":"2"}`),
			NextCursor: &p2,
		},
		"p2": {
			Items:      rawItems(`{"id":"3"}`, `{"id":"4"}`),
			NextCursor: &p3,
		},
		"p3": {
			Items:      rawItems(`{"id":"5"}`),
			NextCursor: nil,
		},
	}

	var seenLimits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("/v1/users/user_test/keyboards", r.URL.Path)
		seenLimits = append(seenLimits, r.URL.Query().Get("limit"))
		page, ok := pages[r.URL.Query().Get("cursor")]
		if !ok {
			http.Error(w, "unexpected cursor", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer srv.Close()

	c := s.testClient(srv.URL)
	items, err := c.listAll(context.Background(), c.userPath("keyboards"))
	s.Require().NoError(err)
	s.Len(items, 5)
	for _, l := range seenLimits {
		s.Equal(fmt.Sprint(listPageLimit), l)
	}
}

func (s *ClientSuite) TestDoJSON_Non2xxReturnsAPIError() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"type":"conflict"}`))
	}))
	defer srv.Close()

	c := s.testClient(srv.URL)
	err := c.doJSON(context.Background(), http.MethodPost, "/v1/profile/user_test", map[string]string{"username": "x"}, nil)
	s.Require().Error(err)

	var apiErr *apiError
	s.Require().True(asAPIError(err, &apiErr))
	s.Equal(http.StatusConflict, apiErr.Status)
}

func rawItems(objs ...string) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(objs))
	for _, o := range objs {
		out = append(out, json.RawMessage(o))
	}
	return out
}
