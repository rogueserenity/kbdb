package repository_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type ValidateFieldsSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
}

func TestValidateFieldsSuite(t *testing.T) {
	suite.Run(t, new(ValidateFieldsSuite))
}

func (s *ValidateFieldsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
}

func (s *ValidateFieldsSuite) TestAllValid_ReturnsNoErrors() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "switch_type").
		Return(&repository.Lookup{Category: "switch_type", Values: []any{"Linear"}}, nil)

	errs, err := repository.ValidateFields(s.T().Context(), s.mockRepo, []repository.FieldCheck{
		{Field: "type", Value: "Linear", Category: "switch_type"},
	})
	s.Require().NoError(err)
	s.Empty(errs)
}

func (s *ValidateFieldsSuite) TestFetchesEachDistinctCategoryOnlyOnce() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "switch_type").
		Return(&repository.Lookup{Category: "switch_type", Values: []any{"Linear"}}, nil).
		Once()

	errs, err := repository.ValidateFields(s.T().Context(), s.mockRepo, []repository.FieldCheck{
		{Field: "type", Value: "Linear", Category: "switch_type"},
		{Field: "type2", Value: "Linear", Category: "switch_type"},
	})
	s.Require().NoError(err)
	s.Empty(errs)
}

func (s *ValidateFieldsSuite) TestCategoryNotFound_TreatsEveryValueInItAsInvalid() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "switch_type").
		Return(nil, repository.ErrNotFound)

	errs, err := repository.ValidateFields(s.T().Context(), s.mockRepo, []repository.FieldCheck{
		{Field: "type", Value: "Linear", Category: "switch_type"},
	})
	s.Require().NoError(err)
	s.Equal([]repository.FieldError{
		{Field: "type", Value: "Linear", Category: "switch_type"},
	}, errs)
}

func (s *ValidateFieldsSuite) TestOtherRepoErrors_Propagate() {
	wantErr := errors.New("boom")
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "switch_type").
		Return(nil, wantErr)

	_, err := repository.ValidateFields(s.T().Context(), s.mockRepo, []repository.FieldCheck{
		{Field: "type", Value: "Linear", Category: "switch_type"},
	})
	s.Require().ErrorIs(err, wantErr)
}

func (s *ValidateFieldsSuite) TestReportsEveryInvalidField_NotJustTheFirst() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "switch_type").
		Return(&repository.Lookup{Category: "switch_type", Values: []any{"Linear"}}, nil)
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(&repository.Lookup{Category: "vendor", Values: []any{"CannonKeys"}}, nil)

	errs, err := repository.ValidateFields(s.T().Context(), s.mockRepo, []repository.FieldCheck{
		{Field: "type", Value: "Tactile", Category: "switch_type"},
		{Field: "vendor", Value: "NovelKeys", Category: "vendor"},
	})
	s.Require().NoError(err)
	s.ElementsMatch([]repository.FieldError{
		{Field: "type", Value: "Tactile", Category: "switch_type"},
		{Field: "vendor", Value: "NovelKeys", Category: "vendor"},
	}, errs)
}
