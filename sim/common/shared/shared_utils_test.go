package shared

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/wowsims/tbc/sim/core"
)

func TestGetOutcomeMapsRangedNoCrit(t *testing.T) {
	spell := &core.Spell{}
	got := runtime.FuncForPC(reflect.ValueOf(GetOutcome(spell, OutcomeRangedNoCrit)).Pointer()).Name()
	want := runtime.FuncForPC(reflect.ValueOf(spell.OutcomeRangedHit).Pointer()).Name()
	if got != want {
		t.Fatalf("ranged no-crit outcome = %s, want %s", got, want)
	}
}

func TestDamageDefenseTypeDistinguishesExplicitNoneFromUnspecified(t *testing.T) {
	tests := []struct {
		name            string
		defenseType     core.DefenseType
		hasDefenseType  bool
		school          core.SpellSchool
		isMelee         bool
		wantDefenseType core.DefenseType
	}{
		{name: "omitted physical", school: core.SpellSchoolPhysical, wantDefenseType: core.DefenseTypeMelee},
		{name: "omitted magical", school: core.SpellSchoolFire, wantDefenseType: core.DefenseTypeMagic},
		{name: "legacy Felsteel Shield Spike", school: core.SpellSchoolPhysical, isMelee: true, wantDefenseType: core.DefenseTypeMelee},
		{name: "explicit none physical", hasDefenseType: true, school: core.SpellSchoolPhysical, wantDefenseType: core.DefenseTypeNone},
		{name: "explicit none magical", hasDefenseType: true, school: core.SpellSchoolFire, wantDefenseType: core.DefenseTypeNone},
		{name: "legacy explicit magic", defenseType: core.DefenseTypeMagic, school: core.SpellSchoolPhysical, wantDefenseType: core.DefenseTypeMagic},
		{name: "explicit ranged", defenseType: core.DefenseTypeRanged, hasDefenseType: true, school: core.SpellSchoolFire, wantDefenseType: core.DefenseTypeRanged},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := damageDefenseType(test.defenseType, test.hasDefenseType, test.school, test.isMelee)
			if got != test.wantDefenseType {
				t.Fatalf("damageDefenseType() = %d, want %d", got, test.wantDefenseType)
			}
		})
	}
}

func TestDamageOutcomeUsesResolvedDefenseType(t *testing.T) {
	tests := []struct {
		name        string
		defenseType core.DefenseType
		cannotCrit  bool
		outcome     OutcomeType
		want        OutcomeType
	}{
		{name: "none", defenseType: core.DefenseTypeNone, want: OutcomeAlwaysHit},
		{name: "none cannot crit", defenseType: core.DefenseTypeNone, cannotCrit: true, want: OutcomeAlwaysHit},
		{name: "magic", defenseType: core.DefenseTypeMagic, want: OutcomeSpellCanCrit},
		{name: "magic cannot crit", defenseType: core.DefenseTypeMagic, cannotCrit: true, want: OutcomeSpellNoCrit},
		{name: "melee", defenseType: core.DefenseTypeMelee, want: OutcomeMeleeCanCrit},
		{name: "melee cannot crit", defenseType: core.DefenseTypeMelee, cannotCrit: true, want: OutcomeMeleeNoCrit},
		{name: "ranged", defenseType: core.DefenseTypeRanged, want: OutcomeRangedCanCrit},
		{name: "ranged cannot crit", defenseType: core.DefenseTypeRanged, cannotCrit: true, want: OutcomeRangedNoCrit},
		{name: "explicit override", defenseType: core.DefenseTypeNone, cannotCrit: true, outcome: OutcomeSpellNoMissCanCrit, want: OutcomeSpellNoMissCanCrit},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := damageOutcome(test.defenseType, test.cannotCrit, test.outcome)
			if got != test.want {
				t.Fatalf("damageOutcome() = %d, want %d", got, test.want)
			}
		})
	}
}
