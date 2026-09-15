package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/stats"
)

func TestClassicReferenceLevel60OffensiveChanceOverlay(t *testing.T) {
	base := inheritedTBCRuleset()
	base.levels.characterLevel = 60
	got, ok := classicReferenceLevel60OffensiveChanceOverlay(base)
	if !ok {
		t.Fatal("Classic reference overlay rejected level 60")
	}

	want := base.ratings
	want.physicalHitRatingPerHitPercent = 1
	want.spellHitRatingPerHitPercent = 1
	want.physicalCritRatingPerCritPercent = 1
	want.spellCritRatingPerCritPercent = 1
	if got != want {
		t.Fatalf("Classic reference overlay = %+v, want %+v", got, want)
	}
	if base.ratings != inheritedTBCRuleset().ratings {
		t.Fatal("Classic reference overlay mutated its base rules")
	}
}

func TestClassicReferenceOffensiveChanceScopeFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		base rulesetProfile
	}{
		{name: "below 60", base: rulesetWithCharacterLevel(59)},
		{name: "above 60", base: rulesetWithCharacterLevel(61)},
		{name: "inherited TBC", base: inheritedTBCRuleset()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := classicReferenceLevel60OffensiveChanceOverlay(test.base)
			if ok {
				t.Fatalf("unsupported level %d resolved to %+v", test.base.levels.characterLevel, got)
			}
			if got != (ratingRules{}) {
				t.Fatalf("rejected level %d returned nonzero rules %+v", test.base.levels.characterLevel, got)
			}
		})
	}
}

func rulesetWithCharacterLevel(level int32) rulesetProfile {
	rules := inheritedTBCRuleset()
	rules.levels.characterLevel = level
	return rules
}

func TestClassicReferenceScopedHitAndCritInputsArePercentagePoints(t *testing.T) {
	rules := inheritedTBCRuleset()
	rules.levels.characterLevel = classicReferenceCharacterLevel
	var ok bool
	rules.ratings, ok = classicReferenceLevel60OffensiveChanceOverlay(rules)
	if !ok {
		t.Fatal("Classic reference overlay rejected level 60")
	}

	unit := Unit{StatDependencyManager: stats.NewStatDependencyManager()}
	unit.addUniversalStatDependenciesWithRuleset(rules)
	input := stats.Stats{
		stats.MeleeHitRating:  2.5,
		stats.SpellHitRating:  1.25,
		stats.MeleeCritRating: -0.5,
		stats.SpellCritRating: 0.75,
	}
	got := unit.StatDependencyManager.SortAndApplyStatDependencies(input)

	assertFloat64(t, "physical hit", got[stats.PhysicalHitPercent], 2.5)
	assertFloat64(t, "spell hit", got[stats.SpellHitPercent], 1.25)
	assertFloat64(t, "physical crit", got[stats.PhysicalCritPercent], -0.5)
	assertFloat64(t, "spell crit", got[stats.SpellCritPercent], 0.75)
}

func TestClassicReferenceSharedHitAndCritUseForeverFanout(t *testing.T) {
	rules := inheritedTBCRuleset()
	rules.levels.characterLevel = classicReferenceCharacterLevel
	var ok bool
	rules.ratings, ok = classicReferenceLevel60OffensiveChanceOverlay(rules)
	if !ok {
		t.Fatal("Classic reference overlay rejected level 60")
	}

	unit := Unit{StatDependencyManager: stats.NewStatDependencyManager()}
	unit.addUniversalStatDependenciesWithRuleset(rules)
	got := unit.StatDependencyManager.SortAndApplyStatDependencies(stats.Stats{
		stats.HitRating:       3.25,
		stats.MeleeHitRating:  0.75,
		stats.SpellHitRating:  -0.25,
		stats.CritRating:      2.5,
		stats.MeleeCritRating: -0.5,
		stats.SpellCritRating: 0.25,
	})

	assertFloat64(t, "physical hit", got[stats.PhysicalHitPercent], 4)
	assertFloat64(t, "spell hit", got[stats.SpellHitPercent], 3)
	assertFloat64(t, "physical crit", got[stats.PhysicalCritPercent], 2)
	assertFloat64(t, "spell crit", got[stats.SpellCritPercent], 2.75)
}

func TestClassicReferenceOffensiveChanceOverlayIsInactive(t *testing.T) {
	active := currentRuleset()
	inherited := inheritedTBCRuleset()
	if active.ratings != inherited.ratings {
		t.Fatal("active rating rules no longer match inherited TBC")
	}

	classic, ok := classicReferenceLevel60OffensiveChanceOverlay(active)
	if ok {
		t.Fatal("Classic reference overlay accepted the active level-70 profile")
	}
	if classic != (ratingRules{}) {
		t.Fatalf("rejected active profile returned nonzero rules %+v", classic)
	}
	if active.levels.characterLevel != inheritedTBCCharacterLevel {
		t.Fatalf("active character level = %d, want inherited TBC level %d", active.levels.characterLevel, inheritedTBCCharacterLevel)
	}
}
