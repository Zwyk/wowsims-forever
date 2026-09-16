package core

import (
	"fmt"
	"math"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

type classic60PaladinHealing struct {
	HolyLight, FlashOfLight, LayOnHands, HolyShockHeal, DivineFavor *Spell
	DivineFavorAura, LayOnHandsArmorAura                            *Aura
	layOnHandsArmorAuras                                            map[*Unit]*Aura
}

// Rank data and talent text are pinned in the Classic reference's
// assets/db_inputs/wowhead_spell_tooltips.csv at 7779ebbf79dc7f1341e6ab939b28a3402c9a730a.
// That simulator does not implement Holy Light, Flash of Light or healing Holy
// Shock. Their coefficient reference is the ordinary cast-time calculation in
// vmangos/core, pinned at 8f4e608450460efe1e38743e4da74397d4773a3a:
// src/game/Spells/SpellEntry.cpp, GetCastTimeForBonus/CalculateDefaultCoefficient.
// Cast time is at least 1.5 seconds, then divided by 3.5 seconds: 5/7 for Holy
// Light and 3/7 for Flash of Light and Holy Shock. These are reference formulas,
// not an assertion that every interaction has been confirmed in client logs.
func (spells *classic60PaladinSpells) registerHealingAbilities(character *Character, talents classic60PaladinTalents) {
	if character == nil || character.resolvedRuleset().id != rulesetClassic60PaladinReference ||
		character.Type != PlayerUnit || character.Class != proto.Class_ClassPaladin ||
		character.Level != 60 || character.Env == nil || character.Env.IsFinalized() || !character.HasManaBar() {
		panic("Classic Paladin healing requires the initialized level-60 Paladin reference")
	}
	if !character.HasHealthBar() {
		character.EnableHealthBar()
	}
	validTarget := func(_ *Simulation, target *Unit) bool {
		return classic60PaladinHealingTarget(&character.Unit, target)
	}
	holyCrit := float64(talents[classic60PaladinHolyPower])
	healingLight := 1 + .04*float64(talents[classic60PaladinHealingLight])
	registerHeal := func(id int32, tag int32, rank int32, cost int32, castTime time.Duration, minimum, maximum, coefficient, multiplier float64, cd Cooldown) *Spell {
		return character.RegisterSpell(SpellConfig{
			ActionID: ActionID{SpellID: id, Tag: tag}, Rank: rank,
			SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
			ProcMask: ProcMaskSpellHealing, Flags: SpellFlagHelpful | SpellFlagAPL,
			ManaCost:         ManaCostOptions{FlatCost: cost},
			Cast:             CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault, CastTime: castTime}, CD: cd},
			DamageMultiplier: multiplier, ThreatMultiplier: .25, BonusCoefficient: coefficient,
			BonusCritPercent: holyCrit, ExtraCastCondition: validTarget,
			ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
				base := sim.Roll(minimum, maximum)
				spells.withDivineFavor(spell, func() {
					result := spell.CalcHealing(sim, target, base, func(sim *Simulation, result *SpellResult, table *AttackTable) {
						// VMaNGOS Unit.cpp SpellHealingBonusTaken applies Light's
						// 400/115 through the spell coefficient after caster healing
						// multipliers, before target multipliers and the crit. This
						// ordering is an executable reference, pending client logs.
						if target == &character.Unit && spells.LightAura != nil && spells.LightAura.IsActive() {
							bonus := 0.0
							if spell == spells.HolyLight {
								bonus = 400
							} else if spell == spells.FlashOfLight {
								bonus = 115
							}
							result.Damage += spell.applyTargetHealingModifiers(bonus*coefficient, table)
						}
						spell.OutcomeHealingCrit(sim, result, table)
					})
					spell.dealClassic60PaladinHealing(sim, result)
				})
			},
		})
	}
	// Rank 9 is the level-60 learned book rank; lower-rank selection and
	// progression availability are separate from this maximum-rank reference.
	spells.HolyLight = registerHeal(25292, 0, 9, 660, 2500*time.Millisecond, 1590, 1770, 5.0/7, healingLight, Cooldown{})
	// The cached tooltip encodes level-58 base 343..383 and 2.6 per level,
	// rendering 348..389 at level 60. Preserve the unrounded 348.2..388.2.
	spells.FlashOfLight = registerHeal(19943, 0, 6, 140, 1500*time.Millisecond, 343+2*2.6, 383+2*2.6, 3.0/7, healingLight, Cooldown{})
	if talents[classic60PaladinHolyShock] != 0 {
		if spells.HolyShock == nil {
			panic("Classic healing Holy Shock requires the registered offensive rank and its shared cooldown")
		}
		previousCondition := spells.HolyShock.ExtraCastCondition
		spells.HolyShock.ExtraCastCondition = func(sim *Simulation, target *Unit) bool {
			return target != nil && character.IsOpponent(target) && (previousCondition == nil || previousCondition(sim, target))
		}
		spells.HolyShockHeal = registerHeal(20930, 1, 3, 325, 0, 365, 395, 3.0/7, 1, spells.HolyShock.CD)
	}
	spells.registerClassicLayOnHands(character, talents, validTarget)

	if talents[classic60PaladinIllumination] != 0 {
		mana := character.NewManaMetrics(ActionID{SpellID: 20272})
		MakePermanent(character.RegisterAura(Aura{
			Label: "Classic Illumination",
			OnHealDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
				if result.DidCrit() && (spell == spells.HolyLight || spell == spells.FlashOfLight || spell == spells.HolyShockHeal) &&
					sim.Proc(.2*float64(talents[classic60PaladinIllumination]), "Illumination") {
					// Classic refunds the entire original base mana cost. Neither
					// offensive Holy Shock nor Lay on Hands triggers Illumination.
					character.AddMana(sim, float64(spell.Cost.BaseCost), mana)
				}
			},
		}))
	}
	if talents[classic60PaladinDivineFavor] != 0 {
		spells.DivineFavorAura = character.RegisterAura(Aura{
			Label: "Classic Divine Favor", ActionID: ActionID{SpellID: 20216}, Duration: NeverExpires,
			OnHealDealt: func(aura *Aura, sim *Simulation, spell *Spell, _ *SpellResult) {
				if spell == spells.HolyLight || spell == spells.FlashOfLight || spell == spells.HolyShockHeal {
					aura.Deactivate(sim)
				}
			},
			OnSpellHitDealt: func(aura *Aura, sim *Simulation, spell *Spell, _ *SpellResult) {
				if spell == spells.HolyShock {
					aura.Deactivate(sim)
				}
			},
		})
		spells.DivineFavor = character.RegisterSpell(SpellConfig{
			ActionID: spells.DivineFavorAura.ActionID, SpellSchool: SpellSchoolHoly,
			Flags: SpellFlagHelpful | SpellFlagAPL | SpellFlagNoOnCastComplete,
			// Cached Classic tooltip specifies 4% base mana. Deliberately do
			// not copy the free cost in the incomplete pinned DPS simulator.
			ManaCost: ManaCostOptions{BaseCostPercent: 4},
			Cast: CastConfig{IgnoreHaste: true, DefaultCast: Cast{NonEmpty: true},
				CD: Cooldown{Timer: character.NewTimer(), Duration: 2 * time.Minute}},
			ExtraCastCondition: func(_ *Simulation, target *Unit) bool {
				return target == &character.Unit && !spells.DivineFavorAura.IsActive()
			},
			ApplyEffects: func(sim *Simulation, _ *Unit, _ *Spell) { spells.DivineFavorAura.Activate(sim) },
		})
		if spells.HolyShock != nil {
			effects := spells.HolyShock.ApplyEffects
			spells.HolyShock.ApplyEffects = func(sim *Simulation, target *Unit, spell *Spell) {
				spells.withDivineFavor(spell, func() { effects(sim, target, spell) })
			}
		}
	}
}

// The cast stays an ordinary healing/magic result, retaining native callbacks
// and metrics. The temporary exact 100% chance also respects the bounded
// Classic offensive crit policy when Divine Favor applies to damaging Shock.
func (spells *classic60PaladinSpells) withDivineFavor(spell *Spell, apply func()) {
	if spells.DivineFavorAura == nil || !spells.DivineFavorAura.IsActive() {
		apply()
		return
	}
	previous := spell.BonusCritPercent
	spell.BonusCritPercent = 100 - spell.Unit.GetStat(stats.SpellCritPercent)
	defer func() { spell.BonusCritPercent = previous }()
	apply()
}

func classic60PaladinHealingTarget(caster, target *Unit) bool {
	return caster != nil && target != nil && target.Type == PlayerUnit && !caster.IsOpponent(target) &&
		target.Env == caster.Env && target.resolvedRuleset().id == rulesetClassic60PaladinReference &&
		target.HasHealthBar() && target.CurrentHealth() > 0 && target.UnitIndex >= 0 &&
		int(target.UnitIndex) < len(caster.Env.AllUnits) && caster.Env.AllUnits[target.UnitIndex] == target
}

func (spells *classic60PaladinSpells) registerClassicLayOnHands(character *Character, talents classic60PaladinTalents, validTarget func(*Simulation, *Unit) bool) {
	if rank := talents[classic60PaladinImprovedLayOnHands]; rank != 0 {
		spells.layOnHandsArmorAuras = make(map[*Unit]*Aura)
		for _, target := range character.Env.Raid.AllPlayerUnits {
			agent := character.Env.Raid.GetPlayerFromUnit(target)
			if agent == nil {
				continue
			}
			recipient := agent.GetCharacter()
			id := int32(20233)
			if rank == 2 {
				id = 20236
			}
			aura := recipient.GetOrRegisterAura(Aura{
				Label:    fmt.Sprintf("Classic Improved Lay on Hands (Rank %d)", rank),
				ActionID: ActionID{SpellID: id}, Duration: 2 * time.Minute,
			})
			if len(aura.ExclusiveEffects) == 0 {
				aura.NewExclusiveEffect("Classic Improved Lay on Hands armor", false, ExclusiveEffect{
					Priority: 1 + .15*float64(rank),
					OnGain: func(effect *ExclusiveEffect, sim *Simulation) {
						recipient.ApplyDynamicEquipScaling(sim, stats.Armor, effect.Priority)
					},
					OnExpire: func(effect *ExclusiveEffect, sim *Simulation) {
						recipient.RemoveDynamicEquipScaling(sim, stats.Armor, effect.Priority)
					},
				})
			}
			spells.layOnHandsArmorAuras[target] = aura
			if target == &character.Unit {
				spells.LayOnHandsArmorAura = aura
			}
		}
	}
	spent := character.NewManaMetrics(ActionID{SpellID: 10310})
	restored := make(map[*Unit]*ResourceMetrics)
	for _, target := range character.Env.Raid.AllPlayerUnits {
		if target.HasManaBar() {
			restored[target] = target.NewManaMetrics(ActionID{SpellID: 10310})
		}
	}
	spells.LayOnHands = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 10310}, Rank: 3,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskSpellHealing, Flags: SpellFlagHelpful | SpellFlagAPL,
		Cast: CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault},
			CD: Cooldown{Timer: character.NewTimer(), Duration: time.Duration(60-10*talents[classic60PaladinImprovedLayOnHands]) * time.Minute}},
		DamageMultiplier: 1, ThreatMultiplier: .25, ExtraCastCondition: validTarget,
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			if cost := character.CurrentMana(); cost > 0 {
				character.SpendMana(sim, cost, spent)
				character.PseudoStats.FiveSecondRuleRefreshTime = sim.CurrentTime + 5*time.Second
			}
			spell.dealClassic60PaladinHealing(sim, spell.CalcHealing(sim, target, character.MaxHealth(), spell.OutcomeHealing))
			if metrics := restored[target]; metrics != nil {
				target.AddMana(sim, 550, metrics)
			}
			if aura := spells.layOnHandsArmorAuras[target]; aura != nil {
				aura.Activate(sim)
			}
		},
	})
}

func (spell *Spell) dealClassic60PaladinHealing(sim *Simulation, result *SpellResult) {
	// VMaNGOS Spell.cpp at the same pinned commit, lines 1360-1365: Paladin
	// healing creates 0.25 threat per effective health restored. Retain native
	// overheal reporting and heal callbacks, but do not threaten from overheal.
	effective := math.Min(result.Damage, math.Max(0, result.Target.MaxHealth()-result.Target.CurrentHealth()))
	result.Threat = spell.ThreatFromDamage(sim, result.Outcome, effective, spell.Unit.AttackTables[result.Target.UnitIndex])
	spell.DealHealing(sim, result)
}
