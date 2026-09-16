package core

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/simsignals"
	"github.com/wowsims/tbc/sim/core/stats"
	"google.golang.org/protobuf/encoding/protojson"
)

type classic60PaladinTestEvent struct {
	kind                 string
	seed                 int64
	at, fsr              time.Duration
	mana, damage, resist float64
	outcome              HitOutcome
	seal, oom            bool
}

type classic60PaladinTestAgent struct {
	FakeAgent
	spells       *classic60PaladinSpells
	events       []classic60PaladinTestEvent
	startingMana float64
	pauseAutos   bool
}

func (agent *classic60PaladinTestAgent) Reset(sim *Simulation) {
	if agent.pauseAutos {
		// startPull runs before the pending time-zero cancellation and can
		// schedule a white swing immediately. Hold that first swing past the
		// encounter so isolated spell tests cannot proc talents at pull.
		agent.AutoAttacks.DelayMeleeBy(sim, sim.Duration+time.Second)
	}
	if agent.startingMana > 0 {
		// Agent.Reset runs after the resource bar reset and before the native
		// APL can cast at pull. This changes only the test's initial state.
		agent.currentMana = agent.startingMana
	}
	agent.record(sim, "reset", nil)
}

func (agent *classic60PaladinTestAgent) record(sim *Simulation, kind string, result *SpellResult) {
	event := classic60PaladinTestEvent{kind: kind, seed: sim.currentSeed, at: sim.CurrentTime,
		fsr: agent.PseudoStats.FiveSecondRuleRefreshTime, mana: agent.CurrentMana(),
		seal: agent.spells.SealAura != nil && agent.spells.SealAura.IsActive(), oom: agent.IsOOM()}
	if result != nil {
		event.damage, event.resist, event.outcome = result.Damage, result.ArmorAndResistanceMultiplier, result.Outcome
	}
	agent.events = append(agent.events, event)
}

func (agent *classic60PaladinTestAgent) OnManaTick(sim *Simulation) {
	agent.record(sim, "regen", nil)
}

type classic60PaladinTestConfig struct {
	duration                  time.Duration
	iterations                int32
	seed                      int64
	targetLevel               int32
	spellDamage, physicalCrit float64
	mp5, armor, startingMana  float64
	apl, pauseAutos           bool
	oneHandShield, front      bool
	talents                   *classic60PaladinTalents
}

// Assemble a pre-racial Human Paladin with a synthetic equipped 2H sword.
// The actual modern event loop, native APL, resource bar, auto-attacks and
// damage reports run under the private Classic profile. No inherited TBC
// class constructor, real item effect, talent tree or encounter AI is used.
func newClassic60PaladinTestSim(t *testing.T, config classic60PaladinTestConfig, configure ...func(*classic60PaladinTestAgent)) (*Simulation, *classic60PaladinTestAgent) {
	t.Helper()
	if config.targetLevel == 0 {
		config.targetLevel = 63
	}
	if config.iterations == 0 {
		config.iterations = 1
	}
	rules := classic60PaladinReferenceRules()
	baseline, ok := classicReferenceInitializeCharacter(60, proto.Race_RaceHuman, proto.Class_ClassPaladin)
	if !ok {
		t.Fatal("missing Classic Human Paladin baseline")
	}
	raid := NewRaid(&proto.Raid{})
	party := NewParty(raid, 0, &proto.Party{}, &proto.Raid{})
	raid.Parties = []*Party{party}
	agent := &classic60PaladinTestAgent{startingMana: config.startingMana, pauseAutos: config.pauseAutos}
	agent.Character = newCharacterWithRuleset(party, 0, &proto.Player{
		Name: "Classic Paladin fixture", Race: proto.Race_RaceHuman, Class: proto.Class_ClassPaladin,
		Spec: &proto.Player_RetributionPaladin{}, Equipment: &proto.EquipmentSpec{},
	}, rules, baseline.baseStats)
	agent.AddBaseClassStatDependencies()
	agent.EnableManaBar()
	agent.AddStats(stats.Stats{stats.SpellDamage: config.spellDamage,
		stats.PhysicalCritPercent: config.physicalCrit, stats.MP5: config.mp5})
	agent.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{
		ID: -1, Type: proto.ItemType_ItemTypeWeapon,
		WeaponType: proto.WeaponType_WeaponTypeSword, HandType: proto.HandType_HandTypeTwoHand,
		WeaponDamageMin: 100, WeaponDamageMax: 100, SwingSpeed: 3,
	}
	if config.oneHandShield {
		agent.Equipment[proto.ItemSlot_ItemSlotMainHand].HandType = proto.HandType_HandTypeOneHand
		agent.Equipment[proto.ItemSlot_ItemSlotOffHand] = Item{ID: -2, Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeShield, HandType: proto.HandType_HandTypeOffHand}
	}
	agent.PseudoStats.InFrontOfTarget = config.front
	agent.EnableAutoAttacks(agent, AutoAttackOptions{MainHand: agent.WeaponFromMainHand(), AutoSwingMelee: true})
	agent.AutoAttacks.RandomMeleeOffset = false
	party.Players = []Agent{agent}
	raid.updatePlayersAndPets()
	targetConfig := &proto.Target{Level: config.targetLevel, Stats: stats.Stats{stats.Armor: config.armor}.ToProtoArray()}
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
	if config.talents == nil {
		agent.spells = registerClassic60ReferencePaladin(&agent.Character, true)
	} else {
		var err error
		agent.spells, err = registerClassic60PaladinBuild(&agent.Character, *config.talents)
		if err != nil {
			t.Fatal(err)
		}
	}
	// Judgement intentionally suppresses OnCastComplete/OnApplyEffects.
	// Observe its real effect entry without changing any casting decision.
	if agent.spells.Judgement != nil {
		judgeEffects := agent.spells.Judgement.ApplyEffects
		agent.spells.Judgement.ApplyEffects = func(sim *Simulation, target *Unit, spell *Spell) {
			agent.record(sim, "judge", nil)
			judgeEffects(sim, target, spell)
		}
	}
	agent.RegisterAura(Aura{Label: "Observe Classic Paladin", Duration: NeverExpires,
		OnReset: func(aura *Aura, sim *Simulation) {
			aura.Activate(sim)
			sim.AddPendingAction(&PendingAction{NextActionAt: 0, Priority: ActionPriorityPrePull, OnAction: func(sim *Simulation) {
				if config.pauseAutos {
					agent.AutoAttacks.CancelAutoSwing(sim)
				}
			}})
		},
		OnApplyEffects: func(_ *Aura, sim *Simulation, _ *Unit, spell *Spell) {
			switch spell {
			case agent.spells.SealOfCommand:
				agent.record(sim, "seal", nil)
			case agent.spells.CommandProc:
				agent.record(sim, "proc_cast", nil)
			}
		},
		OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
			switch spell {
			case agent.AutoAttacks.MHAuto():
				agent.record(sim, "white", result)
			case agent.spells.CommandProc:
				agent.record(sim, "proc_hit", result)
			case agent.spells.CommandJudgement:
				agent.record(sim, "judgement_hit", result)
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
		rotation := &proto.APLRotation{}
		if err := protojson.Unmarshal([]byte(`{"type":"TypeAPL","priorityList":[
			{"action":{"condition":{"auraIsInactive":{"auraId":{"spellId":20920}}},"castSpell":{"spellId":{"spellId":20920}}}},
			{"action":{"castSpell":{"spellId":{"spellId":20271}}}}
		]}`), rotation); err != nil {
			t.Fatal(err)
		}
		agent.Rotation = agent.newAPLRotation(rotation)
		if len(agent.Rotation.priorityList) != 2 {
			t.Fatal("native Paladin APL lost a seal or judgement action")
		}
		for _, validations := range agent.Rotation.priorityListValidations {
			if len(validations) != 0 {
				t.Fatalf("native Paladin APL did not resolve: %v", validations)
			}
		}
	}
	env.setupAttackTables()
	for i := 0; i < len(env.postFinalizeEffects); i++ {
		env.postFinalizeEffects[i]()
	}
	env.postFinalizeEffects = nil
	env.State = Finalized
	if config.talents == nil && (agent.BaseMana != 1512 || agent.MaxMana() != 2282 || agent.GetStat(stats.Spirit) != 75 || agent.GetStat(stats.AttackPower) != 370) {
		t.Fatalf("Paladin initialization changed the pre-racial Classic baseline: %+v", agent.GetStats())
	}
	return newSimWithEnv(env, &proto.SimOptions{Iterations: config.iterations, RandomSeed: config.seed, IsTest: true}, simsignals.CreateSignals()), agent
}

func classic60PaladinAt(agent *classic60PaladinTestAgent, at time.Duration, action func(*Simulation)) {
	agent.RegisterAura(Aura{Label: "Paladin fixture action " + at.String(), OnReset: func(_ *Aura, sim *Simulation) {
		sim.AddPendingAction(&PendingAction{NextActionAt: at, OnAction: action})
	}})
}

// Pin only a single spell's random draws; restore the simulation's streams
// immediately so unrelated auto-attacks and iteration reseeding remain real.
func classic60PaladinRolls(t *testing.T, sim *Simulation, values []float64, action func()) {
	t.Helper()
	previous, previousIsTest := sim.rand, sim.isTest
	roll := &sequenceMeleeRoll{values: values}
	sim.rand, sim.isTest = roll, false
	defer func() { sim.rand, sim.isTest = previous, previousIsTest }()
	action()
	if roll.calls != len(values) {
		t.Fatalf("Paladin outcome consumed %d random draws, want %d", roll.calls, len(values))
	}
}

func classic60PaladinResult(t *testing.T, result *proto.RaidSimResult) *proto.UnitMetrics {
	t.Helper()
	if result.Error != nil {
		t.Fatalf("Classic Paladin simulation failed: %v", result.Error)
	}
	return result.RaidMetrics.Parties[0].Players[0]
}

func TestClassic60PaladinManaSealAndExpiration(t *testing.T) {
	sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{
		duration: 32 * time.Second, seed: 41, mp5: 25,
	}, func(agent *classic60PaladinTestAgent) {
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if !agent.spells.SealOfCommand.Cast(sim, agent.CurrentTarget) {
				t.Fatal("ready Command failed")
			}
			assertFloat64(t, "rank-5 seal costs 210 mana immediately", agent.CurrentMana(), 2072)
			if agent.NextGCDAt() != 2500*time.Millisecond || agent.PseudoStats.FiveSecondRuleRefreshTime != 6*time.Second {
				t.Fatal("instant seal lost its 1.5-second GCD or spending-time FSR")
			}
		})
		classic60PaladinAt(agent, 1100*time.Millisecond, func(sim *Simulation) {
			before := agent.CurrentMana()
			if agent.spells.SealOfCommand.Cast(sim, agent.CurrentTarget) || agent.CurrentMana() != before {
				t.Fatal("seal bypassed the active GCD or charged a failed cast")
			}
		})
		classic60PaladinAt(agent, 31*time.Second+time.Nanosecond, func(_ *Simulation) {
			if agent.spells.SealAura.IsActive() {
				t.Fatal("seal outlived its 30-second duration")
			}
		})
	})
	assertFloat64(t, "Paladin spirit regen excludes Mage formula", agent.SpiritManaRegenPerSecond(), 15)
	assertFloat64(t, "Judgement uses six percent of Classic base mana", agent.spells.Judgement.Cost.GetCurrentCost(), 90.72)
	assertFloat64(t, "finalized judgement retains its fractional cost", agent.spells.Judgement.DefaultCast.Cost, 90.72)
	classic60PaladinResult(t, sim.run())
	for _, event := range agent.events {
		if event.kind != "regen" {
			continue
		}
		// 25 MP5 gives 10 per two-second tick. At t=6 exactly, the
		// 75-Spirit Paladin adds 30, with the final gain capped at 2282.
		want := 2072 + 10*float64(event.at/(2*time.Second))
		if event.at >= 6*time.Second {
			want = math.Min(2282, 2092+40*float64((event.at-4*time.Second)/(2*time.Second)))
		}
		assertFloat64(t, "Classic Paladin FSR/MP5 boundary and mana cap", event.mana, want)
	}
}

func TestClassic60PaladinJudgementOffGCDMissConsumptionAndCooldown(t *testing.T) {
	sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 12 * time.Second, seed: 19}, func(agent *classic60PaladinTestAgent) {
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if !agent.spells.SealOfCommand.Cast(sim, agent.CurrentTarget) {
				t.Fatal("initial seal failed")
			}
			classic60PaladinRolls(t, sim, []float64{.5, .999, .99}, func() {
				if !agent.spells.Judgement.Cast(sim, agent.CurrentTarget) {
					t.Fatal("Judgement was blocked by the seal's GCD")
				}
			})
			if agent.spells.SealAura.IsActive() || agent.NextGCDAt() != 2500*time.Millisecond || agent.spells.Judgement.CD.ReadyAt() != 11*time.Second {
				t.Fatal("missed judgement failed to consume the seal or changed GCD/CD")
			}
			assertFloat64(t, "missed judgement still spends exact mana", agent.CurrentMana(), 1981.28)
		})
		classic60PaladinAt(agent, 2500*time.Millisecond, func(sim *Simulation) {
			if !agent.spells.SealOfCommand.Cast(sim, agent.CurrentTarget) {
				t.Fatal("could not reseal after judgement")
			}
			before := agent.CurrentMana()
			if agent.spells.Judgement.Cast(sim, agent.CurrentTarget) || !agent.spells.SealAura.IsActive() || agent.CurrentMana() != before {
				t.Fatal("Judgement bypassed its cooldown or consumed resources on failure")
			}
		})
		classic60PaladinAt(agent, 11*time.Second, func(sim *Simulation) {
			classic60PaladinRolls(t, sim, []float64{.5, 0, .99, .99}, func() {
				if !agent.spells.Judgement.Cast(sim, agent.CurrentTarget) {
					t.Fatal("Judgement did not become available exactly at 10 seconds")
				}
			})
		})
	})
	classic60PaladinResult(t, sim.run())
	misses, hits := 0, 0
	for _, event := range agent.events {
		if event.kind == "judgement_hit" {
			if event.outcome.Matches(OutcomeMiss) {
				misses++
				assertFloat64(t, "missed judgement deals no damage", event.damage, 0)
			} else {
				hits++
			}
		}
	}
	if misses != 1 || hits != 1 {
		t.Fatalf("Judgement hit-check accounting: misses %d hits %d", misses, hits)
	}
}

func TestClassic60PaladinCommandDamageClassificationAndScaling(t *testing.T) {
	for _, level := range []int32{60, 63} {
		for _, spellDamage := range []float64{0, 100} {
			config := classic60PaladinTestConfig{duration: 6 * time.Second, seed: 13, targetLevel: level,
				spellDamage: spellDamage, physicalCrit: 25, armor: 3000, pauseAutos: true}
			sim, agent := newClassic60PaladinTestSim(t, config, func(agent *classic60PaladinTestAgent) {
				for index := 0; index < 5; index++ {
					index := index
					classic60PaladinAt(agent, time.Duration(index+1)*time.Second, func(sim *Simulation) {
						spell := agent.spells.CommandProc
						rolls := []float64{.5, .99, .99, .99}
						if index == 1 || index == 3 {
							spell = agent.spells.CommandJudgement
							rolls = []float64{.5, 0, .99, .99}
						}
						if index == 2 || index == 3 {
							rolls[3] = 0 // Independent melee crit roll, after hit.
						}
						if index == 4 {
							rolls[1] = .005 // +3 NPC partial resistance despite Holy school.
						}
						classic60PaladinRolls(t, sim, rolls, func() {
							if !spell.Cast(sim, agent.CurrentTarget) {
								t.Fatal("triggered Command spell failed")
							}
						})
					})
				}
			})
			classic60PaladinResult(t, sim.run())
			const weaponDamage = 100 + 3.0*370/14 // Fresh, non-normalized weapon/AP roll.
			procDamage := .7*weaponDamage + .20*spellDamage
			judgeDamage := 178 + .429*spellDamage
			count := 0
			for _, event := range agent.events {
				if event.kind != "proc_hit" && event.kind != "judgement_hit" {
					continue
				}
				count++
				want := procDamage
				if event.kind == "judgement_hit" {
					want = judgeDamage
				} else if event.at%time.Second != 10*time.Millisecond {
					t.Fatalf("Command proc lost its explicit 10ms delivery delay: %+v", event)
				}
				if event.at >= 3*time.Second && event.at < 5*time.Second {
					want *= 2
					if !event.outcome.Matches(OutcomeCrit) {
						t.Fatal("Holy melee spell lost its melee critical outcome")
					}
				}
				if event.at >= 5*time.Second && level == 63 {
					want *= .25
					assertFloat64(t, "Holy retains source level-based partial resistance", event.resist, .25)
				}
				assertFloat64(t, "Command source weapon/SP/crit scaling ignores armor", event.damage, want)
			}
			if count != 5 {
				t.Fatalf("expected five triggered damage events, got %d", count)
			}
		}
	}
}

func TestClassic60PaladinCommandWhiteOnlyPPMAndICD(t *testing.T) {
	sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{
		duration: 3 * time.Second, seed: 53, pauseAutos: true,
	}, func(agent *classic60PaladinTestAgent) {
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if !agent.spells.SealOfCommand.Cast(sim, agent.CurrentTarget) {
				t.Fatal("initial seal failed")
			}
		})
		for _, sample := range []struct {
			at      time.Duration
			kind    string
			outcome HitOutcome
			rolls   []float64
		}{
			{1100 * time.Millisecond, "white", OutcomeHit, []float64{.349, .5, .99, .99, .99}},
			{1200 * time.Millisecond, "white", OutcomeHit, nil},            // 1s ICD blocks even an eligible white.
			{2100 * time.Millisecond, "white", OutcomeHit, []float64{.35}}, // Exact 7*3/60 threshold fails.
			{2200 * time.Millisecond, "special", OutcomeHit, nil},
			{2300 * time.Millisecond, "magic", OutcomeHit, nil},
			{2400 * time.Millisecond, "white", OutcomeMiss, nil},
			{2500 * time.Millisecond, "white", OutcomeGlance, []float64{.349, .5, .99, .99, .99}},
		} {
			sample := sample
			classic60PaladinAt(agent, sample.at, func(sim *Simulation) {
				spell := agent.AutoAttacks.MHAuto()
				if sample.kind == "special" {
					spell = agent.spells.CommandJudgement
				} else if sample.kind == "magic" {
					spell = agent.spells.judgementHitCheck
				}
				classic60PaladinRolls(t, sim, sample.rolls, func() {
					agent.OnSpellHitDealt(sim, spell, &SpellResult{Target: agent.CurrentTarget, Outcome: sample.outcome})
				})
			})
		}
	})
	classic60PaladinResult(t, sim.run())
	casts, hits := 0, 0
	for _, event := range agent.events {
		if event.kind == "proc_cast" {
			casts++
		}
		if event.kind == "proc_hit" {
			hits++
			if event.at != 1110*time.Millisecond && event.at != 2510*time.Millisecond {
				t.Fatalf("ineligible hit, recursive proc or incorrect ICD: %+v", event)
			}
		}
	}
	if casts != 2 || hits != 2 {
		t.Fatalf("expected two eligible Command procs, got %d casts / %d hits", casts, hits)
	}
}

func TestClassic60PaladinNativeAPLOOMAutosRecoveryAndReset(t *testing.T) {
	config := classic60PaladinTestConfig{duration: 90 * time.Second, iterations: 3, seed: 173, startingMana: 210, apl: true}
	sim, agent := newClassic60PaladinTestSim(t, config)
	initialStats, active := agent.GetStats(), currentRuleset()
	result := sim.run()
	player := classic60PaladinResult(t, result)
	seals, judgements, resets, swings, oomSwings := 0, 0, 0, 0, 0
	lastJudge := map[int64]time.Duration{}
	for _, event := range agent.events {
		if math.IsNaN(event.mana) || event.mana < 0 || event.mana > 2282 {
			t.Fatalf("invalid Paladin mana during native APL: %+v", event)
		}
		switch event.kind {
		case "reset":
			resets++
			if event.mana != 210 || event.fsr != 0 || event.seal || event.oom {
				t.Fatalf("Paladin seal/mana/FSR/OOM leaked across reset: %+v", event)
			}
		case "seal":
			seals++
			if event.seal {
				t.Fatal("native APL spent mana refreshing an already active seal")
			}
		case "judge":
			judgements++
			if !event.seal {
				t.Fatal("native APL judged without a seal")
			}
			if previous, ok := lastJudge[event.seed]; ok && event.at-previous < 10*time.Second {
				t.Fatal("native APL bypassed Judgement's cooldown")
			}
			lastJudge[event.seed] = event.at
		case "white":
			if event.at != time.Duration(swings%31)*3*time.Second {
				t.Fatalf("seal, judgement or OOM interrupted the white swing timer: %+v", event)
			}
			swings++
			if event.oom {
				oomSwings++
			}
		}
	}
	if resets != 3 || seals < 9 || judgements < 6 || swings != 93 || oomSwings == 0 || player.SecondsOomAvg <= 0 {
		t.Fatalf("incomplete native APL/OOM recovery: resets %d seals %d judgements %d swings %d OOM swings %d OOM seconds %g",
			resets, seals, judgements, swings, oomSwings, player.SecondsOomAvg)
	}
	resources := map[int32]*proto.ResourceMetrics{}
	for _, resource := range player.Resources {
		resources[resource.Id.GetSpellId()] = resource
	}
	for _, cost := range []struct {
		id    int32
		count int
		cost  float64
	}{{20920, seals, 210}, {20271, judgements, 90.72}} {
		resource := resources[cost.id]
		if resource == nil || int(resource.Events) != cost.count {
			t.Fatalf("spell %d mana report lost casts", cost.id)
		}
		assertFloat64(t, "native APL resource report preserves exact costs", resource.ActualGain, -float64(cost.count)*cost.cost)
	}
	if agent.GetStats() != initialStats || currentRuleset() != active || CharacterLevel != 70 {
		t.Fatal("Paladin reference changed its baseline or selected public TBC rules")
	}
	repeat, repeatedAgent := newClassic60PaladinTestSim(t, config)
	classic60MeleeTestAssertResultEqual(t, result, repeat.run())
	if !reflect.DeepEqual(agent.events, repeatedAgent.events) {
		t.Fatal("native Paladin APL trace differs with identical seeds")
	}
}

func TestClassic60PaladinPendingCommandCleanupAcrossIterations(t *testing.T) {
	config := classic60PaladinTestConfig{duration: 5 * time.Millisecond, iterations: 3, seed: 79, pauseAutos: true}
	sim, agent := newClassic60PaladinTestSim(t, config, func(agent *classic60PaladinTestAgent) {
		classic60PaladinAt(agent, 0, func(sim *Simulation) {
			classic60PaladinRolls(t, sim, []float64{.5, .99, .99, .99}, func() {
				agent.spells.CommandProc.Cast(sim, agent.CurrentTarget)
			})
		})
	})
	classic60PaladinResult(t, sim.run())
	casts := 0
	for _, event := range agent.events {
		if event.kind == "proc_hit" {
			t.Fatal("a 10ms Command projectile landed after the 5ms encounter ended")
		}
		if event.kind == "proc_cast" {
			casts++
		}
	}
	if casts != 3 {
		t.Fatalf("deferred Command lost or duplicated casts across reset: %d", casts)
	}
	if result := agent.spells.CommandProc.resultCache[agent.CurrentTarget]; result == nil || result.inUse {
		t.Fatal("cancelled delayed Command retained an in-use damage result")
	}
}
