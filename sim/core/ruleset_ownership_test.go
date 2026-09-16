package core

import (
	"reflect"
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

// Deliberately synthetic, not a Classic profile. Distinct coefficients detect
// accidental fallback to the active profile at every owned construction hook.
func ownershipTestRules() rulesetProfile {
	rules := inheritedTBCRuleset()
	rules.levels.characterLevel = 60
	rules.attributes.healthPerStamina = 3
	rules.attributes.playerHealthOffset = -10
	rules.attributes.armorPerAgility = 4
	rules.attributes.manaPerIntellect = 5
	rules.attributes.manaOffset = -20
	rules.attributes.classes[proto.Class_ClassPaladin] = classAttributeRules{
		attackPowerPerStrength: 2.5, attackPowerPerAgility: 1.5,
		rangedAttackPowerPerAgility: 0.5, physicalCritPercentPerAgility: 0.2,
		spellCritPercentPerIntellect: 0.3,
	}
	rules.ratings.physicalHitRatingPerHitPercent = 2
	rules.ratings.spellHitRatingPerHitPercent = 7
	rules.ratings.physicalCritRatingPerCritPercent = 11
	rules.ratings.spellCritRatingPerCritPercent = 22
	rules.ratings.dodgeRatingPerDodgePercent = 5
	rules.ratings.parryRatingPerParryPercent = 6
	rules.ratings.blockRatingPerBlockPercent = 7
	rules.ratings.defenseRatingPerDefenseLevel = 10
	rules.ratings.defenseChancePerDefenseLevelPercent = 0.5
	rules.ratings.resilienceRatingPerCritReductionPercent = 3
	rules.ratings.expertisePerQuarterPercentReduction = 10
	rules.combat.outcomes.expertiseAvoidanceStepsPerUnit = 200
	rules.combat.outcomes.magicCritDamageMultiplier = 1.75
	rules.combat.versusEnemy[levelBandCharacterPlusThree].baseMissChance = 0.17
	rules.combat.targetPhysicalCritByLevel[levelBandCharacterPlusThree] = 13
	return rules
}

func ownershipTestBaseStats() stats.Stats {
	return stats.Stats{
		stats.Strength: 10, stats.Agility: 20, stats.Stamina: 30,
		stats.Intellect: 40, stats.Spirit: 50,
		stats.Health: 500, stats.Mana: 600, stats.AttackPower: 7, stats.RangedAttackPower: 11,
		stats.HitRating: 14, stats.CritRating: 22,
		stats.DodgeRating: 15, stats.ParryRating: 24, stats.BlockRating: 14, stats.BlockPercent: 0.01,
		stats.DefenseRating: 49, stats.ResilienceRating: 12, stats.ExpertiseRating: 20,
	}
}

func ownershipTestCharacter(rules rulesetProfile, raw stats.Stats) Character {
	return newCharacterWithRuleset(&Party{Raid: &Raid{}}, 0, &proto.Player{
		Race: proto.Race_RaceHuman, Class: proto.Class_ClassPaladin,
		Spec: &proto.Player_RetributionPaladin{}, Equipment: &proto.EquipmentSpec{},
	}, rules, raw)
}

func TestRulesetOwnershipCharacterConstructionAndDeferredHooks(t *testing.T) {
	active := currentRuleset()
	rules := ownershipTestRules()
	raw := ownershipTestBaseStats()
	character := ownershipTestCharacter(rules, raw)
	if !character.rulesInitialized || character.Level != 60 || character.resolvedRuleset() != rules || character.GetBaseStats() != raw {
		t.Fatal("character did not retain its explicit rules and raw base-stat seed")
	}
	// Mutating caller-owned input must not affect deferred class/mana hooks.
	rules.attributes.classes[proto.Class_ClassPaladin] = classAttributeRules{}
	rules.attributes.manaPerIntellect = 99
	raw[stats.Mana] = 9999
	character.AddBaseClassStatDependencies()
	character.EnableManaBar()
	got := character.SortAndApplyStatDependencies(character.GetStats())
	want := ownershipTestBaseStats()
	want[stats.Health], want[stats.Mana], want[stats.Armor] = 580, 780, 80
	want[stats.AttackPower], want[stats.RangedAttackPower] = 62, 21
	want[stats.PhysicalHitPercent], want[stats.SpellHitPercent] = 7, 2
	want[stats.PhysicalCritPercent], want[stats.SpellCritPercent] = 6, 13
	for stat := range want {
		assertFloat64(t, stats.Stat(stat).StatName(), got[stat], want[stat])
	}
	if !character.HasManaBar() || character.BaseMana != 600 || character.GetBaseStats() != ownershipTestBaseStats() {
		t.Fatal("mana setup changed the explicit raw base-mana seed")
	}
	if currentRuleset() != active || CharacterLevel != 70 {
		t.Fatal("synthetic ownership test changed the active build-time rules")
	}
}

func TestRulesetOwnershipAvoidanceAndSpellFallbacks(t *testing.T) {
	character := ownershipTestCharacter(ownershipTestRules(), ownershipTestBaseStats())
	assertFloat64(t, "owned dodge", character.GetDodgeFromRating(), 0.03)
	assertFloat64(t, "owned parry", character.GetParryFromRating(), 0.04)
	assertFloat64(t, "owned block", character.GetBlockFromRating(), 0.03)
	assertFloat64(t, "owned defense", character.GetDefenseReduction(), 0.02)
	assertFloat64(t, "owned resilience", character.GetResilienceReduction(), 0.04)
	character.PseudoStats.BaseReducedCritTakenPercent = 0.01
	character.updateReducedCritTakenPercent()
	assertFloat64(t, "owned reduced crit", character.PseudoStats.ReducedCritTakenPercent, 0.07)
	chances := getCritChances(0.5, &character.Unit)
	assertFloat64(t, "owned actual crit", chances.actual, 0.43)
	assertFloat64(t, "owned suppressed crit", chances.suppressed, 0.04)
	spell := &Spell{Unit: &character.Unit, DefenseType: DefenseTypeMagic, CritMultiplierPct: 1}
	assertFloat64(t, "owned expertise", spell.DodgeParrySuppression(), 0.01)
	assertFloat64(t, "owned healing crit base", spell.CritDamageMultiplier(nil), 1.75)
	assertFloat64(t, "owned contextless crit base", spell.CritDamageMultiplier(&AttackTable{CritMultiplier: 1}), 1.75)
}

func TestRulesetOwnershipPetInheritsSnapshotWithoutPlayerDependencies(t *testing.T) {
	owner := ownershipTestCharacter(ownershipTestRules(), ownershipTestBaseStats())
	pet := NewPet(PetConfig{Owner: &owner, Name: "rules owner", BaseStats: ownershipTestBaseStats()})
	if !pet.rulesInitialized || pet.Level != 60 || pet.resolvedRuleset() != owner.resolvedRuleset() {
		t.Fatal("pet did not inherit its owner's rule snapshot")
	}
	owner.rules.attributes.healthPerStamina = 99
	owner.rules.ratings.dodgeRatingPerDodgePercent = 99
	pet.Class = proto.Class_ClassPaladin
	pet.AddBaseClassStatDependencies()
	pet.EnableManaBar()
	got := pet.SortAndApplyStatDependencies(pet.GetStats())
	want := ownershipTestBaseStats()
	want[stats.Health], want[stats.Armor] = 590, 80
	want[stats.PhysicalHitPercent], want[stats.SpellHitPercent] = 7, 2
	want[stats.PhysicalCritPercent], want[stats.SpellCritPercent] = 2, 1
	for stat := range want {
		assertFloat64(t, stats.Stat(stat).StatName(), got[stat], want[stat])
	}
	assertFloat64(t, "pet snapshot dodge", pet.GetDodgeFromRating(), 0.03)
	if pet.BaseMana != 600 {
		t.Fatal("pet acquired a player mana offset")
	}
}

func TestRulesetOwnershipTargetAndAttackTableSnapshots(t *testing.T) {
	rules := ownershipTestRules()
	character := ownershipTestCharacter(rules, ownershipTestBaseStats())
	target := newTargetWithRuleset(&proto.Target{Stats: ownershipTestBaseStats().ToProtoArray()}, 0, rules)
	if !target.rulesInitialized || target.Level != 63 || target.resolvedRuleset() != rules {
		t.Fatal("target did not retain its constructor's rules")
	}
	assertFloat64(t, "target crit seed", target.GetStat(stats.PhysicalCritPercent), 13)
	assertFloat64(t, "target owned dodge", target.GetDodgeFromRating(), 0.03)
	targetStats := target.SortAndApplyStatDependencies(target.GetStats())
	assertFloat64(t, "target physical hit", targetStats[stats.PhysicalHitPercent], 7)
	assertFloat64(t, "target spell hit", targetStats[stats.SpellHitPercent], 2)
	assertFloat64(t, "target converted crit", targetStats[stats.PhysicalCritPercent], 15)
	table := NewAttackTable(&character.Unit, &target.Unit)
	assertFloat64(t, "owned table seed", table.BaseMissChance, 0.17)
	if table.resolvedRatingRules() != rules.ratings || table.resolvedOutcomeRules() != rules.combat.outcomes {
		t.Fatal("public attack table did not capture the attacker's rules")
	}
	// Explicit component-test injection is allowed without rewriting either
	// endpoint. Encounter-wide compatibility is not silently inferred here.
	beforeAttacker, beforeDefender := character.resolvedRuleset(), target.resolvedRuleset()
	explicit := inheritedTBCRuleset()
	override := newAttackTableWithRuleset(&character.Unit, &target.Unit, explicit)
	if override.resolvedRatingRules() != explicit.ratings || character.resolvedRuleset() != beforeAttacker || target.resolvedRuleset() != beforeDefender {
		t.Fatal("explicit attack-table injection modified endpoint ownership")
	}
	legacy := &AttackTable{Attacker: &character.Unit}
	if legacy.resolvedRatingRules() != rules.ratings || legacy.resolvedOutcomeRules() != rules.combat.outcomes {
		t.Fatal("legacy table with an owner fell back to global rules")
	}
	character.rules.ratings.dodgeRatingPerDodgePercent = 999
	character.rules.combat.outcomes.magicCritDamageMultiplier = 999
	rules.ratings.physicalHitRatingPerHitPercent = 999
	if table.resolvedRatingRules() != ownershipTestRules().ratings || table.resolvedOutcomeRules() != ownershipTestRules().combat.outcomes {
		t.Fatal("engine attack table did not preserve its value snapshot")
	}
	if target.resolvedRuleset() != ownershipTestRules() {
		t.Fatal("target snapshot aliases its constructor input")
	}
}

func TestRulesetOwnershipZeroValueAndActiveConstructionCompatibility(t *testing.T) {
	active := currentRuleset()
	unit := Unit{}
	if unit.resolvedRuleset() != active {
		t.Fatal("zero-value unit lost active-profile fallback")
	}
	copy := unit.resolvedRuleset()
	copy.levels.characterLevel = 1
	if unit.resolvedRuleset() != active || currentRuleset() != active {
		t.Fatal("resolved fallback exposes shared mutable rules")
	}
	table := &AttackTable{}
	if table.resolvedRatingRules() != active.ratings || table.resolvedOutcomeRules() != active.combat.outcomes {
		t.Fatal("contextless legacy table lost active-profile fallback")
	}
	character := NewCharacter(&Party{}, 0, &proto.Player{
		Race: proto.Race_RaceHuman, Class: proto.Class_ClassWarrior,
		Spec: &proto.Player_DpsWarrior{}, Equipment: &proto.EquipmentSpec{},
	})
	if !character.rulesInitialized || character.Level != 70 || character.resolvedRuleset() != active || character.GetBaseStats() != BaseStats[BaseStatsKey{Race: proto.Race_RaceHuman, Class: proto.Class_ClassWarrior}] {
		t.Fatal("public character constructor changed the active TBC baseline")
	}
	resolved := character.SortAndApplyStatDependencies(character.GetStats())
	assertFloat64(t, "active health", resolved[stats.Health], 5594)
	if currentRuleset() != active {
		t.Fatal("ownership construction mutated active rules")
	}
}

func TestRulesetOwnershipNarrowGettersAndNilFallback(t *testing.T) {
	active := currentRuleset()
	for _, unit := range []*Unit{nil, {}} {
		if unit.resolvedRuleset() != active || unit.resolvedRatingRules() != active.ratings || unit.resolvedOutcomeRules() != active.combat.outcomes {
			t.Fatal("nil/zero unit lost the active-profile fallback")
		}
	}
	rules := ownershipTestRules()
	unit := Unit{rules: rules, rulesInitialized: true}
	if unit.resolvedRatingRules() != rules.ratings || unit.resolvedOutcomeRules() != rules.combat.outcomes {
		t.Fatal("narrow getters ignored the owned profile")
	}
	ratings, outcomes := unit.resolvedRatingRules(), unit.resolvedOutcomeRules()
	ratings.dodgeRatingPerDodgePercent = -999
	outcomes.magicCritDamageMultiplier = -999
	if unit.resolvedRatingRules() != rules.ratings || unit.resolvedOutcomeRules() != rules.combat.outcomes || currentRuleset() != active {
		t.Fatal("narrow getters exposed mutable shared rules")
	}
	// Initialization state, not non-zero contents, distinguishes a selected
	// profile from the fallback. Narrow getters must preserve this invariant.
	zeroSelected := Unit{rulesInitialized: true}
	if zeroSelected.resolvedRatingRules() != (ratingRules{}) || zeroSelected.resolvedOutcomeRules() != (outcomeRules{}) {
		t.Fatal("explicit zero subprofiles were replaced by fallback rules")
	}

	// The versus-enemy seed path historically permits a contextless attacker;
	// it only needs the defender's level. Preserve that constructor behavior.
	enemy := &Unit{Type: EnemyUnit, Level: active.levels.defaultBossLevel()}
	got := NewAttackTable(nil, enemy)
	want := NewAttackTable(&Unit{}, enemy)
	want.Attacker = nil
	if !reflect.DeepEqual(got, want) {
		t.Fatal("nil-attacker enemy table differs from the active-profile fallback")
	}
}

func TestRulesetOwnershipCritMultiplierContextPrecedence(t *testing.T) {
	owner := ownershipTestCharacter(ownershipTestRules(), ownershipTestBaseStats())
	spell := &Spell{Unit: &owner.Unit, DefenseType: DefenseTypeMagic, CritMultiplierPct: 1}
	otherRules := ownershipTestRules()
	otherRules.combat.outcomes.magicCritDamageMultiplier = 1.25
	other := Unit{rules: otherRules, rulesInitialized: true}
	cached := otherRules.combat.outcomes
	cached.magicCritDamageMultiplier = 2.5
	for _, test := range []struct {
		name  string
		table *AttackTable
		want  float64
	}{
		{"healing owns its rules", nil, 1.75},
		{"contextless multiplier only", &AttackTable{CritMultiplier: 1.2}, 2.1},
		{"legacy attacker owns rules", &AttackTable{Attacker: &other, CritMultiplier: 1}, 1.25},
		{"cached rules precede both owners", &AttackTable{Attacker: &other, rulesInitialized: true, outcomes: cached, CritMultiplier: 1}, 2.5},
		{"cached contextless rules", &AttackTable{rulesInitialized: true, outcomes: cached, CritMultiplier: 1.2}, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertFloat64(t, "critical damage multiplier", spell.CritDamageMultiplier(test.table), test.want)
		})
	}
}
