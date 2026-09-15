package core

import (
	"math"

	"github.com/wowsims/tbc/sim/core/proto"
)

// weaponSkillModel selects how a short-lived physical attack-table view is
// derived. The active inherited TBC profile keeps this disabled. The Classic
// reference is compiled for characterization only and is not consumed by any
// production outcome path.
type weaponSkillModel uint8

const (
	weaponSkillModelDisabled weaponSkillModel = iota
	weaponSkillModelClassicReference
)

const classicReferenceCharacterLevel int32 = 60

// physicalAttackTableView keeps weapon-specific values out of the shared,
// mutable AttackTable for an attacker/defender pair. A future activation must
// route every relevant physical outcome through one such view atomically.
type physicalAttackTableView struct {
	baseMissChance       float64
	hitSuppression       float64
	baseBlockChance      float64
	baseDodgeChance      float64
	baseParryChance      float64
	baseGlanceChance     float64
	glanceMultiplierMin  float64
	glanceMultiplierMax  float64
	meleeCritSuppression float64
}

func (view physicalAttackTableView) glanceMultiplierMean() float64 {
	return (view.glanceMultiplierMin + view.glanceMultiplierMax) / 2
}

func inheritedPhysicalAttackTableView(table *AttackTable) physicalAttackTableView {
	return physicalAttackTableView{
		baseMissChance:       table.BaseMissChance,
		hitSuppression:       table.HitSuppression,
		baseBlockChance:      table.BaseBlockChance,
		baseDodgeChance:      table.BaseDodgeChance,
		baseParryChance:      table.BaseParryChance,
		baseGlanceChance:     table.BaseGlanceChance,
		glanceMultiplierMin:  table.GlanceMultiplier,
		glanceMultiplierMax:  table.GlanceMultiplier,
		meleeCritSuppression: table.MeleeCritSuppression,
	}
}

// resolvePhysicalAttackTableView is deliberately unused by live outcomes.
// It proves that a profile can derive per-weapon values without mutating the
// pair-wide AttackTable. Unsupported Classic contexts fail closed.
func resolvePhysicalAttackTableView(spell *Spell, table *AttackTable) (physicalAttackTableView, bool) {
	if table == nil {
		return physicalAttackTableView{}, false
	}

	switch table.resolvedOutcomeRules().weaponSkillModel {
	case weaponSkillModelDisabled:
		return inheritedPhysicalAttackTableView(table), true
	case weaponSkillModelClassicReference:
		if spell == nil || spell.Unit == nil || table.Attacker == nil || table.Defender == nil ||
			spell.Unit != table.Attacker || table.Attacker.Type != PlayerUnit || table.Defender.Type != EnemyUnit {
			return physicalAttackTableView{}, false
		}
		character := spell.weaponAttackCharacter()
		if character == nil || &character.Unit != spell.Unit {
			return physicalAttackTableView{}, false
		}
		context := spell.weaponAttackContext()
		if !classicReferenceSupportsWeaponContext(spell.DefenseType, context) {
			return physicalAttackTableView{}, false
		}
		weaponSkillBonus := context.weaponSkillBonus
		if context.classification.kind == weaponClassificationUnarmed {
			// The pinned Classic implementation returns no skill bonus when
			// its weapon item is nil. Equipped fist weapons still use the
			// Unarmed category through the equipped-item branch above.
			weaponSkillBonus = 0
		}
		return classicReferencePhysicalAttackTable(
			table.Attacker.Level,
			table.Defender.Level,
			weaponSkillBonus,
		)
	default:
		return physicalAttackTableView{}, false
	}
}

func classicReferenceSupportsWeaponContext(defenseType DefenseType, context weaponAttackContext) bool {
	category := context.classification.skillCategory
	switch context.classification.kind {
	case weaponClassificationEquippedItem:
		switch context.source {
		case WeaponAttackSourceMainHand:
			return defenseType == DefenseTypeMelee && classicReferenceSupportsMainHandCategory(category)
		case WeaponAttackSourceOffHand:
			return defenseType == DefenseTypeMelee && classicReferenceSupportsOffHandCategory(category)
		case WeaponAttackSourceRanged:
			return defenseType == DefenseTypeRanged && classicReferenceSupportsRangedCategory(category)
		default:
			return false
		}
	case weaponClassificationUnarmed:
		return defenseType == DefenseTypeMelee &&
			context.source == WeaponAttackSourceMainHand &&
			category == proto.WeaponSkillCategory_WeaponSkillCategoryUnarmed
	case weaponClassificationSynthetic:
		// FeralCombatWeapon is the only currently audited synthetic player
		// weapon constructor. Generic synthetic weapons stay fail-closed.
		return defenseType == DefenseTypeMelee &&
			context.source == WeaponAttackSourceMainHand &&
			category == proto.WeaponSkillCategory_WeaponSkillCategoryFeralCombat
	default:
		return false
	}
}

func classicReferenceSupportsMainHandCategory(category proto.WeaponSkillCategory) bool {
	switch category {
	case proto.WeaponSkillCategory_WeaponSkillCategoryAxes,
		proto.WeaponSkillCategory_WeaponSkillCategorySwords,
		proto.WeaponSkillCategory_WeaponSkillCategoryMaces,
		proto.WeaponSkillCategory_WeaponSkillCategoryDaggers,
		proto.WeaponSkillCategory_WeaponSkillCategoryUnarmed,
		proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedAxes,
		proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedSwords,
		proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedMaces,
		proto.WeaponSkillCategory_WeaponSkillCategoryPolearms,
		proto.WeaponSkillCategory_WeaponSkillCategoryStaves:
		return true
	default:
		return false
	}
}

func classicReferenceSupportsOffHandCategory(category proto.WeaponSkillCategory) bool {
	switch category {
	case proto.WeaponSkillCategory_WeaponSkillCategoryAxes,
		proto.WeaponSkillCategory_WeaponSkillCategorySwords,
		proto.WeaponSkillCategory_WeaponSkillCategoryMaces,
		proto.WeaponSkillCategory_WeaponSkillCategoryDaggers,
		proto.WeaponSkillCategory_WeaponSkillCategoryUnarmed:
		return true
	default:
		return false
	}
}

func classicReferenceSupportsRangedCategory(category proto.WeaponSkillCategory) bool {
	switch category {
	case proto.WeaponSkillCategory_WeaponSkillCategoryThrown,
		proto.WeaponSkillCategory_WeaponSkillCategoryBows,
		proto.WeaponSkillCategory_WeaponSkillCategoryCrossbows,
		proto.WeaponSkillCategory_WeaponSkillCategoryGuns:
		return true
	default:
		// Wand is a Forever-fork extension at category 16. The pinned Classic
		// implementation has no corresponding weapon-skill behavior.
		return false
	}
}

// classicReferencePhysicalAttackTable reproduces the player-versus-enemy
// weapon-skill formulas in wowsims/classic commit
// 7779ebbf79dc7f1341e6ab939b28a3402c9a730a. It is a comparison fallback,
// not evidence that Forever uses these values.
//
// The characterized scope is a capped level-60 attacker against an equal-level
// through +3 target with a finite skill bonus. Lower targets are rejected
// because the upstream formula can produce a negative glance chance. A
// negative value remains an additive modifier; it does not model training a
// weapon from below the assumed capped base skill.
func classicReferencePhysicalAttackTable(attackerLevel int32, defenderLevel int32, weaponSkillBonus float64) (physicalAttackTableView, bool) {
	if attackerLevel != classicReferenceCharacterLevel ||
		defenderLevel < attackerLevel || defenderLevel > attackerLevel+3 ||
		math.IsNaN(weaponSkillBonus) || math.IsInf(weaponSkillBonus, 0) {
		return physicalAttackTableView{}, false
	}

	baseWeaponSkill := float64(attackerLevel * 5)
	weaponSkill := baseWeaponSkill + weaponSkillBonus
	targetDefense := float64(defenderLevel * 5)
	weaponSkillDeficit := targetDefense - weaponSkill
	baseSkillDeficit := targetDefense - baseWeaponSkill

	view := physicalAttackTableView{
		baseBlockChance:  0.05,
		baseDodgeChance:  0.05 + weaponSkillDeficit*0.001,
		baseGlanceChance: 0.10 + baseSkillDeficit*0.02,
		glanceMultiplierMin: Clamp(
			1.3-0.05*weaponSkillDeficit,
			0.01,
			0.91,
		),
		glanceMultiplierMax: Clamp(
			1.2-0.03*weaponSkillDeficit,
			0.20,
			0.99,
		),
	}

	if weaponSkillDeficit > 10 {
		view.hitSuppression = (weaponSkillDeficit - 10) * 0.002
		view.baseMissChance = 0.05 + weaponSkillDeficit*0.002
	} else {
		view.baseMissChance = 0.05 + weaponSkillDeficit*0.001
	}

	if baseSkillDeficit > 10 {
		view.baseParryChance = 0.05 + baseSkillDeficit*0.006
	} else {
		view.baseParryChance = 0.05 + baseSkillDeficit*0.001
	}

	if baseSkillDeficit > 0 {
		view.meleeCritSuppression = baseSkillDeficit * 0.002
	} else {
		view.meleeCritSuppression = baseSkillDeficit * 0.0004
	}
	if defenderLevel-attackerLevel >= 3 {
		view.meleeCritSuppression += 0.018
	}

	return view, true
}
