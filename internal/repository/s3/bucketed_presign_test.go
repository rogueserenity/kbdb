package s3

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/stretchr/testify/suite"
)

type GetPresignConfigSuite struct {
	suite.Suite
}

func TestGetPresignConfigSuite(t *testing.T) {
	suite.Run(t, new(GetPresignConfigSuite))
}

func (s *GetPresignConfigSuite) TestWindow_MidBucket_UsesBucketStartAndRemainingTime() {
	bucketStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := getPresignConfig{
		bucket:  24 * time.Hour,
		minTTL:  3 * time.Hour,
		nowFunc: func() time.Time { return bucketStart.Add(10 * time.Hour) },
	}

	signingTime, expires := cfg.window()

	s.True(bucketStart.Equal(signingTime))
	s.Equal(14*time.Hour, expires)
}

func (s *GetPresignConfigSuite) TestWindow_TwoCallsInSameBucket_ProduceSameSigningTime() {
	bucketStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := getPresignConfig{bucket: 24 * time.Hour, minTTL: 3 * time.Hour}

	cfg.nowFunc = func() time.Time { return bucketStart.Add(1 * time.Hour) }
	signingTime1, _ := cfg.window()

	cfg.nowFunc = func() time.Time { return bucketStart.Add(20 * time.Hour) }
	signingTime2, _ := cfg.window()

	s.True(signingTime1.Equal(signingTime2))
}

func (s *GetPresignConfigSuite) TestWindow_RemainingBelowMinTTL_RollsToNextBucket() {
	bucketStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := getPresignConfig{
		bucket: 24 * time.Hour,
		minTTL: 3 * time.Hour,
		// Only 1h left in today's bucket - below the 3h floor.
		nowFunc: func() time.Time { return bucketStart.Add(23 * time.Hour) },
	}

	signingTime, expires := cfg.window()

	s.True(bucketStart.Equal(signingTime))
	s.Equal(25*time.Hour, expires)
}

func (s *GetPresignConfigSuite) TestWindow_RemainingAtExactlyMinTTL_DoesNotRoll() {
	bucketStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := getPresignConfig{
		bucket:  24 * time.Hour,
		minTTL:  3 * time.Hour,
		nowFunc: func() time.Time { return bucketStart.Add(21 * time.Hour) },
	}

	_, expires := cfg.window()

	s.Equal(3*time.Hour, expires)
}

func (s *GetPresignConfigSuite) TestWindow_ZeroBucket_FallsThroughToNowWithNoExpiry() {
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	cfg := getPresignConfig{nowFunc: func() time.Time { return now }}

	signingTime, expires := cfg.window()

	s.True(now.Equal(signingTime))
	s.Zero(expires)
}

func (s *GetPresignConfigSuite) TestPresignOptionFns_ZeroBucket_ReturnsNil() {
	cfg := getPresignConfig{}

	s.Nil(cfg.presignOptionFns())
}

func (s *GetPresignConfigSuite) TestCacheControl_MidBucket_UsesRemainingWindowAsMaxAge() {
	bucketStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := getPresignConfig{
		bucket:  24 * time.Hour,
		minTTL:  3 * time.Hour,
		nowFunc: func() time.Time { return bucketStart.Add(10 * time.Hour) },
	}

	s.Equal("public, max-age=50400, immutable", cfg.cacheControl())
}

func (s *GetPresignConfigSuite) TestCacheControl_ZeroBucket_ReturnsEmpty() {
	cfg := getPresignConfig{}

	s.Empty(cfg.cacheControl())
}

func (s *GetPresignConfigSuite) TestNow_NoNowFunc_UsesWallClock() {
	cfg := getPresignConfig{}

	before := time.Now()
	got := cfg.now()
	after := time.Now()

	s.False(got.Before(before))
	s.False(got.After(after))
}

type FixedTimeSignerSuite struct {
	suite.Suite
}

func TestFixedTimeSignerSuite(t *testing.T) {
	suite.Run(t, new(FixedTimeSignerSuite))
}

type recordingPresigner struct {
	gotSigningTime time.Time
}

func (r *recordingPresigner) PresignHTTP(
	_ context.Context, _ aws.Credentials, _ *http.Request,
	_ string, _ string, _ string, signingTime time.Time,
	_ ...func(*v4.SignerOptions),
) (string, http.Header, error) {
	r.gotSigningTime = signingTime

	return "https://example.com/presigned", nil, nil
}

func (s *FixedTimeSignerSuite) TestPresignHTTP_OverridesSigningTimeWithFixedValue() {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	inner := &recordingPresigner{}
	signer := fixedTimeSigner{wrapped: inner, signingTime: fixed}

	url, _, err := signer.PresignHTTP(
		s.T().Context(), aws.Credentials{}, &http.Request{},
		"hash", "s3", "us-east-1", time.Now(),
	)

	s.Require().NoError(err)
	s.Equal("https://example.com/presigned", url)
	s.True(fixed.Equal(inner.gotSigningTime))
}
