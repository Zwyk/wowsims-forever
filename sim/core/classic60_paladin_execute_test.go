package core

import (
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func classic60PaladinExecuteAt(agent *classic60PaladinTestAgent, proportion float64) {
	// Equal boundaries let the real event loop cross directly into execute.
	agent.Env.Encounter.ExecuteProportion_90 = proportion
	agent.Env.Encounter.ExecuteProportion_45 = proportion
	agent.Env.Encounter.ExecuteProportion_35 = proportion
	agent.Env.Encounter.ExecuteProportion_25 = proportion
	agent.Env.Encounter.ExecuteProportion_20 = proportion
}

func TestClassic60PaladinHammerExecuteCastCostAndCooldown(t *testing.T) {
	var hits []time.Duration
	sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{
		duration: 12 * time.Second, pauseAutos: true,
	}, func(agent *classic60PaladinTestAgent) {
		agent.spells.registerHammerOfWrath(&agent.Character, classic60PaladinTalents{})
		classic60PaladinExecuteAt(agent, .75)
		hammer := agent.spells.HammerOfWrath
		agent.RegisterAura(Aura{Label: "Observe Hammer cast completion", Duration: NeverExpires,
			OnReset: func(aura *Aura, sim *Simulation) { aura.Activate(sim) },
			OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, _ *SpellResult) {
				if spell == hammer {
					hits = append(hits, sim.CurrentTime)
				}
			},
		})
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if hammer.CanCast(sim, agent.CurrentTarget) || hammer.Cast(sim, agent.CurrentTarget) ||
				agent.CurrentMana() != agent.MaxMana() || !agent.GCD.IsReady(sim) || !hammer.CD.IsReady(sim) {
				t.Fatal("Hammer cast before the 20% execute phase or spent resources on rejection")
			}
		})
		classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
			if !sim.IsExecutePhase20() || !hammer.Cast(sim, agent.CurrentTarget) {
				t.Fatal("Hammer did not start at the real execute boundary")
			}
		})
		classic60PaladinAt(agent, 3500*time.Millisecond, func(_ *Simulation) {
			if len(hits) != 0 || agent.CurrentMana() != agent.MaxMana() || agent.Hardcast.Expires != 4*time.Second || agent.GCD.ReadyAt() != 4*time.Second {
				t.Fatal("Hammer's one-second cast/GCD or mana-spend timing changed")
			}
		})
		classic60PaladinAt(agent, 4*time.Second+time.Nanosecond, func(_ *Simulation) {
			assertFloat64(t, "Hammer spends 425 mana on completion", agent.CurrentMana(), agent.MaxMana()-425)
			if hammer.CD.ReadyAt() != 10*time.Second {
				t.Fatal("Hammer cooldown must start at completion and last six seconds")
			}
		})
		classic60PaladinAt(agent, 9*time.Second, func(sim *Simulation) {
			if hammer.CanCast(sim, agent.CurrentTarget) || hammer.Cast(sim, agent.CurrentTarget) {
				t.Fatal("Hammer bypassed its cooldown")
			}
		})
		classic60PaladinAt(agent, 10*time.Second, func(sim *Simulation) {
			if !hammer.Cast(sim, agent.CurrentTarget) {
				t.Fatal("Hammer failed at its cooldown boundary")
			}
		})
	})
	hammer := agent.spells.HammerOfWrath
	if hammer.SpellID != 24239 || hammer.Rank != 3 || hammer.DefenseType != DefenseTypeRanged ||
		hammer.ProcMask != ProcMaskRangedSpecial || hammer.DefaultCast.CastTime != time.Second ||
		hammer.DefaultCast.GCD != time.Second || hammer.CD.Duration != 6*time.Second || hammer.MaxRange != 30 {
		t.Fatal("rank-3 Hammer configuration differs from the Classic reference")
	}
	assertFloat64(t, "rank-3 Hammer mana cost", hammer.DefaultCast.Cost, 425)
	classic60PaladinResult(t, sim.run())
	if len(hits) != 2 || hits[0] != 4*time.Second || hits[1] != 11*time.Second {
		t.Fatalf("Hammer did not complete at the expected times: %v", hits)
	}
}

func TestClassic60PaladinHammerDamageUsesRangedOutcomes(t *testing.T) {
	for _, test := range []struct {
		name                string
		spellPower, partial float64
		miss, crit          bool
	}{
		{name: "base", partial: .99},
		{name: "spell_power", spellPower: 100, partial: .99},
		{name: "physical_crit", spellPower: 100, partial: .99, crit: true},
		{name: "physical_miss", spellPower: 100, partial: .99, miss: true},
		{name: "holy_partial_resist", spellPower: 100, partial: .1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var damage float64
			var outcome HitOutcome
			hits := 0
			sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{
				duration: 2 * time.Second, pauseAutos: true, spellDamage: test.spellPower, physicalCrit: 10,
			}, func(agent *classic60PaladinTestAgent) {
				agent.spells.registerHammerOfWrath(&agent.Character, classic60PaladinTalents{})
				classic60PaladinExecuteAt(agent, 1)
				hammer := agent.spells.HammerOfWrath
				apply := hammer.ApplyEffects
				hammer.ApplyEffects = func(sim *Simulation, target *Unit, spell *Spell) {
					rolls := []float64{.5, test.partial, .99}
					if test.miss {
						rolls[2] = 0
					} else if test.crit {
						rolls = append(rolls, 0)
					} else {
						rolls = append(rolls, .99)
					}
					classic60PaladinRolls(t, sim, rolls, func() { apply(sim, target, spell) })
				}
				agent.RegisterAura(Aura{Label: "Observe Hammer outcome", Duration: NeverExpires,
					OnReset: func(aura *Aura, sim *Simulation) { aura.Activate(sim) },
					OnSpellHitDealt: func(_ *Aura, _ *Simulation, spell *Spell, result *SpellResult) {
						if spell == hammer {
							hits++
							damage, outcome = result.Damage, result.Outcome
						}
					},
				})
				classic60PaladinAt(agent, time.Millisecond, func(sim *Simulation) {
					if !hammer.Cast(sim, agent.CurrentTarget) {
						t.Fatal("Hammer failed in execute")
					}
				})
			})
			classic60PaladinResult(t, sim.run())
			want := 530 + .429*test.spellPower // midpoint of corrected 504–556
			if test.crit {
				want *= 2
			}
			if test.miss {
				want = 0
			}
			if test.partial == .1 {
				want *= .75
			}
			assertFloat64(t, "Hammer damage", damage, want)
			if hits != 1 || outcome.Matches(OutcomeCrit) != test.crit || outcome.Matches(OutcomeMiss) != test.miss ||
				outcome.Matches(OutcomeDodge|OutcomeParry|OutcomeGlance|OutcomeBlock) {
				t.Fatalf("Hammer used an incorrect ranged result: hits %d, outcome %v", hits, outcome)
			}
		})
	}
}

func TestClassic60PaladinHammerExcludesPrecisionAndMeleeWeaponSkill(t *testing.T) {
	_, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{}, func(agent *classic60PaladinTestAgent) {
		agent.AddStat(stats.PhysicalHitPercent, 3)
		agent.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedSwords, 5)
		agent.spells.registerHammerOfWrath(&agent.Character, classic60PaladinTalents{
			classic60PaladinPrecision: 3, classic60PaladinHolyPower: 5,
		})
	})
	hammer, table := agent.spells.HammerOfWrath, agent.AttackTables[agent.CurrentTarget.UnitIndex]
	view := liveClassic60PaladinRangedView(hammer, table)
	assertFloat64(t, "Hammer uses zero ranged skill bonus", view.baseMissChance, .08)
	assertFloat64(t, "Hammer keeps zero-skill hit suppression", view.hitSuppression, .01)
	assertFloat64(t, "Precision is excluded from ranged hit", hammer.PhysicalHitChance(table), 0)
	assertFloat64(t, "Hammer physical miss against +3", hammer.GetPhysicalMissChance(table), .08)
	assertFloat64(t, "Holy Power does not alter ranged crit", hammer.BonusCritPercent, 0)
	assertFloat64(t, "Hammer crit multiplier", hammer.CritDamageMultiplier(table), 2)
}
