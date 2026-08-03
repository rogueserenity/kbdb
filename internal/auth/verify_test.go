package auth

import (
	"errors"
	"testing"
	"time"

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
	expiry := time.Now().Add(time.Hour)
	s.mockVerifier.EXPECT().
		Verify(mock.Anything, "raw-token").
		Return(&oidc.IDToken{Subject: "user-123", Expiry: expiry}, nil)

	claims, err := s.verifier.VerifyToken(s.T().Context(), "raw-token")

	s.Require().NoError(err)
	s.Equal("user-123", claims.Subject)
	s.Equal(expiry, claims.Expiry)
}

func (s *VerifyTokenSuite) TestInvalidToken_Rejected() {
	s.mockVerifier.EXPECT().
		Verify(mock.Anything, "bad-token").
		Return(nil, errors.New("oidc: token is expired"))

	claims, err := s.verifier.VerifyToken(s.T().Context(), "bad-token")

	s.Require().Error(err)
	s.Nil(claims)
}
