package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/stats"
)

func newInheritedBossOutcomeFixture() (*Unit, *Unit, *Spell, *AttackTable) {
	rules := inheritedTBCRuleset()
	attacker := &Unit{
		Type:        PlayerUnit,
		Level:       rules.levels.characterLevel,
		PseudoStats: stats.NewPseudoStats(),
	}
	defender := &Unit{
		Type:        EnemyUnit,
		Level:       rules.levels.defaultBossLevel(),
		PseudoStats: stats.NewPseudoStats(),
	}
	table := newAttackTableWithRuleset(attacker, defender, rules)
	attacker.AttackTables = []*AttackTable{table}
	spell := &Spell{
		Unit:                   attacker,
		DefenseType:            DefenseTypeMelee,
		ProcMask:               ProcMaskMelee,
		CritMultiplierPct:      1,
		CritMultiplierAdditive: 0,
	}
	return attacker, defender, spell, table
}

func TestInheritedPhysicalAndSpellHitFloors(t *testing.T) {
	attacker, _, spell, table := newInheritedBossOutcomeFixture()

	assertFloat64(t, "base physical miss", spell.GetPhysicalMissChance(table), 0.08)
	attacker.AutoAttacks.IsDualWielding = true
	assertFloat64(t, "dual-wield physical miss", spell.GetPhysicalMissChance(table), 0.27)
	attacker.PseudoStats.DisableDWMissPenalty = true
	assertFloat64(t, "disabled dual-wield penalty", spell.GetPhysicalMissChance(table), 0.08)

	attacker.AutoAttacks.IsDualWielding = false
	attacker.PseudoStats.DisableDWMissPenalty = false
	attacker.stats[stats.PhysicalHitPercent] = 9
	assertFloat64(t, "physical hit after suppression", spell.PhysicalHitChance(table), 0.08)
	assertFloat64(t, "capped physical miss", spell.GetPhysicalMissChance(table), 0)

	attacker.stats[stats.SpellHitPercent] = 0
	assertFloat64(t, "base spell miss", spell.SpellChanceToMiss(table), 0.17)
	attacker.stats[stats.SpellHitPercent] = 16
	assertFloat64(t, "spell miss floor at cap", spell.SpellChanceToMiss(table), 0.01)
	attacker.stats[stats.SpellHitPercent] = 30
	assertFloat64(t, "spell miss floor over cap", spell.SpellChanceToMiss(table), 0.01)
}

func TestInheritedPhysicalAndSpellCritSuppression(t *testing.T) {
	attacker, defender, spell, table := newInheritedBossOutcomeFixture()
	attacker.stats[stats.PhysicalCritPercent] = 10
	attacker.stats[stats.SpellCritPercent] = 10

	assertFloat64(t, "physical crit", spell.PhysicalCritChance(table), 0.052)
	assertFloat64(t, "spell crit", spell.SpellCritChance(defender), 0.079)
}

func TestInheritedWhiteAndSpecialOutcomeTables(t *testing.T) {
	attacker, defender, spell, table := newInheritedBossOutcomeFixture()
	defender.stats[stats.BlockValue] = 20

	attacker.PseudoStats.InFrontOfTarget = true
	frontWhite := &SpellResult{Target: defender, Damage: 100}
	spell.OutcomeExpectedMeleeWhite(nil, frontWhite, table)
	assertFloat64(t, "front white damage", frontWhite.Damage, 64.5)

	attacker.PseudoStats.InFrontOfTarget = false
	rearWhite := &SpellResult{Target: defender, Damage: 100}
	spell.OutcomeExpectedMeleeWhite(nil, rearWhite, table)
	assertFloat64(t, "rear white damage", rearWhite.Damage, 79.5)

	attacker.AutoAttacks.IsDualWielding = true
	dualWieldWhite := &SpellResult{Target: defender, Damage: 100}
	spell.OutcomeExpectedMeleeWhite(nil, dualWieldWhite, table)
	assertFloat64(t, "rear dual-wield white damage", dualWieldWhite.Damage, 60.5)

	dualWieldSpecial := &SpellResult{Target: defender, Damage: 100}
	spell.OutcomeExpectedMeleeWeaponSpecialHitAndCrit(nil, dualWieldSpecial, table)
	assertFloat64(t, "rear dual-wield special damage", dualWieldSpecial.Damage, 85.5)
}

func TestInheritedWhiteCritCapAndSpecialCritOrdering(t *testing.T) {
	attacker, defender, spell, table := newInheritedBossOutcomeFixture()
	attacker.stats[stats.PhysicalCritPercent] = 100

	white := &SpellResult{Target: defender, Damage: 100}
	spell.OutcomeExpectedMeleeWhite(nil, white, table)
	assertFloat64(t, "white one-roll crit cap", white.Damage, 141)

	attacker.AutoAttacks.IsDualWielding = true
	special := &SpellResult{Target: defender, Damage: 100}
	spell.OutcomeExpectedMeleeWeaponSpecialHitAndCrit(nil, special, table)
	assertFloat64(t, "special separate crit roll", special.Damage, 166.896)
}
