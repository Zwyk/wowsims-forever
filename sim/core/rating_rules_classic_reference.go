package core

// classicReferenceLevel60OffensiveChanceOverlay returns a copy of base's
// rating rules with only the four well-supported Classic Hit/Crit input
// conversions replaced.
// In the pinned Classic implementation, one stored Hit or Crit unit is one
// percentage point for the corresponding physical or spell outcome channel.
//
// The current HitRating and CritRating names are provisional transport names.
// Their shared fan-out is a Forever engine decision; Classic itself keeps the
// physical and spell sources separate. No production ruleset selects this
// overlay, and every uncharacterized conversion remains supplied by base.
func classicReferenceLevel60OffensiveChanceOverlay(base rulesetProfile) (ratingRules, bool) {
	if base.levels.characterLevel != classicReferenceCharacterLevel {
		return ratingRules{}, false
	}

	const inputUnitsPerPercentagePoint = 1.0
	ratings := base.ratings
	ratings.physicalHitRatingPerHitPercent = inputUnitsPerPercentagePoint
	ratings.spellHitRatingPerHitPercent = inputUnitsPerPercentagePoint
	ratings.physicalCritRatingPerCritPercent = inputUnitsPerPercentagePoint
	ratings.spellCritRatingPerCritPercent = inputUnitsPerPercentagePoint
	return ratings, true
}
