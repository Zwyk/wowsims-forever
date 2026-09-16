package core

import (
	"fmt"
	"testing"
	"time"
)

func TestClassic60PaladinFreedomCleansesAndPreventsRealMovementImpairment(t *testing.T) {
	talents := classic60PaladinTalents{classic60PaladinImprovedDevotionAura: 5, classic60PaladinGuardiansFavor: 2}
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 21 * time.Second, iterations: 2, pauseAutos: true, talents: &talents}, func(agent *classic60PaladinTestAgent) {
		slow, slowAura := registerClassic60PaladinMovementImpairment(agent.CurrentTarget, &agent.Character, 9991044, SpellSchoolFrost, 20*time.Second, .5)
		root, rootAura := registerClassic60PaladinMovementImpairment(agent.CurrentTarget, &agent.Character, 9991045, SpellSchoolNature, 20*time.Second, 0)
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if slowAura.IsActive() || rootAura.IsActive() || agent.spells.FreedomAura.IsActive() {
				t.Fatal("movement impairment leaked across iterations")
			}
			assertFloat64(t, "baseline movement speed", agent.GetMovementSpeed(), 7)
			agent.MoveTo(70, sim)
		})
		classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
			classic60PaladinRolls(t, sim, []float64{0}, func() { slow.Cast(sim, &agent.Unit) })
			assertFloat64(t, "position when snared", agent.DistanceFromTarget, 7)
			assertFloat64(t, "actual snared movement speed", agent.GetMovementSpeed(), 3.5)
		})
		classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
			classic60PaladinRolls(t, sim, []float64{0}, func() { root.Cast(sim, &agent.Unit) })
			assertFloat64(t, "root stops at the current position", agent.DistanceFromTarget, 10.5)
			assertFloat64(t, "rooted movement speed", agent.GetMovementSpeed(), 0)
			if agent.Moving {
				t.Fatal("root did not stop ongoing movement")
			}
			// A move command while rooted remembers the destination rather
			// than teleporting or dividing travel distance by zero.
			agent.MoveTo(70, sim)
		})
		classic60PaladinAt(agent, 4*time.Second, func(sim *Simulation) {
			assertFloat64(t, "no movement while rooted", agent.DistanceFromTarget, 10.5)
			before := agent.CurrentMana()
			if !agent.spells.BlessingOfFreedom.Cast(sim, &agent.Unit) {
				t.Fatal("Freedom failed while rooted")
			}
			assertFloat64(t, "Freedom costs ten percent base mana", before-agent.CurrentMana(), 151.2)
			if slowAura.IsActive() || rootAura.IsActive() || !agent.Moving {
				t.Fatal("Freedom did not cleanse both impairments and resume movement")
			}
			if agent.spells.FreedomAura.ExpiresAt() != 20*time.Second {
				t.Fatal("Guardian's Favor did not extend Freedom to sixteen seconds")
			}
		})
		classic60PaladinAt(agent, 5*time.Second, func(sim *Simulation) {
			agent.UpdatePosition(sim, false)
			assertFloat64(t, "Freedom resumes travel at ordinary speed", agent.DistanceFromTarget, 17.5)
			classic60PaladinRolls(t, sim, nil, func() { slow.Cast(sim, &agent.Unit); root.Cast(sim, &agent.Unit) })
			if slowAura.IsActive() || rootAura.IsActive() {
				t.Fatal("movement impairment bypassed Freedom")
			}
		})
		classic60PaladinAt(agent, 12501*time.Millisecond, func(_ *Simulation) {
			assertFloat64(t, "resumed movement reaches its original absolute destination", agent.DistanceFromTarget, 70)
			if agent.Moving {
				t.Fatal("movement did not finish at the correct destination")
			}
		})
		classic60PaladinAt(agent, 20100*time.Millisecond, func(sim *Simulation) {
			if agent.spells.FreedomAura.IsActive() {
				t.Fatal("Freedom outlived sixteen seconds")
			}
			classic60PaladinRolls(t, sim, []float64{0}, func() { root.Cast(sim, &agent.Unit) })
			if !rootAura.IsActive() {
				t.Fatal("expired Freedom continued preventing roots")
			}
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinGuardianFavorRanksAndFreedomBlessingExclusivity(t *testing.T) {
	for rank := uint8(0); rank <= 2; rank++ {
		t.Run(fmt.Sprint(rank), func(t *testing.T) {
			talents := classic60PaladinTalents{classic60PaladinImprovedDevotionAura: 5, classic60PaladinGuardiansFavor: rank}
			sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 6 * time.Second, pauseAutos: true, talents: &talents}, func(agent *classic60PaladinTestAgent) {
				classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
					if agent.spells.BlessingOfFreedom.CanCast(sim, agent.CurrentTarget) || !agent.spells.BlessingOfFreedom.Cast(sim, &agent.Unit) {
						t.Fatal("Freedom did not require its supported self target")
					}
				})
				classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
					if !agent.spells.BlessingOfMight.Cast(sim, &agent.Unit) || agent.spells.FreedomAura.IsActive() {
						t.Fatal("Might did not replace Freedom")
					}
				})
				classic60PaladinAt(agent, 5*time.Second, func(sim *Simulation) {
					if agent.spells.BlessingOfFreedom.Cast(sim, &agent.Unit) {
						t.Fatal("replacing Freedom removed its cooldown")
					}
				})
			})
			if agent.spells.FreedomAura.Duration != time.Duration(10+3*rank)*time.Second || agent.spells.BlessingOfProtection.CD.Duration != time.Duration(5-rank)*time.Minute {
				t.Fatal("Guardian's Favor did not affect both Freedom duration and Protection cooldown")
			}
			if agent.spells.BlessingOfFreedom.CD.Duration != 20*time.Second || agent.spells.BlessingOfFreedom.DefaultCast.GCD != 1500*time.Millisecond {
				t.Fatal("Freedom lost its twenty-second cooldown or normal GCD")
			}
			classic60PaladinResult(t, sim.run())
		})
	}
}

func TestClassic60PaladinRootExpiryResumesMovement(t *testing.T) {
	talents := classic60PaladinTalents{}
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 6 * time.Second, pauseAutos: true, talents: &talents}, func(agent *classic60PaladinTestAgent) {
		root, _ := registerClassic60PaladinMovementImpairment(agent.CurrentTarget, &agent.Character, 9991046, SpellSchoolFrost, time.Second, 0)
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) { agent.MoveTo(21, sim) })
		classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
			classic60PaladinRolls(t, sim, []float64{0}, func() { root.Cast(sim, &agent.Unit) })
		})
		classic60PaladinAt(agent, 4*time.Second, func(sim *Simulation) {
			agent.UpdatePosition(sim, false)
			assertFloat64(t, "root expiration resumes movement without Freedom", agent.DistanceFromTarget, 14)
		})
		classic60PaladinAt(agent, 5100*time.Millisecond, func(_ *Simulation) {
			assertFloat64(t, "root delay preserves the destination", agent.DistanceFromTarget, 21)
			if agent.Moving {
				t.Fatal("root delayed movement never completed")
			}
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinBubblesCleanseMovementImpairments(t *testing.T) {
	for _, shield := range []bool{false, true} {
		talents := classic60PaladinTalents{}
		sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 5 * time.Second, iterations: 2, pauseAutos: true, talents: &talents}, func(agent *classic60PaladinTestAgent) {
			slow, slowAura := registerClassic60PaladinMovementImpairment(agent.CurrentTarget, &agent.Character, 9991050, SpellSchoolFrost, 20*time.Second, .5)
			root, rootAura := registerClassic60PaladinMovementImpairment(agent.CurrentTarget, &agent.Character, 9991051, SpellSchoolNature, 20*time.Second, 0)
			classic60PaladinAt(agent, time.Second, func(sim *Simulation) { agent.MoveTo(70, sim) })
			classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
				classic60PaladinRolls(t, sim, []float64{0, 0}, func() { slow.Cast(sim, &agent.Unit); root.Cast(sim, &agent.Unit) })
			})
			classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
				spell := agent.spells.DivineProtection
				if shield {
					spell = agent.spells.DivineShield
				}
				if !spell.Cast(sim, &agent.Unit) {
					t.Fatal("bubble failed while movement impaired")
				}
				if slowAura.IsActive() || rootAura.IsActive() || !agent.Moving {
					t.Fatal("bubble did not cleanse root/snare and resume movement")
				}
			})
			classic60PaladinAt(agent, 4*time.Second, func(sim *Simulation) {
				agent.UpdatePosition(sim, false)
				assertFloat64(t, "bubble restored movement", agent.DistanceFromTarget, 14)
				classic60PaladinRolls(t, sim, nil, func() { slow.Cast(sim, &agent.Unit); root.Cast(sim, &agent.Unit) })
				if slowAura.IsActive() || rootAura.IsActive() {
					t.Fatal("movement impairment bypassed bubble immunity")
				}
			})
		})
		classic60PaladinResult(t, sim.run())
	}
}

func TestClassic60PaladinRootedMoveCanCancelHeldDestination(t *testing.T) {
	talents := classic60PaladinTalents{}
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 6 * time.Second, iterations: 2, pauseAutos: true, talents: &talents}, func(agent *classic60PaladinTestAgent) {
		root, rootAura := registerClassic60PaladinMovementImpairment(agent.CurrentTarget, &agent.Character, 9991052, SpellSchoolFrost, 2*time.Second, 0)
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) { agent.MoveTo(21, sim) })
		classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
			classic60PaladinRolls(t, sim, []float64{0}, func() { root.Cast(sim, &agent.Unit) })
			assertFloat64(t, "position when rooted", agent.DistanceFromTarget, 7)
		})
		classic60PaladinAt(agent, 2500*time.Millisecond, func(sim *Simulation) {
			// A request to stay here supersedes the old destination, even
			// though its distance matches the current rooted position.
			agent.MoveTo(agent.DistanceFromTarget, sim)
		})
		classic60PaladinAt(agent, 5*time.Second, func(sim *Simulation) {
			agent.UpdatePosition(sim, false)
			assertFloat64(t, "root expiration respects the replacement destination", agent.DistanceFromTarget, 7)
			if rootAura.IsActive() || agent.Moving || agent.spells.classic60PaladinMobility.resumeMove {
				t.Fatal("root expiry resumed a canceled destination")
			}
		})
	})
	classic60PaladinResult(t, sim.run())
}
