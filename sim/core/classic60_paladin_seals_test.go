package core

import (
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func classic60PaladinSealAlly(agent *classic60PaladinTestAgent) (*FakeAgent, *classic60PaladinSpells) {
	ally := classic60PaladinHealingAlly(agent)
	ally.Equipment = agent.Equipment
	ally.EnableAutoAttacks(ally, AutoAttackOptions{MainHand: ally.WeaponFromMainHand(), AutoSwingMelee: true})
	ally.AutoAttacks.RandomMeleeOffset = false
	spells := registerClassic60ReferencePaladin(&ally.Character, true)
	classic60PaladinAt(agent, 100*time.Millisecond, func(sim *Simulation) { ally.AutoAttacks.CancelAutoSwing(sim) })
	return ally, spells
}

func TestClassic60PaladinCrossCasterCrusaderExclusivity(t *testing.T) {
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 6 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		ally, strong := classic60PaladinSealAlly(agent)
		agent.spells.registerAdditionalSeals(&agent.Character, classic60PaladinTalents{})
		strong.registerAdditionalSeals(&ally.Character, classic60PaladinTalents{classic60PaladinImprovedSealOfTheCrusader: 3})
		ally.Finalize()
		weak := agent.spells
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			assertFloat64(t, "cross-caster Crusader resets", agent.CurrentTarget.classic60HolyDamageTaken, 0)
			weak.CrusaderJudgement.Cast(sim, agent.CurrentTarget)
			assertFloat64(t, "first Crusader bonus", agent.CurrentTarget.classic60HolyDamageTaken, 140)
			strong.CrusaderJudgement.Cast(sim, agent.CurrentTarget)
			assertFloat64(t, "strongest Crusader replaces rather than adds", agent.CurrentTarget.classic60HolyDamageTaken, 161)
			if weak.activeJudgement.IsActive() || !strong.activeJudgement.IsActive() {
				t.Fatal("stronger Crusader did not exclusively own the target effect")
			}
		})
		classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
			weak.CrusaderJudgement.Cast(sim, agent.CurrentTarget)
			assertFloat64(t, "weaker Crusader cannot replace stronger", agent.CurrentTarget.classic60HolyDamageTaken, 161)
			weak.LightJudgement.Cast(sim, agent.CurrentTarget)
			assertFloat64(t, "another caster's judgement switch preserves Crusader", agent.CurrentTarget.classic60HolyDamageTaken, 161)
			if !strong.activeJudgement.IsActive() || !weak.activeJudgement.IsActive() {
				t.Fatal("different casters lost independently owned judgements")
			}
		})
		classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
			strong.WisdomJudgement.Cast(sim, agent.CurrentTarget)
			assertFloat64(t, "Crusader owner's switch removes its bonus", agent.CurrentTarget.classic60HolyDamageTaken, 0)
			if !weak.activeJudgement.IsActive() {
				t.Fatal("foreign judgement switch removed Light")
			}
			weak.CrusaderJudgement.Cast(sim, agent.CurrentTarget)
			assertFloat64(t, "weaker Crusader works once stronger is gone", agent.CurrentTarget.classic60HolyDamageTaken, 140)
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinCrossCasterRestorativeJudgements(t *testing.T) {
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 5 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		ally, second := classic60PaladinSealAlly(agent)
		agent.spells.registerAdditionalSeals(&agent.Character, classic60PaladinTalents{})
		second.registerAdditionalSeals(&ally.Character, classic60PaladinTalents{})
		ally.Finalize()
		first := agent.spells
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			first.LightJudgement.Cast(sim, agent.CurrentTarget)
			second.LightJudgement.Cast(sim, agent.CurrentTarget)
			if first.activeJudgement.IsActive() || !second.activeJudgement.IsActive() {
				t.Fatal("same-rank Light retained two active proc auras")
			}
			agent.RemoveHealth(sim, 500)
			before := agent.CurrentHealth()
			attack := agent.AutoAttacks.MHAuto()
			classic60PaladinRolls(t, sim, []float64{.1}, func() { attack.CalcAndDealOutcome(sim, agent.CurrentTarget, attack.OutcomeAlwaysHit) })
			assertFloat64(t, "two Paladins still provide only one Light proc", agent.CurrentHealth()-before, 61)
		})
		classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
			first.WisdomJudgement.Cast(sim, agent.CurrentTarget)
			if !second.activeJudgement.IsActive() {
				t.Fatal("former Light owner removed replacement caster's Light")
			}
			second.WisdomJudgement.Cast(sim, agent.CurrentTarget)
			if first.activeJudgement.IsActive() || !second.activeJudgement.IsActive() {
				t.Fatal("same-rank Wisdom retained two active proc auras")
			}
			agent.currentMana = 1000
			attack := agent.AutoAttacks.MHAuto()
			classic60PaladinRolls(t, sim, []float64{.1}, func() { attack.CalcAndDealOutcome(sim, agent.CurrentTarget, attack.OutcomeAlwaysHit) })
			assertFloat64(t, "two Paladins still provide only one Wisdom proc", agent.CurrentMana(), 1059)
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinRestorativeHealingEffectiveThreat(t *testing.T) {
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 4 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		ally := classic60PaladinHealingAlly(agent)
		// Only recipient class identity is needed for this non-attacking ally's
		// proc threat; its health fixture remains an owned level-60 player.
		ally.Class = proto.Class_ClassWarrior
		agent.spells.registerAdditionalSeals(&agent.Character, classic60PaladinTalents{})
		defense := agent.spells.registerDefense(&agent.Character, classic60PaladinTalents{classic60PaladinImprovedRighteousFury: 3})
		ally.Finalize()
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if !defense.RighteousFury.Cast(sim, &agent.Unit) {
				t.Fatal("RF setup failed")
			}
			for _, test := range []struct {
				unit   *Unit
				spell  *Spell
				factor float64
			}{
				{&agent.Unit, agent.spells.LightProc, .25 * 1.9},
				{&agent.Unit, agent.GetSpell(ActionID{SpellID: 20343}), .25 * 1.9},
				{&ally.Unit, ally.GetSpell(ActionID{SpellID: 20343}), .5},
			} {
				metrics := &test.spell.SpellMetrics[test.unit.UnitIndex]
				before := metrics.TotalThreat
				test.spell.Cast(sim, test.unit)
				assertFloat64(t, "full overheal produces no restorative threat", metrics.TotalThreat-before, 0)
				test.unit.RemoveHealth(sim, 20)
				test.spell.Cast(sim, test.unit)
				assertFloat64(t, "restorative threat uses effective health and caster class", metrics.TotalThreat-before, 20*test.factor)
				assertFloat64(t, "partial overheal restores only missing health", test.unit.CurrentHealth(), test.unit.MaxHealth())
			}
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinCrusaderSwapJudgementRefreshAndReset(t *testing.T) {
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 38 * time.Second, iterations: 2, targetLevel: 60, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		second := classic60PaladinAddAbilityTarget(agent, proto.MobType_MobTypeHumanoid)
		agent.spells.registerAdditionalSeals(&agent.Character, classic60PaladinTalents{classic60PaladinImprovedSealOfTheCrusader: 3, classic60PaladinBenediction: 5})
		s := agent.spells
		assertFloat64(t, "Benediction Crusader cost", s.SealOfTheCrusader.Cost.GetCurrentCost(), 136)
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			assertFloat64(t, "reset Crusader AP", agent.GetStat(stats.AttackPower), 370)
			assertFloat64(t, "reset Crusader haste", agent.PseudoStats.MeleeSpeedMultiplier, 1)
			assertFloat64(t, "reset judgement target", agent.CurrentTarget.classic60HolyDamageTaken, 0)
			if !s.SealOfTheCrusader.Cast(sim, agent.CurrentTarget) {
				t.Fatal("Crusader failed")
			}
			assertFloat64(t, "Crusader AP", agent.GetStat(stats.AttackPower), 370+325.2*1.15)
			assertFloat64(t, "Crusader haste", agent.PseudoStats.MeleeSpeedMultiplier, 1.4)
			assertFloat64(t, "Crusader white penalty", agent.AutoAttacks.MHAuto().DamageMultiplier, 1/1.4)
		})
		classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
			if !s.SealOfCommand.Cast(sim, agent.CurrentTarget) || s.CrusaderAura.IsActive() || !s.SealAura.IsActive() {
				t.Fatal("Command did not replace Crusader")
			}
			assertFloat64(t, "Crusader AP restored", agent.GetStat(stats.AttackPower), 370)
			assertFloat64(t, "Crusader speed restored", agent.PseudoStats.MeleeSpeedMultiplier, 1)
		})
		classic60PaladinAt(agent, 5*time.Second, func(sim *Simulation) {
			if !s.SealOfTheCrusader.Cast(sim, agent.CurrentTarget) || s.SealAura.IsActive() {
				t.Fatal("Crusader did not replace Command")
			}
			if !s.Judgement.Cast(sim, agent.CurrentTarget) || s.currentSeal != nil {
				t.Fatal("Judgement did not consume selected seal")
			}
			assertFloat64(t, "Crusader judgement", agent.CurrentTarget.classic60HolyDamageTaken, 161)
			assertFloat64(t, "consumed Crusader AP", agent.GetStat(stats.AttackPower), 370)
		})
		classic60PaladinAt(agent, 10*time.Second, func(sim *Simulation) {
			attack := agent.AutoAttacks.MHAuto()
			attack.CalcAndDealOutcome(sim, agent.CurrentTarget, attack.OutcomeAlwaysMiss)
			if s.activeJudgement.ExpiresAt() != 15*time.Second {
				t.Fatal("miss refreshed judgement")
			}
			attack.CalcAndDealOutcome(sim, agent.CurrentTarget, attack.OutcomeAlwaysHit)
			if s.activeJudgement.ExpiresAt() != 20*time.Second {
				t.Fatal("own landed melee failed to refresh judgement")
			}
		})
		classic60PaladinAt(agent, 16*time.Second, func(sim *Simulation) {
			if !s.SealOfTheCrusader.Cast(sim, second) || !s.Judgement.Cast(sim, second) {
				t.Fatal("second Crusader judgement failed")
			}
			assertFloat64(t, "old target judgement removed", agent.CurrentTarget.classic60HolyDamageTaken, 0)
			assertFloat64(t, "new target judgement", second.classic60HolyDamageTaken, 161)
		})
		classic60PaladinAt(agent, 27*time.Second, func(_ *Simulation) {
			assertFloat64(t, "judgement expired", second.classic60HolyDamageTaken, 0)
			assertFloat64(t, "no lingering penalty", agent.AutoAttacks.MHAuto().DamageMultiplier, 1)
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinRighteousnessFormulaAndProcEligibility(t *testing.T) {
	var hits []classic60PaladinAbilityEvent
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 5 * time.Second, targetLevel: 60, spellDamage: 100, physicalCrit: 100, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		agent.spells.registerAdditionalSeals(&agent.Character, classic60PaladinTalents{classic60PaladinImprovedSealOfRighteousness: 5, classic60PaladinHolyPower: 5})
		s := agent.spells
		agent.RegisterAura(Aura{Label: "Observe Righteousness", Duration: NeverExpires,
			OnReset: func(aura *Aura, sim *Simulation) { aura.Activate(sim) },
			OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
				if spell == s.RighteousnessProc || spell == s.RighteousnessJudgement {
					hits = append(hits, classic60PaladinAbilityEvent{spellID: spell.SpellID, damage: result.Damage, outcome: result.Outcome})
				}
			},
		})
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if !s.SealOfRighteousness.Cast(sim, agent.CurrentTarget) {
				t.Fatal("Righteousness failed")
			}
			attack := agent.AutoAttacks.MHAuto()
			attack.CalcAndDealOutcome(sim, agent.CurrentTarget, attack.OutcomeAlwaysMiss)
			if len(hits) != 0 {
				t.Fatal("miss triggered Righteousness")
			}
			attack.CalcAndDealOutcome(sim, agent.CurrentTarget, attack.OutcomeAlwaysHit)
			if len(hits) != 1 {
				t.Fatal("landed white did not trigger exactly one Righteousness")
			}
		})
		classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
			classic60PaladinRolls(t, sim, []float64{.5, .5, .999}, func() {
				if !s.Judgement.Cast(sim, agent.CurrentTarget) {
					t.Fatal("Righteousness judgement failed")
				}
			})
			if s.RighteousnessAura.IsActive() {
				t.Fatal("Righteousness not consumed")
			}
		})
	})
	classic60PaladinResult(t, sim.run())
	if len(hits) != 2 {
		t.Fatalf("got %d Righteousness results, want2", len(hits))
	}
	base := 1880.0/87 + (1880.0/25-1880.0/87)*(3.0-1.5)/2.5
	assertFloat64(t, "SoR one coefficient and base-only talent", hits[0].damage, base*1.15+12.5)
	if hits[0].outcome != OutcomeHit {
		t.Fatal("SoR crit or avoided despite guaranteed proc")
	}
	assertFloat64(t, "JoR rank and base-only talent", hits[1].damage, 178.2*1.15+50)
}

func TestClassic60PaladinRestorativeSealsAndLastingJudgement(t *testing.T) {
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 12 * time.Second, iterations: 2, targetLevel: 60, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		agent.spells.registerAdditionalSeals(&agent.Character, classic60PaladinTalents{classic60PaladinLastingJudgement: 3, classic60PaladinBenediction: 5})
		s := agent.spells
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			agent.RemoveHealth(sim, 500)
			if !s.SealOfLight.Cast(sim, agent.CurrentTarget) {
				t.Fatal("Light failed")
			}
			health := agent.CurrentHealth()
			attack := agent.AutoAttacks.MHAuto()
			attack.CalcAndDealOutcome(sim, agent.CurrentTarget, attack.OutcomeAlwaysMiss)
			assertFloat64(t, "Light excludes miss", agent.CurrentHealth(), health)
			classic60PaladinRolls(t, sim, nil, func() { attack.CalcAndDealOutcome(sim, agent.CurrentTarget, attack.OutcomeAlwaysHit) })
			assertFloat64(t, "20PPM at3seconds heals94", agent.CurrentHealth(), health+94)
			if !s.Judgement.Cast(sim, agent.CurrentTarget) || s.SealOfLightAura.IsActive() {
				t.Fatal("Light judgement failed")
			}
			if s.activeJudgement.Duration != 40*time.Second {
				t.Fatal("Lasting Judgement missing")
			}
			classic60PaladinRolls(t, sim, []float64{.499}, func() { attack.CalcAndDealOutcome(sim, agent.CurrentTarget, attack.OutcomeAlwaysHit) })
			assertFloat64(t, "Light judgement heals61", agent.CurrentHealth(), health+155)
		})
		classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
			if !s.SealOfWisdom.Cast(sim, agent.CurrentTarget) {
				t.Fatal("Wisdom failed")
			}
			before := agent.CurrentMana()
			attack := agent.AutoAttacks.MHAuto()
			// Wisdom seal proc is guaranteed at this 3s weapon's20PPM;
			// the still-active Light judgement rolls independently and misses.
			classic60PaladinRolls(t, sim, []float64{.999}, func() { attack.CalcAndDealOutcome(sim, agent.CurrentTarget, attack.OutcomeAlwaysHit) })
			assertFloat64(t, "Wisdom seal mana", agent.CurrentMana(), before+90)
		})
		classic60PaladinAt(agent, 11*time.Second, func(sim *Simulation) {
			if !s.Judgement.Cast(sim, agent.CurrentTarget) || s.SealOfWisdomAura.IsActive() {
				t.Fatal("Wisdom judgement failed")
			}
			before := agent.CurrentMana()
			attack := agent.AutoAttacks.MHAuto()
			classic60PaladinRolls(t, sim, []float64{.499}, func() { attack.CalcAndDealOutcome(sim, agent.CurrentTarget, attack.OutcomeAlwaysHit) })
			assertFloat64(t, "Wisdom judgement mana", agent.CurrentMana(), before+59)
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinOneHandSealScalingAndPPM(t *testing.T) {
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 4 * time.Second, targetLevel: 60, spellDamage: 100, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		agent.Equipment[proto.ItemSlot_ItemSlotMainHand].HandType = proto.HandType_HandTypeOneHand
		agent.Equipment[proto.ItemSlot_ItemSlotMainHand].SwingSpeed = 1.5
		agent.AutoAttacks.SetMH(agent.WeaponFromMainHand())
		agent.spells.registerAdditionalSeals(&agent.Character, classic60PaladinTalents{})
		s := agent.spells
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			result := s.RighteousnessProc.CalcDamage(sim, agent.CurrentTarget, 1880.0/87, s.RighteousnessProc.OutcomeAlwaysHit)
			assertFloat64(t, "one hand coefficient", result.Damage, 1880.0/87+10)
			s.RighteousnessProc.DisposeResult(result)
			if !s.SealOfWisdom.Cast(sim, agent.CurrentTarget) {
				t.Fatal("Wisdom failed")
			}
			mana := agent.CurrentMana()
			attack := agent.AutoAttacks.MHAuto()
			classic60PaladinRolls(t, sim, []float64{.5}, func() { attack.CalcAndDealOutcome(sim, agent.CurrentTarget, attack.OutcomeAlwaysHit) })
			assertFloat64(t, "20PPM at1.5sec miss boundary", agent.CurrentMana(), mana)
			classic60PaladinRolls(t, sim, []float64{.499}, func() { attack.CalcAndDealOutcome(sim, agent.CurrentTarget, attack.OutcomeAlwaysHit) })
			assertFloat64(t, "20PPM at1.5sec hit boundary", agent.CurrentMana(), mana+90)
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinCrusaderBonusAfterOutgoingModifiers(t *testing.T) {
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 3 * time.Second, targetLevel: 60, spellDamage: 100, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		agent.spells.registerAdditionalSeals(&agent.Character, classic60PaladinTalents{classic60PaladinImprovedSealOfTheCrusader: 3})
		probe := agent.RegisterSpell(SpellConfig{ActionID: ActionID{SpellID: 10314, Tag: 99}, SpellSchool: SpellSchoolHoly,
			DefenseType: DefenseTypeMagic, ProcMask: ProcMaskSpellDamage, WeaponAttackSource: WeaponAttackSourceNone,
			Flags: SpellFlagBinary, Cast: CastConfig{IgnoreHaste: true}, DamageMultiplier: 1.5, BonusCoefficient: .5})
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if !agent.spells.SealOfTheCrusader.Cast(sim, agent.CurrentTarget) || !agent.spells.Judgement.Cast(sim, agent.CurrentTarget) {
				t.Fatal("Crusader setup failed")
			}
			result := probe.CalcDamage(sim, agent.CurrentTarget, 100, probe.OutcomeAlwaysHit)
			assertFloat64(t, "Crusader coefficient added after outgoing multiplier", result.Damage, (100+50)*1.5+161*.5)
			probe.DisposeResult(result)
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinJusticeSealStunAndUnsupportedJudgement(t *testing.T) {
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 5 * time.Second, iterations: 2, targetLevel: 60, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		agent.spells.registerAdditionalSeals(&agent.Character, classic60PaladinTalents{classic60PaladinBenediction: 5})
		s := agent.spells
		assertFloat64(t, "Justice base mana and Benediction", s.SealOfJustice.Cost.GetCurrentCost(), 1512*.13*.85)
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			target := agent.CurrentTarget
			target.PseudoStats.StunImmune = false
			if !s.SealOfJustice.Cast(sim, target) {
				t.Fatal("Justice seal failed")
			}
			mana := agent.CurrentMana()
			if s.Judgement.Cast(sim, target) || !s.SealOfJusticeAura.IsActive() || !s.Judgement.CD.IsReady(sim) || agent.CurrentMana() != mana {
				t.Fatal("unsupported Justice judgement consumed resources or seal")
			}
			attack := agent.AutoAttacks.MHAuto()
			classic60PaladinRolls(t, sim, []float64{.25}, func() { attack.CalcAndDealOutcome(sim, target, attack.OutcomeAlwaysHit) })
			if target.PseudoStats.Stunned {
				t.Fatal("Justice exceeded5PPM boundary")
			}
			classic60PaladinRolls(t, sim, []float64{.249, .5}, func() { attack.CalcAndDealOutcome(sim, target, attack.OutcomeAlwaysHit) })
			if !target.PseudoStats.Stunned {
				t.Fatal("Justice failed to stun")
			}
		})
		classic60PaladinAt(agent, 3100*time.Millisecond, func(sim *Simulation) {
			target := agent.CurrentTarget
			if target.PseudoStats.Stunned {
				t.Fatal("Justice exceeded two seconds")
			}
			target.PseudoStats.StunImmune = true
			attack := agent.AutoAttacks.MHAuto()
			classic60PaladinRolls(t, sim, []float64{.1}, func() { attack.CalcAndDealOutcome(sim, target, attack.OutcomeAlwaysHit) })
			if target.PseudoStats.Stunned {
				t.Fatal("Justice bypassed explicit stun immunity")
			}
		})
	})
	classic60PaladinResult(t, sim.run())
}
