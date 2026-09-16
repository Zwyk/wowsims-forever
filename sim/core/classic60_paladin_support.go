package core

import (
	"time"

	"github.com/wowsims/tbc/sim/core/stats"
)

type classic60PaladinSupport struct {
	BlessingOfSalvation, BlessingOfSanctuary, BlessingOfLight           *Spell
	SalvationAura, SanctuaryAura, LightAura                             *Aura
	RetributionProc, SanctuaryProc                                      *Spell
	RetributionEffect, ConcentrationEffect                              *Aura
	FireResistanceEffect, FrostResistanceEffect, ShadowResistanceEffect *Aura
}

// Pinned Classic source: wowsims/classic at
// 7779ebbf79dc7f1341e6ab939b28a3402c9a730a, sim/core/buffs.go,
// sim/paladin/blessing_of_sanctuary.go and the cached tooltips for
// 1038, 19979, 20914, 10301, 19746, 19900, 19898 and 19896.
// As with the initial blessings, these target only the caster. Auras are
// selected before the encounter; no live aura-switch timing is inferred.
func (spells *classic60PaladinSpells) registerSupport(character *Character, talents classic60PaladinTalents) {
	registerBlessing := func(aura *Aura, rank int32, cost ManaCostOptions) *Spell {
		return character.RegisterSpell(SpellConfig{
			ActionID: aura.ActionID, Rank: rank,
			SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
			ProcMask: ProcMaskEmpty, Flags: SpellFlagAPL | SpellFlagHelpful,
			ManaCost:           cost,
			Cast:               CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault}},
			ExtraCastCondition: func(_ *Simulation, target *Unit) bool { return target == &character.Unit },
			ApplyEffects:       func(sim *Simulation, _ *Unit, _ *Spell) { spells.activateBlessing(sim, aura) },
		})
	}
	spells.SalvationAura = character.RegisterAura(Aura{
		Label: "Classic Blessing of Salvation", ActionID: ActionID{SpellID: 1038}, Duration: 5 * time.Minute,
		OnGain:   func(_ *Aura, _ *Simulation) { character.PseudoStats.ThreatMultiplier *= 0.7 },
		OnExpire: func(_ *Aura, _ *Simulation) { character.PseudoStats.ThreatMultiplier /= 0.7 },
	})
	spells.BlessingOfSalvation = registerBlessing(spells.SalvationAura, 0, ManaCostOptions{BaseCostPercent: 8})
	spells.LightAura = character.RegisterAura(Aura{
		Label: "Classic Blessing of Light (Rank 3)", ActionID: ActionID{SpellID: 19979}, Duration: 5 * time.Minute,
	})
	// The healing bundle owns the Holy Light / Flash of Light bonus formula;
	// this aura must not grant a generic bonus to other healing spells.
	spells.BlessingOfLight = registerBlessing(spells.LightAura, 3, ManaCostOptions{FlatCost: 135})

	if talents[classic60PaladinBlessingOfSanctuary] != 0 {
		spells.SanctuaryProc = character.RegisterSpell(SpellConfig{
			ActionID: ActionID{SpellID: 20914, Tag: 1}, Rank: 4,
			SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic, ProcMask: ProcMaskSpellDamage,
			WeaponAttackSource: WeaponAttackSourceNone,
			Flags:              SpellFlagIgnoreResists | SpellFlagPassiveSpell | SpellFlagNoOnCastComplete,
			Cast:               CastConfig{IgnoreHaste: true}, DamageMultiplier: 1, ThreatMultiplier: 1,
			ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
				spell.CalcAndDealDamage(sim, target, 35, spell.OutcomeMagicHit)
			},
		})
		spells.SanctuaryAura = character.RegisterAura(Aura{
			Label: "Classic Blessing of Sanctuary (Rank 4)", ActionID: ActionID{SpellID: 20914}, Duration: 5 * time.Minute,
			OnGain:   func(_ *Aura, _ *Simulation) { character.PseudoStats.BonusDamageTakenBeforeModifiers -= 24 },
			OnExpire: func(_ *Aura, _ *Simulation) { character.PseudoStats.BonusDamageTakenBeforeModifiers += 24 },
			OnSpellHitTaken: func(_ *Aura, sim *Simulation, incoming *Spell, result *SpellResult) {
				if result.DidBlock() && incoming.ProcMask.Matches(ProcMaskMelee) {
					spells.SanctuaryProc.Cast(sim, incoming.Unit)
				}
			},
		})
		spells.BlessingOfSanctuary = registerBlessing(spells.SanctuaryAura, 4, ManaCostOptions{FlatCost: 135})
	}

	activateSelected := func(aura *Aura, sim *Simulation) {
		if aura == spells.selectedAura {
			aura.Activate(sim)
		}
	}
	spells.RetributionProc = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 10301}, Rank: 5,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic, ProcMask: ProcMaskEmpty,
		WeaponAttackSource: WeaponAttackSourceNone,
		Flags:              SpellFlagBinary | SpellFlagPassiveSpell | SpellFlagNoOnCastComplete,
		Cast:               CastConfig{IgnoreHaste: true}, DamageMultiplier: 1, ThreatMultiplier: 1,
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			spell.CalcAndDealDamage(sim, target, 20*(1+.25*float64(talents[classic60PaladinImprovedRetributionAura])), spell.OutcomeMagicHit)
		},
	})
	spells.RetributionEffect = character.RegisterAura(Aura{
		Label: "Classic Retribution Aura (Rank 5)", ActionID: ActionID{SpellID: 10301}, Duration: NeverExpires,
		OnReset: activateSelected,
		OnSpellHitTaken: func(_ *Aura, sim *Simulation, incoming *Spell, result *SpellResult) {
			if result.Landed() && incoming.ProcMask.Matches(ProcMaskMelee) {
				spells.RetributionProc.Cast(sim, incoming.Unit)
			}
		},
	})
	// The additional silence/interrupt resistance is evaluated by the owned
	// Classic hostile-control effect boundary in classic60_paladin_misc_talents.go.
	pushbackReduction := .35 + .05*float64(talents[classic60PaladinImprovedConcentrationAura])
	spells.ConcentrationEffect = character.RegisterAura(Aura{
		Label: "Classic Concentration Aura", ActionID: ActionID{SpellID: 19746}, Duration: NeverExpires,
		OnReset:  activateSelected,
		OnGain:   func(_ *Aura, _ *Simulation) { character.PseudoStats.PushbackChance -= pushbackReduction },
		OnExpire: func(_ *Aura, _ *Simulation) { character.PseudoStats.PushbackChance += pushbackReduction },
	})
	registerResistance := func(label string, spellID int32, stat stats.Stat) *Aura {
		aura := character.RegisterAura(Aura{
			Label: label, ActionID: ActionID{SpellID: spellID}, Duration: NeverExpires, OnReset: activateSelected,
		})
		makeFlatStatBuff(aura, stat, 60)
		return aura
	}
	spells.FireResistanceEffect = registerResistance("Classic Fire Resistance Aura (Rank 3)", 19900, stats.FireResistance)
	spells.FrostResistanceEffect = registerResistance("Classic Frost Resistance Aura (Rank 3)", 19898, stats.FrostResistance)
	spells.ShadowResistanceEffect = registerResistance("Classic Shadow Resistance Aura (Rank 3)", 19896, stats.ShadowResistance)
}
