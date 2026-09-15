package main

import (
	"fmt"
	"reflect"
)

const (
	foreverDeepDiveURL  = "https://news.blizzard.com/en-us/article/24303313/world-of-warcraft-forever-deep-dive-panel-recap"
	foreverWhatsNextURL = "https://news.blizzard.com/en-us/article/24303862/world-of-warcraft-forever-whats-next-panel-recap"

	foreverDeepDiveCombatLocator  = "Combat, Classes, and Itemization"
	foreverWhatsNextPlacesLocator = "New Places to Explore"
)

// ruleReviewStatus describes a simulator decision, not the confidence of the
// underlying source. Reviewed facts stay inactive until a separate change
// promotes them into a complete runtime ruleset.
type ruleReviewStatus uint8

const (
	ruleReviewUnknown ruleReviewStatus = iota
	ruleReviewReviewedInactive
)

type ruleEvidenceBasis uint8

const (
	ruleEvidenceBasisUnknown ruleEvidenceBasis = iota
	ruleEvidenceBasisOfficialAnnouncement
)

type ruleEvidenceDocument uint8

const (
	ruleEvidenceDocumentUnknown ruleEvidenceDocument = iota
	ruleEvidenceDocumentBlizzardDeepDive
	ruleEvidenceDocumentBlizzardWhatsNext
)

func (document ruleEvidenceDocument) url() string {
	switch document {
	case ruleEvidenceDocumentBlizzardDeepDive:
		return foreverDeepDiveURL
	case ruleEvidenceDocumentBlizzardWhatsNext:
		return foreverWhatsNextURL
	default:
		return ""
	}
}

type ruleEvidenceRef struct {
	document ruleEvidenceDocument
	basis    ruleEvidenceBasis
	locator  string
}

type reviewedRuleFact[T comparable] struct {
	value    T
	evidence ruleEvidenceRef
	review   ruleReviewStatus
}

// combinedChanceChannels records the chance categories Blizzard says are
// combined. It makes no claim about storage, conversion, or outcome tables.
type combinedChanceChannels uint8

const (
	combinedChanceChannelMelee combinedChanceChannels = 1 << iota
	combinedChanceChannelRanged
	combinedChanceChannelSpell

	combinedChanceChannelsMeleeRangedSpell = combinedChanceChannelMelee | combinedChanceChannelRanged | combinedChanceChannelSpell
)

// avoidanceKinds is a set of outcomes for which Blizzard announced reduction
// effects. A set containing both kinds does not claim that one source feeds both.
type avoidanceKinds uint8

const (
	avoidanceKindDodge avoidanceKinds = 1 << iota
	avoidanceKindParry

	avoidanceKindsParryOrDodge = avoidanceKindParry | avoidanceKindDodge
)

// exactRatio prevents the announced one-third relationship from being stored
// as an imprecise floating-point approximation.
type exactRatio struct {
	numerator   int64
	denominator int64
}

// foreverCoreRuleCatalog contains only reviewed claims with primary evidence.
// It lives in this package-main evidence tool so the simulation cannot import
// it accidentally. Missing formulas and conversions are absent, not zero-filled.
type foreverCoreRuleCatalog struct {
	levelJourneyUpperBound           reviewedRuleFact[int32]
	combinedHitChanceChannels        reviewedRuleFact[combinedChanceChannels]
	combinedCritChanceChannels       reviewedRuleFact[combinedChanceChannels]
	weaponSkillRemainsRelevant       reviewedRuleFact[bool]
	announcedAvoidanceReductionKinds reviewedRuleFact[avoidanceKinds]
	bonusDamagePerBonusHealing       reviewedRuleFact[exactRatio]
}

func reviewedAnnouncement[T comparable](value T, document ruleEvidenceDocument, locator string) reviewedRuleFact[T] {
	return reviewedRuleFact[T]{
		value: value,
		evidence: ruleEvidenceRef{
			document: document,
			basis:    ruleEvidenceBasisOfficialAnnouncement,
			locator:  locator,
		},
		review: ruleReviewReviewedInactive,
	}
}

func foreverCoreRules() foreverCoreRuleCatalog {
	return foreverCoreRuleCatalog{
		levelJourneyUpperBound: reviewedAnnouncement(
			int32(60),
			ruleEvidenceDocumentBlizzardWhatsNext,
			foreverWhatsNextPlacesLocator,
		),
		combinedHitChanceChannels: reviewedAnnouncement(
			combinedChanceChannelsMeleeRangedSpell,
			ruleEvidenceDocumentBlizzardDeepDive,
			foreverDeepDiveCombatLocator,
		),
		combinedCritChanceChannels: reviewedAnnouncement(
			combinedChanceChannelsMeleeRangedSpell,
			ruleEvidenceDocumentBlizzardDeepDive,
			foreverDeepDiveCombatLocator,
		),
		weaponSkillRemainsRelevant: reviewedAnnouncement(
			true,
			ruleEvidenceDocumentBlizzardDeepDive,
			foreverDeepDiveCombatLocator,
		),
		announcedAvoidanceReductionKinds: reviewedAnnouncement(
			avoidanceKindsParryOrDodge,
			ruleEvidenceDocumentBlizzardDeepDive,
			foreverDeepDiveCombatLocator,
		),
		bonusDamagePerBonusHealing: reviewedAnnouncement(
			exactRatio{numerator: 1, denominator: 3},
			ruleEvidenceDocumentBlizzardDeepDive,
			foreverDeepDiveCombatLocator,
		),
	}
}

type catalogFactMetadata struct {
	name             string
	evidence         ruleEvidenceRef
	expectedEvidence ruleEvidenceRef
	review           ruleReviewStatus
}

func (catalog foreverCoreRuleCatalog) factMetadata() []catalogFactMetadata {
	deepDiveEvidence := ruleEvidenceRef{
		document: ruleEvidenceDocumentBlizzardDeepDive,
		basis:    ruleEvidenceBasisOfficialAnnouncement,
		locator:  foreverDeepDiveCombatLocator,
	}
	return []catalogFactMetadata{
		{
			name:     "level-journey upper bound",
			evidence: catalog.levelJourneyUpperBound.evidence,
			expectedEvidence: ruleEvidenceRef{
				document: ruleEvidenceDocumentBlizzardWhatsNext,
				basis:    ruleEvidenceBasisOfficialAnnouncement,
				locator:  foreverWhatsNextPlacesLocator,
			},
			review: catalog.levelJourneyUpperBound.review,
		},
		{
			name:             "combined Hit chance channels",
			evidence:         catalog.combinedHitChanceChannels.evidence,
			expectedEvidence: deepDiveEvidence,
			review:           catalog.combinedHitChanceChannels.review,
		},
		{
			name:             "combined Crit chance channels",
			evidence:         catalog.combinedCritChanceChannels.evidence,
			expectedEvidence: deepDiveEvidence,
			review:           catalog.combinedCritChanceChannels.review,
		},
		{
			name:             "weapon-skill relevance",
			evidence:         catalog.weaponSkillRemainsRelevant.evidence,
			expectedEvidence: deepDiveEvidence,
			review:           catalog.weaponSkillRemainsRelevant.review,
		},
		{
			name:             "announced avoidance-reduction kinds",
			evidence:         catalog.announcedAvoidanceReductionKinds.evidence,
			expectedEvidence: deepDiveEvidence,
			review:           catalog.announcedAvoidanceReductionKinds.review,
		},
		{
			name:             "bonus-damage per bonus-healing ratio",
			evidence:         catalog.bonusDamagePerBonusHealing.evidence,
			expectedEvidence: deepDiveEvidence,
			review:           catalog.bonusDamagePerBonusHealing.review,
		},
	}
}

func (catalog foreverCoreRuleCatalog) factCount() int {
	return len(catalog.factMetadata())
}

// validateForeverCoreRules intentionally pins both provenance and claim scope.
// Any broadened or changed claim therefore needs an explicit reviewed diff.
func validateForeverCoreRules(catalog foreverCoreRuleCatalog) error {
	metadata := catalog.factMetadata()
	if len(metadata) != reflect.TypeOf(catalog).NumField() {
		return fmt.Errorf("core-rule catalog metadata covers %d of %d facts", len(metadata), reflect.TypeOf(catalog).NumField())
	}
	for _, fact := range metadata {
		if fact.review != ruleReviewReviewedInactive {
			return fmt.Errorf("%s must be reviewed and inactive", fact.name)
		}
		if fact.evidence != fact.expectedEvidence {
			return fmt.Errorf("%s must retain its exact evidence reference", fact.name)
		}
	}

	if catalog.levelJourneyUpperBound.value != 60 {
		return fmt.Errorf("level-journey upper bound must remain the reviewed value 60")
	}
	if catalog.combinedHitChanceChannels.value != combinedChanceChannelsMeleeRangedSpell {
		return fmt.Errorf("combined Hit chance must remain scoped to melee, ranged, and spell channels")
	}
	if catalog.combinedCritChanceChannels.value != combinedChanceChannelsMeleeRangedSpell {
		return fmt.Errorf("combined Crit chance must remain scoped to melee, ranged, and spell channels")
	}
	if !catalog.weaponSkillRemainsRelevant.value {
		return fmt.Errorf("weapon skill must remain relevant")
	}
	if catalog.announcedAvoidanceReductionKinds.value != avoidanceKindsParryOrDodge {
		return fmt.Errorf("announced avoidance reduction must remain scoped to parry or dodge")
	}
	if catalog.bonusDamagePerBonusHealing.value != (exactRatio{numerator: 1, denominator: 3}) {
		return fmt.Errorf("bonus damage per bonus healing must remain exactly 1/3")
	}

	return nil
}
