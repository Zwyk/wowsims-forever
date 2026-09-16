package core

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/stats"
)

func TestClassic60PaladinSelfBlessingsSwitchRefreshAndReset(t *testing.T) {
	checks := 0
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 310 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		talents := classic60PaladinTalents{}
		talents[classic60PaladinImprovedBlessingOfMight] = 3
		talents[classic60PaladinImprovedBlessingOfWisdom] = 1
		agent.spells.registerSelfBuffs(&agent.Character, talents)
		var manaAfterWisdom float64
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if agent.spells.MightAura.IsActive() || agent.spells.WisdomAura.IsActive() {
				t.Fatal("self blessings leaked into the next iteration")
			}
			assertFloat64(t, "baseline AP before blessing", agent.GetStat(stats.AttackPower), 370)
			assertFloat64(t, "baseline MP5 before blessing", agent.GetStat(stats.MP5), 0)
			if agent.spells.BlessingOfMight.CanCast(sim, agent.CurrentTarget) {
				t.Fatal("self-only blessing accepted an enemy target")
			}
			before := agent.CurrentMana()
			if !agent.spells.BlessingOfMight.Cast(sim, &agent.Unit) {
				t.Fatal("ready Might failed")
			}
			assertFloat64(t, "Might trainer rank cost", before-agent.CurrentMana(), 110)
			assertFloat64(t, "Might with three talent ranks floors AP", agent.GetStat(stats.AttackPower), 543)
			if agent.NextGCDAt() != 2500*time.Millisecond || agent.spells.MightAura.ExpiresAt() != 301*time.Second {
				t.Fatal("Might lost its 1.5-second GCD or five-minute duration")
			}
		})
		classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
			if !agent.spells.BlessingOfMight.Cast(sim, &agent.Unit) {
				t.Fatal("Might refresh failed")
			}
			assertFloat64(t, "Might refresh does not stack", agent.GetStat(stats.AttackPower), 543)
			if agent.spells.MightAura.ExpiresAt() != 303*time.Second {
				t.Fatal("Might refresh did not extend expiration")
			}
		})
		classic60PaladinAt(agent, 5*time.Second, func(sim *Simulation) {
			before := agent.CurrentMana()
			if !agent.spells.BlessingOfWisdom.Cast(sim, &agent.Unit) || agent.spells.MightAura.IsActive() {
				t.Fatal("Wisdom did not replace this Paladin's Might")
			}
			assertFloat64(t, "Wisdom trainer rank cost", before-agent.CurrentMana(), 115)
			assertFloat64(t, "switching removes Might", agent.GetStat(stats.AttackPower), 370)
			assertFloat64(t, "Wisdom talent rank", agent.GetStat(stats.MP5), 33)
			assertFloat64(t, "Wisdom immediately updates casting regeneration", agent.ManaRegenPerSecondWhileCasting(), 6.6)
			manaAfterWisdom = agent.CurrentMana()
		})
		classic60PaladinAt(agent, 6100*time.Millisecond, func(_ *Simulation) {
			assertFloat64(t, "Wisdom contributes to the actual six-second mana tick", agent.CurrentMana()-manaAfterWisdom, 13.2)
		})
		classic60PaladinAt(agent, 7*time.Second, func(sim *Simulation) {
			if !agent.spells.BlessingOfMight.Cast(sim, &agent.Unit) || agent.spells.WisdomAura.IsActive() {
				t.Fatal("Might did not replace Wisdom")
			}
			assertFloat64(t, "Wisdom removal updates MP5", agent.GetStat(stats.MP5), 0)
			assertFloat64(t, "Wisdom removal updates casting regeneration", agent.ManaRegenPerSecondWhileCasting(), 0)
		})
		classic60PaladinAt(agent, 307*time.Second+time.Nanosecond, func(_ *Simulation) {
			if agent.spells.MightAura.IsActive() {
				t.Fatal("Might did not expire after five minutes")
			}
			assertFloat64(t, "expiration removes Might AP", agent.GetStat(stats.AttackPower), 370)
			checks++
		})
	})
	classic60PaladinResult(t, sim.run())
	if checks != 2 {
		t.Fatalf("validated %d iterations, want 2", checks)
	}
}

func TestClassic60PaladinKingsDependenciesAndMana(t *testing.T) {
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 8 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		talents := classic60PaladinTalents{}
		talents[classic60PaladinBlessingOfKings] = 1
		agent.spells.registerSelfBuffs(&agent.Character, talents)
		var baseline stats.Stats
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			baseline = agent.GetStats()
			assertFloat64(t, "Kings reset restores base mana pool", agent.MaxMana(), 2282)
			before := agent.CurrentMana()
			if !agent.spells.BlessingOfKings.Cast(sim, &agent.Unit) {
				t.Fatal("learned Kings failed")
			}
			assertFloat64(t, "Kings retains its exact 8% cost", before-agent.CurrentMana(), 120.96)
			for _, stat := range []stats.Stat{stats.Strength, stats.Agility, stats.Stamina, stats.Intellect, stats.Spirit} {
				assertFloat64(t, "Kings attribute "+stat.StatName(), agent.GetStat(stat), baseline[stat]*1.1)
			}
			assertFloat64(t, "Kings recomputes AP from multiplied Strength", agent.GetStat(stats.AttackPower), 370+2*((baseline[stats.Strength]*1.1)-baseline[stats.Strength]))
			assertFloat64(t, "Kings recomputes mana from multiplied Intellect", agent.MaxMana(), 2282+15*((baseline[stats.Intellect]*1.1)-baseline[stats.Intellect]))
			assertFloat64(t, "Kings updates Spirit regeneration", agent.SpiritManaRegenPerSecond(), 7.5+(baseline[stats.Spirit]*1.1)/10)
		})
		classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
			before := agent.GetStats()
			if !agent.spells.BlessingOfKings.Cast(sim, &agent.Unit) {
				t.Fatal("Kings refresh failed")
			}
			if agent.GetStats() != before {
				t.Fatal("Kings refresh stacked attribute multipliers")
			}
		})
		classic60PaladinAt(agent, 5*time.Second, func(sim *Simulation) {
			if !agent.spells.BlessingOfWisdom.Cast(sim, &agent.Unit) || agent.spells.KingsAura.IsActive() {
				t.Fatal("Wisdom did not replace Kings")
			}
			want := baseline
			want[stats.MP5] = 30
			if got := agent.GetStats(); got != want {
				t.Fatalf("Kings removal did not restore dependencies: difference %v", got.Subtract(want))
			}
			assertFloat64(t, "Kings removal restores Spirit regeneration", agent.SpiritManaRegenPerSecond(), 15)
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinSelfBuffTalentRanksAndAuraSelection(t *testing.T) {
	for rank := uint8(0); rank <= 5; rank++ {
		for _, selection := range []string{"none", "devotion", "sanctity"} {
			t.Run(fmt.Sprintf("rank%d/%s", rank, selection), func(t *testing.T) {
				var baselineArmor float64
				sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 7 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
					talents := classic60PaladinTalents{}
					talents[classic60PaladinImprovedBlessingOfMight] = rank
					talents[classic60PaladinImprovedBlessingOfWisdom] = min(rank, 2)
					talents[classic60PaladinImprovedDevotionAura] = rank
					talents[classic60PaladinSanctityAura] = 1
					agent.spells.registerSelfBuffs(&agent.Character, talents)
					if agent.spells.BlessingOfKings != nil || agent.spells.KingsAura != nil {
						t.Fatal("Kings became available without its talent")
					}
					if agent.GetSpell(ActionID{SpellID: 10293}) != nil || agent.GetSpell(ActionID{SpellID: 20218}) != nil {
						t.Fatal("configuration-only auras exposed unaudited castable spells")
					}
					if err := agent.spells.selectReferenceAura(agent.spells.MightAura); err == nil {
						t.Fatal("a blessing was accepted as a Paladin aura")
					}
					if err := agent.spells.selectReferenceAura(&Aura{Label: "foreign aura"}); err == nil {
						t.Fatal("an unowned aura was accepted")
					}
					var selected *Aura
					switch selection {
					case "devotion":
						selected = agent.spells.DevotionEffect
					case "sanctity":
						selected = agent.spells.SanctityEffect
					}
					// Configuring, clearing and reselecting does not activate an
					// effect early or stack it on successive simulation resets.
					for _, aura := range []*Aura{selected, nil, selected, selected} {
						if err := agent.spells.selectReferenceAura(aura); err != nil {
							t.Fatal(err)
						}
					}
					if agent.spells.DevotionEffect.IsActive() || agent.spells.SanctityEffect.IsActive() {
						t.Fatal("aura selection activated before simulation reset")
					}
					classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
						if agent.spells.DevotionEffect.IsActive() != (selection == "devotion") || agent.spells.SanctityEffect.IsActive() != (selection == "sanctity") {
							t.Fatal("reset did not activate exactly the selected aura")
						}
						wantArmor, wantHoly := baselineArmor, 1.0
						if selection == "devotion" {
							wantArmor += 735 * (1 + 0.05*float64(rank))
						}
						if selection == "sanctity" {
							wantHoly = 1.1
						}
						assertFloat64(t, "selected Devotion trainer rank and talent", agent.GetStat(stats.Armor), wantArmor)
						assertFloat64(t, "selected Sanctity Holy multiplier", agent.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly], wantHoly)
						assertFloat64(t, "Sanctity excludes Physical damage", agent.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexPhysical], 1)
						if !agent.spells.BlessingOfMight.Cast(sim, &agent.Unit) {
							t.Fatal("Might failed")
						}
						assertFloat64(t, "Might rank scaling", agent.GetStat(stats.AttackPower), 370+math.Floor(155*(1+0.04*float64(rank))))
					})
					classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
						if !agent.spells.BlessingOfWisdom.Cast(sim, &agent.Unit) {
							t.Fatal("Wisdom failed")
						}
						assertFloat64(t, "Wisdom rank scaling", agent.GetStat(stats.MP5), 30*(1+0.1*float64(min(rank, 2))))
					})
				})
				baselineArmor = agent.GetStat(stats.Armor)
				other := agent.spells.DevotionEffect
				if selection == "devotion" {
					other = agent.spells.SanctityEffect
				}
				if err := agent.spells.selectReferenceAura(other); err == nil {
					t.Fatal("aura change was allowed after finalization")
				}
				classic60PaladinResult(t, sim.run())
			})
		}
	}
}

func TestClassic60PaladinSanctityRequiresTalent(t *testing.T) {
	_, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{}, func(agent *classic60PaladinTestAgent) {
		agent.spells.registerSelfBuffs(&agent.Character, classic60PaladinTalents{})
	})
	if agent.spells.SanctityEffect != nil {
		t.Fatal("Sanctity became available without its talent")
	}
}
