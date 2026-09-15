package core

import (
	"math"
	"testing"

	"github.com/wowsims/tbc/sim/core/stats"
)

func TestClassicReferenceArmorDamageModifierVectors(t *testing.T) {
	rules := classicReferenceArmorTestRuleset()
	tests := []struct {
		name                 string
		attackerLevel        int32
		defenderArmor        float64
		flatArmorPenetration float64
		want                 float64
	}{
		{name: "level one lower boundary", attackerLevel: 1, defenderArmor: 485, want: 0.5},
		{name: "zero armor", attackerLevel: 60, want: 1},
		{name: "default boss armor", attackerLevel: 60, defenderArmor: 3731, want: 0.5958184378723865},
		{name: "flat penetration", attackerLevel: 60, defenderArmor: 7685, flatArmorPenetration: 1000, want: 0.4513746409519902},
		{name: "half damage anchor", attackerLevel: 60, defenderArmor: 5500, want: 0.5},
		{name: "seventy five percent reduction", attackerLevel: 60, defenderArmor: 16500, want: 0.25},
		{name: "no source cap", attackerLevel: 60, defenderArmor: 22000, want: 0.2},
		{name: "over penetration", attackerLevel: 60, defenderArmor: 3731, flatArmorPenetration: 5000, want: 1},
		{name: "level sixty three attacker", attackerLevel: 63, defenderArmor: 3731, want: 0.6066835336285052},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := classicReferenceArmorDamageModifier(rules, classicReferenceArmorInput{
				attackerLevel:        test.attackerLevel,
				defenderArmor:        test.defenderArmor,
				flatArmorPenetration: test.flatArmorPenetration,
			})
			if !ok {
				t.Fatal("Classic reference armor policy rejected a supported vector")
			}
			assertFloat64(t, "damage modifier", got, test.want)
		})
	}
}

func TestClassicReferenceArmorKeepsFiniteFractionalInputs(t *testing.T) {
	rules := classicReferenceArmorTestRuleset()
	got, ok := classicReferenceArmorDamageModifier(rules, classicReferenceArmorInput{
		attackerLevel:        60,
		defenderArmor:        3731.5,
		flatArmorPenetration: 0.5,
	})
	if !ok {
		t.Fatal("Classic reference armor policy rejected finite fractional inputs")
	}
	assertFloat64(t, "fractional damage modifier", got, 0.5958184378723865)
}

func TestClassicReferenceArmorScopeFailsClosed(t *testing.T) {
	validRules := classicReferenceArmorTestRuleset()
	tests := []struct {
		name  string
		rules rulesetProfile
		input classicReferenceArmorInput
	}{
		{name: "active inherited profile", rules: currentRuleset(), input: classicReferenceArmorInput{attackerLevel: 60}},
		{name: "wrong boss delta", rules: rulesetWithLevelAndBossDelta(60, 2), input: classicReferenceArmorInput{attackerLevel: 60}},
		{name: "zero attacker level", rules: validRules, input: classicReferenceArmorInput{}},
		{name: "negative attacker level", rules: validRules, input: classicReferenceArmorInput{attackerLevel: -1}},
		{name: "attacker above default boss", rules: validRules, input: classicReferenceArmorInput{attackerLevel: 64}},
		{name: "negative armor", rules: validRules, input: classicReferenceArmorInput{attackerLevel: 60, defenderArmor: -1}},
		{name: "negative penetration", rules: validRules, input: classicReferenceArmorInput{attackerLevel: 60, flatArmorPenetration: -1}},
		{name: "armor not a number", rules: validRules, input: classicReferenceArmorInput{attackerLevel: 60, defenderArmor: math.NaN()}},
		{name: "armor positive infinity", rules: validRules, input: classicReferenceArmorInput{attackerLevel: 60, defenderArmor: math.Inf(1)}},
		{name: "armor negative infinity", rules: validRules, input: classicReferenceArmorInput{attackerLevel: 60, defenderArmor: math.Inf(-1)}},
		{name: "penetration not a number", rules: validRules, input: classicReferenceArmorInput{attackerLevel: 60, flatArmorPenetration: math.NaN()}},
		{name: "penetration positive infinity", rules: validRules, input: classicReferenceArmorInput{attackerLevel: 60, flatArmorPenetration: math.Inf(1)}},
		{name: "penetration negative infinity", rules: validRules, input: classicReferenceArmorInput{attackerLevel: 60, flatArmorPenetration: math.Inf(-1)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := classicReferenceArmorDamageModifier(test.rules, test.input)
			if ok {
				t.Fatalf("unsupported input resolved to %.7f", got)
			}
			if got != 0 {
				t.Fatalf("rejected input returned %.7f, want zero", got)
			}
		})
	}
}

func TestClassicReferenceArmorPolicyIsInactive(t *testing.T) {
	active := currentRuleset()
	if active != inheritedTBCRuleset() {
		t.Fatal("current ruleset differs from inherited TBC")
	}
	if _, ok := classicReferenceArmorDamageModifier(active, classicReferenceArmorInput{attackerLevel: active.levels.characterLevel}); ok {
		t.Fatal("Classic reference armor policy accepted the active inherited profile")
	}

	attacker := Unit{Type: PlayerUnit, Level: active.levels.characterLevel}
	defender := Unit{
		Type:         EnemyUnit,
		Level:        active.levels.defaultBossLevel(),
		initialStats: stats.Stats{stats.Armor: 50000},
		PseudoStats:  stats.NewPseudoStats(),
	}
	defender.stats = defender.initialStats
	assertFloat64(t, "active inherited cap", NewAttackTable(&attacker, &defender).GetArmorDamageModifier(nil), 0.25)
}

func classicReferenceArmorTestRuleset() rulesetProfile {
	rules := inheritedTBCRuleset()
	rules.levels.characterLevel = classicReferenceCharacterLevel
	rules.levels.defaultBossLevelDelta = classicReferenceDefaultBossLevelDelta
	return rules
}

func rulesetWithLevelAndBossDelta(characterLevel int32, bossLevelDelta int32) rulesetProfile {
	rules := inheritedTBCRuleset()
	rules.levels.characterLevel = characterLevel
	rules.levels.defaultBossLevelDelta = bossLevelDelta
	return rules
}
