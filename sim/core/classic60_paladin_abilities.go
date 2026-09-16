package core

import (
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
)

// registerOffensiveAbilities ports the level-60 ranks from the pinned Classic
// paladin/{consecration,exorcism,holy_wrath,holy_shock}.go implementations:
// https://github.com/wowsims/classic/tree/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/paladin
// Holy Shock here is its offensive use only. Healing and Hammer of Wrath's
// distinct ranged outcome table are not silently borrowed from TBC.
func (spells *classic60PaladinSpells) registerOffensiveAbilities(character *Character, talents classic60PaladinTalents) {
	if character == nil || character.resolvedRuleset().id != rulesetClassic60PaladinReference ||
		character.Class != proto.Class_ClassPaladin || character.Level != 60 || !character.HasManaBar() {
		panic("Classic Paladin abilities require the internal level-60 Paladin reference")
	}

	// The source adds Holy Power through SpellCritChance's school modifier.
	// It does not add that bonus to melee-classified seal or judgement crits.
	holyCrit := float64(talents[classic60PaladinHolyPower])
	if talents[classic60PaladinConsecration] != 0 {
		spells.Consecration = character.RegisterSpell(SpellConfig{
			ActionID: ActionID{SpellID: 20924}, Rank: 5,
			SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
			ProcMask: ProcMaskSpellDamage, WeaponAttackSource: WeaponAttackSourceNone,
			Flags:    SpellFlagPureDot | SpellFlagAPL,
			ManaCost: ManaCostOptions{FlatCost: 565},
			Cast: CastConfig{
				IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault},
				CD: Cooldown{Timer: character.NewTimer(), Duration: 8 * time.Second},
			},
			DamageMultiplier: 1, ThreatMultiplier: 1, BonusCoefficient: 0.042,
			Dot: DotConfig{
				IsAOE:         true,
				Aura:          Aura{Label: "Consecration (Rank 5)"},
				NumberOfTicks: 8, TickLength: time.Second, BonusCoefficient: 0.042,
				OnSnapshot: func(_ *Simulation, _ *Unit, dot *Dot) {
					// An AOE dot's aura belongs to its caster; snapshot against the
					// actual enemy so the owned Classic attack table remains valid.
					dot.Snapshot(character.CurrentTarget, 48)
				},
				OnTick: func(sim *Simulation, _ *Unit, dot *Dot) {
					for _, target := range sim.Encounter.ActiveTargetUnits {
						dot.CalcAndDealPeriodicSnapshotDamage(sim, target, dot.Spell.OutcomeMagicHit)
					}
				},
			},
			ApplyEffects: func(sim *Simulation, _ *Unit, spell *Spell) { spell.AOEDot().Apply(sim) },
		})
	}

	spells.Exorcism = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 10314}, Rank: 6,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskSpellDamage, WeaponAttackSource: WeaponAttackSourceNone,
		// The pinned source explicitly models Exorcism as binary despite
		// being a direct Holy nuke. Preserve that source distinction.
		Flags:    SpellFlagMeleeMetrics | SpellFlagAPL | SpellFlagBinary,
		ManaCost: ManaCostOptions{FlatCost: 345},
		Cast: CastConfig{
			IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault},
			CD: Cooldown{Timer: character.NewTimer(), Duration: 15 * time.Second},
		},
		DamageMultiplier: 1, ThreatMultiplier: 1, BonusCoefficient: 0.429,
		BonusCritPercent:   holyCrit,
		ExtraCastCondition: func(_ *Simulation, target *Unit) bool { return classic60PaladinUndeadOrDemon(target) },
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			spell.CalcAndDealDamage(sim, target, sim.Roll(505, 563), spell.OutcomeMagicHitAndCrit)
		},
	})

	spells.HolyWrath = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 10318}, Rank: 2,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskSpellDamage, WeaponAttackSource: WeaponAttackSourceNone,
		Flags:    SpellFlagAPL,
		ManaCost: ManaCostOptions{FlatCost: 805},
		Cast: CastConfig{
			IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault, CastTime: 2 * time.Second},
			CD: Cooldown{Timer: character.NewTimer(), Duration: time.Minute},
		},
		DamageMultiplier: 1, ThreatMultiplier: 1, BonusCoefficient: 0.19,
		BonusCritPercent: holyCrit,
		ApplyEffects: func(sim *Simulation, _ *Unit, spell *Spell) {
			results := make([]*SpellResult, 0, len(sim.Encounter.ActiveTargetUnits))
			for _, target := range sim.Encounter.ActiveTargetUnits {
				if classic60PaladinUndeadOrDemon(target) {
					results = append(results, spell.CalcDamage(sim, target, sim.Roll(490, 576), spell.OutcomeMagicHitAndCrit))
				}
			}
			// Resolve the whole cast before delivering any result, as in the
			// source: an early target's Vengeance proc cannot buff later targets.
			for _, result := range results {
				spell.DealDamage(sim, result)
			}
		},
	})

	if talents[classic60PaladinHolyShock] != 0 {
		spells.HolyShock = character.RegisterSpell(SpellConfig{
			ActionID: ActionID{SpellID: 20930}, Rank: 3,
			SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
			ProcMask: ProcMaskSpellDamage, WeaponAttackSource: WeaponAttackSourceNone,
			Flags:    SpellFlagMeleeMetrics | SpellFlagAPL,
			ManaCost: ManaCostOptions{FlatCost: 325},
			Cast: CastConfig{
				IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault},
				CD: Cooldown{Timer: character.NewTimer(), Duration: 30 * time.Second},
			},
			DamageMultiplier: 1, ThreatMultiplier: 1, BonusCoefficient: 0.429,
			BonusCritPercent: holyCrit,
			ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
				spell.CalcAndDealDamage(sim, target, sim.Roll(365, 395), spell.OutcomeMagicHitAndCrit)
			},
		})
	}
}

func classic60PaladinUndeadOrDemon(target *Unit) bool {
	return target != nil && (target.MobType == proto.MobType_MobTypeUndead || target.MobType == proto.MobType_MobTypeDemon)
}
