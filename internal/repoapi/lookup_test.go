package repoapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/lookup"
)

type LookupToAPISuite struct {
	suite.Suite
}

func TestLookupToAPISuite(t *testing.T) {
	suite.Run(t, new(LookupToAPISuite))
}

// mustGetCategory looks up a real, catalog-sourced Lookup - LookupToAPI's
// contract is that it's only ever called with values from
// lookup.GetCategory, never a hand-built Lookup, so tests exercise it the
// same way.
func (s *LookupToAPISuite) mustGetCategory(category lookup.Category) lookup.Lookup {
	l, ok := lookup.GetCategory(context.Background(), category)
	s.Require().True(ok, "category %q not found", category)
	return l
}

func (s *LookupToAPISuite) TestPlainStringCategory_PassesThrough() {
	l := s.mustGetCategory(lookup.CategoryVendor)

	got, err := LookupToAPI(l)

	s.Require().NoError(err)
	s.Equal("vendor", got.Category)
	s.Contains(got.Values, "Amazon")
}

func (s *LookupToAPISuite) TestKeyboardLayout_DecodesTyped() {
	l := s.mustGetCategory(lookup.CategoryKeyboardLayout)

	got, err := LookupToAPI(l)

	s.Require().NoError(err)
	s.NotEmpty(got.Values)
	s.IsType(lookup.LayoutValue{}, got.Values[0])
}

func (s *LookupToAPISuite) TestBuildCaseMountType_DecodesTyped() {
	l := s.mustGetCategory(lookup.CategoryBuildCaseMountType)

	got, err := LookupToAPI(l)

	s.Require().NoError(err)
	s.NotEmpty(got.Values)
	s.IsType(lookup.CaseMountTypeValue{}, got.Values[0])
}
