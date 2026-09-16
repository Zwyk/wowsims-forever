package core

import (
	"math"

	"github.com/wowsims/tbc/sim/core/stats"
)

func (result *SpellResult) applyResistances(sim *Simulation, spell *Spell, isPeriodic bool, attackTable *AttackTable) {
	if spell.Unit.Type == EnemyUnit && spell.Unit.resolvedRuleset().id == rulesetClassic60PaladinReference && spell.DefenseType == DefenseTypeMagic {
		result.classic60PreMitigationDamage = result.Damage
	}
	resistanceMultiplier, outcome := spell.ResistanceMultiplier(sim, isPeriodic, attackTable)

	result.Damage *= resistanceMultiplier
	result.Outcome |= outcome

	result.ArmorAndResistanceMultiplier = resistanceMultiplier
	result.PostArmorAndResistanceMultiplier = result.Damage
}

// Modifies damage based on Armor or Magic resistances, depending on the damage type.
func (spell *Spell) ResistanceMultiplier(sim *Simulation, isPeriodic bool, attackTable *AttackTable) (float64, HitOutcome) {
	if attackTable.resolvedOutcomeRules().spellChanceModel == spellChanceModelClassicReference60 {
		// Also validate always-hit periodic damage and ignore-resist paths, which
		// need not call a spell hit/crit getter during their outcome step.
		if attackTable.resolvedResourceRules().manaModel == manaModelClassic60PaladinReference && spell.Unit.Type == EnemyUnit {
			if spell.DefenseType == DefenseTypeMagic {
				liveClassic60PaladinIncomingSpellView(spell, attackTable)
			} else {
				validateClassic60PaladinIncoming(spell, attackTable)
			}
		} else if attackTable.resolvedResourceRules().manaModel == manaModelClassic60PaladinReference && (spell.DefenseType == DefenseTypeMelee || spell.classic60PaladinAttack == classic60PaladinHammerOfWrath) {
			livePhysicalAttackTableView(spell, attackTable)
		} else {
			liveClassicSpellChanceView(spell, attackTable)
		}
	}
	if spell.Flags.Matches(SpellFlagIgnoreResists) {
		return 1, OutcomeEmpty
	}
	if attackTable.resolvedMitigationRules().resistanceModel == resistanceMitigationClassicReference60 {
		// Validate school selection even for binary spells or physical hybrids,
		// which otherwise bypass the partial-resistance entry point below.
		attackTable.classicReferenceResistanceInputForSpell(spell)
	}

	if spell.SpellSchool.Matches(SpellSchoolPhysical) {
		// All physical dots (Bleeds) ignore armor.
		if isPeriodic && !spell.Flags.Matches(SpellFlagApplyArmorReduction) {
			return 1, OutcomeEmpty
		}

		// Physical resistance (armor).
		return attackTable.GetArmorDamageModifier(spell), OutcomeEmpty
	}

	// Magical resistance.
	if spell.Flags.Matches(SpellFlagBinary) {
		return 1, OutcomeEmpty
	}

	resistanceRoll := sim.RandomFloat("Partial Resist")

	threshold00, threshold25, threshold50 := attackTable.GetPartialResistThresholds(spell)
	//if sim.Log != nil {
	//	sim.Log("Resist thresholds: %0.04f, %0.04f, %0.04f", threshold00, threshold25, threshold50)
	//}

	if resistanceRoll > threshold00 {
		// No partial resist.
		return 1, OutcomeEmpty
	} else if resistanceRoll > threshold25 {
		return 0.75, OutcomePartial1_4
	} else if resistanceRoll > threshold50 {
		return 0.5, OutcomePartial2_4
	} else {
		return 0.25, OutcomePartial3_4
	}
}

func (at *AttackTable) GetPartialResistThresholds(spell *Spell) (float64, float64, float64) {
	switch at.resolvedMitigationRules().resistanceModel {
	case resistanceMitigationInheritedTBC:
		return at.Defender.partialResistRollThresholds(spell, at.Attacker)
	case resistanceMitigationClassicReference60:
		input, magical := at.classicReferenceResistanceInputForSpell(spell)
		if !magical {
			return 0, 0, 0
		}
		projection, ok := classicReferencePartialResistProjection(classicReferenceMitigationScope(), input, spell.Flags.Matches(SpellFlagPureDot))
		if !ok {
			panic("unsupported Classic-60 partial resistance inputs")
		}
		return projection.threshold00, projection.threshold25, projection.threshold50
	default:
		panic("unsupported resistance mitigation model")
	}
}

func (at *AttackTable) GetBinaryHitChance(spell *Spell) float64 {
	switch at.resolvedMitigationRules().resistanceModel {
	case resistanceMitigationInheritedTBC:
		return at.Defender.binaryHitChance(spell, at.Attacker)
	case resistanceMitigationClassicReference60:
		input, magical := at.classicReferenceResistanceInputForSpell(spell)
		if !magical {
			return 1
		}
		projection, ok := classicReferenceBinaryResistProjection(classicReferenceMitigationScope(), input)
		if !ok {
			panic("unsupported Classic-60 binary resistance inputs")
		}
		return projection.baseHitChanceMultiplier
	default:
		panic("unsupported resistance mitigation model")
	}
}

// This runtime slice supports the seven original, individual damage schools.
// Holy has no explicit resistance stat: using ResistanceStat's zero sentinel
// would incorrectly read Strength. Hybrid-school selection is a separate
// porting task and must not inherit the modern fallback silently.
func (at *AttackTable) classicReferenceResistanceInputForSpell(spell *Spell) (classicReferenceResistanceInput, bool) {
	input := classicReferenceResistanceInput{
		attackerLevel:        at.Attacker.Level,
		defenderLevel:        at.Defender.Level,
		defenderIsEnemy:      at.Defender.Type == EnemyUnit,
		flatSpellPenetration: at.Attacker.stats[stats.SpellPenetration],
	}
	switch spell.SpellSchool {
	case SpellSchoolNone, SpellSchoolPhysical:
		return input, false
	case SpellSchoolHoly:
		// Holy still participates in the non-binary level component.
	case SpellSchoolArcane, SpellSchoolFire, SpellSchoolFrost, SpellSchoolNature, SpellSchoolShadow:
		input.selectedResistance = at.Defender.GetStat(spell.SpellSchool.ResistanceStat())
	default:
		panic("unsupported Classic-60 resistance school")
	}
	return input, true
}

// All of the following calculations are based on this guide:
// https://royalgiraffe.github.io/resist-guide

func (unit *Unit) resistCoeff(spell *Spell, attacker *Unit, binary bool) float64 {
	if spell.SchoolIndex <= stats.SchoolIndexPhysical {
		return 0
	}

	resistance := max(0, unit.GetStat(spell.SpellSchool.ResistanceStat())-attacker.stats[stats.SpellPenetration])
	if resistance <= 0 {
		// Level-based resistance does not apply to binary spells.
		if binary {
			return 0
		}
		return unit.levelBasedResist(attacker)
	}

	resistanceCap := float64(attacker.Level * 5)
	resistanceCoef := resistance / resistanceCap

	if !binary && unit.Type == EnemyUnit && unit.Level > attacker.Level {
		avgMitigationAdded := unit.levelBasedResist(attacker)
		// coef is scaled 0 to 1, not 0 to 0.75
		resistanceCoef += avgMitigationAdded * 1 / 0.75
	}

	return min(1, resistanceCoef)
}

func (unit *Unit) levelBasedResist(attacker *Unit) float64 {
	if unit.Type == EnemyUnit && unit.Level > attacker.Level {
		// 2% average mitigation per level difference
		return 0.02 * float64(unit.Level-attacker.Level)
	}
	return 0
}

func (unit *Unit) binaryHitChance(spell *Spell, attacker *Unit) float64 {
	resistCoeff := unit.resistCoeff(spell, attacker, true)
	return 1 - 0.75*resistCoeff
}

// Roll threshold for each type of partial resist.
func (unit *Unit) partialResistRollThresholds(spell *Spell, attacker *Unit) (float64, float64, float64) {
	resistCoeff := unit.resistCoeff(spell, attacker, false)

	// Based on the piecewise linear regression estimates at https://royalgiraffe.github.io/partial-resist-table.
	if val := resistCoeff * 3; val <= 1 {
		return 0.76 * val, 0.21 * val, 0.03 * val
	} else if val <= 2 {
		val -= 1
		return 0.76 + 0.24*val, 0.21 + 0.57*val, 0.03 + 0.19*val
	} else {
		val -= 2
		return 1, 0.78 + 0.18*val, 0.22 + 0.58*val
	}
}

// https://web.archive.org/web/20130208043756/http://elitistjerks.com/f15/t29453-combat_ratings_level_85_cataclysm/
// https://web.archive.org/web/20110309163709/http://elitistjerks.com/f78/t105429-cataclysm_mechanics_testing/
func (at *AttackTable) GetArmorDamageModifier(spell *Spell) float64 {
	if at.IgnoreArmor {
		return 1.0
	}

	ignoreArmorFactor := Clamp(at.ArmorIgnoreFactor, 0.0, 1.0)
	defenderArmor := at.Defender.Armor() - (at.Defender.Armor() * ignoreArmorFactor)

	switch at.resolvedMitigationRules().armorModel {
	case armorMitigationClassicReference60:
		if math.IsNaN(defenderArmor) || math.IsInf(defenderArmor, 0) {
			panic("unsupported Classic-60 armor inputs")
		}
		// The pinned Classic Unit.Armor floors negative resolved armor;
		// modern Unit.Armor does not. Keep that source step local to Classic.
		defenderArmor = max(defenderArmor, 0)
		modifier, ok := classicReferenceArmorDamageModifier(classicReferenceMitigationScope(), classicReferenceArmorInput{
			attackerLevel:        at.Attacker.Level,
			defenderArmor:        defenderArmor,
			flatArmorPenetration: at.Attacker.stats[stats.ArmorPenetration],
		})
		if !ok {
			panic("unsupported Classic-60 armor inputs")
		}
		return modifier
	case armorMitigationInheritedTBC:
		// Preserve the imported arithmetic and its 75% mitigation cap.
	default:
		panic("unsupported armor mitigation model")
	}

	// Assume target > 80
	armorConstant := float64(at.Attacker.Level)*467.5 - 22167.5
	// TBC ANNI: Apply flat ArP
	defenderArmor = max(defenderArmor-at.Attacker.stats[stats.ArmorPenetration], 0)
	// Damage reduction from armor is capped at 75%.
	return max(1-defenderArmor/(defenderArmor+armorConstant), 0.25)
}
