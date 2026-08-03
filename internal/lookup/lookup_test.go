package lookup_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type ValidateImageContentTypeSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
}

func TestValidateImageContentTypeSuite(t *testing.T) {
	suite.Run(t, new(ValidateImageContentTypeSuite))
}

func (s *ValidateImageContentTypeSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
}

func (s *ValidateImageContentTypeSuite) TestValid_ReturnsNoError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryImageContentType).
		Return(&repository.Lookup{Category: repository.CategoryImageContentType, Values: []any{"image/png"}}, nil)

	fieldErr, err := lookup.ValidateImageContentType(s.T().Context(), s.mockRepo, "image/png")
	s.Require().NoError(err)
	s.Nil(fieldErr)
}

func (s *ValidateImageContentTypeSuite) TestInvalid_ReturnsFieldError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryImageContentType).
		Return(&repository.Lookup{Category: repository.CategoryImageContentType, Values: []any{"image/png"}}, nil)

	fieldErr, err := lookup.ValidateImageContentType(s.T().Context(), s.mockRepo, "image/bmp")
	s.Require().NoError(err)
	s.Equal(&lookup.FieldError{Field: "content_type", Value: "image/bmp", Category: repository.CategoryImageContentType}, fieldErr)
}

func (s *ValidateImageContentTypeSuite) TestCategoryNotFound_TreatsValueAsInvalid() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryImageContentType).
		Return(nil, repository.ErrNotFound)

	fieldErr, err := lookup.ValidateImageContentType(s.T().Context(), s.mockRepo, "image/png")
	s.Require().NoError(err)
	s.Equal(&lookup.FieldError{Field: "content_type", Value: "image/png", Category: repository.CategoryImageContentType}, fieldErr)
}

func (s *ValidateImageContentTypeSuite) TestOtherRepoErrors_Propagate() {
	wantErr := errors.New("boom")
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryImageContentType).
		Return(nil, wantErr)

	_, err := lookup.ValidateImageContentType(s.T().Context(), s.mockRepo, "image/png")
	s.Require().ErrorIs(err, wantErr)
}
