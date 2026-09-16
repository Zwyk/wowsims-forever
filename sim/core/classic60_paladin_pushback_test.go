package core

import (
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
)

func TestClassic60PaladinPushbackSequenceCompletionAndReset(t *testing.T) {
	var heals []classic60PaladinHealEvent
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 13 * time.Second, iterations: 2}, func(agent *classic60PaladinTestAgent) {
		agent.spells.registerHealingAbilities(&agent.Character, classic60PaladinTalents{})
		agent.spells.registerClassicPushback(&agent.Character, classic60PaladinTalents{})
		classic60PaladinObserveHealing(agent, &heals)
		incoming := &Spell{Unit: agent.CurrentTarget, ProcMask: ProcMaskMeleeMHAuto}
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if !agent.spells.HolyLight.Cast(sim, &agent.Unit) {
				t.Fatal("Holy Light failed")
			}
			if agent.Hardcast.classic60PushbackCount != 0 {
				t.Fatal("pushback count leaked across iterations")
			}
		})
		for i, elapsed := range []time.Duration{time.Second, 1800 * time.Millisecond, 2400 * time.Millisecond, 2800 * time.Millisecond, 3 * time.Second, 3200 * time.Millisecond} {
			classic60PaladinAt(agent, 1100*time.Millisecond+time.Duration(i)*100*time.Millisecond, func(sim *Simulation) {
				// Native hit-taken notification; incoming outcome policy is
				// covered separately by the defense integration tests.
				agent.OnSpellHitTaken(sim, incoming, &SpellResult{Target: &agent.Unit, Outcome: OutcomeHit, Damage: 1})
				want := 3500*time.Millisecond + elapsed
				if agent.Hardcast.Expires != want || agent.NextGCDAt() != want {
					t.Fatalf("pushback expiry/GCD %v/%v, want %v", agent.Hardcast.Expires, agent.NextGCDAt(), want)
				}
				if agent.AutoAttacks.mh.swingAt != want+3*time.Second {
					t.Fatal("pushback did not restart the melee swing after casting")
				}
			})
		}
		classic60PaladinAt(agent, 8*time.Second, func(sim *Simulation) {
			if !agent.spells.HolyLight.Cast(sim, &agent.Unit) {
				t.Fatal("second Holy Light failed")
			}
			if agent.Hardcast.classic60PushbackCount != 0 {
				t.Fatal("a new cast inherited the previous cast's pushback count")
			}
		})
		classic60PaladinAt(agent, 8100*time.Millisecond, func(sim *Simulation) {
			agent.OnSpellHitTaken(sim, incoming, &SpellResult{Target: &agent.Unit, Outcome: OutcomeHit, Damage: 1})
			if agent.Hardcast.Expires != 11500*time.Millisecond {
				t.Fatal("new cast did not restart one-second pushback")
			}
		})
	})
	classic60PaladinResult(t, sim.run())
	if len(heals) != 4 {
		t.Fatalf("hardcast completed %d times, want 4", len(heals))
	}
	for i, heal := range heals {
		want := []time.Duration{6700 * time.Millisecond, 11500 * time.Millisecond}[i%2]
		if heal.at != want {
			t.Fatalf("heal completed at %v, want %v", heal.at, want)
		}
	}
}

func TestClassic60PaladinPushbackPreventionAndEligibility(t *testing.T) {
	for _, test := range []struct {
		name                     string
		focus                    uint8
		concentration, holyWrath bool
		roll                     float64
		wantPushback             bool
	}{
		{"unprotected", 0, false, false, 0, true},
		{"focus_below_boundary", 5, false, false, .299, true},
		{"focus_above_boundary", 5, false, false, .301, false},
		{"concentration_below_boundary", 0, true, false, .649, true},
		{"concentration_above_boundary", 0, true, false, .651, false},
		{"additive_immunity", 5, true, false, 0, false},
		{"focus_excludes_damage_cast", 5, false, true, 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 5 * time.Second, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
				talents := classic60PaladinTalents{classic60PaladinSpiritualFocus: test.focus}
				agent.spells.registerOffensiveAbilities(&agent.Character, talents)
				agent.spells.registerHealingAbilities(&agent.Character, talents)
				agent.spells.registerSelfBuffs(&agent.Character, talents)
				agent.spells.registerSupport(&agent.Character, talents)
				agent.spells.registerClassicPushback(&agent.Character, talents)
				if test.concentration {
					if err := agent.spells.selectReferenceAura(agent.spells.ConcentrationEffect); err != nil {
						t.Fatal(err)
					}
				}
				cast, target := agent.spells.HolyLight, &agent.Unit
				if test.holyWrath {
					cast, target = agent.spells.HolyWrath, agent.CurrentTarget
					target.MobType = proto.MobType_MobTypeUndead
				}
				classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
					if !cast.Cast(sim, target) {
						t.Fatalf("cast failed: %v", cast.ActionID)
					}
				})
				classic60PaladinAt(agent, 1200*time.Millisecond, func(sim *Simulation) {
					before := agent.Hardcast.Expires
					for _, event := range []struct {
						mask    ProcMask
						outcome HitOutcome
						damage  float64
					}{
						{ProcMaskMeleeMHAuto, OutcomeMiss, 10}, {ProcMaskMeleeMHAuto, OutcomeDodge, 10},
						{ProcMaskMeleeMHAuto, OutcomeHit, 0}, {ProcMaskEmpty, OutcomeHit, 10},
					} {
						agent.OnSpellHitTaken(sim, &Spell{Unit: agent.CurrentTarget, ProcMask: event.mask}, &SpellResult{Target: &agent.Unit, Outcome: event.outcome, Damage: event.damage})
					}
					if agent.Hardcast.Expires != before {
						t.Fatal("non-damaging, missed or non-direct event pushed back the cast")
					}
					rolls := []float64{test.roll}
					if test.name == "unprotected" || test.name == "additive_immunity" || test.holyWrath {
						rolls = nil
					}
					classic60PaladinRolls(t, sim, rolls, func() {
						agent.OnSpellHitTaken(sim, &Spell{Unit: agent.CurrentTarget, ProcMask: ProcMaskMeleeMHAuto}, &SpellResult{Target: &agent.Unit, Outcome: OutcomeHit, Damage: 10})
					})
					want := before
					if test.wantPushback {
						want += time.Second
					}
					if agent.Hardcast.Expires != want {
						t.Fatalf("pushback prevention gave expiry %v, want %v", agent.Hardcast.Expires, want)
					}
				})
			})
			classic60PaladinResult(t, sim.run())
		})
	}
}
