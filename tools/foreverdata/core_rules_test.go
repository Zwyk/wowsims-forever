package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestForeverCoreRulesMatchReviewedAnnouncements(t *testing.T) {
	catalog := foreverCoreRules()
	if err := validateForeverCoreRules(catalog); err != nil {
		t.Fatalf("reviewed catalog is invalid: %v", err)
	}

	if catalog.levelJourneyUpperBound.value != 60 {
		t.Fatalf("level-journey upper bound = %d, want 60", catalog.levelJourneyUpperBound.value)
	}
	if catalog.combinedHitChanceChannels.value != combinedChanceChannelsMeleeRangedSpell {
		t.Fatalf("combined Hit chance channels = %b, want %b", catalog.combinedHitChanceChannels.value, combinedChanceChannelsMeleeRangedSpell)
	}
	if catalog.combinedCritChanceChannels.value != combinedChanceChannelsMeleeRangedSpell {
		t.Fatalf("combined Crit chance channels = %b, want %b", catalog.combinedCritChanceChannels.value, combinedChanceChannelsMeleeRangedSpell)
	}
	if !catalog.weaponSkillRemainsRelevant.value {
		t.Fatal("weapon-skill relevance is not recorded")
	}
	if catalog.announcedAvoidanceReductionKinds.value != avoidanceKindsParryOrDodge {
		t.Fatalf("avoidance kinds = %b, want %b", catalog.announcedAvoidanceReductionKinds.value, avoidanceKindsParryOrDodge)
	}
	if catalog.bonusDamagePerBonusHealing.value != (exactRatio{numerator: 1, denominator: 3}) {
		t.Fatalf("bonus-damage per bonus-healing ratio = %+v, want 1/3", catalog.bonusDamagePerBonusHealing.value)
	}

	const wantWhatsNextURL = "https://news.blizzard.com/en-us/article/24303862/world-of-warcraft-forever-whats-next-panel-recap"
	if got := catalog.levelJourneyUpperBound.evidence.document.url(); got != wantWhatsNextURL {
		t.Fatalf("level evidence URL = %q, want %q", got, wantWhatsNextURL)
	}
	if got := catalog.levelJourneyUpperBound.evidence.locator; got != "New Places to Explore" {
		t.Fatalf("level evidence locator = %q, want %q", got, "New Places to Explore")
	}

	const wantDeepDiveURL = "https://news.blizzard.com/en-us/article/24303313/world-of-warcraft-forever-deep-dive-panel-recap"
	for _, fact := range []struct {
		name     string
		evidence ruleEvidenceRef
	}{
		{name: "Hit", evidence: catalog.combinedHitChanceChannels.evidence},
		{name: "Crit", evidence: catalog.combinedCritChanceChannels.evidence},
		{name: "weapon skill", evidence: catalog.weaponSkillRemainsRelevant.evidence},
		{name: "avoidance reduction", evidence: catalog.announcedAvoidanceReductionKinds.evidence},
		{name: "healing contribution", evidence: catalog.bonusDamagePerBonusHealing.evidence},
	} {
		if got := fact.evidence.document.url(); got != wantDeepDiveURL {
			t.Errorf("%s evidence URL = %q, want %q", fact.name, got, wantDeepDiveURL)
		}
		if got := fact.evidence.locator; got != "Combat, Classes, and Itemization" {
			t.Errorf("%s evidence locator = %q, want %q", fact.name, got, "Combat, Classes, and Itemization")
		}
	}
	if got := catalog.factCount(); got != 6 {
		t.Fatalf("fact count = %d, want 6", got)
	}
}

func TestForeverCoreRulesRejectIncompleteOrBroadenedClaims(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*foreverCoreRuleCatalog)
		want   string
	}{
		{
			name: "unreviewed",
			mutate: func(catalog *foreverCoreRuleCatalog) {
				catalog.levelJourneyUpperBound.review = ruleReviewUnknown
			},
			want: "reviewed and inactive",
		},
		{
			name: "wrong evidence basis",
			mutate: func(catalog *foreverCoreRuleCatalog) {
				catalog.combinedHitChanceChannels.evidence.basis = ruleEvidenceBasisUnknown
			},
			want: "exact evidence reference",
		},
		{
			name: "wrong recognized document",
			mutate: func(catalog *foreverCoreRuleCatalog) {
				catalog.combinedCritChanceChannels.evidence.document = ruleEvidenceDocumentBlizzardWhatsNext
			},
			want: "exact evidence reference",
		},
		{
			name: "wrong nonempty locator",
			mutate: func(catalog *foreverCoreRuleCatalog) {
				catalog.weaponSkillRemainsRelevant.evidence.locator = "Transmogrification and Player Choice"
			},
			want: "exact evidence reference",
		},
		{
			name: "changed journey bound",
			mutate: func(catalog *foreverCoreRuleCatalog) {
				catalog.levelJourneyUpperBound.value = 63
			},
			want: "upper bound must remain the reviewed value 60",
		},
		{
			name: "incomplete Hit categories",
			mutate: func(catalog *foreverCoreRuleCatalog) {
				catalog.combinedHitChanceChannels.value = combinedChanceChannelSpell
			},
			want: "melee, ranged, and spell channels",
		},
		{
			name: "incomplete Crit categories",
			mutate: func(catalog *foreverCoreRuleCatalog) {
				catalog.combinedCritChanceChannels.value = combinedChanceChannelMelee
			},
			want: "melee, ranged, and spell channels",
		},
		{
			name: "weapon skill removed",
			mutate: func(catalog *foreverCoreRuleCatalog) {
				catalog.weaponSkillRemainsRelevant.value = false
			},
			want: "must remain relevant",
		},
		{
			name: "missing avoidance target",
			mutate: func(catalog *foreverCoreRuleCatalog) {
				catalog.announcedAvoidanceReductionKinds.value = avoidanceKindDodge
			},
			want: "parry or dodge",
		},
		{
			name: "rounded healing ratio",
			mutate: func(catalog *foreverCoreRuleCatalog) {
				catalog.bonusDamagePerBonusHealing.value = exactRatio{numerator: 333, denominator: 1000}
			},
			want: "exactly 1/3",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := foreverCoreRules()
			test.mutate(&catalog)
			err := validateForeverCoreRules(catalog)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateForeverCoreRules() error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestForeverCoreRulesReturnAValueCopy(t *testing.T) {
	first := foreverCoreRules()
	first.levelJourneyUpperBound.value = 1
	first.combinedHitChanceChannels.evidence.locator = "changed"

	second := foreverCoreRules()
	if err := validateForeverCoreRules(second); err != nil {
		t.Fatalf("mutating one catalog changed the next: %v", err)
	}
}

func TestVerifyCLIIncludesInactiveCoreRules(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runCLI(context.Background(), []string{"verify"}, &stdout, &stderr, func() time.Time { return time.Time{} })
	if code != 0 {
		t.Fatalf("verify exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "verified talentsforever snapshot ") {
		t.Fatalf("snapshot verification output missing:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "verified 6 reviewed-inactive Forever core facts\n") {
		t.Fatalf("core-rule verification output missing:\n%s", stdout.String())
	}
}
