package core

import "time"

// Classic's pinned sim/core/cast.go at
// 7779ebbf79dc7f1341e6ab939b28a3402c9a730a adds 1, .8, .6, .4, then .2 seconds
// for successive damaging direct hits during a cast. Spiritual Focus's
// cached tooltip protects Holy Light / Flash of Light by 14% per rank.
// Apply prevention to the spell being cast; the pinned generic handler's
// incoming-spell PushbackReduction lookup does not implement that talent.
// Classic Paladins have no channeled spell in this bundle.
func (spells *classic60PaladinSpells) registerClassicPushback(character *Character, talents classic60PaladinTalents) {
	MakePermanent(character.RegisterAura(Aura{
		Label: "Classic Paladin Spell Pushback",
		OnSpellHitTaken: func(_ *Aura, sim *Simulation, incoming *Spell, result *SpellResult) {
			hardcast := &character.Hardcast
			if !result.Landed() || result.Damage <= 0 || !incoming.ProcMask.Matches(ProcMaskDirect) ||
				hardcast.Expires <= sim.CurrentTime || hardcast.IsChanneled {
				return
			}
			casting := character.GetSpell(hardcast.ActionID)
			if casting == nil || hardcast.Target == nil {
				return
			}
			chance := character.PseudoStats.PushbackChance
			if casting == spells.HolyLight || casting == spells.FlashOfLight {
				chance -= .14 * float64(talents[classic60PaladinSpiritualFocus])
			}
			if !sim.Proc(max(0, min(1, chance)), "Classic Paladin pushback") {
				return
			}
			pushback := time.Second - time.Duration(min(hardcast.classic60PushbackCount, 4))*200*time.Millisecond
			if hardcast.classic60PushbackCount < 4 {
				hardcast.classic60PushbackCount++
			}
			hardcast.Expires += pushback
			casting.SpellMetrics[hardcast.Target.UnitIndex].TotalCastTime += pushback
			character.SetGCDTimer(sim, max(character.NextGCDAt(), hardcast.Expires))
			character.newHardcastAction(sim)
			if character.AutoAttacks.mh.enabled {
				character.AutoAttacks.StopMeleeUntil(sim, hardcast.Expires)
			}
			if sim.Log != nil {
				character.Log(sim, "%s pushed back %s while casting", hardcast.ActionID, pushback)
			}
		},
	}))
}
