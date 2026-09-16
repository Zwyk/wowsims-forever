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
	// A spell-only reference must reject physical combat, not fall back to TBC.
	weaponSkillModelUnavailable
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
	if table.resolvedOutcomeRules().weaponSkillModel != weaponSkillModelClassicReference {
		panic("physical outcomes unavailable for selected rules")
	}
	if spell != nil && spell.classic60PaladinAttack == classic60PaladinHammerOfWrath {
		return liveClassic60PaladinRangedView(spell, table)
	}
	if spell == nil || spell.Unit == nil || table.Attacker != spell.Unit || table.Defender == nil ||
		table.Attacker.Type != PlayerUnit || table.Defender.Type != EnemyUnit {
		panic("Classic melee reference requires a player attacking an enemy")
	}
	character := spell.weaponAttackCharacter()
	paladin := character != nil && character.Class == proto.Class_ClassPaladin && character.resolvedRuleset().id == rulesetClassic60PaladinReference
	if character == nil || &character.Unit != spell.Unit || character.Race != proto.Race_RaceHuman ||
		(!paladin && spell.Unit.PseudoStats.InFrontOfTarget) || spell.DefenseType != DefenseTypeMelee ||
		spell.Flags.Matches(SpellFlagCannotBeDodged) {
		panic("Classic melee reference requires a supported Human rear melee attack")
	}
	if paladin {
		spell.validateClassic60PaladinMelee(table, character)
	} else if character.Class != proto.Class_ClassWarrior || spell.SpellSchool != SpellSchoolPhysical ||
		spell.classic60PaladinAttack != classic60PaladinAttackNone {
		panic("Classic melee reference requires a supported class and school")
	}
	// Hand selection is explicit and must agree with the proc category. Do not
	// infer skill from a broad/mixed mask or permit an unequipped off-hand.
	switch spell.weaponAttackSource {
	case WeaponAttackSourceMainHand:
		if spell.ProcMask != ProcMaskMeleeMHAuto && spell.ProcMask != ProcMaskMeleeMHSpecial {
			panic("Classic main-hand attack requires a matching auto or special proc mask")
		}
	case WeaponAttackSourceOffHand:
		if !spell.Unit.AutoAttacks.IsDualWielding || (spell.ProcMask != ProcMaskMeleeOHAuto && spell.ProcMask != ProcMaskMeleeOHSpecial) {
			panic("Classic off-hand attack requires dual wield and a matching proc mask")
		}
	default:
		panic("Classic melee reference requires an explicit melee weapon source")
	}
	if spell.Unit.AutoAttacks.IsDualWielding {
		mainHand := character.MainHand()
		if mainHand.HandType == proto.HandType_HandTypeTwoHand ||
			weaponSkillCategoryFromEquippedItem(mainHand, WeaponAttackSourceMainHand) == proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified ||
			weaponSkillCategoryFromEquippedItem(character.OffHand(), WeaponAttackSourceOffHand) == proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified {
			panic("Classic dual wield requires two compatible equipped melee weapons")
		}
	}
	if spell.Unit.PseudoStats.DisableDWMissPenalty {
		panic("Classic melee reference does not support queued-attack miss-penalty overrides")
	}
	context := spell.weaponAttackContext()
	if context.classification.kind != weaponClassificationEquippedItem ||
		context.weaponSkillBonus < 0 || context.weaponSkillBonus > 15 {
		panic("Classic melee reference requires an equipped weapon and a skill bonus between zero and fifteen")
	}
	if (!paladin && spell.Unit.HasManaBar()) || spell.Unit.HasRageBar() || spell.Unit.HasEnergyBar() || spell.Unit.HasFocusBar() ||
		spell.Unit.stats[stats.MeleeHasteRating] != 0 || spell.Unit.stats[stats.SpellHasteRating] != 0 ||
		spell.Unit.PseudoStats.AttackSpeedMultiplier != 1 || (!paladin && spell.Unit.PseudoStats.MeleeSpeedMultiplier != 1) || (paladin && !classic60PaladinMeleeSpeedSupported(spell.Unit)) {
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
	if paladin && table.Defender.PseudoStats.Stunned {
		view.baseDodgeChance, view.baseParryChance, view.baseBlockChance = 0, 0, 0
	}
	if spell.Unit.foreverRet != nil {
		reduction := spell.Unit.foreverRet.config.AvoidanceReduction / 100
		view.baseDodgeChance = max(0, view.baseDodgeChance-reduction)
		view.baseParryChance = max(0, view.baseParryChance-reduction)
	}
	return view
}

func classic60PaladinMeleeSpeedSupported(unit *Unit) bool {
	expected := 1.0
	if unit.HasActiveAura("Seal of the Crusader (Rank 6)") {
		expected *= 1.4
	}
	if unit.HasActiveAura("Classic Divine Shield (Rank 2)") {
		expected *= .5
	}
	return math.Abs(unit.PseudoStats.MeleeSpeedMultiplier-expected) < 1e-9
}

// The Command exception is private and deliberately restricted to its two
// registered Holy attacks. Generic physical specials and magical melee spells
// do not become supported merely by selecting the combined profile.
func (spell *Spell) validateClassic60PaladinMelee(table *AttackTable, character *Character) {
	index := table.Defender.UnitIndex
	if !table.rulesInitialized || index < 0 || int(index) >= len(spell.Unit.AttackTables) ||
		spell.Unit.AttackTables[index] != table || spell.Unit.Level != 60 ||
		table.resolvedResourceRules().manaModel != manaModelClassic60PaladinReference ||
		table.resolvedOutcomeRules().spellChanceModel != spellChanceModelClassicReference60 ||
		table.resolvedMitigationRules().resistanceModel != resistanceMitigationClassicReference60 {
		panic("Classic Paladin requires its owned combined attack table")
	}
	validateClassicSpellResources(spell, table)
	weapon := character.MainHand()
	if spell.weaponAttackSource != WeaponAttackSourceMainHand || spell.Unit.AutoAttacks.IsDualWielding ||
		(weapon.HandType != proto.HandType_HandTypeTwoHand && weapon.HandType != proto.HandType_HandTypeOneHand && weapon.HandType != proto.HandType_HandTypeMainHand) ||
		(weapon.WeaponType != proto.WeaponType_WeaponTypeSword && weapon.WeaponType != proto.WeaponType_WeaponTypeMace && weapon.WeaponType != proto.WeaponType_WeaponTypeAxe) {
		panic("Classic Paladin reference requires an equipped two-handed sword, mace or axe")
	}
	if spell.Flags.Matches(SpellFlagBinary | SpellFlagIgnoreResists | SpellFlagPureDot) {
		panic("Classic Paladin attacks require ordinary armor or nonbinary resistance")
	}
	switch spell.classic60PaladinAttack {
	case classic60PaladinAttackNone:
		if spell.SpellSchool != SpellSchoolPhysical || spell.ProcMask != ProcMaskMeleeMHAuto {
			panic("Classic Paladin physical reference supports white main-hand attacks")
		}
	case foreverPaladinHolyStrike:
		if spell.Unit.foreverRet == nil || spell.SpellID != 17143 || spell.SpellSchool != SpellSchoolHoly || spell.ProcMask != ProcMaskMeleeMHSpecial || !spell.IgnoreHaste {
			panic("Forever Holy Strike requires its owned Ret simulation")
		}
	case classic60PaladinCommandProc, classic60PaladinCommandJudgement:
		expectedID := int32(20947)
		if spell.classic60PaladinAttack == classic60PaladinCommandJudgement {
			expectedID = 20966
		}
		if spell.SpellID != expectedID || spell.SpellSchool != SpellSchoolHoly || spell.SchoolIndex != stats.SchoolIndexHoly ||
			spell.ProcMask != ProcMaskMeleeMHSpecial || !spell.IgnoreHaste {
			panic("Classic Paladin Holy melee requires a registered Command attack")
		}
	default:
		panic("unsupported Classic Paladin attack")
	}
	if table.CritMultiplier != 1 || spell.Unit.PseudoStats.CritDamageMultiplier != 1 ||
		spell.CritMultiplierPct != 1 || spell.CritMultiplierAdditive != 0 || spell.Unit.PseudoStats.CastSpeedMultiplier != 1 {
		panic("Classic Paladin reference does not support crit or cast-speed modifiers")
	}
}

// The same weapon view supplies white and special chances, but their outcome
// tables differ. Reject a caller that sends a special through the glancing
// white table or a white swing through the no-dual-wield-penalty special table.
func (spell *Spell) requireClassicMeleeOutcome(table *AttackTable, white bool) {
	livePhysicalAttackTableView(spell, table)
	if spell.ProcMask.Matches(ProcMaskMeleeWhiteHit) != white {
		panic("Classic melee outcome does not match the attack's auto/special category")
	}
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
