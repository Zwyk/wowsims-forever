package core

import (
	"fmt"
	"time"

	"github.com/wowsims/tbc/sim/core/stats"
)

type classic60PaladinMiscTalents struct {
	PursuitOfJusticeAura  *Aura
	VindicationProc       *Spell
	VindicationAuras      AuraArray
	unit                  *Unit
	unyieldingFaith       float64
	improvedConcentration float64
	concentrationAura     func() *Aura
	controlEffects        []*classic60PaladinControlEffect
}

// Original Classic Spell.dbc, preserved by cmangos/mangos-classic at
// 8ec338a1704e7dcb1c0213eb7ed58f9231ade40f, gives Pursuit 4/8 percent
// running and mounted speed. The encounter engine models running only;
// mounted travel is outside its state model. Use its existing strongest-
// passive movement category rather than stacking this with other passives.
func (spells *classic60PaladinSpells) registerMiscTalents(character *Character, talents classic60PaladinTalents) {
	state := &spells.classic60PaladinMiscTalents
	state.unit = &character.Unit
	state.unyieldingFaith = .05 * float64(talents[classic60PaladinUnyieldingFaith])
	state.improvedConcentration = .05 * float64(talents[classic60PaladinImprovedConcentrationAura])
	state.concentrationAura = func() *Aura { return spells.ConcentrationEffect }
	character.classic60PaladinMisc = state
	if rank := talents[classic60PaladinPursuitOfJustice]; rank > 0 {
		spells.PursuitOfJusticeAura = character.NewPassiveMovementSpeedAura("Classic Pursuit of Justice", ActionID{SpellID: []int32{0, 26022, 26023}[rank]}, .04*float64(rank))
		spells.PursuitOfJusticeAura.ApplyOnInit(func(aura *Aura, _ *Simulation) {
			// A learned passive survives a stronger temporary effect. The
			// generic helper's SingleAura category otherwise deletes it.
			aura.ExclusiveEffects[0].Category.SingleAura = false
		})
	}
	rank := talents[classic60PaladinVindication]
	if rank == 0 {
		return
	}
	// Spell.dbc 9452/26016/26021 proc flags20 allow landed melee swings
	// and melee abilities, triggering 67/26017/26018: -5/10/15 percent
	// Strength AND Agility for ten seconds. VMaNGOS release db-13b49dc
	// spell_proc_event entry9452 supplies 3PPM, inherited by higher ranks
	// in SpellMgr.cpp. These are explicit server-reference proc semantics,
	// not the pinned DPS simulator's incorrect self attack-power buff.
	spellID := []int32{0, 67, 26017, 26018}[rank]
	multiplier := 1 - .05*float64(rank)
	spells.VindicationAuras = character.NewEnemyAuraArray(func(target *Unit) *Aura {
		strength := target.NewDynamicMultiplyStat(stats.Strength, multiplier)
		agility := target.NewDynamicMultiplyStat(stats.Agility, multiplier)
		aura := target.RegisterAura(Aura{
			Label: fmt.Sprintf("Classic Vindication-%d", character.UnitIndex), ActionID: ActionID{SpellID: spellID}, Duration: 10 * time.Second,
		})
		// Preserve any explicit target attribute dependencies. NPC attack
		// power/dodge are otherwise independent encounter inputs; inventing
		// Strength->AP here would count already supplied NPC AP twice.
		aura.NewExclusiveEffect("Classic Vindication", true, ExclusiveEffect{
			Priority: float64(rank),
			OnGain: func(_ *ExclusiveEffect, sim *Simulation) {
				target.EnableDynamicStatDep(sim, strength)
				target.EnableDynamicStatDep(sim, agility)
			},
			OnExpire: func(_ *ExclusiveEffect, sim *Simulation) {
				target.DisableDynamicStatDep(sim, strength)
				target.DisableDynamicStatDep(sim, agility)
			},
		})
		return aura
	})
	spells.VindicationProc = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: spellID}, Rank: int32(rank),
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskEmpty, WeaponAttackSource: WeaponAttackSourceNone,
		Flags: SpellFlagPassiveSpell | SpellFlagBinary | SpellFlagNoOnCastComplete,
		Cast:  CastConfig{IgnoreHaste: true}, RelatedAuraArrays: spells.VindicationAuras.ToMap(),
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			result := spell.CalcOutcome(sim, target, func(sim *Simulation, result *SpellResult, table *AttackTable) {
				// VMaNGOS Creature.cpp maps explicit CREATURE_IMMUNITY_MOD_STAT
				// (0x04) to aura137 immunity. Level alone does not imply it.
				if target.classic60VindicationImmune || target.classic60ImmuneSchools.Matches(SpellSchoolHoly) {
					result.Outcome = OutcomeImmune
					spell.SpellMetrics[target.UnitIndex].Misses++
					return
				}
				spell.OutcomeMagicHit(sim, result, table)
			})
			if result.Landed() {
				spells.VindicationAuras.Get(target).Activate(sim)
			}
			spell.DealOutcome(sim, result)
		},
	})
	ppm := character.NewStaticLegacyPPMManager(3, ProcMaskMelee)
	MakePermanent(character.RegisterAura(Aura{
		Label: "Classic Vindication Trigger",
		OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
			// UnitAuraProcHandler.cpp at 8f4e608450460efe1e38743e4da74397d4773a3a
			// excludes aura-triggered proc chains: Seal damage is not another
			// Vindication opportunity. Ordinary melee specials remain eligible.
			if spell.Unit != &character.Unit || spell.Flags.Matches(SpellFlagPassiveSpell) || !result.Landed() ||
				!spell.ProcMask.Matches(ProcMaskMelee) || result.Target.Type != EnemyUnit ||
				result.Target.Env != character.Env || result.Target.resolvedRuleset().id != rulesetClassic60PaladinReference {
				return
			}
			if ppm.Proc(sim, spell.ProcMask, "Classic Vindication") {
				spells.VindicationProc.Cast(sim, result.Target)
			}
		},
	}))
}

// These values are Classic mechanic IDs, not aura types. In particular,
// Repentance is mechanic14 and is not Unyielding Faith's disorient mechanic2.
type classic60PaladinControlMechanic uint8

const (
	classic60ControlDisorient classic60PaladinControlMechanic = 2
	classic60ControlFear      classic60PaladinControlMechanic = 5
	classic60ControlSilence   classic60PaladinControlMechanic = 9
	classic60ControlInterrupt classic60PaladinControlMechanic = 26
	classic60DisorientAuraTag                                 = "Classic Disorient"
)

type classic60PaladinControlEffect struct {
	aura         *Aura
	mechanic     classic60PaladinControlMechanic
	lockedSchool SpellSchool
}

func (state *classic60PaladinMiscTalents) blocksCast(spell *Spell) bool {
	if state == nil || spell.Unit != state.unit || spell.Flags.Matches(SpellFlagPassiveSpell) || spell.SpellSchool == SpellSchoolPhysical {
		return false
	}
	for _, effect := range state.controlEffects {
		if !effect.aura.IsActive() {
			continue
		}
		// VMaNGOS Spell.cpp::CheckCasterAuras permits an immunity-granting
		// cast through the aura it will remove. These two Classic spells
		// grant all-school immunity and purge the magical silence effect.
		if effect.mechanic == classic60ControlSilence && (spell.SpellID == 1020 || spell.SpellID == 5573) {
			continue
		}
		if effect.mechanic == classic60ControlSilence ||
			effect.mechanic == classic60ControlInterrupt && effect.lockedSchool.Matches(spell.SpellSchool) {
			return true
		}
	}
	return false
}

func (state *classic60PaladinMiscTalents) breakControls(sim *Simulation) {
	if state == nil {
		return
	}
	for _, effect := range state.controlEffects {
		if effect.mechanic != classic60ControlInterrupt {
			effect.aura.Deactivate(sim)
		}
	}
}

func (state *classic60PaladinMiscTalents) controlResistance(mechanic classic60PaladinControlMechanic) float64 {
	switch mechanic {
	case classic60ControlFear, classic60ControlDisorient:
		return state.unyieldingFaith
	case classic60ControlSilence, classic60ControlInterrupt:
		if aura := state.concentrationAura(); aura != nil && aura.IsActive() {
			return state.improvedConcentration
		}
	}
	return 0
}

// This private encounter API intentionally describes a control EFFECT with
// main Mechanic=0. VMaNGOS Unit.cpp::IsEffectResist at 8f4e608450460efe1e38743e4da74397d4773a3a
// gives such an effect its own resistance roll after the spell's magic hit.
// Spells whose main Mechanic is control instead alter the initial magic hit
// chance; that different form is not represented by this API. Durations and
// spell IDs remain explicit encounter inputs rather than an invented NPC
// spell catalog. Native fear/disorient, cast interruption and school lockout
// state make these effects observable to the actual cast/swing scheduler.
func registerClassic60PaladinControlEffect(caster *Unit, character *Character, spellID int32, school SpellSchool, duration time.Duration, mechanic classic60PaladinControlMechanic) (*Spell, *Aura) {
	if caster == nil || character == nil || caster.Type != EnemyUnit || caster.Level < 60 || caster.Level > 63 || caster.Env != character.Env ||
		character.resolvedRuleset().id != rulesetClassic60PaladinReference || character.classic60PaladinMisc == nil || duration <= 0 {
		panic("Classic control effects require an owned Paladin, NPC and positive duration")
	}
	switch school {
	case SpellSchoolArcane, SpellSchoolFire, SpellSchoolFrost, SpellSchoolHoly, SpellSchoolNature, SpellSchoolShadow:
	default:
		panic("Classic control effects require a single magical school")
	}
	state := character.classic60PaladinMisc
	effect := &classic60PaladinControlEffect{mechanic: mechanic}
	label := fmt.Sprintf("Classic control effect %d-%d", spellID, caster.UnitIndex)
	actionID := ActionID{SpellID: spellID}
	switch mechanic {
	case classic60ControlFear:
		effect.aura = character.RegisterFearAura(label, actionID, duration)
	case classic60ControlDisorient:
		kind := &incapacitateKind{tag: classic60DisorientAuraTag, isImmune: func(*Unit) bool { return false }}
		effect.aura = kind.registerAura(&character.Unit, label, actionID, duration)
	case classic60ControlSilence, classic60ControlInterrupt:
		effect.aura = character.RegisterAura(Aura{Label: label, ActionID: actionID, Duration: duration,
			OnGain:   func(_ *Aura, sim *Simulation) { character.Interrupt(sim) },
			OnExpire: func(_ *Aura, _ *Simulation) { effect.lockedSchool = SpellSchoolNone },
		})
	default:
		panic("unsupported Classic control effect mechanic")
	}
	state.controlEffects = append(state.controlEffects, effect)
	spell := caster.RegisterSpell(SpellConfig{
		ActionID: actionID, SpellSchool: school, DefenseType: DefenseTypeMagic,
		WeaponAttackSource: WeaponAttackSourceNone, ProcMask: ProcMaskEmpty, Flags: SpellFlagBinary,
		Cast:               CastConfig{IgnoreHaste: true},
		ExtraCastCondition: func(_ *Simulation, target *Unit) bool { return target == &character.Unit },
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			result := spell.CalcOutcome(sim, target, func(sim *Simulation, result *SpellResult, table *AttackTable) {
				liveClassic60PaladinIncomingSpellView(spell, table)
				if target.classic60ImmuneSchools.Matches(school) || mechanic == classic60ControlFear && target.PseudoStats.FearImmune {
					result.Outcome = OutcomeImmune
					spell.SpellMetrics[target.UnitIndex].Misses++
					return
				}
				spell.OutcomeMagicHitNoHitCounter(sim, result, table)
				if !result.Landed() {
					return
				}
				if sim.Proc(state.controlResistance(mechanic), "Classic control effect resistance") {
					result.Outcome = OutcomeMiss
					spell.SpellMetrics[target.UnitIndex].Misses++
					return
				}
				spell.SpellMetrics[target.UnitIndex].Hits++
			})
			if result.Landed() {
				if mechanic != classic60ControlInterrupt {
					effect.aura.Activate(sim)
				} else if character.Hardcast.Expires > sim.CurrentTime {
					if interrupted := character.GetSpell(character.Hardcast.ActionID); interrupted != nil {
						effect.lockedSchool = interrupted.SpellSchool
						effect.aura.Activate(sim)
					}
				}
			}
			spell.DealOutcome(sim, result)
		},
	})
	return spell, effect.aura
}
