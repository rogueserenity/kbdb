package dynamo

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"
)

type QueryLimitSuite struct {
	suite.Suite
}

func TestQueryLimitSuite(t *testing.T) {
	suite.Run(t, new(QueryLimitSuite))
}

func (s *QueryLimitSuite) TestClampsToRange() {
	cases := []struct {
		name string
		in   int
		want int32
	}{
		{"zero -> 1", 0, 1},
		{"negative -> 1", -5, 1},
		{"in range passes through", 20, 20},
		{"max int32 passes through", math.MaxInt32, math.MaxInt32},
		{"above max int32 -> max int32", math.MaxInt32 + 1, math.MaxInt32},
	}

	for _, c := range cases {
		s.Run(c.name, func() {
			got := queryLimit(c.in)
			s.Require().NotNil(got)
			s.Equal(c.want, *got)
		})
	}
}
