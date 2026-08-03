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

type ValidateKeyboardSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
}

func TestValidateKeyboardSuite(t *testing.T) {
	suite.Run(t, new(ValidateKeyboardSuite))
}

func (s *ValidateKeyboardSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
}

func (s *ValidateKeyboardSuite) TestAllFieldsUnset_SkipsValidation() {
	kb := repository.Keyboard{}

	errs, err := lookup.ValidateKeyboard(s.T().Context(), s.mockRepo, kb)
	s.Require().NoError(err)
	s.Empty(errs)
}

func (s *ValidateKeyboardSuite) TestInvalidSize_ReturnsFieldError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardSize).
		Return(&repository.Lookup{Category: repository.CategoryKeyboardSize, Values: []any{"60%"}}, nil)

	size := "40%"
	kb := repository.Keyboard{Size: &size}

	errs, err := lookup.ValidateKeyboard(s.T().Context(), s.mockRepo, kb)
	s.Require().NoError(err)
	s.Equal([]lookup.FieldError{
		{Field: "size", Value: "40%", Category: repository.CategoryKeyboardSize},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestValidSizeAndLayout_ReturnsNoErrors() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardSize).
		Return(&repository.Lookup{Category: repository.CategoryKeyboardSize, Values: []any{"60%"}}, nil)
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardLayout).
		Return(&repository.Lookup{
			Category: repository.CategoryKeyboardLayout,
			Values:   []any{map[string]any{"name": "WK", "sizes": []any{"60%", "65%"}}},
		}, nil)

	size := "60%"
	layout := "WK"
	kb := repository.Keyboard{Size: &size, Layout: &layout}

	errs, err := lookup.ValidateKeyboard(s.T().Context(), s.mockRepo, kb)
	s.Require().NoError(err)
	s.Empty(errs)
}

func (s *ValidateKeyboardSuite) TestLayoutNotValidForSize_ReturnsFieldError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardSize).
		Return(&repository.Lookup{Category: repository.CategoryKeyboardSize, Values: []any{"60%", "65%"}}, nil)
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardLayout).
		Return(&repository.Lookup{
			Category: repository.CategoryKeyboardLayout,
			Values:   []any{map[string]any{"name": "WK", "sizes": []any{"60%"}}},
		}, nil)

	size := "65%"
	layout := "WK"
	kb := repository.Keyboard{Size: &size, Layout: &layout}

	errs, err := lookup.ValidateKeyboard(s.T().Context(), s.mockRepo, kb)
	s.Require().NoError(err)
	s.Equal([]lookup.FieldError{
		{Field: "layout", Value: "WK", Category: repository.CategoryKeyboardSize},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestInvalidSize_SkipsLayoutSizeCrossCheck() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardSize).
		Return(&repository.Lookup{Category: repository.CategoryKeyboardSize, Values: []any{"60%"}}, nil)
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardLayout).
		Return(&repository.Lookup{
			Category: repository.CategoryKeyboardLayout,
			Values:   []any{map[string]any{"name": "WK", "sizes": []any{"60%"}}},
		}, nil)

	size := "40%"
	layout := "WK"
	kb := repository.Keyboard{Size: &size, Layout: &layout}

	errs, err := lookup.ValidateKeyboard(s.T().Context(), s.mockRepo, kb)
	s.Require().NoError(err)
	s.Equal([]lookup.FieldError{
		{Field: "size", Value: "40%", Category: repository.CategoryKeyboardSize},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestInvalidLayoutName_ReturnsFieldError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardLayout).
		Return(&repository.Lookup{
			Category: repository.CategoryKeyboardLayout,
			Values:   []any{map[string]any{"name": "WK", "sizes": []any{"60%"}}},
		}, nil)

	layout := "ANSI"
	kb := repository.Keyboard{Layout: &layout}

	errs, err := lookup.ValidateKeyboard(s.T().Context(), s.mockRepo, kb)
	s.Require().NoError(err)
	s.Equal([]lookup.FieldError{
		{Field: "layout", Value: "ANSI", Category: repository.CategoryKeyboardLayout},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestFieldValidationRepoError_Propagates() {
	wantErr := errors.New("boom")
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardSize).
		Return(nil, wantErr)

	size := "60%"
	kb := repository.Keyboard{Size: &size}

	_, err := lookup.ValidateKeyboard(s.T().Context(), s.mockRepo, kb)
	s.Require().ErrorIs(err, wantErr)
}

func (s *ValidateKeyboardSuite) TestLayoutCategoryNotFound_ReturnsFieldError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardLayout).
		Return(nil, repository.ErrNotFound)

	layout := "WK"
	kb := repository.Keyboard{Layout: &layout}

	errs, err := lookup.ValidateKeyboard(s.T().Context(), s.mockRepo, kb)
	s.Require().NoError(err)
	s.Equal([]lookup.FieldError{
		{Field: "layout", Value: "WK", Category: repository.CategoryKeyboardLayout},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestLayoutLookupRepoError_Propagates() {
	wantErr := errors.New("boom")
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardLayout).
		Return(nil, wantErr)

	layout := "WK"
	kb := repository.Keyboard{Layout: &layout}

	_, err := lookup.ValidateKeyboard(s.T().Context(), s.mockRepo, kb)
	s.Require().ErrorIs(err, wantErr)
}

func (s *ValidateKeyboardSuite) TestLayoutValuesWrongShape_Errors() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardLayout).
		Return(&repository.Lookup{Category: repository.CategoryKeyboardLayout, Values: []any{"WK"}}, nil)

	layout := "WK"
	kb := repository.Keyboard{Layout: &layout}

	_, err := lookup.ValidateKeyboard(s.T().Context(), s.mockRepo, kb)
	s.Require().Error(err)
}

func (s *ValidateKeyboardSuite) TestInvalidPlateMaterial_ReturnsIndexedFieldError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardPlateMaterial).
		Return(&repository.Lookup{Category: repository.CategoryKeyboardPlateMaterial, Values: []any{"Aluminum"}}, nil)

	kb := repository.Keyboard{
		Design: repository.KeyboardDesign{Plates: []string{"Aluminum", "Brass"}},
	}

	errs, err := lookup.ValidateKeyboard(s.T().Context(), s.mockRepo, kb)
	s.Require().NoError(err)
	s.Equal([]lookup.FieldError{
		{Field: "design.plates[1]", Value: "Brass", Category: repository.CategoryKeyboardPlateMaterial},
	}, errs)
}
