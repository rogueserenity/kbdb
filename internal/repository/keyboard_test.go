package repository_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/repository"
)

type KeyboardTotalCostSuite struct {
	suite.Suite
}

func TestKeyboardTotalCostSuite(t *testing.T) {
	suite.Run(t, new(KeyboardTotalCostSuite))
}

func (s *KeyboardTotalCostSuite) TestSumsBaseAndPartsSkippingUnpriced() {
	kb := repository.Keyboard{
		Purchase: repository.KeyboardPurchase{Price: new(380.0)},
		Plates: []repository.KeyboardPlate{
			{ID: "p1"},
			{ID: "p2", Purchase: repository.KeyboardPurchase{Price: new(35.0)}},
		},
		PCBs: []repository.KeyboardPCB{
			{ID: "b1", Purchase: repository.KeyboardPurchase{Price: new(45.5)}},
		},
	}

	s.Equal(new(460.5), kb.TotalCost())
}

func (s *KeyboardTotalCostSuite) TestPartsOnly_SumsWithoutBase() {
	kb := repository.Keyboard{
		Plates: []repository.KeyboardPlate{{ID: "p1", Purchase: repository.KeyboardPurchase{Price: new(0.1)}}},
		PCBs:   []repository.KeyboardPCB{{ID: "b1", Purchase: repository.KeyboardPurchase{Price: new(0.2)}}},
	}

	s.Equal(new(0.3), kb.TotalCost())
}

func (s *KeyboardTotalCostSuite) TestNothingPriced_Nil() {
	kb := repository.Keyboard{Plates: []repository.KeyboardPlate{{ID: "p1"}}}

	s.Nil(kb.TotalCost())
}

type KeyboardPartLookupSuite struct {
	suite.Suite

	kb repository.Keyboard
}

func TestKeyboardPartLookupSuite(t *testing.T) {
	suite.Run(t, new(KeyboardPartLookupSuite))
}

func (s *KeyboardPartLookupSuite) SetupTest() {
	s.kb = repository.Keyboard{
		Plates: []repository.KeyboardPlate{{ID: "p1", Purchase: repository.KeyboardPurchase{Price: new(35.0)}}},
		PCBs:   []repository.KeyboardPCB{{ID: "b1", Purchase: repository.KeyboardPurchase{Price: new(45.0)}}},
	}
}

func (s *KeyboardPartLookupSuite) TestKnownIDs_ReturnPartAndPrice() {
	s.Equal(new(35.0), s.kb.Plate(new("p1")).Price())
	s.Equal(new(45.0), s.kb.PCB(new("b1")).Price())
}

func (s *KeyboardPartLookupSuite) TestUnknownOrNilID_ReturnsNilWithNilPrice() {
	s.Nil(s.kb.Plate(new("b1")))
	s.Nil(s.kb.PCB(nil))
	s.Nil(s.kb.Plate(nil).Price())
	s.Nil(s.kb.PCB(new("p1")).Price())
}
