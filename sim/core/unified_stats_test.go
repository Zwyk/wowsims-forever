package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/stats"
)

func TestUnifiedHitAndCritFeedSeparateOutcomeChannels(t *testing.T) {
	unit := Unit{StatDependencyManager: stats.NewStatDependencyManager()}
	unit.addUniversalStatDependencies()

	input := stats.Stats{
		stats.HitRating:  100,
		stats.CritRating: 100,
	}
	got := unit.StatDependencyManager.SortAndApplyStatDependencies(input)

	assertFloat64(t, "physical hit", got[stats.PhysicalHitPercent], 100/PhysicalHitRatingPerHitPercent)
	assertFloat64(t, "spell hit", got[stats.SpellHitPercent], 100/SpellHitRatingPerHitPercent)
	assertFloat64(t, "physical crit", got[stats.PhysicalCritPercent], 100/PhysicalCritRatingPerCritPercent)
	assertFloat64(t, "spell crit", got[stats.SpellCritPercent], 100/SpellCritRatingPerCritPercent)
}

func TestScopedHitAndCritSourcesRemainScoped(t *testing.T) {
	unit := Unit{StatDependencyManager: stats.NewStatDependencyManager()}
	unit.addUniversalStatDependencies()

	input := stats.Stats{
		stats.MeleeHitRating:  100,
		stats.MeleeCritRating: 100,
	}
	got := unit.StatDependencyManager.SortAndApplyStatDependencies(input)

	assertFloat64(t, "physical hit", got[stats.PhysicalHitPercent], 100/PhysicalHitRatingPerHitPercent)
	assertFloat64(t, "physical crit", got[stats.PhysicalCritPercent], 100/PhysicalCritRatingPerCritPercent)
	assertFloat64(t, "spell hit", got[stats.SpellHitPercent], 0)
	assertFloat64(t, "spell crit", got[stats.SpellCritPercent], 0)
}
