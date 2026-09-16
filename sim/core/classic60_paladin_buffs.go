package core

import (
	"fmt"
	"math"
	"time"

	"github.com/wowsims/tbc/sim/core/stats"
)

// registerSelfBuffs provides the level-60 trainer ranks, not the later
// AQ-book upgrades: Might 6 (155 AP) and Wisdom 5 (30 MP5). Values, costs and
// durations are pinned in assets/db_inputs/wowhead_spell_tooltips.csv at
// wowsims/classic 7779ebbf79dc7f1341e6ab939b28a3402c9a730a (19838, 19854,
// 20217, 20218, 10293). Stat effects follow that source's sim/core/buffs.go;
// talent ranks use their actual rank values rather than its raid tristates.
// These are self casts only; other players, aura range and raid buffs are
// deliberately outside this private class bundle.
func (spells *classic60PaladinSpells) registerSelfBuffs(character *Character, talents classic60PaladinTalents) {
	selfOnly := func(_ *Simulation, target *Unit) bool { return target == &character.Unit }
	activateBlessing := func(sim *Simulation, selected *Aura) {
		for _, aura := range []*Aura{spells.MightAura, spells.WisdomAura, spells.KingsAura} {
			if aura != nil && aura != selected {
				aura.Deactivate(sim)
			}
		}
		selected.Activate(sim)
	}
	registerBlessing := func(aura *Aura, rank int32, cost ManaCostOptions) *Spell {
		return character.RegisterSpell(SpellConfig{
			ActionID: aura.ActionID, Rank: rank,
			SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
			ProcMask: ProcMaskEmpty, Flags: SpellFlagAPL | SpellFlagHelpful,
			ManaCost:           cost,
			Cast:               CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault}},
			ExtraCastCondition: selfOnly,
			ApplyEffects:       func(sim *Simulation, _ *Unit, _ *Spell) { activateBlessing(sim, aura) },
		})
	}

	spells.MightAura = character.RegisterAura(Aura{
		Label: "Classic Blessing of Might (Rank 6)", ActionID: ActionID{SpellID: 19838}, Duration: 5 * time.Minute,
	})
	makeFlatStatBuff(spells.MightAura, stats.AttackPower, math.Floor(155*(1+0.04*float64(talents[classic60PaladinImprovedBlessingOfMight]))))
	spells.BlessingOfMight = registerBlessing(spells.MightAura, 6, ManaCostOptions{FlatCost: 110})

	spells.WisdomAura = character.RegisterAura(Aura{
		Label: "Classic Blessing of Wisdom (Rank 5)", ActionID: ActionID{SpellID: 19854}, Duration: 5 * time.Minute,
	})
	makeFlatStatBuff(spells.WisdomAura, stats.MP5, 30*(1+0.1*float64(talents[classic60PaladinImprovedBlessingOfWisdom])))
	spells.BlessingOfWisdom = registerBlessing(spells.WisdomAura, 5, ManaCostOptions{FlatCost: 115})

	if talents[classic60PaladinBlessingOfKings] != 0 {
		spells.KingsAura = character.RegisterAura(Aura{
			Label: "Classic Blessing of Kings", ActionID: ActionID{SpellID: 20217}, Duration: 5 * time.Minute,
		})
		for _, stat := range []stats.Stat{stats.Strength, stats.Agility, stats.Stamina, stats.Intellect, stats.Spirit} {
			// The character owns the Classic dependency rounding policy.
			makeMultiplierBuff(spells.KingsAura, stat, 1.1)
		}
		// Classic's tooltip specifies 8%, i.e. 120.96 of 1512 base mana.
		spells.BlessingOfKings = registerBlessing(spells.KingsAura, 0, ManaCostOptions{BaseCostPercent: 8})
	}

	activateSelectedAura := func(aura *Aura, sim *Simulation) {
		if aura == spells.selectedAura {
			aura.Activate(sim)
		}
	}
	spells.DevotionEffect = character.RegisterAura(Aura{
		Label: "Classic Devotion Aura (Rank 7)", ActionID: ActionID{SpellID: 10293}, Duration: NeverExpires,
		OnReset: activateSelectedAura,
	})
	// Modern Unit.Armor reads Armor directly; its unused BonusArmor slot
	// does not carry the pinned source's dependency. A dynamic Armor bonus
	// also remains outside Toughness's equipment-only armor scaling.
	makeFlatStatBuff(spells.DevotionEffect, stats.Armor, 735*(1+0.05*float64(talents[classic60PaladinImprovedDevotionAura])))

	if talents[classic60PaladinSanctityAura] != 0 {
		spells.SanctityEffect = character.RegisterAura(Aura{
			Label: "Classic Sanctity Aura", ActionID: ActionID{SpellID: 20218}, Duration: NeverExpires,
			OnReset: activateSelectedAura,
			OnGain: func(_ *Aura, _ *Simulation) {
				character.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly] *= 1.1
			},
			OnExpire: func(_ *Aura, _ *Simulation) {
				character.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly] /= 1.1
			},
		})
	}
}

// selectReferenceAura configures one persistent aura before finalization,
// following the pinned source's selected-aura model. No castable aura spell
// is registered: in-combat switching and its GCD are not yet audited.
func (spells *classic60PaladinSpells) selectReferenceAura(aura *Aura) error {
	if spells.DevotionEffect == nil {
		return fmt.Errorf("Classic Paladin self buffs must be registered before selecting an aura")
	}
	if aura != nil && aura != spells.DevotionEffect && aura != spells.SanctityEffect {
		return fmt.Errorf("Classic Paladin aura selection requires an owned, learned reference aura")
	}
	if aura == spells.selectedAura {
		return nil
	}
	if env := spells.DevotionEffect.Unit.Env; env != nil && env.IsFinalized() {
		return fmt.Errorf("Classic Paladin aura selection must be configured before finalization")
	}
	spells.selectedAura = aura
	return nil
}
