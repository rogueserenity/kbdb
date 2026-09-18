package repoapi

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type ResolveImageURLSuite struct {
	suite.Suite
}

func TestResolveImageURLSuite(t *testing.T) {
	suite.Run(t, new(ResolveImageURLSuite))
}

func (s *ResolveImageURLSuite) TestCacheHit_WithinFloor_ReusesCachedURLWithoutMintingOrWritingBack() {
	cachedURL := "https://example.com/cached"
	expiresAt := time.Now().Add(2 * getPresignRefreshFloor)

	mintCalled := false
	writeBackCalled := false

	url, err := resolveImageURL(&cachedURL, &expiresAt, time.Hour,
		func() (string, error) {
			mintCalled = true
			return "https://example.com/fresh", nil
		},
		func(string, time.Time) error {
			writeBackCalled = true
			return nil
		},
	)
	s.Require().NoError(err)

	s.Equal(cachedURL, url)
	s.False(mintCalled, "a cached URL with plenty of validity left must not be re-minted")
	s.False(writeBackCalled, "reusing a cached URL must not write back")
}

func (s *ResolveImageURLSuite) TestCacheMiss_NoCachedURL_MintsAndWritesBack() {
	mintCalled := false
	var writtenURL string
	var writtenExpiresAt time.Time

	url, err := resolveImageURL(nil, nil, time.Hour,
		func() (string, error) {
			mintCalled = true
			return "https://example.com/fresh", nil
		},
		func(u string, e time.Time) error {
			writtenURL = u
			writtenExpiresAt = e
			return nil
		},
	)
	s.Require().NoError(err)

	s.True(mintCalled)
	s.Equal("https://example.com/fresh", url)
	s.Equal("https://example.com/fresh", writtenURL)
	s.WithinDuration(time.Now().Add(time.Hour), writtenExpiresAt, 5*time.Second)
}

func (s *ResolveImageURLSuite) TestCacheExpired_BelowRefreshFloor_MintsFreshURL() {
	cachedURL := "https://example.com/cached"
	// Within the refresh floor - treated as not usable.
	expiresAt := time.Now().Add(getPresignRefreshFloor / 2)

	mintCalled := false

	url, err := resolveImageURL(&cachedURL, &expiresAt, time.Hour,
		func() (string, error) {
			mintCalled = true
			return "https://example.com/fresh", nil
		},
		func(string, time.Time) error { return nil },
	)
	s.Require().NoError(err)

	s.True(mintCalled)
	s.Equal("https://example.com/fresh", url)
}

func (s *ResolveImageURLSuite) TestMintError_Propagates() {
	_, err := resolveImageURL(nil, nil, time.Hour,
		func() (string, error) { return "", errors.New("s3: access denied") },
		func(string, time.Time) error { return nil },
	)

	s.Require().Error(err)
}

func (s *ResolveImageURLSuite) TestWriteBackError_IsNonFatal_StillReturnsFreshURL() {
	url, err := resolveImageURL(nil, nil, time.Hour,
		func() (string, error) { return "https://example.com/fresh", nil },
		func(string, time.Time) error { return errors.New("conditional check failed") },
	)
	s.Require().NoError(err)

	s.Equal("https://example.com/fresh", url)
}
