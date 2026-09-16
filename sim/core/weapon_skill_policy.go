package core

import (
	"math"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

// weaponSkillModel selects how a short-lived physical attack-table view is
// derived. The active inherited TBC profile keeps this disabled. The Classic
// reference is available to a bounded internal combat slice, while the normal
// application still selects inherited TBC rules.
type weaponSkillModel uint8

const (
	weaponSkillModelDisabled weaponSkillModel = iota
	weaponSkillModelClassicReference
)

// physicalAttackTableView keeps weapon-specific values out of the shared,
// mutable AttackTable for an attacker/defender pair. The bounded Classic live
// slice routes all of its physical outcomes through the same weapon view.
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

// livePhysicalAttackTableView is the deliberately narrow activation boundary
// for the first Classic combat slice. The broader pure reference policy below
// remains available for characterization; it is not a promise that every
// classified weapon, class or combat modifier has a complete runtime profile.
func livePhysicalAttackTableView(spell *Spell, table *AttackTable) physicalAttackTableView {
	if table.resolvedOutcomeRules().weaponSkillModel == weaponSkillModelDisabled {
		return inheritedPhysicalAttackTableView(table)
	}
	if spell == nil || spell.Unit == nil || table.Attacker != spell.Unit || table.Defender == nil ||
		table.Attacker.Type != PlayerUnit || table.Defender.Type != EnemyUnit {
		panic("Classic melee reference requires a player attacking an enemy")
	}
	character := spell.weaponAttackCharacter()
	if character == nil || &character.Unit != spell.Unit || character.Class != proto.Class_ClassWarrior || character.Race != proto.Race_RaceHuman ||
		spell.Unit.PseudoStats.InFrontOfTarget || spell.Unit.AutoAttacks.IsDualWielding ||
		spell.DefenseType != DefenseTypeMelee || spell.SpellSchool != SpellSchoolPhysical ||
		spell.ProcMask != ProcMaskMeleeMHAuto || spell.weaponAttackSource != WeaponAttackSourceMainHand ||
		spell.Flags.Matches(SpellFlagCannotBeDodged) {
		panic("Classic melee reference supports only Human Warrior rear main-hand physical auto-attacks")
	}
	context := spell.weaponAttackContext()
	if context.classification.kind != weaponClassificationEquippedItem ||
		context.weaponSkillBonus < 0 || context.weaponSkillBonus > 15 {
		panic("Classic melee reference requires an equipped weapon and a skill bonus between zero and fifteen")
	}
	if spell.Unit.HasManaBar() || spell.Unit.HasRageBar() || spell.Unit.HasEnergyBar() || spell.Unit.HasFocusBar() ||
		spell.Unit.stats[stats.MeleeHasteRating] != 0 || spell.Unit.stats[stats.SpellHasteRating] != 0 ||
		spell.Unit.PseudoStats.AttackSpeedMultiplier != 1 || spell.Unit.PseudoStats.MeleeSpeedMultiplier != 1 {
		panic("Classic melee reference does not support resource bars or haste")
	}
	for _, value := range []float64{spell.Unit.stats[stats.PhysicalHitPercent], spell.Unit.stats[stats.PhysicalCritPercent], spell.BonusHitPercent, spell.BonusCritPercent} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			panic("Classic melee reference requires finite hit and crit chances")
		}
	}
	if spell.Unit.stats[stats.ExpertiseRating] != 0 || spell.BonusExpertiseRating != 0 ||
		spell.Unit.PseudoStats.DodgeReduction != 0 || table.Defender.PseudoStats.DodgeReduction != 0 ||
		table.Defender.PseudoStats.BaseDodgeChance != 0 || table.Defender.PseudoStats.BaseParryChance != 0 ||
		table.Defender.PseudoStats.BaseBlockChance != 0 || table.Defender.PseudoStats.ReducedPhysicalHitTakenChance != 0 ||
		table.Defender.PseudoStats.ReducedCritTakenPercent != 0 {
		panic("unsupported avoidance modifier in Classic melee reference")
	}
	for _, stat := range []stats.Stat{stats.DefenseRating, stats.DodgeRating, stats.ParryRating, stats.BlockRating, stats.BlockPercent, stats.ResilienceRating} {
		if table.Defender.stats[stat] != 0 {
			panic("unsupported defensive stat in Classic melee reference")
		}
	}
	view, ok := resolvePhysicalAttackTableView(spell, table)
	if !ok {
		panic("unsupported weapon or level in Classic melee reference")
	}
	return view
}

// resolvePhysicalAttackTableView derives per-weapon values without mutating the
// pair-wide AttackTable. It supports the broader characterization scope;
// livePhysicalAttackTableView applies additional runtime capability checks.
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
