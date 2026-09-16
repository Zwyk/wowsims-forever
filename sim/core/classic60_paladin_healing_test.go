package core

import (
	"math"
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

type classic60PaladinHealEvent struct {
	id                   int32
	at                   time.Duration
	amount, health, mana float64
	outcome              HitOutcome
	threat               float64
}

func classic60PaladinObserveHealing(agent *classic60PaladinTestAgent, events *[]classic60PaladinHealEvent) {
	MakePermanent(agent.RegisterAura(Aura{Label: "Observe Classic healing", OnHealDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
		*events = append(*events, classic60PaladinHealEvent{spell.SpellID, sim.CurrentTime, result.Damage, result.Target.CurrentHealth(), agent.CurrentMana(), result.Outcome, result.Threat})
	}}))
}

func classic60PaladinControlledHeal(t *testing.T, spell *Spell, critRoll float64) {
	t.Helper()
	effects := spell.ApplyEffects
	spell.ApplyEffects = func(sim *Simulation, target *Unit, spell *Spell) {
		classic60PaladinRolls(t, sim, []float64{.5, critRoll}, func() { effects(sim, target, spell) })
	}
}

func TestClassic60PaladinHealingRanksScalingAndLifecycle(t *testing.T) {
	var events []classic60PaladinHealEvent
	sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 7 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		talents := classic60PaladinTalents{classic60PaladinHolyShock: 1, classic60PaladinHolyPower: 5, classic60PaladinHealingLight: 3}
		agent.AddStats(stats.Stats{stats.HealingPower: 700, stats.SpellDamage: 1000, stats.HolyDamage: 1000})
		agent.spells.registerOffensiveAbilities(&agent.Character, talents)
		agent.spells.registerHealingAbilities(&agent.Character, talents)
		classic60PaladinObserveHealing(agent, &events)
		for _, spell := range []*Spell{agent.spells.HolyLight, agent.spells.FlashOfLight, agent.spells.HolyShockHeal} {
			classic60PaladinControlledHeal(t, spell, .999)
		}
		for _, cast := range []struct {
			at    time.Duration
			spell *Spell
			cost  float64
		}{
			{time.Second, agent.spells.HolyLight, 660}, {4 * time.Second, agent.spells.FlashOfLight, 140}, {6 * time.Second, agent.spells.HolyShockHeal, 325},
		} {
			classic60PaladinAt(agent, cast.at, func(sim *Simulation) {
				agent.RemoveHealth(sim, agent.CurrentHealth()-1)
				before := agent.CurrentMana()
				if !cast.spell.Cast(sim, &agent.Unit) {
					t.Fatalf("healing cast failed: %v", cast.spell.ActionID)
				}
				if cast.spell.DefaultCast.CastTime > 0 {
					assertFloat64(t, "hardcast defers cost", agent.CurrentMana(), before)
				} else {
					assertFloat64(t, "instant healing spends immediately", before-agent.CurrentMana(), cast.cost)
				}
			})
		}
	})
	classic60PaladinResult(t, sim.run())
	if len(events) != 6 {
		t.Fatalf("want three heals per iteration, got %+v", events)
	}
	for i, event := range events {
		want := []struct {
			id     int32
			at     time.Duration
			amount float64
		}{{25292, 3500 * time.Millisecond, 2441.6}, {19943, 5500 * time.Millisecond, 748.384}, {20930, 6 * time.Second, 680}}[i%3]
		if event.id != want.id || event.at != want.at || event.outcome != OutcomeHit {
			t.Fatalf("unexpected heal timing/outcome: %+v", event)
		}
		assertFloat64(t, "healing uses healing power and correct talent multiplier", event.amount, want.amount)
		assertFloat64(t, "actual health caps healing and preserves overheal metrics", event.health, math.Min(agent.MaxHealth(), 1+want.amount))
		assertFloat64(t, "Paladin healing threat uses only effective health restored", event.threat, (event.health-1)*.25)
	}
	if agent.spells.HolyShock.CD.Timer != agent.spells.HolyShockHeal.CD.Timer || agent.spells.HolyShockHeal.CD.Duration != 30*time.Second {
		t.Fatal("Holy Shock heal and damage did not share the Classic cooldown")
	}
	if agent.spells.HolyLight.DefaultCast.CastTime != 2500*time.Millisecond || agent.spells.FlashOfLight.DefaultCast.CastTime != 1500*time.Millisecond {
		t.Fatal("healing cast times changed")
	}
}

func TestClassic60PaladinDivineFavorIlluminationAndInterrupt(t *testing.T) {
	var events []classic60PaladinHealEvent
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 9 * time.Second, iterations: 2, pauseAutos: true, targetLevel: 60}, func(agent *classic60PaladinTestAgent) {
		talents := classic60PaladinTalents{classic60PaladinHolyShock: 1, classic60PaladinHolyPower: 5, classic60PaladinIllumination: 5, classic60PaladinDivineFavor: 1}
		agent.spells.registerOffensiveAbilities(&agent.Character, talents)
		agent.spells.registerHealingAbilities(&agent.Character, talents)
		classic60PaladinObserveHealing(agent, &events)
		classic60PaladinControlledHeal(t, agent.spells.FlashOfLight, .999)
		shockEffects := agent.spells.HolyShock.ApplyEffects
		agent.spells.HolyShock.ApplyEffects = func(sim *Simulation, target *Unit, spell *Spell) {
			classic60PaladinRolls(t, sim, []float64{.5, .999, .5, .999}, func() { shockEffects(sim, target, spell) })
		}
		MakePermanent(agent.RegisterAura(Aura{Label: "Observe Divine Favor offensive result", OnSpellHitDealt: func(_ *Aura, _ *Simulation, spell *Spell, result *SpellResult) {
			if spell == agent.spells.HolyShock {
				if result.Outcome != OutcomeCrit {
					t.Fatal("Divine Favor did not guarantee damaging Holy Shock's crit")
				}
				assertFloat64(t, "damaging Holy Shock retains its Classic crit multiplier", result.Damage, 570)
			}
		}}))
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if agent.spells.DivineFavorAura.IsActive() {
				t.Fatal("Divine Favor leaked between iterations")
			}
			if !agent.spells.DivineFavor.Cast(sim, &agent.Unit) || !agent.spells.HolyLight.Cast(sim, &agent.Unit) {
				t.Fatal("Divine Favor should be off GCD and permit Holy Light immediately")
			}
			assertFloat64(t, "Divine Favor pays four percent base mana", agent.CurrentMana(), 2221.52)
		})
		classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) { agent.Interrupt(sim) })
		classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
			if !agent.spells.DivineFavorAura.IsActive() {
				t.Fatal("interrupted heal consumed Divine Favor")
			}
			assertFloat64(t, "interrupted Holy Light did not spend", agent.CurrentMana(), 2221.52)
			if !agent.spells.FlashOfLight.Cast(sim, &agent.Unit) {
				t.Fatal("Flash of Light failed")
			}
		})
		classic60PaladinAt(agent, 4501*time.Millisecond, func(_ *Simulation) {
			if agent.spells.DivineFavorAura.IsActive() {
				t.Fatal("completed heal did not consume Divine Favor")
			}
			assertFloat64(t, "Illumination refunds full Classic base cost", agent.CurrentMana(), 2221.52)
			if agent.spells.DivineFavor.CD.ReadyAt() != 121*time.Second {
				t.Fatal("Divine Favor cooldown should start on activation")
			}
			assertFloat64(t, "temporary crit guarantee restores Holy Power bonus", agent.spells.FlashOfLight.BonusCritPercent, 5)
		})
		classic60PaladinAt(agent, 6*time.Second, func(sim *Simulation) {
			agent.spells.DivineFavor.CD.Reset()
			if !agent.spells.DivineFavor.Cast(sim, &agent.Unit) {
				t.Fatal("test reset did not ready Divine Favor")
			}
			before := agent.CurrentMana()
			if !agent.spells.HolyShock.Cast(sim, agent.CurrentTarget) {
				t.Fatal("damaging Holy Shock failed")
			}
			assertFloat64(t, "damaging Holy Shock cannot trigger Illumination", before-agent.CurrentMana(), 325)
			if agent.spells.DivineFavorAura.IsActive() {
				t.Fatal("damaging Shock did not consume Divine Favor")
			}
			assertFloat64(t, "temporary guarantee restores offensive Holy Power", agent.spells.HolyShock.BonusCritPercent, 5)
		})
		classic60PaladinAt(agent, 8*time.Second, func(sim *Simulation) {
			if agent.spells.HolyShockHeal.CanCast(sim, &agent.Unit) {
				t.Fatal("healing Shock bypassed shared cooldown after offensive Shock")
			}
		})
	})
	classic60PaladinResult(t, sim.run())
	if len(events) != 2 {
		t.Fatalf("interrupted casts should not heal: %+v", events)
	}
	for _, event := range events {
		if event.id != 19943 || event.at != 4500*time.Millisecond || event.outcome != OutcomeCrit {
			t.Fatalf("Divine Favor did not guarantee the completed heal: %+v", event)
		}
		assertFloat64(t, "Classic healing crit is 150 percent", event.amount, 368.2*1.5)
		assertFloat64(t, "fully overhealing critical heal generates no healing threat", event.threat, 0)
	}
}

func classic60PaladinHealingAlly(agent *classic60PaladinTestAgent) *FakeAgent {
	baseline, _ := classicReferenceInitializeCharacter(60, proto.Race_RaceHuman, proto.Class_ClassPaladin)
	ally := &FakeAgent{Character: newCharacterWithRuleset(agent.Party, 1, &proto.Player{
		Name: "Classic healing ally", Race: proto.Race_RaceHuman, Class: proto.Class_ClassPaladin, Spec: &proto.Player_RetributionPaladin{}, Equipment: &proto.EquipmentSpec{},
	}, agent.resolvedRuleset(), baseline.baseStats)}
	ally.AddBaseClassStatDependencies()
	ally.EnableManaBar()
	ally.EnableHealthBar()
	ally.Env, ally.UnitIndex = agent.Env, int32(len(agent.Env.AllUnits))
	ally.CurrentTarget = agent.CurrentTarget
	agent.Party.Players = append(agent.Party.Players, ally)
	agent.Env.AllUnits = append(agent.Env.AllUnits, &ally.Unit)
	agent.Env.Raid.updatePlayersAndPets()
	ally.initialize(ally)
	return ally
}

func TestClassic60PaladinLayOnHandsRecipientArmorAndReset(t *testing.T) {
	var events []classic60PaladinHealEvent
	checks := 0
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 123 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		ally := classic60PaladinHealingAlly(agent)
		// Synthetic armor is sufficient to distinguish equipment scaling from
		// agility/base armor. No real gear data enters the healing fixture.
		ally.cachedEquipStats[stats.Armor] = 1000
		ally.equipStatsApplied = true
		ally.AddStat(stats.Armor, 1000)
		agent.spells.registerHealingAbilities(&agent.Character, classic60PaladinTalents{classic60PaladinImprovedLayOnHands: 2})
		classic60PaladinObserveHealing(agent, &events)
		ally.Finalize()
		var baselineArmor float64
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			baselineArmor = ally.GetStat(stats.Armor)
			if agent.spells.layOnHandsArmorAuras[&ally.Unit].IsActive() {
				t.Fatal("improved Lay on Hands leaked across iterations")
			}
			ally.RemoveHealth(sim, ally.CurrentHealth()-1)
			ally.currentMana = 500
			if !agent.spells.LayOnHands.Cast(sim, &ally.Unit) {
				t.Fatal("Lay on Hands rejected a friendly player")
			}
			assertFloat64(t, "Lay on Hands drains caster's remaining mana", agent.CurrentMana(), 0)
			assertFloat64(t, "Lay on Hands restores recipient mana", ally.CurrentMana(), 1050)
			assertFloat64(t, "Lay on Hands heals recipient for caster maximum health", ally.CurrentHealth(), ally.MaxHealth())
			assertFloat64(t, "improved Lay on Hands scales equipped armor", ally.GetStat(stats.Armor), baselineArmor+300)
			if agent.spells.LayOnHands.CD.ReadyAt() != 2401*time.Second || agent.PseudoStats.FiveSecondRuleRefreshTime != 6*time.Second {
				t.Fatal("Lay on Hands lost talented cooldown or mana-spending FSR")
			}
		})
		classic60PaladinAt(agent, 121*time.Second+time.Millisecond, func(_ *Simulation) {
			assertFloat64(t, "improved Lay on Hands removes armor after two minutes", ally.GetStat(stats.Armor), baselineArmor)
			checks++
		})
	})
	classic60PaladinResult(t, sim.run())
	if len(events) != 2 || checks != 2 {
		t.Fatalf("wrong Lay on Hands lifecycle counts: events=%d checks=%d", len(events), checks)
	}
	for _, event := range events {
		if event.id != 10310 || event.outcome != OutcomeHit {
			t.Fatal("Lay on Hands should not crit")
		}
	}
}

func TestClassic60PaladinHealingRejectsEnemyDeadAndMovingTargets(t *testing.T) {
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 2 * time.Second, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		agent.spells.registerHealingAbilities(&agent.Character, classic60PaladinTalents{})
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			mana := agent.CurrentMana()
			if agent.spells.HolyLight.Cast(sim, agent.CurrentTarget) || agent.spells.LayOnHands.Cast(sim, agent.CurrentTarget) {
				t.Fatal("healing spell accepted an enemy")
			}
			agent.Moving = true
			if agent.spells.HolyLight.Cast(sim, &agent.Unit) {
				t.Fatal("hardcast heal ignored movement")
			}
			agent.Moving = false
			agent.RemoveHealth(sim, agent.CurrentHealth())
			if agent.spells.LayOnHands.Cast(sim, &agent.Unit) {
				t.Fatal("Lay on Hands resurrected a dead target")
			}
			assertFloat64(t, "failed heals spend no mana", agent.CurrentMana(), mana)
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinBlessingOfLightHealingStage(t *testing.T) {
	var events []classic60PaladinHealEvent
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 6 * time.Second, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		talents := classic60PaladinTalents{classic60PaladinHealingLight: 3}
		agent.spells.registerSelfBuffs(&agent.Character, talents)
		agent.spells.registerSupport(&agent.Character, talents)
		agent.spells.registerHealingAbilities(&agent.Character, talents)
		classic60PaladinObserveHealing(agent, &events)
		classic60PaladinControlledHeal(t, agent.spells.HolyLight, .999)
		classic60PaladinControlledHeal(t, agent.spells.FlashOfLight, 0)
		classic60PaladinAt(agent, 0, func(sim *Simulation) {
			if !agent.spells.BlessingOfLight.Cast(sim, &agent.Unit) {
				t.Fatal("Blessing of Light failed")
			}
		})
		classic60PaladinAt(agent, 1500*time.Millisecond, func(sim *Simulation) {
			if !agent.spells.HolyLight.Cast(sim, &agent.Unit) {
				t.Fatal("blessed Holy Light failed")
			}
		})
		classic60PaladinAt(agent, 4*time.Second+time.Millisecond, func(sim *Simulation) {
			if !agent.spells.FlashOfLight.Cast(sim, &agent.Unit) {
				t.Fatal("blessed Flash of Light failed")
			}
		})
	})
	classic60PaladinResult(t, sim.run())
	if len(events) != 2 {
		t.Fatalf("expected two blessed heals, got %+v", events)
	}
	assertFloat64(t, "Light bonus follows source taken-stage coefficient after Healing Light", events[0].amount, 1680*1.12+400.0*5/7)
	assertFloat64(t, "Light taken bonus also receives the subsequent healing crit", events[1].amount, (368.2*1.12+115.0*3/7)*1.5)
}
