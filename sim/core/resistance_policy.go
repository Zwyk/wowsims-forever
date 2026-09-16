package core

import "math"

// classicReferenceResistanceInput contains the numeric inputs consumed after
// a magical school has already selected its defender resistance. The selected
// value may be negative after debuffs; the pinned source floors effective
// resistance only after subtracting flat spell penetration.
type classicReferenceResistanceInput struct {
	attackerLevel        int32
	defenderLevel        int32
	defenderIsEnemy      bool
	selectedResistance   float64
	flatSpellPenetration float64
}

type classicReferencePartialResistView struct {
	coefficient float64
	threshold00 float64
	threshold25 float64
	threshold50 float64
}

type classicReferenceBinaryResistView struct {
	coefficient             float64
	baseHitChanceMultiplier float64
}

// classicReferencePartialResistProjection reproduces the non-binary threshold
// projection in wowsims/classic commit
// 7779ebbf79dc7f1341e6ab939b28a3402c9a730a. Explicit internal Classic attack
// tables select this policy; the default live simulator remains inherited TBC.
// Pure dots divide only the explicit-resistance component by ten; the enemy
// level component is unchanged.
func classicReferencePartialResistProjection(base rulesetProfile, input classicReferenceResistanceInput, pureDot bool) (classicReferencePartialResistView, bool) {
	coefficient, ok := classicReferenceResistanceCoefficient(base, input, false, pureDot)
	if !ok {
		return classicReferencePartialResistView{}, false
	}

	view := classicReferencePartialResistView{coefficient: coefficient}
	if val := coefficient * 3; val <= 1 {
		view.threshold00 = 0.76 * val
		view.threshold25 = 0.21 * val
		view.threshold50 = 0.03 * val
	} else if val <= 2 {
		val -= 1
		view.threshold00 = 0.76 + 0.24*val
		view.threshold25 = 0.21 + 0.57*val
		view.threshold50 = 0.03 + 0.19*val
	} else {
		val -= 2
		view.threshold00 = 1
		view.threshold25 = 0.78 + 0.18*val
		view.threshold50 = 0.22 + 0.58*val
	}
	return view, true
}

// classicReferenceBinaryResistProjection returns the resistance multiplier
// applied to a binary spell's base hit chance. It is not a damage multiplier
// or a final hit chance; base miss, spell Hit and the miss floor remain outside
// this numerical policy.
func classicReferenceBinaryResistProjection(base rulesetProfile, input classicReferenceResistanceInput) (classicReferenceBinaryResistView, bool) {
	coefficient, ok := classicReferenceResistanceCoefficient(base, input, true, false)
	if !ok {
		return classicReferenceBinaryResistView{}, false
	}
	return classicReferenceBinaryResistView{
		coefficient:             coefficient,
		baseHitChanceMultiplier: 1 - 0.75*coefficient,
	}, true
}

func classicReferenceResistanceCoefficient(base rulesetProfile, input classicReferenceResistanceInput, binary bool, pureDot bool) (float64, bool) {
	if base.levels.characterLevel != classicReferenceCharacterLevel ||
		base.levels.defaultBossLevelDelta != classicReferenceDefaultBossLevelDelta ||
		input.attackerLevel < 1 || input.attackerLevel > base.levels.defaultBossLevel() ||
		input.defenderLevel < 1 || input.defenderLevel > base.levels.defaultBossLevel() ||
		math.IsNaN(input.selectedResistance) || math.IsInf(input.selectedResistance, 0) ||
		input.flatSpellPenetration < 0 ||
		math.IsNaN(input.flatSpellPenetration) || math.IsInf(input.flatSpellPenetration, 0) {
		return 0, false
	}

	effectiveResistance := max(input.selectedResistance-input.flatSpellPenetration, 0)
	coefficient := effectiveResistance / float64(input.attackerLevel*5)
	if pureDot {
		coefficient /= 10
	}
	if !binary && input.defenderIsEnemy && input.defenderLevel > input.attackerLevel {
		const averageMitigationPerLevel = 0.02
		coefficient += averageMitigationPerLevel * float64(input.defenderLevel-input.attackerLevel) / 0.75
	}
	return min(1, coefficient), true
}
