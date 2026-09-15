package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func TestInheritedTBCRatingRules(t *testing.T) {
	ratings := inheritedTBCRuleset().ratings
	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{"expertise per quarter percent", ratings.expertisePerQuarterPercentReduction, 3.942308},
		{"defense rating per level", ratings.defenseRatingPerDefenseLevel, 2.365385},
		{"dodge rating per percent", ratings.dodgeRatingPerDodgePercent, 18.923079},
		{"parry rating per percent", ratings.parryRatingPerParryPercent, 23.653847},
		{"block rating per percent", ratings.blockRatingPerBlockPercent, 7.884615},
		{"physical hit rating per percent", ratings.physicalHitRatingPerHitPercent, 15.769233},
		{"spell hit rating per percent", ratings.spellHitRatingPerHitPercent, 12.615385},
		{"physical crit rating per percent", ratings.physicalCritRatingPerCritPercent, 22.076923},
		{"spell crit rating per percent", ratings.spellCritRatingPerCritPercent, 22.076923},
		{"physical haste rating per percent", ratings.physicalHasteRatingPerHastePercent, 15.769233},
		{"spell haste rating per percent", ratings.spellHasteRatingPerHastePercent, 15.76923},
		{"defense chance per level", ratings.defenseChancePerDefenseLevelPercent, 0.04},
		{"resilience rating per crit reduction", ratings.resilienceRatingPerCritReductionPercent, 39.4231},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertFloat64(t, test.name, test.got, test.want)
		})
	}
}

func TestInheritedTBCCombatRules(t *testing.T) {
	rules := inheritedTBCRuleset()
	wantTargetCrit := [levelBandCount]float64{4.6, 5.0, 5.2, 5.4, 5.6}
	for band, want := range wantTargetCrit {
		assertFloat64(t, "target physical crit", rules.combat.targetPhysicalCritByLevel[band], want)
	}

	outcomes := rules.combat.outcomes
	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{"expertise steps per unit", outcomes.expertiseAvoidanceStepsPerUnit, 400},
		{"dual-wield miss penalty", outcomes.dualWieldMissPenalty, 0.19},
		{"minimum spell miss", outcomes.minimumSpellMissChance, 0.01},
		{"magic crit damage", outcomes.magicCritDamageMultiplier, 1.5},
		{"melee crit damage", outcomes.meleeCritDamageMultiplier, 2},
		{"ranged crit damage", outcomes.rangedCritDamageMultiplier, 2},
		{"enemy crit damage", outcomes.enemyCritDamageMultiplier, 2},
		{"resilience crit damage scale", outcomes.resilienceCritDamageReductionScale, 0.5},
		{"crushing blow damage", outcomes.crushingBlowDamageMultiplier, 1.5},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertFloat64(t, test.name, test.got, test.want)
		})
	}
}

func TestActiveCompatibilityConstantsMatchRuleset(t *testing.T) {
	rules := currentRuleset()
	if int32(CharacterLevel) != rules.levels.characterLevel {
		t.Fatalf("character level: constant=%d, ruleset=%d", CharacterLevel, rules.levels.characterLevel)
	}
	if int32(DefaultBossLevel) != rules.levels.defaultBossLevel() {
		t.Fatalf("boss level: constant=%d, ruleset=%d", DefaultBossLevel, rules.levels.defaultBossLevel())
	}

	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{"expertise", ExpertisePerQuarterPercentReduction, rules.ratings.expertisePerQuarterPercentReduction},
		{"defense rating", DefenseRatingPerDefenseLevel, rules.ratings.defenseRatingPerDefenseLevel},
		{"dodge rating", DodgeRatingPerDodgePercent, rules.ratings.dodgeRatingPerDodgePercent},
		{"parry rating", ParryRatingPerParryPercent, rules.ratings.parryRatingPerParryPercent},
		{"block rating", BlockRatingPerBlockPercent, rules.ratings.blockRatingPerBlockPercent},
		{"physical hit", PhysicalHitRatingPerHitPercent, rules.ratings.physicalHitRatingPerHitPercent},
		{"spell hit", SpellHitRatingPerHitPercent, rules.ratings.spellHitRatingPerHitPercent},
		{"physical crit", PhysicalCritRatingPerCritPercent, rules.ratings.physicalCritRatingPerCritPercent},
		{"spell crit", SpellCritRatingPerCritPercent, rules.ratings.spellCritRatingPerCritPercent},
		{"physical haste", PhysicalHasteRatingPerHastePercent, rules.ratings.physicalHasteRatingPerHastePercent},
		{"spell haste", SpellHasteRatingPerHastePercent, rules.ratings.spellHasteRatingPerHastePercent},
		{"defense chance", MissDodgeParryBlockCritChancePerDefense, rules.ratings.defenseChancePerDefenseLevelPercent},
		{"resilience", ResilienceRatingPerCritReductionChance, rules.ratings.resilienceRatingPerCritReductionPercent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertFloat64(t, test.name, test.got, test.want)
		})
	}
}

func TestActiveConstructorsCacheCurrentRuleset(t *testing.T) {
	rules := currentRuleset()
	player := &Unit{Type: PlayerUnit, Level: rules.levels.characterLevel}
	boss := &Unit{Type: EnemyUnit, Level: rules.levels.defaultBossLevel()}
	table := NewAttackTable(player, boss)
	if !table.rulesInitialized {
		t.Fatal("active attack table did not cache its ruleset")
	}
	if table.resolvedRatingRules() != rules.ratings {
		t.Fatal("active attack table cached different rating rules")
	}
	if table.resolvedOutcomeRules() != rules.combat.outcomes {
		t.Fatal("active attack table cached different outcome rules")
	}

	target := NewTarget(&proto.Target{}, 0)
	if target.Level != rules.levels.defaultBossLevel() {
		t.Fatalf("active target level: got %d, want %d", target.Level, rules.levels.defaultBossLevel())
	}
	assertFloat64(t, "active target physical crit", target.stats[stats.PhysicalCritPercent], rules.targetPhysicalCritPercent(target.Level))
}

func TestLegacyAttackTableLiteralFallsBackToCurrentRuleset(t *testing.T) {
	rules := currentRuleset()
	table := &AttackTable{}
	if table.resolvedRatingRules() != rules.ratings {
		t.Fatal("legacy attack table did not fall back to active rating rules")
	}
	if table.resolvedOutcomeRules() != rules.combat.outcomes {
		t.Fatal("legacy attack table did not fall back to active outcome rules")
	}
}

func TestRulesetAttackTableSelectionUsesRebasedLevelBandsAndDirection(t *testing.T) {
	rules := currentRuleset()
	rules.levels.characterLevel = 60
	rules.levels.defaultBossLevelDelta = 3

	tests := []struct {
		name  string
		band  levelBand
		level int32
	}{
		{"minus-two", levelBandCharacterMinusTwo, rules.levels.characterLevel - 2},
		{"same-level", levelBandCharacter, rules.levels.characterLevel},
		{"plus-one", levelBandCharacterPlusOne, rules.levels.characterLevel + 1},
		{"plus-two", levelBandCharacterPlusTwo, rules.levels.characterLevel + 2},
		{"plus-three-boss", levelBandCharacterPlusThree, rules.levels.defaultBossLevel()},
	}

	for index, test := range tests {
		versusEnemyMarker := float64(index+1) / 100
		versusNonEnemyMarker := float64(index+11) / 100
		rules.combat.versusEnemy[test.band].baseMissChance = versusEnemyMarker
		rules.combat.versusNonEnemy[test.band].baseMissChance = versusNonEnemyMarker

		t.Run(test.name, func(t *testing.T) {
			player := Unit{Type: PlayerUnit, Level: rules.levels.characterLevel}
			enemy := Unit{Type: EnemyUnit, Level: test.level}

			versusEnemy := newAttackTableWithRuleset(&player, &enemy, rules)
			assertFloat64(t, "versus-enemy row", versusEnemy.BaseMissChance, versusEnemyMarker)

			versusNonEnemy := newAttackTableWithRuleset(&enemy, &player, rules)
			assertFloat64(t, "versus-non-enemy row", versusNonEnemy.BaseMissChance, versusNonEnemyMarker)
		})
	}
}

func TestUnitLevelFloat64UsesActiveRulesetBands(t *testing.T) {
	rules := currentRuleset()
	tests := []struct {
		name  string
		level int32
		want  float64
	}{
		{"minus-two", rules.levels.characterLevel - 2, 1},
		{"same-level", rules.levels.characterLevel, 2},
		{"plus-one", rules.levels.characterLevel + 1, 3},
		{"plus-two", rules.levels.characterLevel + 2, 4},
		{"boss", rules.levels.defaultBossLevel(), 5},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertFloat64(t, "selected value", UnitLevelFloat64(test.level, 1, 2, 3, 4, 5), test.want)
		})
	}
}

func TestRulesetTargetPhysicalCritUsesRebasedLevelBands(t *testing.T) {
	rules := currentRuleset()
	rules.levels.characterLevel = 60
	rules.levels.defaultBossLevelDelta = 3

	tests := []struct {
		name  string
		band  levelBand
		level int32
	}{
		{"minus-two", levelBandCharacterMinusTwo, rules.levels.characterLevel - 2},
		{"same-level", levelBandCharacter, rules.levels.characterLevel},
		{"plus-one", levelBandCharacterPlusOne, rules.levels.characterLevel + 1},
		{"plus-two", levelBandCharacterPlusTwo, rules.levels.characterLevel + 2},
		{"boss", levelBandCharacterPlusThree, rules.levels.defaultBossLevel()},
	}

	for index, test := range tests {
		marker := float64(index + 11)
		rules.combat.targetPhysicalCritByLevel[test.band] = marker

		t.Run(test.name, func(t *testing.T) {
			target := newTargetWithRuleset(&proto.Target{Level: test.level}, 0, rules)
			assertFloat64(t, "physical crit", target.stats[stats.PhysicalCritPercent], marker)
		})
	}
}

func TestRulesetRatingDependencyInjection(t *testing.T) {
	rules := currentRuleset()
	rules.ratings.physicalHitRatingPerHitPercent = 10
	rules.ratings.spellHitRatingPerHitPercent = 20
	rules.ratings.physicalCritRatingPerCritPercent = 25
	rules.ratings.spellCritRatingPerCritPercent = 40

	unit := Unit{StatDependencyManager: stats.NewStatDependencyManager()}
	unit.addUniversalStatDependenciesWithRuleset(rules)
	got := unit.StatDependencyManager.SortAndApplyStatDependencies(stats.Stats{
		stats.HitRating:       100,
		stats.MeleeHitRating:  50,
		stats.SpellHitRating:  20,
		stats.CritRating:      100,
		stats.MeleeCritRating: 25,
		stats.SpellCritRating: 40,
	})

	assertFloat64(t, "physical hit", got[stats.PhysicalHitPercent], 15)
	assertFloat64(t, "spell hit", got[stats.SpellHitPercent], 6)
	assertFloat64(t, "physical crit", got[stats.PhysicalCritPercent], 5)
	assertFloat64(t, "spell crit", got[stats.SpellCritPercent], 3.5)
}

func TestRulesetOutcomeInjectionDoesNotMutateActiveProfile(t *testing.T) {
	original := currentRuleset()
	rules := original
	rules.ratings.expertisePerQuarterPercentReduction = 10
	rules.ratings.resilienceRatingPerCritReductionPercent = 25
	rules.combat.outcomes.expertiseAvoidanceStepsPerUnit = 250
	rules.combat.outcomes.dualWieldMissPenalty = 0.31
	rules.combat.outcomes.minimumSpellMissChance = 0.07
	rules.combat.outcomes.enemyCritDamageMultiplier = 3
	rules.combat.outcomes.resilienceCritDamageReductionScale = 0.25
	rules.combat.outcomes.crushingBlowDamageMultiplier = 1.75

	player := &Unit{
		Type:        PlayerUnit,
		Level:       rules.levels.characterLevel,
		stats:       stats.Stats{stats.ExpertiseRating: 10, stats.SpellHitPercent: 100},
		PseudoStats: stats.NewPseudoStats(),
	}
	boss := &Unit{
		Type:        EnemyUnit,
		Level:       rules.levels.defaultBossLevel(),
		PseudoStats: stats.NewPseudoStats(),
	}
	playerTable := newAttackTableWithRuleset(player, boss, rules)
	bossTable := newAttackTableWithRuleset(boss, player, rules)

	// Tables own value snapshots, even if the profile used to construct them is
	// changed later by a test or future build-time assembly code.
	rules.ratings.expertisePerQuarterPercentReduction = 99
	rules.combat.outcomes.dualWieldMissPenalty = 0.99

	player.AutoAttacks.IsDualWielding = true
	playerSpell := &Spell{Unit: player}

	assertFloat64(t, "dual-wield miss", playerSpell.GetPhysicalMissChance(playerTable), playerTable.BaseMissChance+0.31)
	assertFloat64(t, "spell miss floor", playerSpell.SpellChanceToMiss(playerTable), 0.07)
	assertFloat64(t, "expertise step", playerSpell.dodgeParrySuppression(playerTable.resolvedRatingRules(), playerTable.resolvedOutcomeRules()), 0.004)

	boss.AutoAttacks.IsDualWielding = true
	boss.PseudoStats.CanCrush = true
	boss.stats[stats.PhysicalCritPercent] = 100
	bossSpell := &Spell{Unit: boss}

	missChance := 0.0
	missResult := &SpellResult{Target: player, Damage: 100}
	if missResult.applyEnemyAttackTableMiss(bossSpell, bossTable, 1, &missChance) {
		t.Fatal("marker enemy attack unexpectedly missed at roll 1")
	}
	assertFloat64(t, "enemy dual-wield miss", missChance, bossTable.BaseMissChance+0.31)

	player.stats[stats.ResilienceRating] = playerTable.resolvedRatingRules().resilienceRatingPerCritReductionPercent * 4
	player.PseudoStats.ReducedCritTakenPercent = 0.04
	critChance := 0.0
	critResult := &SpellResult{Target: player, Damage: 100}
	if !critResult.applyEnemyAttackTableCrit(bossSpell, bossTable, 0.5, &critChance, false) {
		t.Fatal("marker enemy crit did not land")
	}
	assertFloat64(t, "enemy crit damage", critResult.Damage, 297)

	bossTable.BaseCrushChance = 1
	crushChance := 0.0
	crushResult := &SpellResult{Target: player, Damage: 100}
	if !crushResult.applyEnemyAttackTableCrush(bossSpell, bossTable, 0, &crushChance, false) {
		t.Fatal("marker crushing blow did not land")
	}
	assertFloat64(t, "crushing blow damage", crushResult.Damage, 175)

	fresh := currentRuleset()
	assertFloat64(t, "active expertise divisor", fresh.ratings.expertisePerQuarterPercentReduction, original.ratings.expertisePerQuarterPercentReduction)
	assertFloat64(t, "active dual-wield penalty", fresh.combat.outcomes.dualWieldMissPenalty, original.combat.outcomes.dualWieldMissPenalty)
	assertFloat64(t, "active spell miss floor", fresh.combat.outcomes.minimumSpellMissChance, original.combat.outcomes.minimumSpellMissChance)
}
