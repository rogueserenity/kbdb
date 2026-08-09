package lookup_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/lookup"
)

type ValidateImageContentTypeSuite struct {
	suite.Suite
}

func TestValidateImageContentTypeSuite(t *testing.T) {
	suite.Run(t, new(ValidateImageContentTypeSuite))
}

func (s *ValidateImageContentTypeSuite) TestValid_ReturnsNoError() {
	fieldErr := lookup.ValidateImageContentType(s.T().Context(), "image/png")
	s.Nil(fieldErr)
}

func (s *ValidateImageContentTypeSuite) TestInvalid_ReturnsFieldError() {
	fieldErr := lookup.ValidateImageContentType(s.T().Context(), "image/bmp")
	s.Equal(&lookup.FieldError{Field: "content_type", Value: "image/bmp", Category: lookup.CategoryImageContentType}, fieldErr)
}
