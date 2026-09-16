package core

import (
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
)

type classic60PaladinSpells struct {
	SealOfCommand    *Spell
	Judgement        *Spell
	CommandProc      *Spell
	CommandJudgement *Spell
	SealAura         *Aura

	Consecration, Exorcism, HolyWrath, HolyShock                                    *Spell
	BlessingOfMight, BlessingOfWisdom, BlessingOfKings                              *Spell
	MightAura, WisdomAura, KingsAura, SanctityEffect, DevotionEffect, VengeanceAura *Aura

	judgementHitCheck *Spell
	commandICD        Cooldown
	commandPPM        *DynamicProcManager
	commandPending    map[*SpellResult]struct{}
	selectedAura      *Aura
}

func (spells *classic60PaladinSpells) disposePendingCommandResults() {
	for result := range spells.commandPending {
		spells.CommandProc.DisposeResult(result)
		delete(spells.commandPending, result)
	}
}

// registerClassic60ReferencePaladin ports the learned rank-5 Seal of Command
// and its Judgement from the executable source pinned at:
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/paladin/soc.go
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/paladin/judgement.go
//
// This private bundle is not a public Paladin agent or a complete talent build.
// Learning Command is explicit because the pinned registration omits that
// talent check. The validated build entrypoint adds supported talents and other
// abilities; stunned-target damage, alternate seals, weapon swaps and twisting
// remain outside this reference.
// Auto-attacks must already be configured, as in the source's class constructor.
func registerClassic60ReferencePaladin(character *Character, sealOfCommandLearned bool) *classic60PaladinSpells {
	if !sealOfCommandLearned {
		panic("Classic Command reference requires the learned talent")
	}
	talents := classic60PaladinTalents{}
	talents[classic60PaladinSealOfCommand] = 1
	return registerClassic60PaladinCommand(character, talents)
}

func registerClassic60PaladinCommand(character *Character, talents classic60PaladinTalents) *classic60PaladinSpells {
	if character == nil || character.Type != PlayerUnit || character.Level != 60 ||
		character.Race != proto.Race_RaceHuman || character.Class != proto.Class_ClassPaladin ||
		!character.HasManaBar() || character.resolvedRuleset().id != rulesetClassic60PaladinReference ||
		!character.AutoAttacks.AutoSwingMelee {
		panic("Classic rank-5 Command requires the internal level-60 Human Paladin mana/melee reference and a learned seal")
	}

	spells := &classic60PaladinSpells{
		commandICD: Cooldown{Timer: character.NewTimer(), Duration: time.Second},
		// The pinned PPM manager captures unhasted weapon speed. No swap
		// callback is needed in this immutable-weapon reference.
		commandPPM:     character.NewStaticLegacyPPMManager(7, ProcMaskMelee),
		commandPending: make(map[*SpellResult]struct{}),
	}

	if talents[classic60PaladinSealOfCommand] == 0 {
		return spells
	}
	weaponMultiplier := 1 + .02*float64(talents[classic60PaladinTwoHandedWeaponSpecialization])
	benediction := float64(100-3*talents[classic60PaladinBenediction]) / 100

	// The source asks its Judgement wrapper for a magical hit-only result.
	// Use a separately classified, non-damaging helper in the modern engine;
	// the actual Judgement of Command remains a Holy melee crit-only spell.
	spells.judgementHitCheck = character.RegisterSpell(SpellConfig{
		ActionID:    ActionID{SpellID: 20271, Tag: 1},
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskEmpty, WeaponAttackSource: WeaponAttackSourceNone,
		Flags: SpellFlagNoMetrics | SpellFlagNoLogs | SpellFlagNoOnCastComplete,
		Cast:  CastConfig{IgnoreHaste: true},
	})

	spells.CommandJudgement = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 20966}, Rank: 5,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMelee,
		ProcMask: ProcMaskMeleeMHSpecial, WeaponAttackSource: WeaponAttackSourceMainHand,
		Flags:            SpellFlagMeleeMetrics | SpellFlagNoOnCastComplete,
		Cast:             CastConfig{IgnoreHaste: true},
		DamageMultiplier: weaponMultiplier, ThreatMultiplier: 1, BonusCoefficient: 0.429,
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			// Only the base roll is halved for an unstunned target. The source
			// does this unconditionally and does not implement the stun bonus.
			baseDamage := sim.Roll(339, 373) * 0.5
			hit := spells.judgementHitCheck.CalcOutcome(sim, target, spells.judgementHitCheck.OutcomeMagicHit)
			landed := hit.Landed()
			spells.judgementHitCheck.DisposeResult(hit)
			if landed {
				spell.CalcAndDealDamage(sim, target, baseDamage, spell.OutcomeMeleeSpecialCritOnly)
			} else {
				spell.CalcAndDealDamage(sim, target, baseDamage, spell.OutcomeAlwaysMiss)
			}
		},
	})
	spells.CommandJudgement.classic60PaladinAttack = classic60PaladinCommandJudgement

	spells.CommandProc = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 20947}, Rank: 5,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMelee,
		// The pinned mask also includes generic melee/damage-proc bits.
		// With external proc hooks excluded, this explicit MH-special mask
		// preserves its hand/outcome contract without admitting those hooks.
		ProcMask: ProcMaskMeleeMHSpecial, WeaponAttackSource: WeaponAttackSourceMainHand,
		Flags: SpellFlagMeleeMetrics | SpellFlagPassiveSpell,
		Cast:  CastConfig{IgnoreHaste: true},
		// CalcDamage adds the coefficient before applying DamageMultiplier:
		// executable source scaling is 0.7 * (weapon damage + 0.29 * SP),
		// i.e. an effective 0.203 SP coefficient, not an additive 0.29 SP.
		DamageMultiplier: 0.7 * weaponMultiplier, ThreatMultiplier: 1, BonusCoefficient: 0.29,
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			baseDamage := character.MHWeaponDamage(sim, spell.MeleeAttackPower(target))
			result := spell.CalcDamage(sim, target, baseDamage, spell.OutcomeMeleeSpecialHitAndCrit)
			spells.commandPending[result] = struct{}{}
			// Preserve only this explicit source delay: Classic's pinned
			// SpellBatchWindow is 10ms. Do not inherit the TBC batch window.
			action := sim.GetConsumedPendingActionFromPool()
			action.NextActionAt = sim.CurrentTime + 10*time.Millisecond
			action.Priority = ActionPriorityGCD
			action.OnAction = func(sim *Simulation) {
				delete(spells.commandPending, result)
				spell.DealDamage(sim, result)
			}
			action.CleanUp = func(_ *Simulation) {
				if _, pending := spells.commandPending[result]; pending {
					delete(spells.commandPending, result)
					spell.DisposeResult(result)
				}
			}
			sim.AddPendingAction(action)
		},
	})
	spells.CommandProc.classic60PaladinAttack = classic60PaladinCommandProc

	spells.SealAura = character.RegisterAura(Aura{
		Label: "Seal of Command (Rank 5)", ActionID: ActionID{SpellID: 20920},
		Duration: 30 * time.Second,
		// Step can discard the first action past the encounter boundary
		// before generic pending-action cleanup sees it. Keep ownership of
		// calculated results until delivery or the iteration/reset boundary.
		OnDoneIteration: func(_ *Aura, _ *Simulation) { spells.disposePendingCommandResults() },
		OnReset:         func(_ *Aura, _ *Simulation) { spells.disposePendingCommandResults() },
		OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
			if !result.Landed() || !spell.ProcMask.Matches(ProcMaskMeleeWhiteHit) {
				return
			}
			if spells.commandICD.IsReady(sim) && spells.commandPPM.Proc(sim, spell.ProcMask, "seal of command") {
				spells.commandICD.Use(sim)
				spells.CommandProc.Cast(sim, result.Target)
			}
		},
	})

	spells.SealOfCommand = character.RegisterSpell(SpellConfig{
		ActionID: spells.SealAura.ActionID, Rank: 5,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskEmpty, WeaponAttackSource: WeaponAttackSourceNone,
		Flags:    SpellFlagAPL,
		ManaCost: ManaCostOptions{FlatCost: 210, PercentModifier: benediction},
		Cast: CastConfig{
			IgnoreHaste: true,
			DefaultCast: Cast{GCD: GCDDefault},
		},
		ApplyEffects: func(sim *Simulation, _ *Unit, _ *Spell) { spells.SealAura.Activate(sim) },
	})

	spells.Judgement = character.RegisterSpell(SpellConfig{
		ActionID:    ActionID{SpellID: 20271},
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskEmpty, WeaponAttackSource: WeaponAttackSourceNone,
		Flags: SpellFlagMeleeMetrics | SpellFlagAPL | SpellFlagNoOnCastComplete | SpellFlagPassiveSpell,
		// The pinned base mana is 1512: preserve the fractional 90.72 cost.
		ManaCost: ManaCostOptions{BaseCostPercent: 6, PercentModifier: benediction},
		Cast: CastConfig{
			IgnoreHaste: true,
			DefaultCast: Cast{NonEmpty: true},
			CD:          Cooldown{Timer: character.NewTimer(), Duration: time.Duration(10-talents[classic60PaladinImprovedJudgement]) * time.Second},
		},
		ExtraCastCondition: func(_ *Simulation, _ *Unit) bool { return spells.SealAura.IsActive() },
		ApplyEffects: func(sim *Simulation, target *Unit, _ *Spell) {
			spells.CommandJudgement.Cast(sim, target)
			// The seal is consumed whether the judgement hits or misses.
			spells.SealAura.Deactivate(sim)
		},
	})

	return spells
}
