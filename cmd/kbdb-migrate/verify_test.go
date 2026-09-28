package main

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type VerifySuite struct {
	suite.Suite
}

func TestVerifySuite(t *testing.T) {
	suite.Run(t, new(VerifySuite))
}

func (s *VerifySuite) TestCompareScalars_CurrencyDiffers_Matches() {
	dump := `{"id":"a","brand":"B","purchase":{"price":10,"currency":"EUR"},"stabs":{"price":5,"currency":"EUR"},"currency":"EUR"}`
	live := `{"id":"b","brand":"B","purchase":{"price":10,"currency":"USD"},"stabs":{"price":5,"currency":"USD"},"currency":"USD"}`

	ok, detail := compareScalars([]byte(dump), []byte(live))

	s.True(ok, detail)
}

func (s *VerifySuite) TestCompareScalars_DumpPredatesCurrency_Matches() {
	dump := `{"brand":"B","kits":[{"name":"Base","purchase":{"price":10}}]}`
	live := `{"brand":"B","kits":[{"name":"Base","purchase":{"price":10,"currency":"USD"}}]}`

	ok, detail := compareScalars([]byte(dump), []byte(live))

	s.True(ok, detail)
}

func (s *VerifySuite) TestCompareScalars_PriceDiffers_Mismatch() {
	dump := `{"brand":"B","purchase":{"price":10,"currency":"USD"}}`
	live := `{"brand":"B","purchase":{"price":12,"currency":"USD"}}`

	ok, _ := compareScalars([]byte(dump), []byte(live))

	s.False(ok)
}
