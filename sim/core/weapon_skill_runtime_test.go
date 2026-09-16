package core

import (
	"math"
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func classicMeleeOutcomeFixture() (*Character, *Spell, *AttackTable) {
	rules := classic60MeleeReferenceRules()
	character := &Character{
		Unit:  Unit{Type: PlayerUnit, Level: 60, PseudoStats: stats.NewPseudoStats(), rules: rules, rulesInitialized: true},
		Class: proto.Class_ClassWarrior,
		Race:  proto.Race_RaceHuman,
	}
	character.AutoAttacks.character = character
	character.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{
		ID: 1, Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeSword, HandType: proto.HandType_HandTypeTwoHand,
	}
	character.stats[stats.PhysicalCritPercent] = 10
	defender := &Unit{Type: EnemyUnit, Level: 63, PseudoStats: stats.NewPseudoStats(), rules: rules, rulesInitialized: true}
	table := newAttackTableWithRuleset(&character.Unit, defender, rules)
	spell := &Spell{
		Unit: &character.Unit, DefenseType: DefenseTypeMelee, SpellSchool: SpellSchoolPhysical,
		ProcMask: ProcMaskMeleeMHAuto, weaponAttackSource: WeaponAttackSourceMainHand,
		CritMultiplierPct: 1, SpellMetrics: make([]SpellMetrics, 1),
	}
	return character, spell, table
}

type sequenceMeleeRoll struct {
	Rand
	values []float64
	calls  int
}

func (roll *sequenceMeleeRoll) NextFloat64() float64 {
	value := roll.values[roll.calls]
	roll.calls++
	return value
}

func TestClassicRuntimeWhiteOutcomeBoundaries(t *testing.T) {
	for _, test := range []struct {
		name             string
		roll, glanceRoll float64
		outcome          HitOutcome
		damage           float64
	}{
		{"miss", 0.079, 0, OutcomeMiss, 0},
		{"dodge", 0.08, 0, OutcomeDodge, 0},
		{"glance minimum", 0.146, 0, OutcomeGlance, 55},
		{"glance midpoint", 0.146, 0.5, OutcomeGlance, 65},
		{"glance upper range", 0.146, 0.999, OutcomeGlance, 74.98},
		{"crit", 0.546, 0, OutcomeCrit, 200},
		{"normal hit", 0.598, 0, OutcomeHit, 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, spell, table := classicMeleeOutcomeFixture()
			roll := &sequenceMeleeRoll{values: []float64{test.roll, test.glanceRoll}}
			result := &SpellResult{Target: table.Defender, Damage: 100}
			spell.OutcomeMeleeWhite(&Simulation{rand: roll}, result, table)
			if result.Outcome != test.outcome {
				t.Fatalf("outcome=%v, want %v", result.Outcome, test.outcome)
			}
			assertFloat64(t, "damage", result.Damage, test.damage)
			if roll.calls != 2 {
				t.Fatalf("Classic white attack consumed %d rolls, want 2", roll.calls)
			}
		})
	}
}

func TestClassicRuntimeWeaponSkillUpdatesAllConsumers(t *testing.T) {
	character, spell, table := classicMeleeOutcomeFixture()
	// Poison pair-wide seeds: every allowed outcome consumer must use the
	// current weapon's Classic view, without rewriting the shared table.
	table.BaseMissChance, table.HitSuppression = 0.91, 0.92
	table.BaseDodgeChance, table.BaseGlanceChance = 0.93, 0.94
	table.GlanceMultiplier, table.MeleeCritSuppression = 0.95, 0.96
	character.stats[stats.PhysicalHitPercent] = 1
	assertFloat64(t, "first hit percent is suppressed", spell.PhysicalHitChance(table), 0)
	assertFloat64(t, "300 skill miss", spell.GetPhysicalMissChance(table), 0.08)
	character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedSwords, 5)
	assertFloat64(t, "305 skill hit", spell.PhysicalHitChance(table), 0.01)
	assertFloat64(t, "305 skill miss", spell.GetPhysicalMissChance(table), 0.05)
	assertFloat64(t, "305 skill dodge", table.Defender.GetTotalDodgeChanceAsDefender(spell, table), 0.06)
	assertFloat64(t, "source boss crit", spell.PhysicalCritChance(table), 0.052)
	result := &SpellResult{Target: table.Defender, Damage: 100}
	spell.OutcomeMeleeWhite(&Simulation{rand: &sequenceMeleeRoll{values: []float64{0.3, 0.5}}}, result, table)
	assertFloat64(t, "305 skill sampled glance", result.Damage, 85)
	character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedSwords, 3)
	assertFloat64(t, "308 skill miss", spell.GetPhysicalMissChance(table), 0.047)
	assertFloat64(t, "308 skill dodge", table.Defender.GetTotalDodgeChanceAsDefender(spell, table), 0.057)
	assertFloat64(t, "pair-wide miss unchanged", table.BaseMissChance, 0.91)
	assertFloat64(t, "pair-wide glance unchanged", table.GlanceMultiplier, 0.95)
}

func TestClassicRuntimeExpectedWhiteMatchesOutcomeTable(t *testing.T) {
	for _, test := range []struct{ skill, crit, want float64 }{
		{0, 10, 76.7}, {5, 10, 87.2}, {8, 10, 91.8},
		{0, 100, 117}, // Crit is capped after miss, dodge and glance.
	} {
		character, spell, table := classicMeleeOutcomeFixture()
		character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedSwords, test.skill)
		character.stats[stats.PhysicalCritPercent] = test.crit
		result := &SpellResult{Target: table.Defender, Damage: 100}
		spell.OutcomeExpectedMeleeWhite(nil, result, table)
		assertFloat64(t, "expected white damage", result.Damage, test.want)
	}
	_, spell, table := classicMeleeOutcomeFixture()
	zero := &SpellResult{Target: table.Defender}
	spell.OutcomeExpectedMeleeWhite(nil, zero, table)
	assertFloat64(t, "zero base damage remains finite", zero.Damage, 0)
}

func TestClassicRuntimeRejectsUnauditedMeleeContexts(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Character, *Spell, *AttackTable)
	}{
		{"class", func(c *Character, _ *Spell, _ *AttackTable) { c.Class = proto.Class_ClassMage }},
		{"race", func(c *Character, _ *Spell, _ *AttackTable) { c.Race = proto.Race_RaceOrc }},
		{"front", func(c *Character, _ *Spell, _ *AttackTable) { c.PseudoStats.InFrontOfTarget = true }},
		{"dual wield", func(c *Character, _ *Spell, _ *AttackTable) { c.AutoAttacks.IsDualWielding = true }},
		{"queued attack override", func(c *Character, _ *Spell, _ *AttackTable) { c.PseudoStats.DisableDWMissPenalty = true }},
		{"unarmed", func(c *Character, _ *Spell, _ *AttackTable) { c.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{} }},
		{"mixed auto and special", func(_ *Character, s *Spell, _ *AttackTable) { s.ProcMask = ProcMaskMeleeMHAuto | ProcMaskMeleeMHSpecial }},
		{"offhand", func(_ *Character, s *Spell, _ *AttackTable) { s.weaponAttackSource = WeaponAttackSourceOffHand }},
		{"school", func(_ *Character, s *Spell, _ *AttackTable) { s.SpellSchool = SpellSchoolFire }},
		{"cannot dodge", func(_ *Character, s *Spell, _ *AttackTable) { s.Flags |= SpellFlagCannotBeDodged }},
		{"expertise", func(c *Character, _ *Spell, _ *AttackTable) { c.stats[stats.ExpertiseRating] = 1 }},
		{"haste rating", func(c *Character, _ *Spell, _ *AttackTable) { c.stats[stats.MeleeHasteRating] = 1 }},
		{"attack speed", func(c *Character, _ *Spell, _ *AttackTable) { c.PseudoStats.AttackSpeedMultiplier = 1.1 }},
		{"melee speed", func(c *Character, _ *Spell, _ *AttackTable) { c.PseudoStats.MeleeSpeedMultiplier = 1.1 }},
		{"mana bar", func(c *Character, _ *Spell, _ *AttackTable) { c.manaBar.unit = &c.Unit }},
		{"nonfinite hit", func(c *Character, _ *Spell, _ *AttackTable) { c.stats[stats.PhysicalHitPercent] = math.NaN() }},
		{"defense", func(_ *Character, _ *Spell, at *AttackTable) { at.Defender.stats[stats.DefenseRating] = 1 }},
		{"dodge modifier", func(_ *Character, _ *Spell, at *AttackTable) { at.Defender.PseudoStats.DodgeReduction = 0.01 }},
		{"level", func(_ *Character, _ *Spell, at *AttackTable) { at.Defender.Level = 64 }},
		{"skill", func(c *Character, _ *Spell, _ *AttackTable) {
			c.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedSwords, 16)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			character, spell, table := classicMeleeOutcomeFixture()
			test.change(character, spell, table)
			defer func() {
				if recover() == nil {
					t.Fatal("unsupported Classic context did not fail closed")
				}
			}()
			spell.GetPhysicalMissChance(table)
		})
	}
}

func TestClassicRuntimeAbsentDefensiveRatingsStayFinite(t *testing.T) {
	unit := Unit{rules: classic60MeleeReferenceRules(), rulesInitialized: true}
	for _, chance := range []float64{unit.GetDodgeFromRating(), unit.GetParryFromRating(), unit.GetBlockFromRating(), unit.GetDefenseReduction(), unit.GetResilienceReduction()} {
		assertFloat64(t, "absent unaudited stat", chance, 0)
	}
	unit.updateReducedCritTakenPercent()
	assertFloat64(t, "finalization crit reduction", unit.PseudoStats.ReducedCritTakenPercent, 0)
	spell := &Spell{Unit: &unit}
	assertFloat64(t, "absent expertise", spell.DodgeParrySuppression(), 0)
	for _, test := range []struct {
		name string
		stat stats.Stat
		read func() float64
	}{
		{"dodge", stats.DodgeRating, unit.GetDodgeFromRating},
		{"parry", stats.ParryRating, unit.GetParryFromRating},
		{"block", stats.BlockRating, unit.GetBlockFromRating},
		{"defense", stats.DefenseRating, unit.GetDefenseReduction},
		{"resilience", stats.ResilienceRating, unit.GetResilienceReduction},
		{"expertise", stats.ExpertiseRating, spell.DodgeParrySuppression},
	} {
		t.Run(test.name, func(t *testing.T) {
			unit.stats = stats.Stats{}
			unit.stats[test.stat] = 1
			defer func() {
				if recover() == nil {
					t.Fatal("nonzero unsupported stat did not fail closed")
				}
			}()
			test.read()
		})
	}
	for _, divisor := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid conversion divisor did not fail closed")
				}
			}()
			checkedRatingQuotient(1, divisor)
		}()
	}
}

func TestInheritedWhiteOutcomeKeepsOneRandomDraw(t *testing.T) {
	character, spell, _ := classicMeleeOutcomeFixture()
	rules := inheritedTBCRuleset()
	defender := &Unit{Type: EnemyUnit, Level: 73, PseudoStats: stats.NewPseudoStats()}
	table := newAttackTableWithRuleset(&character.Unit, defender, rules)
	roll := &sequenceMeleeRoll{values: []float64{0.2}}
	result := &SpellResult{Target: defender, Damage: 100}
	spell.OutcomeMeleeWhite(&Simulation{rand: roll}, result, table)
	if roll.calls != 1 || result.Outcome != OutcomeGlance {
		t.Fatalf("inherited white attack changed: rolls=%d outcome=%v", roll.calls, result.Outcome)
	}
	assertFloat64(t, "inherited constant glance", result.Damage, 75)
}
