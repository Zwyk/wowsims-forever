package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func TestInheritedTBCLevelBaseline(t *testing.T) {
	if CharacterLevel != 70 {
		t.Fatalf("character level: got %d, want 70", CharacterLevel)
	}
	if DefaultBossLevel != 73 {
		t.Fatalf("default boss level: got %d, want 73", DefaultBossLevel)
	}
}

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

func TestInheritedEnemyVsPlayerAttackTables(t *testing.T) {
	player := Unit{Type: PlayerUnit, Level: CharacterLevel}

	tests := []struct {
		name         string
		level        int32
		spellMiss    float64
		physicalMiss float64
		block        float64
		dodge        float64
		parry        float64
		crush        float64
	}{
		{"minus-two", CharacterLevel - 2, 0.05, 0.054, 0.054, 0.004, 0.054, 0.00},
		{"same-level", CharacterLevel, 0.05, 0.050, 0.050, 0.000, 0.050, 0.00},
		{"plus-one", CharacterLevel + 1, 0.05, 0.048, 0.048, -0.002, 0.048, 0.00},
		{"plus-two", CharacterLevel + 2, 0.05, 0.046, 0.046, -0.004, 0.046, 0.00},
		{"boss", DefaultBossLevel, 0.05, 0.044, 0.044, -0.006, 0.044, 0.15},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			enemy := Unit{Type: EnemyUnit, Level: test.level}
			table := NewAttackTable(&enemy, &player)

			assertFloat64(t, "spell miss", table.BaseSpellMissChance, test.spellMiss)
			assertFloat64(t, "physical miss", table.BaseMissChance, test.physicalMiss)
			assertFloat64(t, "block", table.BaseBlockChance, test.block)
			assertFloat64(t, "dodge", table.BaseDodgeChance, test.dodge)
			assertFloat64(t, "parry", table.BaseParryChance, test.parry)
			assertFloat64(t, "crush", table.BaseCrushChance, test.crush)
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
		name        string
		rating      float64
		bonusRating float64
		want        float64
	}{
		{"below-first-step", 3.942307, 0, 0},
		{"first-step", 3.942308, 0, 0.0025},
		{"below-second-step", 7.884615, 0, 0.0025},
		{"second-step", 7.884616, 0, 0.005},
		{"spell-bonus-rating", 0, 3.942308, 0.0025},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unit := Unit{stats: stats.Stats{stats.ExpertiseRating: test.rating}}
			spell := Spell{Unit: &unit, BonusExpertiseRating: test.bonusRating}
			assertFloat64(t, "avoidance suppression", spell.DodgeParrySuppression(), test.want)
		})
	}
}

func TestInheritedDodgeReductionRemainsSeparateFromParry(t *testing.T) {
	attacker := Unit{
		Type:        PlayerUnit,
		Level:       CharacterLevel,
		stats:       stats.Stats{stats.ExpertiseRating: 3.942308},
		PseudoStats: stats.NewPseudoStats(),
	}
	attacker.PseudoStats.InFrontOfTarget = true
	attacker.PseudoStats.DodgeReduction = 0.02
	defender := Unit{
		Type:        EnemyUnit,
		Level:       DefaultBossLevel,
		PseudoStats: stats.NewPseudoStats(),
	}
	table := NewAttackTable(&attacker, &defender)
	spell := Spell{Unit: &attacker}

	assertFloat64(t, "dodge", defender.GetTotalDodgeChanceAsDefender(&spell, table), 0.0425)
	assertFloat64(t, "parry", defender.GetTotalParryChanceAsDefender(&spell, table), 0.1375)
}

func assertFloat64(t *testing.T, field string, got float64, want float64) {
	t.Helper()
	if !WithinToleranceFloat64(want, got, 0.0000001) {
		t.Errorf("%s: got %.7f, want %.7f", field, got, want)
	}
}
