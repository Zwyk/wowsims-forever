package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/stats"
)

func TestClassic60PaladinVindicationProcAndLifecycle(t *testing.T) {
	for rank := uint8(1); rank <= 3; rank++ {
		t.Run(fmt.Sprint(rank), func(t *testing.T) {
			checks := 0
			sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 13 * time.Second, iterations: 2, pauseAutos: true, targetLevel: 60}, func(agent *classic60PaladinTestAgent) {
				target := agent.CurrentTarget
				target.AddStats(stats.Stats{stats.Strength: 100, stats.Agility: 80, stats.AttackPower: 200})
				agent.spells.registerMiscTalents(&agent.Character, classic60PaladinTalents{classic60PaladinVindication: rank})
				trigger := func(sim *Simulation, rolls []float64, spell *Spell, outcome HitOutcome) {
					classic60PaladinRolls(t, sim, rolls, func() {
						agent.OnSpellHitDealt(sim, spell, &SpellResult{Target: target, Outcome: outcome, Damage: 100})
					})
				}
				classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
					assertFloat64(t, "target Strength reset", target.GetStat(stats.Strength), 100)
					assertFloat64(t, "target Agility reset", target.GetStat(stats.Agility), 80)
					selfAP := agent.GetStat(stats.AttackPower)
					for _, outcome := range []HitOutcome{OutcomeMiss, OutcomeDodge, OutcomeParry} {
						trigger(sim, nil, agent.AutoAttacks.MHAuto(), outcome)
					}
					trigger(sim, nil, agent.spells.CommandProc, OutcomeCrit)
					trigger(sim, nil, &Spell{Unit: &agent.Unit, ProcMask: ProcMaskSpellDamage}, OutcomeHit)
					trigger(sim, []float64{.151}, agent.AutoAttacks.MHAuto(), OutcomeHit)
					if agent.spells.VindicationAuras.Get(target).IsActive() {
						t.Fatal("failed or ineligible melee event applied Vindication")
					}
					// The 3-second fixture weapon at3PPM has15% proc chance;
					// a successful proc still makes its own binary magic hit roll.
					trigger(sim, []float64{.149, .99}, agent.AutoAttacks.MHAuto(), OutcomeHit)
					if agent.spells.VindicationAuras.Get(target).IsActive() {
						t.Fatal("resisted magical debuff was applied")
					}
					trigger(sim, []float64{.149, .5}, agent.AutoAttacks.MHAuto(), OutcomeCrit)
					assertFloat64(t, "enemy Strength decreases", target.GetStat(stats.Strength), 100*(1-.05*float64(rank)))
					assertFloat64(t, "enemy Agility decreases", target.GetStat(stats.Agility), 80*(1-.05*float64(rank)))
					assertFloat64(t, "flat NPC attack power remains independent", target.GetStat(stats.AttackPower), 200)
					assertFloat64(t, "Vindication never buffs caster AP", agent.GetStat(stats.AttackPower), selfAP)
				})
				classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
					// Actual melee special eligibility and no refresh stacking.
					trigger(sim, []float64{.149, .5}, agent.spells.CommandJudgement, OutcomeHit)
					assertFloat64(t, "refresh does not stack", target.GetStat(stats.Strength), 100*(1-.05*float64(rank)))
				})
				classic60PaladinAt(agent, 11500*time.Millisecond, func(_ *Simulation) {
					if !agent.spells.VindicationAuras.Get(target).IsActive() {
						t.Fatal("Vindication expired before refreshed ten-second duration")
					}
				})
				classic60PaladinAt(agent, 12001*time.Millisecond, func(_ *Simulation) {
					checks++
					assertFloat64(t, "expired target Strength restored", target.GetStat(stats.Strength), 100)
					assertFloat64(t, "expired target Agility restored", target.GetStat(stats.Agility), 80)
				})
			})
			classic60PaladinResult(t, sim.run())
			if checks != 2 {
				t.Fatal("both native iterations must finish Vindication lifecycle")
			}
		})
	}
}

func TestClassic60PaladinVindicationExplicitImmunity(t *testing.T) {
	for _, immuneSchool := range []bool{false, true} {
		t.Run(fmt.Sprint(immuneSchool), func(t *testing.T) {
			outcomes := []HitOutcome{}
			sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 2 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
				target := agent.CurrentTarget
				target.AddStats(stats.Stats{stats.Strength: 100, stats.Agility: 80})
				if immuneSchool {
					target.classic60ImmuneSchools = SpellSchoolHoly
				} else {
					target.classic60VindicationImmune = true
				}
				agent.spells.registerMiscTalents(&agent.Character, classic60PaladinTalents{classic60PaladinVindication: 3})
				MakePermanent(agent.RegisterAura(Aura{Label: "Observe Vindication immunity", OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
					if spell == agent.spells.VindicationProc && sim.CurrentTime == time.Second {
						outcomes = append(outcomes, result.Outcome)
					}
				}}))
				classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
					classic60PaladinRolls(t, sim, []float64{.1}, func() {
						agent.OnSpellHitDealt(sim, agent.AutoAttacks.MHAuto(), &SpellResult{Target: target, Outcome: OutcomeHit, Damage: 100})
					})
					assertFloat64(t, "immune target Strength unchanged", target.GetStat(stats.Strength), 100)
					assertFloat64(t, "immune target Agility unchanged", target.GetStat(stats.Agility), 80)
					if agent.spells.VindicationAuras.Get(target).IsActive() {
						t.Fatal("explicitly immune target received Vindication")
					}
				})
			})
			classic60PaladinResult(t, sim.run())
			if len(outcomes) != 2 || outcomes[0] != OutcomeImmune || outcomes[1] != OutcomeImmune {
				t.Fatalf("immunity results missing: %v", outcomes)
			}
		})
	}
}

func TestClassic60PaladinVindicationChangesConfiguredNPCDamage(t *testing.T) {
	// This encounter explicitly derives NPC AP from Strength. A flat NPC AP
	// input is otherwise already final, so the ability must not invent this
	// dependency for all creatures. Native white attacks prove the stat
	// reduction can affect the next incoming hit and restores after expiry.
	sim, _, _, events := classic60PaladinDefenseFixture(t, classic60PaladinTestConfig{duration: 13 * time.Second, iterations: 2, pauseAutos: true}, classic60PaladinTalents{}, nil, 2, func(agent *classic60PaladinTestAgent, _ *classic60PaladinDefenseState) {
		target := agent.CurrentTarget
		target.AddStat(stats.Strength, 140)
		target.AddStatDependency(stats.Strength, stats.AttackPower, 2)
		agent.spells.registerMiscTalents(&agent.Character, classic60PaladinTalents{classic60PaladinVindication: 3})
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			classic60PaladinRolls(t, sim, []float64{.1, .5}, func() {
				agent.OnSpellHitDealt(sim, agent.AutoAttacks.MHAuto(), &SpellResult{Target: target, Outcome: OutcomeHit, Damage: 100})
			})
		})
	})
	classic60PaladinResult(t, sim.run())
	if len(*events) != 14 {
		t.Fatalf("native incoming attack count: got%d", len(*events))
	}
	for iteration := 0; iteration < 2; iteration++ {
		chunk := (*events)[iteration*7 : (iteration+1)*7]
		if chunk[0].at != 0 || chunk[1].at != 2*time.Second || chunk[6].at != 12*time.Second {
			t.Fatalf("unexpected incoming schedule: %+v", chunk)
		}
		assertFloat64(t, "reduced NPC Strength changes AP-derived damage", chunk[0].damage-chunk[1].damage, 400*.00052*42*chunk[0].resistance)
		assertFloat64(t, "expiry restores native incoming damage", chunk[6].damage, chunk[0].damage)
	}
}

func TestClassic60PaladinPursuitOfJusticeMovementAndExclusivity(t *testing.T) {
	for rank := uint8(0); rank <= 2; rank++ {
		t.Run(fmt.Sprint(rank), func(t *testing.T) {
			checks := 0
			sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 4 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
				agent.spells.registerMiscTalents(&agent.Character, classic60PaladinTalents{classic60PaladinPursuitOfJustice: rank})
				stronger := agent.RegisterAura(Aura{Label: "Fixture stronger passive movement", Duration: time.Second})
				stronger.NewPassiveMovementSpeedEffect(.2)
				classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
					wantSpeed := 7 * (1 + .04*float64(rank))
					assertFloat64(t, "rank running speed and reset", agent.GetMovementSpeed(), wantSpeed)
					start := agent.DistanceFromTarget
					agent.MoveTo(start+wantSpeed, sim)
					if agent.movementAction.NextActionAt != 2*time.Second {
						t.Fatalf("running movement should take one second, ends%s", agent.movementAction.NextActionAt)
					}
				})
				classic60PaladinAt(agent, 2001*time.Millisecond, func(sim *Simulation) {
					if agent.Moving {
						t.Fatal("native movement did not finish")
					}
					stronger.Activate(sim)
					assertFloat64(t, "strongest passive wins without stacking", agent.GetMovementSpeed(), 8.4)
				})
				classic60PaladinAt(agent, 3002*time.Millisecond, func(_ *Simulation) {
					checks++
					assertFloat64(t, "Pursuit resumes after stronger passive expires", agent.GetMovementSpeed(), 7*(1+.04*float64(rank)))
				})
			})
			classic60PaladinResult(t, sim.run())
			if checks != 2 {
				t.Fatal("movement lifecycle did not run twice")
			}
		})
	}
}

func TestClassic60PaladinUnyieldingFaithControlEffects(t *testing.T) {
	for _, mechanic := range []classic60PaladinControlMechanic{classic60ControlFear, classic60ControlDisorient} {
		for rank := uint8(1); rank <= 2; rank++ {
			t.Run(fmt.Sprintf("mechanic%d/rank%d", mechanic, rank), func(t *testing.T) {
				checks := 0
				var heals []classic60PaladinHealEvent
				sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 6 * time.Second, iterations: 2, seed: 9}, func(agent *classic60PaladinTestAgent) {
					talents := classic60PaladinTalents{classic60PaladinUnyieldingFaith: rank}
					agent.spells.classic60PaladinDefenseState = agent.spells.registerDefense(&agent.Character, talents)
					agent.spells.registerHealingAbilities(&agent.Character, talents)
					agent.spells.registerMiscTalents(&agent.Character, talents)
					classic60PaladinObserveHealing(agent, &heals)
					incoming, aura := registerClassic60PaladinControlEffect(agent.CurrentTarget, &agent.Character, 900800+int32(mechanic), SpellSchoolShadow, 2*time.Second, mechanic)
					classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
						if aura.IsActive() || agent.PseudoStats.Incapacitated {
							t.Fatal("control leaked into next iteration")
						}
						if !agent.spells.HolyLight.Cast(sim, &agent.Unit) {
							t.Fatal("Holy Light failed before control")
						}
					})
					classic60PaladinAt(agent, 1500*time.Millisecond, func(sim *Simulation) {
						// A failed magic hit consumes no mechanic-resistance roll.
						classic60PaladinRolls(t, sim, []float64{.99}, func() { incoming.Cast(sim, &agent.Unit) })
						if aura.IsActive() {
							t.Fatal("missed NPC control landed")
						}
						classic60PaladinRolls(t, sim, []float64{.5, .05*float64(rank) - .001}, func() { incoming.Cast(sim, &agent.Unit) })
						if aura.IsActive() || agent.Hardcast.Expires != 3500*time.Millisecond {
							t.Fatal("Unyielding Faith failed to preserve active cast")
						}
					})
					classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
						classic60PaladinRolls(t, sim, []float64{.5, .05*float64(rank) + .001}, func() { incoming.Cast(sim, &agent.Unit) })
						if !aura.IsActive() || !agent.PseudoStats.Incapacitated || agent.Hardcast.Expires > sim.CurrentTime {
							t.Fatal("landed NPC control did not interrupt/incapacitate")
						}
						before := agent.CurrentMana()
						if agent.spells.SealOfCommand.CanCast(sim, &agent.Unit) || agent.spells.SealOfCommand.Cast(sim, &agent.Unit) {
							t.Fatal("incapacitated Paladin cast through control")
						}
						assertFloat64(t, "control prevents mana spending", agent.CurrentMana(), before)
					})
					classic60PaladinAt(agent, 4001*time.Millisecond, func(sim *Simulation) {
						checks++
						if aura.IsActive() || agent.PseudoStats.Incapacitated {
							t.Fatal("control did not expire")
						}
						if !agent.spells.FlashOfLight.Cast(sim, &agent.Unit) {
							t.Fatal("cast did not resume after control")
						}
					})
				})
				classic60PaladinResult(t, sim.run())
				if checks != 2 || len(heals) != 2 {
					t.Fatalf("expected only post-control heals in both iterations: checks%d heals%+v", checks, heals)
				}
				for _, event := range heals {
					if event.id != 19943 || event.at != 5501*time.Millisecond {
						t.Fatalf("unexpected heal lifecycle: %+v", event)
					}
				}
			})
		}
	}
}

func TestClassic60PaladinConcentrationControlEffects(t *testing.T) {
	for _, mechanic := range []classic60PaladinControlMechanic{classic60ControlSilence, classic60ControlInterrupt} {
		for rank := uint8(1); rank <= 3; rank++ {
			t.Run(fmt.Sprintf("mechanic%d/rank%d", mechanic, rank), func(t *testing.T) {
				var heals []classic60PaladinHealEvent
				sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 8 * time.Second, iterations: 2, seed: 33}, func(agent *classic60PaladinTestAgent) {
					talents := classic60PaladinTalents{classic60PaladinImprovedConcentrationAura: rank}
					agent.spells.classic60PaladinDefenseState = agent.spells.registerDefense(&agent.Character, talents)
					agent.spells.registerSelfBuffs(&agent.Character, talents)
					agent.spells.registerSupport(&agent.Character, talents)
					agent.spells.registerHealingAbilities(&agent.Character, talents)
					agent.spells.registerMiscTalents(&agent.Character, talents)
					if err := agent.spells.selectReferenceAura(agent.spells.ConcentrationEffect); err != nil {
						t.Fatal(err)
					}
					classic60PaladinObserveHealing(agent, &heals)
					incoming, aura := registerClassic60PaladinControlEffect(agent.CurrentTarget, &agent.Character, 900820+int32(mechanic), SpellSchoolArcane, 2*time.Second, mechanic)
					classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
						if aura.IsActive() {
							t.Fatal("silence/lockout leaked across iterations")
						}
						if !agent.spells.HolyLight.Cast(sim, &agent.Unit) {
							t.Fatal("Holy Light failed")
						}
					})
					classic60PaladinAt(agent, 1500*time.Millisecond, func(sim *Simulation) {
						classic60PaladinRolls(t, sim, []float64{.5, .05*float64(rank) - .001}, func() { incoming.Cast(sim, &agent.Unit) })
						if aura.IsActive() || agent.Hardcast.Expires != 3500*time.Millisecond {
							t.Fatal("Concentration resistance did not protect cast")
						}
					})
					classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
						classic60PaladinRolls(t, sim, []float64{.5, .05*float64(rank) + .001}, func() { incoming.Cast(sim, &agent.Unit) })
						if !aura.IsActive() || agent.Hardcast.Expires > sim.CurrentTime {
							t.Fatal("landed silence/interrupt did not cancel cast")
						}
						if agent.PseudoStats.Incapacitated {
							t.Fatal("silence/lockout incorrectly incapacitates melee")
						}
						before := agent.CurrentMana()
						if agent.spells.FlashOfLight.CanCast(sim, &agent.Unit) || agent.spells.FlashOfLight.Cast(sim, &agent.Unit) {
							t.Fatal("Holy cast bypassed silence/lockout")
						}
						assertFloat64(t, "blocked cast spends no mana", agent.CurrentMana(), before)
					})
					classic60PaladinAt(agent, 4001*time.Millisecond, func(sim *Simulation) {
						if aura.IsActive() || !agent.spells.FlashOfLight.Cast(sim, &agent.Unit) {
							t.Fatal("casting failed after control expiry")
						}
					})
					classic60PaladinAt(agent, 6*time.Second, func(sim *Simulation) {
						agent.spells.ConcentrationEffect.Deactivate(sim)
						if !agent.spells.HolyLight.Cast(sim, &agent.Unit) {
							t.Fatal("second Holy Light failed")
						}
						// No second RNG draw when the resistance aura is absent.
						classic60PaladinRolls(t, sim, []float64{.5}, func() { incoming.Cast(sim, &agent.Unit) })
						if !aura.IsActive() {
							t.Fatal("inactive Concentration still supplied resistance")
						}
					})
				})
				classic60PaladinResult(t, sim.run())
				if len(heals) != 2 {
					t.Fatalf("interrupted heals escaped: %+v", heals)
				}
				whiteAtThree := 0
				for _, event := range agent.events {
					if event.kind == "white" && event.at == 3*time.Second {
						whiteAtThree++
					}
				}
				if whiteAtThree != 2 {
					t.Fatalf("silence/lockout stopped white swings: %d", whiteAtThree)
				}
			})
		}
	}
}

func TestClassic60PaladinBubbleCleansesTypedControlsAndPreservesLockout(t *testing.T) {
	for _, protection := range []bool{false, true} {
		t.Run(fmt.Sprint(protection), func(t *testing.T) {
			checks := 0
			sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 5 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
				talents := classic60PaladinTalents{}
				agent.spells.classic60PaladinDefenseState = agent.spells.registerDefense(&agent.Character, talents)
				agent.spells.registerHealingAbilities(&agent.Character, talents)
				agent.spells.registerMiscTalents(&agent.Character, talents)
				agent.spells.registerDefensiveCooldowns(&agent.Character, talents)
				bubble := agent.spells.DivineShield
				if protection {
					bubble = agent.spells.DivineProtection
				}
				var incoming []*Spell
				var auras []*Aura
				for _, mechanic := range []classic60PaladinControlMechanic{classic60ControlFear, classic60ControlDisorient, classic60ControlSilence} {
					spell, aura := registerClassic60PaladinControlEffect(agent.CurrentTarget, &agent.Character, 900860+int32(mechanic), SpellSchoolShadow, 10*time.Second, mechanic)
					incoming, auras = append(incoming, spell), append(auras, aura)
				}
				interrupt, lockout := registerClassic60PaladinControlEffect(agent.CurrentTarget, &agent.Character, 900900, SpellSchoolArcane, 2*time.Second, classic60ControlInterrupt)
				classic60PaladinAt(agent, 500*time.Millisecond, func(sim *Simulation) {
					if !agent.spells.HolyLight.Cast(sim, &agent.Unit) {
						t.Fatal("pre-lockout cast failed")
					}
				})
				classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
					classic60PaladinRolls(t, sim, []float64{.5}, func() { interrupt.Cast(sim, &agent.Unit) })
					if !lockout.IsActive() {
						t.Fatal("interrupt failed to lock Holy school")
					}
					for _, spell := range incoming {
						classic60PaladinRolls(t, sim, []float64{.5}, func() { spell.Cast(sim, &agent.Unit) })
					}
				})
				classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
					before := agent.CurrentMana()
					if bubble.CanCast(sim, &agent.Unit) || bubble.Cast(sim, &agent.Unit) {
						t.Fatal("bubble bypassed Holy school lockout")
					}
					if !lockout.IsActive() || lockout.ExpiresAt() != 3*time.Second {
						t.Fatal("failed bubble cleared/shortened interrupt lockout")
					}
					assertFloat64(t, "failed bubble spends no mana", agent.CurrentMana(), before)
				})
				classic60PaladinAt(agent, 3100*time.Millisecond, func(sim *Simulation) {
					for _, aura := range auras {
						if !aura.IsActive() {
							t.Fatal("test control ended before bubble")
						}
					}
					if !bubble.CanCast(sim, &agent.Unit) || !bubble.Cast(sim, &agent.Unit) {
						t.Fatal("immunity-granting bubble could not escape typed control")
					}
					for _, aura := range auras {
						if aura.IsActive() {
							t.Fatal("bubble did not cleanse typed control")
						}
					}
					if agent.PseudoStats.Incapacitated {
						t.Fatal("bubble left stale incapacitated state")
					}
				})
				classic60PaladinAt(agent, 4*time.Second, func(sim *Simulation) {
					checks++
					for i, spell := range incoming {
						classic60PaladinRolls(t, sim, nil, func() { spell.Cast(sim, &agent.Unit) })
						if auras[i].IsActive() {
							t.Fatal("immune bubble accepted another control effect")
						}
					}
				})
			})
			classic60PaladinResult(t, sim.run())
			if checks != 2 {
				t.Fatal("bubble control lifecycle did not reset")
			}
		})
	}
}
