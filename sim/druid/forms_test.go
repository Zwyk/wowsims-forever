package druid

import (
	"testing"

	"github.com/wowsims/tbc/sim/core"
	"github.com/wowsims/tbc/sim/core/proto"
)

func TestFormWeaponsCarryFeralCombatCategory(t *testing.T) {
	druid := &Druid{Talents: &proto.DruidTalents{}}
	druid.Consumables = &proto.ConsumesSpec{}

	catWeapon := druid.GetCatWeapon()
	if catWeapon != core.FeralCombatWeapon(catWeapon) {
		t.Fatal("cat weapon is missing its feral combat category")
	}
	bearWeapon := druid.GetBearWeapon()
	if bearWeapon != core.FeralCombatWeapon(bearWeapon) {
		t.Fatal("bear weapon is missing its feral combat category")
	}
}

func TestPredatoryInstinctsOnlyAppliesInCatOrBearForm(t *testing.T) {
	physicalSpell := &core.Spell{
		SpellSchool:       core.SpellSchoolPhysical,
		CritMultiplierPct: 1,
	}
	magicSpell := &core.Spell{
		SpellSchool:       core.SpellSchoolArcane,
		CritMultiplierPct: 1,
	}
	druid := &Druid{
		Character:    core.Character{Unit: core.Unit{Spellbook: []*core.Spell{physicalSpell, magicSpell}}},
		Talents:      &proto.DruidTalents{PredatoryInstincts: 5},
		CatFormAura:  &core.Aura{},
		BearFormAura: &core.Aura{},
		form:         Humanoid,
	}

	druid.applyPredatoryInstincts()
	assertCritMultiplierPct(t, "humanoid", physicalSpell, 1)

	druid.CatFormAura.OnGain(druid.CatFormAura, &core.Simulation{})
	assertCritMultiplierPct(t, "cat", physicalSpell, 1.1)
	druid.CatFormAura.OnExpire(druid.CatFormAura, &core.Simulation{})
	assertCritMultiplierPct(t, "after cat", physicalSpell, 1)

	druid.BearFormAura.OnGain(druid.BearFormAura, &core.Simulation{})
	assertCritMultiplierPct(t, "bear", physicalSpell, 1.1)
	druid.BearFormAura.OnExpire(druid.BearFormAura, &core.Simulation{})
	assertCritMultiplierPct(t, "after bear", physicalSpell, 1)

	assertCritMultiplierPct(t, "magic", magicSpell, 1)
}

func TestPredatoryInstinctsAppliesToStartingFeralForm(t *testing.T) {
	physicalSpell := &core.Spell{
		SpellSchool:       core.SpellSchoolPhysical,
		CritMultiplierPct: 1,
	}
	druid := &Druid{
		Character:    core.Character{Unit: core.Unit{Spellbook: []*core.Spell{physicalSpell}}},
		Talents:      &proto.DruidTalents{PredatoryInstincts: 5},
		CatFormAura:  &core.Aura{},
		BearFormAura: &core.Aura{},
		form:         Cat,
	}

	druid.applyPredatoryInstincts()
	assertCritMultiplierPct(t, "starting cat", physicalSpell, 1.1)
}

func assertCritMultiplierPct(t *testing.T, form string, spell *core.Spell, want float64) {
	t.Helper()
	if spell.CritMultiplierPct != want {
		t.Fatalf("%s CritMultiplierPct = %v, want %v", form, spell.CritMultiplierPct, want)
	}
}
