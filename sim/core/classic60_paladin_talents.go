package core

import (
	"fmt"
	"strings"
)

// Classic talent-string order, independent of the inherited TBC protobuf.
// Tree positions, maximum ranks and prerequisite arrows are pinned at:
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/ui/core/talents/trees/paladin.json
type classic60PaladinTalent uint8

const (
	classic60PaladinDivineStrength classic60PaladinTalent = iota
	classic60PaladinDivineIntellect
	classic60PaladinSpiritualFocus
	classic60PaladinImprovedSealOfRighteousness
	classic60PaladinHealingLight
	classic60PaladinConsecration
	classic60PaladinImprovedLayOnHands
	classic60PaladinUnyieldingFaith
	classic60PaladinIllumination
	classic60PaladinImprovedBlessingOfWisdom
	classic60PaladinDivineFavor
	classic60PaladinLastingJudgement
	classic60PaladinHolyPower
	classic60PaladinHolyShock
	classic60PaladinImprovedDevotionAura
	classic60PaladinRedoubt
	classic60PaladinPrecision
	classic60PaladinGuardiansFavor
	classic60PaladinToughness
	classic60PaladinBlessingOfKings
	classic60PaladinImprovedRighteousFury
	classic60PaladinShieldSpecialization
	classic60PaladinAnticipation
	classic60PaladinImprovedHammerOfJustice
	classic60PaladinImprovedConcentrationAura
	classic60PaladinBlessingOfSanctuary
	classic60PaladinReckoning
	classic60PaladinOneHandedWeaponSpecialization
	classic60PaladinHolyShield
	classic60PaladinImprovedBlessingOfMight
	classic60PaladinBenediction
	classic60PaladinImprovedJudgement
	classic60PaladinImprovedSealOfTheCrusader
	classic60PaladinDeflection
	classic60PaladinVindication
	classic60PaladinConviction
	classic60PaladinSealOfCommand
	classic60PaladinPursuitOfJustice
	classic60PaladinEyeForAnEye
	classic60PaladinImprovedRetributionAura
	classic60PaladinTwoHandedWeaponSpecialization
	classic60PaladinSanctityAura
	classic60PaladinVengeance
	classic60PaladinRepentance
	classic60PaladinTalentCount
)

type classic60PaladinTalents [classic60PaladinTalentCount]uint8

type classic60PaladinTalentDescriptor struct {
	name               string
	maxRank, tree, row uint8
	prereq             classic60PaladinTalent
}

// Read-only schema. The count sentinel means there is no prerequisite arrow.
var classic60PaladinTalentDescriptors = [classic60PaladinTalentCount]classic60PaladinTalentDescriptor{
	classic60PaladinDivineStrength:                {"Divine Strength", 5, 0, 0, classic60PaladinTalentCount},
	classic60PaladinDivineIntellect:               {"Divine Intellect", 5, 0, 0, classic60PaladinTalentCount},
	classic60PaladinSpiritualFocus:                {"Spiritual Focus", 5, 0, 1, classic60PaladinTalentCount},
	classic60PaladinImprovedSealOfRighteousness:   {"Improved Seal of Righteousness", 5, 0, 1, classic60PaladinTalentCount},
	classic60PaladinHealingLight:                  {"Healing Light", 3, 0, 2, classic60PaladinTalentCount},
	classic60PaladinConsecration:                  {"Consecration", 1, 0, 2, classic60PaladinTalentCount},
	classic60PaladinImprovedLayOnHands:            {"Improved Lay on Hands", 2, 0, 2, classic60PaladinTalentCount},
	classic60PaladinUnyieldingFaith:               {"Unyielding Faith", 2, 0, 2, classic60PaladinTalentCount},
	classic60PaladinIllumination:                  {"Illumination", 5, 0, 3, classic60PaladinTalentCount},
	classic60PaladinImprovedBlessingOfWisdom:      {"Improved Blessing of Wisdom", 2, 0, 3, classic60PaladinTalentCount},
	classic60PaladinDivineFavor:                   {"Divine Favor", 1, 0, 4, classic60PaladinIllumination},
	classic60PaladinLastingJudgement:              {"Lasting Judgement", 3, 0, 4, classic60PaladinTalentCount},
	classic60PaladinHolyPower:                     {"Holy Power", 5, 0, 5, classic60PaladinTalentCount},
	classic60PaladinHolyShock:                     {"Holy Shock", 1, 0, 6, classic60PaladinDivineFavor},
	classic60PaladinImprovedDevotionAura:          {"Improved Devotion Aura", 5, 1, 0, classic60PaladinTalentCount},
	classic60PaladinRedoubt:                       {"Redoubt", 5, 1, 0, classic60PaladinTalentCount},
	classic60PaladinPrecision:                     {"Precision", 3, 1, 1, classic60PaladinTalentCount},
	classic60PaladinGuardiansFavor:                {"Guardian's Favor", 2, 1, 1, classic60PaladinTalentCount},
	classic60PaladinToughness:                     {"Toughness", 5, 1, 1, classic60PaladinTalentCount},
	classic60PaladinBlessingOfKings:               {"Blessing of Kings", 1, 1, 2, classic60PaladinTalentCount},
	classic60PaladinImprovedRighteousFury:         {"Improved Righteous Fury", 3, 1, 2, classic60PaladinTalentCount},
	classic60PaladinShieldSpecialization:          {"Shield Specialization", 3, 1, 2, classic60PaladinRedoubt},
	classic60PaladinAnticipation:                  {"Anticipation", 5, 1, 2, classic60PaladinTalentCount},
	classic60PaladinImprovedHammerOfJustice:       {"Improved Hammer of Justice", 3, 1, 3, classic60PaladinTalentCount},
	classic60PaladinImprovedConcentrationAura:     {"Improved Concentration Aura", 3, 1, 3, classic60PaladinTalentCount},
	classic60PaladinBlessingOfSanctuary:           {"Blessing of Sanctuary", 1, 1, 4, classic60PaladinTalentCount},
	classic60PaladinReckoning:                     {"Reckoning", 5, 1, 4, classic60PaladinTalentCount},
	classic60PaladinOneHandedWeaponSpecialization: {"One-Handed Weapon Specialization", 5, 1, 5, classic60PaladinTalentCount},
	classic60PaladinHolyShield:                    {"Holy Shield", 1, 1, 6, classic60PaladinBlessingOfSanctuary},
	classic60PaladinImprovedBlessingOfMight:       {"Improved Blessing of Might", 5, 2, 0, classic60PaladinTalentCount},
	classic60PaladinBenediction:                   {"Benediction", 5, 2, 0, classic60PaladinTalentCount},
	classic60PaladinImprovedJudgement:             {"Improved Judgement", 2, 2, 1, classic60PaladinTalentCount},
	classic60PaladinImprovedSealOfTheCrusader:     {"Improved Seal of the Crusader", 3, 2, 1, classic60PaladinTalentCount},
	classic60PaladinDeflection:                    {"Deflection", 5, 2, 1, classic60PaladinTalentCount},
	classic60PaladinVindication:                   {"Vindication", 3, 2, 2, classic60PaladinTalentCount},
	classic60PaladinConviction:                    {"Conviction", 5, 2, 2, classic60PaladinTalentCount},
	classic60PaladinSealOfCommand:                 {"Seal of Command", 1, 2, 2, classic60PaladinTalentCount},
	classic60PaladinPursuitOfJustice:              {"Pursuit of Justice", 2, 2, 2, classic60PaladinTalentCount},
	classic60PaladinEyeForAnEye:                   {"Eye for an Eye", 2, 2, 3, classic60PaladinTalentCount},
	classic60PaladinImprovedRetributionAura:       {"Improved Retribution Aura", 2, 2, 3, classic60PaladinTalentCount},
	classic60PaladinTwoHandedWeaponSpecialization: {"Two-Handed Weapon Specialization", 3, 2, 4, classic60PaladinTalentCount},
	classic60PaladinSanctityAura:                  {"Sanctity Aura", 1, 2, 4, classic60PaladinTalentCount},
	classic60PaladinVengeance:                     {"Vengeance", 5, 2, 5, classic60PaladinConviction},
	classic60PaladinRepentance:                    {"Repentance", 1, 2, 6, classic60PaladinTalentCount},
}

// Parse the Classic calculator format: Holy-Protection-Retribution, allowing
// omitted trailing ranks and trees. Empty input is an unspent build. Structural
// validity does not imply that every selected talent has a runtime effect.
func parseClassic60PaladinTalents(value string) (classic60PaladinTalents, error) {
	var talents classic60PaladinTalents
	trees := strings.Split(value, "-")
	if len(trees) > 3 {
		return talents, fmt.Errorf("Classic Paladin talents require at most three trees")
	}
	offset := 0
	for tree, digits := range trees {
		size := [...]int{14, 15, 15}[tree]
		if len(digits) > size {
			return classic60PaladinTalents{}, fmt.Errorf("Classic Paladin tree %d has more than %d talent slots", tree+1, size)
		}
		for slot := 0; slot < len(digits); slot++ {
			if digits[slot] < '0' || digits[slot] > '9' {
				return classic60PaladinTalents{}, fmt.Errorf("Classic Paladin talent ranks must be ASCII digits")
			}
			talents[offset+slot] = digits[slot] - '0'
		}
		offset += size
	}
	if err := talents.validate(); err != nil {
		return classic60PaladinTalents{}, err
	}
	return talents, nil
}

func (talents classic60PaladinTalents) validate() error {
	var pointsByRow [3][7]int
	total := 0
	for talent, rank := range talents {
		descriptor := classic60PaladinTalentDescriptors[talent]
		if rank > descriptor.maxRank {
			return fmt.Errorf("%s has rank %d, maximum %d", descriptor.name, rank, descriptor.maxRank)
		}
		pointsByRow[descriptor.tree][descriptor.row] += int(rank)
		total += int(rank)
	}
	// The level-60 point budget is 60 - 9, independent of TBC's public cap.
	if total > 51 {
		return fmt.Errorf("Classic level-60 Paladin talents spend %d points, maximum 51", total)
	}
	for talent, rank := range talents {
		if rank == 0 {
			continue
		}
		descriptor := classic60PaladinTalentDescriptors[talent]
		lowerPoints := 0
		for row := uint8(0); row < descriptor.row; row++ {
			lowerPoints += pointsByRow[descriptor.tree][row]
		}
		if lowerPoints < 5*int(descriptor.row) {
			return fmt.Errorf("%s requires %d points in earlier rows of its tree", descriptor.name, 5*int(descriptor.row))
		}
		if descriptor.prereq != classic60PaladinTalentCount {
			prerequisite := classic60PaladinTalentDescriptors[descriptor.prereq]
			if talents[descriptor.prereq] != prerequisite.maxRank {
				return fmt.Errorf("%s requires %d/%d %s", descriptor.name, prerequisite.maxRank, prerequisite.maxRank, prerequisite.name)
			}
		}
	}
	return nil
}

// String returns three trees with trailing zero ranks removed. Like parsing,
// serialization is separate from runtime support for the selected talents.
func (talents classic60PaladinTalents) String() string {
	var trees [3]string
	for tree := range trees {
		var digits strings.Builder
		for talent, rank := range talents {
			if int(classic60PaladinTalentDescriptors[talent].tree) == tree {
				fmt.Fprint(&digits, rank)
			}
		}
		trees[tree] = strings.TrimRight(digits.String(), "0")
	}
	return strings.Join(trees[:], "-")
}
