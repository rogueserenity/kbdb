package lookup_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/lookup"
)

type CatalogSuite struct {
	suite.Suite
}

func TestCatalogSuite(t *testing.T) {
	suite.Run(t, new(CatalogSuite))
}

// TestEveryCategory_DecodesViaItsExpectedAccessor loops every category
// init() loaded and confirms it decodes cleanly via whichever accessor
// its shape expects - the same check validateShape already runs at init
// (so a bad data file already fails the build), but doing it again here
// as an ordinary test assertion means a failure reads as a normal test
// failure naming the category, not an init-time panic.
func (s *CatalogSuite) TestEveryCategory_DecodesViaItsExpectedAccessor() {
	ctx := context.Background()

	for _, category := range lookup.ListCategories(ctx) {
		l, ok := lookup.GetCategory(ctx, category)
		s.Require().True(ok, "category %q from ListCategories not found via GetCategory", category)

		var err error
		switch category {
		case lookup.CategoryKeyboardLayout:
			_, err = l.LayoutValues()
		case lookup.CategoryBuildCaseMountType:
			_, err = l.CaseMountTypeValues()
		default:
			_, err = l.Strings()
		}
		s.NoError(err, "category %q failed to decode via its expected accessor", category)
	}
}

func (s *CatalogSuite) TestListCategoryNames_MatchesListCategoriesWidened() {
	ctx := context.Background()

	categories := lookup.ListCategories(ctx)
	names := lookup.ListCategoryNames(ctx)

	s.Require().Len(names, len(categories))
	for i, c := range categories {
		s.Equal(string(c), names[i])
	}
}

// TestListCategoryNames_MutatingResult_DoesNotAffectLaterCalls proves
// ListCategoryNames returns a clone, not the package's cached slice - a
// caller mutating what it got back must not corrupt what the next caller
// sees.
func (s *CatalogSuite) TestListCategoryNames_MutatingResult_DoesNotAffectLaterCalls() {
	ctx := context.Background()

	first := lookup.ListCategoryNames(ctx)
	original := first[0]
	first[0] = "corrupted"

	second := lookup.ListCategoryNames(ctx)
	s.Equal(original, second[0])
}

// TestGetCategory_MutatingValues_DoesNotAffectLaterCalls is
// TestListCategoryNames_MutatingResult_DoesNotAffectLaterCalls's
// equivalent for GetCategory's Lookup.Values.
func (s *CatalogSuite) TestGetCategory_MutatingValues_DoesNotAffectLaterCalls() {
	ctx := context.Background()

	first, ok := lookup.GetCategory(ctx, lookup.CategoryVendor)
	s.Require().True(ok)
	original := first.Values[0]
	first.Values[0] = "corrupted"

	second, ok := lookup.GetCategory(ctx, lookup.CategoryVendor)
	s.Require().True(ok)
	s.Equal(original, second.Values[0])
}

// TestStrings_MutatingResult_DoesNotAffectLaterCalls is
// TestListCategoryNames_MutatingResult_DoesNotAffectLaterCalls's
// equivalent for Lookup.Strings' cached decode.
func (s *CatalogSuite) TestStrings_MutatingResult_DoesNotAffectLaterCalls() {
	ctx := context.Background()

	l, ok := lookup.GetCategory(ctx, lookup.CategoryVendor)
	s.Require().True(ok)

	first, err := l.Strings()
	s.Require().NoError(err)
	original := first[0]
	first[0] = "corrupted"

	second, err := l.Strings()
	s.Require().NoError(err)
	s.Equal(original, second[0])
}

func (s *CatalogSuite) TestGetCategory_UnknownCategory_ReturnsFalse() {
	_, ok := lookup.GetCategory(context.Background(), "not-a-real-category")
	s.False(ok)
}

func (s *CatalogSuite) TestStrings_WrongAccessorForCategory_Errors() {
	l, ok := lookup.GetCategory(context.Background(), lookup.CategoryKeyboardLayout)
	s.Require().True(ok)

	_, err := l.Strings()
	s.Error(err)
}

func (s *CatalogSuite) TestLayoutValues_WrongAccessorForCategory_Errors() {
	l, ok := lookup.GetCategory(context.Background(), lookup.CategoryVendor)
	s.Require().True(ok)

	_, err := l.LayoutValues()
	s.Error(err)
}

func (s *CatalogSuite) TestCaseMountTypeValues_WrongAccessorForCategory_Errors() {
	l, ok := lookup.GetCategory(context.Background(), lookup.CategoryVendor)
	s.Require().True(ok)

	_, err := l.CaseMountTypeValues()
	s.Error(err)
}
