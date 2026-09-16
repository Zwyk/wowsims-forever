package core

import (
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
)

// registerClassic60ReferenceFireball ports the untalented, pre-AQ rank 11 from:
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/mage/fireball.go
//
// Rank 11 is spell 10151 (561–715 damage, 395 mana); spell 25306 is the
// separately gated AQ rank 12. This private diagnostic is not registered by a
// Mage agent or a public simulation. Haste, talents and class-specific proc
// hooks remain excluded by the reference profile.
func registerClassic60ReferenceFireball(character *Character) *Spell {
	if character == nil || character.Type != PlayerUnit || character.Level != 60 ||
		character.Class != proto.Class_ClassMage || !character.HasManaBar() ||
		character.resolvedRuleset().id != rulesetClassic60MageManaReference {
		panic("Classic rank-11 Fireball requires the internal level-60 Mage mana reference")
	}

	actionID := ActionID{SpellID: 10151}
	return character.RegisterSpell(SpellConfig{
		ActionID: actionID, Rank: 11,
		SpellSchool: SpellSchoolFire, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskSpellDamage, WeaponAttackSource: WeaponAttackSourceNone,
		Flags: SpellFlagAPL, MissileSpeed: 24,
		ManaCost: ManaCostOptions{FlatCost: 395},
		Cast: CastConfig{
			IgnoreHaste: true,
			DefaultCast: Cast{GCD: GCDDefault, CastTime: 3500 * time.Millisecond},
		},
		Dot: DotConfig{
			Aura:          Aura{Label: "Fireball (Rank 11)", ActionID: actionID.WithTag(1)},
			NumberOfTicks: 4, TickLength: 2 * time.Second,
			// The source gives the residual no spell-damage coefficient. Modern
			// Dot.Snapshot uses this field separately from the direct coefficient.
			BonusCoefficient: 0,
			OnSnapshot: func(_ *Simulation, target *Unit, dot *Dot) {
				dot.Snapshot(target, 18)
			},
			OnTick: func(sim *Simulation, target *Unit, dot *Dot) {
				dot.CalcAndDealPeriodicSnapshotDamage(sim, target, dot.OutcomeTick)
			},
		},
		DamageMultiplier: 1, ThreatMultiplier: 1, BonusCoefficient: 1,
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			result := spell.CalcDamage(sim, target, sim.Roll(561, 715), spell.OutcomeMagicHitAndCrit)
			// Match the source: resolve direct damage at cast completion, deliver
			// it on impact, and snapshot/apply the residual only on a landed hit.
			spell.WaitTravelTime(sim, func(sim *Simulation) {
				spell.DealDamage(sim, result)
				if result.Landed() {
					spell.Dot(target).Apply(sim)
				}
			})
		},
	})
}
