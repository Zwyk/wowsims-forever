package core

import (
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
)

func TestClassic60PaladinDefensiveCooldownIncomingImmunity(t *testing.T) {
	for _, name := range []string{"shield", "divine_protection", "blessing_protection"} {
		t.Run(name, func(t *testing.T) {
			var bubble *Spell
			var effect *Aura
			var healthAfterCast float64
			var magicOutcome HitOutcome
			sim, agent, _, incoming := classic60PaladinDefenseFixture(t,
				classic60PaladinTestConfig{duration: 14 * time.Second, iterations: 2, pauseAutos: true}, classic60PaladinTalents{}, nil, 1,
				func(agent *classic60PaladinTestAgent, _ *classic60PaladinDefenseState) {
					talents := classic60PaladinTalents{}
					agent.spells.registerSelfBuffs(&agent.Character, talents)
					agent.spells.registerSupport(&agent.Character, talents)
					agent.spells.registerOffensiveAbilities(&agent.Character, talents)
					agent.spells.registerHealingAbilities(&agent.Character, talents)
					agent.spells.registerDefensiveCooldowns(&agent.Character, talents)
					agent.trackChanceOfDeath(nil)
					if err := agent.spells.selectReferenceAura(agent.spells.RetributionEffect); err != nil {
						t.Fatal(err)
					}
					target := agent.Env.Encounter.AllTargets[0]
					target.AutoAttacks.mh.BaseDamageMin, target.AutoAttacks.mh.BaseDamageMax = 40, 40
					target.MobType = proto.MobType_MobTypeUndead
					enemySpell := target.RegisterSpell(SpellConfig{
						ActionID: ActionID{SpellID: 9991020}, SpellSchool: SpellSchoolFire, DefenseType: DefenseTypeMagic,
						WeaponAttackSource: WeaponAttackSourceNone, ProcMask: ProcMaskSpellDamage,
						Cast: CastConfig{IgnoreHaste: true}, DamageMultiplier: 1, ThreatMultiplier: 1,
						ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
							result := spell.CalcDamage(sim, target, 100, spell.OutcomeMagicHitAndCrit)
							magicOutcome = result.Outcome
							spell.DealDamage(sim, result)
						},
					})
					switch name {
					case "shield":
						bubble, effect = agent.spells.DivineShield, agent.spells.DivineShieldAura
					case "divine_protection":
						bubble, effect = agent.spells.DivineProtection, agent.spells.DivineProtectionAura
					case "blessing_protection":
						bubble, effect = agent.spells.BlessingOfProtection, agent.spells.ProtectionAura
					}
					classic60PaladinAt(agent, 500*time.Millisecond, func(sim *Simulation) {
						if agent.classic60ImmuneSchools != SpellSchoolNone || agent.classic60PaladinPacified || agent.spells.ForbearanceAura.IsActive() {
							t.Fatal("immunity state leaked across iterations")
						}
						before := agent.CurrentMana()
						if bubble.CanCast(sim, agent.CurrentTarget) || !bubble.Cast(sim, &agent.Unit) {
							t.Fatal("defensive cooldown failed its self-only cast")
						}
						assertFloat64(t, "cooldown spends its registered cost", before-agent.CurrentMana(), map[string]float64{"shield": 110, "divine_protection": 35, "blessing_protection": 105.84}[name])
						healthAfterCast = agent.CurrentHealth()
						assertFloat64(t, "Divine Shield melee speed penalty", agent.PseudoStats.MeleeSpeedMultiplier, map[string]float64{"shield": .5, "divine_protection": 1, "blessing_protection": 1}[name])
					})
					classic60PaladinAt(agent, 2100*time.Millisecond, func(sim *Simulation) {
						if !effect.IsActive() || !agent.spells.ForbearanceAura.IsActive() {
							t.Fatal("immunity or Forbearance did not activate")
						}
						if agent.AutoAttacks.MHAuto().CanCast(sim, agent.CurrentTarget) != (name == "shield") {
							t.Fatal("physical pacification differs from the chosen protection")
						}
						if !agent.spells.HolyLight.CanCast(sim, &agent.Unit) || !agent.spells.Exorcism.CanCast(sim, agent.CurrentTarget) {
							t.Fatal("physical pacification incorrectly blocked Holy spells")
						}
						rolls := []float64(nil)
						if name == "blessing_protection" {
							rolls = []float64{.99, 0, .99}
						}
						classic60PaladinRolls(t, sim, rolls, func() { enemySpell.Cast(sim, &agent.Unit) })
						wantHealth, wantOutcome := healthAfterCast, OutcomeImmune
						if name == "blessing_protection" {
							wantHealth -= 100
							wantOutcome = OutcomeHit
						}
						if magicOutcome != wantOutcome {
							t.Fatalf("magic result %v, want %v", magicOutcome, wantOutcome)
						}
						assertFloat64(t, "damage immunity protects actual health by school", agent.CurrentHealth(), wantHealth)
						if casts := agent.spells.RetributionProc.SpellMetrics[agent.CurrentTarget.UnitIndex].Casts; casts != 1 {
							t.Fatalf("immune melee attacks triggered retaliation: %d casts", casts)
						}
					})
					classic60PaladinAt(agent, 13*time.Second, func(_ *Simulation) {
						if effect.IsActive() || agent.classic60ImmuneSchools != SpellSchoolNone || agent.classic60PaladinPacified {
							t.Fatal("expired protection retained immunity or pacification")
						}
						assertFloat64(t, "Divine Shield expiry restores melee speed", agent.PseudoStats.MeleeSpeedMultiplier, 1)
					})
				})
			classic60PaladinResult(t, sim.run())
			for _, event := range *incoming {
				immune := event.at >= time.Second && event.at < 500*time.Millisecond+effect.Duration
				if event.outcome.Matches(OutcomeImmune) != immune {
					t.Fatalf("%s incoming outcome at %v = %v, immune=%v", name, event.at, event.outcome, immune)
				}
				if immune && event.damage != 0 {
					t.Fatal("an immune attack dealt damage")
				}
			}
			if agent.spells.DivineShield.CD.Timer != agent.spells.DivineProtection.CD.Timer {
				t.Fatal("Divine Shield and Protection do not share their client cooldown category")
			}
		})
	}
}

func TestClassic60PaladinForbearanceAndBlessingReplacement(t *testing.T) {
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 63 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
		agent.spells.registerSelfBuffs(&agent.Character, classic60PaladinTalents{})
		agent.spells.registerDefensiveCooldowns(&agent.Character, classic60PaladinTalents{})
		classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
			if !agent.spells.BlessingOfProtection.Cast(sim, &agent.Unit) {
				t.Fatal("Protection failed")
			}
		})
		classic60PaladinAt(agent, 3*time.Second, func(sim *Simulation) {
			if !agent.spells.BlessingOfMight.Cast(sim, &agent.Unit) || agent.spells.ProtectionAura.IsActive() || agent.classic60PaladinPacified || agent.classic60ImmuneSchools != 0 {
				t.Fatal("Might did not replace the Protection blessing")
			}
			if !agent.spells.ForbearanceAura.IsActive() {
				t.Fatal("replacing Protection incorrectly removed Forbearance")
			}
		})
		classic60PaladinAt(agent, 60*time.Second, func(sim *Simulation) {
			before := agent.CurrentMana()
			if agent.spells.DivineShield.Cast(sim, &agent.Unit) || agent.spells.DivineProtection.Cast(sim, &agent.Unit) {
				t.Fatal("Forbearance failed to reject another immunity")
			}
			assertFloat64(t, "failed immunity casts do not spend mana", agent.CurrentMana(), before)
		})
		classic60PaladinAt(agent, 61*time.Second+time.Nanosecond, func(sim *Simulation) {
			if agent.spells.ForbearanceAura.IsActive() || !agent.spells.DivineShield.Cast(sim, &agent.Unit) {
				t.Fatal("Forbearance did not expire at sixty seconds")
			}
			if agent.spells.DivineProtection.CD.IsReady(sim) {
				t.Fatal("Divine Shield did not also start Divine Protection's five-minute cooldown")
			}
		})
	})
	classic60PaladinResult(t, sim.run())
}

func TestClassic60PaladinBubblesBreakAndPreventOwnedControl(t *testing.T) {
	for _, shield := range []bool{false, true} {
		sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 14 * time.Second, iterations: 2, pauseAutos: true}, func(agent *classic60PaladinTestAgent) {
			agent.spells.registerDefensiveCooldowns(&agent.Character, classic60PaladinTalents{})
			fear := agent.RegisterFearAura("Bubble test fear", ActionID{SpellID: 9991100}, 20*time.Second)
			stun := agent.RegisterStunAura("Bubble test stun", ActionID{SpellID: 9991101}, 20*time.Second)
			repentance := classic60RepentanceKind.registerAura(&agent.Unit, "Bubble test repentance", ActionID{SpellID: 9991102}, 20*time.Second)
			classic60PaladinAt(agent, 500*time.Millisecond, func(sim *Simulation) {
				if !ApplyFear(sim, fear) || !ApplyStun(sim, stun) || !classic60RepentanceKind.apply(sim, repentance) {
					t.Fatal("control immunity leaked across iterations")
				}
			})
			classic60PaladinAt(agent, time.Second, func(sim *Simulation) {
				spell := agent.spells.DivineProtection
				if shield {
					spell = agent.spells.DivineShield
				}
				if !spell.Cast(sim, &agent.Unit) {
					t.Fatal("bubble could not be cast while controlled")
				}
				if fear.IsActive() || stun.IsActive() || repentance.IsActive() || agent.PseudoStats.Incapacitated || agent.PseudoStats.Stunned {
					t.Fatal("bubble failed to remove owned crowd control")
				}
				if ApplyFear(sim, fear) || ApplyStun(sim, stun) || classic60RepentanceKind.apply(sim, repentance) {
					t.Fatal("crowd control bypassed the active bubble")
				}
			})
			classic60PaladinAt(agent, 13500*time.Millisecond, func(sim *Simulation) {
				if agent.PseudoStats.FearImmune || agent.PseudoStats.StunImmune || agent.classic60RepentanceImmune {
					t.Fatal("control immunity outlived its bubble")
				}
				if !ApplyFear(sim, fear) || !ApplyStun(sim, stun) || !classic60RepentanceKind.apply(sim, repentance) {
					t.Fatal("control did not work after bubble expiration")
				}
			})
		})
		classic60PaladinResult(t, sim.run())
	}
}

func TestClassic60PaladinDivineShieldComposesWithCrusader(t *testing.T) {
	sim, _ := newClassic60PaladinTestSim(t, classic60PaladinTestConfig{duration: 16 * time.Second, iterations: 2}, func(agent *classic60PaladinTestAgent) {
		agent.spells.registerAdditionalSeals(&agent.Character, classic60PaladinTalents{})
		agent.spells.registerDefensiveCooldowns(&agent.Character, classic60PaladinTalents{})
		classic60PaladinAt(agent, 100*time.Millisecond, func(sim *Simulation) {
			assertFloat64(t, "speed resets across iterations", agent.PseudoStats.MeleeSpeedMultiplier, 1)
			if !agent.spells.SealOfTheCrusader.Cast(sim, agent.CurrentTarget) {
				t.Fatal("Crusader failed")
			}
		})
		classic60PaladinAt(agent, 2100*time.Millisecond, func(sim *Simulation) {
			if !agent.spells.DivineShield.Cast(sim, &agent.Unit) {
				t.Fatal("Divine Shield failed with Crusader")
			}
			assertFloat64(t, "Divine Shield composes with Crusader", agent.PseudoStats.MeleeSpeedMultiplier, .7)
		})
		classic60PaladinAt(agent, 14200*time.Millisecond, func(sim *Simulation) {
			assertFloat64(t, "shield expiry retains Crusader speed", agent.PseudoStats.MeleeSpeedMultiplier, 1.4)
			if !agent.spells.SealOfCommand.Cast(sim, agent.CurrentTarget) {
				t.Fatal("Command failed to replace Crusader")
			}
			assertFloat64(t, "seal replacement restores base speed", agent.PseudoStats.MeleeSpeedMultiplier, 1)
		})
	})
	classic60PaladinResult(t, sim.run())
}
