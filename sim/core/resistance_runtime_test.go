package core

import (
	"math"
	"testing"

	"github.com/wowsims/tbc/sim/core/stats"
)

func TestClassicRuntimeResistanceSelectedPolicyAndDynamicInputs(t *testing.T) {
	rules := classicRuntimeMitigationTestRules()
	attacker := Unit{Type: PlayerUnit, Level: 60, PseudoStats: stats.NewPseudoStats()}
	defender := Unit{Type: EnemyUnit, Level: 63, PseudoStats: stats.NewPseudoStats()}
	spell := &Spell{Unit: &attacker, SpellSchool: SpellSchoolFire, SchoolIndex: stats.SchoolIndexFire}
	table := newAttackTableWithRuleset(&attacker, &defender, rules)
	rules.mitigation = mitigationRules{}
	attacker.rules, attacker.rulesInitialized = inheritedTBCRuleset(), true
	assertRuntimePartialThresholds(t, table, spell, 0.1824, 0.0504, 0.0072)
	assertFloat64(t, "no binary level resistance", table.GetBinaryHitChance(spell), 1)

	defender.stats[stats.FireResistance] = 100
	assertRuntimePartialThresholds(t, table, spell, 0.8176, 0.3468, 0.0756)
	assertFloat64(t, "binary one-third coefficient", table.GetBinaryHitChance(spell), 0.75)
	attacker.stats[stats.SpellPenetration] = 40
	assertRuntimePartialThresholds(t, table, spell, 0.6384, 0.1764, 0.0252)
	assertFloat64(t, "dynamic binary penetration", table.GetBinaryHitChance(spell), 0.85)
	attacker.stats[stats.SpellPenetration] = 100
	assertRuntimePartialThresholds(t, table, spell, 0.1824, 0.0504, 0.0072)
	assertFloat64(t, "full penetration preserves only non-binary level effect", table.GetBinaryHitChance(spell), 1)

	// Resistance changes remain live even though rule selection is cached.
	defender.stats[stats.FireResistance] = -25
	attacker.stats[stats.SpellPenetration] = 0
	assertRuntimePartialThresholds(t, table, spell, 0.1824, 0.0504, 0.0072)
	if currentRuleset() != inheritedTBCRuleset() {
		t.Fatal("Classic resistance changed the active engine")
	}
}

func TestClassicRuntimeResistancePureDotsAndSchools(t *testing.T) {
	attacker := Unit{Type: PlayerUnit, Level: 60}
	defender := Unit{Type: EnemyUnit, Level: 63, stats: stats.Stats{
		stats.Strength:         999,
		stats.ArcaneResistance: 200,
		stats.FireResistance:   200,
		stats.FrostResistance:  200,
		stats.NatureResistance: 200,
		stats.ShadowResistance: 200,
	}}
	table := newAttackTableWithRuleset(&attacker, &defender, classicRuntimeMitigationTestRules())
	for _, school := range []SpellSchool{SpellSchoolArcane, SpellSchoolFire, SpellSchoolFrost, SpellSchoolNature, SpellSchoolShadow} {
		spell := &Spell{Unit: &attacker, SpellSchool: school, SchoolIndex: school.SchoolIndex()}
		assertRuntimePartialThresholds(t, table, spell, 1, 0.8232, 0.3592)
		spell.Flags = SpellFlagPureDot
		assertRuntimePartialThresholds(t, table, spell, 0.3344, 0.0924, 0.0132)
		assertFloat64(t, "pure dot flag cannot alter binary resistance", table.GetBinaryHitChance(spell), 0.5)
	}
	spell := &Spell{Unit: &attacker, SpellSchool: SpellSchoolHoly, SchoolIndex: stats.SchoolIndexHoly}
	assertRuntimePartialThresholds(t, table, spell, 0.1824, 0.0504, 0.0072)
	assertFloat64(t, "Holy does not read Strength as resistance", table.GetBinaryHitChance(spell), 1)
	for _, school := range []SpellSchool{SpellSchoolNone, SpellSchoolPhysical} {
		spell.SpellSchool = school
		assertRuntimePartialThresholds(t, table, spell, 0, 0, 0)
		assertFloat64(t, "nonmagical binary multiplier", table.GetBinaryHitChance(spell), 1)
	}

	// The imported TBC engine must ignore the newly available Classic flag.
	spell.SpellSchool, spell.SchoolIndex = SpellSchoolFire, stats.SchoolIndexFire
	tbcTable := newAttackTableWithRuleset(&attacker, &defender, inheritedTBCRuleset())
	spell.Flags = SpellFlagPureDot
	assertRuntimePartialThresholds(t, tbcTable, spell, 1, 0.8232, 0.3592)
}

func TestClassicRuntimeResistanceOwnershipAndIncomingBoss(t *testing.T) {
	attacker := Unit{Type: EnemyUnit, Level: 63, rules: classicRuntimeMitigationTestRules(), rulesInitialized: true}
	defender := Unit{Type: PlayerUnit, Level: 60, stats: stats.Stats{stats.FireResistance: 100}}
	spell := &Spell{Unit: &attacker, SpellSchool: SpellSchoolFire, SchoolIndex: stats.SchoolIndexFire}
	for _, table := range []*AttackTable{NewAttackTable(&attacker, &defender), {Attacker: &attacker, Defender: &defender}} {
		assertRuntimePartialThresholds(t, table, spell, 0.7238095238095238, 0.2, 0.028571428571428567)
		assertFloat64(t, "incoming binary multiplier", table.GetBinaryHitChance(spell), 0.7619047619047619)
	}

	attacker.Level, attacker.Type = 60, PlayerUnit
	defender.Level, defender.Type = 63, EnemyUnit
	defender.stats[stats.FireResistance] = 0
	assertRuntimePartialThresholds(t, newAttackTableWithRuleset(&attacker, &defender, inheritedTBCRuleset()), spell, 0.1368, 0.0378, 0.0054)
}

type fixedResistanceRoll struct {
	Rand
	value float64
}

func (roll fixedResistanceRoll) NextFloat64() float64 { return roll.value }

func TestClassicRuntimePartialResistanceDamagePipeline(t *testing.T) {
	attacker := Unit{Type: PlayerUnit, Level: 60}
	defender := Unit{Type: EnemyUnit, Level: 63}
	table := newAttackTableWithRuleset(&attacker, &defender, classicRuntimeMitigationTestRules())
	spell := &Spell{Unit: &attacker, SpellSchool: SpellSchoolFire, SchoolIndex: stats.SchoolIndexFire}
	for _, test := range []struct {
		roll        float64
		wantDamage  float64
		wantOutcome HitOutcome
	}{
		{0.005, 250, OutcomePartial3_4},
		{0.02, 500, OutcomePartial2_4},
		{0.15, 750, OutcomePartial1_4},
		{0.5, 1000, OutcomeEmpty},
	} {
		sim := &Simulation{rand: fixedResistanceRoll{value: test.roll}}
		result := SpellResult{Damage: 1000, Outcome: OutcomeHit}
		result.applyResistances(sim, spell, false, table)
		assertFloat64(t, "resolved magical damage", result.Damage, test.wantDamage)
		assertFloat64(t, "saved post-mitigation damage", result.PostArmorAndResistanceMultiplier, test.wantDamage)
		assertFloat64(t, "saved resistance multiplier", result.ArmorAndResistanceMultiplier, test.wantDamage/1000)
		if result.Outcome != OutcomeHit|test.wantOutcome {
			t.Fatalf("roll %g: outcome=%v, want %v", test.roll, result.Outcome, OutcomeHit|test.wantOutcome)
		}
	}
	spell.Flags = SpellFlagBinary
	modifier, outcome := spell.ResistanceMultiplier(nil, false, table)
	assertFloat64(t, "binary bypasses partial damage resistance", modifier, 1)
	if outcome != OutcomeEmpty {
		t.Fatal("binary spell acquired a partial resistance outcome")
	}
}

func TestClassicRuntimeResistanceRejectsUnsupportedInputs(t *testing.T) {
	tests := []struct {
		name          string
		attackerLevel int32
		defenderLevel int32
		resistance    float64
		penetration   float64
		school        SpellSchool
		model         resistanceMitigationModel
	}{
		{"attacker outside Classic envelope", 64, 63, 0, 0, SpellSchoolFire, resistanceMitigationClassicReference60},
		{"defender outside Classic envelope", 60, 64, 0, 0, SpellSchoolFire, resistanceMitigationClassicReference60},
		{"zero attacker", 0, 63, 0, 0, SpellSchoolFire, resistanceMitigationClassicReference60},
		{"zero defender", 60, 0, 0, 0, SpellSchoolFire, resistanceMitigationClassicReference60},
		{"nonfinite resistance", 60, 63, math.NaN(), 0, SpellSchoolFire, resistanceMitigationClassicReference60},
		{"negative penetration", 60, 63, 0, -1, SpellSchoolFire, resistanceMitigationClassicReference60},
		{"nonfinite penetration", 60, 63, 0, math.Inf(1), SpellSchoolFire, resistanceMitigationClassicReference60},
		{"magic hybrid", 60, 63, 0, 0, SpellSchoolShadowFlame, resistanceMitigationClassicReference60},
		{"physical hybrid", 60, 63, 0, 0, SpellSchoolPhysical | SpellSchoolFire, resistanceMitigationClassicReference60},
		{"unknown model", 60, 63, 0, 0, SpellSchoolFire, resistanceMitigationModel(255)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rules := classicRuntimeMitigationTestRules()
			rules.mitigation.resistanceModel = test.model
			attacker := Unit{Type: PlayerUnit, Level: test.attackerLevel, stats: stats.Stats{stats.SpellPenetration: test.penetration}}
			defender := Unit{Type: EnemyUnit, Level: test.defenderLevel, stats: stats.Stats{stats.FireResistance: test.resistance}}
			spell := &Spell{Unit: &attacker, SpellSchool: test.school, SchoolIndex: stats.SchoolIndexFire}
			table := newAttackTableWithRuleset(&attacker, &defender, rules)
			assertClassicRuntimeMitigationRejects(t, func() { table.GetPartialResistThresholds(spell) })
			assertClassicRuntimeMitigationRejects(t, func() { table.GetBinaryHitChance(spell) })
			if test.school == SpellSchoolPhysical|SpellSchoolFire {
				assertClassicRuntimeMitigationRejects(t, func() { spell.ResistanceMultiplier(nil, false, table) })
			}
		})
	}
}

func assertRuntimePartialThresholds(t *testing.T, table *AttackTable, spell *Spell, want00, want25, want50 float64) {
	t.Helper()
	threshold00, threshold25, threshold50 := table.GetPartialResistThresholds(spell)
	assertFloat64(t, "0% threshold", threshold00, want00)
	assertFloat64(t, "25% threshold", threshold25, want25)
	assertFloat64(t, "50% threshold", threshold50, want50)
}
