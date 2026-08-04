package repository_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/repository"
)

type ParseLookupValuesSuite struct {
	suite.Suite
}

func TestParseLookupValuesSuite(t *testing.T) {
	suite.Run(t, new(ParseLookupValuesSuite))
}

func (s *ParseLookupValuesSuite) TestStrings_AllStrings_Succeeds() {
	got, err := repository.Lookup{Values: []any{"a", "b"}}.Strings()
	s.Require().NoError(err)
	s.Equal([]string{"a", "b"}, got)
}

func (s *ParseLookupValuesSuite) TestStrings_NonStringEntry_Errors() {
	_, err := repository.Lookup{Values: []any{"a", 1}}.Strings()
	s.Require().Error(err)
}

func (s *ParseLookupValuesSuite) TestToAnySlice_WidensTypedSlice() {
	got := repository.ToAnySlice([]repository.LayoutValue{{Name: "WK", Sizes: []string{"60%"}}})
	s.Equal([]any{repository.LayoutValue{Name: "WK", Sizes: []string{"60%"}}}, got)
}

func (s *ParseLookupValuesSuite) TestToAnySlice_Empty_ReturnsEmptySliceNotNil() {
	got := repository.ToAnySlice([]repository.LayoutValue{})
	s.NotNil(got)
	s.Empty(got)
}

func (s *ParseLookupValuesSuite) TestLayoutValues_Valid_Succeeds() {
	got, err := repository.Lookup{Values: []any{
		map[string]any{"name": "WK", "sizes": []any{"60%", "65%"}},
	}}.LayoutValues()
	s.Require().NoError(err)
	s.Equal([]repository.LayoutValue{{Name: "WK", Sizes: []string{"60%", "65%"}}}, got)
}

func (s *ParseLookupValuesSuite) TestLayoutValues_WrongShape_Errors() {
	_, err := repository.Lookup{Values: []any{"WK"}}.LayoutValues()
	s.Require().Error(err)
}

func (s *ParseLookupValuesSuite) TestLayoutValues_MissingName_Errors() {
	_, err := repository.Lookup{Values: []any{
		map[string]any{"sizes": []any{"60%"}},
	}}.LayoutValues()
	s.Require().Error(err)
}

func (s *ParseLookupValuesSuite) TestCaseMountTypeValues_Valid_Succeeds() {
	got, err := repository.Lookup{Values: []any{
		map[string]any{"name": "Gasket Mount", "supports_durometer": true},
	}}.CaseMountTypeValues()
	s.Require().NoError(err)
	s.Equal([]repository.CaseMountTypeValue{{Name: "Gasket Mount", SupportsDurometer: true}}, got)
}

func (s *ParseLookupValuesSuite) TestCaseMountTypeValues_WrongShape_Errors() {
	_, err := repository.Lookup{Values: []any{"Gasket Mount"}}.CaseMountTypeValues()
	s.Require().Error(err)
}

func (s *ParseLookupValuesSuite) TestCaseMountTypeValues_MissingName_Errors() {
	_, err := repository.Lookup{Values: []any{
		map[string]any{"supports_durometer": true},
	}}.CaseMountTypeValues()
	s.Require().Error(err)
}
