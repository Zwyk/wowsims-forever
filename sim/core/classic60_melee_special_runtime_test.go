package core

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
	googleProto "google.golang.org/protobuf/proto"
)

func classic60SpecialOutcomeFixture(source WeaponAttackSource) (*Character, *Spell, *AttackTable) {
	character, spell, table := classicMeleeOutcomeFixture()
	character.Equipment[proto.ItemSlot_ItemSlotMainHand].HandType = proto.HandType_HandTypeOneHand
	character.Equipment[proto.ItemSlot_ItemSlotOffHand] = Item{
		ID: 2, Type: proto.ItemType_ItemTypeWeapon,
		WeaponType: proto.WeaponType_WeaponTypeAxe, HandType: proto.HandType_HandTypeOneHand,
	}
	character.AutoAttacks.IsDualWielding = true
	character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategorySwords, 5)
	character.stats[stats.PhysicalCritPercent] = 24
	spell.weaponAttackSource = source
	spell.ProcMask = ProcMaskMeleeMHSpecial
	if source == WeaponAttackSourceOffHand {
		spell.ProcMask = ProcMaskMeleeOHSpecial
	}
	// These pair-wide TBC seeds must not affect either hand's Classic special.
	table.BaseMissChance, table.HitSuppression = 0.91, 0.92
	table.BaseDodgeChance, table.BaseGlanceChance = 0.93, 0.94
	table.MeleeCritSuppression = 0.95
	return character, spell, table
}

func TestClassic60SpecialOutcomeBoundariesAndRandomDraws(t *testing.T) {
	// Pinned Classic spell_outcome.go: rear weapon specials first roll miss +
	// dodge, then roll crit independently only if the attack landed. With
	// 305/300 skill against 315 defense the miss/dodge pairs are .06/.06 and
	// .08/.065. Both hands have 24% - 4.8% = 19.2% conditional crit.
	for _, hand := range []struct {
		name         string
		source       WeaponAttackSource
		miss, landed float64
	}{
		{"main hand", WeaponAttackSourceMainHand, 0.06, 0.12},
		{"off hand", WeaponAttackSourceOffHand, 0.08, 0.145},
	} {
		for _, test := range []struct {
			name        string
			hit, crit   float64
			outcome     HitOutcome
			damage      float64
			randomDraws int
		}{
			{"miss", hand.miss - 0.0001, 0, OutcomeMiss, 0, 1},
			{"dodge at miss boundary", hand.miss, 0, OutcomeDodge, 0, 1},
			{"dodge before landed boundary", hand.landed - 0.0001, 0, OutcomeDodge, 0, 1},
			{"crit", hand.landed + 0.0001, 0.1919, OutcomeCrit, 200, 2},
			{"hit", hand.landed + 0.0001, 0.1921, OutcomeHit, 100, 2},
		} {
			t.Run(hand.name+"/"+test.name, func(t *testing.T) {
				_, spell, table := classic60SpecialOutcomeFixture(hand.source)
				assertFloat64(t, "special hit-check excludes dual-wield penalty", spell.GetPhysicalMissChance(table), hand.miss)
				roll := &sequenceMeleeRoll{values: []float64{test.hit, test.crit}}
				result := &SpellResult{Target: table.Defender, Damage: 100}
				spell.OutcomeMeleeWeaponSpecialHitAndCrit(&Simulation{rand: roll}, result, table)
				if result.Outcome != test.outcome || roll.calls != test.randomDraws {
					t.Fatalf("special outcome/draws = %v/%d, want %v/%d", result.Outcome, roll.calls, test.outcome, test.randomDraws)
				}
				assertFloat64(t, "special damage", result.Damage, test.damage)
				assertFloat64(t, "shared miss seed remains unchanged", table.BaseMissChance, 0.91)
				assertFloat64(t, "shared dodge seed remains unchanged", table.BaseDodgeChance, 0.93)
			})
		}
	}
}

func TestClassic60ExpectedSpecialUsesConditionalCritAndWeaponSkill(t *testing.T) {
	for _, test := range []struct {
		name                 string
		source               WeaponAttackSource
		hit, crit, base, want float64
	}{
		{"305 main hand", WeaponAttackSourceMainHand, 0, 24, 100, 104.896},
		{"300 off hand", WeaponAttackSourceOffHand, 0, 24, 100, 101.916},
		{"main hand hit capped", WeaponAttackSourceMainHand, 9, 24, 100, 112.048},
		{"off hand hit capped", WeaponAttackSourceOffHand, 9, 24, 100, 111.452},
		{"critical chance capped", WeaponAttackSourceMainHand, 0, 110, 100, 176},
		{"critical chance floored", WeaponAttackSourceOffHand, 0, 0, 100, 85.5},
		{"zero damage", WeaponAttackSourceOffHand, 0, 24, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			character, spell, table := classic60SpecialOutcomeFixture(test.source)
			character.stats[stats.PhysicalHitPercent] = test.hit
			character.stats[stats.PhysicalCritPercent] = test.crit
			result := &SpellResult{Target: table.Defender, Damage: test.base}
			spell.OutcomeExpectedMeleeWeaponSpecialHitAndCrit(nil, result, table)
			assertFloat64(t, "expected special damage", result.Damage, test.want)
		})
	}
}

func TestClassic60SpecialRejectsMismatchedOutcomeKind(t *testing.T) {
	for _, test := range []struct {
		name  string
		mask  ProcMask
		apply func(*Spell, *SpellResult, *AttackTable)
	}{
		{"special passed to white", ProcMaskMeleeMHSpecial, func(s *Spell, r *SpellResult, a *AttackTable) {
			s.OutcomeMeleeWhite(&Simulation{rand: &sequenceMeleeRoll{values: []float64{0.9, 0.5}}}, r, a)
		}},
		{"special passed to expected white", ProcMaskMeleeMHSpecial, func(s *Spell, r *SpellResult, a *AttackTable) {
			s.OutcomeExpectedMeleeWhite(nil, r, a)
		}},
		{"white passed to special", ProcMaskMeleeMHAuto, func(s *Spell, r *SpellResult, a *AttackTable) {
			s.OutcomeMeleeWeaponSpecialHitAndCrit(&Simulation{rand: &sequenceMeleeRoll{values: []float64{0.9, 0.5}}}, r, a)
		}},
		{"white passed to expected special", ProcMaskMeleeMHAuto, func(s *Spell, r *SpellResult, a *AttackTable) {
			s.OutcomeExpectedMeleeWeaponSpecialHitAndCrit(nil, r, a)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, spell, table := classic60SpecialOutcomeFixture(WeaponAttackSourceMainHand)
			spell.ProcMask = test.mask
			defer func() {
				if recover() == nil {
					t.Fatal("mismatched Classic outcome kind did not fail closed")
				}
			}()
			test.apply(spell, &SpellResult{Target: table.Defender, Damage: 100}, table)
		})
	}
}

type classic60SpecialCastRecord struct {
	seed    int64
	at      time.Duration
	source  WeaponAttackSource
	outcome HitOutcome
	damage  float64
}

func newClassic60SpecialTestSim(t *testing.T, config classic60MeleeTestConfig) (*Simulation, *[]classic60SpecialCastRecord) {
	t.Helper()
	casts := []classic60SpecialCastRecord{}
	sim, _, _ := newClassic60MeleeTestSim(t, config, func(agent *FakeAgent) {
		for _, hand := range []struct {
			source WeaponAttackSource
			mask   ProcMask
			tag    int32
			period time.Duration
		}{
			{WeaponAttackSourceMainHand, ProcMaskMeleeMHSpecial, 101, 3 * time.Second},
			{WeaponAttackSourceOffHand, ProcMaskMeleeOHSpecial, 102, 5 * time.Second},
		} {
			// Diagnostic actions exercise core scheduling only. They represent
			// no actual class ability, resource cost, talent or spell rank.
			spell := agent.RegisterSpell(SpellConfig{
				ActionID:           ActionID{OtherID: proto.OtherAction_OtherActionAttack, Tag: hand.tag},
				SpellSchool:        SpellSchoolPhysical,
				DefenseType:        DefenseTypeMelee,
				ProcMask:           hand.mask,
				WeaponAttackSource: hand.source,
				Flags:              SpellFlagMeleeMetrics | SpellFlagNoOnCastComplete,

				Cast: CastConfig{CD: Cooldown{Timer: agent.NewTimer(), Duration: hand.period}},

				BonusCritPercent:         20,
				DamageMultiplier:         1,
				DamageMultiplierAdditive: 1,
				ThreatMultiplier:         1,

				ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
					var baseDamage float64
					if hand.source == WeaponAttackSourceMainHand {
						baseDamage = agent.MHNormalizedWeaponDamage(sim, spell.MeleeAttackPower(target))
					} else {
						baseDamage = agent.OHNormalizedWeaponDamage(sim, spell.MeleeAttackPower(target))
					}
					spell.CalcAndDealDamage(sim, target, baseDamage, spell.OutcomeMeleeWeaponSpecialHitAndCrit)
				},
			})
			agent.RegisterAura(Aura{
				Label: "Schedule Classic diagnostic " + spell.ActionID.String(), Duration: NeverExpires,
				OnReset: func(aura *Aura, sim *Simulation) {
					aura.Activate(sim)
					StartPeriodicAction(sim, PeriodicActionOptions{
						Period: hand.period, TickImmediately: true,
						OnAction: func(sim *Simulation) {
							if !spell.CanCast(sim, agent.CurrentTarget) || !spell.Cast(sim, agent.CurrentTarget) {
								t.Fatal("diagnostic special was not ready on its scheduled cooldown")
							}
							if spell.CanCast(sim, agent.CurrentTarget) || spell.Cast(sim, agent.CurrentTarget) {
								t.Fatal("diagnostic special bypassed its cooldown")
							}
						},
					})
				},
				OnSpellHitDealt: func(_ *Aura, sim *Simulation, actual *Spell, result *SpellResult) {
					if actual == spell {
						casts = append(casts, classic60SpecialCastRecord{sim.currentSeed, sim.CurrentTime, hand.source, result.Outcome, result.Damage})
					}
				},
			})
		}
	})
	return sim, &casts
}

func TestClassic60SpecialRuntimeCastsCooldownsMetricsAndReset(t *testing.T) {
	config := classic60MeleeTestConfig{
		targetLevel: 63, armor: 5500, skillBonus: 5, dualWield: true,
		duration: 30 * time.Second, iterations: 3, seed: 940,
	}
	sim, casts := newClassic60SpecialTestSim(t, config)
	result := sim.run()
	if result.Error != nil {
		t.Fatalf("scheduled special simulation failed: %v", result.Error)
	}
	for _, hand := range []struct {
		source       WeaponAttackSource
		tag          int32
		period       time.Duration
		castsPerRun  int
		normalDamage float64
	}{
		// AP400, normalized speed2.4: MH 100+2.4*400/14=1180/7;
		// OH .5*(80+2.4*400/14)=520/7. Level60 armor constant5500
		// halves both against armor5500, irrespective of target level63.
		{WeaponAttackSourceMainHand, 101, 3 * time.Second, 11, 590.0 / 7},
		{WeaponAttackSourceOffHand, 102, 5 * time.Second, 7, 260.0 / 7},
	} {
		count, damage := 0, 0.0
		for _, cast := range *casts {
			if cast.source != hand.source {
				continue
			}
			if cast.seed != config.seed+int64(count/hand.castsPerRun) || cast.at != time.Duration(count%hand.castsPerRun)*hand.period {
				t.Fatalf("special cooldown or iteration timing did not reset: %+v", cast)
			}
			want := 0.0
			switch cast.outcome {
			case OutcomeHit:
				want = hand.normalDamage
			case OutcomeCrit:
				want = 2 * hand.normalDamage
			case OutcomeMiss, OutcomeDodge:
			default:
				t.Fatalf("rear special produced unsupported outcome: %+v", cast)
			}
			assertFloat64(t, "normalized special damage after armor", cast.damage, want)
			damage += cast.damage
			count++
		}
		if count != int(config.iterations)*hand.castsPerRun {
			t.Fatalf("hand %v special count=%d", hand.source, count)
		}
		var metrics *proto.TargetedActionMetrics
		for _, action := range result.RaidMetrics.Parties[0].Players[0].Actions {
			if action.Id.GetOtherId() == proto.OtherAction_OtherActionAttack && action.Id.Tag == hand.tag {
				for _, target := range action.Targets {
					if target.UnitIndex == 0 {
						metrics = target
					}
				}
			}
		}
		if metrics == nil || int(metrics.Casts) != count || metrics.Hits+metrics.Crits+metrics.Misses+metrics.Dodges != metrics.Casts || metrics.Glances != 0 {
			t.Fatalf("hand %v special metrics do not match scheduled outcomes: %+v", hand.source, metrics)
		}
		assertFloat64(t, "special damage is included in report", metrics.Damage, damage)
		if math.IsNaN(damage) || math.IsInf(damage, 0) || damage <= 0 {
			t.Fatal("scheduled specials did not produce finite damage")
		}
	}
	repeat, repeatCasts := newClassic60SpecialTestSim(t, config)
	if !googleProto.Equal(result, repeat.run()) || !reflect.DeepEqual(*casts, *repeatCasts) {
		t.Fatal("scheduled specials were not reproducible with identical seeds")
	}
}
