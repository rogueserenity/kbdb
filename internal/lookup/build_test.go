package lookup_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type ValidateBuildSuite struct {
	suite.Suite
}

func TestValidateBuildSuite(t *testing.T) {
	suite.Run(t, new(ValidateBuildSuite))
}

func (s *ValidateBuildSuite) TestAllFieldsUnset_SkipsValidation() {
	b := repository.Build{}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Empty(errs)
}

func (s *ValidateBuildSuite) TestValidFields_ReturnsNoErrors() {
	mountType := "Gasket Mount"
	durometer := "40A"
	stabName := "Durock v3"
	stabMount := "Screw-in"
	b := repository.Build{
		CaseMountType: &repository.BuildCaseMountType{Type: &mountType, Durometer: &durometer},
		Stabs:         &repository.BuildStabs{Name: &stabName, MountType: &stabMount},
	}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Empty(errs)
}

func (s *ValidateBuildSuite) TestInvalidCaseMountType_ReturnsFieldError() {
	mountType := "NotAMountType"
	b := repository.Build{CaseMountType: &repository.BuildCaseMountType{Type: &mountType}}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Equal([]lookup.FieldError{
		{Field: "case_mount_type.type", Value: "NotAMountType", Category: lookup.CategoryBuildCaseMountType},
	}, errs)
}

func (s *ValidateBuildSuite) TestInvalidDurometer_ReturnsFieldError() {
	durometer := "NotADurometer"
	b := repository.Build{CaseMountType: &repository.BuildCaseMountType{Durometer: &durometer}}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Equal([]lookup.FieldError{
		{Field: "case_mount_type.durometer", Value: "NotADurometer", Category: lookup.CategoryBuildDurometer},
	}, errs)
}

func (s *ValidateBuildSuite) TestInvalidStabName_ReturnsFieldError() {
	name := "NotAStab"
	b := repository.Build{Stabs: &repository.BuildStabs{Name: &name}}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Equal([]lookup.FieldError{
		{Field: "stabs.name", Value: "NotAStab", Category: lookup.CategoryBuildStabilizer},
	}, errs)
}

func (s *ValidateBuildSuite) TestInvalidStabMountType_ReturnsFieldError() {
	mountType := "NotAMountType"
	b := repository.Build{Stabs: &repository.BuildStabs{MountType: &mountType}}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Equal([]lookup.FieldError{
		{Field: "stabs.mount_type", Value: "NotAMountType", Category: lookup.CategoryBuildStabilizerMountType},
	}, errs)
}
