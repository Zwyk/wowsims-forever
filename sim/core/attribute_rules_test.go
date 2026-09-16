package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func attributeRulesTestCharacter(unitType UnitType, class proto.Class, raw stats.Stats) Character {
	return Character{
		Unit: Unit{
			Type:                  unitType,
			stats:                 raw,
			StatDependencyManager: stats.NewStatDependencyManager(),
		},
		Class: class, baseStats: raw,
	}
}

func attributeRulesTestStats() stats.Stats {
	return stats.Stats{
		stats.Strength: 100, stats.Agility: 80, stats.Stamina: 60, stats.Intellect: 120, stats.Spirit: 40,
		stats.Health: 1000, stats.Mana: 2000, stats.Armor: 100,
		stats.AttackPower: 30, stats.RangedAttackPower: 50,
		stats.PhysicalCritPercent: 1.5, stats.SpellCritPercent: 2.5,
	}
}

func TestAttributeRulesInheritedClassConversions(t *testing.T) {
	// Expected outputs are literal regression vectors, not computed from the
	// profile or generated maps under test. Mana setup is explicitly requested
	// even for Warrior/Rogue here to exercise every class coefficient.
	tests := []struct {
		class                                          proto.Class
		attackPower, rangedAP, physicalCrit, spellCrit float64
	}{
		{proto.Class_ClassWarrior, 230, 50, 3.924, 2.5},
		{proto.Class_ClassPaladin, 230, 50, 4.7, 4},
		{proto.Class_ClassHunter, 210, 130, 3.5, 4},
		{proto.Class_ClassRogue, 210, 50, 3.5, 2.5},
		{proto.Class_ClassPriest, 30, 50, 4.7, 4},
		{proto.Class_ClassShaman, 230, 50, 4.7, 4},
		{proto.Class_ClassMage, 30, 50, 4.7, 4},
		{proto.Class_ClassWarlock, 130, 50, 4.74, 3.964},
		{proto.Class_ClassDruid, 130, 50, 4.7, 4},
	}
	for _, test := range tests {
		t.Run(test.class.String(), func(t *testing.T) {
			rules := inheritedTBCRuleset()
			character := attributeRulesTestCharacter(PlayerUnit, test.class, attributeRulesTestStats())
			character.addUniversalStatDependenciesWithRuleset(rules)
			character.addManaStatDependenciesWithRuleset(rules)
			character.addBaseClassStatDependenciesWithRuleset(rules)
			got := character.SortAndApplyStatDependencies(character.GetStats())
			want := attributeRulesTestStats()
			want[stats.Health], want[stats.Mana], want[stats.Armor] = 1600, 3520, 260
			want[stats.AttackPower], want[stats.RangedAttackPower] = test.attackPower, test.rangedAP
			want[stats.PhysicalCritPercent], want[stats.SpellCritPercent] = test.physicalCrit, test.spellCrit
			for stat := range want {
				assertFloat64(t, stats.Stat(stat).StatName(), got[stat], want[stat])
			}

			// Extra maintenance guard: generated tables remain authoritative for
			// unrelated consumers until their own migration is reviewed.
			classRules := rules.attributes.forClass(test.class)
			assertFloat64(t, "generated agility crit parity", classRules.physicalCritPercentPerAgility, CritPerAgiMaxLevel[test.class])
			assertFloat64(t, "generated intellect crit parity", classRules.spellCritPercentPerIntellect, CritPerIntMaxLevel[test.class])
		})
	}
}

func TestAttributeRulesProfileInjection(t *testing.T) {
	rules := inheritedTBCRuleset()
	rules.attributes.healthPerStamina = 7.5
	rules.attributes.playerHealthOffset = -123.25
	rules.attributes.armorPerAgility = 3.25
	rules.attributes.manaPerIntellect = 12.5
	rules.attributes.manaOffset = -175.5
	rules.attributes.classes[proto.Class_ClassWarrior] = classAttributeRules{
		attackPowerPerStrength: 3, attackPowerPerAgility: 0.5, rangedAttackPowerPerAgility: 1.5,
		physicalCritPercentPerAgility: 0.0625, spellCritPercentPerIntellect: 0.03125,
	}
	rules.ratings.physicalHitRatingPerHitPercent = 10
	rules.ratings.spellHitRatingPerHitPercent = 20
	rules.ratings.physicalCritRatingPerCritPercent = 25
	rules.ratings.spellCritRatingPerCritPercent = 40
	raw := attributeRulesTestStats()
	raw[stats.HitRating], raw[stats.MeleeHitRating], raw[stats.SpellHitRating] = 100, 50, 20
	raw[stats.CritRating], raw[stats.MeleeCritRating], raw[stats.SpellCritRating] = 100, 25, 40
	character := attributeRulesTestCharacter(PlayerUnit, proto.Class_ClassWarrior, raw)
	character.addUniversalStatDependenciesWithRuleset(rules)
	character.addManaStatDependenciesWithRuleset(rules)
	character.addBaseClassStatDependenciesWithRuleset(rules)
	got := character.SortAndApplyStatDependencies(character.GetStats())
	want := raw
	want[stats.Health], want[stats.Mana], want[stats.Armor] = 1326.75, 3324.5, 360
	want[stats.AttackPower], want[stats.RangedAttackPower] = 370, 170
	want[stats.PhysicalHitPercent], want[stats.SpellHitPercent] = 15, 6
	want[stats.PhysicalCritPercent], want[stats.SpellCritPercent] = 11.5, 9.75
	for stat := range want {
		assertFloat64(t, stats.Stat(stat).StatName(), got[stat], want[stat])
	}
}

func TestAttributeRulesOffsetsAndFlooring(t *testing.T) {
	rules := inheritedTBCRuleset()
	rules.attributes.playerHealthOffset = -180
	raw := stats.Stats{stats.Health: 1000.75, stats.Mana: 2000.25, stats.Stamina: 60.75, stats.Intellect: 120.75}
	character := attributeRulesTestCharacter(PlayerUnit, proto.Class_ClassPaladin, raw)
	character.addUniversalStatDependenciesWithRuleset(rules)
	character.addManaStatDependenciesWithRuleset(rules)
	character.MultiplyStat(stats.Stamina, 1.1)
	character.MultiplyStat(stats.Intellect, 1.1)
	character.MultiplyStat(stats.Health, 1.05)
	character.MultiplyStat(stats.Mana, 1.1)

	for range 3 {
		got := character.SortAndApplyStatDependencies(character.GetStats()).FloorGameStats()
		// Source attributes are floored after their multiplier and before use.
		// Resources retain fractions: (1000.75 - 180 + 66*10)*1.05 and
		// (2000.25 - 280 + 132*15)*1.1. Re-measurement must not add offsets again.
		assertFloat64(t, "stamina", got[stats.Stamina], 66)
		assertFloat64(t, "intellect", got[stats.Intellect], 132)
		assertFloat64(t, "health", got[stats.Health], 1554.7875)
		assertFloat64(t, "mana", got[stats.Mana], 4070.275)
		assertFloat64(t, "raw health offset once", character.GetStats()[stats.Health], 820.75)
		assertFloat64(t, "raw mana offset once", character.GetStats()[stats.Mana], 1720.25)
		if character.GetBaseStats() != raw {
			t.Fatal("resource offsets changed the base-stat row")
		}
	}
}

func TestAttributeRulesPlayerOnlyConversions(t *testing.T) {
	rules := inheritedTBCRuleset()
	rules.attributes.playerHealthOffset = -180
	for _, unitType := range []UnitType{PetUnit, EnemyUnit} {
		character := attributeRulesTestCharacter(unitType, proto.Class_ClassHunter, attributeRulesTestStats())
		character.addUniversalStatDependenciesWithRuleset(rules)
		character.addManaStatDependenciesWithRuleset(rules)
		character.addBaseClassStatDependenciesWithRuleset(rules)
		got := character.SortAndApplyStatDependencies(character.GetStats())
		want := attributeRulesTestStats()
		want[stats.Health], want[stats.Armor] = 1600, 260
		for stat := range want {
			assertFloat64(t, stats.Stat(stat).StatName(), got[stat], want[stat])
		}
	}
}

func TestAttributeRulesPetConstructorKeepsOwnManaScaling(t *testing.T) {
	owner := &Character{Party: &Party{Raid: &Raid{}}}
	pet := NewPet(PetConfig{Owner: owner, Name: "attribute test", BaseStats: attributeRulesTestStats()})
	// Even an explicitly assigned player class must not install player AP/crit
	// or mana conversions on a pet. The pet owns those dependencies itself.
	pet.Class = proto.Class_ClassHunter
	pet.AddBaseClassStatDependencies()
	pet.EnableManaBar()
	got := pet.SortAndApplyStatDependencies(pet.GetStats())
	want := attributeRulesTestStats()
	want[stats.Health], want[stats.Armor] = 1600, 260
	for stat := range want {
		assertFloat64(t, stats.Stat(stat).StatName(), got[stat], want[stat])
	}
	if !pet.HasManaBar() || pet.BaseMana != 2000 || pet.GetBaseStats() != attributeRulesTestStats() {
		t.Fatal("pet mana setup changed its base mana or base-stat row")
	}
}

func TestAttributeRulesCharacterConstructorPreservesBaseMana(t *testing.T) {
	character := NewCharacter(&Party{}, 0, &proto.Player{
		Race: proto.Race_RaceHuman, Class: proto.Class_ClassPaladin,
		Spec: &proto.Player_RetributionPaladin{}, Equipment: &proto.EquipmentSpec{},
	})
	character.AddBaseClassStatDependencies()
	character.EnableManaBar()
	got := character.SortAndApplyStatDependencies(character.GetStats())
	assertFloat64(t, "spell cost base mana", character.BaseMana, 2953)
	assertFloat64(t, "unchanged raw base mana", character.GetBaseStats()[stats.Mana], 2953)
	assertFloat64(t, "offset raw mana", character.GetStats()[stats.Mana], 2673)
	assertFloat64(t, "derived mana", got[stats.Mana], 3918)
	assertFloat64(t, "derived health", got[stats.Health], 4397)
	assertFloat64(t, "derived attack power", got[stats.AttackPower], 442)
	assertFloat64(t, "derived physical crit", got[stats.PhysicalCritPercent], 3.732)
	assertFloat64(t, "derived spell crit", got[stats.SpellCritPercent], 4.373)
}

func TestAttributeRulesClassLookupAndCopyIsolation(t *testing.T) {
	original := inheritedTBCRuleset()
	rules := original
	marker := classAttributeRules{attackPowerPerStrength: 999}
	// Poison in-range non-class slots too: lookup must reject unknown/holes,
	// rather than relying on their array entries happening to be zero.
	for _, class := range []proto.Class{proto.Class_ClassUnknown, 6, 10} {
		rules.attributes.classes[class] = marker
	}
	for _, class := range []proto.Class{-2147483648, -1, 0, 6, 10, 12, 999, 2147483647} {
		if got := rules.attributes.forClass(class); got != (classAttributeRules{}) {
			t.Errorf("class %d: got %+v, want no conversions", class, got)
		}
	}
	row := rules.attributes.forClass(proto.Class_ClassWarrior)
	row.attackPowerPerStrength = 123
	if rules.attributes.forClass(proto.Class_ClassWarrior) == row {
		t.Fatal("mutating a returned class row affected its profile")
	}
	rules.attributes.classes[proto.Class_ClassWarrior] = marker
	rules.attributes.healthPerStamina = 999
	rules.attributes.playerHealthOffset = -999
	rules.attributes.armorPerAgility = 999
	rules.attributes.manaPerIntellect = 999
	rules.attributes.manaOffset = -999
	if inheritedTBCRuleset() != original || currentRuleset() != original {
		t.Fatal("mutating a profile copy affected the inherited or active profile")
	}
}
