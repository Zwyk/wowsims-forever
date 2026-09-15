package core

const (
	inheritedTBCCharacterLevel        = 70
	inheritedTBCDefaultBossLevelDelta = 3
)

// inheritedTBCRuleset is the behavior imported from tbc-new v0.0.137. These
// values are refactoring anchors, not evidence for Forever mechanics.
func inheritedTBCRuleset() rulesetProfile {
	return rulesetProfile{
		id: rulesetInheritedTBC,
		levels: levelRules{
			characterLevel:        inheritedTBCCharacterLevel,
			defaultBossLevelDelta: inheritedTBCDefaultBossLevelDelta,
		},
		combat: combatRules{
			versusEnemy: attackTableByLevel{
				levelBandCharacterMinusTwo: {
					baseMissChance:      0.04,
					baseSpellMissChance: 0.02,
					baseBlockChance:     0.05,
					baseDodgeChance:     0.04,
					baseParryChance:     0.04,
					glanceMultiplier:    0.95,
				},
				levelBandCharacter: {
					baseMissChance:      0.05,
					baseSpellMissChance: 0.04,
					baseBlockChance:     0.05,
					baseDodgeChance:     0.05,
					baseParryChance:     0.05,
					baseGlanceChance:    0.06,
					glanceMultiplier:    0.95,
				},
				levelBandCharacterPlusOne: {
					baseMissChance:       0.055,
					baseSpellMissChance:  0.05,
					baseBlockChance:      0.05,
					baseDodgeChance:      0.055,
					baseParryChance:      0.055,
					baseGlanceChance:     0.12,
					glanceMultiplier:     0.95,
					meleeCritSuppression: 0.01,
				},
				levelBandCharacterPlusTwo: {
					baseMissChance:       0.06,
					baseSpellMissChance:  0.06,
					baseBlockChance:      0.05,
					baseDodgeChance:      0.06,
					baseParryChance:      0.06,
					baseGlanceChance:     0.18,
					glanceMultiplier:     0.85,
					meleeCritSuppression: 0.02,
					spellCritSuppression: 0.003,
				},
				levelBandCharacterPlusThree: {
					baseMissChance:       0.08,
					baseSpellMissChance:  0.17,
					baseBlockChance:      0.05,
					baseDodgeChance:      0.065,
					baseParryChance:      0.14,
					baseGlanceChance:     0.24,
					glanceMultiplier:     0.75,
					meleeCritSuppression: 0.048,
					spellCritSuppression: 0.021,
					hitSuppression:       0.01,
				},
			},
			versusNonEnemy: attackTableByLevel{
				levelBandCharacterMinusTwo: {
					baseMissChance:      0.054,
					baseSpellMissChance: 0.05,
					baseBlockChance:     0.054,
					baseDodgeChance:     0.004,
					baseParryChance:     0.054,
				},
				levelBandCharacter: {
					baseMissChance:      0.05,
					baseSpellMissChance: 0.05,
					baseBlockChance:     0.05,
					baseParryChance:     0.05,
				},
				levelBandCharacterPlusOne: {
					baseMissChance:      0.048,
					baseSpellMissChance: 0.05,
					baseBlockChance:     0.048,
					baseDodgeChance:     -0.002,
					baseParryChance:     0.048,
				},
				levelBandCharacterPlusTwo: {
					baseMissChance:      0.046,
					baseSpellMissChance: 0.05,
					baseBlockChance:     0.046,
					baseDodgeChance:     -0.004,
					baseParryChance:     0.046,
				},
				levelBandCharacterPlusThree: {
					baseMissChance:      0.044,
					baseSpellMissChance: 0.05,
					baseBlockChance:     0.044,
					baseDodgeChance:     -0.006,
					baseParryChance:     0.044,
					baseCrushChance:     0.15,
				},
			},
		},
	}
}
