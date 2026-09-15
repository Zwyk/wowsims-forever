package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/stats"
)

func TestInheritedBossResistanceTable(t *testing.T) {
	attacker := Unit{
		Type:        PlayerUnit,
		Level:       CharacterLevel,
		PseudoStats: stats.NewPseudoStats(),
	}
	defender := Unit{
		Type:        EnemyUnit,
		Level:       DefaultBossLevel,
		PseudoStats: stats.NewPseudoStats(),
	}
	spell := Spell{
		Unit:        &attacker,
		SpellSchool: SpellSchoolFire,
		SchoolIndex: stats.SchoolIndexFire,
	}
	table := NewAttackTable(&attacker, &defender)

	threshold00, threshold25, threshold50 := table.GetPartialResistThresholds(&spell)
	assertFloat64(t, "level-only 0% threshold", threshold00, 0.1368)
	assertFloat64(t, "level-only 25% threshold", threshold25, 0.0378)
	assertFloat64(t, "level-only 50% threshold", threshold50, 0.0054)
	assertFloat64(t, "level-only binary hit", table.GetBinaryHitChance(&spell), 1)

	defender.stats[stats.FireResistance] = 100
	threshold00, threshold25, threshold50 = table.GetPartialResistThresholds(&spell)
	assertFloat64(t, "100 resistance 0% threshold", threshold00, 0.7833142857142857)
	assertFloat64(t, "100 resistance 25% threshold", threshold25, 0.2653714285714286)
	assertFloat64(t, "100 resistance 50% threshold", threshold50, 0.0484571428571429)
	assertFloat64(t, "100 resistance binary hit", table.GetBinaryHitChance(&spell), 0.7857142857142857)

	spell.Flags = SpellFlagBinary
	assertFloat64(t, "100 resistance binary miss", spell.SpellChanceToMiss(table), 0.3478571428571429)
	attacker.stats[stats.SpellHitPercent] = 16
	assertFloat64(t, "100 resistance binary miss with hit", spell.SpellChanceToMiss(table), 0.18785714285714283)
	attacker.stats[stats.SpellHitPercent] = 35
	assertFloat64(t, "100 resistance binary miss floor", spell.SpellChanceToMiss(table), 0.01)
}
