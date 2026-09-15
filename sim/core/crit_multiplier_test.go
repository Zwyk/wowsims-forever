package core

import (
	"math"
	"testing"

	"github.com/wowsims/tbc/sim/core/stats"
)

func TestCritDamageMultiplierUsesDefenseTypeBase(t *testing.T) {
	unit := &Unit{PseudoStats: stats.NewPseudoStats()}
	tests := []struct {
		name        string
		defenseType DefenseType
		want        float64
	}{
		{name: "magic", defenseType: DefenseTypeMagic, want: 1.5},
		{name: "melee", defenseType: DefenseTypeMelee, want: 2},
		{name: "ranged", defenseType: DefenseTypeRanged, want: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spell := &Spell{
				Unit:              unit,
				DefenseType:       test.defenseType,
				CritMultiplierPct: 1,
			}
			if got := spell.CritDamageMultiplier(&AttackTable{CritMultiplier: 1}); math.Abs(got-test.want) > 1e-9 {
				t.Fatalf("CritDamageMultiplier() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestCritDamageMultiplierUsesAttackTableRuleset(t *testing.T) {
	rules := currentRuleset()
	rules.combat.outcomes.magicCritDamageMultiplier = 1.6
	rules.combat.outcomes.meleeCritDamageMultiplier = 2.1
	rules.combat.outcomes.rangedCritDamageMultiplier = 2.2
	table := &AttackTable{
		outcomes:         rules.combat.outcomes,
		rulesInitialized: true,
		CritMultiplier:   1,
	}
	unit := &Unit{PseudoStats: stats.NewPseudoStats()}
	tests := []struct {
		name        string
		defenseType DefenseType
		want        float64
	}{
		{name: "magic", defenseType: DefenseTypeMagic, want: 1.6},
		{name: "melee", defenseType: DefenseTypeMelee, want: 2.1},
		{name: "ranged", defenseType: DefenseTypeRanged, want: 2.2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spell := &Spell{
				Unit:              unit,
				DefenseType:       test.defenseType,
				CritMultiplierPct: 1,
			}
			if got := spell.CritDamageMultiplier(table); math.Abs(got-test.want) > 1e-9 {
				t.Fatalf("CritDamageMultiplier() = %v, want injected %v", got, test.want)
			}
		})
	}
}

func TestCritDamageMultiplierTreatsMissingAttackTableAsNeutralForHealing(t *testing.T) {
	spell := &Spell{
		Unit:              &Unit{PseudoStats: stats.NewPseudoStats()},
		DefenseType:       DefenseTypeMagic,
		CritMultiplierPct: 1,
	}

	if got, want := spell.CritDamageMultiplier(nil), 1.5; math.Abs(got-want) > 1e-9 {
		t.Fatalf("CritDamageMultiplier(nil) = %v, want %v", got, want)
	}
	if got, want := spell.CritDamageMultiplier(&AttackTable{CritMultiplier: 1.05}), 1.575; math.Abs(got-want) > 1e-9 {
		t.Fatalf("CritDamageMultiplier(attack table) = %v, want %v", got, want)
	}
}

func TestOutcomeHealingCritUsesNeutralAttackTableMultiplier(t *testing.T) {
	unit := &Unit{
		stats:       stats.Stats{stats.SpellCritPercent: 100},
		PseudoStats: stats.NewPseudoStats(),
	}
	spell := &Spell{
		Unit:              unit,
		DefenseType:       DefenseTypeMagic,
		CritMultiplierPct: 1,
		SpellMetrics:      make([]SpellMetrics, 1),
	}
	result := &SpellResult{Target: &Unit{UnitIndex: 0}, Damage: 100}

	spell.OutcomeHealingCrit(
		&Simulation{rand: NewSplitMix(1)},
		result,
		&AttackTable{CritMultiplier: 1.05},
	)

	if !result.DidCrit() {
		t.Fatal("100% healing crit chance did not crit")
	}
	if got, want := result.Damage, 150.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("healing crit damage = %v, want %v", got, want)
	}
}

func TestCritDamageMultiplierRejectsInvalidDefenseType(t *testing.T) {
	spell := &Spell{
		Unit:              &Unit{PseudoStats: stats.NewPseudoStats()},
		DefenseType:       DefenseType(DefenseTypeLen + 1),
		CritMultiplierPct: 1,
	}

	defer func() {
		if recover() == nil {
			t.Fatal("CritDamageMultiplier() accepted an invalid DefenseType")
		}
	}()
	spell.CritDamageMultiplier(nil)
}
