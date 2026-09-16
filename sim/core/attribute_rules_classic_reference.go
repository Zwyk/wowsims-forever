package core

import "github.com/wowsims/tbc/sim/core/proto"

// classicReferenceLevel60AttributeRules characterizes the dependencies actually
// installed by the nine player classes in the pinned Classic simulator:
// https://github.com/wowsims/classic/tree/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim
// Numeric coefficients: sim/core/base_stats.go:133-182; resource corrections:
// sim/core/character.go:281-284 and sim/core/mana.go:38-49.
// Adapted under MIT (copyright 2024 wowsims team; see LICENSE).
//
// This is not a complete character profile or verified Forever behavior. It
// excludes base crit/dodge, racials, talents, forms, pets and regeneration. Mana
// rules describe the default modifier of 1 for the seven mana-using classes;
// Warrior/Rogue must not install them. The linear resource model assumes at
// least 20 Stamina/Intellect. Race eligibility and fractional-stat rounding are
// separate policies, not inferred here. There is no production caller.
func classicReferenceLevel60AttributeRules(base rulesetProfile) (attributeRules, bool) {
	if base.levels.characterLevel != classicReferenceCharacterLevel {
		return attributeRules{}, false
	}

	return attributeRules{
		healthPerStamina:   10,
		playerHealthOffset: -180,
		armorPerAgility:    2,
		manaPerIntellect:   15,
		manaOffset:         -280,
		classes: [proto.Class_ClassDruid + 1]classAttributeRules{
			proto.Class_ClassWarrior: {
				attackPowerPerStrength: 2, physicalCritPercentPerAgility: 0.05,
			},
			proto.Class_ClassPaladin: {
				attackPowerPerStrength: 2, physicalCritPercentPerAgility: 0.0506, spellCritPercentPerIntellect: 0.0167,
			},
			proto.Class_ClassHunter: {
				// hunter.go:279-283 installs 1 melee AP/Agi despite the map's 0.
				attackPowerPerStrength: 1, attackPowerPerAgility: 1, rangedAttackPowerPerAgility: 2,
				physicalCritPercentPerAgility: 0.0189, spellCritPercentPerIntellect: 0.0165,
			},
			proto.Class_ClassRogue: {
				attackPowerPerStrength: 1, attackPowerPerAgility: 1, rangedAttackPowerPerAgility: 1,
				physicalCritPercentPerAgility: 0.0345,
			},
			proto.Class_ClassPriest: {
				// priest.go:115-118 installs no Agility-to-physical-crit dependency.
				attackPowerPerStrength: 1, spellCritPercentPerIntellect: 0.0168,
			},
			proto.Class_ClassShaman: {
				attackPowerPerStrength: 2, physicalCritPercentPerAgility: 0.0508, spellCritPercentPerIntellect: 0.0169,
			},
			proto.Class_ClassMage: {
				// mage.go:134-137 likewise omits the populated Agility crit row.
				attackPowerPerStrength: 1, spellCritPercentPerIntellect: 0.0168,
			},
			proto.Class_ClassWarlock: {
				attackPowerPerStrength: 1, physicalCritPercentPerAgility: 0.05, spellCritPercentPerIntellect: 0.0165,
			},
			proto.Class_ClassDruid: {
				// The 2 AP/Str baseline already applies in Cat. Its Agility-to-AP
				// dependency is form-only, despite APPerAgility's unconditional 1.
				attackPowerPerStrength: 2, physicalCritPercentPerAgility: 0.05, spellCritPercentPerIntellect: 0.0167,
			},
		},
	}, true
}

// Only the additional AP rules active during Cat Form, not the full form or
// its base AP/Strength dependency. FeralAP is an already-resolved stat input;
// nothing here derives it from equipment. There is deliberately no extra
// Strength coefficient: copying the inherited TBC form hook would double-count
// that contribution on top of Classic's 2 AP/Strength baseline.
type classicReferenceCatAttackPower struct {
	flatBonus, perAgility, perFeralAttackPower float64
}

// classicReferenceLevel60CatAttackPower characterizes executable Cat AP wiring:
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/druid/forms.go#L86-L95
// Adapted under MIT (copyright 2024 wowsims team; see LICENSE).
// It excludes talents, resource/weapon/armor/threat changes and shift behavior.
// The source's Bear implementation is commented out, so there is no generic
// form lookup or inferred Bear fallback. No live form calls this reference.
func classicReferenceLevel60CatAttackPower(base rulesetProfile) (classicReferenceCatAttackPower, bool) {
	if base.levels.characterLevel != classicReferenceCharacterLevel {
		return classicReferenceCatAttackPower{}, false
	}
	return classicReferenceCatAttackPower{flatBonus: 120, perAgility: 1, perFeralAttackPower: 1}, true
}
