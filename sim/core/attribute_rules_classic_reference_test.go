package core

import (
	"maps"
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func classicReferenceAttributeRuleVectors() attributeRules {
	// Literal installed dependencies from Classic@7779ebb class constructors,
	// not every entry in its AP/crit maps. Columns are AP/Str, AP/Agi,
	// RAP/Agi, physical crit percentage points/Agi, spell crit points/Int.
	return attributeRules{
		healthPerStamina: 10, playerHealthOffset: -180, armorPerAgility: 2,
		manaPerIntellect: 15, manaOffset: -280,
		classes: [proto.Class_ClassDruid + 1]classAttributeRules{
			proto.Class_ClassWarrior: {2, 0, 0, 0.0500, 0},
			proto.Class_ClassPaladin: {2, 0, 0, 0.0506, 0.0167},
			proto.Class_ClassHunter:  {1, 1, 2, 0.0189, 0.0165},
			proto.Class_ClassRogue:   {1, 1, 1, 0.0345, 0},
			proto.Class_ClassPriest:  {1, 0, 0, 0, 0.0168},
			proto.Class_ClassShaman:  {2, 0, 0, 0.0508, 0.0169},
			proto.Class_ClassMage:    {1, 0, 0, 0, 0.0168},
			proto.Class_ClassWarlock: {1, 0, 0, 0.0500, 0.0165},
			proto.Class_ClassDruid:   {2, 0, 0, 0.0500, 0.0167},
		},
	}
}

func TestClassicReferenceLevel60AttributeRules(t *testing.T) {
	for _, bossDelta := range []int32{-2147483648, -4, 0, 3, 99, 2147483647} {
		rules := rulesetWithCharacterLevel(60)
		rules.levels.defaultBossLevelDelta = bossDelta
		got, ok := classicReferenceLevel60AttributeRules(rules)
		if !ok || got != classicReferenceAttributeRuleVectors() {
			t.Fatalf("boss delta %d: got %+v, %v; want literal Classic dependencies", bossDelta, got, ok)
		}
	}
}

func TestClassicReferenceAttributeRuleConversions(t *testing.T) {
	// Integer primary inputs deliberately avoid claiming equivalence with
	// Classic's fractional dependency behavior: the modern manager floors
	// primary sources whereas Classic@7779ebb stats/deps.go does not.
	tests := []struct {
		class                                                proto.Class
		hasMana                                              bool
		mana, attackPower, rangedAP, physicalCrit, spellCrit float64
	}{
		{proto.Class_ClassWarrior, false, 2000, 230, 50, 5.5, 2.5},
		{proto.Class_ClassPaladin, true, 3520, 230, 50, 5.548, 4.504},
		{proto.Class_ClassHunter, true, 3520, 210, 210, 3.012, 4.48},
		{proto.Class_ClassRogue, false, 2000, 210, 130, 4.26, 2.5},
		{proto.Class_ClassPriest, true, 3520, 130, 50, 1.5, 4.516},
		{proto.Class_ClassShaman, true, 3520, 230, 50, 5.564, 4.528},
		{proto.Class_ClassMage, true, 3520, 130, 50, 1.5, 4.516},
		{proto.Class_ClassWarlock, true, 3520, 130, 50, 5.5, 4.48},
		{proto.Class_ClassDruid, true, 3520, 230, 50, 5.5, 4.504},
	}
	for _, test := range tests {
		t.Run(test.class.String(), func(t *testing.T) {
			rules := rulesetWithCharacterLevel(60)
			var ok bool
			rules.attributes, ok = classicReferenceLevel60AttributeRules(rules)
			if !ok {
				t.Fatal("level-60 attribute reference rejected")
			}
			raw := attributeRulesTestStats()
			character := attributeRulesTestCharacter(PlayerUnit, test.class, raw)
			character.addUniversalStatDependenciesWithRuleset(rules)
			character.addBaseClassStatDependenciesWithRuleset(rules)
			if test.hasMana {
				character.addManaStatDependenciesWithRuleset(rules)
			}
			want := raw
			want[stats.Health], want[stats.Mana], want[stats.Armor] = 1420, test.mana, 260
			want[stats.AttackPower], want[stats.RangedAttackPower] = test.attackPower, test.rangedAP
			want[stats.PhysicalCritPercent], want[stats.SpellCritPercent] = test.physicalCrit, test.spellCrit
			for range 3 {
				got := character.SortAndApplyStatDependencies(character.GetStats())
				for stat := range want {
					assertFloat64(t, stats.Stat(stat).StatName(), got[stat], want[stat])
				}
			}
			if character.GetBaseStats() != raw {
				t.Fatal("conversion changed raw base stats or spell-cost base mana")
			}
		})
	}
}

func TestClassicReferenceAttributeRulesComposeRawBaseAttributes(t *testing.T) {
	// No ClassBaseCrit, racial multiplier, talents, forms or gear are included.
	// These are reference compositions, not final naked-character stat lines.
	tests := []struct {
		race                                                                proto.Race
		class                                                               proto.Class
		health, mana, armor, attackPower, rangedAP, physicalCrit, spellCrit float64
	}{
		{race: proto.Race_RaceHuman, class: proto.Class_ClassPaladin, health: 2201, mana: 2282, armor: 130, attackPower: 370, physicalCrit: 3.289, spellCrit: 1.169},
		{race: proto.Race_RaceNightElf, class: proto.Class_ClassHunter, health: 2177, mana: 2415, armor: 260, attackPower: 282, rangedAP: 360, physicalCrit: 2.457, spellCrit: 1.0725},
		{race: proto.Race_RaceNightElf, class: proto.Class_ClassDruid, health: 1993, mana: 2464, armor: 130, attackPower: 104, physicalCrit: 3.25, spellCrit: 1.67},
	}
	for _, test := range tests {
		t.Run(test.race.String()+"/"+test.class.String(), func(t *testing.T) {
			rules := rulesetWithCharacterLevel(60)
			base, ok := classicReferenceLevel60BaseAttributes(rules, test.race, test.class)
			if !ok {
				t.Fatal("supported base attributes rejected")
			}
			rules.attributes, ok = classicReferenceLevel60AttributeRules(rules)
			if !ok {
				t.Fatal("level-60 conversions rejected")
			}
			raw := stats.Stats{
				stats.Strength: base.strength, stats.Agility: base.agility, stats.Stamina: base.stamina,
				stats.Intellect: base.intellect, stats.Spirit: base.spirit, stats.Health: base.health,
				stats.Mana: base.mana, stats.AttackPower: base.attackPower, stats.RangedAttackPower: base.rangedAttackPower,
			}
			character := attributeRulesTestCharacter(PlayerUnit, test.class, raw)
			character.addUniversalStatDependenciesWithRuleset(rules)
			character.addBaseClassStatDependenciesWithRuleset(rules)
			character.addManaStatDependenciesWithRuleset(rules)
			got := character.SortAndApplyStatDependencies(character.GetStats())
			want := raw
			want[stats.Health], want[stats.Mana], want[stats.Armor] = test.health, test.mana, test.armor
			want[stats.AttackPower], want[stats.RangedAttackPower] = test.attackPower, test.rangedAP
			want[stats.PhysicalCritPercent], want[stats.SpellCritPercent] = test.physicalCrit, test.spellCrit
			for stat := range want {
				assertFloat64(t, stats.Stat(stat).StatName(), got[stat], want[stat])
			}
		})
	}
}

func TestClassicReferenceAttributeRulesResourceBoundary(t *testing.T) {
	rules := rulesetWithCharacterLevel(60)
	var ok bool
	rules.attributes, ok = classicReferenceLevel60AttributeRules(rules)
	if !ok {
		t.Fatal("level-60 reference rejected")
	}
	for _, test := range []struct{ attribute, health, mana float64 }{{20, 1020, 2020}, {21, 1030, 2035}} {
		raw := stats.Stats{stats.Stamina: test.attribute, stats.Intellect: test.attribute, stats.Health: 1000, stats.Mana: 2000}
		character := attributeRulesTestCharacter(PlayerUnit, proto.Class_ClassPaladin, raw)
		character.addUniversalStatDependenciesWithRuleset(rules)
		character.addManaStatDependenciesWithRuleset(rules)
		got := character.SortAndApplyStatDependencies(character.GetStats())
		assertFloat64(t, "health at supported resource boundary", got[stats.Health], test.health)
		assertFloat64(t, "mana at supported resource boundary", got[stats.Mana], test.mana)
	}
}

func TestClassicReferenceAttributeRulesScopeFailsClosed(t *testing.T) {
	levels := []int32{-2147483648, 2147483647}
	for level := int32(-1); level <= 100; level++ {
		if level != 60 {
			levels = append(levels, level)
		}
	}
	for _, level := range levels {
		got, ok := classicReferenceLevel60AttributeRules(rulesetWithCharacterLevel(level))
		if ok || got != (attributeRules{}) {
			t.Errorf("level %d: got %+v, %v; want no reference", level, got, ok)
		}
	}
	rules, ok := classicReferenceLevel60AttributeRules(rulesetWithCharacterLevel(60))
	if !ok {
		t.Fatal("level-60 reference rejected")
	}
	for _, class := range []proto.Class{-2147483648, -1, 0, 6, 10, 12, 999, 2147483647} {
		if got := rules.forClass(class); got != (classAttributeRules{}) {
			t.Errorf("class %d: got %+v, want no conversion", class, got)
		}
	}
}

func TestClassicReferenceAttributeRulesIsolationAndInactivity(t *testing.T) {
	active := currentRuleset()
	beforeBaseStats, beforeClassStats := maps.Clone(BaseStats), maps.Clone(ClassBaseStats)
	beforeRaceOffsets, beforeExtraStats := maps.Clone(RaceOffsets), maps.Clone(ExtraClassBaseStats)
	beforeAgiCrit, beforeIntCrit := maps.Clone(CritPerAgiMaxLevel), maps.Clone(CritPerIntMaxLevel)
	base := rulesetWithCharacterLevel(60)
	base.attributes = attributeRules{healthPerStamina: -999, playerHealthOffset: 999, armorPerAgility: -999, manaPerIntellect: -999, manaOffset: 999}
	for index := range base.attributes.classes {
		base.attributes.classes[index] = classAttributeRules{999, 999, 999, 999, 999}
	}
	beforeBase := base
	got, ok := classicReferenceLevel60AttributeRules(base)
	if !ok || got != classicReferenceAttributeRuleVectors() || base != beforeBase {
		t.Fatalf("reference inherited poisoned coefficients or mutated input: %+v, %v", got, ok)
	}
	got.healthPerStamina, got.playerHealthOffset, got.armorPerAgility = -999, 999, -999
	got.manaPerIntellect, got.manaOffset = -999, 999
	for index := range got.classes {
		got.classes[index] = classAttributeRules{999, 999, 999, 999, 999}
	}
	again, ok := classicReferenceLevel60AttributeRules(base)
	if !ok || again != classicReferenceAttributeRuleVectors() || again == got || base != beforeBase {
		t.Fatal("returned rules alias a previous result or input")
	}
	if currentRuleset() != active || active != inheritedTBCRuleset() || active.levels.characterLevel != 70 || CharacterLevel != 70 {
		t.Fatal("inactive reference changed active level-70 TBC profile")
	}
	if got, ok := classicReferenceLevel60AttributeRules(active); ok || got != (attributeRules{}) {
		t.Fatalf("reference accepted active TBC profile: %+v, %v", got, ok)
	}
	if !maps.Equal(BaseStats, beforeBaseStats) || !maps.Equal(ClassBaseStats, beforeClassStats) ||
		!maps.Equal(RaceOffsets, beforeRaceOffsets) || !maps.Equal(ExtraClassBaseStats, beforeExtraStats) ||
		!maps.Equal(CritPerAgiMaxLevel, beforeAgiCrit) || !maps.Equal(CritPerIntMaxLevel, beforeIntCrit) {
		t.Fatal("reference mutated active base-stat or crit tables")
	}
}
