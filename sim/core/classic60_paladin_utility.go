package core

import (
	"fmt"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
)

var classic60RepentanceKind = &incapacitateKind{
	tag:       classic60RepentanceAuraTag,
	isImmune:  func(unit *Unit) bool { return unit.classic60RepentanceImmune },
	setImmune: func(unit *Unit, immune bool) { unit.classic60RepentanceImmune = immune },
}

// The pinned class source does not implement these control abilities. Their
// rank/cost/range/duration/cooldown data instead come from its Classic tooltip
// snapshot: 10308, 20066 and Improved Hammer of Justice 20487–20489.
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/assets/db_inputs/wowhead_spell_tooltips.csv
//
// Use the engine's binary magical hit and control lifecycle. Enemy immunity
// is explicit encounter input, never inferred from level or creature type.
// The rotation skips known immune targets. No PvP diminishing returns,
// dispels or encounter-specific immunity changes are implied by this slice.
func (spells *classic60PaladinSpells) registerCrowdControl(character *Character, talents classic60PaladinTalents) {
	if character == nil || character.resolvedRuleset().id != rulesetClassic60PaladinReference ||
		character.Class != proto.Class_ClassPaladin || character.Level != 60 || !character.HasManaBar() {
		panic("Classic Paladin control requires the internal level-60 Paladin reference")
	}
	validEnemy := func(target *Unit) bool {
		return target != nil && target.Type == EnemyUnit && target.Env == character.Env && target.Level >= 60 && target.Level <= 63
	}
	spells.HammerOfJusticeAuras = character.NewEnemyAuraArray(func(target *Unit) *Aura {
		return target.RegisterStunAura(fmt.Sprintf("Classic Hammer of Justice-%d", character.UnitIndex), ActionID{SpellID: 10308}, 6*time.Second)
	})
	spells.HammerOfJustice = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 10308}, Rank: 4,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskEmpty, WeaponAttackSource: WeaponAttackSourceNone,
		Flags:    SpellFlagAPL | SpellFlagBinary,
		ManaCost: ManaCostOptions{FlatCost: 100},
		Cast: CastConfig{
			IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault},
			CD: Cooldown{Timer: character.NewTimer(), Duration: time.Duration(60-5*talents[classic60PaladinImprovedHammerOfJustice]) * time.Second},
		},
		MaxRange: 10, RelatedAuraArrays: spells.HammerOfJusticeAuras.ToMap(),
		ExtraCastCondition: func(_ *Simulation, target *Unit) bool { return validEnemy(target) && !target.PseudoStats.StunImmune },
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			result := spell.CalcOutcome(sim, target, spell.OutcomeMagicHit)
			if result.Landed() {
				ApplyStun(sim, spells.HammerOfJusticeAuras.Get(target))
			}
			spell.DealOutcome(sim, result)
		},
	})
	if talents[classic60PaladinRepentance] == 0 {
		return
	}
	spells.RepentanceAuras = character.NewEnemyAuraArray(func(target *Unit) *Aura {
		aura := classic60RepentanceKind.registerAura(target, fmt.Sprintf("Classic Repentance-%d", character.UnitIndex), ActionID{SpellID: 20066}, 6*time.Second)
		breakOnDamage := func(aura *Aura, sim *Simulation, _ *Spell, result *SpellResult) {
			if result.Damage > 0 {
				aura.Deactivate(sim)
			}
		}
		aura.OnSpellHitTaken = breakOnDamage
		aura.OnPeriodicDamageTaken = breakOnDamage
		return aura
	})
	spells.Repentance = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 20066}, Rank: 1,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskEmpty, WeaponAttackSource: WeaponAttackSourceNone,
		Flags:    SpellFlagAPL | SpellFlagBinary,
		ManaCost: ManaCostOptions{FlatCost: 60},
		Cast: CastConfig{
			IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault},
			CD: Cooldown{Timer: character.NewTimer(), Duration: time.Minute},
		},
		MaxRange: 20, RelatedAuraArrays: spells.RepentanceAuras.ToMap(),
		ExtraCastCondition: func(_ *Simulation, target *Unit) bool {
			return validEnemy(target) && target.MobType == proto.MobType_MobTypeHumanoid && !target.classic60RepentanceImmune
		},
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			result := spell.CalcOutcome(sim, target, spell.OutcomeMagicHit)
			if result.Landed() {
				classic60RepentanceKind.apply(sim, spells.RepentanceAuras.Get(target))
			}
			spell.DealOutcome(sim, result)
		},
	})
}
