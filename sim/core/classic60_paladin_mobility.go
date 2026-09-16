package core

import (
	"fmt"
	"math"
	"time"
)

type classic60PaladinMobility struct {
	BlessingOfFreedom *Spell
	FreedomAura       *Aura
	unit              *Unit
	impairments       []classic60MovementImpairment
	resumeMove        bool
	resumeDestination float64
}

// The pinned Classic tooltips at 7779ebbf79dc7f1341e6ab939b28a3402c9a730a
// and original client DBC for 1044 grant root/snare immunity for
// ten seconds, costs 10% base mana, and has a twenty-second cooldown.
// Guardian's Favor 20174/20175 adds three/six seconds. This models actual
// encounter movement; mounted travel and a complete NPC spell catalog are
// not part of the class bundle.
type classic60MovementImpairment struct {
	aura       *Aura
	multiplier float64
}

func (spells *classic60PaladinSpells) registerMobility(character *Character, talents classic60PaladinTalents) {
	state := &spells.classic60PaladinMobility
	state.unit = &character.Unit
	character.classic60PaladinMobility = state
	spells.FreedomAura = character.RegisterAura(Aura{
		Label: "Classic Blessing of Freedom", ActionID: ActionID{SpellID: 1044},
		Duration: time.Duration(10+3*talents[classic60PaladinGuardiansFavor]) * time.Second,
		OnReset:  func(_ *Aura, _ *Simulation) { state.resumeMove = false },
		OnGain: func(_ *Aura, sim *Simulation) {
			state.clearImpairments(sim)
		},
	})
	spells.BlessingOfFreedom = character.RegisterSpell(SpellConfig{
		ActionID: spells.FreedomAura.ActionID, SpellSchool: SpellSchoolHoly,
		DefenseType: DefenseTypeMagic, WeaponAttackSource: WeaponAttackSourceNone, ProcMask: ProcMaskEmpty,
		Flags: SpellFlagHelpful | SpellFlagAPL, ManaCost: ManaCostOptions{BaseCostPercent: 10},
		Cast:               CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault}, CD: Cooldown{Timer: character.NewTimer(), Duration: 20 * time.Second}},
		ExtraCastCondition: func(_ *Simulation, target *Unit) bool { return target == &character.Unit },
		ApplyEffects:       func(sim *Simulation, _ *Unit, _ *Spell) { spells.activateBlessing(sim, spells.FreedomAura) },
	})
}

func (state *classic60PaladinMobility) clearImpairments(sim *Simulation) {
	for _, impairment := range state.impairments {
		impairment.aura.Deactivate(sim)
	}
}

func (state *classic60PaladinMobility) movementMultiplier() float64 {
	multiplier := 1.0
	for _, impairment := range state.impairments {
		if impairment.aura.IsActive() {
			multiplier = min(multiplier, impairment.multiplier)
		}
	}
	return multiplier
}

// MoveTo calls this after updating its current position. Remember the latest
// requested destination while rooted, without dividing distance by zero.
func (state *classic60PaladinMobility) holdRootedMove(destination float64, sim *Simulation) bool {
	if state.movementMultiplier() != 0 {
		return false
	}
	state.resumeMove, state.resumeDestination = true, destination
	state.stopMovement(sim)
	return true
}

func (state *classic60PaladinMobility) stopMovement(sim *Simulation) {
	if action := state.unit.movementAction; action != nil {
		action.Cancel(sim)
		state.unit.moveAura.Deactivate(sim)
		state.unit.OnMovement(sim, state.unit.DistanceFromTarget, MovementEnd)
	}
}

func (state *classic60PaladinMobility) refreshMovement(sim *Simulation) {
	unit := state.unit
	if action := unit.movementAction; action != nil && action.speed != 0 {
		state.resumeDestination = action.srcPosition + action.moveDistance
		state.resumeMove = true
		unit.UpdatePosition(sim, false)
		state.stopMovement(sim)
	}
	if state.resumeMove && state.movementMultiplier() != 0 {
		destination := state.resumeDestination
		state.resumeMove = false
		unit.MoveTo(destination, sim)
	}
}

// A typed encounter effect, with its own explicit duration and movement
// multiplier (zero=root, 0<factor<1=snare). The binary NPC spell follows the
// audited incoming magical table; Freedom removes/prevents only this owned
// movement-impairment type, never arbitrary debuffs or incapacitation.
func registerClassic60PaladinMovementImpairment(caster *Unit, character *Character, spellID int32, school SpellSchool, duration time.Duration, factor float64) (*Spell, *Aura) {
	if caster == nil || character == nil || caster.Type != EnemyUnit || caster.Level < 60 || caster.Level > 63 || caster.Env != character.Env ||
		character.resolvedRuleset().id != rulesetClassic60PaladinReference || character.classic60PaladinMobility == nil || duration <= 0 ||
		math.IsNaN(factor) || math.IsInf(factor, 0) || factor < 0 || factor >= 1 {
		panic("Classic movement impairment requires an owned Paladin, NPC, duration and root/snare multiplier")
	}
	switch school {
	case SpellSchoolArcane, SpellSchoolFire, SpellSchoolFrost, SpellSchoolHoly, SpellSchoolNature, SpellSchoolShadow:
	default:
		panic("Classic movement impairment requires a single magical school")
	}
	state := character.classic60PaladinMobility
	aura := character.RegisterAura(Aura{
		Label: fmt.Sprintf("Classic movement impairment %d-%d", spellID, caster.UnitIndex), ActionID: ActionID{SpellID: spellID}, Duration: duration,
		OnGain:   func(_ *Aura, sim *Simulation) { state.refreshMovement(sim) },
		OnExpire: func(_ *Aura, sim *Simulation) { state.refreshMovement(sim) },
	})
	state.impairments = append(state.impairments, classic60MovementImpairment{aura: aura, multiplier: factor})
	spell := caster.RegisterSpell(SpellConfig{
		ActionID: aura.ActionID, SpellSchool: school, DefenseType: DefenseTypeMagic,
		WeaponAttackSource: WeaponAttackSourceNone, ProcMask: ProcMaskEmpty, Flags: SpellFlagBinary,
		Cast:               CastConfig{IgnoreHaste: true},
		ExtraCastCondition: func(_ *Simulation, target *Unit) bool { return target == &character.Unit },
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			result := spell.CalcOutcome(sim, target, func(sim *Simulation, result *SpellResult, table *AttackTable) {
				liveClassic60PaladinIncomingSpellView(spell, table)
				if state.FreedomAura.IsActive() || target.classic60ImmuneSchools.Matches(school) {
					result.Outcome = OutcomeImmune
					spell.SpellMetrics[target.UnitIndex].Misses++
					return
				}
				spell.OutcomeMagicHit(sim, result, table)
			})
			if result.Landed() {
				aura.Activate(sim)
				// Aura expiration is otherwise lazy until another event. Root
				// or snare expiry must resume/replan continuous movement at its
				// actual boundary, even in an otherwise idle encounter.
				wake := sim.GetConsumedPendingActionFromPool()
				wake.NextActionAt, wake.Priority = aura.ExpiresAt(), ActionPriorityDOT
				wake.OnAction = func(_ *Simulation) {}
				sim.AddPendingAction(wake)
			}
			spell.DealOutcome(sim, result)
		},
	})
	return spell, aura
}
