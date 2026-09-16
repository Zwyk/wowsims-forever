package core

import (
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

type classic60PaladinAbilityEvent struct {
	spellID, target int32
	at              time.Duration
	damage          float64
	outcome         HitOutcome
}

func classic60PaladinObserveAbilities(agent *classic60PaladinTestAgent, events *[]classic60PaladinAbilityEvent) {
	record := func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
		if spell == agent.spells.Consecration || spell == agent.spells.Exorcism || spell == agent.spells.HolyWrath || spell == agent.spells.HolyShock {
			*events = append(*events, classic60PaladinAbilityEvent{spell.SpellID, result.Target.UnitIndex, sim.CurrentTime, result.Damage, result.Outcome})
		}
	}
	agent.RegisterAura(Aura{Label: "Observe Classic offensive abilities", Duration: NeverExpires,
		OnReset:         func(aura *Aura, sim *Simulation) { aura.Activate(sim) },
		OnSpellHitDealt: record, OnPeriodicDamageDealt: record,
	})
}

// Add a real initialized target before spell registration/finalization. Unit
// indices need not match target indices; keeping the original indices stable
// also exercises the normal per-unit spell metrics and attack-table lookup.
func classic60PaladinAddAbilityTarget(agent *classic60PaladinTestAgent, mobType proto.MobType) *Unit {
	env := agent.Env
	config := &proto.Target{Level: agent.CurrentTarget.Level, MobType: mobType}
	target := newTargetWithRuleset(config, int32(len(env.Encounter.AllTargets)), agent.resolvedRuleset())
	target.Env, target.UnitIndex = env, int32(len(env.AllUnits))
	target.stats[stats.BlockValue] = 0
	env.AllUnits = append(env.AllUnits, &target.Unit)
	env.Encounter.AllTargets = append(env.Encounter.AllTargets, target)
	env.Encounter.ActiveTargets = append(env.Encounter.ActiveTargets, target)
	env.Encounter.AllTargetUnits = append(env.Encounter.AllTargetUnits, &target.Unit)
	env.Encounter.ActiveTargetUnits = append(env.Encounter.ActiveTargetUnits, &target.Unit)
	env.Encounter.updateAOECapMultiplier()
	target.initialize(config)
	target.finalize()
	return &target.Unit
}

func TestClassic60PaladinOffensiveAbilityRanksAndTalents(t *testing.T) {
	for _, learned := range []bool{false, true} {
		t.Run(map[bool]string{false: "unlearned", true: "learned"}[learned], func(t *testing.T) {
			_, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: time.Second}, func(agent *classic60PaladinTestAgent) {
				talents := classic60PaladinTalents{classic60PaladinHolyPower: 5}
				if learned {
					talents[classic60PaladinConsecration], talents[classic60PaladinHolyShock] = 1, 1
				}
				agent.spells.registerOffensiveAbilities(&agent.Character, talents)
			})
			if (agent.spells.Consecration != nil) != learned || (agent.spells.HolyShock != nil) != learned {
				t.Fatal("talent-gated spells did not match learned ranks")
			}
			for _, expected := range []struct {
				spell    *Spell
				id       int32
				rank     int32
				cost     float64
				cd, cast time.Duration
				binary   bool
			}{
				{agent.spells.Exorcism, 10314, 6, 345, 15 * time.Second, 0, true},
				{agent.spells.HolyWrath, 10318, 2, 805, time.Minute, 2 * time.Second, false},
				{agent.spells.Consecration, 20924, 5, 565, 8 * time.Second, 0, false},
				{agent.spells.HolyShock, 20930, 3, 325, 30 * time.Second, 0, false},
			} {
				if expected.spell == nil {
					continue
				}
				spell := expected.spell
				if spell.SpellID != expected.id || spell.Rank != expected.rank || spell.CD.Duration != expected.cd ||
					spell.DefaultCast.CastTime != expected.cast || spell.DefaultCast.GCD != GCDDefault ||
					spell.Flags.Matches(SpellFlagBinary) != expected.binary || !spell.IgnoreHaste {
					t.Fatalf("incorrect rank/cast configuration for %v", spell.ActionID)
				}
				assertFloat64(t, "Classic ability cost", spell.Cost.GetCurrentCost(), expected.cost)
				if spell != agent.spells.Consecration {
					assertFloat64(t, "Holy Power applies to magic crit", spell.BonusCritPercent, 5)
				}
			}
			assertFloat64(t, "Holy Power excludes Holy melee crit", agent.spells.CommandJudgement.BonusCritPercent, 0)
		})
	}
}

func TestClassic60PaladinExorcismRestrictionAndHolyShockDamage(t *testing.T) {
	var events []classic60PaladinAbilityEvent
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 6 * time.Second, targetLevel: 60, spellDamage: 100, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		agent.spells.registerOffensiveAbilities(&agent.Character, classic60PaladinTalents{classic60PaladinHolyShock: 1})
		classic60PaladinObserveAbilities(agent, &events)
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			agent.CurrentTarget.MobType = proto.MobType_MobTypeHumanoid
			if agent.spells.Exorcism.Cast(sim, agent.CurrentTarget) || agent.CurrentMana() != agent.MaxMana() || !agent.GCD.IsReady(sim) {
				t.Fatal("invalid Exorcism target spent mana or consumed the GCD")
			}
			agent.CurrentTarget.MobType = proto.MobType_MobTypeUndead
			classic60PaladinRolls(t, sim, []float64{.5, .5, .999}, func() {
				if !agent.spells.Exorcism.Cast(sim, agent.CurrentTarget) {
					t.Fatal("Exorcism rejected undead")
				}
			})
			if agent.spells.Exorcism.CD.ReadyAt() != 16*time.Second {
				t.Fatal("Exorcism cooldown did not start at cast")
			}
		})
		classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
			agent.CurrentTarget.MobType = proto.MobType_MobTypeDemon
			before := agent.CurrentMana()
			if agent.spells.Exorcism.Cast(sim, agent.CurrentTarget) || agent.CurrentMana() != before {
				t.Fatal("Exorcism bypassed cooldown")
			}
			agent.CurrentTarget.MobType = proto.MobType_MobTypeHumanoid
			classic60PaladinRolls(t, sim, []float64{.5, .999, .5, .999}, func() {
				if !agent.spells.HolyShock.Cast(sim, agent.CurrentTarget) {
					t.Fatal("offensive Holy Shock rejected humanoid")
				}
			})
		})
	})
	classic60PaladinResult(t, sim.run())
	if len(events) != 2 {
		t.Fatalf("got %d offensive results, want 2", len(events))
	}
	assertFloat64(t, "Exorcism base damage and SP", events[0].damage, 534+42.9)
	assertFloat64(t, "Holy Shock base damage and SP", events[1].damage, 380+42.9)
	if events[0].outcome != OutcomeHit || events[1].outcome != OutcomeHit {
		t.Fatal("controlled noncrit magic hits changed")
	}
}

func TestClassic60PaladinConsecrationSnapshotTicksAndActiveTargets(t *testing.T) {
	var events []classic60PaladinAbilityEvent
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 10 * time.Second, targetLevel: 60, spellDamage: 400, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		classic60PaladinAddAbilityTarget(agent, proto.MobType_MobTypeHumanoid)
		agent.spells.registerOffensiveAbilities(&agent.Character, classic60PaladinTalents{classic60PaladinConsecration: 1})
		classic60PaladinObserveAbilities(agent, &events)
		dot := agent.spells.Consecration.AOEDot()
		tick := dot.onTick
		dot.onTick = func(sim *Simulation, target *Unit, dot *Dot) {
			rolls := []float64{.999, .5} // first target lands
			if len(sim.Encounter.ActiveTargetUnits) == 2 {
				rolls = append(rolls, .999, .999)
			} // second misses independently
			classic60PaladinRolls(t, sim, rolls, func() { tick(sim, target, dot) })
		}
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if !agent.spells.Consecration.Cast(sim, agent.CurrentTarget) {
				t.Fatal("Consecration failed")
			}
		})
		classic60PaladinAt(agent, 5500*time.Millisecond, func(sim *Simulation) {
			agent.AddStatDynamic(sim, stats.SpellDamage, 400)
			agent.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly] = 2
			sim.Encounter.ActiveTargetUnits = sim.Encounter.ActiveTargetUnits[:1]
		})
	})
	classic60PaladinResult(t, sim.run())
	if len(events) != 12 {
		t.Fatalf("want eight primary and four secondary ticks, got %d", len(events))
	}
	primary, misses := 0, 0
	for _, event := range events {
		if event.target == 0 {
			primary++
			assertFloat64(t, "Consecration snapshots SP and attacker multiplier", event.damage, 64.8)
			if event.outcome != OutcomeHit || event.at != time.Duration(primary+1)*time.Second {
				t.Fatalf("unexpected primary tick: %+v", event)
			}
		} else {
			misses++
			if event.outcome != OutcomeMiss || event.damage != 0 || event.at > 5*time.Second {
				t.Fatalf("inactive target or independent hit failed: %+v", event)
			}
		}
	}
	if primary != 8 || misses != 4 {
		t.Fatalf("wrong tick/miss counts: %d/%d", primary, misses)
	}
}

func TestClassic60PaladinHolyWrathCastAndSimultaneousAOE(t *testing.T) {
	var events []classic60PaladinAbilityEvent
	sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 8 * time.Second, targetLevel: 60, spellDamage: 100}, func(agent *classic60PaladinTestAgent) {
		agent.CurrentTarget.MobType = proto.MobType_MobTypeDemon
		classic60PaladinAddAbilityTarget(agent, proto.MobType_MobTypeUndead)
		classic60PaladinAddAbilityTarget(agent, proto.MobType_MobTypeHumanoid)
		agent.spells.registerOffensiveAbilities(&agent.Character, classic60PaladinTalents{})
		classic60PaladinObserveAbilities(agent, &events)
		effects := agent.spells.HolyWrath.ApplyEffects
		agent.spells.HolyWrath.ApplyEffects = func(sim *Simulation, target *Unit, spell *Spell) {
			classic60PaladinRolls(t, sim, []float64{.5, .999, .5, 0, .5, .999, .5, 0}, func() { effects(sim, target, spell) })
		}
		agent.RegisterAura(Aura{Label: "Holy Wrath multiplier changes after first result", Duration: NeverExpires,
			OnReset: func(aura *Aura, sim *Simulation) { aura.Activate(sim) },
			OnSpellHitDealt: func(_ *Aura, _ *Simulation, spell *Spell, _ *SpellResult) {
				if spell == agent.spells.HolyWrath {
					agent.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly] = 2
				}
			},
		})
		classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
			if !agent.spells.HolyWrath.Cast(sim, agent.CurrentTarget) {
				t.Fatal("Holy Wrath failed")
			}
			assertFloat64(t, "hardcast does not charge at start", agent.CurrentMana(), agent.MaxMana())
		})
		classic60PaladinAt(agent, 4*time.Second+time.Millisecond, func(sim *Simulation) {
			assertFloat64(t, "Holy Wrath cost at completion", agent.CurrentMana(), agent.MaxMana()-805)
			if agent.spells.HolyWrath.CD.ReadyAt() != 64*time.Second || agent.PseudoStats.FiveSecondRuleRefreshTime != 9*time.Second {
				t.Fatal("Holy Wrath completion cooldown/FSR changed")
			}
		})
	})
	classic60PaladinResult(t, sim.run())
	if len(events) != 2 {
		t.Fatalf("Holy Wrath hit %d targets, want undead+demon", len(events))
	}
	for _, event := range events {
		assertFloat64(t, "Holy Wrath resolves all targets before proc delivery", event.damage, (533+19)*1.5)
		if event.at != 4*time.Second || event.outcome != OutcomeCrit {
			t.Fatalf("unexpected Holy Wrath result: %+v", event)
		}
	}
	var swings []time.Duration
	for _, event := range agent.events {
		if event.kind == "white" {
			swings = append(swings, event.at)
		}
	}
	if len(swings) != 3 || swings[0] != 0 || swings[1] != 4*time.Second || swings[2] != 7*time.Second {
		t.Fatalf("Holy Wrath did not pause due swings through hardcast: %v", swings)
	}
}

func TestClassic60PaladinHolyWrathInterruptAndFailedCost(t *testing.T) {
	for _, insufficientMana := range []bool{false, true} {
		t.Run(map[bool]string{false: "interrupted", true: "insufficient_mana"}[insufficientMana], func(t *testing.T) {
			var events []classic60PaladinAbilityEvent
			config := classic60PaladinTestConfig{duration: 7 * time.Second, targetLevel: 60}
			if insufficientMana {
				config.startingMana = 100
			}
			sim, agent := newClassic60PaladinTestSim(t, config, func(agent *classic60PaladinTestAgent) {
				agent.CurrentTarget.MobType = proto.MobType_MobTypeUndead
				agent.spells.registerOffensiveAbilities(&agent.Character, classic60PaladinTalents{})
				classic60PaladinObserveAbilities(agent, &events)
				classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
					if agent.spells.HolyWrath.Cast(sim, agent.CurrentTarget) == insufficientMana {
						t.Fatal("Holy Wrath cast ignored resource state")
					}
				})
				classic60PaladinAt(agent, 2500*time.Millisecond, func(sim *Simulation) {
					if !insufficientMana {
						agent.Interrupt(sim)
					}
				})
			})
			classic60PaladinResult(t, sim.run())
			if len(events) != 0 {
				t.Fatal("failed/interrupted Holy Wrath dealt damage")
			}
			cost := agent.spells.HolyWrath.Cost.ResourceCostImpl.(*ManaCost).ResourceMetrics
			if cost.Events != 0 || agent.PseudoStats.FiveSecondRuleRefreshTime != 0 {
				t.Fatal("failed/interrupted Holy Wrath charged mana")
			}
			var swings []time.Duration
			for _, event := range agent.events {
				if event.kind == "white" {
					swings = append(swings, event.at)
				}
			}
			if len(swings) != 3 || swings[0] != 0 || swings[1] != 3*time.Second || swings[2] != 6*time.Second {
				t.Fatalf("failed/interrupted cast changed later auto timing: %v", swings)
			}
		})
	}
}

func TestClassic60PaladinHolyWrathConsecutiveCastBoundaries(t *testing.T) {
	var events []classic60PaladinAbilityEvent
	var completions []time.Duration
	sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{
		duration: 7 * time.Second, targetLevel: 60, iterations: 2,
	}, func(agent *classic60PaladinTestAgent) {
		agent.CurrentTarget.MobType = proto.MobType_MobTypeUndead
		agent.spells.registerOffensiveAbilities(&agent.Character, classic60PaladinTalents{})
		classic60PaladinObserveAbilities(agent, &events)
		wrath := agent.spells.HolyWrath
		effects := wrath.ApplyEffects
		wrath.ApplyEffects = func(sim *Simulation, target *Unit, spell *Spell) {
			classic60PaladinRolls(t, sim, []float64{.5, .999, .5, .999}, func() { effects(sim, target, spell) })
		}
		completedThisIteration := 0
		agent.RegisterAura(Aura{
			Label: "Chain Holy Wrath at its completion boundary", Duration: NeverExpires,
			OnReset: func(aura *Aura, sim *Simulation) {
				completedThisIteration = 0
				aura.Activate(sim)
			},
			OnCastComplete: func(_ *Aura, sim *Simulation, spell *Spell) {
				if spell != wrath {
					return
				}
				completedThisIteration++
				completions = append(completions, sim.CurrentTime)
				assertFloat64(t, "completed Holy Wrath charges exactly once", agent.CurrentMana(), agent.MaxMana()-805*float64(completedThisIteration))
				if completedThisIteration == 1 {
					// An immediate recast deliberately exercises ownership of the
					// first cast's queued completion action. The production spell's
					// cooldown alone is bypassed for this scheduling regression.
					wrath.CD.Reset()
					if !wrath.Cast(sim, agent.CurrentTarget) {
						t.Fatal("completion callback could not start the next Holy Wrath")
					}
				}
			},
		})
		classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
			if !wrath.Cast(sim, agent.CurrentTarget) {
				t.Fatal("initial Holy Wrath failed")
			}
		})
		classic60PaladinAt(agent, 5*time.Second, func(_ *Simulation) {
			if completedThisIteration != 1 || agent.Hardcast.Expires != 6*time.Second {
				t.Fatal("the old completion action finished or replaced the second hardcast early")
			}
			assertFloat64(t, "in-progress second Holy Wrath has not spent mana", agent.CurrentMana(), agent.MaxMana()-805)
		})
	})
	classic60PaladinResult(t, sim.run())
	if len(completions) != 4 || len(events) != 4 {
		t.Fatalf("two iterations should each complete/deal exactly twice: completions=%v, results=%+v", completions, events)
	}
	for index, at := range completions {
		want := time.Duration(4+2*(index%2)) * time.Second
		if at != want || events[index].at != want || events[index].outcome != OutcomeHit {
			t.Fatalf("cast %d completed or dealt damage outside its boundary: completion=%v result=%+v", index, at, events[index])
		}
		assertFloat64(t, "each Holy Wrath deals one complete result", events[index].damage, 533)
	}
	var swings []time.Duration
	for _, event := range agent.events {
		if event.kind == "white" {
			swings = append(swings, event.at)
		}
	}
	if len(swings) != 4 || swings[0] != 0 || swings[1] != 6*time.Second || swings[2] != 0 || swings[3] != 6*time.Second {
		t.Fatalf("due auto-attacks should wait for both consecutive casts in each iteration: %v", swings)
	}
}

func TestClassic60PaladinHolyWrathCompletesBeforeBoundaryAPLSpending(t *testing.T) {
	for _, pauseAutos := range []bool{false, true} {
		t.Run(map[bool]string{false: "auto_attack_wakeup", true: "rotation_wakeup"}[pauseAutos], func(t *testing.T) {
			var events []classic60PaladinAbilityEvent
			sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{
				duration: 5 * time.Second, targetLevel: 60, iterations: 2,
				startingMana: 805, pauseAutos: pauseAutos,
			}, func(agent *classic60PaladinTestAgent) {
				agent.CurrentTarget.MobType = proto.MobType_MobTypeUndead
				agent.spells.registerOffensiveAbilities(&agent.Character, classic60PaladinTalents{})
				classic60PaladinObserveAbilities(agent, &events)
				effects := agent.spells.HolyWrath.ApplyEffects
				agent.spells.HolyWrath.ApplyEffects = func(sim *Simulation, target *Unit, spell *Spell) {
					classic60PaladinRolls(t, sim, []float64{.5, .999, .5, .999}, func() { effects(sim, target, spell) })
				}
				// The seal becomes available exactly when Holy Wrath is due.
				// There is enough mana for either spell but not for both. A
				// wakeup that spends on the seal first would cancel the nuke.
				agent.spells.SealOfCommand.ExtraCastCondition = func(sim *Simulation, _ *Unit) bool {
					return sim.CurrentTime >= 4*time.Second
				}
				classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
					if !agent.spells.HolyWrath.Cast(sim, agent.CurrentTarget) {
						t.Fatal("initial Holy Wrath failed")
					}
				})
			})
			agent.Rotation = agent.newAPLRotation(&proto.APLRotation{
				Type: proto.APLRotation_TypeAPL,
				PriorityList: []*proto.APLListItem{{Action: &proto.APLAction{
					Action: &proto.APLAction_CastSpell{CastSpell: &proto.APLActionCastSpell{
						SpellId: agent.spells.SealOfCommand.ActionID.ToProto(),
					}},
				}}},
			})
			if len(agent.Rotation.priorityList) != 1 {
				t.Fatal("boundary-spending APL did not resolve its seal")
			}
			classic60PaladinResult(t, sim.run())
			if len(events) != 2 {
				t.Fatalf("each iteration should finish its due Holy Wrath: %+v", events)
			}
			for _, event := range events {
				if event.at != 4*time.Second || event.spellID != 10318 || event.outcome != OutcomeHit {
					t.Fatalf("due Holy Wrath was delayed or failed: %+v", event)
				}
			}
			for _, event := range agent.events {
				if event.kind == "seal" {
					t.Fatal("boundary APL spent mana needed by the completing Holy Wrath")
				}
			}
		})
	}
}
