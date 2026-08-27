package s3

import (
	"errors"
	"testing"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/repository/s3/mocks"
)

type ProfileImageStoreSuite struct {
	suite.Suite

	mockClient  *mocks.MockS3API
	mockPresign *mocks.MockS3PresignAPI
	store       *ProfileImageStore
}

func TestProfileImageStoreSuite(t *testing.T) {
	suite.Run(t, new(ProfileImageStoreSuite))
}

func (s *ProfileImageStoreSuite) SetupTest() {
	s.mockClient = mocks.NewMockS3API(s.T())
	s.mockPresign = mocks.NewMockS3PresignAPI(s.T())
	s.store = &ProfileImageStore{client: s.mockClient, presign: s.mockPresign, bucket: "images-bucket"}
}

func (s *ProfileImageStoreSuite) TestPresignGet_Succeeds() {
	s.mockPresign.EXPECT().
		PresignGetObject(mock.Anything, mock.MatchedBy(func(in *s3.GetObjectInput) bool {
			return *in.Bucket == "images-bucket" && *in.Key == "profiles/user-alice/avatar"
		})).
		Return(&v4.PresignedHTTPRequest{URL: "https://example.com/presigned-get"}, nil)

	url, err := s.store.PresignGet(s.T().Context(), "profiles/user-alice/avatar")

	s.Require().NoError(err)
	s.Equal("https://example.com/presigned-get", url)
}

func (s *ProfileImageStoreSuite) TestPresignGet_SDKError_Propagates() {
	s.mockPresign.EXPECT().
		PresignGetObject(mock.Anything, mock.Anything).
		Return(nil, errors.New("s3: access denied"))

	_, err := s.store.PresignGet(s.T().Context(), "profiles/user-alice/avatar")

	s.Require().ErrorContains(err, "s3: access denied")
}

func (s *ProfileImageStoreSuite) TestPresignPut_Succeeds() {
	s.mockPresign.EXPECT().
		PresignPutObject(mock.Anything, mock.MatchedBy(func(in *s3.PutObjectInput) bool {
			return *in.Bucket == "images-bucket" &&
				*in.Key == "profiles/user-alice/avatar" &&
				*in.ContentType == "image/png"
		})).
		Return(&v4.PresignedHTTPRequest{URL: "https://example.com/presigned-put"}, nil)

	url, err := s.store.PresignPut(s.T().Context(), "profiles/user-alice/avatar", "image/png")

	s.Require().NoError(err)
	s.Equal("https://example.com/presigned-put", url)
}

func (s *ProfileImageStoreSuite) TestDelete_Succeeds() {
	s.mockClient.EXPECT().
		DeleteObject(mock.Anything, mock.MatchedBy(func(in *s3.DeleteObjectInput) bool {
			return *in.Bucket == "images-bucket" && *in.Key == "profiles/user-alice/avatar"
		})).
		Return(&s3.DeleteObjectOutput{}, nil)

	s.Require().NoError(s.store.Delete(s.T().Context(), "profiles/user-alice/avatar"))
}

func (s *ProfileImageStoreSuite) TestDelete_SDKError_Propagates() {
	s.mockClient.EXPECT().
		DeleteObject(mock.Anything, mock.Anything).
		Return(nil, errors.New("s3: boom"))

	s.Require().ErrorContains(s.store.Delete(s.T().Context(), "profiles/user-alice/avatar"), "s3: boom")
}
