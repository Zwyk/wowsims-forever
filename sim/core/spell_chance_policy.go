package core

import (
	"math"

	"github.com/wowsims/tbc/sim/core/stats"
)

type spellChanceModel uint8

const (
	spellChanceModelInheritedTBC spellChanceModel = iota
	spellChanceModelClassicReference60
)

type classicSpellChanceView struct {
	baseMissChance float64
	bonusHitChance float64
	critChance     float64
}

// classicSpellChanceAttackTable selects from the cached offensive table. A
// profile on the unit cannot override an already constructed table's model.
// Legacy TBC callers without attack tables retain their existing behavior.
func (spell *Spell) classicSpellChanceAttackTable(target *Unit) *AttackTable {
	if target != nil && target.UnitIndex >= 0 && int(target.UnitIndex) < len(spell.Unit.AttackTables) {
		if table := spell.Unit.AttackTables[target.UnitIndex]; table != nil {
			switch table.resolvedOutcomeRules().spellChanceModel {
			case spellChanceModelInheritedTBC:
				return nil
			case spellChanceModelClassicReference60:
				if table.Defender != target {
					panic("Classic spell reference requires the attack table's actual defender")
				}
				return table
			default:
				panic("unsupported spell chance model")
			}
		}
	}
	if spell.Unit.resolvedOutcomeRules().spellChanceModel != spellChanceModelInheritedTBC {
		panic("Classic spell reference requires an owned attack table")
	}
	return nil
}

// liveClassicSpellChanceView is a bounded executable reference to:
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/target.go#L324-L348
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_result.go#L195-L233
//
// The source populates SpellCritSuppression but comments out its consumption.
// Preserve executable behavior here, not the unused seed or a claim about the
// game's true suppression. School hit applies without a class-mask condition.
// School/target crit modifiers have no proven one-to-one modern mapping and
// remain unsupported. Restrict crit to [0,1], where the source's unclamped
// getter and its expected-outcome helpers describe the same probabilities.
func liveClassicSpellChanceView(spell *Spell, table *AttackTable) classicSpellChanceView {
	if table != nil && table.Attacker != nil && table.Attacker.Type == EnemyUnit && table.resolvedResourceRules().manaModel == manaModelClassic60PaladinReference {
		return liveClassic60PaladinIncomingSpellView(spell, table)
	}
	if spell == nil || spell.Unit == nil || table == nil || !table.rulesInitialized ||
		table.Attacker != spell.Unit || table.Defender == nil ||
		table.Attacker.Type != PlayerUnit || table.Attacker.Level != 60 ||
		table.Defender.Type != EnemyUnit || table.Defender.Level < 60 || table.Defender.Level > 63 {
		panic("Classic spell reference requires a level-60 player attacking a level 60–63 enemy")
	}
	index := table.Defender.UnitIndex
	if index < 0 || int(index) >= len(spell.Unit.AttackTables) || spell.Unit.AttackTables[index] != table {
		panic("Classic spell reference requires the attacker's registered table")
	}
	if table.resolvedOutcomeRules().spellChanceModel != spellChanceModelClassicReference60 ||
		table.resolvedMitigationRules().resistanceModel != resistanceMitigationClassicReference60 {
		panic("Classic spell reference requires consistent cached chance and resistance rules")
	}
	sealProc := spell.Unit.resolvedRuleset().id == rulesetClassic60PaladinReference &&
		spell.classic60PaladinAttack == classic60PaladinRighteousnessProc && spell.SpellID == 25713 &&
		spell.weaponAttackSource == WeaponAttackSourceMainHand && spell.ProcMask == ProcMaskMeleeMHSpecial &&
		spell.SpellSchool == SpellSchoolHoly && spell.Flags.Matches(SpellFlagIgnoreResists)
	if spell.DefenseType != DefenseTypeMagic || (!sealProc && (spell.weaponAttackSource != WeaponAttackSourceNone ||
		(spell.ProcMask != ProcMaskSpellDamage && spell.ProcMask != ProcMaskEmpty))) {
		panic("Classic spell reference requires an explicitly non-weapon magical damage spell")
	}
	switch spell.SpellSchool {
	case SpellSchoolArcane, SpellSchoolFire, SpellSchoolFrost, SpellSchoolHoly, SpellSchoolNature, SpellSchoolShadow:
	default:
		panic("Classic spell reference requires one magical school")
	}
	if spell.SchoolIndex != spell.SpellSchool.SchoolIndex() {
		panic("Classic spell reference requires consistent school classification")
	}
	unit := spell.Unit
	validateClassicSpellResources(spell, table)
	if unit.stats[stats.SpellHasteRating] != 0 || unit.stats[stats.MeleeHasteRating] != 0 ||
		unit.PseudoStats.CastSpeedMultiplier != 1 || !spell.IgnoreHaste {
		panic("Classic spell reference does not support haste")
	}
	if table.BonusSpellCritPercent != 0 || table.Defender.PseudoStats.BonusSpellCritPercentTaken != 0 ||
		table.Defender.PseudoStats.ReducedCritTakenPercent != 0 || table.Defender.stats[stats.ResilienceRating] != 0 ||
		table.CritMultiplier != 1 || unit.PseudoStats.CritDamageMultiplier != 1 ||
		spell.CritMultiplierPct != 1 || spell.CritMultiplierAdditive != 0 {
		panic("unsupported crit modifier in Classic spell reference")
	}
	hitPercent := unit.stats[stats.SpellHitPercent] + spell.BonusHitPercent + unit.PseudoStats.SchoolBonusHitChance[spell.SchoolIndex]
	critPercent := unit.stats[stats.SpellCritPercent] + spell.BonusCritPercent
	if math.IsNaN(hitPercent) || math.IsInf(hitPercent, 0) || hitPercent < 0 ||
		math.IsNaN(critPercent) || math.IsInf(critPercent, 0) || critPercent < 0 || critPercent > 100 {
		panic("Classic spell reference requires finite nonnegative hit and zero-to-100 percent crit")
	}
	return classicSpellChanceView{
		baseMissChance: [...]float64{0.04, 0.05, 0.06, 0.17}[table.Defender.Level-60],
		bonusHitChance: hitPercent / 100,
		critChance:     critPercent / 100,
	}
}

func (spell *Spell) classicSpellChanceToMiss(table *AttackTable) float64 {
	view := liveClassicSpellChanceView(spell, table)
	missChance := view.baseMissChance - view.bonusHitChance
	if spell.Flags.Matches(SpellFlagBinary) {
		// Resistance reduces base level-hit first; bonus hit is added afterwards.
		missChance = 1 - (1-view.baseMissChance)*table.GetBinaryHitChance(spell) - view.bonusHitChance
	}
	return max(0.01, missChance)
}
