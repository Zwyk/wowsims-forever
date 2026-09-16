package core

import (
	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

// classicReferenceCharacterBaseline is a resolved unequipped, untalented humanoid
// stat baseline, with or without passive racials, not a simulated Character.
// It carries no Unit, mutable dependency graph, active ruleset or resource bar.
// Returning only values prevents live TBC class/racial/avoidance helpers from
// being called on a partially converted level-60 character.
type classicReferenceCharacterBaseline struct {
	level     int32
	race      proto.Race
	class     proto.Class
	baseStats stats.Stats // Raw attributes/resources/AP plus baseline crit, before dependencies.
	stats     stats.Stats // Resolved resources/AP/crit; defense-family rating slots stay zero.
	baseMana  float64     // Unmodified class mana for percentage-base-mana spell costs.
	hasMana   bool
	avoidance classicReferenceBaselineAvoidance
}

// These are percentage points before opponent-level and combat-state effects,
// not TBC ratings. The source grants raw 5% parry/block even when the class or
// equipment cannot use them, so keep capabilities separate from raw values.
type classicReferenceBaselineAvoidance struct {
	dodgePercent, parryPercent, blockPercent float64
	canParry, canBlock                       bool
}

func (avoidance classicReferenceBaselineAvoidance) effectiveParryPercent() float64 {
	if !avoidance.canParry {
		return 0
	}
	return avoidance.parryPercent
}

func (avoidance classicReferenceBaselineAvoidance) effectiveBlockPercent() float64 {
	if !avoidance.canBlock {
		return 0
	}
	return avoidance.blockPercent
}

// classicReferenceInitializeCharacter composes the pinned Classic references
// through the existing attribute dependency installers in one bounded path.
// It accepts only level 60 and the 40 original combinations. All reference data
// is constructed locally; no active TBC base table, racial, class constructor
// or avoidance getter is consulted.
//
// This entry point deliberately stops before racials. For a baseline including
// unconditional racial stat effects, use classicReferenceInitializeCharacterWithRacials.
// Neither entry point includes gear/bonus stats, talents, forms, pets,
// regeneration or actual combat tables.
// Source provenance and known constructor omissions are documented by the
// component references; this composition is not verified Forever behavior.
func classicReferenceInitializeCharacter(level int32, race proto.Race, class proto.Class) (classicReferenceCharacterBaseline, bool) {
	return classicReferenceInitializeCharacterWithPassiveRacials(level, race, class, false)
}

// classicReferenceInitializeCharacterWithRacials includes the pinned source's
// unconditional own-character racial stat effects, keeping raw baseStats and
// spell-cost baseMana unchanged. Classic@7779ebb preserves fractional primary
// stat sources in sim/core/stats/deps.go and does not floor its finalized stats;
// this reference explicitly follows that source behavior. It is not a verified
// Forever rounding policy and does not change live TBC dependency rounding.
func classicReferenceInitializeCharacterWithRacials(level int32, race proto.Race, class proto.Class) (classicReferenceCharacterBaseline, bool) {
	return classicReferenceInitializeCharacterWithPassiveRacials(level, race, class, true)
}

func classicReferenceInitializeCharacterWithPassiveRacials(level int32, race proto.Race, class proto.Class, includeRacials bool) (classicReferenceCharacterBaseline, bool) {
	// Do not start from inheritedTBCRuleset: this is a local dependency-only
	// profile, with no inferred TBC combat/defense/haste policy mixed into it.
	rules := rulesetProfile{levels: levelRules{characterLevel: level}}
	base, ok := classicReferenceLevel60BaseAttributes(rules, race, class)
	if !ok {
		return classicReferenceCharacterBaseline{}, false
	}
	chances, ok := classicReferenceLevel60BaselineChances(rules, class)
	if !ok {
		return classicReferenceCharacterBaseline{}, false
	}
	rules.attributes, ok = classicReferenceLevel60AttributeRules(rules)
	if !ok {
		return classicReferenceCharacterBaseline{}, false
	}
	// The universal dependency installer consumes only these four rating
	// divisors. All uncharacterized ratings remain absent and are never read.
	rules.ratings, ok = classicReferenceLevel60OffensiveChanceOverlay(rules)
	if !ok {
		return classicReferenceCharacterBaseline{}, false
	}
	var racials classicReferencePassiveRacials
	if includeRacials {
		racials, ok = classicReferenceLevel60PassiveRacials(rules, race)
		if !ok {
			return classicReferenceCharacterBaseline{}, false
		}
	}

	raw := stats.Stats{
		stats.Strength: base.strength, stats.Agility: base.agility, stats.Stamina: base.stamina,
		stats.Intellect: base.intellect, stats.Spirit: base.spirit,
		stats.Health: base.health, stats.Mana: base.mana,
		stats.AttackPower: base.attackPower, stats.RangedAttackPower: base.rangedAttackPower,
		stats.PhysicalCritPercent: chances.physicalCritPercent, stats.SpellCritPercent: chances.spellCritPercent,
	}
	character := Character{
		Unit: Unit{
			Type: PlayerUnit, Level: level,
			StatDependencyManager: stats.NewStatDependencyManagerWithSourceRounding(stats.PreserveFractionalSources),
		},
		Race: race, Class: class, baseStats: raw,
	}
	character.AddStats(raw)
	character.addUniversalStatDependenciesWithRuleset(rules)
	character.addBaseClassStatDependenciesWithRuleset(rules)
	hasMana := class != proto.Class_ClassWarrior && class != proto.Class_ClassRogue
	if hasMana {
		character.addManaStatDependenciesWithRuleset(rules)
	}
	if includeRacials {
		character.AddStats(racials.bonusStats)
		character.MultiplyStat(stats.Intellect, racials.intellectMultiplier)
		character.MultiplyStat(stats.Spirit, racials.spiritMultiplier)
		character.MultiplyStat(stats.Health, racials.healthMultiplier)
	}
	resolved := character.SortAndApplyStatDependencies(character.GetStats())

	return classicReferenceCharacterBaseline{
		level: level, race: race, class: class,
		baseStats: raw, stats: resolved, baseMana: base.mana, hasMana: hasMana,
		avoidance: classicReferenceBaselineAvoidance{
			// Keep dodge in percentage points rather than a fake DodgeRating
			// slot. The optional racial bonus is additive, not a multiplier.
			dodgePercent: chances.dodgePercent + resolved[stats.Agility]*chances.dodgePercentPerAgility + racials.bonusDodgePercent,
			parryPercent: chances.parryPercent, blockPercent: chances.blockPercent,
			canParry: chances.canParry, canBlock: false, // No shield in this baseline.
		},
	}, true
}
