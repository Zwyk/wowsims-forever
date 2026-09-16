package core

import (
	"maps"
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func TestClassicReferenceCharacterWithRacialsAllPairs(t *testing.T) {
	// Start from the independent literal fixtures, never from either production
	// initializer. Racial values below characterize Classic@7779ebb racials.go
	// and its fractional-source dependency evaluator, not verified Forever rules.
	wants := classicReferenceCharacterVectors()
	rawVectors := classicReferenceBaseAttributeVectors()
	if len(wants) != 40 || len(rawVectors) != 40 {
		t.Fatalf("expected 40 original resolved/raw vectors, got %d/%d", len(wants), len(rawVectors))
	}
	critSeeds := map[proto.Class][2]float64{
		proto.Class_ClassWarrior: {0, 0}, proto.Class_ClassPaladin: {0.7, 3.5},
		proto.Class_ClassHunter: {0, 3.6}, proto.Class_ClassRogue: {0, 0},
		proto.Class_ClassPriest: {3, 0.8}, proto.Class_ClassShaman: {1.7, 2.3},
		proto.Class_ClassMage: {3.2, 0.2}, proto.Class_ClassWarlock: {2, 1.7},
		proto.Class_ClassDruid: {0.9, 1.8},
	}
	// Literal post-racial values deliberately catch both primary-source flooring
	// and final-stat flooring. In particular Tauren Endurance multiplies the
	// complete derived health pool, not only base health or stamina health.
	changedStats := map[BaseStatsKey]stats.Stats{
		{proto.Race_RaceHuman, proto.Class_ClassWarrior}: {stats.Spirit: 47.25},
		{proto.Race_RaceHuman, proto.Class_ClassPaladin}: {stats.Spirit: 78.75},
		{proto.Race_RaceHuman, proto.Class_ClassRogue}:   {stats.Spirit: 52.5},
		{proto.Race_RaceHuman, proto.Class_ClassPriest}:  {stats.Spirit: 131.25},
		{proto.Race_RaceHuman, proto.Class_ClassMage}:    {stats.Spirit: 126},
		{proto.Race_RaceHuman, proto.Class_ClassWarlock}: {stats.Spirit: 120.75},
		{proto.Race_RaceGnome, proto.Class_ClassWarrior}: {stats.Intellect: 34.65},
		{proto.Race_RaceGnome, proto.Class_ClassRogue}:   {stats.Intellect: 39.9},
		{proto.Race_RaceGnome, proto.Class_ClassMage}: {
			stats.Intellect: 134.4, stats.Mana: 2949, stats.SpellCritPercent: 2.45792,
		},
		{proto.Race_RaceGnome, proto.Class_ClassWarlock}: {
			stats.Intellect: 118.65, stats.Mana: 2872.75, stats.SpellCritPercent: 3.657725,
		},
		{proto.Race_RaceTauren, proto.Class_ClassWarrior}: {stats.Health: 2760.45},
		{proto.Race_RaceTauren, proto.Class_ClassHunter}:  {stats.Health: 2317.35},
		{proto.Race_RaceTauren, proto.Class_ClassShaman}:  {stats.Health: 2173.5},
		{proto.Race_RaceTauren, proto.Class_ClassDruid}:   {stats.Health: 2124.15},
	}
	nightElfDodge := map[proto.Class]float64{
		proto.Class_ClassWarrior: 5.25, proto.Class_ClassHunter: 1,
		proto.Class_ClassRogue: 10.315, proto.Class_ClassPriest: 4, proto.Class_ClassDruid: 5.15,
	}
	for key, want := range wants {
		t.Run(key.Race.String()+"/"+key.Class.String(), func(t *testing.T) {
			raw, exists := rawVectors[key]
			if !exists {
				t.Fatal("resolved fixture is not an original race/class pair")
			}
			got, ok := classicReferenceInitializeCharacterWithRacials(60, key.Race, key.Class)
			if !ok {
				t.Fatal("supported racial baseline rejected")
			}
			if got.level != 60 || got.race != key.Race || got.class != key.Class {
				t.Fatalf("identity = %d/%v/%v, want 60/%v", got.level, got.race, got.class, key)
			}
			wantBase := stats.Stats{
				stats.Strength: raw.strength, stats.Agility: raw.agility, stats.Stamina: raw.stamina,
				stats.Intellect: raw.intellect, stats.Spirit: raw.spirit,
				stats.Health: raw.health, stats.Mana: raw.mana,
				stats.AttackPower: raw.attackPower, stats.RangedAttackPower: raw.rangedAttackPower,
				stats.PhysicalCritPercent: critSeeds[key.Class][0], stats.SpellCritPercent: critSeeds[key.Class][1],
			}
			if got.baseStats != wantBase {
				t.Fatalf("racial effects changed raw base stats: got %+v, want %+v", got.baseStats, wantBase)
			}
			wantStats := wantBase
			wantStats[stats.Health], wantStats[stats.Mana], wantStats[stats.Armor] = want.health, want.mana, want.armor
			wantStats[stats.AttackPower], wantStats[stats.RangedAttackPower] = want.attackPower, want.rangedAP
			wantStats[stats.PhysicalCritPercent], wantStats[stats.SpellCritPercent] = want.physicalCrit, want.spellCrit
			for stat, value := range changedStats[key] {
				if value != 0 {
					wantStats[stat] = value
				}
			}
			switch key.Race {
			case proto.Race_RaceDwarf:
				wantStats[stats.FrostResistance] = 10
			case proto.Race_RaceGnome:
				wantStats[stats.ArcaneResistance] = 10
			case proto.Race_RaceNightElf, proto.Race_RaceTauren:
				wantStats[stats.NatureResistance] = 10
			case proto.Race_RaceUndead:
				wantStats[stats.ShadowResistance] = 10
			}
			// Whole-array checking also rejects weapon expertise/hit injection,
			// defense-rating transport and active/conditional racial bonuses.
			for stat := range wantStats {
				assertFloat64(t, stats.Stat(stat).StatName(), got.stats[stat], wantStats[stat])
			}
			assertFloat64(t, "unmodified spell-cost base mana", got.baseMana, want.baseMana)
			if got.hasMana != want.hasMana {
				t.Errorf("mana capability = %v, want %v", got.hasMana, want.hasMana)
			}
			wantDodge := want.dodge
			if key.Race == proto.Race_RaceNightElf {
				wantDodge = nightElfDodge[key.Class]
			}
			assertFloat64(t, "dodge percentage points", got.avoidance.dodgePercent, wantDodge)
			assertFloat64(t, "raw parry percentage points", got.avoidance.parryPercent, 5)
			assertFloat64(t, "raw block percentage points", got.avoidance.blockPercent, 5)
			if got.avoidance.canParry != want.canParry || got.avoidance.canBlock {
				t.Errorf("avoidance capabilities = %v/%v, want %v/false", got.avoidance.canParry, got.avoidance.canBlock, want.canParry)
			}
			wantParry := 0.0
			if want.canParry {
				wantParry = 5
			}
			assertFloat64(t, "effective parry percentage points", got.avoidance.effectiveParryPercent(), wantParry)
			assertFloat64(t, "effective block without shield", got.avoidance.effectiveBlockPercent(), 0)
		})
	}
}

func TestClassicReferenceCharacterWithRacialsScopeFailsClosed(t *testing.T) {
	wants := classicReferenceCharacterVectors()
	races := []proto.Race{-2147483648, -1, 11, 999, 2147483647}
	for race := range proto.Race_name {
		races = append(races, proto.Race(race))
	}
	classes := []proto.Class{-2147483648, -1, 6, 10, 999, 2147483647}
	for class := range proto.Class_name {
		classes = append(classes, proto.Class(class))
	}
	for _, race := range races {
		for _, class := range classes {
			_, supported := wants[BaseStatsKey{Race: race, Class: class}]
			got, ok := classicReferenceInitializeCharacterWithRacials(60, race, class)
			if ok != supported || (!ok && got != (classicReferenceCharacterBaseline{})) {
				t.Errorf("race %d/class %d: returned %+v, %v; want supported=%v and zero on rejection", race, class, got, ok, supported)
			}
		}
	}
	levels := []int32{-2147483648, 2147483647}
	for level := int32(-1); level <= 100; level++ {
		if level != 60 {
			levels = append(levels, level)
		}
	}
	for _, level := range levels {
		for key := range wants {
			got, ok := classicReferenceInitializeCharacterWithRacials(level, key.Race, key.Class)
			if ok || got != (classicReferenceCharacterBaseline{}) {
				t.Errorf("level %d/%v: rejected initialization returned %+v, %v", level, key, got, ok)
			}
		}
	}
}

func TestClassicReferenceCharacterWithRacialsIsolationAndInactivity(t *testing.T) {
	active := currentRuleset()
	beforeBaseStats, beforeClassStats := maps.Clone(BaseStats), maps.Clone(ClassBaseStats)
	beforeRaceOffsets, beforeExtraStats := maps.Clone(RaceOffsets), maps.Clone(ExtraClassBaseStats)
	beforeAgiCrit, beforeIntCrit := maps.Clone(CritPerAgiMaxLevel), maps.Clone(CritPerIntMaxLevel)
	for key := range classicReferenceCharacterVectors() {
		preRacial, preOK := classicReferenceInitializeCharacter(60, key.Race, key.Class)
		got, ok := classicReferenceInitializeCharacterWithRacials(60, key.Race, key.Class)
		if !ok || !preOK {
			t.Fatalf("supported pair %v rejected", key)
		}
		original := got
		for index := range got.stats {
			got.stats[index], got.baseStats[index] = -999, -999
		}
		got.level, got.race, got.class = -1, proto.Race_RaceUnknown, proto.Class_ClassUnknown
		got.baseMana, got.hasMana = -999, !got.hasMana
		got.avoidance = classicReferenceBaselineAvoidance{}
		again, ok := classicReferenceInitializeCharacterWithRacials(60, key.Race, key.Class)
		if !ok || again != original || again == got {
			t.Fatalf("returned racial baseline aliases previous value for %v: %+v, %v", key, again, ok)
		}
		againPreRacial, ok := classicReferenceInitializeCharacter(60, key.Race, key.Class)
		if !ok || againPreRacial != preRacial {
			t.Fatalf("racial initialization changed pre-racial reference for %v", key)
		}
	}
	if currentRuleset() != active || active != inheritedTBCRuleset() || active.levels.characterLevel != 70 || CharacterLevel != 70 {
		t.Fatal("racial reference initialization changed the active level-70 TBC profile")
	}
	if !maps.Equal(BaseStats, beforeBaseStats) || !maps.Equal(ClassBaseStats, beforeClassStats) ||
		!maps.Equal(RaceOffsets, beforeRaceOffsets) || !maps.Equal(ExtraClassBaseStats, beforeExtraStats) ||
		!maps.Equal(CritPerAgiMaxLevel, beforeAgiCrit) || !maps.Equal(CritPerIntMaxLevel, beforeIntCrit) {
		t.Fatal("racial reference initialization mutated active base-stat or crit tables")
	}
}
