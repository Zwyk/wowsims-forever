package core

import (
	"maps"
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

// Literal ClassBaseStats + RaceOffsets vectors from wowsims/classic
// 7779ebbf79dc7f1341e6ab939b28a3402c9a730a, sim/core/base_stats.go.
// The 40 supported pairs are pinned separately in ui/core/proto_utils/utils.ts.
// Values are Str, Agi, Sta, Int, Spi, base health, base mana, base AP, base RAP;
// no resource adjustment, dependency, racial multiplier or ClassBaseCrit is applied.
func classicReferenceBaseAttributeVectors() map[BaseStatsKey]classicReferenceBaseAttributes {
	return map[BaseStatsKey]classicReferenceBaseAttributes{
		{proto.Race_RaceHuman, proto.Class_ClassWarrior}:    {120, 80, 110, 30, 45, 1689, 0, 160, 0},
		{proto.Race_RaceDwarf, proto.Class_ClassWarrior}:    {122, 76, 113, 29, 44, 1689, 0, 160, 0},
		{proto.Race_RaceNightElf, proto.Class_ClassWarrior}: {117, 85, 109, 30, 45, 1689, 0, 160, 0},
		{proto.Race_RaceGnome, proto.Class_ClassWarrior}:    {115, 83, 109, 33, 45, 1689, 0, 160, 0},
		{proto.Race_RaceOrc, proto.Class_ClassWarrior}:      {123, 77, 112, 27, 48, 1689, 0, 160, 0},
		{proto.Race_RaceTauren, proto.Class_ClassWarrior}:   {125, 75, 112, 25, 47, 1689, 0, 160, 0},
		{proto.Race_RaceTroll, proto.Class_ClassWarrior}:    {121, 82, 111, 26, 46, 1689, 0, 160, 0},
		{proto.Race_RaceUndead, proto.Class_ClassWarrior}:   {119, 78, 111, 28, 50, 1689, 0, 160, 0},
		{proto.Race_RaceHuman, proto.Class_ClassPaladin}:    {105, 65, 100, 70, 75, 1381, 1512, 160, 0},
		{proto.Race_RaceDwarf, proto.Class_ClassPaladin}:    {107, 61, 103, 69, 74, 1381, 1512, 160, 0},
		{proto.Race_RaceDwarf, proto.Class_ClassHunter}:     {57, 121, 93, 64, 69, 1467, 1720, 100, 100},
		{proto.Race_RaceNightElf, proto.Class_ClassHunter}:  {52, 130, 89, 65, 70, 1467, 1720, 100, 100},
		{proto.Race_RaceOrc, proto.Class_ClassHunter}:       {58, 122, 92, 62, 73, 1467, 1720, 100, 100},
		{proto.Race_RaceTauren, proto.Class_ClassHunter}:    {60, 120, 92, 60, 72, 1467, 1720, 100, 100},
		{proto.Race_RaceTroll, proto.Class_ClassHunter}:     {56, 127, 91, 61, 71, 1467, 1720, 100, 100},
		{proto.Race_RaceHuman, proto.Class_ClassRogue}:      {80, 130, 75, 35, 50, 1523, 0, 100, 0},
		{proto.Race_RaceDwarf, proto.Class_ClassRogue}:      {82, 126, 78, 34, 49, 1523, 0, 100, 0},
		{proto.Race_RaceNightElf, proto.Class_ClassRogue}:   {77, 135, 74, 35, 50, 1523, 0, 100, 0},
		{proto.Race_RaceGnome, proto.Class_ClassRogue}:      {75, 133, 74, 38, 50, 1523, 0, 100, 0},
		{proto.Race_RaceOrc, proto.Class_ClassRogue}:        {83, 127, 77, 32, 53, 1523, 0, 100, 0},
		{proto.Race_RaceTroll, proto.Class_ClassRogue}:      {81, 132, 76, 31, 51, 1523, 0, 100, 0},
		{proto.Race_RaceUndead, proto.Class_ClassRogue}:     {79, 128, 76, 33, 55, 1523, 0, 100, 0},
		{proto.Race_RaceHuman, proto.Class_ClassPriest}:     {35, 40, 50, 120, 125, 1397, 1376, -10, 0},
		{proto.Race_RaceDwarf, proto.Class_ClassPriest}:     {37, 36, 53, 119, 124, 1397, 1376, -10, 0},
		{proto.Race_RaceNightElf, proto.Class_ClassPriest}:  {32, 45, 49, 120, 125, 1397, 1376, -10, 0},
		{proto.Race_RaceTroll, proto.Class_ClassPriest}:     {36, 42, 51, 116, 126, 1397, 1376, -10, 0},
		{proto.Race_RaceUndead, proto.Class_ClassPriest}:    {34, 38, 51, 118, 130, 1397, 1376, -10, 0},
		{proto.Race_RaceOrc, proto.Class_ClassShaman}:       {88, 52, 97, 87, 103, 1280, 1520, 100, 0},
		{proto.Race_RaceTauren, proto.Class_ClassShaman}:    {90, 50, 97, 85, 102, 1280, 1520, 100, 0},
		{proto.Race_RaceTroll, proto.Class_ClassShaman}:     {86, 57, 96, 86, 101, 1280, 1520, 100, 0},
		{proto.Race_RaceHuman, proto.Class_ClassMage}:       {30, 35, 45, 125, 120, 1370, 1213, -10, 0},
		{proto.Race_RaceGnome, proto.Class_ClassMage}:       {25, 38, 44, 128, 120, 1370, 1213, -10, 0},
		{proto.Race_RaceTroll, proto.Class_ClassMage}:       {31, 37, 46, 121, 121, 1370, 1213, -10, 0},
		{proto.Race_RaceUndead, proto.Class_ClassMage}:      {29, 33, 46, 123, 125, 1370, 1213, -10, 0},
		{proto.Race_RaceHuman, proto.Class_ClassWarlock}:    {45, 50, 65, 110, 115, 1414, 1373, -10, 0},
		{proto.Race_RaceGnome, proto.Class_ClassWarlock}:    {40, 53, 64, 113, 115, 1414, 1373, -10, 0},
		{proto.Race_RaceOrc, proto.Class_ClassWarlock}:      {48, 47, 67, 107, 118, 1414, 1373, -10, 0},
		{proto.Race_RaceUndead, proto.Class_ClassWarlock}:   {44, 48, 66, 108, 120, 1414, 1373, -10, 0},
		{proto.Race_RaceNightElf, proto.Class_ClassDruid}:   {62, 65, 69, 100, 110, 1483, 1244, -20, 0},
		{proto.Race_RaceTauren, proto.Class_ClassDruid}:     {70, 55, 72, 95, 112, 1483, 1244, -20, 0},
	}
}

func TestClassicReferenceLevel60BaseAttributes(t *testing.T) {
	vectors := classicReferenceBaseAttributeVectors()
	if len(vectors) != 40 {
		t.Fatalf("literal reference vectors = %d, want 40", len(vectors))
	}
	for key, want := range vectors {
		t.Run(key.Race.String()+"/"+key.Class.String(), func(t *testing.T) {
			// Boss-level policy is deliberately not part of an attribute lookup.
			for _, bossDelta := range []int32{-4, 0, 3, 99} {
				rules := rulesetWithCharacterLevel(60)
				rules.levels.defaultBossLevelDelta = bossDelta
				got, ok := classicReferenceLevel60BaseAttributes(rules, key.Race, key.Class)
				if !ok || got != want {
					t.Fatalf("boss delta %d: got %+v, %v; want %+v, true", bossDelta, got, ok, want)
				}
			}
		})
	}
}

func TestClassicReferenceBaseAttributesScopeFailsClosed(t *testing.T) {
	wants := classicReferenceBaseAttributeVectors()
	races := []proto.Race{-1, 11, 999, -2147483648, 2147483647}
	for race := range proto.Race_name {
		races = append(races, proto.Race(race))
	}
	classes := []proto.Class{-1, 6, 10, 999, -2147483648, 2147483647}
	for class := range proto.Class_name {
		classes = append(classes, proto.Class(class))
	}
	for _, race := range races {
		for _, class := range classes {
			want, supported := wants[BaseStatsKey{Race: race, Class: class}]
			got, ok := classicReferenceLevel60BaseAttributes(rulesetWithCharacterLevel(60), race, class)
			if ok != supported || got != want {
				t.Errorf("race %d/class %d: got %+v, %v; want %+v, %v", race, class, got, ok, want, supported)
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
			got, ok := classicReferenceLevel60BaseAttributes(rulesetWithCharacterLevel(level), key.Race, key.Class)
			if ok || got != (classicReferenceBaseAttributes{}) {
				t.Errorf("level %d/%v: rejected lookup returned %+v, %v", level, key, got, ok)
			}
		}
	}
}

func TestClassicReferenceBaseAttributesCopyIsolation(t *testing.T) {
	rules := rulesetWithCharacterLevel(60)
	beforeRules := rules
	beforeBaseStats := maps.Clone(BaseStats)
	beforeClassStats := maps.Clone(ClassBaseStats)
	beforeRaceOffsets := maps.Clone(RaceOffsets)
	beforeExtraClassStats := maps.Clone(ExtraClassBaseStats)
	for key, want := range classicReferenceBaseAttributeVectors() {
		got, ok := classicReferenceLevel60BaseAttributes(rules, key.Race, key.Class)
		if !ok {
			t.Fatalf("supported pair %v rejected", key)
		}
		got.strength, got.agility, got.stamina, got.intellect, got.spirit = -999, -999, -999, -999, -999
		got.health, got.mana, got.attackPower, got.rangedAttackPower = -999, -999, -999, -999
		again, ok := classicReferenceLevel60BaseAttributes(rules, key.Race, key.Class)
		if !ok || again != want || again == got {
			t.Fatalf("mutating returned attributes affected pair %v: %+v, %v", key, again, ok)
		}
	}
	if rules != beforeRules || !maps.Equal(BaseStats, beforeBaseStats) || !maps.Equal(ClassBaseStats, beforeClassStats) ||
		!maps.Equal(RaceOffsets, beforeRaceOffsets) || !maps.Equal(ExtraClassBaseStats, beforeExtraClassStats) {
		t.Fatal("reference lookup mutated inherited rules or base-stat tables")
	}
}

func TestClassicReferenceBaseAttributesAreInactive(t *testing.T) {
	active := currentRuleset()
	if active != inheritedTBCRuleset() || active.levels.characterLevel != 70 || CharacterLevel != 70 {
		t.Fatal("active rules no longer match inherited level-70 TBC")
	}
	got, ok := classicReferenceLevel60BaseAttributes(active, proto.Race_RaceHuman, proto.Class_ClassWarrior)
	if ok || got != (classicReferenceBaseAttributes{}) {
		t.Fatalf("reference accepted active TBC profile: %+v, %v", got, ok)
	}

	// Exercise the constructor, not just the selector: a level-70 character must
	// keep the inherited row, including its independently stored baseline crit.
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
}
