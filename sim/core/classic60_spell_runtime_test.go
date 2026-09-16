package core

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/simsignals"
	"github.com/wowsims/tbc/sim/core/stats"
)

type classic60SpellTestConfig struct {
	level                                   int32
	resistance, penetration, hit, bonusCrit float64
	binary, pureDot                         bool
	duration                                time.Duration
	iterations                              int32
	seed                                    int64
}

type classic60SpellTestEvent struct {
	seed               int64
	at                 time.Duration
	periodic           bool
	outcome            HitOutcome
	damage, mitigation float64
}

// A pre-racial Human Mage supplies integral primary stats and the characterized
// spell-crit dependency. No Mage constructor, mana bar, regeneration, talents,
// equipment, haste or real spell ranks are imported. Resource-free diagnostic
// spells still use the actual hardcast, GCD, cooldown, Dot, reset and report paths.
func newClassic60SpellTestSim(t *testing.T, config classic60SpellTestConfig) (*Simulation, *FakeAgent, *[]classic60SpellTestEvent) {
	t.Helper()
	rules := classic60SpellReferenceRules()
	baseline, ok := classicReferenceInitializeCharacter(60, proto.Race_RaceHuman, proto.Class_ClassMage)
	if !ok {
		t.Fatal("missing Human Mage baseline")
	}
	raid := NewRaid(&proto.Raid{})
	party := NewParty(raid, 0, &proto.Party{}, &proto.Raid{})
	raid.Parties = []*Party{party}
	agent := &FakeAgent{Character: newCharacterWithRuleset(party, 0, &proto.Player{
		Name: "Classic spell fixture", Race: proto.Race_RaceHuman, Class: proto.Class_ClassMage,
		Spec: &proto.Player_Mage{}, Equipment: &proto.EquipmentSpec{},
	}, rules, baseline.baseStats)}
	agent.AddBaseClassStatDependencies()
	agent.addManaStatDependenciesWithRuleset(rules)
	agent.AddStats(stats.Stats{stats.SpellHitPercent: config.hit, stats.SpellPenetration: config.penetration})
	party.Players = []Agent{agent}
	raid.updatePlayersAndPets()
	targetConfig := &proto.Target{Level: config.level, Stats: stats.Stats{stats.FireResistance: config.resistance}.ToProtoArray()}
	target := newTargetWithRuleset(targetConfig, 0, rules)
	target.stats[stats.BlockValue] = 0
	env := &Environment{
		State: Constructed, Raid: raid, BaseDuration: config.duration,
		Encounter: Encounter{
			Duration: config.duration, AllTargets: []*Target{target}, ActiveTargets: []*Target{target},
			AllTargetUnits: []*Unit{&target.Unit}, ActiveTargetUnits: []*Unit{&target.Unit},
		},
		AllUnits: []*Unit{&target.Unit, &agent.Unit},
	}
	env.Encounter.updateAOECapMultiplier()
	for index, unit := range env.AllUnits {
		unit.Env, unit.UnitIndex = env, int32(index)
	}
	agent.CurrentTarget = &target.Unit
	target.initialize(targetConfig)
	agent.initialize(agent)

	period, castTime := 3*time.Second, 1500*time.Millisecond
	flags := SpellFlagNoOnCastComplete
	if config.binary {
		flags |= SpellFlagBinary
	}
	if config.pureDot {
		flags |= SpellFlagPureDot
		period, castTime = 8*time.Second, 0
	}
	spellConfig := SpellConfig{
		ActionID:    ActionID{OtherID: proto.OtherAction_OtherActionAttack, Tag: 201},
		SpellSchool: SpellSchoolFire, DefenseType: DefenseTypeMagic,
		ProcMask: ProcMaskSpellDamage, WeaponAttackSource: WeaponAttackSourceNone,
		Flags: flags, BonusCritPercent: config.bonusCrit,
		DamageMultiplier: 1, DamageMultiplierAdditive: 1, ThreatMultiplier: 1,
		Cast: CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: 1500 * time.Millisecond, CastTime: castTime},
			// The engine starts a hardcast's cooldown on completion.
			CD: Cooldown{Timer: agent.NewTimer(), Duration: period - castTime}},
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			spell.CalcAndDealDamage(sim, target, 100, spell.OutcomeMagicHitAndCrit)
		},
	}
	if config.pureDot {
		spellConfig.Dot = DotConfig{
			Aura: Aura{Label: "Classic diagnostic pure dot"}, NumberOfTicks: 3, TickLength: 2 * time.Second,
			OnSnapshot: func(_ *Simulation, target *Unit, dot *Dot) { dot.Snapshot(target, 100) },
			OnTick: func(sim *Simulation, target *Unit, dot *Dot) {
				dot.CalcAndDealPeriodicSnapshotDamage(sim, target, dot.OutcomeTick)
			},
		}
		spellConfig.ApplyEffects = func(sim *Simulation, target *Unit, spell *Spell) {
			result := spell.CalcOutcome(sim, target, spell.OutcomeMagicHit)
			if result.Landed() {
				spell.Dot(target).Apply(sim)
			}
			spell.DealOutcome(sim, result)
		}
	}
	agent.Spell = agent.RegisterSpell(spellConfig)
	events := []classic60SpellTestEvent{}
	record := func(sim *Simulation, spell *Spell, result *SpellResult, periodic bool) {
		if spell == agent.Spell {
			events = append(events, classic60SpellTestEvent{
				seed: sim.currentSeed, at: sim.CurrentTime, periodic: periodic,
				outcome: result.Outcome, damage: result.Damage, mitigation: result.ArmorAndResistanceMultiplier,
			})
		}
	}
	agent.RegisterAura(Aura{
		Label: "Schedule and record Classic diagnostic casts", Duration: NeverExpires,
		OnReset: func(aura *Aura, sim *Simulation) {
			aura.Activate(sim)
			StartPeriodicAction(sim, PeriodicActionOptions{Period: period, TickImmediately: true,
				OnAction: func(sim *Simulation) {
					// Do not start a cast that would finish after the fight.
					if sim.CurrentTime+castTime > sim.Duration {
						return
					}
					if !agent.Spell.CanCast(sim, agent.CurrentTarget) || !agent.Spell.Cast(sim, agent.CurrentTarget) {
						t.Fatal("scheduled diagnostic spell was not ready")
					}
					if agent.Spell.CanCast(sim, agent.CurrentTarget) || agent.Spell.Cast(sim, agent.CurrentTarget) {
						t.Fatal("diagnostic spell bypassed its cast/GCD/cooldown lock")
					}
				},
			})
		},
		OnSpellHitDealt:       func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) { record(sim, spell, result, false) },
		OnPeriodicDamageDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) { record(sim, spell, result, true) },
	})
	env.State = Initialized
	target.finalize()
	agent.Finalize()
	env.setupAttackTables()
	for i := 0; i < len(env.postFinalizeEffects); i++ {
		env.postFinalizeEffects[i]()
	}
	env.postFinalizeEffects = nil
	env.State = Finalized
	wantStats := baseline.stats
	wantStats[stats.SpellHitPercent], wantStats[stats.SpellPenetration] = config.hit, config.penetration
	if agent.GetStats() != wantStats || agent.HasManaBar() || agent.AutoAttacks.AutoSwingMelee {
		t.Fatal("caster acquired stats, resources or auto-attacks outside its Classic baseline")
	}
	for _, unit := range env.AllUnits {
		if unit.resolvedRuleset() != rules || math.IsNaN(unit.PseudoStats.ReducedCritTakenPercent) {
			t.Fatal("caster fixture lost explicit rules or acquired invalid defense modifiers")
		}
	}
	return newSimWithEnv(env, &proto.SimOptions{Iterations: config.iterations, RandomSeed: config.seed, IsTest: true}, simsignals.CreateSignals()), agent, &events
}

func classic60SpellTestMetrics(t *testing.T, result *proto.RaidSimResult) *proto.TargetedActionMetrics {
	t.Helper()
	if result.Error != nil {
		t.Fatalf("spell simulation failed: %v", result.Error)
	}
	for _, action := range result.RaidMetrics.Parties[0].Players[0].Actions {
		if action.Id.Tag == 201 {
			for _, target := range action.Targets {
				if target.UnitIndex == 0 {
					return target
				}
			}
		}
	}
	t.Fatal("no diagnostic spell metrics")
	return nil
}

func TestClassic60SpellRuntimeHardcastsMetricsAndReset(t *testing.T) {
	for _, binary := range []bool{false, true} {
		t.Run(map[bool]string{false: "partial", true: "binary"}[binary], func(t *testing.T) {
			active := currentRuleset()
			config := classic60SpellTestConfig{level: 63, resistance: 100, hit: 10, bonusCrit: 20, binary: binary,
				// End with the final cast's cooldown still pending at 30s.
				duration: 29 * time.Second, iterations: 3, seed: 1364}
			sim, agent, events := newClassic60SpellTestSim(t, config)
			initialStats := agent.GetStats()
			result := sim.run()
			metrics := classic60SpellTestMetrics(t, result)
			if metrics.Casts != 30 || len(*events) != 30 || result.IterationsDone != 3 {
				t.Fatalf("bad cast count: %v, events=%d", metrics, len(*events))
			}
			damage := 0.0
			for i, event := range *events {
				if event.periodic || event.at != 1500*time.Millisecond+time.Duration(i%10)*3*time.Second || event.seed != config.seed+int64(i/10) {
					t.Fatalf("hardcast completion/iteration did not reset: %+v", event)
				}
				wantDamage := 100 * event.mitigation
				switch event.outcome &^ OutcomePartial {
				case OutcomeMiss:
					wantDamage = 0
				case OutcomeCrit:
					wantDamage *= 1.5
				case OutcomeHit:
				default:
					t.Fatalf("unsupported spell outcome: %+v", event)
				}
				if binary && event.mitigation != 1 {
					t.Fatalf("binary cast partially resisted: %+v", event)
				}
				if !binary && event.mitigation != 1 && event.mitigation != .75 && event.mitigation != .5 && event.mitigation != .25 {
					t.Fatalf("invalid resistance bucket: %+v", event)
				}
				assertFloat64(t, "spell damage after mitigation and crit", event.damage, wantDamage)
				damage += event.damage
			}
			assertFloat64(t, "reported spell damage", metrics.Damage, damage)
			assertFloat64(t, "reported caster DPS", result.RaidMetrics.Dps.Avg, damage/87)
			if metrics.Hits+metrics.Crits+metrics.Misses != metrics.Casts || metrics.Dodges+metrics.Parries+metrics.Glances != 0 {
				t.Fatalf("invalid spell report: %+v", metrics)
			}
			if agent.GetStats() != initialStats || currentRuleset() != active || CharacterLevel != 70 {
				t.Fatal("caster changed its baseline or active profile")
			}
			repeat, _, repeatedEvents := newClassic60SpellTestSim(t, config)
			classic60MeleeTestAssertResultEqual(t, result, repeat.run())
			if !reflect.DeepEqual(*events, *repeatedEvents) {
				t.Fatal("spell trace changed with identical seed")
			}
		})
	}
}

func TestClassic60SpellRuntimeDistributionsAndPenetration(t *testing.T) {
	// Independent literal vectors from Classic@7779ebb. Human Mage crit is
	// .2 + 125*.0168 = 2.3%, with no executable boss suppression. +20 bonus
	// gives 22.3% conditional crit. Binary Hit adds AFTER resistance; direct
	// partial resistance includes the unpenetrable +3 level coefficient.
	for _, test := range []struct {
		name                                           string
		binary                                         bool
		resistance, penetration, hit, miss, meanDamage float64
	}{
		{"direct zero resistance", false, 0, 0, 0, .17, 86.71923},
		{"direct full penetration", false, 100, 100, 0, .17, 86.71923},
		{"direct hit floor", false, 0, 0, 30, .01, 103.43619},
		{"binary resistance", true, 100, 0, 10, .2775, 80.305875},
		{"binary full penetration", true, 100, 100, 10, .07, 103.3695},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := classic60SpellTestConfig{level: 63, resistance: test.resistance, penetration: test.penetration, hit: test.hit,
				bonusCrit: 20, binary: test.binary, duration: 30000 * time.Second, iterations: 1, seed: 742}
			sim, _, events := newClassic60SpellTestSim(t, config)
			metrics := classic60SpellTestMetrics(t, sim.run())
			n := float64(len(*events))
			if n != 10000 {
				t.Fatalf("sample count = %g", n)
			}
			if math.Abs(float64(metrics.Misses)/n-test.miss) > .015 {
				t.Fatalf("miss fraction=%g want %g", float64(metrics.Misses)/n, test.miss)
			}
			if math.Abs(float64(metrics.Crits)/(n-float64(metrics.Misses))-.223) > .02 {
				t.Fatal("conditional spell crit did not match Classic baseline")
			}
			if math.Abs(metrics.Damage/n-test.meanDamage) > 2 {
				t.Fatalf("mean damage=%g want %g", metrics.Damage/n, test.meanDamage)
			}
		})
	}
}

func TestClassic60SpellRuntimePureDotApplicationsTicksAndReset(t *testing.T) {
	config := classic60SpellTestConfig{level: 63, resistance: 200, hit: 30, bonusCrit: 95, pureDot: true,
		// The last application's final tick is pending at the iteration end.
		duration: 29 * time.Second, iterations: 3, seed: 157}
	sim, _, events := newClassic60SpellTestSim(t, config)
	result := sim.run()
	metrics := classic60SpellTestMetrics(t, result)
	applications, expectedTicks, ticks, ticksInApplication := 0, 0, 0, 0
	damage := 0.0
	var lastApplication classic60SpellTestEvent
	for _, event := range *events {
		if !event.periodic {
			if event.at != time.Duration(applications%4)*8*time.Second || event.seed != config.seed+int64(applications/4) {
				t.Fatalf("dot application did not reset: %+v", event)
			}
			applications++
			lastApplication = event
			ticksInApplication = 0
			if event.outcome.Matches(OutcomeLanded) {
				if event.at == 24*time.Second {
					expectedTicks += 2
				} else {
					expectedTicks += 3
				}
			}
			if event.damage != 0 {
				t.Fatal("pure dot dealt direct damage")
			}
			continue
		}
		if !lastApplication.outcome.Matches(OutcomeLanded) || event.seed != lastApplication.seed {
			t.Fatal("dot ticked after a miss or leaked across iterations")
		}
		ticksInApplication++
		delta := event.at - lastApplication.at
		if ticksInApplication > 3 || delta != time.Duration(ticksInApplication)*2*time.Second {
			t.Fatalf("unexpected tick schedule: %+v", event)
		}
		if event.outcome&^OutcomePartial != OutcomeHit {
			t.Fatalf("pure dot tick rolled hit/crit: %+v", event)
		}
		assertFloat64(t, "periodic damage after resistance", event.damage, 100*event.mitigation)
		ticks++
		damage += event.damage
	}
	if applications != 12 || ticks != expectedTicks || int(metrics.Ticks) != ticks || metrics.Crits != 0 || metrics.CritTicks != 0 {
		t.Fatalf("dot metrics do not match applications/ticks: %+v", metrics)
	}
	assertFloat64(t, "dot report damage", metrics.Damage, damage)
	repeat, _, repeatedEvents := newClassic60SpellTestSim(t, config)
	classic60MeleeTestAssertResultEqual(t, result, repeat.run())
	if !reflect.DeepEqual(*events, *repeatedEvents) {
		t.Fatal("dot trace not reproducible")
	}
}

func TestClassic60SpellRuntimePureDotResistanceAndMissedApplications(t *testing.T) {
	for _, test := range []struct {
		name                  string
		penetration, meanTick float64
	}{
		{"200 resistance uses pure-dot scaling", 0, 89},
		{"penetration removes explicit resistance only", 200, 94},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := classic60SpellTestConfig{level: 63, resistance: 200, penetration: test.penetration,
				pureDot: true, bonusCrit: 95, duration: 79998 * time.Second, iterations: 1, seed: 294}
			sim, _, events := newClassic60SpellTestSim(t, config)
			metrics := classic60SpellTestMetrics(t, sim.run())
			applications, landed, ticks := 0, 0, 0
			lastLanded := false
			for _, event := range *events {
				if !event.periodic {
					applications++
					lastLanded = event.outcome.Matches(OutcomeLanded)
					if lastLanded {
						landed++
					}
				} else {
					if !lastLanded {
						t.Fatal("missed application scheduled dot damage")
					}
					ticks++
				}
			}
			if applications != 10000 || ticks != 3*landed || metrics.Misses == 0 || metrics.CritTicks != 0 || metrics.Crits != 0 {
				t.Fatalf("invalid pure-dot schedule: applications=%d landed=%d ticks=%d metrics=%+v", applications, landed, ticks, metrics)
			}
			if math.Abs(float64(metrics.Misses)/10000-.17) > .015 {
				t.Fatal("pure-dot application did not use ordinary spell miss")
			}
			if math.Abs(metrics.Damage/float64(ticks)-test.meanTick) > 1 {
				t.Fatalf("mean tick damage=%g want %g", metrics.Damage/float64(ticks), test.meanTick)
			}
		})
	}
}
