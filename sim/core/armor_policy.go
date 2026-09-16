package core

import "math"

// mitigationRules is cached with an attack table's offensive rules. Each
// model names a complete numerical policy; the Classic model has an explicit
// level-60/+3 support envelope instead of borrowing TBC coefficients.
type mitigationRules struct {
	armorModel      armorMitigationModel
	resistanceModel resistanceMitigationModel
}

type armorMitigationModel uint8

const (
	armorMitigationInheritedTBC armorMitigationModel = iota
	armorMitigationClassicReference60
)

type resistanceMitigationModel uint8

const (
	resistanceMitigationInheritedTBC resistanceMitigationModel = iota
	resistanceMitigationClassicReference60
)

func classicReferenceMitigationScope() rulesetProfile {
	return rulesetProfile{levels: levelRules{
		characterLevel:        classicReferenceCharacterLevel,
		defaultBossLevelDelta: classicReferenceDefaultBossLevelDelta,
	}}
}

// classicReferenceArmorInput contains only the numeric inputs consumed by the
// pinned Classic armor formula. DefenderArmor is the already-resolved,
// nonnegative value after the target's armor multiplier. FlatArmorPenetration
// is subtracted in armor points; this policy makes no rating or percent claim.
type classicReferenceArmorInput struct {
	attackerLevel        int32
	defenderArmor        float64
	flatArmorPenetration float64
}

// classicReferenceArmorDamageModifier reproduces wowsims/classic commit
// 7779ebbf79dc7f1341e6ab939b28a3402c9a730a for the pinned level-60 world.
// Explicit internal Classic attack tables and the standalone baseline lab use
// this policy. The default live simulator still selects inherited TBC rules.
//
// The actual attacker may be anywhere from level 1 through the default +3
// boss because the source formula uses attacker level for outgoing and
// incoming damage. Unsupported profiles and malformed numeric inputs fail
// closed. The source has no explicit 75% mitigation cap.
func classicReferenceArmorDamageModifier(base rulesetProfile, input classicReferenceArmorInput) (float64, bool) {
	if base.levels.characterLevel != classicReferenceCharacterLevel ||
		base.levels.defaultBossLevelDelta != classicReferenceDefaultBossLevelDelta ||
		input.attackerLevel < 1 || input.attackerLevel > base.levels.defaultBossLevel() ||
		input.defenderArmor < 0 || input.flatArmorPenetration < 0 ||
		math.IsNaN(input.defenderArmor) || math.IsInf(input.defenderArmor, 0) ||
		math.IsNaN(input.flatArmorPenetration) || math.IsInf(input.flatArmorPenetration, 0) {
		return 0, false
	}

	effectiveArmor := max(input.defenderArmor-input.flatArmorPenetration, 0)
	armorConstant := 400 + 85*float64(input.attackerLevel)
	return 1 - effectiveArmor/(effectiveArmor+armorConstant), true
}
