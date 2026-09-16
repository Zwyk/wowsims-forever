package core

import "github.com/wowsims/tbc/sim/core/proto"

// These values characterize the inherited class constructors, not inferred
// Classic/Forever behavior. In particular, Mage/Priest install no Strength-to-
// AP dependency, and Druid forms add their own dynamic dependencies elsewhere.
// The crit values match base_stats_auto_gen.go; parity tests keep its remaining
// compatibility/pet consumers aligned without making this profile map-backed.
func inheritedTBCAttributeRules() attributeRules {
	return attributeRules{
		healthPerStamina: 10,
		armorPerAgility:  2,
		manaPerIntellect: 15,
		manaOffset:       -280,
		classes: [proto.Class_ClassDruid + 1]classAttributeRules{
			proto.Class_ClassWarrior: {
				attackPowerPerStrength: 2, physicalCritPercentPerAgility: 0.0303,
			},
			proto.Class_ClassPaladin: {
				attackPowerPerStrength: 2, physicalCritPercentPerAgility: 0.04, spellCritPercentPerIntellect: 0.0125,
			},
			proto.Class_ClassHunter: {
				attackPowerPerStrength: 1, attackPowerPerAgility: 1, rangedAttackPowerPerAgility: 1,
				physicalCritPercentPerAgility: 0.025, spellCritPercentPerIntellect: 0.0125,
			},
			proto.Class_ClassRogue: {
				attackPowerPerStrength: 1, attackPowerPerAgility: 1, physicalCritPercentPerAgility: 0.025,
			},
			proto.Class_ClassPriest: {
				physicalCritPercentPerAgility: 0.04, spellCritPercentPerIntellect: 0.0125,
			},
			proto.Class_ClassShaman: {
				attackPowerPerStrength: 2, physicalCritPercentPerAgility: 0.04, spellCritPercentPerIntellect: 0.0125,
			},
			proto.Class_ClassMage: {
				physicalCritPercentPerAgility: 0.04, spellCritPercentPerIntellect: 0.0125,
			},
			proto.Class_ClassWarlock: {
				attackPowerPerStrength: 1, physicalCritPercentPerAgility: 0.0405, spellCritPercentPerIntellect: 0.0122,
			},
			proto.Class_ClassDruid: {
				attackPowerPerStrength: 1, physicalCritPercentPerAgility: 0.04, spellCritPercentPerIntellect: 0.0125,
			},
		},
	}
}
