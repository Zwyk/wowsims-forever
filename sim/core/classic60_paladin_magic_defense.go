package core

import (
	"math"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

type classic60PaladinMagicDefense struct {
	EyeForAnEye *Spell
}

func (spells *classic60PaladinSpells) registerMagicDefense(character *Character, talents classic60PaladinTalents) {
	if character == nil || character.classic60PaladinDefense == nil {
		panic("Classic incoming spells require registered Paladin defenses")
	}
	rank := talents[classic60PaladinEyeForAnEye]
	if rank > 2 {
		panic("Classic Eye for an Eye has only two ranks")
	}
	if rank == 0 {
		return
	}
	// Original Classic DBC (CMaNGOS@8ec338a) and cached Classic tooltips:
	// 9799/25988 return 15/30%.
	// Spell 25997 is Holy, magic, coefficient zero, with no cannot-crit,
	// always-hit or ignore-resistance attribute. Use ordinary magic outcomes.
	// VMaNGOS@8f4e6084 UnitAuraProcHandler.cpp takes the pre-mitigation
	// critical amount, caps the triggered base damage at half the Paladin's
	// maximum health, and then casts 25997 through normal spell processing.
	reflectedBaseDamage := 0.0
	spells.EyeForAnEye = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 25997}, SpellSchool: SpellSchoolHoly,
		DefenseType: DefenseTypeMagic, ProcMask: ProcMaskSpellDamage,
		WeaponAttackSource: WeaponAttackSourceNone,
		Flags:              SpellFlagPassiveSpell | SpellFlagNoOnCastComplete,
		Cast:               CastConfig{IgnoreHaste: true}, DamageMultiplier: 1, ThreatMultiplier: 1,
		BonusCritPercent: float64(talents[classic60PaladinHolyPower]),
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			spell.CalcAndDealDamage(sim, target, reflectedBaseDamage, spell.OutcomeMagicHitAndCrit)
		},
	})
	MakePermanent(character.RegisterAura(Aura{
		Label: "Classic Eye for an Eye", ActionID: ActionID{SpellID: [...]int32{0, 9799, 25988}[rank]},
		OnSpellHitTaken: func(_ *Aura, sim *Simulation, incoming *Spell, result *SpellResult) {
			if !result.DidCrit() || incoming.DefenseType != DefenseTypeMagic || incoming.Unit.Type != EnemyUnit ||
				result.Target != &character.Unit || result.classic60PreMitigationDamage <= 0 {
				return
			}
			reflectedBaseDamage = min(.15*float64(rank)*result.classic60PreMitigationDamage, character.MaxHealth()/2)
			spells.EyeForAnEye.Cast(sim, incoming.Unit)
		},
	}))
}

// The reverse table uses the executable Classic source at
// wowsims/classic@7779ebbf79dc7f1341e6ab939b28a3402c9a730a:
// sim/core/target.go (defender != EnemyUnit) and spell_result.go. Its base spell
// miss chance against a player is 5%, independently of the NPC's level. The
// modern table seeds, rating conversions and crit suppression are not used.
func liveClassic60PaladinIncomingSpellView(spell *Spell, table *AttackTable) classicSpellChanceView {
	if spell == nil || spell.Unit == nil || table == nil || !table.rulesInitialized ||
		table.Attacker != spell.Unit || table.Attacker.Type != EnemyUnit ||
		table.Attacker.Level < 60 || table.Attacker.Level > 63 || table.Defender == nil ||
		table.Defender.Type != PlayerUnit || table.Defender.Level != 60 ||
		table.resolvedResourceRules().manaModel != manaModelClassic60PaladinReference ||
		table.resolvedOutcomeRules().spellChanceModel != spellChanceModelClassicReference60 ||
		table.resolvedMitigationRules().resistanceModel != resistanceMitigationClassicReference60 {
		panic("Classic incoming spells require an owned level-60 Paladin and level 60–63 NPC table")
	}
	state, index := table.Defender.classic60PaladinDefense, table.Defender.UnitIndex
	if state == nil || state.character == nil || &state.character.Unit != table.Defender ||
		state.character.Class != proto.Class_ClassPaladin || state.character.Race != proto.Race_RaceHuman ||
		state.character.resolvedRuleset().id != rulesetClassic60PaladinReference ||
		index < 0 || int(index) >= len(spell.Unit.AttackTables) || spell.Unit.AttackTables[index] != table {
		panic("Classic incoming spells require registered Paladin defenses and the attacker's actual table")
	}
	if spell.DefenseType != DefenseTypeMagic || spell.weaponAttackSource != WeaponAttackSourceNone ||
		(spell.ProcMask != ProcMaskSpellDamage && spell.ProcMask != ProcMaskEmpty) {
		panic("Classic incoming spells require explicitly non-weapon magical damage")
	}
	switch spell.SpellSchool {
	case SpellSchoolArcane, SpellSchoolFire, SpellSchoolFrost, SpellSchoolHoly, SpellSchoolNature, SpellSchoolShadow:
	default:
		panic("Classic incoming spells require one magical school")
	}
	if spell.SchoolIndex != spell.SpellSchool.SchoolIndex() {
		panic("Classic incoming spells require consistent school classification")
	}
	unit := spell.Unit
	if unit.HasManaBar() || unit.HasRageBar() || unit.HasEnergyBar() || unit.HasFocusBar() || spell.Cost != nil {
		panic("Classic incoming NPC spells do not support resources")
	}
	if unit.stats[stats.SpellHasteRating] != 0 || unit.stats[stats.MeleeHasteRating] != 0 ||
		unit.PseudoStats.CastSpeedMultiplier != 1 || !spell.IgnoreHaste {
		panic("Classic incoming spells do not support haste")
	}
	if table.BonusSpellCritPercent != 0 || table.Defender.PseudoStats.BonusSpellCritPercentTaken != 0 ||
		table.Defender.PseudoStats.ReducedCritTakenPercent != 0 || table.Defender.stats[stats.ResilienceRating] != 0 ||
		table.CritMultiplier != 1 || unit.PseudoStats.CritDamageMultiplier != 1 ||
		spell.CritMultiplierPct != 1 || spell.CritMultiplierAdditive != 0 {
		panic("unsupported crit modifier in Classic incoming spells")
	}
	hitPercent := unit.stats[stats.SpellHitPercent] + spell.BonusHitPercent + unit.PseudoStats.SchoolBonusHitChance[spell.SchoolIndex]
	critPercent := unit.stats[stats.SpellCritPercent] + spell.BonusCritPercent
	if math.IsNaN(hitPercent) || math.IsInf(hitPercent, 0) || hitPercent < 0 ||
		math.IsNaN(critPercent) || math.IsInf(critPercent, 0) || critPercent < 0 || critPercent > 100 {
		panic("Classic incoming spells require finite nonnegative hit and zero-to-100 percent crit")
	}
	return classicSpellChanceView{baseMissChance: .05, bonusHitChance: hitPercent / 100, critChance: critPercent / 100}
}
