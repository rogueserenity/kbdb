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

func (s *ParseLookupValuesSuite) TestParseStrings_AllStrings_Succeeds() {
	got, err := repository.ParseStrings([]any{"a", "b"})
	s.Require().NoError(err)
	s.Equal([]string{"a", "b"}, got)
}

func (s *ParseLookupValuesSuite) TestParseStrings_NonStringEntry_Errors() {
	_, err := repository.ParseStrings([]any{"a", 1})
	s.Require().Error(err)
}

func (s *ParseLookupValuesSuite) TestParseLayoutValues_Valid_Succeeds() {
	got, err := repository.ParseLayoutValues([]any{
		map[string]any{"name": "WK", "sizes": []any{"60%", "65%"}},
	})
	s.Require().NoError(err)
	s.Equal([]repository.LayoutValue{{Name: "WK", Sizes: []string{"60%", "65%"}}}, got)
}

func (s *ParseLookupValuesSuite) TestParseLayoutValues_WrongShape_Errors() {
	_, err := repository.ParseLayoutValues([]any{"WK"})
	s.Require().Error(err)
}

func (s *ParseLookupValuesSuite) TestParseLayoutValues_MissingName_Errors() {
	_, err := repository.ParseLayoutValues([]any{
		map[string]any{"sizes": []any{"60%"}},
	})
	s.Require().Error(err)
}

func (s *ParseLookupValuesSuite) TestParseCaseMountTypeValues_Valid_Succeeds() {
	got, err := repository.ParseCaseMountTypeValues([]any{
		map[string]any{"name": "Gasket Mount", "supports_durometer": true},
	})
	s.Require().NoError(err)
	s.Equal([]repository.CaseMountTypeValue{{Name: "Gasket Mount", SupportsDurometer: true}}, got)
}

func (s *ParseLookupValuesSuite) TestParseCaseMountTypeValues_WrongShape_Errors() {
	_, err := repository.ParseCaseMountTypeValues([]any{"Gasket Mount"})
	s.Require().Error(err)
}

func (s *ParseLookupValuesSuite) TestParseCaseMountTypeValues_MissingName_Errors() {
	_, err := repository.ParseCaseMountTypeValues([]any{
		map[string]any{"supports_durometer": true},
	})
	s.Require().Error(err)
}
