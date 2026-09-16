package core

import "github.com/wowsims/tbc/sim/core/proto"

// classicReferenceBaselineChances uses percentage points, not probabilities or
// rating units. Parry/block are raw seeds: usable avoidance must also respect
// capability, equipment and combat-state gates. Every naked baseline has no
// shield, so CanBlock is deliberately not inferred from these chance values.
type classicReferenceBaselineChances struct {
	physicalCritPercent, spellCritPercent float64
	dodgePercent, dodgePercentPerAgility  float64
	parryPercent, blockPercent            float64
	canParry                              bool
}

// classicReferenceLevel60BaselineChances characterizes ClassBaseCrit, universal
// parry/block seeds and the dependencies actually installed by class constructors:
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/base_stats.go#L84-L196
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/character.go#L280-L290
// Adapted under MIT (copyright 2024 wowsims team; see LICENSE).
//
// Source parity is not confirmation of game behavior. Hunter, Priest and Mage
// constructors omit their populated Agility-to-Dodge map entries; preserve that
// omission explicitly, not as a claim that those classes cannot gain dodge from
// Agility in-game. Shaman's talent-gated Parry is not a baseline capability.
// This excludes defense bonuses, racials, talents, forms, pets and rounding.
// Only inactive baseline composition consumes this; no live profile selects it
// and no active rating constant is consulted.
func classicReferenceLevel60BaselineChances(base rulesetProfile, class proto.Class) (classicReferenceBaselineChances, bool) {
	if base.levels.characterLevel != classicReferenceCharacterLevel {
		return classicReferenceBaselineChances{}, false
	}

	chances := classicReferenceBaselineChances{parryPercent: 5, blockPercent: 5}
	switch class {
	case proto.Class_ClassWarrior:
		chances.dodgePercentPerAgility = 0.05
		chances.canParry = true
	case proto.Class_ClassPaladin:
		chances.physicalCritPercent, chances.spellCritPercent, chances.dodgePercent = 0.7, 3.5, 0.7
		// paladin.go:151 uses the CritPerAgi row, numerically equal to DodgePerAgi.
		chances.dodgePercentPerAgility = 0.0506
		chances.canParry = true
	case proto.Class_ClassHunter:
		chances.spellCritPercent = 3.6
		// hunter.go:279-283 installs no Agility-to-Dodge dependency.
		chances.canParry = true
	case proto.Class_ClassRogue:
		chances.dodgePercentPerAgility = 0.069
		chances.canParry = true
	case proto.Class_ClassPriest:
		chances.physicalCritPercent, chances.spellCritPercent, chances.dodgePercent = 3, 0.8, 3
		// priest.go:115-118 installs no Agility-to-Dodge dependency.
	case proto.Class_ClassShaman:
		chances.physicalCritPercent, chances.spellCritPercent, chances.dodgePercent = 1.7, 2.3, 1.7
		chances.dodgePercentPerAgility = 0.0508
	case proto.Class_ClassMage:
		chances.physicalCritPercent, chances.spellCritPercent, chances.dodgePercent = 3.2, 0.2, 3.2
		// mage.go:134-137 installs no Agility-to-Dodge dependency.
	case proto.Class_ClassWarlock:
		chances.physicalCritPercent, chances.spellCritPercent, chances.dodgePercent = 2, 1.7, 2
		chances.dodgePercentPerAgility = 0.05
	case proto.Class_ClassDruid:
		chances.physicalCritPercent, chances.spellCritPercent, chances.dodgePercent = 0.9, 1.8, 0.9
		chances.dodgePercentPerAgility = 0.05
	default:
		return classicReferenceBaselineChances{}, false
	}
	return chances, true
}
