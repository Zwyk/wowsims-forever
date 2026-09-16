package core

import (
	"math"
	"testing"

	"github.com/wowsims/tbc/sim/core/stats"
)

func classicRuntimeMitigationTestRules() rulesetProfile {
	// Deliberately do not inherit any TBC rating, outcome, or attribute values.
	rules := classicReferenceMitigationScope()
	rules.mitigation = mitigationRules{
		armorModel:      armorMitigationClassicReference60,
		resistanceModel: resistanceMitigationClassicReference60,
	}
	return rules
}

func TestClassicRuntimeArmorUsesSelectedPolicyAndDynamicInputs(t *testing.T) {
	rules := classicRuntimeMitigationTestRules()
	attacker := Unit{Type: PlayerUnit, Level: 60, PseudoStats: stats.NewPseudoStats()}
	defender := Unit{Type: EnemyUnit, Level: 63, PseudoStats: stats.NewPseudoStats(), stats: stats.Stats{stats.Armor: 3750}}
	table := newAttackTableWithRuleset(&attacker, &defender, rules)
	// Caller and unit changes must not replace the table's selected model.
	rules.mitigation = mitigationRules{}
	attacker.rules, attacker.rulesInitialized = inheritedTBCRuleset(), true
	assertFloat64(t, "Classic 60 versus 3750 armor", table.GetArmorDamageModifier(nil), 5500.0/9250.0)

	attacker.stats[stats.ArmorPenetration] = 1000
	assertFloat64(t, "dynamic flat penetration", table.GetArmorDamageModifier(nil), 5500.0/8250.0)
	defender.PseudoStats.ArmorMultiplier = 0.8
	assertFloat64(t, "dynamic defender armor multiplier", table.GetArmorDamageModifier(nil), 5500.0/7500.0)
	table.ArmorIgnoreFactor = 0.25
	assertFloat64(t, "explicit per-attack armor override", table.GetArmorDamageModifier(nil), 5500.0/6750.0)
	table.IgnoreArmor = true
	assertFloat64(t, "ignore armor", table.GetArmorDamageModifier(nil), 1)
	table.IgnoreArmor, table.ArmorIgnoreFactor = false, 0
	defender.stats[stats.Armor] = -100
	assertFloat64(t, "Classic resolved armor floor", table.GetArmorDamageModifier(nil), 1)

	// Classic's pinned source has no TBC 75% mitigation cap. The attacker
	// level also matters for an incoming boss hit.
	attacker.Level, attacker.Type = 63, EnemyUnit
	attacker.stats[stats.ArmorPenetration] = 0
	defender.Level, defender.Type = 60, PlayerUnit
	defender.stats[stats.Armor] = 50000
	defender.PseudoStats.ArmorMultiplier = 1
	table.IgnoreArmor, table.ArmorIgnoreFactor = false, 0
	assertFloat64(t, "incoming Classic boss uncapped mitigation", table.GetArmorDamageModifier(nil), 5755.0/55755.0)

	if currentRuleset() != inheritedTBCRuleset() || CharacterLevel != 70 {
		t.Fatal("internal Classic table changed the default engine")
	}
}

func TestClassicRuntimeArmorTableOwnershipFallback(t *testing.T) {
	attacker := Unit{Type: PlayerUnit, Level: 60, rules: classicRuntimeMitigationTestRules(), rulesInitialized: true}
	defender := Unit{Type: EnemyUnit, Level: 63, PseudoStats: stats.NewPseudoStats(), stats: stats.Stats{stats.Armor: 3750}}
	for _, table := range []*AttackTable{NewAttackTable(&attacker, &defender), {Attacker: &attacker, Defender: &defender}} {
		assertFloat64(t, "attacker-owned mitigation", table.GetArmorDamageModifier(nil), 5500.0/9250.0)
	}
	assertFloat64(t, "explicit TBC override remains TBC", newAttackTableWithRuleset(&attacker, &defender, inheritedTBCRuleset()).GetArmorDamageModifier(nil), 5882.5/9632.5)
}

func TestClassicRuntimeArmorDamagePipeline(t *testing.T) {
	attacker := Unit{Type: PlayerUnit, Level: 60}
	defender := Unit{Type: EnemyUnit, Level: 63, PseudoStats: stats.NewPseudoStats(), stats: stats.Stats{stats.Armor: 3750}}
	table := newAttackTableWithRuleset(&attacker, &defender, classicRuntimeMitigationTestRules())
	spell := &Spell{Unit: &attacker, SpellSchool: SpellSchoolPhysical}
	result := SpellResult{Damage: 925}
	result.applyResistances(nil, spell, false, table)
	assertFloat64(t, "resolved direct physical damage", result.Damage, 550)
	assertFloat64(t, "saved armor multiplier", result.ArmorAndResistanceMultiplier, 5500.0/9250.0)
	assertFloat64(t, "saved post-mitigation damage", result.PostArmorAndResistanceMultiplier, 550)
	if result.Outcome != OutcomeEmpty {
		t.Fatal("armor added a magical resistance outcome")
	}
	modifier, outcome := spell.ResistanceMultiplier(nil, true, table)
	assertFloat64(t, "physical bleed bypass", modifier, 1)
	if outcome != OutcomeEmpty {
		t.Fatal("physical bleed added a resistance outcome")
	}
	spell.Flags = SpellFlagIgnoreResists
	modifier, _ = spell.ResistanceMultiplier(nil, false, table)
	assertFloat64(t, "explicit resistance bypass", modifier, 1)
}

func TestClassicRuntimeArmorRejectsUnsupportedInputs(t *testing.T) {
	tests := []struct {
		name        string
		level       int32
		armor       float64
		penetration float64
		model       armorMitigationModel
	}{
		{"zero attacker level", 0, 3750, 0, armorMitigationClassicReference60},
		{"level outside Classic envelope", 64, 3750, 0, armorMitigationClassicReference60},
		{"negative penetration", 60, 3750, -1, armorMitigationClassicReference60},
		{"nonfinite armor", 60, math.NaN(), 0, armorMitigationClassicReference60},
		{"negative infinite armor", 60, math.Inf(-1), 0, armorMitigationClassicReference60},
		{"nonfinite penetration", 60, 3750, math.Inf(1), armorMitigationClassicReference60},
		{"unknown model", 60, 3750, 0, armorMitigationModel(255)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rules := classicRuntimeMitigationTestRules()
			rules.mitigation.armorModel = test.model
			attacker := Unit{Level: test.level, stats: stats.Stats{stats.ArmorPenetration: test.penetration}}
			defender := Unit{Level: 63, PseudoStats: stats.NewPseudoStats(), stats: stats.Stats{stats.Armor: test.armor}}
			assertClassicRuntimeMitigationRejects(t, func() {
				newAttackTableWithRuleset(&attacker, &defender, rules).GetArmorDamageModifier(nil)
			})
		})
	}
}

func assertClassicRuntimeMitigationRejects(t *testing.T, action func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("unsupported Classic mitigation inputs were silently accepted")
		}
	}()
	action()
}
