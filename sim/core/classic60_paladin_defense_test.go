package core

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

type classic60PaladinIncomingEvent struct {
	at                 time.Duration
	outcome            HitOutcome
	damage, resistance float64
}

// The NPC's ordinary auto-attack scheduler and normal damage/outcome entry
// points remain intact. Only its named one-roll outcome stream is controlled.
func classic60PaladinDefenseFixture(t *testing.T, config classic60PaladinTestConfig, talents classic60PaladinTalents, rolls []float64, speed float64, setup func(*classic60PaladinTestAgent, *classic60PaladinDefenseState)) (*Simulation, *classic60PaladinTestAgent, *classic60PaladinDefenseState, *[]classic60PaladinIncomingEvent) {
	t.Helper()
	var state *classic60PaladinDefenseState
	events := []classic60PaladinIncomingEvent{}
	sim, agent := newClassic60PaladinTestSim(t, config, func(agent *classic60PaladinTestAgent) {
		agent.Equipment[proto.ItemSlot_ItemSlotMainHand].HandType = proto.HandType_HandTypeOneHand
		agent.Equipment[proto.ItemSlot_ItemSlotOffHand] = Item{ID: -2, Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeShield, HandType: proto.HandType_HandTypeOffHand}
		agent.AutoAttacks.SetMH(agent.WeaponFromMainHand())
		agent.PseudoStats.InFrontOfTarget = true
		if !agent.HasHealthBar() {
			agent.EnableHealthBar()
		}
		state = agent.spells.registerDefense(&agent.Character, talents)
		target := agent.Env.Encounter.AllTargets[0]
		target.CurrentTarget = &agent.Unit
		target.PseudoStats.CanCrush = true
		target.EnableAutoAttacks(target, AutoAttackOptions{MainHand: Weapon{BaseDamageMin: 400, BaseDamageMax: 400, SwingSpeed: speed, SpellSchool: SpellSchoolPhysical}, AutoSwingMelee: true})
		target.AutoAttacks.RandomMeleeOffset = false
		original := target.AutoAttacks.mh.config.ApplyEffects
		index := 0
		target.AutoAttacks.mh.config.ApplyEffects = func(sim *Simulation, target *Unit, spell *Spell) {
			roll := .99
			if index < len(rolls) {
				roll = rolls[index]
			}
			index++
			previous, exists := sim.testRands["Enemy White Hit Table"]
			sim.testRands["Enemy White Hit Table"] = &sequenceMeleeRoll{values: []float64{roll}}
			defer func() {
				if exists {
					sim.testRands["Enemy White Hit Table"] = previous
				} else {
					delete(sim.testRands, "Enemy White Hit Table")
				}
			}()
			original(sim, target, spell)
		}
		agent.RegisterAura(Aura{Label: "Observe actual Classic incoming attacks", Duration: NeverExpires,
			OnReset: func(aura *Aura, sim *Simulation) { index = 0; aura.Activate(sim) },
			OnSpellHitTaken: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
				if spell == target.AutoAttacks.MHAuto() {
					events = append(events, classic60PaladinIncomingEvent{sim.CurrentTime, result.Outcome, result.Damage, result.ArmorAndResistanceMultiplier})
				}
			},
		})
		if setup != nil {
			setup(agent, state)
		}
	})
	return sim, agent, state, &events
}

func TestClassic60PaladinIncomingTableAndShieldValue(t *testing.T) {
	talents := classic60PaladinTalents{classic60PaladinAnticipation: 5, classic60PaladinDeflection: 5, classic60PaladinShieldSpecialization: 3}
	// Midpoints of miss/dodge/parry/block/crit/crush/hit for the source
	// Human baseline plus ten bonus Defense and five percent parry.
	rolls := []float64{.02, .06, .13, .20, .25, .35, .99}
	sim, agent, state, events := classic60PaladinDefenseFixture(t, classic60PaladinTestConfig{duration: 6500 * time.Millisecond, seed: 87, pauseAutos: true}, talents, rolls, 1, func(agent *classic60PaladinTestAgent, _ *classic60PaladinDefenseState) {
		agent.AddStats(stats.Stats{stats.BlockValue: 100, stats.Armor: 2870})
	})
	view := state.incomingView(63, true)
	assertFloat64(t, "Classic miss with ten Defense", view.miss, .048)
	assertFloat64(t, "Classic Agility dodge with ten Defense", view.dodge, .03789)
	assertFloat64(t, "Classic parry with Deflection", view.parry, .098)
	assertFloat64(t, "Shield Specialization does not add block chance", view.block, .048)
	assertFloat64(t, "Classic critical suppression", view.crit, .052)
	assertFloat64(t, "bonus Defense does not remove level-based crush", view.crush, .15)
	assertFloat64(t, "shield amount bonus excludes Strength", view.blockValue, 134.25)
	classic60PaladinResult(t, sim.run())
	want := []HitOutcome{OutcomeMiss, OutcomeDodge, OutcomeParry, OutcomeBlock, OutcomeCrit, OutcomeCrush, OutcomeHit}
	if len(*events) != len(want) {
		t.Fatalf("scheduled incoming attacks: got %d, want %d: %+v", len(*events), len(want), *events)
	}
	armorMultiplier := 5755.0 / (5755 + agent.GetStat(stats.Armor))
	base := 400 * armorMultiplier
	for i, event := range *events {
		if event.at != time.Duration(i)*time.Second || event.outcome != want[i] {
			t.Fatalf("incoming event %d: %+v, want %v", i, event, want[i])
		}
		assertFloat64(t, "Classic incoming armor multiplier", event.resistance, armorMultiplier)
		damage := []float64{0, 0, 0, base - 134.25, 2 * base, 1.5 * base, base}[i]
		assertFloat64(t, "incoming one-roll damage", event.damage, damage)
	}
}

func TestClassic60PaladinHolyShieldRedoubtAndRighteousFury(t *testing.T) {
	talents := classic60PaladinTalents{classic60PaladinRedoubt: 5, classic60PaladinHolyShield: 1, classic60PaladinImprovedRighteousFury: 3}
	procs := []classic60PaladinIncomingEvent{}
	sim, agent, state, events := classic60PaladinDefenseFixture(t, classic60PaladinTestConfig{duration: 7500 * time.Millisecond, iterations: 2, seed: 91, pauseAutos: true, spellDamage: 100}, talents, []float64{.18, .99, .2, .2, .2, .2, .2, .99}, 1, func(agent *classic60PaladinTestAgent, state *classic60PaladinDefenseState) {
		agent.RegisterAura(Aura{Label: "Observe Holy Shield retaliation", Duration: NeverExpires,
			OnReset: func(aura *Aura, sim *Simulation) { aura.Activate(sim) },
			OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
				if spell != state.HolyShieldProc {
					return
				}
				procs = append(procs, classic60PaladinIncomingEvent{sim.CurrentTime, result.Outcome, result.Damage, result.ArmorAndResistanceMultiplier})
				assertFloat64(t, "Holy Shield plus improved RF threat", spell.ThreatMultiplier, 1.2*1.9)
			},
		})
		classic60PaladinAt(agent, 100*time.Millisecond, func(sim *Simulation) {
			if !state.RighteousFury.Cast(sim, &agent.Unit) {
				t.Fatal("ready Righteous Fury failed")
			}
			assertFloat64(t, "Righteous Fury costs thirty percent base mana", agent.CurrentMana(), 2282-453.6)
			if state.RedoubtAura.GetStacks() != 5 {
				t.Fatal("incoming crit did not supply five Redoubt charges")
			}
		})
		classic60PaladinAt(agent, 1600*time.Millisecond, func(sim *Simulation) {
			before := agent.CurrentMana()
			if !state.HolyShield.Cast(sim, &agent.Unit) {
				t.Fatal("ready Holy Shield failed")
			}
			assertFloat64(t, "Holy Shield costs 240", before-agent.CurrentMana(), 240)
			if state.HolyShieldAura.GetStacks() != 4 || state.HolyShield.ReadyAt() != 11600*time.Millisecond {
				t.Fatal("Holy Shield charges or cooldown mismatch")
			}
		})
		classic60PaladinAt(agent, 5100*time.Millisecond, func(sim *Simulation) {
			if state.HolyShieldAura.IsActive() || state.RedoubtAura.GetStacks() != 1 {
				t.Fatal("four real blocks did not exhaust Holy Shield and leave one Redoubt charge")
			}
			if !state.RighteousFury.Cast(sim, &agent.Unit) {
				t.Fatal("Righteous Fury refresh failed")
			}
			assertFloat64(t, "refresh cannot stack Righteous Fury", state.HolyShieldProc.ThreatMultiplier, 1.2*1.9)
			assertFloat64(t, "RF leaves physical white threat unchanged", agent.AutoAttacks.MHAuto().ThreatMultiplier, 1)
		})
		classic60PaladinAt(agent, 6100*time.Millisecond, func(_ *Simulation) {
			if state.RedoubtAura.IsActive() {
				t.Fatal("fifth real block did not exhaust Redoubt")
			}
			assertFloat64(t, "shield charge expiration restores block chance", agent.GetStat(stats.BlockPercent), 0)
		})
	})
	assertFloat64(t, "fractional RF cost survives finalization", state.RighteousFury.DefaultCast.Cost, 453.6)
	classic60PaladinResult(t, sim.run())
	if len(*events) != 16 || len(procs) != 8 {
		t.Fatalf("real incoming or retaliation counts: %d / %d", len(*events), len(procs))
	}
	for i, event := range procs {
		if event.at != time.Duration(2+i%4)*time.Second || !event.outcome.Matches(OutcomeLanded) {
			t.Fatalf("Holy Shield proc %d: %+v", i, event)
		}
		base := event.damage / event.resistance
		if event.outcome.Matches(OutcomeCrit) {
			base /= 1.5
		}
		assertFloat64(t, "Holy Shield fixed damage plus five percent SP", base, 135)
	}
	_ = agent
}

func TestClassic60PaladinReckoningAndParryHaste(t *testing.T) {
	for _, tc := range []struct {
		name     string
		talent   classic60PaladinTalents
		roll     float64
		duration time.Duration
		want     []time.Duration
	}{
		{"Reckoning advances the next normal swing", classic60PaladinTalents{classic60PaladinReckoning: 5}, .18, 5500 * time.Millisecond, []time.Duration{0, 2 * time.Second, 4 * time.Second}},
		{"parry haste respects remaining twenty percent floor", classic60PaladinTalents{}, .10, 5500 * time.Millisecond, []time.Duration{0, 2600 * time.Millisecond, 4600 * time.Millisecond}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sim, agent, _, _ := classic60PaladinDefenseFixture(t, classic60PaladinTestConfig{duration: tc.duration, iterations: 2, seed: 97}, tc.talent, []float64{.99, tc.roll, tc.roll}, 2, nil)
			classic60PaladinResult(t, sim.run())
			for _, seed := range []int64{97, 98} {
				var swings []time.Duration
				for _, event := range agent.events {
					if event.seed == seed && event.kind == "white" {
						swings = append(swings, event.at)
					}
				}
				if !reflect.DeepEqual(swings, tc.want) {
					t.Fatalf("seed %d actual scheduled white swings %v, want %v", seed, swings, tc.want)
				}
			}
		})
	}
}

func TestClassic60PaladinIncomingRejectsRatingInputs(t *testing.T) {
	sim, agent, _, _ := classic60PaladinDefenseFixture(t, classic60PaladinTestConfig{duration: time.Second}, classic60PaladinTalents{}, nil, 2, nil)
	enemy := sim.Encounter.AllTargets[0]
	spell := enemy.AutoAttacks.MHAuto()
	table := enemy.AttackTables[agent.UnitIndex]
	// The modern target constructor seeds a level-dependent crit stat even
	// though Classic's enemy-white policy derives its baseline directly.
	// Additional NPC crit inputs must not be silently discarded.
	baselineCrit := enemy.stats[stats.PhysicalCritPercent]
	enemy.stats[stats.PhysicalCritPercent]++
	classic60PaladinDefenseMustPanic(t, "additional NPC physical crit", func() { validateClassic60PaladinIncoming(spell, table) })
	enemy.stats[stats.PhysicalCritPercent] = baselineCrit
	validateClassic60PaladinIncoming(spell, table)
	for _, stat := range []stats.Stat{stats.DefenseRating, stats.DodgeRating, stats.ParryRating, stats.BlockRating, stats.ResilienceRating} {
		agent.stats[stat] = 1
		classic60PaladinDefenseMustPanic(t, "TBC defensive rating", func() { validateClassic60PaladinIncoming(spell, table) })
		agent.stats[stat] = 0
	}
	agent.stats[stats.BlockValue] = math.NaN()
	classic60PaladinDefenseMustPanic(t, "non-finite shield value", func() { validateClassic60PaladinIncoming(spell, table) })
}

func classic60PaladinDefenseMustPanic(t *testing.T, label string, action func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s was silently accepted", label)
		}
	}()
	action()
}

func TestClassic60PaladinSanctuaryAndRetributionRealIncoming(t *testing.T) {
	retaliations, sanctuaries := 0, 0
	talents := classic60PaladinTalents{classic60PaladinBlessingOfSanctuary: 1, classic60PaladinImprovedRetributionAura: 2}
	sim, agent, _, events := classic60PaladinDefenseFixture(t, classic60PaladinTestConfig{duration: 3500 * time.Millisecond, iterations: 2, seed: 117, pauseAutos: true, spellDamage: 100}, talents, []float64{.01, .14, .99, .01}, 1, func(agent *classic60PaladinTestAgent, _ *classic60PaladinDefenseState) {
		agent.spells.registerSelfBuffs(&agent.Character, talents)
		agent.spells.registerSupport(&agent.Character, talents)
		if err := agent.spells.selectReferenceAura(agent.spells.RetributionEffect); err != nil {
			t.Fatal(err)
		}
		agent.AddStat(stats.BlockValue, 100)
		agent.RegisterAura(Aura{Label: "Observe source retaliation", Duration: NeverExpires,
			OnReset: func(aura *Aura, sim *Simulation) { aura.Activate(sim) },
			OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
				if spell == agent.spells.RetributionProc {
					retaliations++
					if result.Landed() {
						assertFloat64(t, "improved Ret aura has no SP coefficient", result.Damage, 30)
					}
				}
				if spell == agent.spells.SanctuaryProc {
					sanctuaries++
					if result.Landed() {
						assertFloat64(t, "Sanctuary has no SP coefficient", result.Damage, 35)
					}
				}
			},
		})
		classic60PaladinAt(agent, 100*time.Millisecond, func(sim *Simulation) {
			if !agent.spells.BlessingOfSanctuary.Cast(sim, &agent.Unit) {
				t.Fatal("ready Sanctuary failed")
			}
		})
	})
	classic60PaladinResult(t, sim.run())
	if retaliations != 4 || sanctuaries != 2 {
		t.Fatalf("real hit/block retaliation counts: Ret %d Sanctuary %d", retaliations, sanctuaries)
	}
	base := 400 * 5755 / (5755 + agent.GetStat(stats.Armor))
	for i, event := range *events {
		switch i % 4 {
		case 1:
			assertFloat64(t, "Sanctuary flat reduction before shield block", event.damage, base-24-104.25)
		case 2:
			assertFloat64(t, "Sanctuary flat reduction after armor", event.damage, base-24)
		default:
			if event.damage != 0 {
				t.Fatalf("miss triggered damage: %+v", event)
			}
		}
	}
}
