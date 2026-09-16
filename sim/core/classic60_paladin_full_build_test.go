package core

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
	"google.golang.org/protobuf/encoding/protojson"
)

// These allocations spend the real 51-point budget and pass the same full
// build validator as a supplied Classic talent string. Synthetic weapon and
// durability stats isolate class integration from the later item-data port.
func classic60PaladinFullHolyTalents(t *testing.T) classic60PaladinTalents {
	t.Helper()
	talents := classic60PaladinTalents{
		classic60PaladinDivineStrength: 5, classic60PaladinDivineIntellect: 5,
		classic60PaladinSpiritualFocus: 5, classic60PaladinHealingLight: 3,
		classic60PaladinConsecration: 1, classic60PaladinImprovedLayOnHands: 2,
		classic60PaladinIllumination: 5, classic60PaladinImprovedBlessingOfWisdom: 2,
		classic60PaladinDivineFavor: 1, classic60PaladinHolyPower: 5, classic60PaladinHolyShock: 1,
		classic60PaladinImprovedDevotionAura: 5, classic60PaladinRedoubt: 5,
		classic60PaladinToughness: 5, classic60PaladinBlessingOfKings: 1,
	}
	classic60PaladinRequireFullBuild(t, talents, [3]int{35, 16, 0})
	return talents
}

func classic60PaladinFullProtectionTalents(t *testing.T) classic60PaladinTalents {
	t.Helper()
	talents := classic60PaladinTalents{
		classic60PaladinDivineStrength: 5, classic60PaladinDivineIntellect: 5,
		classic60PaladinImprovedSealOfRighteousness: 5, classic60PaladinHealingLight: 3,
		classic60PaladinConsecration: 1, classic60PaladinImprovedLayOnHands: 1,
		classic60PaladinRedoubt: 5, classic60PaladinPrecision: 3, classic60PaladinToughness: 2,
		classic60PaladinBlessingOfKings: 1, classic60PaladinImprovedRighteousFury: 3,
		classic60PaladinShieldSpecialization: 3, classic60PaladinAnticipation: 3,
		classic60PaladinBlessingOfSanctuary: 1, classic60PaladinReckoning: 4,
		classic60PaladinOneHandedWeaponSpecialization: 5, classic60PaladinHolyShield: 1,
	}
	classic60PaladinRequireFullBuild(t, talents, [3]int{20, 31, 0})
	return talents
}

func classic60PaladinRequireFullBuild(t *testing.T, talents classic60PaladinTalents, want [3]int) {
	t.Helper()
	var points [3]int
	for talent, rank := range talents {
		points[classic60PaladinTalentDescriptors[talent].tree] += int(rank)
	}
	if points != want || points[0]+points[1]+points[2] != 51 {
		t.Fatalf("incorrect complete Classic build allocation: %v", points)
	}
	parsed, err := parseClassic60PaladinTalents(talents.String())
	if err != nil || parsed != talents {
		t.Fatalf("full build does not survive Classic talent serialization: %v", err)
	}
	if err := talents.validateOffensiveSupport(); err != nil {
		t.Fatal(err)
	}
}

type classic60PaladinFullBuildEvent struct {
	kind                 string
	seed                 int64
	at                   time.Duration
	spellID, tag         int32
	outcome              HitOutcome
	health, mana, amount float64
	favor, righteousFury bool
	righteousness        bool
	stats                stats.Stats
}

func classic60PaladinFullBuildSim(t *testing.T, protection bool) (*Simulation, *classic60PaladinTestAgent, *[]classic60PaladinFullBuildEvent) {
	t.Helper()
	talents := classic60PaladinFullHolyTalents(t)
	config := classic60PaladinTestConfig{
		duration: 90 * time.Second, iterations: 3, seed: 7123, targetLevel: 63,
		mp5: 100, talents: &talents, pauseAutos: true, oneHandShield: true, front: true,
	}
	if protection {
		talents = classic60PaladinFullProtectionTalents(t)
		config.mp5, config.spellDamage, config.pauseAutos = 350, 100, false
	}
	events := []classic60PaladinFullBuildEvent{}
	sim, agent := newClassic60PaladinTestSim(t, config, func(agent *classic60PaladinTestAgent) {
		if agent.classic60PaladinDefense == nil || !agent.classic60PaladinDefense.hasShield() {
			t.Fatal("complete build did not initialize its actual shield defenses")
		}
		agent.AddStats(stats.Stats{stats.Health: 1500, stats.Armor: 1800, stats.BlockValue: 70, stats.HealingPower: 200})
		selected := agent.spells.ConcentrationEffect
		if protection {
			selected = agent.spells.DevotionEffect
		}
		if err := agent.spells.selectReferenceAura(selected); err != nil {
			t.Fatal(err)
		}
		target := agent.Env.Encounter.AllTargets[0]
		target.CurrentTarget, target.defaultTarget = &agent.Unit, &agent.Unit
		target.PseudoStats.CanCrush = true
		whiteDamage := 260.0
		if protection {
			whiteDamage = 110
		}
		target.EnableAutoAttacks(target, AutoAttackOptions{
			MainHand:       Weapon{BaseDamageMin: whiteDamage, BaseDamageMax: whiteDamage, SwingSpeed: 2, SpellSchool: SpellSchoolPhysical},
			AutoSwingMelee: true,
		})
		target.AutoAttacks.RandomMeleeOffset = false
		fire := target.RegisterSpell(SpellConfig{
			ActionID: ActionID{SpellID: 900301}, SpellSchool: SpellSchoolFire, DefenseType: DefenseTypeMagic,
			ProcMask: ProcMaskSpellDamage, WeaponAttackSource: WeaponAttackSourceNone,
			Cast: CastConfig{IgnoreHaste: true}, DamageMultiplier: 1, ThreatMultiplier: 1,
			ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
				spell.CalcAndDealDamage(sim, target, 150, spell.OutcomeMagicHitAndCrit)
			},
		})
		for at := 5 * time.Second; at < config.duration; at += 7 * time.Second {
			classic60PaladinAt(agent, at, func(sim *Simulation) { fire.Cast(sim, &agent.Unit) })
		}
		// Use the ordinary incoming-health consumer, death accounting and
		// reactive APL wakeup. Damage is not a fabricated health assignment.
		agent.trackChanceOfDeath(nil)
		record := func(kind string, sim *Simulation, spell *Spell, result *SpellResult) {
			event := classic60PaladinFullBuildEvent{
				kind: kind, seed: sim.currentSeed, at: sim.CurrentTime,
				health: agent.CurrentHealth(), mana: agent.CurrentMana(),
				favor:         agent.spells.DivineFavorAura != nil && agent.spells.DivineFavorAura.IsActive(),
				righteousFury: agent.spells.RighteousFuryAura.IsActive(),
				righteousness: agent.spells.RighteousnessAura.IsActive(),
			}
			if spell != nil {
				event.spellID, event.tag = spell.SpellID, spell.Tag
			}
			if result != nil {
				event.amount, event.outcome = result.Damage, result.Outcome
			}
			if kind == "reset" || kind == "done" {
				event.stats = agent.GetStats()
			}
			events = append(events, event)
		}
		agent.RegisterAura(Aura{Label: "Observe complete Classic Paladin build", Duration: NeverExpires,
			OnReset: func(aura *Aura, sim *Simulation) {
				aura.Activate(sim)
				sim.AddPendingAction(&PendingAction{NextActionAt: 0, Priority: ActionPriorityPrePull + 100,
					OnAction: func(sim *Simulation) { record("reset", sim, nil, nil) },
				})
			},
			OnDoneIteration: func(_ *Aura, sim *Simulation) { record("done", sim, nil, nil) },
			OnApplyEffects:  func(_ *Aura, sim *Simulation, _ *Unit, spell *Spell) { record("cast", sim, spell, nil) },
			OnHealDealt:     func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) { record("heal", sim, spell, result) },
			OnSpellHitTaken: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
				record("incoming", sim, spell, result)
			},
			OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
				record("outgoing", sim, spell, result)
			},
		})
	})
	rotationJSON := `{"type":"TypeAPL","priorityList":[
		{"action":{"condition":{"auraIsInactive":{"auraId":{"spellId":19854}}},"castSpell":{"spellId":{"spellId":19854},"target":{"type":"Self"}}}},
		{"action":{"condition":{"cmp":{"op":"OpLt","lhs":{"currentHealthPercent":{}},"rhs":{"const":{"val":"90%"}}}},"castSpell":{"spellId":{"spellId":20216},"target":{"type":"Self"}}}},
		{"action":{"condition":{"auraIsActive":{"auraId":{"spellId":20216}}},"castSpell":{"spellId":{"spellId":25292},"target":{"type":"Self"}}}},
		{"action":{"condition":{"cmp":{"op":"OpLt","lhs":{"currentHealthPercent":{}},"rhs":{"const":{"val":"45%"}}}},"castSpell":{"spellId":{"spellId":25292},"target":{"type":"Self"}}}},
		{"action":{"condition":{"cmp":{"op":"OpLt","lhs":{"currentHealthPercent":{}},"rhs":{"const":{"val":"90%"}}}},"castSpell":{"spellId":{"spellId":20930,"tag":1},"target":{"type":"Self"}}}},
		{"action":{"condition":{"cmp":{"op":"OpLt","lhs":{"currentHealthPercent":{}},"rhs":{"const":{"val":"90%"}}}},"castSpell":{"spellId":{"spellId":19943},"target":{"type":"Self"}}}}
	]}`
	if protection {
		rotationJSON = `{"type":"TypeAPL","priorityList":[
			{"action":{"condition":{"auraIsInactive":{"auraId":{"spellId":25780}}},"castSpell":{"spellId":{"spellId":25780},"target":{"type":"Self"}}}},
			{"action":{"condition":{"auraIsInactive":{"auraId":{"spellId":20914}}},"castSpell":{"spellId":{"spellId":20914},"target":{"type":"Self"}}}},
			{"action":{"condition":{"auraIsInactive":{"auraId":{"spellId":20928}}},"castSpell":{"spellId":{"spellId":20928},"target":{"type":"Self"}}}},
			{"action":{"condition":{"cmp":{"op":"OpLt","lhs":{"currentHealthPercent":{}},"rhs":{"const":{"val":"65%"}}}},"castSpell":{"spellId":{"spellId":19943},"target":{"type":"Self"}}}},
			{"action":{"condition":{"auraIsInactive":{"auraId":{"spellId":20293}}},"castSpell":{"spellId":{"spellId":20293}}}},
			{"action":{"condition":{"cmp":{"op":"OpGt","lhs":{"currentMana":{}},"rhs":{"const":{"val":"400"}}}},"castSpell":{"spellId":{"spellId":20271}}}},
			{"action":{"castSpell":{"spellId":{"spellId":20924}}}},
			{"action":{"condition":{"cmp":{"op":"OpLt","lhs":{"currentHealthPercent":{}},"rhs":{"const":{"val":"85%"}}}},"castSpell":{"spellId":{"spellId":19943},"target":{"type":"Self"}}}}
		]}`
	}
	rotation := &proto.APLRotation{}
	if err := protojson.Unmarshal([]byte(rotationJSON), rotation); err != nil {
		t.Fatal(err)
	}
	agent.Rotation = agent.newAPLRotation(rotation)
	if len(agent.Rotation.priorityList) != len(rotation.PriorityList) {
		t.Fatal("a complete-build native APL action failed to resolve")
	}
	for _, validations := range agent.Rotation.priorityListValidations {
		if len(validations) != 0 {
			t.Fatalf("complete-build APL validation: %v", validations)
		}
	}
	return sim, agent, &events
}

func TestClassic60PaladinCompleteHolyAndProtectionNativeAPLs(t *testing.T) {
	for _, protection := range []bool{false, true} {
		t.Run(map[bool]string{false: "Holy_35_16_0", true: "Protection_20_31_0"}[protection], func(t *testing.T) {
			sim, agent, events := classic60PaladinFullBuildSim(t, protection)
			initialStats, active := agent.GetStats(), currentRuleset()
			result := sim.run()
			player := classic60PaladinResult(t, result)
			resets, done, physicalIncoming, magicalIncoming, heals, criticalHeals, favoredLights := 0, 0, 0, 0, 0, 0, 0
			var firstReset stats.Stats
			for _, event := range *events {
				if math.IsNaN(event.health) || math.IsNaN(event.mana) || event.health <= 0 || event.health > agent.MaxHealth() || event.mana < -1e-9 || event.mana > agent.MaxMana()+1e-9 {
					t.Fatalf("invalid complete-build health or mana: %+v", event)
				}
				switch event.kind {
				case "reset":
					resets++
					if event.health != agent.MaxHealth() || event.mana != agent.MaxMana() || event.favor || event.righteousFury || event.righteousness {
						t.Fatalf("complete build did not reset its health, mana or cooldown aura: %+v", event)
					}
					if resets == 1 {
						firstReset = event.stats
					} else {
						for stat, value := range firstReset {
							assertFloat64(t, "reset stats retain fractional talent values", event.stats[stat], value)
						}
					}
				case "done":
					done++
					if event.favor || event.righteousFury || event.righteousness {
						t.Fatal("complete-build cleanup left a cooldown aura active")
					}
					for stat, value := range initialStats {
						assertFloat64(t, "full build removes temporary stat effects", event.stats[stat], value)
					}
				case "incoming":
					if event.amount > 0 {
						if event.spellID == 900301 {
							magicalIncoming++
						} else {
							physicalIncoming++
						}
					}
				case "heal":
					heals++
					if event.outcome.Matches(OutcomeCrit) {
						criticalHeals++
					}
				case "cast":
					if event.spellID == 25292 && event.favor {
						favoredLights++
					}
					if (event.spellID == 25292 || event.spellID == 19943 || (event.spellID == 20930 && event.tag == 1)) && event.health == agent.MaxHealth() {
						t.Fatal("native healing APL cast into full health")
					}
				}
			}
			if resets != 3 || done != 3 || physicalIncoming == 0 || magicalIncoming == 0 || heals == 0 || player.ChanceOfDeath != 0 || player.Dtps.Avg <= 0 || player.Hps.Avg <= 0 {
				t.Fatalf("incomplete live build: resets %d done %d physical %d magic %d heals %d death %g DTPS %g HPS %g", resets, done, physicalIncoming, magicalIncoming, heals, player.ChanceOfDeath, player.Dtps.Avg, player.Hps.Avg)
			}
			actions := map[int32]int32{}
			damage, threat := map[int32]float64{}, map[int32]float64{}
			for _, action := range player.Actions {
				for _, target := range action.Targets {
					actions[action.Id.GetSpellId()] += target.Casts
					damage[action.Id.GetSpellId()] += target.Damage
					threat[action.Id.GetSpellId()] += target.Threat
				}
			}
			resources := map[int32]*proto.ResourceMetrics{}
			for _, resource := range player.Resources {
				if resource.Type == proto.ResourceType_ResourceTypeMana {
					resources[resource.Id.GetSpellId()] = resource
				}
			}
			if protection {
				for _, id := range []int32{25780, 20914, 20928, 20293, 20286, 20924} {
					if actions[id] == 0 {
						t.Fatalf("Protection native rotation did not exercise spell %d", id)
					}
				}
				// Passive retaliation and seal procs intentionally report hits
				// and damage without adding player cast counts.
				for _, id := range []int32{20914, 20957, 25713, 20286, 20924} {
					if damage[id] <= 0 {
						t.Fatalf("Protection spell %d dealt no actual damage", id)
					}
					multiplier := 1.9
					if id == 20957 {
						multiplier *= 1.2
					}
					assertFloat64(t, "Protection Holy threat uses improved RF", threat[id], damage[id]*multiplier)
				}
				oom, recovered := map[int64]bool{}, map[int64]bool{}
				for _, event := range agent.events {
					if event.oom {
						oom[event.seed] = true
					}
					if oom[event.seed] && !event.oom {
						recovered[event.seed] = true
					}
				}
				if len(oom) != 3 || len(recovered) != 3 {
					t.Fatal("Protection native rotation did not recover from mana pressure in every iteration")
				}
				if resource := resources[25780]; resource == nil || resource.Events != 3 {
					t.Fatal("Protection did not maintain one Righteous Fury cast per iteration")
				} else {
					assertFloat64(t, "fractional RF mana cost survives full rotation", resource.ActualGain, -3*453.6)
				}
				assertFloat64(t, "Holy Shield RF threat multiplier resets", agent.spells.HolyShieldProc.ThreatMultiplier, 1.2)
				assertFloat64(t, "Sanctuary flat reduction resets", agent.PseudoStats.BonusDamageTakenBeforeModifiers, 0)
			} else {
				for _, id := range []int32{19854, 25292, 19943, 20930} {
					if actions[id] == 0 {
						t.Fatalf("Holy native rotation did not exercise spell %d", id)
					}
				}
				if favoredLights != 3 || criticalHeals < favoredLights {
					t.Fatalf("Holy did not consume Divine Favor on its real healing casts: favored %d critical %d", favoredLights, criticalHeals)
				}
				if refund := resources[20272]; refund == nil || refund.Events < 3 || refund.ActualGain < 3*660 {
					t.Fatal("Holy healing did not trigger and report its full-cost Illumination refunds")
				}
				if cost := resources[20216]; cost == nil || cost.Events != 3 {
					t.Fatal("Holy did not cast Divine Favor once per iteration")
				} else {
					assertFloat64(t, "fractional Divine Favor cost survives full rotation", cost.ActualGain, -3*60.48)
				}
			}
			if currentRuleset() != active || CharacterLevel != 70 {
				t.Fatal("complete Paladin fixture changed the public inherited rules")
			}
			repeatedSim, repeatedAgent, repeatedEvents := classic60PaladinFullBuildSim(t, protection)
			classic60MeleeTestAssertResultEqual(t, result, repeatedSim.run())
			if !reflect.DeepEqual(*events, *repeatedEvents) || !reflect.DeepEqual(agent.events, repeatedAgent.events) {
				t.Fatal("complete Paladin build is not reproducible with the same seed")
			}
		})
	}
}
