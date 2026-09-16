package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/stats"
)

func TestClassic60PaladinSupportBlessingCostsExclusivityAndReset(t *testing.T) {
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 311 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		talents := classic60PaladinTalents{}
		talents[classic60PaladinBlessingOfSanctuary] = 1
		agent.spells.registerSelfBuffs(&agent.Character, talents)
		agent.spells.registerSupport(&agent.Character, talents)
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			assertFloat64(t, "Salvation resets threat", agent.PseudoStats.ThreatMultiplier, 1)
			assertFloat64(t, "Sanctuary resets reduction", agent.PseudoStats.BonusDamageTakenBeforeModifiers, 0)
			before := agent.CurrentMana()
			if agent.spells.BlessingOfSalvation.CanCast(sim, agent.CurrentTarget) || !agent.spells.BlessingOfSalvation.Cast(sim, &agent.Unit) {
				t.Fatal("Salvation did not require a self target")
			}
			assertFloat64(t, "Salvation costs eight percent base mana", before-agent.CurrentMana(), 120.96)
			result := agent.spells.RetributionProc.CalcDamage(sim, agent.CurrentTarget, 100, agent.spells.RetributionProc.OutcomeAlwaysHit)
			assertFloat64(t, "Salvation leaves damage unchanged", result.Damage, 100)
			assertFloat64(t, "Salvation reduces actual damage threat", result.Threat, 70)
			agent.spells.RetributionProc.DisposeResult(result)
			if agent.spells.SalvationAura.ExpiresAt() != 301*time.Second || agent.NextGCDAt() != 2500*time.Millisecond {
				t.Fatal("Salvation duration or GCD differs from a normal blessing")
			}
		})
		classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
			before := agent.CurrentMana()
			if !agent.spells.BlessingOfSanctuary.Cast(sim, &agent.Unit) || agent.spells.SalvationAura.IsActive() {
				t.Fatal("Sanctuary did not replace Salvation")
			}
			assertFloat64(t, "Sanctuary trainer rank cost", before-agent.CurrentMana(), 135)
			assertFloat64(t, "Salvation removal restores threat", agent.PseudoStats.ThreatMultiplier, 1)
			assertFloat64(t, "Sanctuary rank four reduction", agent.PseudoStats.BonusDamageTakenBeforeModifiers, -24)
		})
		classic60PaladinAt(agent, 5*time.Second, func(sim *Simulation) {
			before := agent.CurrentMana()
			if !agent.spells.BlessingOfLight.Cast(sim, &agent.Unit) || agent.spells.SanctuaryAura.IsActive() {
				t.Fatal("Light did not replace Sanctuary")
			}
			assertFloat64(t, "Light trainer rank cost", before-agent.CurrentMana(), 135)
			assertFloat64(t, "Sanctuary removal restores reduction", agent.PseudoStats.BonusDamageTakenBeforeModifiers, 0)
		})
		classic60PaladinAt(agent, 7*time.Second, func(sim *Simulation) {
			if !agent.spells.BlessingOfMight.Cast(sim, &agent.Unit) || agent.spells.LightAura.IsActive() {
				t.Fatal("the original Might blessing did not replace Light")
			}
		})
		classic60PaladinAt(agent, 9*time.Second, func(sim *Simulation) {
			if !agent.spells.BlessingOfSalvation.Cast(sim, &agent.Unit) || agent.spells.MightAura.IsActive() {
				t.Fatal("Salvation did not replace the original Might blessing")
			}
			assertFloat64(t, "Salvation removes Might AP", agent.GetStat(stats.AttackPower), 370)
		})
		classic60PaladinAt(agent, 309*time.Second+time.Nanosecond, func(_ *Simulation) {
			if agent.spells.SalvationAura.IsActive() {
				t.Fatal("Salvation lasted longer than five minutes")
			}
			assertFloat64(t, "Salvation expiry restores threat", agent.PseudoStats.ThreatMultiplier, 1)
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinSupportAuraSelectionAndReset(t *testing.T) {
	for _, selection := range []string{"fire", "frost", "shadow", "concentration"} {
		t.Run(selection, func(t *testing.T) {
			sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 2 * time.Second, iterations: 3, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
				agent.spells.registerSelfBuffs(&agent.Character, classic60PaladinTalents{})
				agent.spells.registerSupport(&agent.Character, classic60PaladinTalents{})
				selected := map[string]*Aura{"fire": agent.spells.FireResistanceEffect, "frost": agent.spells.FrostResistanceEffect,
					"shadow": agent.spells.ShadowResistanceEffect, "concentration": agent.spells.ConcentrationEffect}[selection]
				if err := agent.spells.selectReferenceAura(selected); err != nil {
					t.Fatal(err)
				}
				classic60PaladinAt(agent, time.Second, func(_ *Simulation) {
					if !selected.IsActive() || agent.spells.DevotionEffect.IsActive() || agent.spells.RetributionEffect.IsActive() {
						t.Fatal("reset activated an aura other than the selected support aura")
					}
					for name, stat := range map[string]stats.Stat{"fire": stats.FireResistance, "frost": stats.FrostResistance, "shadow": stats.ShadowResistance} {
						want := 0.0
						if name == selection {
							want = 60
						}
						assertFloat64(t, name+" resistance", agent.GetStat(stat), want)
					}
					wantPushback := 1.0
					if selection == "concentration" {
						wantPushback = .65
					}
					assertFloat64(t, "selected Concentration pushback chance", agent.PseudoStats.PushbackChance, wantPushback)
				})
			})
			classic60PaladinResult(t, sim.run())
		})
	}
}

// Exercise native hit-taken aura dispatch and real outgoing proc damage. The
// synthetic notification intentionally does not claim to validate incoming
// Classic attack outcomes, which have their own policy and integration tests.
func TestClassic60PaladinSupportRetaliationCallbacks(t *testing.T) {
	for rank := uint8(0); rank <= 2; rank++ {
		t.Run(fmt.Sprint(rank), func(t *testing.T) {
			retHits, sanctuaryHits := 0, 0
			sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 4 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
				talents := classic60PaladinTalents{}
				talents[classic60PaladinBlessingOfSanctuary] = 1
				talents[classic60PaladinImprovedRetributionAura] = rank
				agent.spells.registerSelfBuffs(&agent.Character, talents)
				agent.spells.registerSupport(&agent.Character, talents)
				if err := agent.spells.selectReferenceAura(agent.spells.RetributionEffect); err != nil {
					t.Fatal(err)
				}
				incoming := &Spell{Unit: agent.CurrentTarget, ProcMask: ProcMaskMeleeMHAuto}
				MakePermanent(agent.RegisterAura(Aura{Label: "Support retaliation observer", OnSpellHitDealt: func(_ *Aura, _ *Simulation, spell *Spell, result *SpellResult) {
					switch spell {
					case agent.spells.RetributionProc:
						retHits++
						assertFloat64(t, "Retribution rank and talent damage", result.Damage, 20*(1+.25*float64(rank)))
					case agent.spells.SanctuaryProc:
						sanctuaryHits++
						assertFloat64(t, "Sanctuary block damage ignores resistance", result.Damage, 35)
					}
				}}))
				classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
					if !agent.spells.BlessingOfSanctuary.Cast(sim, &agent.Unit) {
						t.Fatal("Sanctuary failed")
					}
					for _, outcome := range []HitOutcome{OutcomeMiss, OutcomeDodge, OutcomeParry} {
						agent.OnSpellHitTaken(sim, incoming, &SpellResult{Target: &agent.Unit, Outcome: outcome})
					}
					incoming.ProcMask = ProcMaskRangedAuto
					agent.OnSpellHitTaken(sim, incoming, &SpellResult{Target: &agent.Unit, Outcome: OutcomeHit})
					incoming.ProcMask = ProcMaskMeleeMHAuto
					classic60PaladinRolls(t, sim, []float64{0}, func() {
						agent.OnSpellHitTaken(sim, incoming, &SpellResult{Target: &agent.Unit, Outcome: OutcomeHit})
					})
					classic60PaladinRolls(t, sim, []float64{0, 0}, func() {
						agent.OnSpellHitTaken(sim, incoming, &SpellResult{Target: &agent.Unit, Outcome: OutcomeBlock})
					})
				})
				classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
					if !agent.spells.BlessingOfMight.Cast(sim, &agent.Unit) {
						t.Fatal("Might failed")
					}
					classic60PaladinRolls(t, sim, []float64{0}, func() {
						agent.OnSpellHitTaken(sim, incoming, &SpellResult{Target: &agent.Unit, Outcome: OutcomeBlock})
					})
				})
			})
			classic60PaladinResult(t, sim.run())
			if retHits != 6 || sanctuaryHits != 2 {
				t.Fatalf("retaliation counts: Retribution=%d, Sanctuary=%d", retHits, sanctuaryHits)
			}
		})
	}
}

func TestClassic60PaladinSanctuaryRequiresTalent(t *testing.T) {
	_, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{}, func(agent *classic60PaladinTestAgent) {
		agent.spells.registerSelfBuffs(&agent.Character, classic60PaladinTalents{})
		agent.spells.registerSupport(&agent.Character, classic60PaladinTalents{})
	})
	if agent.spells.BlessingOfSanctuary != nil || agent.spells.SanctuaryAura != nil || agent.spells.SanctuaryProc != nil {
		t.Fatal("Sanctuary registered without the learned talent")
	}
}
