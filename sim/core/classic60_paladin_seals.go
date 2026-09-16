package core

import (
	"fmt"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

// Crusader follows the pinned executable Classic implementation:
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/paladin/sotc.go
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/debuffs.go
// Its tooltip rounds the level-60 AP value to 326; the source level curve
// retains 306 + 8*2.4 = 325.2. Item/libram additions are intentionally absent.
func (spells *classic60PaladinSpells) registerAdditionalSeals(character *Character, talents classic60PaladinTalents) {
	spells.registerRighteousness(character, talents)
	spells.registerRestorativeSeals(character, talents)
	spells.registerJusticeSeal(character, talents)
	improved := 1 + .05*float64(talents[classic60PaladinImprovedSealOfTheCrusader])
	attackPower := 325.2 * improved
	holyBonus := 140 * improved
	debuffs := character.NewEnemyAuraArray(func(target *Unit) *Aura {
		aura := target.RegisterAura(Aura{
			Label:    fmt.Sprintf("Classic Judgement of the Crusader-%d", character.UnitIndex),
			ActionID: ActionID{SpellID: 20303}, Duration: time.Duration(10+10*talents[classic60PaladinLastingJudgement]) * time.Second,
			OnSpellHitTaken: func(aura *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
				if spell.Unit == &character.Unit && result.Landed() && spell.ProcMask.Matches(ProcMaskMelee) {
					aura.Refresh(sim)
				}
			},
		})
		// One target-side Crusader effect, as in the pinned shared aura.
		// Keep caster-owned auras for refresh/consumption, but select only
		// the strongest bonus rather than summing multiple Paladins.
		aura.NewExclusiveEffect("Classic Judgement of the Crusader", true, ExclusiveEffect{
			Priority: holyBonus,
			OnGain:   func(effect *ExclusiveEffect, _ *Simulation) { target.classic60HolyDamageTaken += effect.Priority },
			OnExpire: func(effect *ExclusiveEffect, _ *Simulation) { target.classic60HolyDamageTaken -= effect.Priority },
		})
		return aura
	})
	spells.CrusaderJudgement = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 20303}, Rank: 6,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskEmpty, WeaponAttackSource: WeaponAttackSourceNone,
		Flags: SpellFlagMeleeMetrics | SpellFlagNoOnCastComplete,
		Cast:  CastConfig{IgnoreHaste: true}, RelatedAuraArrays: debuffs.ToMap(),
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			spell.CalcAndDealOutcome(sim, target, spell.OutcomeAlwaysHit)
			spells.activateJudgement(sim, debuffs.Get(target))
		},
	})
	spells.CrusaderAura = character.RegisterAura(Aura{
		Label: "Seal of the Crusader (Rank 6)", ActionID: ActionID{SpellID: 20308}, Duration: 30 * time.Second,
		OnGain: func(_ *Aura, sim *Simulation) {
			character.MultiplyMeleeSpeed(sim, 1.4)
			character.AutoAttacks.MHAuto().DamageMultiplier /= 1.4
			character.AddStatDynamic(sim, stats.AttackPower, attackPower)
		},
		OnExpire: func(aura *Aura, sim *Simulation) {
			character.MultiplyMeleeSpeed(sim, 1/1.4)
			character.AutoAttacks.MHAuto().DamageMultiplier *= 1.4
			character.AddStatDynamic(sim, stats.AttackPower, -attackPower)
			spells.sealExpired(aura)
		},
	})
	spells.SealOfTheCrusader = character.RegisterSpell(SpellConfig{
		ActionID: spells.CrusaderAura.ActionID, Rank: 6,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskEmpty, WeaponAttackSource: WeaponAttackSourceNone,
		Flags:    SpellFlagAPL,
		ManaCost: ManaCostOptions{FlatCost: 160, PercentModifier: float64(100-3*talents[classic60PaladinBenediction]) / 100},
		Cast:     CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault}},
		ApplyEffects: func(sim *Simulation, _ *Unit, _ *Spell) {
			spells.activateSeal(sim, spells.CrusaderAura, spells.CrusaderJudgement)
		},
	})
}

// The SoD-derived Righteousness implementation in the pinned WoWSims source
// crits and adds spell power twice. Use the Classic server formula instead:
// vmangos/core@8f4e608450460efe1e38743e4da74397d4773a3a,
// UnitAuraProcHandler.cpp and Spells/SpellEntry.cpp. The rank's 1786 + 2*47
// scaling is also present in CMaNGOS's original Classic spell data at
// 8ec338a1704e7dcb1c0213eb7ed58f9231ade40f, sql/base/dbc/original_data/Spell.sql.
// This is an explicit server reference; the no-crit behavior is independently
// confirmed by Blizzard's May 10, 2024 Classic Era/Hardcore hotfix.
func (spells *classic60PaladinSpells) registerRighteousness(character *Character, talents classic60PaladinTalents) {
	improved := 1 + .03*float64(talents[classic60PaladinImprovedSealOfRighteousness])
	coefficient := .1
	if character.MainHand().HandType == proto.HandType_HandTypeTwoHand {
		coefficient = .125
	}
	amount := 1786.0 + 2*47
	minimum, maximum := amount/87, amount/25
	base := (minimum + (maximum-minimum)*(character.AutoAttacks.MH().SwingSpeed-1.5)/2.5) * improved
	spells.RighteousnessProc = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 25713}, Rank: 8,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskMeleeMHSpecial, WeaponAttackSource: WeaponAttackSourceMainHand,
		Flags: SpellFlagMeleeMetrics | SpellFlagPassiveSpell | SpellFlagIgnoreResists | SpellFlagNoOnCastComplete,
		Cast:  CastConfig{IgnoreHaste: true}, DamageMultiplier: 1, ThreatMultiplier: 1, BonusCoefficient: coefficient,
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			spell.CalcAndDealDamage(sim, target, base, spell.OutcomeAlwaysHit)
		},
	})
	spells.RighteousnessProc.classic60PaladinAttack = classic60PaladinRighteousnessProc
	spells.RighteousnessJudgement = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 20286}, Rank: 8,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskSpellDamage, WeaponAttackSource: WeaponAttackSourceNone,
		Flags: SpellFlagBinary | SpellFlagNoOnCastComplete,
		Cast:  CastConfig{IgnoreHaste: true}, DamageMultiplier: 1, ThreatMultiplier: 1, BonusCoefficient: .5,
		BonusCritPercent: float64(talents[classic60PaladinHolyPower]),
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			spell.CalcAndDealDamage(sim, target, sim.Roll(170.2, 186.2)*improved, spell.OutcomeMagicHitAndCrit)
		},
	})
	spells.RighteousnessAura = character.RegisterAura(Aura{
		Label: "Seal of Righteousness (Rank 8)", ActionID: ActionID{SpellID: 20293}, Duration: 30 * time.Second,
		OnExpire: func(aura *Aura, _ *Simulation) { spells.sealExpired(aura) },
		OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
			if result.Landed() && spell.ProcMask.Matches(ProcMaskMeleeWhiteHit) {
				spells.RighteousnessProc.Cast(sim, result.Target)
			}
		},
	})
	spells.SealOfRighteousness = spells.registerSeal(character, talents, spells.RighteousnessAura, spells.RighteousnessJudgement, 8, 200)
}

func (spells *classic60PaladinSpells) registerSeal(character *Character, talents classic60PaladinTalents, aura *Aura, judgement *Spell, rank, mana int32) *Spell {
	return character.RegisterSpell(SpellConfig{
		ActionID: aura.ActionID, Rank: rank, SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskEmpty, WeaponAttackSource: WeaponAttackSourceNone, Flags: SpellFlagAPL,
		ManaCost:     ManaCostOptions{FlatCost: mana, PercentModifier: float64(100-3*talents[classic60PaladinBenediction]) / 100},
		Cast:         CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault}},
		ApplyEffects: func(sim *Simulation, _ *Unit, _ *Spell) { spells.activateSeal(sim, aura, judgement) },
	})
}

// Light/Wisdom use the explicit server reference's rank-chain proc rates:
// vmangos/core release db-13b49dc, spell_proc_event entries 20165/20166 (20 PPM,
// no cooldown), independently present in cmangos/mangos-classic at
// 8ec338a1704e7dcb1c0213eb7ed58f9231ade40f/sql/base/mangos.sql. These rates are
// server-reference choices, not client-log measurements. The rank amounts,
// 50% judgement chances and zero healing coefficients are in the Classic DBC.
func (spells *classic60PaladinSpells) registerRestorativeSeals(character *Character, talents classic60PaladinTalents) {
	if !character.HasHealthBar() {
		character.EnableHealthBar()
	}
	registerHeal := func(unit *Unit, id int32, amount float64) *Spell {
		// VMaNGOS Spell.cpp's healing path applies .25 for a Paladin
		// recipient-owned proc and .5 for other classes, to effective healing.
		// Original Classic DBC 20340/20343 have no NO_HELPFUL_THREAT flag.
		threat := .5
		if owner := unit.Env.Raid.GetPlayerFromUnit(unit); owner != nil && owner.GetCharacter().Class == proto.Class_ClassPaladin {
			threat = .25
		}
		return unit.GetOrRegisterSpell(SpellConfig{
			ActionID: ActionID{SpellID: id}, SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
			ProcMask: ProcMaskSpellHealing, Flags: SpellFlagHelpful | SpellFlagPassiveSpell | SpellFlagNoOnCastComplete,
			Cast: CastConfig{IgnoreHaste: true}, DamageMultiplier: 1, ThreatMultiplier: threat,
			ApplyEffects: func(sim *Simulation, _ *Unit, spell *Spell) {
				spell.dealClassic60PaladinHealing(sim, spell.CalcHealing(sim, unit, amount, spell.OutcomeHealing))
			},
		})
	}
	spells.LightProc = registerHeal(&character.Unit, 20340, 94)
	mana := character.NewManaMetrics(ActionID{SpellID: 20351})
	spells.WisdomProc = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 20351}, SpellSchool: SpellSchoolHoly, ProcMask: ProcMaskEmpty,
		Flags: SpellFlagHelpful | SpellFlagPassiveSpell | SpellFlagNoOnCastComplete, Cast: CastConfig{IgnoreHaste: true},
		ApplyEffects: func(sim *Simulation, _ *Unit, _ *Spell) { character.AddMana(sim, 90, mana) },
	})
	lightHeals := make(map[*Unit]*Spell)
	wisdomMana := make(map[*Unit]*ResourceMetrics)
	for _, unit := range character.Env.AllUnits {
		if unit.Type == EnemyUnit {
			continue
		}
		if unit.HasHealthBar() {
			lightHeals[unit] = registerHeal(unit, 20343, 61)
		}
		if unit.HasManaBar() {
			wisdomMana[unit] = unit.NewManaMetrics(ActionID{SpellID: 20353})
		}
	}
	registerJudgement := func(id int32, rank int32, light bool) *Spell {
		debuffs := character.NewEnemyAuraArray(func(target *Unit) *Aura {
			aura := target.RegisterAura(Aura{
				Label: fmt.Sprintf("Classic restorative Judgement %d-%d", id, character.UnitIndex), ActionID: ActionID{SpellID: id},
				Duration: time.Duration(10+10*talents[classic60PaladinLastingJudgement]) * time.Second,
				OnSpellHitTaken: func(aura *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
					if !result.Landed() {
						return
					}
					if spell.Unit == &character.Unit && spell.ProcMask.Matches(ProcMaskMelee) {
						aura.Refresh(sim)
					}
					eligible := spell.ProcMask.Matches(ProcMaskMelee)
					if !light {
						eligible = spell.ProcMask.Matches(ProcMaskDirect)
					}
					if !eligible {
						return
					}
					if light {
						if heal := lightHeals[spell.Unit]; heal != nil && sim.Proc(.5, "Judgement of Light") {
							heal.Cast(sim, spell.Unit)
						}
					} else if metrics := wisdomMana[spell.Unit]; metrics != nil && sim.Proc(.5, "Judgement of Wisdom") {
						spell.Unit.AddMana(sim, 59, metrics)
					}
				},
			})
			// Equal-rank Judgements replace the target's previous copy. Their
			// callbacks must never both roll for the same attack. Each caster
			// still owns its own aura when switching judgement or target.
			aura.NewExclusiveEffect(fmt.Sprintf("Classic restorative Judgement %d", id), true, ExclusiveEffect{Priority: float64(rank)})
			return aura
		})
		return character.RegisterSpell(SpellConfig{
			ActionID: ActionID{SpellID: id}, Rank: rank, SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
			ProcMask: ProcMaskEmpty, WeaponAttackSource: WeaponAttackSourceNone,
			Flags: SpellFlagNoOnCastComplete, Cast: CastConfig{IgnoreHaste: true}, RelatedAuraArrays: debuffs.ToMap(),
			ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
				spell.CalcAndDealOutcome(sim, target, spell.OutcomeAlwaysHit)
				spells.activateJudgement(sim, debuffs.Get(target))
			},
		})
	}
	spells.LightJudgement = registerJudgement(20346, 4, true)
	spells.WisdomJudgement = registerJudgement(20355, 3, false)
	registerAura := func(id int32, label string, proc *Spell) *Aura {
		ppm := character.NewStaticLegacyPPMManager(20, ProcMaskMelee)
		return character.RegisterAura(Aura{
			Label: label, ActionID: ActionID{SpellID: id}, Duration: 30 * time.Second,
			OnExpire: func(aura *Aura, _ *Simulation) { spells.sealExpired(aura) },
			OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
				if result.Landed() && spell.ProcMask.Matches(ProcMaskMelee) && ppm.Proc(sim, spell.ProcMask, label) {
					proc.Cast(sim, &character.Unit)
				}
			},
		})
	}
	spells.SealOfLightAura = registerAura(20349, "Seal of Light (Rank 4)", spells.LightProc)
	spells.SealOfWisdomAura = registerAura(20357, "Seal of Wisdom (Rank 3)", spells.WisdomProc)
	spells.SealOfLight = spells.registerSeal(character, talents, spells.SealOfLightAura, spells.LightJudgement, 4, 210)
	spells.SealOfWisdom = spells.registerSeal(character, talents, spells.SealOfWisdomAura, spells.WisdomJudgement, 3, 200)
}

// The same Classic server proc table specifies five PPM for Justice (20164).
// Spell 20170 stuns for two seconds. Judgement of Justice's fleeing suppression
// requires encounter fleeing behavior, so it is deliberately not registered:
// the shared Judgement refuses to consume this seal for an inert effect.
func (spells *classic60PaladinSpells) registerJusticeSeal(character *Character, talents classic60PaladinTalents) {
	stuns := character.NewEnemyAuraArray(func(target *Unit) *Aura {
		return target.RegisterStunAura(fmt.Sprintf("Classic Seal of Justice-%d", character.UnitIndex), ActionID{SpellID: 20170}, 2*time.Second)
	})
	spells.JusticeProc = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 20170}, SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskEmpty, WeaponAttackSource: WeaponAttackSourceNone,
		Flags: SpellFlagBinary | SpellFlagPassiveSpell | SpellFlagNoOnCastComplete,
		Cast:  CastConfig{IgnoreHaste: true}, RelatedAuraArrays: stuns.ToMap(),
		ExtraCastCondition: func(_ *Simulation, target *Unit) bool { return !target.PseudoStats.StunImmune },
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			result := spell.CalcOutcome(sim, target, spell.OutcomeMagicHit)
			if result.Landed() {
				ApplyStun(sim, stuns.Get(target))
			}
			spell.DealOutcome(sim, result)
		},
	})
	ppm := character.NewStaticLegacyPPMManager(5, ProcMaskMelee)
	spells.SealOfJusticeAura = character.RegisterAura(Aura{
		Label: "Seal of Justice", ActionID: ActionID{SpellID: 20164}, Duration: 30 * time.Second,
		OnExpire: func(aura *Aura, _ *Simulation) { spells.sealExpired(aura) },
		OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
			if result.Landed() && spell.ProcMask.Matches(ProcMaskMelee) && ppm.Proc(sim, spell.ProcMask, "Seal of Justice") {
				spells.JusticeProc.Cast(sim, result.Target)
			}
		},
	})
	spells.SealOfJustice = character.RegisterSpell(SpellConfig{
		ActionID: spells.SealOfJusticeAura.ActionID, Rank: 1, SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskEmpty, WeaponAttackSource: WeaponAttackSourceNone, Flags: SpellFlagAPL,
		ManaCost:     ManaCostOptions{BaseCostPercent: 13, PercentModifier: float64(100-3*talents[classic60PaladinBenediction]) / 100},
		Cast:         CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault}},
		ApplyEffects: func(sim *Simulation, _ *Unit, _ *Spell) { spells.activateSeal(sim, spells.SealOfJusticeAura, nil) },
	})
}
