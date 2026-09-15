package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

// These tests intentionally pin the inherited TBC combat table before any
// Forever formulas are installed. A later rules-data change should update the
// table and these expectations together, using beta evidence.
func TestInheritedPlayerVsEnemyAttackTables(t *testing.T) {
	player := Unit{Type: PlayerUnit, Level: CharacterLevel}

	tests := []struct {
		name                 string
		level                int32
		spellMiss            float64
		physicalMiss         float64
		dodge                float64
		parry                float64
		glance               float64
		glanceMultiplier     float64
		hitSuppression       float64
		meleeCritSuppression float64
		spellCritSuppression float64
	}{
		{"minus-two", CharacterLevel - 2, 0.02, 0.04, 0.04, 0.04, 0.00, 0.95, 0.00, 0.000, 0.000},
		{"same-level", CharacterLevel, 0.04, 0.05, 0.05, 0.05, 0.06, 0.95, 0.00, 0.000, 0.000},
		{"plus-one", CharacterLevel + 1, 0.05, 0.055, 0.055, 0.055, 0.12, 0.95, 0.00, 0.010, 0.000},
		{"plus-two", CharacterLevel + 2, 0.06, 0.06, 0.06, 0.06, 0.18, 0.85, 0.00, 0.020, 0.003},
		{"boss", DefaultBossLevel, 0.17, 0.08, 0.065, 0.14, 0.24, 0.75, 0.01, 0.048, 0.021},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			enemy := Unit{Type: EnemyUnit, Level: test.level}
			table := NewAttackTable(&player, &enemy)

			assertFloat64(t, "spell miss", table.BaseSpellMissChance, test.spellMiss)
			assertFloat64(t, "physical miss", table.BaseMissChance, test.physicalMiss)
			assertFloat64(t, "dodge", table.BaseDodgeChance, test.dodge)
			assertFloat64(t, "parry", table.BaseParryChance, test.parry)
			assertFloat64(t, "glance", table.BaseGlanceChance, test.glance)
			assertFloat64(t, "glance multiplier", table.GlanceMultiplier, test.glanceMultiplier)
			assertFloat64(t, "hit suppression", table.HitSuppression, test.hitSuppression)
			assertFloat64(t, "melee crit suppression", table.MeleeCritSuppression, test.meleeCritSuppression)
			assertFloat64(t, "spell crit suppression", table.SpellCritSuppression, test.spellCritSuppression)
		})
	}
}

func TestDefaultTargetUsesBossLevel(t *testing.T) {
	target := NewTarget(&proto.Target{}, 0)
	if target.Level != DefaultBossLevel {
		t.Fatalf("default target level: got %d, want %d", target.Level, DefaultBossLevel)
	}
}

func TestInheritedExpertiseQuarterPercentFloor(t *testing.T) {
	tests := []struct {
		name   string
		rating float64
		want   float64
	}{
		{"below-first-step", ExpertisePerQuarterPercentReduction - 0.000001, 0},
		{"first-step", ExpertisePerQuarterPercentReduction, 0.0025},
		{"below-second-step", 2*ExpertisePerQuarterPercentReduction - 0.000001, 0.0025},
		{"second-step", 2 * ExpertisePerQuarterPercentReduction, 0.005},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unit := Unit{stats: stats.Stats{stats.ExpertiseRating: test.rating}}
			spell := Spell{Unit: &unit}
			assertFloat64(t, "avoidance suppression", spell.DodgeParrySuppression(), test.want)
		})
	}
}

func assertFloat64(t *testing.T, field string, got float64, want float64) {
	t.Helper()
	if !WithinToleranceFloat64(want, got, 0.0000001) {
		t.Errorf("%s: got %.7f, want %.7f", field, got, want)
	}
}
