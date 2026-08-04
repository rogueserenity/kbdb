package repomcp_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type LookupToMCPSuite struct {
	suite.Suite
}

func TestLookupToMCPSuite(t *testing.T) {
	suite.Run(t, new(LookupToMCPSuite))
}

func (s *LookupToMCPSuite) TestPlainStringCategory_PassesValuesThrough() {
	out, err := repomcp.LookupToMCP(repository.Lookup{Category: "vendor", Values: []any{"a", "b"}})

	s.Require().NoError(err)
	s.Equal(schema.GetLookupOutput{Category: "vendor", Values: []any{"a", "b"}}, out)
}

func (s *LookupToMCPSuite) TestKeyboardLayout_DecodesTypedValues() {
	in := repository.Lookup{
		Category: repository.CategoryKeyboardLayout,
		Values:   []any{map[string]any{"name": "WK", "sizes": []any{"60%", "65%"}}},
	}

	out, err := repomcp.LookupToMCP(in)

	s.Require().NoError(err)
	s.Equal(repository.CategoryKeyboardLayout, out.Category)
	s.Equal([]any{repository.LayoutValue{Name: "WK", Sizes: []string{"60%", "65%"}}}, out.Values)
}

func (s *LookupToMCPSuite) TestKeyboardLayout_WrongShape_Errors() {
	in := repository.Lookup{Category: repository.CategoryKeyboardLayout, Values: []any{"WK"}}

	_, err := repomcp.LookupToMCP(in)

	s.Require().Error(err)
}

func (s *LookupToMCPSuite) TestBuildCaseMountType_DecodesTypedValues() {
	in := repository.Lookup{
		Category: repository.CategoryBuildCaseMountType,
		Values:   []any{map[string]any{"name": "Top Mount", "supports_durometer": true}},
	}

	out, err := repomcp.LookupToMCP(in)

	s.Require().NoError(err)
	s.Equal(repository.CategoryBuildCaseMountType, out.Category)
	s.Equal([]any{repository.CaseMountTypeValue{Name: "Top Mount", SupportsDurometer: true}}, out.Values)
}

func (s *LookupToMCPSuite) TestBuildCaseMountType_WrongShape_Errors() {
	in := repository.Lookup{Category: repository.CategoryBuildCaseMountType, Values: []any{"Top Mount"}}

	_, err := repomcp.LookupToMCP(in)

	s.Require().Error(err)
}
