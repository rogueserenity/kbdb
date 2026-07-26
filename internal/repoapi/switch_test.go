package repoapi

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

func strPtr(s string) *string     { return &s }
func intPtr(i int) *int           { return &i }
func floatPtr(f float64) *float64 { return &f }
func boolPtr(b bool) *bool        { return &b }

func fullRepoSwitch() repository.Switch {
	return repository.Switch{
		UserID:       "alice",
		ID:           "sw1",
		Brand:        "Gateron",
		Manufacturer: strPtr("Gateron Inc"),
		Name:         "Yellow",
		Type:         "Linear",
		Pins:         intPtr(5),
		FactoryLubed: boolPtr(true),
		Material: repository.SwitchMaterial{
			TopHousing:    strPtr("PC"),
			BottomHousing: strPtr("Nylon"),
			Stem:          strPtr("POM"),
		},
		Force: repository.SwitchForce{
			Actuation: floatPtr(50),
			BottomOut: floatPtr(60),
		},
		Spring: repository.SwitchSpring{
			Material:    strPtr("Steel"),
			PreTravel:   floatPtr(2),
			TotalTravel: floatPtr(4),
		},
		Purchase: repository.SwitchPurchase{
			Vendor:   strPtr("CannonKeys"),
			Price:    floatPtr(0.35),
			Quantity: intPtr(90),
		},
		Notes:      strPtr("smooth"),
		Visibility: repository.VisibilityPrivate,
	}
}

func TestSwitchToAPI_FullRoundTrip_PreservesEveryField(t *testing.T) {
	sw := fullRepoSwitch()
	out := SwitchToAPI(sw)

	assert.Equal(t, sw.ID, out.Id)
	assert.Equal(t, sw.Brand, out.Brand)
	assert.Equal(t, sw.Manufacturer, out.Manufacturer)
	assert.Equal(t, sw.Name, out.Name)
	assert.Equal(t, sw.Type, out.Type)
	assert.Equal(t, sw.Pins, out.Pins)
	assert.Equal(t, sw.FactoryLubed, out.FactoryLubed)
	assert.Equal(t, sw.Notes, out.Notes)
	assert.Equal(t, api.Visibility(sw.Visibility), out.Visibility)

	if assert.NotNil(t, out.Material) {
		assert.Equal(t, sw.Material.TopHousing, out.Material.TopHousing)
		assert.Equal(t, sw.Material.BottomHousing, out.Material.BottomHousing)
		assert.Equal(t, sw.Material.Stem, out.Material.Stem)
	}
	if assert.NotNil(t, out.Force) {
		assert.Equal(t, sw.Force.Actuation, out.Force.Actuation)
		assert.Equal(t, sw.Force.BottomOut, out.Force.BottomOut)
	}
	if assert.NotNil(t, out.Spring) {
		assert.Equal(t, sw.Spring.Material, out.Spring.Material)
		assert.Equal(t, sw.Spring.PreTravel, out.Spring.PreTravel)
		assert.Equal(t, sw.Spring.TotalTravel, out.Spring.TotalTravel)
	}
	if assert.NotNil(t, out.Purchase) {
		assert.Equal(t, sw.Purchase.Vendor, out.Purchase.Vendor)
		assert.Equal(t, sw.Purchase.Price, out.Purchase.Price)
		assert.Equal(t, sw.Purchase.Quantity, out.Purchase.Quantity)
	}
}

func TestSwitchToAPI_AllOptionalFieldsNil_SubStructsOmitted(t *testing.T) {
	sw := repository.Switch{ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: repository.VisibilityPrivate}

	out := SwitchToAPI(sw)

	assert.Nil(t, out.Manufacturer)
	assert.Nil(t, out.Pins)
	assert.Nil(t, out.FactoryLubed)
	assert.Nil(t, out.Notes)
	assert.Nil(t, out.Material, "an all-nil SwitchMaterial must map to a nil pointer, not an empty object")
	assert.Nil(t, out.Force, "an all-nil SwitchForce must map to a nil pointer, not an empty object")
	assert.Nil(t, out.Spring, "an all-nil SwitchSpring must map to a nil pointer, not an empty object")
	assert.Nil(t, out.Purchase, "an all-nil SwitchPurchase must map to a nil pointer, not an empty object")
}

func TestSwitchToAPI_OneFieldSetInSubStruct_SubStructPresent(t *testing.T) {
	sw := repository.Switch{
		ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: repository.VisibilityPrivate,
		Material: repository.SwitchMaterial{Stem: strPtr("POM")},
	}

	out := SwitchToAPI(sw)

	if assert.NotNil(t, out.Material) {
		assert.Nil(t, out.Material.TopHousing)
		assert.Nil(t, out.Material.BottomHousing)
		assert.Equal(t, strPtr("POM"), out.Material.Stem)
	}
}

func TestSwitchToRepo_FullRoundTrip_PreservesEveryField(t *testing.T) {
	in := api.SwitchInput{
		Brand:        "Gateron",
		Manufacturer: strPtr("Gateron Inc"),
		Name:         "Yellow",
		Type:         "Linear",
		Pins:         intPtr(5),
		FactoryLubed: boolPtr(true),
		Material: &api.SwitchMaterial{
			TopHousing:    strPtr("PC"),
			BottomHousing: strPtr("Nylon"),
			Stem:          strPtr("POM"),
		},
		Force: &api.SwitchForce{
			Actuation: floatPtr(50),
			BottomOut: floatPtr(60),
		},
		Spring: &api.SwitchSpring{
			Material:    strPtr("Steel"),
			PreTravel:   floatPtr(2),
			TotalTravel: floatPtr(4),
		},
		Purchase: &api.SwitchPurchase{
			Vendor:   strPtr("CannonKeys"),
			Price:    floatPtr(0.35),
			Quantity: intPtr(90),
		},
		Notes:      strPtr("smooth"),
		Visibility: api.Private,
	}

	sw := SwitchToRepo(in)

	assert.Empty(t, sw.UserID, "SwitchToRepo must not set UserID - that's the handler's job")
	assert.Empty(t, sw.ID, "SwitchToRepo must not set ID - that's the handler's job")
	assert.Equal(t, in.Brand, sw.Brand)
	assert.Equal(t, in.Manufacturer, sw.Manufacturer)
	assert.Equal(t, in.Name, sw.Name)
	assert.Equal(t, in.Type, sw.Type)
	assert.Equal(t, in.Pins, sw.Pins)
	assert.Equal(t, in.FactoryLubed, sw.FactoryLubed)
	assert.Equal(t, in.Notes, sw.Notes)
	assert.Equal(t, repository.Visibility(in.Visibility), sw.Visibility)

	assert.Equal(t, in.Material.TopHousing, sw.Material.TopHousing)
	assert.Equal(t, in.Material.BottomHousing, sw.Material.BottomHousing)
	assert.Equal(t, in.Material.Stem, sw.Material.Stem)
	assert.Equal(t, in.Force.Actuation, sw.Force.Actuation)
	assert.Equal(t, in.Force.BottomOut, sw.Force.BottomOut)
	assert.Equal(t, in.Spring.Material, sw.Spring.Material)
	assert.Equal(t, in.Spring.PreTravel, sw.Spring.PreTravel)
	assert.Equal(t, in.Spring.TotalTravel, sw.Spring.TotalTravel)
	assert.Equal(t, in.Purchase.Vendor, sw.Purchase.Vendor)
	assert.Equal(t, in.Purchase.Price, sw.Purchase.Price)
	assert.Equal(t, in.Purchase.Quantity, sw.Purchase.Quantity)
}

func TestSwitchToRepo_NilSubStructs_ProduceZeroValueStructs(t *testing.T) {
	in := api.SwitchInput{Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: api.Private}

	sw := SwitchToRepo(in)

	assert.Equal(t, repository.SwitchMaterial{}, sw.Material)
	assert.Equal(t, repository.SwitchForce{}, sw.Force)
	assert.Equal(t, repository.SwitchSpring{}, sw.Spring)
	assert.Equal(t, repository.SwitchPurchase{}, sw.Purchase)
}

func TestSwitchToAPISummary_MapsOnlySummaryFields(t *testing.T) {
	sw := fullRepoSwitch()

	summary := SwitchToAPISummary(sw)

	assert.Equal(t, &sw.ID, summary.Id)
	assert.Equal(t, &sw.Brand, summary.Brand)
	assert.Equal(t, &sw.Name, summary.Name)
	assert.Equal(t, &sw.Type, summary.Type)
}
