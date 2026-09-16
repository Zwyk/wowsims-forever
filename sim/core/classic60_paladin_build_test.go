package core

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
	"google.golang.org/protobuf/encoding/protojson"
)

func classic60PaladinSupportedBuild(t *testing.T) classic60PaladinTalents {
	t.Helper()
	// A legal 11/8/32 allocation using implemented effects only: 51 points.
	talents, err := parseClassic60PaladinTalents("550001-503-55205051000315")
	if err != nil {
		t.Fatal(err)
	}
	return talents
}

func TestClassic60PaladinSupportedBuildAndVengeance(t *testing.T) {
	talents := classic60PaladinSupportedBuild(t)
	sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{
		duration: 11 * time.Second, iterations: 3, pauseAutos: true, talents: &talents,
	}, func(agent *classic60PaladinTestAgent) {
		if err := agent.spells.selectReferenceAura(agent.spells.SanctityEffect); err != nil {
			t.Fatal(err)
		}
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			assertFloat64(t, "Vengeance resets Physical multiplier", agent.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexPhysical], 1.06)
			assertFloat64(t, "selected Sanctity resets Holy multiplier", agent.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly], 1.1)
			classic60PaladinRolls(t, sim, []float64{.5, .99, .99, 0}, func() {
				agent.spells.CommandProc.Cast(sim, agent.CurrentTarget)
			})
		})
		classic60PaladinAt(agent, 1011*time.Millisecond, func(_ *Simulation) {
			assertFloat64(t, "critical Command activates Vengeance", agent.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexPhysical], 1.06*1.15)
			assertFloat64(t, "Vengeance multiplies Sanctity", agent.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly], 1.1*1.15)
		})
		classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
			classic60PaladinRolls(t, sim, []float64{.5, 0, .99, 0}, func() {
				agent.spells.CommandJudgement.Cast(sim, agent.CurrentTarget)
			})
			assertFloat64(t, "Vengeance refresh does not stack", agent.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly], 1.1*1.15)
			if agent.spells.VengeanceAura.ExpiresAt() != 10*time.Second {
				t.Fatal("Vengeance did not refresh to eight seconds after the latest crit")
			}
		})
		classic60PaladinAt(agent, 10*time.Second+time.Nanosecond, func(_ *Simulation) {
			if agent.spells.VengeanceAura.IsActive() {
				t.Fatal("Vengeance did not expire")
			}
			assertFloat64(t, "Vengeance expiry preserves weapon talent", agent.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexPhysical], 1.06)
			assertFloat64(t, "Vengeance expiry preserves selected aura", agent.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly], 1.1)
		})
	})
	assertFloat64(t, "Divine Strength preserves fractional attributes", agent.GetStat(stats.Strength), 115.5)
	assertFloat64(t, "AP consumes fractional Strength", agent.GetStat(stats.AttackPower), 391)
	assertFloat64(t, "Divine Intellect changes total mana, not base mana", agent.MaxMana(), 2387)
	assertFloat64(t, "base mana stays fixed", agent.BaseMana, 1512)
	assertFloat64(t, "Precision adds physical hit", agent.GetStat(stats.PhysicalHitPercent), 3)
	assertFloat64(t, "Precision does not add spell hit", agent.GetStat(stats.SpellHitPercent), 0)
	assertFloat64(t, "Conviction adds physical crit", agent.GetStat(stats.PhysicalCritPercent), 8.989)
	assertFloat64(t, "Deflection remains percentage parry", agent.PseudoStats.BaseParryChance, .10)
	assertFloat64(t, "Benediction seal cost", agent.spells.SealOfCommand.Cost.GetCurrentCost(), 178.5)
	assertFloat64(t, "Benediction fractional judgement cost", agent.spells.Judgement.Cost.GetCurrentCost(), 77.112)
	assertFloat64(t, "Benediction does not reduce Consecration", agent.spells.Consecration.Cost.GetCurrentCost(), 565)
	assertFloat64(t, "weapon specialization applies to Command separately", agent.spells.CommandProc.DamageMultiplier, .7*1.06)
	assertFloat64(t, "weapon specialization applies to JoC separately", agent.spells.CommandJudgement.DamageMultiplier, 1.06)
	if agent.spells.Judgement.CD.Duration != 8*time.Second {
		t.Fatal("Improved Judgement did not reduce its cooldown")
	}
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinUnsupportedTalentsRejectBeforeMutation(t *testing.T) {
	_, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{})
	// Legal source Ret preset, but contains unsupported Crusader/Vindication.
	talents, err := parseClassic60PaladinTalents("500501-503-52230351200315")
	if err != nil {
		t.Fatal(err)
	}
	before, spells := agent.GetStats(), len(agent.Spellbook)
	if result, err := registerClassic60PaladinBuild(&agent.Character, talents); err == nil || result != nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatal("an unsupported talent build was silently accepted")
	}
	if agent.GetStats() != before || len(agent.Spellbook) != spells {
		t.Fatal("rejected talent selection mutated the character")
	}
}

func TestClassic60PaladinBaseAbilitiesWithoutCommand(t *testing.T) {
	talents := classic60PaladinTalents{}
	sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{
		duration: 4 * time.Second, iterations: 2, pauseAutos: true, talents: &talents,
	}, func(agent *classic60PaladinTestAgent) {
		agent.CurrentTarget.MobType = proto.MobType_MobTypeUndead
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if !agent.spells.Exorcism.Cast(sim, agent.CurrentTarget) {
				t.Fatal("untalented Paladin cannot cast base Exorcism")
			}
		})
	})
	if agent.spells.SealOfCommand != nil || agent.spells.Judgement != nil || agent.spells.Consecration != nil || agent.spells.HolyShock != nil {
		t.Fatal("untalented build gained a talent-gated ability")
	}
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinTalentedNativeRotation(t *testing.T) {
	talents := classic60PaladinSupportedBuild(t)
	config := classic60PaladinTestConfig{duration: 90 * time.Second, iterations: 3, seed: 913, talents: &talents}
	makeSim := func() (*Simulation, *classic60PaladinTestAgent) {
		sim, agent := newClassic60PaladinTestSim(t, config, func(agent *classic60PaladinTestAgent) {
			agent.CurrentTarget.MobType = proto.MobType_MobTypeUndead
			if err := agent.spells.selectReferenceAura(agent.spells.SanctityEffect); err != nil {
				t.Fatal(err)
			}
		})
		rotation := &proto.APLRotation{}
		if err := protojson.Unmarshal([]byte(`{"type":"TypeAPL","priorityList":[
			{"action":{"condition":{"auraIsInactive":{"auraId":{"spellId":19838}}},"castSpell":{"spellId":{"spellId":19838},"target":{"type":"Self"}}}},
			{"action":{"condition":{"auraIsInactive":{"auraId":{"spellId":20920}}},"castSpell":{"spellId":{"spellId":20920}}}},
			{"action":{"castSpell":{"spellId":{"spellId":20271}}}},
			{"action":{"castSpell":{"spellId":{"spellId":10314}}}},
			{"action":{"castSpell":{"spellId":{"spellId":20924}}}},
			{"action":{"castSpell":{"spellId":{"spellId":10318}}}}
		]}`), rotation); err != nil {
			t.Fatal(err)
		}
		agent.Rotation = agent.newAPLRotation(rotation)
		if len(agent.Rotation.priorityList) != 6 {
			t.Fatal("talented Paladin native APL failed to resolve")
		}
		return sim, agent
	}
	sim, agent := makeSim()
	result := sim.run()
	player := classic60PaladinResult(t, result)
	actions := map[int32]bool{}
	for _, action := range player.Actions {
		for _, target := range action.Targets {
			if target.Casts > 0 {
				actions[action.Id.GetSpellId()] = true
			}
		}
	}
	// Judgement's passive wrapper reports its resource action; damage is
	// attributed to the triggered Command judgement instead.
	for _, resource := range player.Resources {
		if resource.Id.GetSpellId() == 20271 && resource.Events > 0 {
			actions[20271] = true
		}
	}
	for _, id := range []int32{19838, 20920, 20271, 10314, 20924, 10318} {
		if !actions[id] {
			t.Fatalf("native rotation did not exercise spell %d", id)
		}
	}
	if player.SecondsOomAvg <= 0 {
		t.Fatal("combined rotation did not reach mana recovery")
	}
	for _, event := range agent.events {
		if event.kind == "reset" && (event.mana != 2387 || event.seal || event.oom) {
			t.Fatalf("talented rotation leaked state across reset: %+v", event)
		}
	}
	repeatedSim, repeatedAgent := makeSim()
	classic60MeleeTestAssertResultEqual(t, result, repeatedSim.run())
	if !reflect.DeepEqual(agent.events, repeatedAgent.events) {
		t.Fatal("combined talented rotation is not reproducible")
	}
}
