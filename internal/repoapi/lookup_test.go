package repoapi

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/repository"
)

type LookupToAPISuite struct {
	suite.Suite
}

func TestLookupToAPISuite(t *testing.T) {
	suite.Run(t, new(LookupToAPISuite))
}

func (s *LookupToAPISuite) TestPlainStringCategory_PassesThrough() {
	l := repository.Lookup{Category: "vendor", Values: []any{"Amazon", "CannonKeys"}}

	got, err := LookupToAPI(l)

	s.Require().NoError(err)
	s.Equal("vendor", got.Category)
	s.Equal([]any{"Amazon", "CannonKeys"}, got.Values)
}

func (s *LookupToAPISuite) TestKeyboardLayout_DecodesTyped() {
	l := repository.Lookup{
		Category: repository.CategoryKeyboardLayout,
		Values: []any{
			map[string]any{"name": "WK", "sizes": []any{"60%", "65%"}},
		},
	}

	got, err := LookupToAPI(l)

	s.Require().NoError(err)
	s.Require().Len(got.Values, 1)
	s.Equal(repository.LayoutValue{Name: "WK", Sizes: []string{"60%", "65%"}}, got.Values[0])
}

func (s *LookupToAPISuite) TestKeyboardLayout_WrongShape_Errors() {
	l := repository.Lookup{
		Category: repository.CategoryKeyboardLayout,
		Values:   []any{"WK"},
	}

	_, err := LookupToAPI(l)

	s.Require().Error(err)
}

func (s *LookupToAPISuite) TestBuildCaseMountType_DecodesTyped() {
	l := repository.Lookup{
		Category: repository.CategoryBuildCaseMountType,
		Values: []any{
			map[string]any{"name": "Gasket Mount", "supports_durometer": true},
		},
	}

	got, err := LookupToAPI(l)

	s.Require().NoError(err)
	s.Require().Len(got.Values, 1)
	s.Equal(repository.CaseMountTypeValue{Name: "Gasket Mount", SupportsDurometer: true}, got.Values[0])
}

func (s *LookupToAPISuite) TestBuildCaseMountType_WrongShape_Errors() {
	l := repository.Lookup{
		Category: repository.CategoryBuildCaseMountType,
		Values:   []any{"Gasket Mount"},
	}

	_, err := LookupToAPI(l)

	s.Require().Error(err)
}
