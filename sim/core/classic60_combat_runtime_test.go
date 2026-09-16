package core

import (
	"math"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/simsignals"
	"github.com/wowsims/tbc/sim/core/stats"
	googleProto "google.golang.org/protobuf/proto"
)

type classic60MeleeTestConfig struct {
	targetLevel       int32
	armor             float64
	skillBonus        float64
	offHandSkillBonus float64
	dualWield         bool
	hitPercent        float64
	duration          time.Duration
	iterations        int32
	seed              int64
}

type classic60MeleeTestSwing struct {
	seed    int64
	at      time.Duration
	outcome HitOutcome
	damage  float64
	source  WeaponAttackSource
}

// This fixture deliberately bypasses agent factories and applyCharacterEffects.
// It runs real auto-attacks through the modern environment, reset, event loop,
// spell outcomes and metrics, without importing a TBC class, racial, talent,
// resource bar, item effect or encounter AI. The in-memory sword and optional
// off-hand axe supply only classified weapons and fixed damage; neither is
// fetched from a database.
// Pre-racial Human Warrior primary stats are integral, so finalization's modern
// primary-stat floor does not alter this narrowly supported Classic baseline.
func newClassic60MeleeTestSim(t *testing.T, config classic60MeleeTestConfig, configure ...func(*FakeAgent)) (*Simulation, *FakeAgent, *[]classic60MeleeTestSwing) {
	t.Helper()
	rules := classic60MeleeReferenceRules()
	baseline, ok := classicReferenceInitializeCharacter(60, proto.Race_RaceHuman, proto.Class_ClassWarrior)
	if !ok {
		t.Fatal("missing level-60 Human Warrior baseline")
	}
	raid := NewRaid(&proto.Raid{})
	party := NewParty(raid, 0, &proto.Party{}, &proto.Raid{})
	raid.Parties = []*Party{party}
	agent := &FakeAgent{Character: newCharacterWithRuleset(party, 0, &proto.Player{
		Name: "Classic melee fixture", Race: proto.Race_RaceHuman, Class: proto.Class_ClassWarrior,
		Spec: &proto.Player_DpsWarrior{}, Equipment: &proto.EquipmentSpec{},
	}, rules, baseline.baseStats)}
	agent.AddBaseClassStatDependencies()
	agent.AddStat(stats.PhysicalHitPercent, config.hitPercent)
	agent.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{
		ID: -1, Type: proto.ItemType_ItemTypeWeapon,
		WeaponType: proto.WeaponType_WeaponTypeSword, HandType: proto.HandType_HandTypeOneHand,
		WeaponDamageMin: 100, WeaponDamageMax: 100, SwingSpeed: 2,
	}
	agent.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategorySwords, config.skillBonus)
	autoAttacks := AutoAttackOptions{MainHand: agent.WeaponFromMainHand(), AutoSwingMelee: true}
	if config.dualWield {
		agent.Equipment[proto.ItemSlot_ItemSlotOffHand] = Item{
			ID: -2, Type: proto.ItemType_ItemTypeWeapon,
			WeaponType: proto.WeaponType_WeaponTypeAxe, HandType: proto.HandType_HandTypeOneHand,
			WeaponDamageMin: 80, WeaponDamageMax: 80, SwingSpeed: 3,
		}
		agent.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryAxes, config.offHandSkillBonus)
		autoAttacks.OffHand = agent.WeaponFromOffHand()
	}
	agent.EnableAutoAttacks(agent, autoAttacks)
	agent.AutoAttacks.RandomMeleeOffset = false
	party.Players = []Agent{agent}
	raid.updatePlayersAndPets()

	targetConfig := &proto.Target{Level: config.targetLevel, Stats: stats.Stats{stats.Armor: config.armor}.ToProtoArray()}
	target := newTargetWithRuleset(targetConfig, 0, rules)
	// This passive fixture has no shield/block damage model. Remove the live
	// target constructor's inherited flat block-value seed, even though rear
	// attacks cannot use it.
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
		unit.Env = env
		unit.UnitIndex = int32(index)
	}
	agent.CurrentTarget = &target.Unit
	target.initialize(targetConfig)
	agent.initialize(agent)
	for _, configureAgent := range configure {
		configureAgent(agent)
	}

	swings := []classic60MeleeTestSwing{}
	agent.RegisterAura(Aura{
		Label: "Record Classic fixture swings", Duration: NeverExpires,
		OnReset: func(aura *Aura, sim *Simulation) { aura.Activate(sim) },
		OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
			if spell == agent.AutoAttacks.MHAuto() || spell == agent.AutoAttacks.OHAuto() {
				swings = append(swings, classic60MeleeTestSwing{
					seed: sim.currentSeed, at: sim.CurrentTime, outcome: result.Outcome,
					damage: result.Damage, source: spell.weaponAttackSource,
				})
			}
		},
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
	wantStats[stats.PhysicalHitPercent] = config.hitPercent
	if agent.GetStats() != wantStats || agent.HasManaBar() || agent.HasRageBar() {
		t.Fatal("fixture acquired stats or resources outside its Classic baseline")
	}
	for _, unit := range env.AllUnits {
		if unit.resolvedRuleset() != rules || math.IsNaN(unit.PseudoStats.ReducedCritTakenPercent) {
			t.Fatal("fixture lost explicit rules or acquired non-finite defense modifiers")
		}
	}
	return newSimWithEnv(env, &proto.SimOptions{
		Iterations: config.iterations, RandomSeed: config.seed, IsTest: true,
	}, simsignals.CreateSignals()), agent, &swings
}

func classic60MeleeTestMetrics(t *testing.T, result *proto.RaidSimResult) *proto.TargetedActionMetrics {
	t.Helper()
	return classic60MeleeTestMetricsForHand(t, result, 1)
}

func classic60MeleeTestMetricsForHand(t *testing.T, result *proto.RaidSimResult, tag int32) *proto.TargetedActionMetrics {
	t.Helper()
	if result.Error != nil {
		t.Fatalf("simulation failed: %v", result.Error)
	}
	for _, action := range result.RaidMetrics.Parties[0].Players[0].Actions {
		if action.Id.GetOtherId() == proto.OtherAction_OtherActionAttack && action.Id.Tag == tag {
			for _, target := range action.Targets {
				if target.UnitIndex == 0 {
					return target
				}
			}
		}
	}
	t.Fatalf("scheduled hand %d attacks produced no target metrics", tag)
	return nil
}

func classic60MeleeTestAssertResultEqual(t *testing.T, want, got *proto.RaidSimResult) {
	t.Helper()
	// UnitMetrics.ToProto appends Actions while ranging over a map. Preserve
	// exact protobuf equality after ordering only that unordered report field;
	// ordered schedules and every numeric metric remain subject to exact checks.
	canonical := func(result *proto.RaidSimResult) *proto.RaidSimResult {
		cloned := googleProto.Clone(result).(*proto.RaidSimResult)
		var sortActions func(*proto.UnitMetrics)
		sortActions = func(unit *proto.UnitMetrics) {
			keys := make(map[*proto.ActionMetrics]string, len(unit.Actions))
			seen := make(map[string]bool, len(unit.Actions))
			for _, action := range unit.Actions {
				encoded, err := (googleProto.MarshalOptions{Deterministic: true}).Marshal(action.Id)
				if err != nil {
					t.Fatalf("cannot encode reported action ID: %v", err)
				}
				key := string(encoded)
				if seen[key] {
					t.Fatalf("report contains duplicate action ID %v", action.Id)
				}
				seen[key] = true
				keys[action] = key
			}
			sort.Slice(unit.Actions, func(i, j int) bool { return keys[unit.Actions[i]] < keys[unit.Actions[j]] })
			for _, pet := range unit.Pets {
				sortActions(pet)
			}
		}
		for _, party := range cloned.GetRaidMetrics().GetParties() {
			for _, player := range party.Players {
				sortActions(player)
			}
		}
		for _, target := range cloned.GetEncounterMetrics().GetTargets() {
			sortActions(target)
		}
		return cloned
	}
	if !googleProto.Equal(canonical(want), canonical(got)) {
		t.Error("identical seeds produced different report metrics after canonicalizing action order")
	}
}

func classic60MeleeTestAssertSwingsEqual(t *testing.T, want, got []classic60MeleeTestSwing) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		if len(want) != len(got) {
			t.Fatalf("identical seeds changed scheduled swing count: %d != %d", len(want), len(got))
		}
		for i := range want {
			if want[i] != got[i] {
				t.Fatalf("identical seeds changed scheduled swing %d: %+v != %+v", i, want[i], got[i])
			}
		}
		t.Fatal("identical seeds changed the scheduled swing trace")
	}
}

func TestClassic60MeleeRuntimeSchedulerResetAndReproducibility(t *testing.T) {
	active := currentRuleset()
	config := classic60MeleeTestConfig{targetLevel: 63, duration: 60 * time.Second, iterations: 3, seed: 724}
	sim, agent, swings := newClassic60MeleeTestSim(t, config)
	initialStats := agent.GetStats()
	result := sim.run()
	metrics := classic60MeleeTestMetrics(t, result)
	// The scheduler includes attacks at pull and at the inclusive end boundary:
	// 0, 2, ... 60 seconds, then resets the same weapon for the next iteration.
	const swingsPerIteration = 31
	if metrics.Casts != 3*swingsPerIteration || len(*swings) != 3*swingsPerIteration || result.IterationsDone != 3 {
		t.Fatalf("scheduled swings/iterations = %d/%d/%d", metrics.Casts, len(*swings), result.IterationsDone)
	}
	var damage float64
	for i, swing := range *swings {
		if swing.at != time.Duration(i%swingsPerIteration)*2*time.Second || swing.seed != config.seed+int64(i/swingsPerIteration) {
			t.Fatalf("iteration did not reset swing timing or seed: swing %d = %+v", i, swing)
		}
		if math.IsNaN(swing.damage) || math.IsInf(swing.damage, 0) || swing.damage < 0 {
			t.Fatalf("invalid scheduled damage: %+v", swing)
		}
		damage += swing.damage
	}
	assertFloat64(t, "collected swing damage", metrics.Damage, damage)
	assertFloat64(t, "raid DPS from scheduled damage", result.RaidMetrics.Dps.Avg, damage/180)
	if agent.GetStats() != initialStats || currentRuleset() != active || CharacterLevel != 70 {
		t.Fatal("Classic fixture changed its initial stats or the active TBC profile")
	}
	repeat, _, repeatSwings := newClassic60MeleeTestSim(t, config)
	classic60MeleeTestAssertResultEqual(t, result, repeat.run())
	classic60MeleeTestAssertSwingsEqual(t, *swings, *repeatSwings)
}

func TestClassic60MeleeRuntimeSourceDistributions(t *testing.T) {
	// Literal source expectations: capped 300 skill, target defense 300/315;
	// +5/+8 skill changes miss/dodge/glancing damage, not glancing probability.
	// At +3, baseline 4% crit is entirely removed by 4.8% suppression. The first
	// point of hit is ineffective only while the skill deficit exceeds ten.
	tests := []struct {
		name                      string
		level                     int32
		skill, hit                float64
		miss, dodge, glance, crit float64
		glanceMin, glanceMax      float64
	}{
		{"equal level", 60, 0, 0, .05, .05, .10, .04, .91, .99},
		{"boss capped skill", 63, 0, 0, .08, .065, .40, 0, .55, .75},
		{"boss first hit suppressed", 63, 0, 1, .08, .065, .40, 0, .55, .75},
		{"boss second hit effective", 63, 0, 2, .07, .065, .40, 0, .55, .75},
		{"boss five skill", 63, 5, 0, .06, .06, .40, 0, .80, .90},
		{"boss five skill hit cap", 63, 5, 6, 0, .06, .40, 0, .80, .90},
		{"boss eight skill", 63, 8, 0, .057, .057, .40, 0, .91, .99},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sim, agent, swings := newClassic60MeleeTestSim(t, classic60MeleeTestConfig{
				targetLevel: test.level, skillBonus: test.skill, hitPercent: test.hit,
				duration: 20000 * time.Second, iterations: 1, seed: 9862,
			})
			spell := agent.AutoAttacks.MHAuto()
			assertFloat64(t, "live white miss chance", spell.GetPhysicalMissChance(agent.AttackTables[0]), test.miss)
			metrics := classic60MeleeTestMetrics(t, sim.run())
			if metrics.Casts != 10001 || len(*swings) != 10001 {
				t.Fatalf("unexpected scheduled sample count %d/%d", metrics.Casts, len(*swings))
			}
			counts := []struct {
				name   string
				got    int32
				chance float64
			}{
				{"miss", metrics.Misses, test.miss}, {"dodge", metrics.Dodges, test.dodge},
				{"glance", metrics.Glances, test.glance}, {"crit", metrics.Crits, test.crit},
				{"hit", metrics.Hits, 1 - test.miss - test.dodge - test.glance - test.crit},
			}
			for _, count := range counts {
				want := float64(metrics.Casts) * count.chance
				// Six binomial standard deviations plus one discrete observation;
				// the fixed seed makes this deterministic while tolerating harmless
				// RNG stream changes. Zero-probability outcomes must remain absent.
				limit := 6*math.Sqrt(float64(metrics.Casts)*count.chance*(1-count.chance)) + 1
				if math.Abs(float64(count.got)-want) > limit || (count.chance == 0 && count.got != 0) {
					t.Errorf("%s count %d, expected %.1f within %.1f", count.name, count.got, want, limit)
				}
			}
			if metrics.Parries != 0 || metrics.Blocks != 0 || metrics.Crushes != 0 ||
				metrics.Hits+metrics.Crits+metrics.Misses+metrics.Dodges+metrics.Glances != metrics.Casts {
				t.Fatal("rear-facing player attack acquired an unsupported or uncounted outcome")
			}
			const baseDamage = 100 + 2.0*400/14 // Fixed sword plus Classic Human Warrior AP.
			low, high := math.Inf(1), math.Inf(-1)
			var glanceTotal float64
			for _, swing := range *swings {
				switch swing.outcome {
				case OutcomeGlance:
					multiplier := swing.damage / baseDamage
					if multiplier < test.glanceMin-1e-12 || multiplier > test.glanceMax+1e-12 {
						t.Fatalf("glance multiplier %g outside [%g, %g]", multiplier, test.glanceMin, test.glanceMax)
					}
					low, high = min(low, multiplier), max(high, multiplier)
					glanceTotal += multiplier
				case OutcomeHit:
					assertFloat64(t, "scheduled normal hit", swing.damage, baseDamage)
				case OutcomeCrit:
					assertFloat64(t, "scheduled critical hit", swing.damage, 2*baseDamage)
				case OutcomeMiss, OutcomeDodge:
					assertFloat64(t, "scheduled avoided hit", swing.damage, 0)
				default:
					t.Fatalf("unsupported outcome %v", swing.outcome)
				}
			}
			width := test.glanceMax - test.glanceMin
			if high-low < .8*width {
				t.Fatalf("glances failed to sample their source range: [%g, %g]", low, high)
			}
			wantMean := (test.glanceMin + test.glanceMax) / 2
			meanLimit := 6 * width / math.Sqrt(12*float64(metrics.Glances))
			if math.Abs(glanceTotal/float64(metrics.Glances)-wantMean) > meanLimit {
				t.Fatal("scheduled glancing damage mean differs from the uniform Classic source range")
			}
		})
	}
}

func TestClassic60MeleeRuntimeArmorUsesAttackerLevel(t *testing.T) {
	for _, targetLevel := range []int32{60, 63} {
		config := classic60MeleeTestConfig{targetLevel: targetLevel, duration: 600 * time.Second, iterations: 2, seed: 5248}
		unarmored, _, plainSwings := newClassic60MeleeTestSim(t, config)
		plainMetrics := classic60MeleeTestMetrics(t, unarmored.run())
		config.armor = 3000
		armored, _, armoredSwings := newClassic60MeleeTestSim(t, config)
		armoredMetrics := classic60MeleeTestMetrics(t, armored.run())
		if len(*plainSwings) != len(*armoredSwings) || plainMetrics.Damage <= 0 {
			t.Fatal("paired armor simulations did not produce matching positive samples")
		}
		const modifier = 5500.0 / 8500 // Classic attacker60: 400 + 85*60, independent of target level.
		for i, plain := range *plainSwings {
			withArmor := (*armoredSwings)[i]
			if plain.at != withArmor.at || plain.seed != withArmor.seed || plain.outcome != withArmor.outcome {
				t.Fatalf("armor changed the scheduled outcome at swing %d", i)
			}
			assertFloat64(t, "scheduled armor multiplier", withArmor.damage, plain.damage*modifier)
		}
		assertFloat64(t, "aggregate armor multiplier", armoredMetrics.Damage/plainMetrics.Damage, modifier)
	}
}
