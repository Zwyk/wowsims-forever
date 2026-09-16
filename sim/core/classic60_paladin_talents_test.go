package core

import (
	"strings"
	"testing"
)

func TestClassic60PaladinTalentBuilds(t *testing.T) {
	tests := []struct {
		name, input, canonical string
		points                 [3]int
		selected               map[classic60PaladinTalent]uint8
	}{
		{"empty", "", "--", [3]int{}, nil},
		{"empty_sections", "-", "--", [3]int{}, nil},
		{"one_tree", "50000000000000", "5--", [3]int{5, 0, 0}, map[classic60PaladinTalent]uint8{classic60PaladinDivineStrength: 5}},
		{"two_trees", "5-05", "5-05-", [3]int{5, 5, 0}, map[classic60PaladinTalent]uint8{classic60PaladinRedoubt: 5}},
		// Pinned Classic Retribution preset, ui/retribution_paladin/presets.ts.
		{"retribution_11_8_32", "500501-503-52230351200315", "500501-503-52230351200315", [3]int{11, 8, 32},
			map[classic60PaladinTalent]uint8{classic60PaladinConsecration: 1, classic60PaladinPrecision: 3,
				classic60PaladinSealOfCommand: 1, classic60PaladinSanctityAura: 1, classic60PaladinVengeance: 5}},
		// Independently constructed level-60 builds exercise the complete Holy
		// and Protection paths instead of importing the stale Holy UI preset.
		{"holy_31_20_0", "55003120521151-55325", "55003120521151-55325-", [3]int{31, 20, 0},
			map[classic60PaladinTalent]uint8{classic60PaladinIllumination: 5, classic60PaladinDivineFavor: 1,
				classic60PaladinHolyPower: 5, classic60PaladinHolyShock: 1}},
		{"protection_11_31_9", "550001-553201330201051-0522", "550001-553201330201051-0522", [3]int{11, 31, 9},
			map[classic60PaladinTalent]uint8{classic60PaladinBlessingOfKings: 1, classic60PaladinShieldSpecialization: 3,
				classic60PaladinBlessingOfSanctuary: 1, classic60PaladinHolyShield: 1}},
		{"repentance_0_0_31", "--552050510003131", "--552050510003131", [3]int{0, 0, 31},
			map[classic60PaladinTalent]uint8{classic60PaladinRepentance: 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			talents, err := parseClassic60PaladinTalents(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got := talents.String(); got != test.canonical {
				t.Fatalf("canonical build = %q, want %q", got, test.canonical)
			}
			var points [3]int
			for talent, rank := range talents {
				points[classic60PaladinTalentDescriptors[talent].tree] += int(rank)
			}
			if points != test.points {
				t.Fatalf("tree points = %v, want %v", points, test.points)
			}
			for talent, want := range test.selected {
				if talents[talent] != want {
					t.Fatalf("%s rank = %d, want %d", classic60PaladinTalentDescriptors[talent].name, talents[talent], want)
				}
			}
			roundTrip, err := parseClassic60PaladinTalents(talents.String())
			if err != nil || roundTrip != talents {
				t.Fatalf("canonical round trip changed build: %v", err)
			}
		})
	}
}

func TestClassic60PaladinTalentParserRejectsInvalidInput(t *testing.T) {
	tests := []struct{ input, message string }{
		{"---", "three trees"},
		{"5-5-5-0", "three trees"},
		{"000000000000000", "talent slots"},
		{"-0000000000000000", "talent slots"},
		{"--0000000000000000", "talent slots"},
		{" 5", "ASCII digits"},
		{"5\n", "ASCII digits"},
		{"+5", "ASCII digits"},
		{"5.0", "ASCII digits"},
		{"５", "ASCII digits"},
		{"٥", "ASCII digits"},
		{"5\x00", "ASCII digits"},
		{"6", "Divine Strength"},
		{"550002", "Consecration"},
		{"-504", "Precision"},
		{"--55000052", "Seal of Command"},
		{"001", "earlier rows"},
		{"--00000001", "earlier rows"},
		{"--00000000000315", "earlier rows"},
		{"500501-003", "earlier rows"},
		{"510501-503-52230351200315", "maximum 51"},
		// The pinned Holy UI preset is not a valid Classic talent layout.
		{"50350151020013053100515221-50023131203", "talent slots"},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			talents, err := parseClassic60PaladinTalents(test.input)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error = %v, want %q", err, test.message)
			}
			if talents != (classic60PaladinTalents{}) {
				t.Fatal("invalid parse returned a partially populated build")
			}
		})
	}
}

func TestClassic60PaladinTalentPrerequisites(t *testing.T) {
	tests := []struct {
		name, build, prerequisite string
		change                    func(*classic60PaladinTalents)
	}{
		{"divine_favor", "55003120521151-55325-", "5/5 Illumination", func(talents *classic60PaladinTalents) {
			talents[classic60PaladinIllumination] = 4
		}},
		{"holy_shock", "55003120521151-55325-", "1/1 Divine Favor", func(talents *classic60PaladinTalents) {
			talents[classic60PaladinDivineFavor] = 0
			talents[classic60PaladinLastingJudgement]++
		}},
		{"shield_specialization", "550001-553201330201051-0522", "5/5 Redoubt", func(talents *classic60PaladinTalents) {
			talents[classic60PaladinRedoubt] = 4
		}},
		{"holy_shield", "550001-553201330201051-0522", "1/1 Blessing of Sanctuary", func(talents *classic60PaladinTalents) {
			talents[classic60PaladinBlessingOfSanctuary] = 0
			talents[classic60PaladinAnticipation]++
		}},
		{"vengeance", "500501-503-52230351200315", "5/5 Conviction", func(talents *classic60PaladinTalents) {
			talents[classic60PaladinConviction] = 4
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			talents, err := parseClassic60PaladinTalents(test.build)
			if err != nil {
				t.Fatal(err)
			}
			test.change(&talents)
			if err := talents.validate(); err == nil || !strings.Contains(err.Error(), test.prerequisite) {
				t.Fatalf("prerequisite validation = %v, want %q", err, test.prerequisite)
			}
		})
	}
}
