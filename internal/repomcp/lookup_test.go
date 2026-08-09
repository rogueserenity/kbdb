package repomcp_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/repomcp"
)

type LookupToMCPSuite struct {
	suite.Suite
}

func TestLookupToMCPSuite(t *testing.T) {
	suite.Run(t, new(LookupToMCPSuite))
}

// mustGetCategory looks up a real, catalog-sourced Lookup - LookupToMCP
// requires one, not a hand-built Lookup.
func (s *LookupToMCPSuite) mustGetCategory(category lookup.Category) lookup.Lookup {
	l, ok := lookup.GetCategory(context.Background(), category)
	s.Require().True(ok, "category %q not found", category)
	return l
}

func (s *LookupToMCPSuite) TestPlainStringCategory_PassesValuesThrough() {
	l := s.mustGetCategory(lookup.CategoryVendor)

	out, err := repomcp.LookupToMCP(l)

	s.Require().NoError(err)
	s.Equal("vendor", out.Category)
	s.Contains(out.Values, "Amazon")
}

func (s *LookupToMCPSuite) TestKeyboardLayout_DecodesTypedValues() {
	l := s.mustGetCategory(lookup.CategoryKeyboardLayout)

	out, err := repomcp.LookupToMCP(l)

	s.Require().NoError(err)
	s.Equal(string(lookup.CategoryKeyboardLayout), out.Category)
	s.NotEmpty(out.Values)
	s.IsType(lookup.LayoutValue{}, out.Values[0])
}

func (s *LookupToMCPSuite) TestBuildCaseMountType_DecodesTypedValues() {
	l := s.mustGetCategory(lookup.CategoryBuildCaseMountType)

	out, err := repomcp.LookupToMCP(l)

	s.Require().NoError(err)
	s.Equal(string(lookup.CategoryBuildCaseMountType), out.Category)
	s.NotEmpty(out.Values)
	s.IsType(lookup.CaseMountTypeValue{}, out.Values[0])
}
