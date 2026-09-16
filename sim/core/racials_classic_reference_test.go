package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func TestClassicReferencePassiveRacialScopeAndIsolation(t *testing.T) {
	wants := map[proto.Race]classicReferencePassiveRacials{
		proto.Race_RaceHuman:    {1, 1.05, 1, 0, stats.Stats{}},
		proto.Race_RaceDwarf:    {1, 1, 1, 0, stats.Stats{stats.FrostResistance: 10}},
		proto.Race_RaceNightElf: {1, 1, 1, 1, stats.Stats{stats.NatureResistance: 10}},
		proto.Race_RaceGnome:    {1.05, 1, 1, 0, stats.Stats{stats.ArcaneResistance: 10}},
		proto.Race_RaceOrc:      {1, 1, 1, 0, stats.Stats{}},
		proto.Race_RaceTauren:   {1, 1, 1.05, 0, stats.Stats{stats.NatureResistance: 10}},
		proto.Race_RaceTroll:    {1, 1, 1, 0, stats.Stats{}},
		proto.Race_RaceUndead:   {1, 1, 1, 0, stats.Stats{stats.ShadowResistance: 10}},
	}
	races := []proto.Race{-2147483648, -1, 11, 999, 2147483647}
	for race := range proto.Race_name {
		races = append(races, proto.Race(race))
	}
	for _, level := range []int32{-2147483648, -1, 0, 59, 60, 61, 70, 2147483647} {
		// A hostile inherited policy must be irrelevant to the level/race
		// lookup. Only own level, not boss delta or TBC numerical data, gates it.
		base := inheritedTBCRuleset()
		base.levels = levelRules{characterLevel: level, defaultBossLevelDelta: -99}
		for _, race := range races {
			want, supported := wants[race]
			supported = supported && level == 60
			got, ok := classicReferenceLevel60PassiveRacials(base, race)
			if !supported {
				if ok || got != (classicReferencePassiveRacials{}) {
					t.Errorf("unsupported level/race %d/%d returned %+v, %v", level, race, got, ok)
				}
				continue
			}
			if !ok || got != want {
				t.Fatalf("race %v = %+v, %v; want %+v", race, got, ok, want)
			}
			for stat := range got.bonusStats {
				got.bonusStats[stat] = -999
			}
			got.healthMultiplier = -999
			again, ok := classicReferenceLevel60PassiveRacials(base, race)
			if !ok || again != want {
				t.Fatalf("race %v returned mutable shared data", race)
			}
		}
	}
}

func TestClassicReferenceRacialResistanceComposition(t *testing.T) {
	for _, test := range []struct {
		race  proto.Race
		class proto.Class
		stat  stats.Stat
	}{
		{proto.Race_RaceDwarf, proto.Class_ClassWarrior, stats.FrostResistance},
		{proto.Race_RaceGnome, proto.Class_ClassMage, stats.ArcaneResistance},
		{proto.Race_RaceNightElf, proto.Class_ClassDruid, stats.NatureResistance},
		{proto.Race_RaceTauren, proto.Class_ClassShaman, stats.NatureResistance},
		{proto.Race_RaceUndead, proto.Class_ClassPriest, stats.ShadowResistance},
	} {
		t.Run(test.race.String(), func(t *testing.T) {
			baseline, ok := classicReferenceInitializeCharacterWithRacials(60, test.race, test.class)
			if !ok {
				t.Fatal("supported post-racial baseline rejected")
			}
			rules := rulesetProfile{levels: levelRules{characterLevel: 60, defaultBossLevelDelta: 3}}
			input := classicReferenceResistanceInput{
				attackerLevel: 63, defenderLevel: baseline.level, selectedResistance: baseline.stats[test.stat],
			}
			partial, ok := classicReferencePartialResistProjection(rules, input, false)
			if !ok {
				t.Fatal("racial resistance rejected")
			}
			assertClassicReferencePartialResistView(t, partial, classicReferencePartialResistView{
				coefficient: 0.031746031746031744, threshold00: 0.07238095238095238,
				threshold25: 0.02, threshold50: 0.002857142857142857,
			})
			binary, ok := classicReferenceBinaryResistProjection(rules, input)
			if !ok {
				t.Fatal("binary racial resistance rejected")
			}
			assertFloat64(t, "binary coefficient", binary.coefficient, 0.031746031746031744)
			assertFloat64(t, "binary base-hit multiplier", binary.baseHitChanceMultiplier, 0.9761904761904762)
			input.flatSpellPenetration = 10
			penetrated, ok := classicReferencePartialResistProjection(rules, input, false)
			if !ok || penetrated != (classicReferencePartialResistView{}) {
				t.Fatalf("fully penetrated racial resistance = %+v, %v", penetrated, ok)
			}
		})
	}
}
