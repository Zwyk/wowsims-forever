package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func TestClassicReferenceCatAttackPowerTransitions(t *testing.T) {
	// Literal no-talents AP vectors characterize executable Humanoid/Cat code:
	// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/druid/forms.go#L86-L100
	// Primary attributes are integers: this does not assert parity between the
	// Classic dependency manager and TBC's source-attribute flooring behavior.
	rules := inheritedTBCRuleset()
	rules.levels.characterLevel = 60
	attributes, ok := classicReferenceLevel60AttributeRules(rules)
	if !ok {
		t.Fatal("level-60 attribute reference is unavailable")
	}
	cat, ok := classicReferenceLevel60CatAttackPower(rules)
	if !ok {
		t.Fatal("level-60 Cat AP reference is unavailable")
	}
	rules.attributes = attributes
	// Isolate AP dependencies; crit, health, mana and other form effects are
	// tested separately or outside the scope of this small Cat AP reference.
	rules.attributes.classes[proto.Class_ClassDruid].physicalCritPercentPerAgility = 0

	tests := []struct {
		name              string
		race              proto.Race
		feralAttackPower  float64
		humanoidAP, catAP float64
	}{
		{name: "NightElf", race: proto.Race_RaceNightElf, humanoidAP: 104, catAP: 289},
		{name: "Tauren", race: proto.Race_RaceTauren, humanoidAP: 120, catAP: 295},
		{name: "Synthetic", humanoidAP: 180, catAP: 380},
		{name: "SyntheticFeralAP", feralAttackPower: 73.5, humanoidAP: 180, catAP: 453.5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := stats.Stats{
				stats.Strength: 100, stats.Agility: 80, stats.Stamina: 60,
				stats.Intellect: 120, stats.Spirit: 40, stats.AttackPower: -20,
				stats.FeralAttackPower: test.feralAttackPower,
			}
			if test.race != proto.Race_RaceUnknown {
				base, supported := classicReferenceLevel60BaseAttributes(rules, test.race, proto.Class_ClassDruid)
				if !supported {
					t.Fatal("original Druid race is not supported by the base reference")
				}
				raw = stats.Stats{
					stats.Strength: base.strength, stats.Agility: base.agility,
					stats.Stamina: base.stamina, stats.Intellect: base.intellect, stats.Spirit: base.spirit,
					stats.Health: base.health, stats.Mana: base.mana,
					stats.AttackPower: base.attackPower, stats.RangedAttackPower: base.rangedAttackPower,
				}
			}
			character := attributeRulesTestCharacter(PlayerUnit, proto.Class_ClassDruid, raw)
			character.addBaseClassStatDependenciesWithRuleset(rules)
			manager := &character.StatDependencyManager
			agilityAP := manager.NewDynamicStatDependency(stats.Agility, stats.AttackPower, cat.perAgility)
			feralAP := manager.NewDynamicStatDependency(stats.FeralAttackPower, stats.AttackPower, cat.perFeralAttackPower)
			manager.FinalizeStatDeps()

			// These are dependency-manager transitions, not active runtime auras.
			// The raw -20 AP remains separate from Cat's temporary flat bonus.
			for _, step := range []struct {
				name string
				cat  bool
				want float64
			}{
				{"Humanoid", false, test.humanoidAP},
				{"Cat", true, test.catAP},
				{"HumanoidAgain", false, test.humanoidAP},
				{"CatAgain", true, test.catAP},
			} {
				t.Run(step.name, func(t *testing.T) {
					input := raw
					if step.cat {
						manager.EnableDynamicStatDep(agilityAP)
						manager.EnableDynamicStatDep(feralAP)
						input[stats.AttackPower] += cat.flatBonus
					} else {
						manager.DisableDynamicStatDep(agilityAP)
						manager.DisableDynamicStatDep(feralAP)
					}
					got := manager.ApplyStatDependencies(input)
					want := raw
					want[stats.AttackPower] = step.want
					for stat := range want {
						assertFloat64(t, stats.Stat(stat).StatName(), got[stat], want[stat])
					}
					if character.GetStats() != raw || character.GetBaseStats() != raw {
						t.Fatal("reference dependency transition changed the raw/base stats")
					}
				})
			}
		})
	}
}

func TestClassicReferenceCatAttackPowerScopeAndIsolation(t *testing.T) {
	active := currentRuleset()
	rules := inheritedTBCRuleset()
	for _, level := range []int32{-2147483648, -1, 0, 1, 59, 60, 61, 70, 2147483647} {
		rules.levels.characterLevel = level
		got, ok := classicReferenceLevel60CatAttackPower(rules)
		if level == 60 {
			if !ok || got != (classicReferenceCatAttackPower{120, 1, 1}) {
				t.Errorf("level 60 Cat AP reference = %+v, %v; want 120/1/1, true", got, ok)
			}
		} else if ok || got != (classicReferenceCatAttackPower{}) {
			t.Errorf("level %d unexpectedly supports Cat AP: %+v, %v", level, got, ok)
		}
	}
	if got, ok := classicReferenceLevel60CatAttackPower(active); ok || got != (classicReferenceCatAttackPower{}) {
		t.Fatalf("active level-70 rules accepted the Cat reference: %+v, %v", got, ok)
	}

	rules.levels.characterLevel = 60
	before := rules
	got, ok := classicReferenceLevel60CatAttackPower(rules)
	if !ok {
		t.Fatal("level-60 Cat AP reference is unavailable")
	}
	got.flatBonus, got.perAgility, got.perFeralAttackPower = -999, -999, -999
	again, ok := classicReferenceLevel60CatAttackPower(rules)
	if !ok || again != (classicReferenceCatAttackPower{120, 1, 1}) || again == got {
		t.Fatal("mutating the returned Cat AP value affected later lookups")
	}
	if rules != before || currentRuleset() != active || inheritedTBCRuleset() != active {
		t.Fatal("Cat AP reference lookup changed the input or active rules")
	}
}
