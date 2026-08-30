package s3

import (
	"errors"
	"testing"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/repository/s3/mocks"
)

type KeycapKitImageStoreSuite struct {
	suite.Suite

	mockClient  *mocks.MockS3API
	mockPresign *mocks.MockS3PresignAPI
	store       *KeycapKitImageStore
}

func TestKeycapKitImageStoreSuite(t *testing.T) {
	suite.Run(t, new(KeycapKitImageStoreSuite))
}

func (s *KeycapKitImageStoreSuite) SetupTest() {
	s.mockClient = mocks.NewMockS3API(s.T())
	s.mockPresign = mocks.NewMockS3PresignAPI(s.T())
	s.store = &KeycapKitImageStore{client: s.mockClient, presign: s.mockPresign, bucket: "images-bucket"}
}

func (s *KeycapKitImageStoreSuite) TestPresignGet_Succeeds() {
	s.mockPresign.EXPECT().
		PresignGetObject(mock.Anything, mock.MatchedBy(func(in *s3.GetObjectInput) bool {
			return *in.Bucket == "images-bucket" && *in.Key == "keycap-sets/alice/ks1/kits/kit1/image"
		}), mock.Anything).
		Return(&v4.PresignedHTTPRequest{URL: "https://example.com/presigned-get"}, nil)

	url, err := s.store.PresignGet(s.T().Context(), "keycap-sets/alice/ks1/kits/kit1/image")

	s.Require().NoError(err)
	s.Equal("https://example.com/presigned-get", url)
}

func (s *KeycapKitImageStoreSuite) TestPresignGet_AppliesConfiguredExpiry() {
	s.store.getExpiry = 24 * time.Hour

	s.mockPresign.EXPECT().
		PresignGetObject(mock.Anything, mock.Anything, mock.MatchedBy(func(optFns []func(*s3.PresignOptions)) bool {
			var opts s3.PresignOptions
			for _, fn := range optFns {
				fn(&opts)
			}

			return opts.Expires == 24*time.Hour
		})).
		Return(&v4.PresignedHTTPRequest{URL: "https://example.com/presigned-get"}, nil)

	_, err := s.store.PresignGet(s.T().Context(), "keycap-sets/alice/ks1/kits/kit1/image")

	s.Require().NoError(err)
}

func (s *KeycapKitImageStoreSuite) TestPresignGet_SDKError_Propagates() {
	s.mockPresign.EXPECT().
		PresignGetObject(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("s3: access denied"))

	url, err := s.store.PresignGet(s.T().Context(), "keycap-sets/alice/ks1/kits/kit1/image")

	s.Require().ErrorContains(err, "s3: access denied")
	s.Empty(url)
}

func (s *KeycapKitImageStoreSuite) TestPresignPut_Succeeds() {
	s.mockPresign.EXPECT().
		PresignPutObject(mock.Anything, mock.MatchedBy(func(in *s3.PutObjectInput) bool {
			return *in.Bucket == "images-bucket" && *in.Key == "keycap-sets/alice/ks1/kits/kit1/image" && *in.ContentType == "image/png"
		})).
		Return(&v4.PresignedHTTPRequest{URL: "https://example.com/presigned-put"}, nil)

	url, err := s.store.PresignPut(s.T().Context(), "keycap-sets/alice/ks1/kits/kit1/image", "image/png")

	s.Require().NoError(err)
	s.Equal("https://example.com/presigned-put", url)
}

func (s *KeycapKitImageStoreSuite) TestPresignPut_SDKError_Propagates() {
	s.mockPresign.EXPECT().
		PresignPutObject(mock.Anything, mock.Anything).
		Return(nil, errors.New("s3: access denied"))

	url, err := s.store.PresignPut(s.T().Context(), "keycap-sets/alice/ks1/kits/kit1/image", "image/png")

	s.Require().ErrorContains(err, "s3: access denied")
	s.Empty(url)
}

func (s *KeycapKitImageStoreSuite) TestDelete_Succeeds() {
	s.mockClient.EXPECT().
		DeleteObject(mock.Anything, mock.MatchedBy(func(in *s3.DeleteObjectInput) bool {
			return *in.Bucket == "images-bucket" && *in.Key == "keycap-sets/alice/ks1/kits/kit1/image"
		})).
		Return(&s3.DeleteObjectOutput{}, nil)

	err := s.store.Delete(s.T().Context(), "keycap-sets/alice/ks1/kits/kit1/image")

	s.Require().NoError(err)
}

func (s *KeycapKitImageStoreSuite) TestDelete_SDKError_Propagates() {
	s.mockClient.EXPECT().
		DeleteObject(mock.Anything, mock.Anything).
		Return(nil, errors.New("s3: access denied"))

	err := s.store.Delete(s.T().Context(), "keycap-sets/alice/ks1/kits/kit1/image")

	s.Require().ErrorContains(err, "s3: access denied")
}

