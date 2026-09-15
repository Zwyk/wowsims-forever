package core

import "testing"

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
