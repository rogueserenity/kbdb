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
	s.verifier = &Verifier{verifier: s.mockVerifier, audience: "expected-audience"}
}

func (s *VerifyTokenSuite) TestValidToken_AudienceMatches_Succeeds() {
	expiry := time.Now().Add(time.Hour)
	s.mockVerifier.EXPECT().
		Verify(mock.Anything, "raw-token").
		Return(&oidc.IDToken{Subject: "user-123", Expiry: expiry, Audience: []string{"expected-audience"}}, nil)

	claims, err := s.verifier.VerifyToken(s.T().Context(), "raw-token")

	s.Require().NoError(err)
	s.Equal("user-123", claims.Subject)
	s.Equal(expiry, claims.Expiry)
}

func (s *VerifyTokenSuite) TestValidToken_AudienceMismatches_Rejected() {
	s.mockVerifier.EXPECT().
		Verify(mock.Anything, "raw-token").
		Return(&oidc.IDToken{Subject: "user-123", Audience: []string{"someone-else"}}, nil)

	claims, err := s.verifier.VerifyToken(s.T().Context(), "raw-token")

	s.Require().Error(err)
	s.Nil(claims)
}

func (s *VerifyTokenSuite) TestInvalidToken_Rejected() {
	s.mockVerifier.EXPECT().
		Verify(mock.Anything, "bad-token").
		Return(nil, errors.New("oidc: token is expired"))

	claims, err := s.verifier.VerifyToken(s.T().Context(), "bad-token")

	s.Require().Error(err)
	s.Nil(claims)
}

// checkAudience is exercised directly (not through VerifyToken) for the
// no-aud/client_id-fallback cases: a real *oidc.IDToken's Claims() only
// works after a genuine Verify() call populates its unexported raw payload,
// so a fakeToken stands in for the parts VerifyToken can't easily fake here.
type fakeToken struct {
	aud           []string
	sub           string
	exp           time.Time
	clientIDValue string
	clientIDErr   error
}

func (f fakeToken) audience() []string        { return f.aud }
func (f fakeToken) subject() string           { return f.sub }
func (f fakeToken) expiry() time.Time         { return f.exp }
func (f fakeToken) clientID() (string, error) { return f.clientIDValue, f.clientIDErr }

func (s *VerifyTokenSuite) TestCheckAudience_AudPresent_MatchesExpected() {
	err := s.verifier.checkAudience(fakeToken{aud: []string{"expected-audience"}})

	s.NoError(err)
}

func (s *VerifyTokenSuite) TestCheckAudience_AudPresent_DoesNotMatch_Rejected() {
	err := s.verifier.checkAudience(fakeToken{aud: []string{"someone-else"}})

	s.Error(err)
}

func (s *VerifyTokenSuite) TestCheckAudience_AudAbsent_ClientIDMatchesExpected() {
	err := s.verifier.checkAudience(fakeToken{clientIDValue: "expected-audience"})

	s.NoError(err)
}

func (s *VerifyTokenSuite) TestCheckAudience_AudAbsent_ClientIDDoesNotMatch_Rejected() {
	err := s.verifier.checkAudience(fakeToken{clientIDValue: "someone-else"})

	s.Error(err)
}

func (s *VerifyTokenSuite) TestCheckAudience_AudAbsent_ClientIDReadFails_Rejected() {
	err := s.verifier.checkAudience(fakeToken{clientIDErr: errors.New("boom")})

	s.Error(err)
}
