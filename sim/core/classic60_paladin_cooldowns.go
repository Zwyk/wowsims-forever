package core

import "time"

type classic60PaladinCooldowns struct {
	DivineShield, DivineProtection, BlessingOfProtection                    *Spell
	DivineShieldAura, DivineProtectionAura, ProtectionAura, ForbearanceAura *Aura
}

// Cached Classic tooltips at 7779ebbf79dc7f1341e6ab939b28a3402c9a730a:
// Divine Shield 1020, Divine Protection 5573, Protection 10278, Guardian's
// Favor 20174/20175. Pinned sim/paladin/forbearance.go supplies the shared
// 60-second exclusion. These self casts model damage immunity and cleansing
// of the owned fear/stun/Repentance/disorient/silence/root/snare types.
// Interrupt lockouts, arbitrary debuffs, and encounter target selection
// remain separate.
func (spells *classic60PaladinSpells) registerDefensiveCooldowns(character *Character, talents classic60PaladinTalents) {
	spells.ForbearanceAura = character.RegisterAura(Aura{
		Label: "Classic Forbearance", ActionID: ActionID{SpellID: 25771}, Duration: time.Minute,
	})
	allSchools := SpellSchoolPhysical | SpellSchoolChaos
	immunityAura := func(label string, id int32, duration time.Duration, schools SpellSchool, pacify, slow bool) *Aura {
		return character.RegisterAura(Aura{
			Label: label, ActionID: ActionID{SpellID: id}, Duration: duration,
			OnGain: func(_ *Aura, sim *Simulation) {
				character.classic60ImmuneSchools |= schools
				if pacify {
					character.classic60PaladinPacified = true
				}
				if slow {
					character.MultiplyMeleeSpeed(sim, .5)
				}
			},
			OnExpire: func(_ *Aura, sim *Simulation) {
				character.classic60ImmuneSchools &^= schools
				if pacify {
					character.classic60PaladinPacified = false
				}
				if slow {
					character.MultiplyMeleeSpeed(sim, 2)
				}
			},
		})
	}
	spells.DivineShieldAura = immunityAura("Classic Divine Shield (Rank 2)", 1020, 12*time.Second, allSchools, false, true)
	// The cached buff explicitly says all attacks and spells: Divine
	// Protection's immunity includes magic; only its pacification is physical.
	spells.DivineProtectionAura = immunityAura("Classic Divine Protection (Rank 2)", 5573, 8*time.Second, allSchools, true, false)
	for _, aura := range []*Aura{spells.DivineShieldAura, spells.DivineProtectionAura} {
		aura.AttachFearImmunity().AttachStunImmunity()
		classic60RepentanceKind.attachImmunity(aura)
		aura.ApplyOnGain(func(_ *Aura, sim *Simulation) {
			if state := character.classic60PaladinMobility; state != nil {
				state.clearImpairments(sim)
			}
			if state := character.classic60PaladinMisc; state != nil {
				state.breakControls(sim)
			}
		})
	}
	spells.ProtectionAura = immunityAura("Classic Blessing of Protection (Rank 3)", 10278, 10*time.Second, SpellSchoolPhysical, true, false)
	register := func(aura *Aura, rank int32, cost ManaCostOptions, cd Cooldown, blessing bool) *Spell {
		flags := SpellFlagHelpful | SpellFlagAPL
		if !blessing {
			flags |= SpellFlagCastWhileIncapacitated
		}
		return character.RegisterSpell(SpellConfig{
			ActionID: aura.ActionID, Rank: rank,
			SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
			WeaponAttackSource: WeaponAttackSourceNone, ProcMask: ProcMaskEmpty,
			Flags: flags, ManaCost: cost,
			Cast: CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault}, CD: cd},
			ExtraCastCondition: func(_ *Simulation, target *Unit) bool {
				return target == &character.Unit && !spells.ForbearanceAura.IsActive()
			},
			ApplyEffects: func(sim *Simulation, _ *Unit, _ *Spell) {
				if blessing {
					spells.activateBlessing(sim, aura)
				} else {
					aura.Activate(sim)
				}
				spells.ForbearanceAura.Activate(sim)
			},
		})
	}
	// Original client DBC, cmangos/mangos-classic at
	// 8ec338a1704e7dcb1c0213eb7ed58f9231ade40f, Spell.sql: both 1020 and
	// 5573 use Category 37 with CategoryRecoveryTime 300000 milliseconds.
	bubbleCD := Cooldown{Timer: character.NewTimer(), Duration: 5 * time.Minute}
	spells.DivineShield = register(spells.DivineShieldAura, 2, ManaCostOptions{FlatCost: 110}, bubbleCD, false)
	spells.DivineProtection = register(spells.DivineProtectionAura, 2, ManaCostOptions{FlatCost: 35}, bubbleCD, false)
	spells.BlessingOfProtection = register(spells.ProtectionAura, 3, ManaCostOptions{BaseCostPercent: 7},
		Cooldown{Timer: character.NewTimer(), Duration: 5*time.Minute - time.Duration(talents[classic60PaladinGuardiansFavor])*time.Minute}, true)
}
