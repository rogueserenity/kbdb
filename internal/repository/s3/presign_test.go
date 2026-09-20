package s3

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/repository/s3/mocks"
)

type PresignGetSuite struct {
	suite.Suite

	mockPresign *mocks.MockS3PresignAPI
}

func TestPresignGetSuite(t *testing.T) {
	suite.Run(t, new(PresignGetSuite))
}

func (s *PresignGetSuite) SetupTest() {
	s.mockPresign = mocks.NewMockS3PresignAPI(s.T())
}

type capturedPresign struct {
	opts  awss3.PresignOptions
	inner awss3.Options
}

func (s *PresignGetSuite) expectPresign() *capturedPresign {
	c := &capturedPresign{}
	s.mockPresign.EXPECT().
		PresignGetObject(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ *awss3.GetObjectInput, optFns ...func(*awss3.PresignOptions)) {
			for _, fn := range optFns {
				fn(&c.opts)
			}
			for _, fn := range c.opts.ClientOptions {
				fn(&c.inner)
			}
		}).
		Return(&v4.PresignedHTTPRequest{URL: "https://example.com/signed"}, nil)

	return c
}

func (s *PresignGetSuite) TestExpiringCredentials_TTLTracksCredentialWindow() {
	got := s.expectPresign()

	before := time.Now()
	u, expiresAt, err := presignGet(s.T().Context(), expiringCredentials(time.Hour), s.mockPresign, "bkt", "k")

	s.Require().NoError(err)
	s.Equal("https://example.com/signed", u)
	s.Equal(time.Hour-presignMargin, got.opts.Expires)
	s.WithinDuration(before.Add(time.Hour-presignMargin), expiresAt, 2*time.Second)
}

func (s *PresignGetSuite) TestExpiringCredentials_ReportedExpiryNeverOutlivesCredentials() {
	for _, window := range []time.Duration{
		12 * time.Hour, time.Hour, 5 * time.Minute, 90 * time.Second,
		time.Minute, 30 * time.Second, 2 * time.Second,
	} {
		s.Run(window.String(), func() {
			s.SetupTest()
			creds := expiringCredentials(window)
			got := s.expectPresign()

			_, expiresAt, err := presignGet(s.T().Context(), creds, s.mockPresign, "bkt", "k")

			s.Require().NoError(err)
			s.False(expiresAt.After(creds.creds.Expires),
				"reported expiry %s outlives credentials expiring %s", expiresAt, creds.creds.Expires)
			s.Positive(got.opts.Expires, "a valid credential window must still produce a signable TTL")
			s.LessOrEqual(got.opts.Expires, window,
				"X-Amz-Expires must not exceed the credential window")
		})
	}
}

func (s *PresignGetSuite) TestExpiringCredentials_TTLNeverFallsAsWindowGrows() {
	var prev time.Duration
	for _, window := range []time.Duration{
		2 * time.Second, 30 * time.Second, time.Minute,
		61 * time.Second, 2 * time.Minute, time.Hour,
	} {
		s.SetupTest()
		got := s.expectPresign()

		_, _, err := presignGet(s.T().Context(), expiringCredentials(window), s.mockPresign, "bkt", "k")

		s.Require().NoError(err)
		s.GreaterOrEqual(got.opts.Expires, prev,
			"TTL fell as the credential window grew (%s)", window)
		prev = got.opts.Expires
	}
}

func (s *PresignGetSuite) TestStaticCredentials_UsesFixedTTL() {
	got := s.expectPresign()

	before := time.Now()
	_, expiresAt, err := presignGet(s.T().Context(), staticCredentials(), s.mockPresign, "bkt", "k")

	s.Require().NoError(err)
	s.Equal(staticCredentialsTTL, got.opts.Expires)
	s.WithinDuration(before.Add(staticCredentialsTTL), expiresAt, 2*time.Second)
}

func (s *PresignGetSuite) TestSignsWithTheExactCredentialsItMeasured() {
	creds := expiringCredentials(time.Hour)
	got := s.expectPresign()

	_, _, err := presignGet(s.T().Context(), creds, s.mockPresign, "bkt", "k")
	s.Require().NoError(err)

	s.Require().NotNil(got.inner.Credentials, "signing credentials must be pinned, not left ambient")
	pinned, err := got.inner.Credentials.Retrieve(s.T().Context())
	s.Require().NoError(err)
	s.Equal(creds.creds.AccessKeyID, pinned.AccessKeyID)
	s.Equal(creds.creds.SessionToken, pinned.SessionToken)
	s.Equal(creds.creds.Expires, pinned.Expires)
}

func (s *PresignGetSuite) TestAlreadyExpiredCredentials_Fails() {
	_, _, err := presignGet(s.T().Context(), expiringCredentials(-time.Minute), s.mockPresign, "bkt", "k")

	s.Require().ErrorContains(err, "signing credentials expired")
}

func (s *PresignGetSuite) TestCredentialsError_Propagates() {
	provider := fakeCredentials{err: errors.New("no credentials")}

	_, _, err := presignGet(s.T().Context(), provider, s.mockPresign, "bkt", "k")

	s.Require().ErrorContains(err, "no credentials")
}

func (s *PresignGetSuite) TestSignerError_Propagates() {
	s.mockPresign.EXPECT().
		PresignGetObject(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("s3: access denied"))

	_, _, err := presignGet(s.T().Context(), expiringCredentials(time.Hour), s.mockPresign, "bkt", "k")

	s.Require().ErrorContains(err, "s3: access denied")
}

func (s *PresignGetSuite) TestSignsTheRequestedObject() {
	var in *awss3.GetObjectInput
	s.mockPresign.EXPECT().
		PresignGetObject(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, got *awss3.GetObjectInput, _ ...func(*awss3.PresignOptions)) {
			in = got
		}).
		Return(&v4.PresignedHTTPRequest{URL: "https://example.com/signed"}, nil)

	_, _, err := presignGet(s.T().Context(), expiringCredentials(time.Hour), s.mockPresign, "bkt", "some/key")

	s.Require().NoError(err)
	s.Require().NotNil(in)
	s.Equal("bkt", aws.ToString(in.Bucket))
	s.Equal("some/key", aws.ToString(in.Key))
}

func (s *PresignGetSuite) TestRealSigner_XAmzExpiresMatchesReportedExpiryAndFitsCredentials() {
	creds := expiringCredentials(time.Hour)
	cfg := aws.Config{Region: "us-east-2", Credentials: creds}
	client := awss3.NewFromConfig(cfg)

	before := time.Now()
	u, expiresAt, err := presignGet(s.T().Context(), creds, awss3.NewPresignClient(client), "bkt", "k")
	s.Require().NoError(err)

	parsed, err := url.Parse(u)
	s.Require().NoError(err)
	q := parsed.Query()

	s.Equal("ASIATESTTESTTESTTEST", q.Get("X-Amz-Credential")[:20])
	s.Equal("session-token", q.Get("X-Amz-Security-Token"))

	urlTTL, err := time.ParseDuration(q.Get("X-Amz-Expires") + "s")
	s.Require().NoError(err)
	s.WithinDuration(before.Add(urlTTL), expiresAt, 2*time.Second,
		"the URL's own window and the reported expiry must describe the same deadline")
	s.False(before.Add(urlTTL).After(creds.creds.Expires),
		"the URL outlives the credentials that signed it")
}
