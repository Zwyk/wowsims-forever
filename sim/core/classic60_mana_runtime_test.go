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

type classic60ManaTestEvent struct {
	kind                     string
	seed                     int64
	at, fsr                  time.Duration
	mana, damage, mitigation float64
	outcome                  HitOutcome
	oom                      bool
}

type classic60ManaTestAgent struct {
	FakeAgent
	continuous bool
	events     []classic60ManaTestEvent
	t          *testing.T
}

func (agent *classic60ManaTestAgent) record(sim *Simulation, kind string, result *SpellResult) {
	event := classic60ManaTestEvent{kind: kind, seed: sim.currentSeed, at: sim.CurrentTime,
		fsr: agent.PseudoStats.FiveSecondRuleRefreshTime, mana: agent.CurrentMana(), oom: agent.IsOOM()}
	if result != nil {
		event.damage, event.mitigation, event.outcome = result.Damage, result.ArmorAndResistanceMultiplier, result.Outcome
	}
	agent.events = append(agent.events, event)
}

func (agent *classic60ManaTestAgent) tryFireball(sim *Simulation) {
	if sim.CurrentTime >= sim.Duration || agent.Hardcast.Expires > sim.CurrentTime || !agent.GCD.IsReady(sim) {
		return
	}
	wasOOM := agent.IsOOM()
	if !agent.Spell.CanCast(sim, agent.CurrentTarget) {
		if !wasOOM && agent.IsOOM() {
			agent.record(sim, "oom", nil)
		}
		return
	}
	before := agent.CurrentMana()
	if !agent.Spell.Cast(sim, agent.CurrentTarget) {
		agent.t.Fatal("ready Fireball failed to cast")
	}
	if agent.CurrentMana() != before {
		agent.t.Fatal("hardcast spent mana before completion")
	}
	agent.record(sim, "start", nil)
}

func (agent *classic60ManaTestAgent) OnManaTick(sim *Simulation) {
	agent.record(sim, "regen", nil)
	if agent.continuous && agent.IsOOM() {
		agent.tryFireball(sim)
	}
}

type classic60ManaTestConfig struct {
	duration                          time.Duration
	iterations                        int32
	seed                              int64
	continuous, manual, apl           bool
	spellDamage, resistance, hit, mp5 float64
}

// This is a single-spell integration agent, not the inherited TBC Mage. It
// assembles the Classic Human Mage baseline, uses a real mana bar and Fireball
// rank 11, and retries after actual mana ticks rather than a regen estimate.
// The fixed 24-yard range makes the source's 24-yard/sec missile take one second.
func newClassic60ManaTestSim(t *testing.T, config classic60ManaTestConfig, configure ...func(*classic60ManaTestAgent)) (*Simulation, *classic60ManaTestAgent) {
	t.Helper()
	rules := classic60MageManaReferenceRules()
	baseline, ok := classicReferenceInitializeCharacter(60, proto.Race_RaceHuman, proto.Class_ClassMage)
	if !ok {
		t.Fatal("missing Human Mage baseline")
	}
	raid := NewRaid(&proto.Raid{})
	party := NewParty(raid, 0, &proto.Party{}, &proto.Raid{})
	raid.Parties = []*Party{party}
	agent := &classic60ManaTestAgent{t: t, continuous: config.continuous}
	agent.Character = newCharacterWithRuleset(party, 0, &proto.Player{
		Name: "Classic mana fixture", Race: proto.Race_RaceHuman, Class: proto.Class_ClassMage,
		Spec: &proto.Player_Mage{}, Equipment: &proto.EquipmentSpec{}, DistanceFromTarget: 24,
	}, rules, baseline.baseStats)
	agent.AddBaseClassStatDependencies()
	agent.EnableManaBar()
	agent.AddStats(stats.Stats{stats.SpellDamage: config.spellDamage, stats.SpellHitPercent: config.hit, stats.MP5: config.mp5})
	party.Players = []Agent{agent}
	raid.updatePlayersAndPets()
	targetConfig := &proto.Target{Level: 63, Stats: stats.Stats{stats.FireResistance: config.resistance}.ToProtoArray()}
	target := newTargetWithRuleset(targetConfig, 0, rules)
	target.stats[stats.BlockValue] = 0
	env := &Environment{State: Constructed, Raid: raid, BaseDuration: config.duration,
		Encounter: Encounter{Duration: config.duration, AllTargets: []*Target{target}, ActiveTargets: []*Target{target},
			AllTargetUnits: []*Unit{&target.Unit}, ActiveTargetUnits: []*Unit{&target.Unit}},
		AllUnits: []*Unit{&target.Unit, &agent.Unit}}
	env.Encounter.updateAOECapMultiplier()
	for i, unit := range env.AllUnits {
		unit.Env, unit.UnitIndex = env, int32(i)
	}
	agent.CurrentTarget = &target.Unit
	target.initialize(targetConfig)
	agent.initialize(agent)
	agent.Spell = registerClassic60ReferenceFireball(&agent.Character)
	agent.RegisterAura(Aura{Label: "Observe Classic mana and Fireball", Duration: NeverExpires,
		OnReset: func(aura *Aura, sim *Simulation) {
			aura.Activate(sim)
			sim.AddPendingAction(&PendingAction{NextActionAt: 0, OnAction: func(sim *Simulation) {
				agent.record(sim, "reset", nil)
				if !config.manual && !config.apl {
					agent.tryFireball(sim)
				}
			}})
		},
		OnCastComplete: func(_ *Aura, sim *Simulation, spell *Spell) {
			if spell != agent.Spell {
				return
			}
			agent.record(sim, "complete", nil)
			if agent.continuous {
				sim.AddPendingAction(&PendingAction{NextActionAt: sim.CurrentTime, OnAction: func(sim *Simulation) { agent.tryFireball(sim) }})
			}
		},
		OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
			if spell == agent.Spell {
				agent.record(sim, "impact", result)
			}
		},
		OnPeriodicDamageDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
			if spell == agent.Spell {
				agent.record(sim, "tick", result)
			}
		},
	})
	for _, setup := range configure {
		setup(agent)
	}
	env.State = Initialized
	target.finalize()
	agent.Finalize()
	if config.apl {
		// Exercise the same parsed priority list and scheduler as the full
		// WoWSims UI, with no test callback choosing or retrying casts.
		agent.Rotation = agent.newAPLRotation(&proto.APLRotation{
			Type: proto.APLRotation_TypeAPL,
			PriorityList: []*proto.APLListItem{{Action: &proto.APLAction{
				Action: &proto.APLAction_CastSpell{CastSpell: &proto.APLActionCastSpell{
					SpellId: ActionID{SpellID: 10151}.ToProto(),
				}},
			}}},
		})
		if len(agent.Rotation.priorityList) != 1 || len(agent.Rotation.priorityListValidations[0]) != 0 {
			t.Fatal("Fireball priority list did not resolve cleanly")
		}
	}
	env.setupAttackTables()
	for i := 0; i < len(env.postFinalizeEffects); i++ {
		env.postFinalizeEffects[i]()
	}
	env.postFinalizeEffects = nil
	env.State = Finalized
	want := baseline.stats
	want[stats.SpellDamage], want[stats.SpellHitPercent], want[stats.MP5] = config.spellDamage, config.hit, config.mp5
	if agent.GetStats() != want || agent.BaseMana != 1213 || agent.MaxMana() != 2808 || !agent.HasManaBar() {
		t.Fatal("mana initialization changed the Classic baseline")
	}
	return newSimWithEnv(env, &proto.SimOptions{Iterations: config.iterations, RandomSeed: config.seed, IsTest: true}, simsignals.CreateSignals()), agent
}

func classic60FireballMetrics(t *testing.T, result *proto.RaidSimResult) *proto.TargetedActionMetrics {
	t.Helper()
	if result.Error != nil {
		t.Fatalf("Classic mana simulation failed: %v", result.Error)
	}
	for _, action := range result.RaidMetrics.Parties[0].Players[0].Actions {
		if action.Id.GetSpellId() == 10151 {
			for _, target := range action.Targets {
				if target.UnitIndex == 0 {
					return target
				}
			}
		}
	}
	t.Fatal("Fireball missing from report")
	return nil
}

func TestClassic60FireballRankTravelDamageAndMana(t *testing.T) {
	config := classic60ManaTestConfig{duration: 14 * time.Second, iterations: 1, seed: 37, hit: 16, spellDamage: 100}
	sim, agent := newClassic60ManaTestSim(t, config)
	spell := agent.Spell
	if spell.ActionID.SpellID != 10151 || spell.DefaultCast.CastTime != 3500*time.Millisecond || spell.DefaultCast.GCD != 1500*time.Millisecond || spell.Cost.GetCurrentCost() != 395 || spell.MissileSpeed != 24 || spell.BonusCoefficient != 1 || spell.Flags.Matches(SpellFlagBinary|SpellFlagPureDot) {
		t.Fatal("Fireball rank 11 lost its pinned source parameters")
	}
	result := sim.run()
	metrics := classic60FireballMetrics(t, result)
	starts, completes, impacts, ticks := 0, 0, 0, 0
	totalDamage := 0.0
	for _, event := range agent.events {
		switch event.kind {
		case "start":
			starts++
			if event.at != 0 || event.mana != 2808 {
				t.Fatalf("bad cast start: %+v", event)
			}
		case "complete":
			completes++
			if event.at != 3500*time.Millisecond || event.mana != 2413 || event.fsr != 8500*time.Millisecond {
				t.Fatalf("bad mana spending/FSR: %+v", event)
			}
		case "impact":
			impacts++
			if event.at != 4500*time.Millisecond || !event.outcome.Matches(OutcomeLanded) {
				t.Fatalf("bad projectile impact: %+v", event)
			}
			base := event.damage / event.mitigation
			if event.outcome.Matches(OutcomeCrit) {
				base /= 1.5
			}
			if base < 661 || base > 815 {
				t.Fatalf("direct coefficient/range mismatch: %+v", event)
			}
			totalDamage += event.damage
		case "tick":
			ticks++
			if event.at != 4500*time.Millisecond+time.Duration(ticks)*2*time.Second || event.outcome.Matches(OutcomeCrit) {
				t.Fatalf("bad residual tick: %+v", event)
			}
			assertFloat64(t, "Fireball residual has no spell-damage coefficient", event.damage, 18*event.mitigation)
			totalDamage += event.damage
		case "regen":
			want := 2413.0
			if event.at == 2*time.Second {
				want = 2808
			}
			if event.at >= 10*time.Second {
				want += 42.5 * (float64(event.at/time.Second) - 8) / 2
			}
			assertFloat64(t, "two-second Classic mana tick", event.mana, want)
		}
	}
	if starts != 1 || completes != 1 || impacts != 1 || ticks != 4 || metrics.Casts != 1 || metrics.Ticks != 4 || metrics.CritTicks != 0 {
		t.Fatalf("incomplete real spell lifecycle: starts=%d completes=%d impacts=%d ticks=%d metrics=%+v", starts, completes, impacts, ticks, metrics)
	}
	assertFloat64(t, "Fireball direct plus residual report", metrics.Damage, totalDamage)
}

func TestClassic60FireballOOMTickRecoveryAndReset(t *testing.T) {
	config := classic60ManaTestConfig{duration: 73 * time.Second, iterations: 3, seed: 191, continuous: true}
	sim, agent := newClassic60ManaTestSim(t, config)
	initialStats, active := agent.GetStats(), currentRuleset()
	result := sim.run()
	metrics := classic60FireballMetrics(t, result)
	// Seven consecutive casts leave 43 mana at 24.5s. Full ticks resume at
	// 30s; the ninth reaches 425.5 mana at 46s. Regen continues while casting
	// until mana is actually spent at 49.5s. The next recovery is at 70s.
	wantStarts := []time.Duration{0, 3500 * time.Millisecond, 7 * time.Second, 10500 * time.Millisecond, 14 * time.Second, 17500 * time.Millisecond, 21 * time.Second, 46 * time.Second, 70 * time.Second}
	wantStartMana := []float64{2808, 2413, 2018, 1623, 1228, 833, 438, 425.5, 413}
	starts, completes, resets, oomEvents := 0, 0, 0, 0
	totalDamage := 0.0
	for _, event := range agent.events {
		if math.IsNaN(event.mana) || math.IsInf(event.mana, 0) || event.mana < 0 || event.mana > 2808 {
			t.Fatalf("invalid mana: %+v", event)
		}
		switch event.kind {
		case "reset":
			resets++
			if event.mana != 2808 || event.oom || event.fsr != 0 {
				t.Fatalf("mana/FSR/OOM leaked across reset: %+v", event)
			}
		case "start":
			i := starts % len(wantStarts)
			if event.at != wantStarts[i] || event.seed != config.seed+int64(starts/len(wantStarts)) || event.mana != wantStartMana[i] || event.oom {
				t.Fatalf("tick-driven recovery differs from literal schedule: index %d event %+v", i, event)
			}
			starts++
		case "complete":
			completes++
		case "oom":
			oomEvents++
		case "impact", "tick":
			totalDamage += event.damage
		}
	}
	if resets != 3 || starts != 27 || completes != 24 || oomEvents != 6 || metrics.Casts != 24 {
		t.Fatalf("bad OOM rotation/reset: resets %d starts %d completes %d oom %d casts %d", resets, starts, completes, oomEvents, metrics.Casts)
	}
	assertFloat64(t, "rotation report damage", metrics.Damage, totalDamage)
	if agent.GetStats() != initialStats || currentRuleset() != active || CharacterLevel != 70 {
		t.Fatal("mana rotation changed active TBC or initial stats")
	}
	repeat, repeatAgent := newClassic60ManaTestSim(t, config)
	classic60MeleeTestAssertResultEqual(t, result, repeat.run())
	if !reflect.DeepEqual(agent.events, repeatAgent.events) {
		t.Fatal("mana/cast/impact traces changed with identical seeds")
	}
}

func TestClassic60FireballPendingTravelAndDotReset(t *testing.T) {
	for _, duration := range []time.Duration{4 * time.Second, 9 * time.Second} {
		config := classic60ManaTestConfig{duration: duration, iterations: 3, seed: 37, hit: 16}
		sim, agent := newClassic60ManaTestSim(t, config)
		result := sim.run()
		metrics := classic60FireballMetrics(t, result)
		impacts, ticks := 0, 0
		for _, event := range agent.events {
			switch event.kind {
			case "reset":
				if event.mana != 2808 || event.fsr != 0 || event.oom {
					t.Fatalf("unclean reset: %+v", event)
				}
			case "impact":
				impacts++
				if event.at != 4500*time.Millisecond {
					t.Fatalf("projectile leaked across iteration: %+v", event)
				}
			case "tick":
				ticks++
				if event.at != 6500*time.Millisecond && event.at != 8500*time.Millisecond {
					t.Fatalf("DoT leaked across iteration: %+v", event)
				}
			}
		}
		if metrics.Casts != 3 {
			t.Fatal("pending work changed completed cast count")
		}
		if duration == 4*time.Second && (impacts != 0 || ticks != 0 || metrics.Damage != 0) {
			t.Fatal("unfinished projectile landed after fight")
		}
		if duration == 9*time.Second && (impacts != 3 || ticks != 6) {
			t.Fatalf("pending DoT lost/duplicated ticks: impacts %d ticks %d", impacts, ticks)
		}
	}
}

func TestClassic60FireballNativeAPLRotation(t *testing.T) {
	config := classic60ManaTestConfig{duration: 73 * time.Second, iterations: 3, seed: 191, apl: true}
	sim, agent := newClassic60ManaTestSim(t, config)
	result := sim.run()
	metrics := classic60FireballMetrics(t, result)
	wantCompletions := []time.Duration{3500 * time.Millisecond, 7 * time.Second, 10500 * time.Millisecond,
		14 * time.Second, 17500 * time.Millisecond, 21 * time.Second, 24500 * time.Millisecond, 49500 * time.Millisecond}
	completions := 0
	for _, event := range agent.events {
		if event.kind == "start" || event.kind == "oom" {
			t.Fatal("test callback, rather than the native APL, controlled a cast")
		}
		if event.kind == "complete" {
			if event.at != wantCompletions[completions%len(wantCompletions)] {
				t.Fatalf("native APL cast/recovery timing: %+v", event)
			}
			completions++
		}
	}
	if completions != 24 || metrics.Casts != 24 {
		t.Fatalf("native APL did not sustain the OOM/recovery rotation: %d completions, %d casts", completions, metrics.Casts)
	}
	player := result.RaidMetrics.Parties[0].Players[0]
	assertFloat64(t, "two OOM waits per iteration", player.SecondsOomAvg, 42)
	spent, regenerated := false, false
	for _, resource := range player.Resources {
		if resource.Id.GetSpellId() == 10151 {
			spent = true
			if resource.Events != 24 || resource.Gain != -9480 || resource.ActualGain != -9480 {
				t.Fatalf("Fireball mana report: %+v", resource)
			}
		}
		if resource.Id.GetOtherId() == proto.OtherAction_OtherActionManaRegen && resource.Id.Tag == 2 {
			regenerated = true
			if resource.Events != 60 || resource.Gain != 2550 || resource.ActualGain != 2422.5 {
				t.Fatalf("mana regeneration report lost capping/iteration totals: %+v", resource)
			}
		}
	}
	if !spent || !regenerated {
		t.Fatal("mana spending or regeneration missing from the report")
	}
	repeat, repeatedAgent := newClassic60ManaTestSim(t, config)
	classic60MeleeTestAssertResultEqual(t, result, repeat.run())
	if !reflect.DeepEqual(agent.events, repeatedAgent.events) {
		t.Fatal("native APL changed its trace with identical seeds")
	}
}

func TestClassic60ManaInstantCostMP5AndFiveSecondBoundary(t *testing.T) {
	config := classic60ManaTestConfig{duration: 25 * time.Second, iterations: 1, seed: 37, mp5: 25, manual: true}
	sim, agent := newClassic60ManaTestSim(t, config, func(agent *classic60ManaTestAgent) {
		instant := agent.RegisterSpell(SpellConfig{
			ActionID: ActionID{SpellID: 900060}, ManaCost: ManaCostOptions{FlatCost: 395},
			Cast: CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault}},
		})
		agent.RegisterAura(Aura{Label: "Instant mana boundary fixture", OnReset: func(_ *Aura, sim *Simulation) {
			sim.AddPendingAction(&PendingAction{NextActionAt: time.Second, OnAction: func(sim *Simulation) {
				if !instant.Cast(sim, agent.CurrentTarget) || agent.CurrentMana() != 2413 || agent.PseudoStats.FiveSecondRuleRefreshTime != 6*time.Second {
					t.Fatal("instant cost/FSR not applied at cast time")
				}
			}})
		}})
	})
	result := sim.run()
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	ticks := 0
	for _, event := range agent.events {
		if event.kind != "regen" {
			continue
		}
		ticks++
		want := 2413 + 10*float64(ticks)
		if event.at >= 6*time.Second {
			want = math.Min(2808, 2433+52.5*float64(ticks-2))
		}
		assertFloat64(t, "MP5 during FSR; full tick exactly at expiry; cap", event.mana, want)
	}
	if ticks != 12 || agent.CurrentMana() != 2808 {
		t.Fatal("missing scheduled ticks or mana cap")
	}
	assertFloat64(t, "capped full-tick nominal gain", agent.manaNotCastingMetrics.Gain, 525)
	assertFloat64(t, "capped full-tick actual gain", agent.manaNotCastingMetrics.ActualGain, 375)
}

func TestClassic60FireballInterruptedCastDoesNotSpendMana(t *testing.T) {
	config := classic60ManaTestConfig{duration: 14 * time.Second, iterations: 3, seed: 37}
	sim, agent := newClassic60ManaTestSim(t, config, func(agent *classic60ManaTestAgent) {
		agent.RegisterAura(Aura{Label: "Interrupt Fireball fixture", OnReset: func(_ *Aura, sim *Simulation) {
			sim.AddPendingAction(&PendingAction{NextActionAt: time.Second, OnAction: func(sim *Simulation) {
				agent.Interrupt(sim)
			}})
		}})
	})
	result := sim.run()
	metrics := classic60FireballMetrics(t, result)
	if metrics.Casts != 0 || metrics.Damage != 0 || metrics.Ticks != 0 {
		t.Fatal("interrupted Fireball completed or dealt damage")
	}
	for _, event := range agent.events {
		if event.mana != 2808 || event.fsr != 0 || event.kind == "complete" || event.kind == "impact" || event.kind == "tick" {
			t.Fatalf("interrupted cast spent mana, refreshed FSR or scheduled damage: %+v", event)
		}
	}
	cost := agent.Spell.Cost.ResourceCostImpl.(*ManaCost).ResourceMetrics
	if cost.Events != 0 || cost.ActualGain != 0 {
		t.Fatal("interrupted cast charged mana in report")
	}
}

func TestClassic60FireballMissAndResidualResistance(t *testing.T) {
	// Source Fireball's residual is not a pure DoT: it takes the ordinary
	// resistance coefficient (including +3 level resistance), not the /10
	// explicit-resistance treatment used by the earlier pure-DoT fixture.
	config := classic60ManaTestConfig{duration: 14 * time.Second, iterations: 1000, seed: 811, resistance: 200, spellDamage: 400}
	sim, agent := newClassic60ManaTestSim(t, config)
	metrics := classic60FireballMetrics(t, sim.run())
	misses, landed, ticks := 0, 0, 0
	lastLanded := false
	totalTickDamage := 0.0
	for _, event := range agent.events {
		switch event.kind {
		case "complete":
			if event.mana != 2413 || event.fsr != 8500*time.Millisecond {
				t.Fatal("spell outcome changed the completed cast's mana cost")
			}
		case "impact":
			lastLanded = event.outcome.Matches(OutcomeLanded)
			if lastLanded {
				landed++
			} else {
				misses++
			}
		case "tick":
			if !lastLanded || event.outcome.Matches(OutcomeCrit) {
				t.Fatal("missed Fireball applied its residual, or its residual crit")
			}
			assertFloat64(t, "residual does not scale with spell damage", event.damage, 18*event.mitigation)
			ticks++
			totalTickDamage += event.damage
		}
	}
	if misses == 0 || landed == 0 || landed+misses != 1000 || ticks != 4*landed || int(metrics.Ticks) != ticks {
		t.Fatalf("miss/application accounting: misses %d, landed %d, ticks %d", misses, landed, ticks)
	}
	// 200 resistance vs level 63 gives 54.56% average mitigation in the
	// pinned Classic projection; a pure-DoT misclassification gives only 11%.
	if got := totalTickDamage / float64(ticks); math.Abs(got-8.1792) > .4 {
		t.Fatalf("Fireball residual mean damage %g, want approximately 8.1792", got)
	}
	assertFloat64(t, "all completed casts spend mana, including misses", agent.Spell.Cost.ResourceCostImpl.(*ManaCost).ResourceMetrics.ActualGain, -395000)
}
