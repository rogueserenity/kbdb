package repository_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/repository"
)

type StillReferencedErrorSuite struct {
	suite.Suite
}

func TestStillReferencedErrorSuite(t *testing.T) {
	suite.Run(t, new(StillReferencedErrorSuite))
}

func (s *StillReferencedErrorSuite) TestIs_MatchesErrStillReferenced() {
	err := &repository.StillReferencedError{BuildIDs: []string{"b1", "b2"}}

	s.Require().ErrorIs(err, repository.ErrStillReferenced)
}

func (s *StillReferencedErrorSuite) TestErrorMessage_IncludesBuildIDs() {
	err := &repository.StillReferencedError{BuildIDs: []string{"b1", "b2"}}

	s.Contains(err.Error(), "b1")
	s.Contains(err.Error(), "b2")
}
