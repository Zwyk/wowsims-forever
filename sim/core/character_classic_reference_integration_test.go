package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func TestClassicReferenceInitializedDruidCatAP(t *testing.T) {
	for _, test := range []struct {
		race          proto.Race
		humanoid, cat float64
	}{
		{proto.Race_RaceNightElf, 104, 289},
		{proto.Race_RaceTauren, 120, 295},
	} {
		t.Run(test.race.String(), func(t *testing.T) {
			baseline, ok := classicReferenceInitializeCharacter(60, test.race, proto.Class_ClassDruid)
			if !ok {
				t.Fatal("supported Druid baseline rejected")
			}
			before := baseline
			rules := rulesetProfile{levels: levelRules{characterLevel: baseline.level}}
			cat, ok := classicReferenceLevel60CatAttackPower(rules)
			if !ok {
				t.Fatal("initialized Druid level cannot resolve Cat AP")
			}
			// The initializer already resolved the baseline 2 AP/Strength.
			// Only Cat's additional AP dependencies belong in this graph.
			manager := stats.NewStatDependencyManager()
			agilityAP := manager.NewDynamicStatDependency(stats.Agility, stats.AttackPower, cat.perAgility)
			feralAP := manager.NewDynamicStatDependency(stats.FeralAttackPower, stats.AttackPower, cat.perFeralAttackPower)
			manager.FinalizeStatDeps()
			for _, inCat := range []bool{false, true, false, true, false} {
				input := baseline.stats
				want := input
				want[stats.AttackPower] = test.humanoid
				if inCat {
					manager.EnableDynamicStatDep(agilityAP)
					manager.EnableDynamicStatDep(feralAP)
					input[stats.AttackPower] += cat.flatBonus
					want[stats.AttackPower] = test.cat
				} else {
					manager.DisableDynamicStatDep(agilityAP)
					manager.DisableDynamicStatDep(feralAP)
				}
				got := manager.ApplyStatDependencies(input)
				for stat := range want {
					assertFloat64(t, stats.Stat(stat).StatName(), got[stat], want[stat])
				}
			}
			if baseline != before {
				t.Fatal("Cat AP composition mutated the initialized baseline")
			}
		})
	}
}

func TestClassicReferenceInitializedArmorAndResistanceInputs(t *testing.T) {
	// Compose existing numerical policies from resolved character data, with no
	// active TBC rules. This is not an incoming attack table or simulated fight.
	for _, test := range []struct {
		race             proto.Race
		class            proto.Class
		armor            float64
		damageMultiplier float64
	}{
		{proto.Race_RaceHuman, proto.Class_ClassWarrior, 160, 0.9729501267962807},
		{proto.Race_RaceNightElf, proto.Class_ClassRogue, 270, 0.9551867219917013},
	} {
		t.Run(test.race.String()+"/"+test.class.String(), func(t *testing.T) {
			baseline, ok := classicReferenceInitializeCharacter(60, test.race, test.class)
			if !ok {
				t.Fatal("supported baseline rejected")
			}
			before := baseline
			rules := rulesetProfile{levels: levelRules{characterLevel: baseline.level, defaultBossLevelDelta: 3}}
			assertFloat64(t, "resolved armor", baseline.stats[stats.Armor], test.armor)
			modifier, ok := classicReferenceArmorDamageModifier(rules, classicReferenceArmorInput{
				attackerLevel: rules.levels.defaultBossLevel(), defenderArmor: baseline.stats[stats.Armor],
			})
			if !ok {
				t.Fatal("initialized armor rejected by Classic policy")
			}
			assertFloat64(t, "level-63 attack armor modifier", modifier, test.damageMultiplier)

			// No racial resistance has been applied at this stage, even for NE.
			input := classicReferenceResistanceInput{
				attackerLevel: rules.levels.defaultBossLevel(), defenderLevel: baseline.level,
				defenderIsEnemy: false, selectedResistance: baseline.stats[stats.NatureResistance],
			}
			partial, ok := classicReferencePartialResistProjection(rules, input, false)
			if !ok || partial != (classicReferencePartialResistView{}) {
				t.Fatalf("pre-racial zero resistance projection = %+v, %v", partial, ok)
			}
			binary, ok := classicReferenceBinaryResistProjection(rules, input)
			if !ok || binary != (classicReferenceBinaryResistView{baseHitChanceMultiplier: 1}) {
				t.Fatalf("pre-racial binary resistance projection = %+v, %v", binary, ok)
			}
			if baseline != before {
				t.Fatal("mitigation composition mutated the initialized baseline")
			}
		})
	}
}
