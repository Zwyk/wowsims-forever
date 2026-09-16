package core

import (
	"math"
	"testing"

	"github.com/wowsims/tbc/sim/core/stats"
)

func TestClassicReferencePartialResistLevelVectors(t *testing.T) {
	rules := classicReferenceResistanceTestRuleset()
	tests := []struct {
		defenderLevel int32
		want          classicReferencePartialResistView
	}{
		{defenderLevel: 60, want: classicReferencePartialResistView{}},
		{defenderLevel: 61, want: classicReferencePartialResistView{coefficient: 0.02666666666666667, threshold00: 0.0608, threshold25: 0.0168, threshold50: 0.0024}},
		{defenderLevel: 62, want: classicReferencePartialResistView{coefficient: 0.05333333333333334, threshold00: 0.1216, threshold25: 0.0336, threshold50: 0.0048}},
		{defenderLevel: 63, want: classicReferencePartialResistView{coefficient: 0.08, threshold00: 0.1824, threshold25: 0.0504, threshold50: 0.0072}},
	}

	for _, test := range tests {
		got, ok := classicReferencePartialResistProjection(rules, classicReferenceResistanceInput{
			attackerLevel:   60,
			defenderLevel:   test.defenderLevel,
			defenderIsEnemy: true,
		}, false)
		if !ok {
			t.Fatalf("Classic reference resistance policy rejected defender level %d", test.defenderLevel)
		}
		assertClassicReferencePartialResistView(t, got, test.want)
	}
}

func TestClassicReferencePartialResistThresholdVectors(t *testing.T) {
	rules := classicReferenceResistanceTestRuleset()
	tests := []struct {
		name    string
		input   classicReferenceResistanceInput
		pureDot bool
		want    classicReferencePartialResistView
	}{
		{
			name:  "one third branch boundary",
			input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 100, flatSpellPenetration: 24},
			want:  classicReferencePartialResistView{coefficient: 1.0 / 3.0, threshold00: 0.76, threshold25: 0.21, threshold50: 0.03},
		},
		{
			name:  "two thirds branch boundary",
			input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 200, flatSpellPenetration: 24},
			want:  classicReferencePartialResistView{coefficient: 2.0 / 3.0, threshold00: 1, threshold25: 0.78, threshold50: 0.22},
		},
		{
			name:  "coefficient cap",
			input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 300, flatSpellPenetration: 24},
			want:  classicReferencePartialResistView{coefficient: 1, threshold00: 1, threshold25: 0.96, threshold50: 0.8},
		},
		{
			name:  "third branch interior",
			input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 200},
			want:  classicReferencePartialResistView{coefficient: 0.7466666666666666, threshold00: 1, threshold25: 0.8231999999999999, threshold50: 0.35919999999999985},
		},
		{
			name:    "pure dot divides explicit resistance only",
			input:   classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 200},
			pureDot: true,
			want:    classicReferencePartialResistView{coefficient: 0.14666666666666667, threshold00: 0.33440000000000003, threshold25: 0.0924, threshold50: 0.0132},
		},
		{
			name:    "pure dot applies penetration before division",
			input:   classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 200, flatSpellPenetration: 50},
			pureDot: true,
			want:    classicReferencePartialResistView{coefficient: 0.13, threshold00: 0.2964, threshold25: 0.0819, threshold50: 0.0117},
		},
		{
			name:    "pure dot division precedes cap",
			input:   classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 3000},
			pureDot: true,
			want:    classicReferencePartialResistView{coefficient: 1, threshold00: 1, threshold25: 0.96, threshold50: 0.8},
		},
		{
			name:  "full penetration keeps level component",
			input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 100, flatSpellPenetration: 100},
			want:  classicReferencePartialResistView{coefficient: 0.08, threshold00: 0.1824, threshold25: 0.0504, threshold50: 0.0072},
		},
		{
			name:  "negative selected resistance is floored",
			input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: -25},
			want:  classicReferencePartialResistView{coefficient: 0.08, threshold00: 0.1824, threshold25: 0.0504, threshold50: 0.0072},
		},
		{
			name:  "fractional inputs are preserved",
			input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 100.5, flatSpellPenetration: 0.5},
			want:  classicReferencePartialResistView{coefficient: 0.41333333333333333, threshold00: 0.8176, threshold25: 0.3468, threshold50: 0.0756},
		},
		{
			name:  "boss attacking non enemy",
			input: classicReferenceResistanceInput{attackerLevel: 63, defenderLevel: 60, selectedResistance: 100},
			want:  classicReferencePartialResistView{coefficient: 0.31746031746031744, threshold00: 0.7238095238095238, threshold25: 0.2, threshold50: 0.028571428571428567},
		},
		{
			name:  "higher level non enemy has no level component",
			input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63},
			want:  classicReferencePartialResistView{},
		},
		{
			name:  "level one lower boundary",
			input: classicReferenceResistanceInput{attackerLevel: 1, defenderLevel: 1, defenderIsEnemy: true},
			want:  classicReferencePartialResistView{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := classicReferencePartialResistProjection(rules, test.input, test.pureDot)
			if !ok {
				t.Fatal("Classic reference resistance policy rejected a supported vector")
			}
			assertClassicReferencePartialResistView(t, got, test.want)
		})
	}
}

func TestClassicReferenceBinaryResistVectors(t *testing.T) {
	rules := classicReferenceResistanceTestRuleset()
	tests := []struct {
		name  string
		input classicReferenceResistanceInput
		want  classicReferenceBinaryResistView
	}{
		{name: "zero resistance", input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true}, want: classicReferenceBinaryResistView{baseHitChanceMultiplier: 1}},
		{name: "spell penetration", input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 100, flatSpellPenetration: 40}, want: classicReferenceBinaryResistView{coefficient: 0.2, baseHitChanceMultiplier: 0.85}},
		{name: "one third coefficient", input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 100}, want: classicReferenceBinaryResistView{coefficient: 1.0 / 3.0, baseHitChanceMultiplier: 0.75}},
		{name: "two thirds coefficient", input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 200}, want: classicReferenceBinaryResistView{coefficient: 2.0 / 3.0, baseHitChanceMultiplier: 0.5}},
		{name: "coefficient cap", input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 300}, want: classicReferenceBinaryResistView{coefficient: 1, baseHitChanceMultiplier: 0.25}},
		{name: "over cap", input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 600}, want: classicReferenceBinaryResistView{coefficient: 1, baseHitChanceMultiplier: 0.25}},
		{name: "full penetration", input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true, selectedResistance: 100, flatSpellPenetration: 100}, want: classicReferenceBinaryResistView{baseHitChanceMultiplier: 1}},
		{name: "boss attacking non enemy", input: classicReferenceResistanceInput{attackerLevel: 63, defenderLevel: 60, selectedResistance: 100}, want: classicReferenceBinaryResistView{coefficient: 0.31746031746031744, baseHitChanceMultiplier: 0.7619047619047619}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := classicReferenceBinaryResistProjection(rules, test.input)
			if !ok {
				t.Fatal("Classic reference binary resistance policy rejected a supported vector")
			}
			assertFloat64(t, "coefficient", got.coefficient, test.want.coefficient)
			assertFloat64(t, "base hit chance multiplier", got.baseHitChanceMultiplier, test.want.baseHitChanceMultiplier)
		})
	}
}

func TestClassicReferenceResistanceScopeFailsClosed(t *testing.T) {
	validRules := classicReferenceResistanceTestRuleset()
	validInput := classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63}
	tests := []struct {
		name  string
		rules rulesetProfile
		input classicReferenceResistanceInput
	}{
		{name: "active inherited profile", rules: currentRuleset(), input: validInput},
		{name: "wrong boss delta", rules: rulesetWithLevelAndBossDelta(60, 2), input: validInput},
		{name: "zero attacker level", rules: validRules, input: classicReferenceResistanceInput{defenderLevel: 60}},
		{name: "negative attacker level", rules: validRules, input: classicReferenceResistanceInput{attackerLevel: -1, defenderLevel: 60}},
		{name: "attacker above default boss", rules: validRules, input: classicReferenceResistanceInput{attackerLevel: 64, defenderLevel: 60}},
		{name: "zero defender level", rules: validRules, input: classicReferenceResistanceInput{attackerLevel: 60}},
		{name: "negative defender level", rules: validRules, input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: -1}},
		{name: "defender above default boss", rules: validRules, input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 64}},
		{name: "negative penetration", rules: validRules, input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, flatSpellPenetration: -1}},
		{name: "resistance not a number", rules: validRules, input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, selectedResistance: math.NaN()}},
		{name: "resistance positive infinity", rules: validRules, input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, selectedResistance: math.Inf(1)}},
		{name: "resistance negative infinity", rules: validRules, input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, selectedResistance: math.Inf(-1)}},
		{name: "penetration not a number", rules: validRules, input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, flatSpellPenetration: math.NaN()}},
		{name: "penetration positive infinity", rules: validRules, input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, flatSpellPenetration: math.Inf(1)}},
		{name: "penetration negative infinity", rules: validRules, input: classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, flatSpellPenetration: math.Inf(-1)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			partial, partialOK := classicReferencePartialResistProjection(test.rules, test.input, false)
			binary, binaryOK := classicReferenceBinaryResistProjection(test.rules, test.input)
			if partialOK || binaryOK {
				t.Fatalf("unsupported input resolved to partial=%+v binary=%+v", partial, binary)
			}
			if partial != (classicReferencePartialResistView{}) || binary != (classicReferenceBinaryResistView{}) {
				t.Fatalf("rejected input returned partial=%+v binary=%+v", partial, binary)
			}
		})
	}
}

func TestClassicReferenceResistancePolicyIsInactive(t *testing.T) {
	active := currentRuleset()
	if active != inheritedTBCRuleset() {
		t.Fatal("current ruleset differs from inherited TBC")
	}
	input := classicReferenceResistanceInput{attackerLevel: 60, defenderLevel: 63, defenderIsEnemy: true}
	if _, ok := classicReferencePartialResistProjection(active, input, false); ok {
		t.Fatal("Classic partial resistance policy accepted the active inherited profile")
	}
	if _, ok := classicReferenceBinaryResistProjection(active, input); ok {
		t.Fatal("Classic binary resistance policy accepted the active inherited profile")
	}

	attacker := Unit{Type: PlayerUnit, Level: 60, PseudoStats: stats.NewPseudoStats()}
	defender := Unit{Type: EnemyUnit, Level: 63, PseudoStats: stats.NewPseudoStats()}
	spell := Spell{Unit: &attacker, SpellSchool: SpellSchoolFire, SchoolIndex: stats.SchoolIndexFire}
	threshold00, threshold25, threshold50 := NewAttackTable(&attacker, &defender).GetPartialResistThresholds(&spell)
	assertFloat64(t, "active inherited 0% threshold", threshold00, 0.1368)
	assertFloat64(t, "active inherited 25% threshold", threshold25, 0.0378)
	assertFloat64(t, "active inherited 50% threshold", threshold50, 0.0054)
}

func assertClassicReferencePartialResistView(t *testing.T, got classicReferencePartialResistView, want classicReferencePartialResistView) {
	t.Helper()
	assertFloat64(t, "coefficient", got.coefficient, want.coefficient)
	assertFloat64(t, "0% threshold", got.threshold00, want.threshold00)
	assertFloat64(t, "25% threshold", got.threshold25, want.threshold25)
	assertFloat64(t, "50% threshold", got.threshold50, want.threshold50)
}

func classicReferenceResistanceTestRuleset() rulesetProfile {
	return rulesetWithLevelAndBossDelta(classicReferenceCharacterLevel, classicReferenceDefaultBossLevelDelta)
}
