package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func TestInheritedTBCLevelBaseline(t *testing.T) {
	rules := inheritedTBCRuleset()
	if rules.id != rulesetInheritedTBC {
		t.Fatalf("ruleset: got %d, want inherited TBC", rules.id)
	}
	if rules.levels.characterLevel != 70 {
		t.Fatalf("character level: got %d, want 70", rules.levels.characterLevel)
	}
	if rules.levels.defaultBossLevel() != 73 {
		t.Fatalf("default boss level: got %d, want 73", rules.levels.defaultBossLevel())
	}
}

// These tests intentionally pin the inherited TBC combat table before any
// Forever formulas are installed. A later rules-data change should update the
// table and these expectations together, using beta evidence.
func TestInheritedPlayerVsEnemyAttackTables(t *testing.T) {
	rules := inheritedTBCRuleset()
	player := Unit{Type: PlayerUnit, Level: rules.levels.characterLevel}

	tests := []struct {
		name                 string
		level                int32
		spellMiss            float64
		physicalMiss         float64
		block                float64
		dodge                float64
		parry                float64
		glance               float64
		glanceMultiplier     float64
		hitSuppression       float64
		meleeCritSuppression float64
		spellCritSuppression float64
	}{
		{"minus-two", rules.levels.characterLevel - 2, 0.02, 0.04, 0.05, 0.04, 0.04, 0.00, 0.95, 0.00, 0.000, 0.000},
		{"same-level", rules.levels.characterLevel, 0.04, 0.05, 0.05, 0.05, 0.05, 0.06, 0.95, 0.00, 0.000, 0.000},
		{"plus-one", rules.levels.characterLevel + 1, 0.05, 0.055, 0.05, 0.055, 0.055, 0.12, 0.95, 0.00, 0.010, 0.000},
		{"plus-two", rules.levels.characterLevel + 2, 0.06, 0.06, 0.05, 0.06, 0.06, 0.18, 0.85, 0.00, 0.020, 0.003},
		{"boss", rules.levels.defaultBossLevel(), 0.17, 0.08, 0.05, 0.065, 0.14, 0.24, 0.75, 0.01, 0.048, 0.021},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			enemy := Unit{Type: EnemyUnit, Level: test.level}
			table := newAttackTableWithRuleset(&player, &enemy, rules)

			assertFloat64(t, "spell miss", table.BaseSpellMissChance, test.spellMiss)
			assertFloat64(t, "physical miss", table.BaseMissChance, test.physicalMiss)
			assertFloat64(t, "block", table.BaseBlockChance, test.block)
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
	rules := inheritedTBCRuleset()
	player := Unit{Type: PlayerUnit, Level: rules.levels.characterLevel}

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
		{"minus-two", rules.levels.characterLevel - 2, 0.05, 0.054, 0.054, 0.004, 0.054, 0.00},
		{"same-level", rules.levels.characterLevel, 0.05, 0.050, 0.050, 0.000, 0.050, 0.00},
		{"plus-one", rules.levels.characterLevel + 1, 0.05, 0.048, 0.048, -0.002, 0.048, 0.00},
		{"plus-two", rules.levels.characterLevel + 2, 0.05, 0.046, 0.046, -0.004, 0.046, 0.00},
		{"boss", rules.levels.defaultBossLevel(), 0.05, 0.044, 0.044, -0.006, 0.044, 0.15},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			enemy := Unit{Type: EnemyUnit, Level: test.level}
			table := newAttackTableWithRuleset(&enemy, &player, rules)

			assertFloat64(t, "spell miss", table.BaseSpellMissChance, test.spellMiss)
			assertFloat64(t, "physical miss", table.BaseMissChance, test.physicalMiss)
			assertFloat64(t, "block", table.BaseBlockChance, test.block)
			assertFloat64(t, "dodge", table.BaseDodgeChance, test.dodge)
			assertFloat64(t, "parry", table.BaseParryChance, test.parry)
			assertFloat64(t, "crush", table.BaseCrushChance, test.crush)
		})
	}
}

func TestInheritedDefaultTargetUsesBossLevel(t *testing.T) {
	rules := inheritedTBCRuleset()
	target := newTargetWithRuleset(&proto.Target{}, 0, rules)
	if target.Level != rules.levels.defaultBossLevel() {
		t.Fatalf("default target level: got %d, want %d", target.Level, rules.levels.defaultBossLevel())
	}
}

func TestInheritedExpertiseQuarterPercentFloor(t *testing.T) {
	rules := inheritedTBCRuleset()
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
		{"many-steps-preserve-division", rules.ratings.expertisePerQuarterPercentReduction * 35.5, 0, float64(35) / 400},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unit := Unit{stats: stats.Stats{stats.ExpertiseRating: test.rating}}
			spell := Spell{Unit: &unit, BonusExpertiseRating: test.bonusRating}
			got := spell.dodgeParrySuppression(rules.ratings, rules.combat.outcomes)
			if test.name == "many-steps-preserve-division" && got != test.want {
				t.Fatalf("avoidance suppression changed floating-point operation: got %x, want %x", got, test.want)
			}
			assertFloat64(t, "avoidance suppression", got, test.want)
		})
	}
}

func TestInheritedDodgeReductionRemainsSeparateFromParry(t *testing.T) {
	rules := inheritedTBCRuleset()
	attacker := Unit{
		Type:        PlayerUnit,
		Level:       rules.levels.characterLevel,
		stats:       stats.Stats{stats.ExpertiseRating: 3.942308},
		PseudoStats: stats.NewPseudoStats(),
	}
	attacker.PseudoStats.InFrontOfTarget = true
	attacker.PseudoStats.DodgeReduction = 0.02
	defender := Unit{
		Type:        EnemyUnit,
		Level:       rules.levels.defaultBossLevel(),
		PseudoStats: stats.NewPseudoStats(),
	}
	table := newAttackTableWithRuleset(&attacker, &defender, rules)
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
