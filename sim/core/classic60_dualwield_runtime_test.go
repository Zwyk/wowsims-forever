package core

import (
	"math"
	"testing"
	"time"
)

func TestClassic60DualWieldRuntimeSchedulerResetAndReproducibility(t *testing.T) {
	config := classic60MeleeTestConfig{
		targetLevel: 63, dualWield: true,
		duration: 60 * time.Second, iterations: 3, seed: 724,
	}
	sim, _, swings := newClassic60MeleeTestSim(t, config)
	result := sim.run()
	mh := classic60MeleeTestMetricsForHand(t, result, 1)
	oh := classic60MeleeTestMetricsForHand(t, result, 2)
	// MH: 0, 2, ... 60. The modern scheduler starts the 3-second OH weapon
	// half a swing later: 1.5, 4.5, ... 58.5. Both schedules reset each pull.
	if mh.Casts != 93 || oh.Casts != 60 || len(*swings) != 153 || result.IterationsDone != 3 {
		t.Fatalf("scheduled MH/OH/recorded swings/iterations = %d/%d/%d/%d", mh.Casts, oh.Casts, len(*swings), result.IterationsDone)
	}
	counts := map[WeaponAttackSource]int{}
	damage := map[WeaponAttackSource]float64{}
	for _, swing := range *swings {
		index := counts[swing.source]
		var wantAt time.Duration
		var wantSeed int64
		switch swing.source {
		case WeaponAttackSourceMainHand:
			wantAt = time.Duration(index%31) * 2 * time.Second
			wantSeed = config.seed + int64(index/31)
		case WeaponAttackSourceOffHand:
			wantAt = 1500*time.Millisecond + time.Duration(index%20)*3*time.Second
			wantSeed = config.seed + int64(index/20)
		default:
			t.Fatalf("unexpected scheduled weapon source %v", swing.source)
		}
		if swing.at != wantAt || swing.seed != wantSeed {
			t.Fatalf("hand %v swing %d did not reset its schedule: %+v", swing.source, index, swing)
		}
		if math.IsNaN(swing.damage) || math.IsInf(swing.damage, 0) || swing.damage < 0 {
			t.Fatalf("invalid scheduled damage: %+v", swing)
		}
		counts[swing.source]++
		damage[swing.source] += swing.damage
	}
	assertFloat64(t, "collected main-hand damage", mh.Damage, damage[WeaponAttackSourceMainHand])
	assertFloat64(t, "collected off-hand damage", oh.Damage, damage[WeaponAttackSourceOffHand])
	assertFloat64(t, "raid DPS from both hands", result.RaidMetrics.Dps.Avg, (mh.Damage+oh.Damage)/180)
	repeat, _, repeatSwings := newClassic60MeleeTestSim(t, config)
	classic60MeleeTestAssertResultEqual(t, result, repeat.run())
	classic60MeleeTestAssertSwingsEqual(t, *swings, *repeatSwings)
}

func TestClassic60DualWieldRuntimeMissPenaltyAndHitSuppression(t *testing.T) {
	// The 19-point white penalty applies to both weapons. Only the weapon
	// whose defense deficit exceeds ten loses the first point of hit.
	tests := []struct {
		name                  string
		mhSkill, ohSkill, hit float64
		mhMiss, ohMiss        float64
	}{
		{"no skill bonus", 0, 0, 0, .27, .27},
		{"main-hand skill bonus", 5, 0, 0, .25, .27},
		{"off-hand skill bonus", 0, 5, 0, .27, .25},
		{"first hit point", 5, 0, 1, .24, .27},
		{"second hit point", 5, 0, 2, .23, .26},
		{"skilled hand capped", 5, 0, 25, 0, .03},
		{"both hands capped", 5, 0, 28, 0, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, agent, _ := newClassic60MeleeTestSim(t, classic60MeleeTestConfig{
				targetLevel: 63, dualWield: true, skillBonus: test.mhSkill,
				offHandSkillBonus: test.ohSkill, hitPercent: test.hit,
				duration: time.Second, iterations: 1, seed: 724,
			})
			table := agent.AttackTables[0]
			assertFloat64(t, "main-hand white miss", agent.AutoAttacks.MHAuto().GetPhysicalMissChance(table), test.mhMiss)
			assertFloat64(t, "off-hand white miss", agent.AutoAttacks.OHAuto().GetPhysicalMissChance(table), test.ohMiss)
		})
	}
}

func TestClassic60DualWieldRuntimeIndependentWeaponSkills(t *testing.T) {
	config := classic60MeleeTestConfig{
		targetLevel: 63, dualWield: true,
		duration: 30000 * time.Second, iterations: 1, seed: 9862,
	}
	baseline, _, baselineSwings := newClassic60MeleeTestSim(t, config)
	baselineResult := baseline.run()
	if classic60MeleeTestMetricsForHand(t, baselineResult, 1).Casts != 15001 ||
		classic60MeleeTestMetricsForHand(t, baselineResult, 2).Casts != 10000 {
		t.Fatal("baseline did not schedule the expected samples for both hands")
	}
	for _, improvedHand := range []WeaponAttackSource{WeaponAttackSourceMainHand, WeaponAttackSourceOffHand} {
		name := "main-hand swords"
		config.skillBonus, config.offHandSkillBonus = 5, 0
		if improvedHand == WeaponAttackSourceOffHand {
			name = "off-hand axes"
			config.skillBonus, config.offHandSkillBonus = 0, 5
		}
		t.Run(name, func(t *testing.T) {
			sim, _, swings := newClassic60MeleeTestSim(t, config)
			result := sim.run()
			if len(*swings) != len(*baselineSwings) {
				t.Fatal("weapon skill changed the scheduled swing count")
			}
			var changed bool
			for i, swing := range *swings {
				baseline := (*baselineSwings)[i]
				if swing.source != baseline.source || swing.seed != baseline.seed || swing.at != baseline.at {
					t.Fatalf("weapon skill changed the schedule at swing %d", i)
				}
				if swing.source != improvedHand && swing != baseline {
					t.Fatalf("skill for hand %v changed the other hand at swing %d", improvedHand, i)
				}
				changed = changed || swing != baseline
			}
			if !changed {
				t.Fatal("weapon skill had no effect on its own scheduled attacks")
			}
			for _, hand := range []WeaponAttackSource{WeaponAttackSourceMainHand, WeaponAttackSourceOffHand} {
				tag, sampleCount := int32(1), int32(15001)
				baseDamage := 100 + 2.0*400/14
				if hand == WeaponAttackSourceOffHand {
					tag, sampleCount = 2, 10000
					// The off-hand penalty halves both raw weapon damage and AP
					// contribution; it is not half of the main-hand weapon's damage.
					baseDamage = .5 * (80 + 3.0*400/14)
				}
				miss, dodge, glanceMin, glanceMax := .27, .065, .55, .75
				if hand == improvedHand {
					miss, dodge, glanceMin, glanceMax = .25, .06, .80, .90
				}
				metrics := classic60MeleeTestMetricsForHand(t, result, tag)
				if metrics.Casts != sampleCount || metrics.Crits != 0 || metrics.Parries != 0 || metrics.Blocks != 0 || metrics.Crushes != 0 ||
					metrics.Hits+metrics.Misses+metrics.Dodges+metrics.Glances != metrics.Casts {
					t.Fatalf("hand %v produced unsupported outcomes or a wrong sample count", hand)
				}
				for _, count := range []struct {
					name   string
					got    int32
					chance float64
				}{
					{"miss", metrics.Misses, miss}, {"dodge", metrics.Dodges, dodge},
					{"glance", metrics.Glances, .40}, {"hit", metrics.Hits, 1 - miss - dodge - .40},
				} {
					want := float64(sampleCount) * count.chance
					limit := 6*math.Sqrt(float64(sampleCount)*count.chance*(1-count.chance)) + 1
					if math.Abs(float64(count.got)-want) > limit {
						t.Errorf("hand %v %s count %d, expected %.1f within %.1f", hand, count.name, count.got, want, limit)
					}
				}
				low, high := math.Inf(1), math.Inf(-1)
				var glanceTotal float64
				for _, swing := range *swings {
					if swing.source != hand {
						continue
					}
					switch swing.outcome {
					case OutcomeGlance:
						multiplier := swing.damage / baseDamage
						if multiplier < glanceMin-1e-12 || multiplier > glanceMax+1e-12 {
							t.Fatalf("hand %v glance multiplier %g outside [%g, %g]", hand, multiplier, glanceMin, glanceMax)
						}
						low, high = min(low, multiplier), max(high, multiplier)
						glanceTotal += multiplier
					case OutcomeHit:
						assertFloat64(t, "scheduled per-hand normal hit", swing.damage, baseDamage)
					case OutcomeMiss, OutcomeDodge:
						assertFloat64(t, "scheduled avoided hit", swing.damage, 0)
					default:
						t.Fatalf("unsupported hand %v outcome %v", hand, swing.outcome)
					}
				}
				width := glanceMax - glanceMin
				if high-low < .8*width {
					t.Fatalf("hand %v glances failed to sample their source range: [%g, %g]", hand, low, high)
				}
				meanLimit := 6 * width / math.Sqrt(12*float64(metrics.Glances))
				if math.Abs(glanceTotal/float64(metrics.Glances)-(glanceMin+glanceMax)/2) > meanLimit {
					t.Fatalf("hand %v glancing damage mean differs from its Classic range", hand)
				}
			}
		})
	}
}

func TestClassic60DualWieldRuntimeArmorScalesBothHands(t *testing.T) {
	config := classic60MeleeTestConfig{
		targetLevel: 63, dualWield: true, skillBonus: 5,
		duration: 600 * time.Second, iterations: 2, seed: 5248,
	}
	unarmored, _, plainSwings := newClassic60MeleeTestSim(t, config)
	plainResult := unarmored.run()
	config.armor = 3000
	armored, _, armoredSwings := newClassic60MeleeTestSim(t, config)
	armoredResult := armored.run()
	if len(*plainSwings) != len(*armoredSwings) {
		t.Fatal("armor changed the scheduled swing count")
	}
	const modifier = 5500.0 / 8500
	for i, plain := range *plainSwings {
		withArmor := (*armoredSwings)[i]
		if plain.source != withArmor.source || plain.at != withArmor.at || plain.seed != withArmor.seed || plain.outcome != withArmor.outcome {
			t.Fatalf("armor changed the scheduled outcome at swing %d", i)
		}
		assertFloat64(t, "scheduled per-hand armor multiplier", withArmor.damage, plain.damage*modifier)
	}
	for _, tag := range []int32{1, 2} {
		plain := classic60MeleeTestMetricsForHand(t, plainResult, tag)
		withArmor := classic60MeleeTestMetricsForHand(t, armoredResult, tag)
		if plain.Damage <= 0 || plain.Casts != withArmor.Casts {
			t.Fatalf("hand %d did not produce matching positive armor samples", tag)
		}
		assertFloat64(t, "aggregate per-hand armor multiplier", withArmor.Damage/plain.Damage, modifier)
	}
}
