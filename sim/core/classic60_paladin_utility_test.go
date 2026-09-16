package core

import (
	"reflect"
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
)

func TestClassic60PaladinHammerOfJusticeHitImmunityAndExpiry(t *testing.T) {
	for _, test := range []struct {
		name         string
		immune, miss bool
	}{
		{name: "lands"}, {name: "spell_miss", miss: true}, {name: "configured_immune", immune: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			interrupted := false
			completed := 0
			sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{
				duration: 8 * time.Second, pauseAutos: true,
			}, func(agent *classic60PaladinTestAgent) {
				agent.spells.registerCrowdControl(&agent.Character, classic60PaladinTalents{classic60PaladinImprovedHammerOfJustice: 3})
				target := agent.CurrentTarget
				target.PseudoStats.StunImmune = test.immune
				enemyCast := target.RegisterSpell(SpellConfig{
					ActionID: ActionID{SpellID: -121}, ProcMask: ProcMaskEmpty,
					Cast:         CastConfig{IgnoreHaste: true, DefaultCast: Cast{CastTime: 5 * time.Second}},
					ApplyEffects: func(_ *Simulation, _ *Unit, _ *Spell) { completed++ },
				})
				classic60PaladinAt(agent, 500*time.Millisecond, func(sim *Simulation) {
					if !enemyCast.Cast(sim, &agent.Unit) {
						t.Fatal("test target did not start its real hardcast")
					}
				})
				classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
					spell := agent.spells.HammerOfJustice
					before := agent.CurrentMana()
					if test.immune {
						if spell.CanCast(sim, target) || spell.Cast(sim, target) || agent.CurrentMana() != before || !spell.CD.IsReady(sim) {
							t.Fatal("rotation did not skip the explicitly stun-immune target")
						}
					} else {
						roll := .5
						if test.miss {
							roll = .99
						}
						classic60PaladinRolls(t, sim, []float64{roll}, func() {
							if !spell.Cast(sim, target) {
								t.Fatal("Hammer of Justice failed on an eligible target")
							}
						})
						assertFloat64(t, "Hammer of Justice spends 100 mana", agent.CurrentMana(), before-100)
						if spell.CD.ReadyAt() != 46*time.Second {
							t.Fatal("Improved Hammer of Justice must reduce cooldown by five seconds per point")
						}
					}
					interrupted = target.Hardcast.Expires <= sim.CurrentTime
					want := !test.immune && !test.miss
					if target.PseudoStats.Stunned != want || target.PseudoStats.Incapacitated != want || interrupted != want ||
						agent.spells.HammerOfJusticeAuras.Get(target).IsActive() != want {
						t.Fatal("Hammer of Justice hit, stun and interruption did not agree")
					}
				})
				classic60PaladinAt(agent, 7*time.Second+time.Nanosecond, func(_ *Simulation) {
					if target.PseudoStats.Stunned || target.PseudoStats.Incapacitated || agent.spells.HammerOfJusticeAuras.Get(target).IsActive() {
						t.Fatal("six-second Hammer of Justice did not expire")
					}
				})
			})
			if spell := agent.spells.HammerOfJustice; spell.SpellID != 10308 || spell.Rank != 4 || spell.MaxRange != 10 || !spell.Flags.Matches(SpellFlagBinary) {
				t.Fatal("Hammer of Justice rank or control configuration changed")
			}
			if agent.spells.Repentance != nil {
				t.Fatal("unlearned Repentance was registered")
			}
			classic60PaladinResult(t, sim.run())
			if (completed == 0) != (!test.immune && !test.miss) || completed > 1 {
				t.Fatalf("Hammer's interruption changed the wrong pending completion: %d", completed)
			}
		})
	}
}

func TestClassic60PaladinRepentanceRestrictionsAndDamageBreak(t *testing.T) {
	for _, periodic := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "periodic"}[periodic], func(t *testing.T) {
			var swings []time.Duration
			sim, agent := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{
				duration: 7 * time.Second, targetLevel: 60, pauseAutos: true,
			}, func(agent *classic60PaladinTestAgent) {
				target := agent.Env.Encounter.AllTargets[0]
				target.MobType = proto.MobType_MobTypeHumanoid
				target.defaultTarget, target.CurrentTarget = &agent.Unit, &agent.Unit
				// Exercise actual auto-attack scheduling without coupling this
				// control test to the separate incoming-damage implementation.
				target.EnableAutoAttacks(target, AutoAttackOptions{
					MainHand: Weapon{SwingSpeed: 2}, AutoSwingMelee: true,
				})
				target.AutoAttacks.mh.config.ApplyEffects = func(sim *Simulation, _ *Unit, _ *Spell) { swings = append(swings, sim.CurrentTime) }
				agent.spells.registerCrowdControl(&agent.Character, classic60PaladinTalents{classic60PaladinRepentance: 1})
				damage := agent.RegisterSpell(SpellConfig{
					ActionID: ActionID{SpellID: -120}, SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic,
					ProcMask: ProcMaskSpellDamage, WeaponAttackSource: WeaponAttackSourceNone,
					Cast: CastConfig{IgnoreHaste: true}, DamageMultiplier: 1,
				})
				classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
					spell := agent.spells.Repentance
					target.MobType = proto.MobType_MobTypeUndead
					if spell.Cast(sim, &target.Unit) || agent.CurrentMana() != agent.MaxMana() {
						t.Fatal("Repentance accepted a non-humanoid or spent mana")
					}
					target.MobType, target.classic60RepentanceImmune = proto.MobType_MobTypeHumanoid, true
					if spell.CanCast(sim, &target.Unit) || spell.Cast(sim, &target.Unit) {
						t.Fatal("Repentance ignored its distinct immunity flag")
					}
					target.classic60RepentanceImmune = false
					target.PseudoStats.StunImmune = true // does not imply incapacitate immunity
					classic60PaladinRolls(t, sim, []float64{.5}, func() {
						if !spell.Cast(sim, &target.Unit) {
							t.Fatal("Repentance rejected an eligible humanoid")
						}
					})
					assertFloat64(t, "Repentance flat mana cost", agent.CurrentMana(), agent.MaxMana()-60)
					if target.PseudoStats.Stunned || !target.PseudoStats.Incapacitated ||
						agent.spells.RepentanceAuras.Get(&target.Unit).ExpiresAt() != 7*time.Second || spell.CD.ReadyAt() != 61*time.Second {
						t.Fatal("Repentance was not a six-second incapacitate with a one-minute cooldown")
					}
				})
				classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
					damage.CalcAndDealOutcome(sim, &target.Unit, damage.OutcomeAlwaysHit)
					if !target.PseudoStats.Incapacitated || target.PseudoStats.Stunned {
						t.Fatal("a zero-damage spell incorrectly broke Repentance")
					}
				})
				classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
					result := damage.CalcDamage(sim, &target.Unit, 10, damage.OutcomeAlwaysHit)
					if periodic {
						damage.DealPeriodicDamage(sim, result)
					} else {
						damage.DealDamage(sim, result)
					}
					if target.PseudoStats.Incapacitated || target.PseudoStats.Stunned || agent.spells.RepentanceAuras.Get(&target.Unit).IsActive() {
						t.Fatal("positive damage did not break Repentance")
					}
					if target.AutoAttacks.NextAttackAt() != 3*time.Second {
						t.Fatal("breaking Repentance did not immediately release the due swing")
					}
				})
			})
			if spell := agent.spells.Repentance; spell.SpellID != 20066 || spell.MaxRange != 20 || !spell.Flags.Matches(SpellFlagBinary) {
				t.Fatal("Repentance rank or control configuration changed")
			}
			classic60PaladinResult(t, sim.run())
			if want := []time.Duration{0, 3 * time.Second, 5 * time.Second, 7 * time.Second}; !reflect.DeepEqual(swings, want) {
				t.Fatalf("incapacitated target's swing timer did not pause/resume correctly: got %v, want %v", swings, want)
			}
		})
	}
}

func TestClassic60PaladinStunChangesJudgementBaseAndFrontAvoidance(t *testing.T) {
	var damages []float64
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{
		duration: 4 * time.Second, targetLevel: 60, spellDamage: 100, pauseAutos: true,
	}, func(agent *classic60PaladinTestAgent) {
		agent.PseudoStats.InFrontOfTarget = true
		agent.spells.registerCrowdControl(&agent.Character, classic60PaladinTalents{classic60PaladinRepentance: 1})
		agent.CurrentTarget.MobType = proto.MobType_MobTypeHumanoid
		agent.RegisterAura(Aura{Label: "Observe controlled-target Judgement", Duration: NeverExpires,
			OnReset: func(aura *Aura, sim *Simulation) { aura.Activate(sim) },
			OnSpellHitDealt: func(_ *Aura, _ *Simulation, spell *Spell, result *SpellResult) {
				if spell == agent.spells.CommandJudgement {
					damages = append(damages, result.Damage)
				}
			},
		})
		for index, at := range []time.Duration{time.Second, 2 * time.Second, 3 * time.Second} {
			classic60PaladinAt(agent, at, func(sim *Simulation) {
				target := agent.CurrentTarget
				if index == 1 {
					ApplyStun(sim, agent.spells.HammerOfJusticeAuras.Get(target))
					view := livePhysicalAttackTableView(agent.AutoAttacks.MHAuto(), agent.AttackTables[target.UnitIndex])
					if view.baseDodgeChance != 0 || view.baseParryChance != 0 || view.baseBlockChance != 0 || target.GetTotalBlockChanceAsDefender(agent.AttackTables[target.UnitIndex]) != 0 {
						t.Fatal("a stunned enemy retained frontal melee avoidance")
					}
				} else if index == 2 {
					agent.spells.HammerOfJusticeAuras.Get(target).Deactivate(sim)
					classic60RepentanceKind.apply(sim, agent.spells.RepentanceAuras.Get(target))
					view := livePhysicalAttackTableView(agent.AutoAttacks.MHAuto(), agent.AttackTables[target.UnitIndex])
					if view.baseDodgeChance == 0 || view.baseParryChance == 0 || view.baseBlockChance == 0 || target.PseudoStats.Stunned {
						t.Fatal("Repentance incorrectly inherited stun's avoidance suppression")
					}
				}
				classic60PaladinRolls(t, sim, []float64{.5, .5, .99, .99}, func() {
					agent.spells.CommandJudgement.Cast(sim, target)
				})
			})
		}
	})
	classic60PaladinResult(t, sim.run())
	if len(damages) != 3 {
		t.Fatalf("missing controlled-target Judgement results: %v", damages)
	}
	assertFloat64(t, "ordinary Judgement base plus SP", damages[0], 178+42.9)
	assertFloat64(t, "stun doubles only Judgement base", damages[1], 356+42.9)
	assertFloat64(t, "Repentance does not double Judgement", damages[2], 178+42.9)
}

func TestClassic60PaladinRepentanceMissExpiryAndOverlappingStun(t *testing.T) {
	for _, miss := range []bool{false, true} {
		t.Run(map[bool]string{false: "expires_under_stun", true: "miss"}[miss], func(t *testing.T) {
			sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{
				duration: 9 * time.Second, iterations: 2, pauseAutos: true,
			}, func(agent *classic60PaladinTestAgent) {
				agent.spells.registerCrowdControl(&agent.Character, classic60PaladinTalents{classic60PaladinRepentance: 1})
				target := agent.CurrentTarget
				target.MobType = proto.MobType_MobTypeHumanoid
				classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
					if target.PseudoStats.Incapacitated || target.PseudoStats.Stunned {
						t.Fatal("control flags leaked across iteration reset")
					}
					roll := .5
					if miss {
						roll = .99
					}
					classic60PaladinRolls(t, sim, []float64{roll}, func() {
						if !agent.spells.Repentance.Cast(sim, target) {
							t.Fatal("eligible Repentance did not cast")
						}
					})
					if target.PseudoStats.Incapacitated == miss || target.PseudoStats.Stunned {
						t.Fatal("Repentance's hit result did not control incapacitation alone")
					}
					assertFloat64(t, "Repentance spends mana even when missed", agent.CurrentMana(), agent.MaxMana()-60)
				})
				classic60PaladinAt(agent, 2*time.Second, func(sim *Simulation) {
					if !miss {
						ApplyStun(sim, agent.spells.HammerOfJusticeAuras.Get(target))
					}
				})
				classic60PaladinAt(agent, 7*time.Second+time.Nanosecond, func(_ *Simulation) {
					if agent.spells.RepentanceAuras.Get(target).IsActive() || target.PseudoStats.Stunned == miss || target.PseudoStats.Incapacitated == miss {
						t.Fatal("Repentance expiry cleared an overlapping stun or retained its own control")
					}
				})
				classic60PaladinAt(agent, 8*time.Second+time.Nanosecond, func(_ *Simulation) {
					if target.PseudoStats.Incapacitated || target.PseudoStats.Stunned {
						t.Fatal("control flags survived expiry of the final overlapping effect")
					}
				})
			})
			classic60PaladinResult(t, sim.run())
		})
	}
}
