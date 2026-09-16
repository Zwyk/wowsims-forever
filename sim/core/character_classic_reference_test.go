package core

import (
	"maps"
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

type classicReferenceCharacterExpected struct {
	health, mana, armor, attackPower, rangedAP float64
	physicalCrit, spellCrit, dodge             float64
	baseMana                                   float64
	hasMana, canParry                          bool
}

func classicReferenceCharacterVectors() map[BaseStatsKey]classicReferenceCharacterExpected {
	// Literal resolved vectors from Classic@7779ebb raw rows plus the actual
	// class constructor dependencies, before racials/talents/gear/forms.
	// Columns: health, mana, armor, AP, RAP, physical crit, spell crit, dodge,
	// spell-cost base mana, mana-user capability, baseline parry capability.
	// Chance values are percentage points, never TBC combat ratings. Hunter's
	// missing dodge dependency and Mage/Priest's missing physical crit/dodge
	// dependencies characterize that source; these are not corrected game data.
	return map[BaseStatsKey]classicReferenceCharacterExpected{
		{proto.Race_RaceHuman, proto.Class_ClassWarrior}:    {2609, 0, 160, 400, 0, 4, 0, 4, 0, false, true},
		{proto.Race_RaceDwarf, proto.Class_ClassWarrior}:    {2639, 0, 152, 404, 0, 3.8, 0, 3.8, 0, false, true},
		{proto.Race_RaceNightElf, proto.Class_ClassWarrior}: {2599, 0, 170, 394, 0, 4.25, 0, 4.25, 0, false, true},
		{proto.Race_RaceGnome, proto.Class_ClassWarrior}:    {2599, 0, 166, 390, 0, 4.15, 0, 4.15, 0, false, true},
		{proto.Race_RaceOrc, proto.Class_ClassWarrior}:      {2629, 0, 154, 406, 0, 3.85, 0, 3.85, 0, false, true},
		{proto.Race_RaceTauren, proto.Class_ClassWarrior}:   {2629, 0, 150, 410, 0, 3.75, 0, 3.75, 0, false, true},
		{proto.Race_RaceTroll, proto.Class_ClassWarrior}:    {2619, 0, 164, 402, 0, 4.1, 0, 4.1, 0, false, true},
		{proto.Race_RaceUndead, proto.Class_ClassWarrior}:   {2619, 0, 156, 398, 0, 3.9, 0, 3.9, 0, false, true},
		{proto.Race_RaceHuman, proto.Class_ClassPaladin}:    {2201, 2282, 130, 370, 0, 3.989, 4.669, 3.989, 1512, true, true},
		{proto.Race_RaceDwarf, proto.Class_ClassPaladin}:    {2231, 2267, 122, 374, 0, 3.7866, 4.6523, 3.7866, 1512, true, true},
		{proto.Race_RaceDwarf, proto.Class_ClassHunter}:     {2217, 2400, 242, 278, 342, 2.2869, 4.656, 0, 1720, true, true},
		{proto.Race_RaceNightElf, proto.Class_ClassHunter}:  {2177, 2415, 260, 282, 360, 2.457, 4.6725, 0, 1720, true, true},
		{proto.Race_RaceOrc, proto.Class_ClassHunter}:       {2207, 2370, 244, 280, 344, 2.3058, 4.623, 0, 1720, true, true},
		{proto.Race_RaceTauren, proto.Class_ClassHunter}:    {2207, 2340, 240, 280, 340, 2.268, 4.59, 0, 1720, true, true},
		{proto.Race_RaceTroll, proto.Class_ClassHunter}:     {2197, 2355, 254, 283, 354, 2.4003, 4.6065, 0, 1720, true, true},
		{proto.Race_RaceHuman, proto.Class_ClassRogue}:      {2093, 0, 260, 310, 130, 4.485, 0, 8.97, 0, false, true},
		{proto.Race_RaceDwarf, proto.Class_ClassRogue}:      {2123, 0, 252, 308, 126, 4.347, 0, 8.694, 0, false, true},
		{proto.Race_RaceNightElf, proto.Class_ClassRogue}:   {2083, 0, 270, 312, 135, 4.6575, 0, 9.315, 0, false, true},
		{proto.Race_RaceGnome, proto.Class_ClassRogue}:      {2083, 0, 266, 308, 133, 4.5885, 0, 9.177, 0, false, true},
		{proto.Race_RaceOrc, proto.Class_ClassRogue}:        {2113, 0, 254, 310, 127, 4.3815, 0, 8.763, 0, false, true},
		{proto.Race_RaceTroll, proto.Class_ClassRogue}:      {2103, 0, 264, 313, 132, 4.554, 0, 9.108, 0, false, true},
		{proto.Race_RaceUndead, proto.Class_ClassRogue}:     {2103, 0, 256, 307, 128, 4.416, 0, 8.832, 0, false, true},
		{proto.Race_RaceHuman, proto.Class_ClassPriest}:     {1717, 2896, 80, 25, 0, 3, 2.816, 3, 1376, true, false},
		{proto.Race_RaceDwarf, proto.Class_ClassPriest}:     {1747, 2881, 72, 27, 0, 3, 2.7992, 3, 1376, true, false},
		{proto.Race_RaceNightElf, proto.Class_ClassPriest}:  {1707, 2896, 90, 22, 0, 3, 2.816, 3, 1376, true, false},
		{proto.Race_RaceTroll, proto.Class_ClassPriest}:     {1727, 2836, 84, 26, 0, 3, 2.7488, 3, 1376, true, false},
		{proto.Race_RaceUndead, proto.Class_ClassPriest}:    {1727, 2866, 76, 24, 0, 3, 2.7824, 3, 1376, true, false},
		{proto.Race_RaceOrc, proto.Class_ClassShaman}:       {2070, 2545, 104, 276, 0, 4.3416, 3.7703, 4.3416, 1520, true, false},
		{proto.Race_RaceTauren, proto.Class_ClassShaman}:    {2070, 2515, 100, 280, 0, 4.24, 3.7365, 4.24, 1520, true, false},
		{proto.Race_RaceTroll, proto.Class_ClassShaman}:     {2060, 2530, 114, 272, 0, 4.5956, 3.7534, 4.5956, 1520, true, false},
		{proto.Race_RaceHuman, proto.Class_ClassMage}:       {1640, 2808, 70, 20, 0, 3.2, 2.3, 3.2, 1213, true, false},
		{proto.Race_RaceGnome, proto.Class_ClassMage}:       {1630, 2853, 76, 15, 0, 3.2, 2.3504, 3.2, 1213, true, false},
		{proto.Race_RaceTroll, proto.Class_ClassMage}:       {1650, 2748, 74, 21, 0, 3.2, 2.2328, 3.2, 1213, true, false},
		{proto.Race_RaceUndead, proto.Class_ClassMage}:      {1650, 2778, 66, 19, 0, 3.2, 2.2664, 3.2, 1213, true, false},
		{proto.Race_RaceHuman, proto.Class_ClassWarlock}:    {1884, 2743, 100, 35, 0, 4.5, 3.515, 4.5, 1373, true, false},
		{proto.Race_RaceGnome, proto.Class_ClassWarlock}:    {1874, 2788, 106, 30, 0, 4.65, 3.5645, 4.65, 1373, true, false},
		{proto.Race_RaceOrc, proto.Class_ClassWarlock}:      {1904, 2698, 94, 38, 0, 4.35, 3.4655, 4.35, 1373, true, false},
		{proto.Race_RaceUndead, proto.Class_ClassWarlock}:   {1894, 2713, 96, 34, 0, 4.4, 3.482, 4.4, 1373, true, false},
		{proto.Race_RaceNightElf, proto.Class_ClassDruid}:   {1993, 2464, 130, 104, 0, 4.15, 3.47, 4.15, 1244, true, false},
		{proto.Race_RaceTauren, proto.Class_ClassDruid}:     {2023, 2389, 110, 120, 0, 3.65, 3.3865, 3.65, 1244, true, false},
	}
}

func TestClassicReferenceCharacterInitializationAllPairs(t *testing.T) {
	wants := classicReferenceCharacterVectors()
	rawVectors := classicReferenceBaseAttributeVectors()
	if len(wants) != 40 || len(rawVectors) != 40 {
		t.Fatalf("expected 40 literal resolved/raw vectors, got %d/%d", len(wants), len(rawVectors))
	}
	// Literal physical/spell crit seeds from the source's ClassBaseCrit,
	// independent of the base-crit lookup used by initialization.
	critSeeds := map[proto.Class][2]float64{
		proto.Class_ClassWarrior: {0, 0}, proto.Class_ClassPaladin: {0.7, 3.5},
		proto.Class_ClassHunter: {0, 3.6}, proto.Class_ClassRogue: {0, 0},
		proto.Class_ClassPriest: {3, 0.8}, proto.Class_ClassShaman: {1.7, 2.3},
		proto.Class_ClassMage: {3.2, 0.2}, proto.Class_ClassWarlock: {2, 1.7},
		proto.Class_ClassDruid: {0.9, 1.8},
	}
	for key, want := range wants {
		t.Run(key.Race.String()+"/"+key.Class.String(), func(t *testing.T) {
			raw, exists := rawVectors[key]
			if !exists {
				t.Fatal("resolved fixture is not an original race/class pair")
			}
			got, ok := classicReferenceInitializeCharacter(60, key.Race, key.Class)
			if !ok {
				t.Fatal("supported baseline rejected")
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
				t.Fatalf("raw base stats changed or include derived values: got %+v, want %+v", got.baseStats, wantBase)
			}
			wantStats := wantBase
			wantStats[stats.Health], wantStats[stats.Mana], wantStats[stats.Armor] = want.health, want.mana, want.armor
			wantStats[stats.AttackPower], wantStats[stats.RangedAttackPower] = want.attackPower, want.rangedAP
			wantStats[stats.PhysicalCritPercent], wantStats[stats.SpellCritPercent] = want.physicalCrit, want.spellCrit
			for stat := range wantStats {
				assertFloat64(t, stats.Stat(stat).StatName(), got.stats[stat], wantStats[stat])
			}
			assertFloat64(t, "raw spell-cost base mana", got.baseMana, want.baseMana)
			if got.hasMana != want.hasMana {
				t.Errorf("mana capability = %v, want %v", got.hasMana, want.hasMana)
			}
			assertFloat64(t, "dodge percentage points", got.avoidance.dodgePercent, want.dodge)
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

func TestClassicReferenceCharacterInitializationScopeFailsClosed(t *testing.T) {
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
			got, ok := classicReferenceInitializeCharacter(60, race, class)
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
			got, ok := classicReferenceInitializeCharacter(level, key.Race, key.Class)
			if ok || got != (classicReferenceCharacterBaseline{}) {
				t.Errorf("level %d/%v: rejected initialization returned %+v, %v", level, key, got, ok)
			}
		}
	}
}

func TestClassicReferenceCharacterInitializationPreRacialBoundary(t *testing.T) {
	for _, test := range []struct {
		race  proto.Race
		class proto.Class
		stat  stats.Stat
		want  float64
	}{
		{proto.Race_RaceHuman, proto.Class_ClassWarrior, stats.Spirit, 45},
		{proto.Race_RaceGnome, proto.Class_ClassMage, stats.Intellect, 128},
		{proto.Race_RaceGnome, proto.Class_ClassMage, stats.Mana, 2853},
		{proto.Race_RaceTauren, proto.Class_ClassWarrior, stats.Health, 2629},
		{proto.Race_RaceNightElf, proto.Class_ClassWarrior, stats.NatureResistance, 0},
	} {
		got, ok := classicReferenceInitializeCharacter(60, test.race, test.class)
		if !ok {
			t.Fatal("supported pre-racial character rejected")
		}
		assertFloat64(t, test.race.String()+" "+test.stat.StatName(), got.stats[test.stat], test.want)
	}
	nightElf, ok := classicReferenceInitializeCharacter(60, proto.Race_RaceNightElf, proto.Class_ClassWarrior)
	if !ok {
		t.Fatal("supported Night Elf rejected")
	}
	assertFloat64(t, "Night Elf dodge excludes Quickness", nightElf.avoidance.dodgePercent, 4.25)
}

func TestClassicReferenceBaselineAvoidanceCapabilityGates(t *testing.T) {
	avoidance := classicReferenceBaselineAvoidance{parryPercent: 5, blockPercent: 5}
	assertFloat64(t, "parry without capability", avoidance.effectiveParryPercent(), 0)
	assertFloat64(t, "block without capability", avoidance.effectiveBlockPercent(), 0)
	avoidance.canParry = true
	assertFloat64(t, "parry with capability", avoidance.effectiveParryPercent(), 5)
	assertFloat64(t, "parry capability cannot enable block", avoidance.effectiveBlockPercent(), 0)
	avoidance.canBlock = true
	avoidance.canParry = false
	assertFloat64(t, "block with capability", avoidance.effectiveBlockPercent(), 5)
	assertFloat64(t, "block capability cannot enable parry", avoidance.effectiveParryPercent(), 0)
}

func TestClassicReferenceCharacterInitializationIsolationAndInactivity(t *testing.T) {
	active := currentRuleset()
	beforeBaseStats, beforeClassStats := maps.Clone(BaseStats), maps.Clone(ClassBaseStats)
	beforeRaceOffsets, beforeExtraStats := maps.Clone(RaceOffsets), maps.Clone(ExtraClassBaseStats)
	beforeAgiCrit, beforeIntCrit := maps.Clone(CritPerAgiMaxLevel), maps.Clone(CritPerIntMaxLevel)
	for key := range classicReferenceCharacterVectors() {
		got, ok := classicReferenceInitializeCharacter(60, key.Race, key.Class)
		if !ok {
			t.Fatalf("supported pair %v rejected", key)
		}
		original := got
		for index := range got.stats {
			got.stats[index], got.baseStats[index] = -999, -999
		}
		got.level, got.race, got.class = -1, proto.Race_RaceUnknown, proto.Class_ClassUnknown
		got.baseMana, got.hasMana = -999, !got.hasMana
		got.avoidance = classicReferenceBaselineAvoidance{}
		again, ok := classicReferenceInitializeCharacter(60, key.Race, key.Class)
		if !ok || again != original || again == got {
			t.Fatalf("returned baseline aliases previous value for %v: %+v, %v", key, again, ok)
		}
	}
	if currentRuleset() != active || active != inheritedTBCRuleset() || active.levels.characterLevel != 70 || CharacterLevel != 70 {
		t.Fatal("reference initialization changed the active level-70 TBC profile")
	}
	if !maps.Equal(BaseStats, beforeBaseStats) || !maps.Equal(ClassBaseStats, beforeClassStats) ||
		!maps.Equal(RaceOffsets, beforeRaceOffsets) || !maps.Equal(ExtraClassBaseStats, beforeExtraStats) ||
		!maps.Equal(CritPerAgiMaxLevel, beforeAgiCrit) || !maps.Equal(CritPerIntMaxLevel, beforeIntCrit) {
		t.Fatal("reference initialization mutated active base-stat or crit tables")
	}
	// Exercise the live constructor after every reference pair: switching the
	// reference must not replace either its level or its independently stored row.
	character := NewCharacter(&Party{}, 0, &proto.Player{
		Race: proto.Race_RaceHuman, Class: proto.Class_ClassWarrior, Spec: &proto.Player_DpsWarrior{},
		Equipment: &proto.EquipmentSpec{},
	})
	want := stats.Stats{
		stats.Strength: 145, stats.Agility: 96, stats.Stamina: 133, stats.Intellect: 33, stats.Spirit: 51,
		stats.Health: 4264, stats.AttackPower: 190, stats.PhysicalCritPercent: 1.14,
	}
	if character.Level != 70 || character.GetBaseStats() != want {
		t.Fatalf("active character level/base stats = %d/%+v, want 70/%+v", character.Level, character.GetBaseStats(), want)
	}
	resolved := character.SortAndApplyStatDependencies(character.GetStats())
	assertFloat64(t, "active TBC health has no Classic -180 correction", resolved[stats.Health], 5594)
}
