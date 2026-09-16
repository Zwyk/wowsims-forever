package core

import (
	"math"
	"reflect"
	"testing"

	"github.com/wowsims/tbc/sim/core/stats"
)

func classicSpellOutcomeFixture() (*Spell, *AttackTable) {
	rules := classic60SpellReferenceRules()
	attacker := &Unit{Type: PlayerUnit, Level: 60, PseudoStats: stats.NewPseudoStats(), rules: rules, rulesInitialized: true}
	defender := &Unit{Type: EnemyUnit, Level: 63, PseudoStats: stats.NewPseudoStats(), rules: rules, rulesInitialized: true}
	attacker.stats[stats.SpellCritPercent] = 10
	table := newAttackTableWithRuleset(attacker, defender, rules)
	attacker.AttackTables = []*AttackTable{table}
	return &Spell{
		Unit: attacker, SpellSchool: SpellSchoolFire, SchoolIndex: stats.SchoolIndexFire,
		DefenseType: DefenseTypeMagic, ProcMask: ProcMaskSpellDamage, weaponAttackSource: WeaponAttackSourceNone,
		IgnoreHaste: true, CritMultiplierPct: 1, SpellMetrics: make([]SpellMetrics, 1),
	}, table
}

func TestClassicSpellChanceLiteralLevelsAndSchoolHit(t *testing.T) {
	for _, test := range []struct {
		level       int32
		miss, bonus float64
	}{
		{60, 0.04, 0.01}, {61, 0.05, 0.02}, {62, 0.06, 0.03}, {63, 0.17, 0.14},
	} {
		spell, table := classicSpellOutcomeFixture()
		table.Defender.Level = test.level
		// Deliberately poison inherited table seeds. The Classic reference uses
		// validated levels and the source's executable, unsuppressed crit path.
		table.BaseSpellMissChance, table.SpellCritSuppression = 0.8, 0.9
		before := *table
		assertFloat64(t, "literal level miss", spell.SpellChanceToMiss(table), test.miss)
		assertFloat64(t, "unused suppression seed", spell.SpellCritChance(table.Defender), 0.1)
		spell.Unit.stats[stats.SpellHitPercent] = 1
		spell.BonusHitPercent = 0.5
		spell.Unit.PseudoStats.SchoolBonusHitChance[stats.SchoolIndexFire] = 1.5
		spell.Unit.PseudoStats.SchoolBonusHitChance[stats.SchoolIndexFrost] = 30
		assertFloat64(t, "school hit without class mask", spell.SpellHitChance(table.Defender), 0.03)
		assertFloat64(t, "hit components", spell.SpellChanceToMiss(table), test.bonus)
		spell.ClassSpellMask = 1
		assertFloat64(t, "class mask does not change Classic hit", spell.SpellChanceToMiss(table), test.bonus)
		spell.Unit.stats[stats.SpellHitPercent] = 100
		assertFloat64(t, "one percent floor", spell.SpellChanceToMiss(table), 0.01)
		if !reflect.DeepEqual(before, *table) {
			t.Fatal("chance getters mutated the shared attack table")
		}
	}
}

func TestClassicSpellChanceBinaryOrderAndExpectedOutcomes(t *testing.T) {
	for _, test := range []struct {
		name                 string
		binary               bool
		resist, pen, hit     float64
		miss, expectedDamage float64
	}{
		{"direct", false, 100, 0, 6, 0.11, 93.45},
		{"binary", true, 100, 0, 6, 0.3175, 71.6625},
		{"binary penetration", true, 100, 40, 6, 0.2345, 80.3775},
		{"binary no explicit resistance", true, 0, 0, 6, 0.11, 93.45},
		{"binary hit beyond level cap", true, 100, 0, 36.75, 0.01, 103.95},
		{"binary capped resistance", true, 600, 0, 6, 0.7325, 28.0875},
	} {
		t.Run(test.name, func(t *testing.T) {
			spell, table := classicSpellOutcomeFixture()
			if test.binary {
				spell.Flags |= SpellFlagBinary
			}
			table.Defender.stats[stats.FireResistance] = test.resist
			spell.Unit.stats[stats.SpellPenetration] = test.pen
			spell.Unit.stats[stats.SpellHitPercent] = test.hit
			assertFloat64(t, "source miss composition", spell.SpellChanceToMiss(table), test.miss)
			result := &SpellResult{Target: table.Defender, Damage: 100}
			spell.OutcomeExpectedMagicHitAndCrit(nil, result, table)
			assertFloat64(t, "expected hit and crit before partial mitigation", result.Damage, test.expectedDamage)
			result.Damage = 100
			spell.OutcomeExpectedMagicHit(nil, result, table)
			assertFloat64(t, "expected hit only", result.Damage, 100*(1-test.miss))
			result.Damage = 100
			spell.OutcomeExpectedMagicCrit(nil, result, table)
			assertFloat64(t, "expected crit only", result.Damage, 105)
		})
	}
}

func TestClassicSpellChanceSampledOutcomeBoundaries(t *testing.T) {
	for _, test := range []struct {
		name          string
		rolls         []float64
		outcome       HitOutcome
		damage        float64
		misses, crits int32
	}{
		{"miss", []float64{0.84}, OutcomeMiss, 0, 1, 0},
		{"crit", []float64{0.82, 0.099}, OutcomeCrit, 150, 0, 1},
		{"hit", []float64{0.82, 0.101}, OutcomeHit, 100, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			spell, table := classicSpellOutcomeFixture()
			roll := &sequenceMeleeRoll{values: test.rolls}
			result := &SpellResult{Target: table.Defender, Damage: 100}
			spell.OutcomeMagicHitAndCrit(&Simulation{rand: roll}, result, table)
			if result.Outcome != test.outcome || roll.calls != len(test.rolls) {
				t.Fatalf("outcome=%v rolls=%d, want %v/%d", result.Outcome, roll.calls, test.outcome, len(test.rolls))
			}
			assertFloat64(t, "sampled spell damage", result.Damage, test.damage)
			metrics := spell.SpellMetrics[0]
			if metrics.Misses != test.misses || metrics.Crits != test.crits {
				t.Fatalf("unexpected outcome counters: %+v", metrics)
			}
		})
	}
}

func TestClassicSpellChanceSingleSchoolsIncludingHoly(t *testing.T) {
	for _, school := range []SpellSchool{SpellSchoolArcane, SpellSchoolFire, SpellSchoolFrost, SpellSchoolHoly, SpellSchoolNature, SpellSchoolShadow} {
		spell, table := classicSpellOutcomeFixture()
		spell.SpellSchool, spell.SchoolIndex = school, school.SchoolIndex()
		assertFloat64(t, "single school base miss", spell.SpellChanceToMiss(table), 0.17)
		// Holy's resistance sentinel must not read the defender's Strength.
		table.Defender.stats[stats.Strength] = 300
		threshold00, threshold25, threshold50 := table.GetPartialResistThresholds(spell)
		assertFloat64(t, "level-only threshold 00", threshold00, 0.1824)
		assertFloat64(t, "level-only threshold 25", threshold25, 0.0504)
		assertFloat64(t, "level-only threshold 50", threshold50, 0.0072)
		spell.Flags |= SpellFlagBinary
		assertFloat64(t, "binary no explicit resistance", spell.SpellChanceToMiss(table), 0.17)
	}
}

func TestClassicSpellChanceRejectsUnsupportedContexts(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Spell, *AttackTable)
	}{
		{"attacker level", func(s *Spell, _ *AttackTable) { s.Unit.Level = 59 }},
		{"defender below range", func(_ *Spell, at *AttackTable) { at.Defender.Level = 59 }},
		{"defender above range", func(_ *Spell, at *AttackTable) { at.Defender.Level = 64 }},
		{"pet", func(s *Spell, _ *AttackTable) { s.Unit.Type = PetUnit }},
		{"enemy caster", func(s *Spell, _ *AttackTable) { s.Unit.Type = EnemyUnit }},
		{"player target", func(_ *Spell, at *AttackTable) { at.Defender.Type = PlayerUnit }},
		{"wrong owner", func(_ *Spell, at *AttackTable) { at.Attacker = &Unit{} }},
		{"unregistered table", func(s *Spell, _ *AttackTable) { s.Unit.AttackTables = nil }},
		{"detached table", func(s *Spell, at *AttackTable) { detached := *at; s.Unit.AttackTables[0] = &detached }},
		{"uncached table", func(_ *Spell, at *AttackTable) { at.rulesInitialized = false }},
		{"mixed resistance model", func(_ *Spell, at *AttackTable) { at.mitigation.resistanceModel = resistanceMitigationInheritedTBC }},
		{"defense none", func(s *Spell, _ *AttackTable) { s.DefenseType = DefenseTypeNone }},
		{"melee defense", func(s *Spell, _ *AttackTable) { s.DefenseType = DefenseTypeMelee }},
		{"unspecified weapon", func(s *Spell, _ *AttackTable) { s.weaponAttackSource = WeaponAttackSourceUnspecified }},
		{"weapon source", func(s *Spell, _ *AttackTable) { s.weaponAttackSource = WeaponAttackSourceRanged }},
		{"healing proc mask", func(s *Spell, _ *AttackTable) { s.ProcMask = ProcMaskSpellHealing }},
		{"hybrid school", func(s *Spell, _ *AttackTable) { s.SpellSchool = SpellSchoolShadowFlame }},
		{"physical school", func(s *Spell, _ *AttackTable) { s.SpellSchool = SpellSchoolPhysical }},
		{"school index mismatch", func(s *Spell, _ *AttackTable) { s.SchoolIndex = stats.SchoolIndexFrost }},
		{"mana", func(s *Spell, _ *AttackTable) { s.Unit.manaBar.unit = s.Unit }},
		{"cost", func(s *Spell, _ *AttackTable) { s.Cost = &SpellCost{} }},
		{"spell haste", func(s *Spell, _ *AttackTable) { s.Unit.stats[stats.SpellHasteRating] = 1 }},
		{"cast speed", func(s *Spell, _ *AttackTable) { s.Unit.PseudoStats.CastSpeedMultiplier = 1.1 }},
		{"haste allowed", func(s *Spell, _ *AttackTable) { s.IgnoreHaste = false }},
		{"target crit", func(_ *Spell, at *AttackTable) { at.Defender.PseudoStats.BonusSpellCritPercentTaken = 1 }},
		{"pair crit", func(_ *Spell, at *AttackTable) { at.BonusSpellCritPercent = 1 }},
		{"crit reduction", func(_ *Spell, at *AttackTable) { at.Defender.PseudoStats.ReducedCritTakenPercent = 1 }},
		{"resilience", func(_ *Spell, at *AttackTable) { at.Defender.stats[stats.ResilienceRating] = 1 }},
		{"crit multiplier", func(s *Spell, _ *AttackTable) { s.CritMultiplierPct = 2 }},
		{"nonfinite hit", func(s *Spell, _ *AttackTable) { s.BonusHitPercent = math.NaN() }},
		{"negative hit", func(s *Spell, _ *AttackTable) { s.BonusHitPercent = -1 }},
		{"nonfinite crit", func(s *Spell, _ *AttackTable) { s.BonusCritPercent = math.Inf(1) }},
		{"negative crit", func(s *Spell, _ *AttackTable) { s.Unit.stats[stats.SpellCritPercent] = -1 }},
		{"crit above 100", func(s *Spell, _ *AttackTable) { s.Unit.stats[stats.SpellCritPercent] = 101 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			spell, table := classicSpellOutcomeFixture()
			test.change(spell, table)
			defer func() {
				if recover() == nil {
					t.Fatal("unsupported spell context did not fail closed")
				}
			}()
			spell.SpellChanceToMiss(table)
		})
	}
}

func TestClassicSpellChanceCachedOwnershipAndActiveIsolation(t *testing.T) {
	spell, table := classicSpellOutcomeFixture()
	spell.Unit.rules = currentRuleset()
	assertFloat64(t, "cached miss ignores changed owner", spell.SpellChanceToMiss(table), 0.17)
	assertFloat64(t, "cached crit ignores changed owner", spell.SpellCritChance(table.Defender), 0.1)
	assertFloat64(t, "cached multiplier ignores changed owner", spell.CritDamageMultiplier(table), 1.5)
	if currentRuleset().id != rulesetInheritedTBC || currentRuleset().levels.characterLevel != 70 ||
		currentRuleset().combat.outcomes.spellChanceModel != spellChanceModelInheritedTBC {
		t.Fatal("caster reference activated public Classic behavior")
	}
	// An explicitly inherited table keeps TBC's school-mask gate and crit
	// suppression even if the owner unit is now a Classic diagnostic.
	spell.Unit.rules = classic60SpellReferenceRules()
	table = newAttackTableWithRuleset(spell.Unit, table.Defender, currentRuleset())
	spell.Unit.AttackTables[0] = table
	table.BaseSpellMissChance, table.SpellCritSuppression = 0.17, 0.021
	spell.Unit.PseudoStats.SchoolBonusHitChance[stats.SchoolIndexFire] = 3
	assertFloat64(t, "TBC no class mask", spell.SpellHitChance(table.Defender), 0)
	assertFloat64(t, "TBC suppression", spell.SpellCritChance(table.Defender), 0.079)
	spell.ClassSpellMask = 1
	assertFloat64(t, "TBC class mask", spell.SpellHitChance(table.Defender), 0.03)
}

func TestClassicSpellChanceRejectsPhysicalAndBypassDamage(t *testing.T) {
	for _, read := range []func(*Spell, *AttackTable){
		func(s *Spell, at *AttackTable) { s.GetPhysicalMissChance(at) },
		func(s *Spell, at *AttackTable) { s.PhysicalCritChance(at) },
		func(s *Spell, at *AttackTable) {
			s.SpellSchool = SpellSchoolPhysical
			s.ResistanceMultiplier(nil, true, at)
		},
		func(s *Spell, at *AttackTable) {
			s.weaponAttackSource = WeaponAttackSourceUnspecified
			s.Flags |= SpellFlagIgnoreResists
			s.ResistanceMultiplier(nil, true, at)
		},
	} {
		func() {
			spell, table := classicSpellOutcomeFixture()
			defer func() {
				if recover() == nil {
					t.Fatal("unsupported physical/periodic bypass did not fail closed")
				}
			}()
			read(spell, table)
		}()
	}
}
