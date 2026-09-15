package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/stats"
)

func TestUnifiedHitAndCritFeedSeparateOutcomeChannels(t *testing.T) {
	unit := Unit{StatDependencyManager: stats.NewStatDependencyManager()}
	unit.addUniversalStatDependenciesWithRuleset(inheritedTBCRuleset())

	input := stats.Stats{
		stats.HitRating:  100,
		stats.CritRating: 100,
	}
	got := unit.StatDependencyManager.SortAndApplyStatDependencies(input)

	// Literal divisors pin the inherited TBC conversions instead of deriving
	// the expected values from the production constants under test.
	assertFloat64(t, "physical hit", got[stats.PhysicalHitPercent], 100/15.769233)
	assertFloat64(t, "spell hit", got[stats.SpellHitPercent], 100/12.615385)
	assertFloat64(t, "physical crit", got[stats.PhysicalCritPercent], 100/22.076923)
	assertFloat64(t, "spell crit", got[stats.SpellCritPercent], 100/22.076923)
}

func TestScopedHitAndCritSourcesRemainScoped(t *testing.T) {
	unit := Unit{StatDependencyManager: stats.NewStatDependencyManager()}
	unit.addUniversalStatDependenciesWithRuleset(inheritedTBCRuleset())

	input := stats.Stats{
		stats.MeleeHitRating:  100,
		stats.MeleeCritRating: 100,
	}
	got := unit.StatDependencyManager.SortAndApplyStatDependencies(input)

	assertFloat64(t, "physical hit", got[stats.PhysicalHitPercent], 100/15.769233)
	assertFloat64(t, "physical crit", got[stats.PhysicalCritPercent], 100/22.076923)
	assertFloat64(t, "spell hit", got[stats.SpellHitPercent], 0)
	assertFloat64(t, "spell crit", got[stats.SpellCritPercent], 0)
}

func TestUnifiedAndScopedHitAndCritSourcesAreAdditive(t *testing.T) {
	unit := Unit{StatDependencyManager: stats.NewStatDependencyManager()}
	unit.addUniversalStatDependenciesWithRuleset(inheritedTBCRuleset())

	input := stats.Stats{
		stats.HitRating:       100,
		stats.MeleeHitRating:  50,
		stats.SpellHitRating:  25,
		stats.CritRating:      100,
		stats.MeleeCritRating: 50,
		stats.SpellCritRating: 25,
	}
	got := unit.StatDependencyManager.SortAndApplyStatDependencies(input)

	assertFloat64(t, "physical hit", got[stats.PhysicalHitPercent], 150/15.769233)
	assertFloat64(t, "spell hit", got[stats.SpellHitPercent], 125/12.615385)
	assertFloat64(t, "physical crit", got[stats.PhysicalCritPercent], 150/22.076923)
	assertFloat64(t, "spell crit", got[stats.SpellCritPercent], 125/22.076923)
}
