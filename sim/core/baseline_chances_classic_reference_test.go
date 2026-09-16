package core

import (
	"maps"
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
)

func classicReferenceBaselineChanceVectors() map[proto.Class]classicReferenceBaselineChances {
	// Independent literal source vectors, in percentage points. Columns:
	// physical crit, spell crit, base dodge, installed dodge/Agi, raw parry,
	// raw block, baseline parry capability. No raw block seed enables blocking.
	return map[proto.Class]classicReferenceBaselineChances{
		proto.Class_ClassWarrior: {0, 0, 0, 0.0500, 5, 5, true},
		proto.Class_ClassPaladin: {0.7, 3.5, 0.7, 0.0506, 5, 5, true},
		proto.Class_ClassHunter:  {0, 3.6, 0, 0, 5, 5, true},
		proto.Class_ClassRogue:   {0, 0, 0, 0.0690, 5, 5, true},
		proto.Class_ClassPriest:  {3, 0.8, 3, 0, 5, 5, false},
		proto.Class_ClassShaman:  {1.7, 2.3, 1.7, 0.0508, 5, 5, false},
		proto.Class_ClassMage:    {3.2, 0.2, 3.2, 0, 5, 5, false},
		proto.Class_ClassWarlock: {2, 1.7, 2, 0.0500, 5, 5, false},
		proto.Class_ClassDruid:   {0.9, 1.8, 0.9, 0.0500, 5, 5, false},
	}
}

func TestClassicReferenceLevel60BaselineChances(t *testing.T) {
	vectors := classicReferenceBaselineChanceVectors()
	if len(vectors) != 9 {
		t.Fatal("baseline chance fixture must cover all nine classes")
	}
	for class, want := range vectors {
		t.Run(class.String(), func(t *testing.T) {
			for _, bossDelta := range []int32{-2147483648, -4, 0, 3, 99, 2147483647} {
				rules := rulesetWithCharacterLevel(60)
				rules.levels.defaultBossLevelDelta = bossDelta
				got, ok := classicReferenceLevel60BaselineChances(rules, class)
				if !ok || got != want {
					t.Fatalf("boss delta %d: got %+v, %v; want %+v", bossDelta, got, ok, want)
				}
			}
		})
	}
}

func TestClassicReferenceBaselineChancesScopeFailsClosed(t *testing.T) {
	vectors := classicReferenceBaselineChanceVectors()
	classes := []proto.Class{-2147483648, 999, 2147483647}
	for class := proto.Class(-1); class <= 32; class++ {
		classes = append(classes, class)
	}
	for class := range proto.Class_name {
		classes = append(classes, proto.Class(class))
	}
	for _, class := range classes {
		want, supported := vectors[class]
		got, ok := classicReferenceLevel60BaselineChances(rulesetWithCharacterLevel(60), class)
		if ok != supported || got != want {
			t.Errorf("class %d: got %+v, %v; want %+v, %v", class, got, ok, want, supported)
		}
	}

	levels := []int32{-2147483648, 2147483647}
	for level := int32(-1); level <= 100; level++ {
		if level != 60 {
			levels = append(levels, level)
		}
	}
	for _, level := range levels {
		for class := range vectors {
			got, ok := classicReferenceLevel60BaselineChances(rulesetWithCharacterLevel(level), class)
			if ok || got != (classicReferenceBaselineChances{}) {
				t.Errorf("level %d/class %v: got %+v, %v; want no reference", level, class, got, ok)
			}
		}
	}
}

func TestClassicReferenceBaselineChancesIsolationAndInactivity(t *testing.T) {
	active := currentRuleset()
	beforeBaseStats, beforeClassStats := maps.Clone(BaseStats), maps.Clone(ClassBaseStats)
	beforeRaceOffsets, beforeExtraStats := maps.Clone(RaceOffsets), maps.Clone(ExtraClassBaseStats)
	beforeAgiCrit, beforeIntCrit := maps.Clone(CritPerAgiMaxLevel), maps.Clone(CritPerIntMaxLevel)
	// Chance values must not inherit the caller's conversions or depend on its
	// combat tables. Poison those inputs while retaining the supported level.
	rules := rulesetWithCharacterLevel(60)
	rules.ratings = ratingRules{}
	rules.ratings.physicalCritRatingPerCritPercent = -999
	rules.ratings.spellCritRatingPerCritPercent = 999
	rules.ratings.dodgeRatingPerDodgePercent = -999
	rules.ratings.parryRatingPerParryPercent = 999
	rules.ratings.blockRatingPerBlockPercent = -999
	rules.attributes, rules.combat = attributeRules{}, combatRules{}
	beforeRules := rules
	for class, want := range classicReferenceBaselineChanceVectors() {
		got, ok := classicReferenceLevel60BaselineChances(rules, class)
		if !ok || got != want {
			t.Fatalf("class %v inherited poisoned input: %+v, %v", class, got, ok)
		}
		got.physicalCritPercent, got.spellCritPercent = -999, -999
		got.dodgePercent, got.dodgePercentPerAgility = -999, -999
		got.parryPercent, got.blockPercent, got.canParry = -999, -999, !got.canParry
		again, ok := classicReferenceLevel60BaselineChances(rules, class)
		if !ok || again != want || again == got {
			t.Fatalf("class %v aliases a previous result: %+v, %v", class, again, ok)
		}
		if got, ok := classicReferenceLevel60BaselineChances(active, class); ok || got != (classicReferenceBaselineChances{}) {
			t.Fatalf("class %v accepted active TBC profile: %+v, %v", class, got, ok)
		}
	}
	if rules != beforeRules || currentRuleset() != active || active != inheritedTBCRuleset() || CharacterLevel != 70 || active.levels.characterLevel != 70 {
		t.Fatal("reference changed its input or the active inherited level-70 profile")
	}
	if !maps.Equal(BaseStats, beforeBaseStats) || !maps.Equal(ClassBaseStats, beforeClassStats) ||
		!maps.Equal(RaceOffsets, beforeRaceOffsets) || !maps.Equal(ExtraClassBaseStats, beforeExtraStats) ||
		!maps.Equal(CritPerAgiMaxLevel, beforeAgiCrit) || !maps.Equal(CritPerIntMaxLevel, beforeIntCrit) {
		t.Fatal("reference changed active base-stat or crit tables")
	}
}
