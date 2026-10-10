package keyboardparts_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/keyboardparts"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type AssignIDsSuite struct {
	suite.Suite

	next int
}

func TestAssignIDsSuite(t *testing.T) {
	suite.Run(t, new(AssignIDsSuite))
}

func (s *AssignIDsSuite) SetupTest() {
	s.next = 0
}

func (s *AssignIDsSuite) newID() string {
	s.next++
	return "new" + strconv.Itoa(s.next)
}

func (s *AssignIDsSuite) TestCreate_AssignsFreshIDs() {
	kb := repository.Keyboard{
		Plates: []repository.KeyboardPlate{{Material: "AL"}, {Material: "PC"}},
		PCBs:   []repository.KeyboardPCB{{}},
	}

	fieldErrs := keyboardparts.AssignIDs(&kb, nil, s.newID)

	s.Empty(fieldErrs)
	s.Equal("new1", kb.Plates[0].ID)
	s.Equal("new2", kb.Plates[1].ID)
	s.Equal("new3", kb.PCBs[0].ID)
}

func (s *AssignIDsSuite) TestCreate_SentID_ReturnsFieldError() {
	kb := repository.Keyboard{Plates: []repository.KeyboardPlate{{ID: "p1", Material: "AL"}}}

	fieldErrs := keyboardparts.AssignIDs(&kb, nil, s.newID)

	s.Require().Len(fieldErrs, 1)
	s.Equal("plates[0].id", fieldErrs[0].Field)
	s.Equal("p1", fieldErrs[0].Value)
}

func (s *AssignIDsSuite) TestUpdate_KeepsExistingIDsAndAssignsNewOnes() {
	existing := &repository.Keyboard{
		Plates: []repository.KeyboardPlate{{ID: "p1", Material: "AL"}},
		PCBs:   []repository.KeyboardPCB{{ID: "b1"}},
	}
	kb := repository.Keyboard{
		Plates: []repository.KeyboardPlate{{Material: "PC"}, {ID: "p1", Material: "AL"}},
		PCBs:   []repository.KeyboardPCB{{ID: "b1"}},
	}

	fieldErrs := keyboardparts.AssignIDs(&kb, existing, s.newID)

	s.Empty(fieldErrs)
	s.Equal("new1", kb.Plates[0].ID)
	s.Equal("p1", kb.Plates[1].ID)
	s.Equal("b1", kb.PCBs[0].ID)
}

func (s *AssignIDsSuite) TestUpdate_IDOfOtherPartKind_ReturnsFieldError() {
	existing := &repository.Keyboard{
		Plates: []repository.KeyboardPlate{{ID: "p1", Material: "AL"}},
		PCBs:   []repository.KeyboardPCB{{ID: "b1"}},
	}
	kb := repository.Keyboard{PCBs: []repository.KeyboardPCB{{ID: "p1"}}}

	fieldErrs := keyboardparts.AssignIDs(&kb, existing, s.newID)

	s.Require().Len(fieldErrs, 1)
	s.Equal("pcbs[0].id", fieldErrs[0].Field)
}

func (s *AssignIDsSuite) TestUpdate_DuplicateID_ReturnsFieldError() {
	existing := &repository.Keyboard{Plates: []repository.KeyboardPlate{{ID: "p1", Material: "AL"}}}
	kb := repository.Keyboard{
		Plates: []repository.KeyboardPlate{{ID: "p1", Material: "AL"}, {ID: "p1", Material: "AL"}},
	}

	fieldErrs := keyboardparts.AssignIDs(&kb, existing, s.newID)

	s.Require().Len(fieldErrs, 1)
	s.Equal("plates[1].id", fieldErrs[0].Field)
}
