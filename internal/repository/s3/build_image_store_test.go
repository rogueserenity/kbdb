package s3

import (
	"errors"
	"testing"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/s3/mocks"
)

type BuildImageStoreSuite struct {
	suite.Suite

	mockClient  *mocks.MockS3API
	mockPresign *mocks.MockS3PresignAPI
	store       *BuildImageStore
}

func TestBuildImageStoreSuite(t *testing.T) {
	suite.Run(t, new(BuildImageStoreSuite))
}

func (s *BuildImageStoreSuite) SetupTest() {
	s.mockClient = mocks.NewMockS3API(s.T())
	s.mockPresign = mocks.NewMockS3PresignAPI(s.T())
	s.store = &BuildImageStore{client: s.mockClient, presign: s.mockPresign, bucket: "images-bucket"}
}

func (s *BuildImageStoreSuite) TestPresignGetBuildImage_Succeeds() {
	s.mockPresign.EXPECT().
		PresignGetObject(mock.Anything, mock.MatchedBy(func(in *s3.GetObjectInput) bool {
			return *in.Bucket == "images-bucket" && *in.Key == "builds/alice/b1/images/img1"
		})).
		Return(&v4.PresignedHTTPRequest{URL: "https://example.com/presigned-get"}, nil)

	url, err := s.store.PresignGetBuildImage(s.T().Context(), "builds/alice/b1/images/img1")

	s.Require().NoError(err)
	s.Equal("https://example.com/presigned-get", url)
}

func (s *BuildImageStoreSuite) TestPresignGetBuildImage_SDKError_Propagates() {
	s.mockPresign.EXPECT().
		PresignGetObject(mock.Anything, mock.Anything).
		Return(nil, errors.New("s3: access denied"))

	url, err := s.store.PresignGetBuildImage(s.T().Context(), "builds/alice/b1/images/img1")

	s.Require().ErrorContains(err, "s3: access denied")
	s.Empty(url)
}

func (s *BuildImageStoreSuite) TestPresignPutBuildImage_Succeeds() {
	s.mockPresign.EXPECT().
		PresignPutObject(mock.Anything, mock.MatchedBy(func(in *s3.PutObjectInput) bool {
			return *in.Bucket == "images-bucket" && *in.Key == "builds/alice/b1/images/img1" && *in.ContentType == "image/png"
		})).
		Return(&v4.PresignedHTTPRequest{URL: "https://example.com/presigned-put"}, nil)

	url, err := s.store.PresignPutBuildImage(s.T().Context(), "builds/alice/b1/images/img1", "image/png")

	s.Require().NoError(err)
	s.Equal("https://example.com/presigned-put", url)
}

func (s *BuildImageStoreSuite) TestPresignPutBuildImage_SDKError_Propagates() {
	s.mockPresign.EXPECT().
		PresignPutObject(mock.Anything, mock.Anything).
		Return(nil, errors.New("s3: access denied"))

	url, err := s.store.PresignPutBuildImage(s.T().Context(), "builds/alice/b1/images/img1", "image/png")

	s.Require().ErrorContains(err, "s3: access denied")
	s.Empty(url)
}

func (s *BuildImageStoreSuite) TestDeleteBuildImage_Succeeds() {
	s.mockClient.EXPECT().
		DeleteObject(mock.Anything, mock.MatchedBy(func(in *s3.DeleteObjectInput) bool {
			return *in.Bucket == "images-bucket" && *in.Key == "builds/alice/b1/images/img1"
		})).
		Return(&s3.DeleteObjectOutput{}, nil)

	err := s.store.DeleteBuildImage(s.T().Context(), "builds/alice/b1/images/img1")

	s.Require().NoError(err)
}

func (s *BuildImageStoreSuite) TestDeleteBuildImage_SDKError_Propagates() {
	s.mockClient.EXPECT().
		DeleteObject(mock.Anything, mock.Anything).
		Return(nil, errors.New("s3: access denied"))

	err := s.store.DeleteBuildImage(s.T().Context(), "builds/alice/b1/images/img1")

	s.Require().ErrorContains(err, "s3: access denied")
}

func (s *BuildImageStoreSuite) TestBestEffortDelete_DeletesEachKey() {
	s.mockClient.EXPECT().
		DeleteObject(mock.Anything, mock.MatchedBy(func(in *s3.DeleteObjectInput) bool {
			return *in.Key == "builds/alice/b1/images/img1"
		})).
		Return(&s3.DeleteObjectOutput{}, nil)
	s.mockClient.EXPECT().
		DeleteObject(mock.Anything, mock.MatchedBy(func(in *s3.DeleteObjectInput) bool {
			return *in.Key == "builds/alice/b1/images/img2"
		})).
		Return(&s3.DeleteObjectOutput{}, nil)

	s.store.BestEffortDelete(s.T().Context(), []repository.BuildImageKey{
		"builds/alice/b1/images/img1",
		"builds/alice/b1/images/img2",
	})
}

func (s *BuildImageStoreSuite) TestBestEffortDelete_PerKeyErrorDoesNotStopRemainingDeletes() {
	s.mockClient.EXPECT().
		DeleteObject(mock.Anything, mock.MatchedBy(func(in *s3.DeleteObjectInput) bool {
			return *in.Key == "builds/alice/b1/images/img1"
		})).
		Return(nil, errors.New("s3: access denied"))
	s.mockClient.EXPECT().
		DeleteObject(mock.Anything, mock.MatchedBy(func(in *s3.DeleteObjectInput) bool {
			return *in.Key == "builds/alice/b1/images/img2"
		})).
		Return(&s3.DeleteObjectOutput{}, nil)

	s.store.BestEffortDelete(s.T().Context(), []repository.BuildImageKey{
		"builds/alice/b1/images/img1",
		"builds/alice/b1/images/img2",
	})
}
