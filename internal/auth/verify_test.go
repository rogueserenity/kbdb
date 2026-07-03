package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/auth/mocks"
)

type VerifyTokenSuite struct {
	suite.Suite

	mockVerifier *mocks.MockTokenVerifier
	verifier     *Verifier
}

func TestVerifyTokenSuite(t *testing.T) {
	suite.Run(t, new(VerifyTokenSuite))
}

func (s *VerifyTokenSuite) SetupTest() {
	s.mockVerifier = mocks.NewMockTokenVerifier(s.T())
	s.verifier = &Verifier{verifier: s.mockVerifier}
}

func (s *VerifyTokenSuite) TestValidToken_Succeeds() {
	s.mockVerifier.EXPECT().
		Verify(mock.Anything, "raw-token").
		Return(&oidc.IDToken{Subject: "user-123"}, nil)

	claims, err := s.verifier.VerifyToken(context.Background(), "raw-token")

	s.Require().NoError(err)
	s.Equal("user-123", claims.Subject)
}

func (s *VerifyTokenSuite) TestInvalidToken_Rejected() {
	s.mockVerifier.EXPECT().
		Verify(mock.Anything, "bad-token").
		Return(nil, errors.New("oidc: token is expired"))

	claims, err := s.verifier.VerifyToken(context.Background(), "bad-token")

	s.Error(err)
	s.Nil(claims)
}
