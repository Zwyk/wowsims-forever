package core

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/stats"
)

func classic60PaladinIncomingTestSpell(agent *classic60PaladinTestAgent, school SpellSchool, binary bool) *Spell {
	flags := SpellFlagNone
	if binary {
		flags = SpellFlagBinary
	}
	return agent.CurrentTarget.RegisterSpell(SpellConfig{
		ActionID:    ActionID{SpellID: 900001 + int32(school), Tag: int32(flags)},
		SpellSchool: school, DefenseType: DefenseTypeMagic, ProcMask: ProcMaskSpellDamage,
		WeaponAttackSource: WeaponAttackSourceNone,
		Flags:              flags, Cast: CastConfig{IgnoreHaste: true}, DamageMultiplier: 1, ThreatMultiplier: 1,
	})
}

func TestClassic60PaladinIncomingMagicChances(t *testing.T) {
	for level := int32(60); level <= 63; level++ {
		t.Run(fmt.Sprint(level), func(t *testing.T) {
			var incoming *Spell
			_, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: time.Second, targetLevel: level}, func(agent *classic60PaladinTestAgent) {
				agent.spells.classic60PaladinDefenseState = agent.spells.registerDefense(&agent.Character, classic60PaladinTalents{})
				incoming = classic60PaladinIncomingTestSpell(agent, SpellSchoolFire, false)
				incoming.BonusHitPercent, incoming.BonusCritPercent = 1, 5
				agent.CurrentTarget.AddStats(stats.Stats{stats.SpellHitPercent: 1, stats.SpellCritPercent: 10})
				agent.CurrentTarget.PseudoStats.SchoolBonusHitChance[stats.SchoolIndexFire] = 1
			})
			table := incoming.Unit.AttackTables[agent.UnitIndex]
			// Deliberately poison inherited values: the owned policy must derive
			// its own Classic reverse-table values instead of consuming these.
			table.BaseSpellMissChance, table.SpellCritSuppression = .91, .5
			view := liveClassic60PaladinIncomingSpellView(incoming, table)
			assertFloat64(t, "Classic player target base miss", view.baseMissChance, .05)
			assertFloat64(t, "NPC direct and school spell hit", incoming.SpellHitChance(&agent.Unit), .03)
			assertFloat64(t, "NPC spell crit", incoming.SpellCritChance(&agent.Unit), .15)
			assertFloat64(t, "NPC spell miss", incoming.SpellChanceToMiss(table), .02)
			incoming.BonusHitPercent = 100
			assertFloat64(t, "Classic one-percent spell miss floor", incoming.SpellChanceToMiss(table), .01)
		})
	}
}

func TestClassic60PaladinIncomingMagicResistanceAura(t *testing.T) {
	var direct, binary *Spell
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 2 * time.Second, targetLevel: 63, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		agent.spells.classic60PaladinDefenseState = agent.spells.registerDefense(&agent.Character, classic60PaladinTalents{})
		agent.spells.registerSelfBuffs(&agent.Character, classic60PaladinTalents{})
		agent.spells.registerSupport(&agent.Character, classic60PaladinTalents{})
		if err := agent.spells.selectReferenceAura(agent.spells.FireResistanceEffect); err != nil {
			t.Fatal(err)
		}
		agent.AddStat(stats.FireResistance, 45)
		agent.CurrentTarget.AddStat(stats.SpellCritPercent, 25)
		direct = classic60PaladinIncomingTestSpell(agent, SpellSchoolFire, false)
		binary = classic60PaladinIncomingTestSpell(agent, SpellSchoolFire, true)
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			table := direct.Unit.AttackTables[agent.UnitIndex]
			assertFloat64(t, "resistance aura plus explicit resistance", agent.GetStat(stats.FireResistance), 105)
			a, b, c := table.GetPartialResistThresholds(direct)
			assertFloat64(t, "one-third NPC resistance 0% threshold", a, .76)
			assertFloat64(t, "one-third NPC resistance 25% threshold", b, .21)
			assertFloat64(t, "one-third NPC resistance 50% threshold", c, .03)
			assertFloat64(t, "binary resistance joins base miss before hit bonus", binary.SpellChanceToMiss(table), .2875)
			classic60PaladinRolls(t, sim, []float64{.4, .5, .1}, func() {
				result := direct.CalcDamage(sim, &agent.Unit, 1000, direct.OutcomeMagicHitAndCrit)
				assertFloat64(t, "partial-resistant NPC spell crit", result.Damage, 1125)
				if result.Outcome != OutcomeCrit|OutcomePartial1_4 {
					t.Fatalf("NPC partial crit outcome: %v", result.Outcome)
				}
				direct.DealDamage(sim, result)
			})
			classic60PaladinRolls(t, sim, []float64{.8}, func() {
				result := binary.CalcDamage(sim, &agent.Unit, 1000, binary.OutcomeMagicHitAndCrit)
				if result.Outcome != OutcomeMiss || result.Damage != 0 {
					t.Fatal("binary NPC spell did not resist completely")
				}
				binary.DealDamage(sim, result)
			})
			classic60PaladinRolls(t, sim, []float64{.5, .9}, func() {
				result := binary.CalcDamage(sim, &agent.Unit, 1000, binary.OutcomeMagicHitAndCrit)
				assertFloat64(t, "landed binary spell has no partial resist", result.Damage, 1000)
				binary.DealDamage(sim, result)
			})
			agent.spells.FireResistanceEffect.Deactivate(sim)
			assertFloat64(t, "expired resistance aura removes only its contribution", agent.GetStat(stats.FireResistance), 45)
			assertFloat64(t, "binary chance follows removed aura", binary.SpellChanceToMiss(table), 1-.95*(1-.75*45/315))
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinIncomingMagicRejectsUnsupportedInputs(t *testing.T) {
	for name, mutate := range map[string]func(*classic60PaladinTestAgent, *Spell, *AttackTable){
		"unregistered defense": func(a *classic60PaladinTestAgent, _ *Spell, _ *AttackTable) { a.classic60PaladinDefense = nil },
		"foreign table": func(_ *classic60PaladinTestAgent, spell *Spell, table *AttackTable) {
			spell.Unit.AttackTables[table.Defender.UnitIndex] = nil
		},
		"NPC level": func(_ *classic60PaladinTestAgent, spell *Spell, _ *AttackTable) { spell.Unit.Level = 64 },
		"weapon spell": func(_ *classic60PaladinTestAgent, spell *Spell, _ *AttackTable) {
			spell.weaponAttackSource = WeaponAttackSourceMainHand
		},
		"ranged spell": func(_ *classic60PaladinTestAgent, spell *Spell, _ *AttackTable) {
			spell.DefenseType = DefenseTypeRanged
		},
		"hybrid school": func(_ *classic60PaladinTestAgent, spell *Spell, _ *AttackTable) {
			spell.SpellSchool = SpellSchoolFire | SpellSchoolFrost
		},
		"haste":              func(_ *classic60PaladinTestAgent, spell *Spell, _ *AttackTable) { spell.IgnoreHaste = false },
		"target crit rating": func(a *classic60PaladinTestAgent, _ *Spell, _ *AttackTable) { a.stats[stats.ResilienceRating] = 1 },
		"table crit bonus":   func(_ *classic60PaladinTestAgent, _ *Spell, table *AttackTable) { table.BonusSpellCritPercent = 1 },
		"invalid crit":       func(_ *classic60PaladinTestAgent, spell *Spell, _ *AttackTable) { spell.BonusCritPercent = math.NaN() },
	} {
		t.Run(name, func(t *testing.T) {
			var incoming *Spell
			_, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: time.Second}, func(agent *classic60PaladinTestAgent) {
				agent.spells.classic60PaladinDefenseState = agent.spells.registerDefense(&agent.Character, classic60PaladinTalents{})
				incoming = classic60PaladinIncomingTestSpell(agent, SpellSchoolFire, false)
			})
			table := incoming.Unit.AttackTables[agent.UnitIndex]
			mutate(agent, incoming, table)
			defer func() {
				if recover() == nil {
					t.Fatal("unsupported incoming magic input was accepted")
				}
			}()
			liveClassic60PaladinIncomingSpellView(incoming, table)
		})
	}
}

func TestClassic60PaladinEyeForAnEyeRanksAndMitigation(t *testing.T) {
	for rank := uint8(1); rank <= 2; rank++ {
		t.Run(fmt.Sprint(rank), func(t *testing.T) {
			var reflected []float64
			sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 2 * time.Second, targetLevel: 63, pauseAutos: true, spellDamage: 1000}, func(agent *classic60PaladinTestAgent) {
				talents := classic60PaladinTalents{classic60PaladinEyeForAnEye: rank, classic60PaladinBlessingOfSanctuary: 1}
				agent.EnableHealthBar()
				agent.spells.classic60PaladinDefenseState = agent.spells.registerDefense(&agent.Character, talents)
				agent.spells.registerMagicDefense(&agent.Character, talents)
				agent.spells.registerSupport(&agent.Character, talents)
				agent.AddStat(stats.FireResistance, 105)
				agent.PseudoStats.DamageTakenMultiplier = .8
				agent.CurrentTarget.AddStat(stats.SpellCritPercent, 25)
				incoming := classic60PaladinIncomingTestSpell(agent, SpellSchoolFire, false)
				MakePermanent(agent.RegisterAura(Aura{Label: "Observe Eye for an Eye",
					OnSpellHitDealt: func(_ *Aura, _ *Simulation, spell *Spell, result *SpellResult) {
						if spell == agent.spells.EyeForAnEye {
							reflected = append(reflected, result.Damage)
						}
					},
				}))
				classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
					agent.spells.SanctuaryAura.Activate(sim)
					// 25% partial resist, critical incoming hit, then an unresisted
					// noncritical Holy reflection. Sanctuary's flat 24 must be
					// applied before reconstructing the pre-resistance amount.
					classic60PaladinRolls(t, sim, []float64{.4, .5, 0, .999, .5, .999}, func() {
						result := incoming.CalcDamage(sim, &agent.Unit, 1000, incoming.OutcomeMagicHitAndCrit)
						assertFloat64(t, "resisted critical damage after Sanctuary", result.Damage, (750-24)*1.5*.8)
						assertFloat64(t, "Eye pre-mitigation crit includes Sanctuary and multiplier once", result.classic60PreMitigationDamage, (1000-24)*1.5*.8)
						incoming.DealDamage(sim, result)
					})
					if len(reflected) != 1 {
						t.Fatalf("Eye reflected %d times", len(reflected))
					}
					assertFloat64(t, "Classic Eye percent and zero spell-power coefficient", reflected[0], .15*float64(rank)*(1000-24)*1.5*.8)
					classic60PaladinRolls(t, sim, []float64{.999, .5, .999}, func() {
						incoming.CalcAndDealDamage(sim, &agent.Unit, 100, incoming.OutcomeMagicHitAndCrit)
					})
					if len(reflected) != 1 {
						t.Fatal("Eye triggered from a noncritical spell")
					}
				})
			})
			classic60PaladinResult(t, sim.run())
		})
	}
}

func TestClassic60PaladinEyeForAnEyeCapCritAndMiss(t *testing.T) {
	for _, test := range []struct {
		name  string
		base  float64
		rolls []float64
		crit  bool
		miss  bool
	}{
		{"critical reflection", 1000, []float64{.999, .5, 0, .999, .5, 0}, true, false},
		{"capped base reflection", 20000, []float64{.999, .5, 0, .999, .5, .999}, false, false},
		{"missed reflection", 1000, []float64{.999, .5, 0, .999, .999}, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			reflections := 0
			sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 2 * time.Second, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
				talents := classic60PaladinTalents{classic60PaladinEyeForAnEye: 2, classic60PaladinVengeance: 5}
				agent.EnableHealthBar()
				agent.spells.classic60PaladinDefenseState = agent.spells.registerDefense(&agent.Character, talents)
				agent.spells.registerMagicDefense(&agent.Character, talents)
				agent.spells.registerVengeance(&agent.Character, talents)
				agent.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly] = 1.1
				agent.CurrentTarget.AddStat(stats.SpellCritPercent, 25)
				incoming := classic60PaladinIncomingTestSpell(agent, SpellSchoolFire, false)
				MakePermanent(agent.RegisterAura(Aura{Label: "Observe Eye outcomes",
					OnSpellHitDealt: func(_ *Aura, _ *Simulation, spell *Spell, result *SpellResult) {
						if spell != agent.spells.EyeForAnEye {
							return
						}
						reflections++
						want := min(.3*test.base*1.5, agent.MaxHealth()/2) * 1.1
						if test.crit {
							want *= 1.5
						}
						if test.miss {
							want = 0
						}
						assertFloat64(t, "Eye cap before ordinary Holy spell modifiers", result.Damage, want)
						if result.DidCrit() != test.crit || (result.Outcome == OutcomeMiss) != test.miss {
							t.Fatalf("wrong Eye outcome: %v", result.Outcome)
						}
					},
				}))
				classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
					classic60PaladinRolls(t, sim, test.rolls, func() {
						incoming.CalcAndDealDamage(sim, &agent.Unit, test.base, incoming.OutcomeMagicHitAndCrit)
					})
					if agent.spells.VengeanceAura.IsActive() != test.crit {
						t.Fatal("Eye critical result did not preserve the normal Vengeance trigger")
					}
				})
			})
			classic60PaladinResult(t, sim.run())
			if reflections != 1 {
				t.Fatalf("expected one reflected result, got %d", reflections)
			}
		})
	}
}

func TestClassic60PaladinEyeForAnEyeExcludesWeaponCrits(t *testing.T) {
	reflected := false
	talents := classic60PaladinTalents{classic60PaladinEyeForAnEye: 2}
	sim, _, _, events := classic60PaladinDefenseFixture(t,
		classic60PaladinTestConfig{duration: 500 * time.Millisecond, pauseAutos: true},
		talents, []float64{.19}, 2, func(agent *classic60PaladinTestAgent, _ *classic60PaladinDefenseState) {
			agent.spells.registerMagicDefense(&agent.Character, talents)
			MakePermanent(agent.RegisterAura(Aura{Label: "Observe forbidden weapon reflection",
				OnSpellHitDealt: func(_ *Aura, _ *Simulation, spell *Spell, _ *SpellResult) {
					if spell == agent.spells.EyeForAnEye {
						reflected = true
					}
				},
			}))
		})
	classic60PaladinResult(t, sim.run())
	if len(*events) != 1 || (*events)[0].outcome != OutcomeCrit || reflected {
		t.Fatalf("weapon critical must not trigger Eye: events=%+v, reflected=%v", *events, reflected)
	}
}

func TestClassic60PaladinEyeForAnEyeWhileIncapacitated(t *testing.T) {
	for _, feared := range []bool{true, false} {
		name := "stunned"
		if feared {
			name = "feared"
		}
		t.Run(name, func(t *testing.T) {
			reflections := 0
			sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 2 * time.Second, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
				talents := classic60PaladinTalents{classic60PaladinEyeForAnEye: 2}
				agent.EnableHealthBar()
				agent.spells.classic60PaladinDefenseState = agent.spells.registerDefense(&agent.Character, talents)
				agent.spells.registerMagicDefense(&agent.Character, talents)
				agent.CurrentTarget.AddStat(stats.SpellCritPercent, 25)
				incoming := classic60PaladinIncomingTestSpell(agent, SpellSchoolFire, false)
				var control *Aura
				if feared {
					control = agent.RegisterFearAura("Eye fear fixture", ActionID{SpellID: 900200}, 5*time.Second)
				} else {
					control = agent.RegisterStunAura("Eye stun fixture", ActionID{SpellID: 900201}, 5*time.Second)
				}
				MakePermanent(agent.RegisterAura(Aura{Label: "Observe incapacitated Eye",
					OnSpellHitDealt: func(_ *Aura, _ *Simulation, spell *Spell, result *SpellResult) {
						if spell == agent.spells.EyeForAnEye {
							reflections++
							assertFloat64(t, "incapacitated Eye still reflects a spell critical", result.Damage, 450)
						}
					},
				}))
				classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
					control.Activate(sim)
					if !agent.PseudoStats.Incapacitated {
						t.Fatal("native control did not incapacitate caster")
					}
					classic60PaladinRolls(t, sim, []float64{.999, .5, 0, .999, .5, .999}, func() {
						incoming.CalcAndDealDamage(sim, &agent.Unit, 1000, incoming.OutcomeMagicHitAndCrit)
					})
				})
			})
			classic60PaladinResult(t, sim.run())
			if reflections != 1 {
				t.Fatalf("expected one passive reflection while %s, got %d", name, reflections)
			}
		})
	}
}
