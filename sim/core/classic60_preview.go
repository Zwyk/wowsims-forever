package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

// This diagnostic facade is intentionally separate from the simulation/proto
// API. It exposes only value snapshots of the pinned Classic baseline: callers
// cannot obtain a partially converted live Character or request a TBC fight.
const classic60PreviewSourceCommit = "7779ebbf79dc7f1341e6ab939b28a3402c9a730a"

type classic60PreviewRequest struct {
	Operation      string      `json:"operation"`
	Level          int32       `json:"level"`
	Race           proto.Race  `json:"race"`
	Class          proto.Class `json:"class"`
	PassiveRacials bool        `json:"passiveRacials"`
}

type classic60PreviewPair struct {
	Race      proto.Race  `json:"race"`
	RaceName  string      `json:"raceName"`
	Class     proto.Class `json:"class"`
	ClassName string      `json:"className"`
}

type classic60PreviewStat struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type classic60PreviewAvoidance struct {
	DodgePercent float64 `json:"dodgePercent"`
	ParryPercent float64 `json:"parryPercent"`
	BlockPercent float64 `json:"blockPercent"`
	CanParry     bool    `json:"canParry"`
	CanBlock     bool    `json:"canBlock"`
}

type classic60PreviewSnapshot struct {
	classic60PreviewPair
	Level                          int32                     `json:"level"`
	PassiveRacials                 bool                      `json:"passiveRacials"`
	SourceCommit                   string                    `json:"sourceCommit"`
	Rounding                       string                    `json:"rounding"`
	Stats                          []classic60PreviewStat    `json:"stats"`
	BaseMana                       float64                   `json:"baseMana"`
	HasMana                        bool                      `json:"hasMana"`
	Avoidance                      classic60PreviewAvoidance `json:"avoidance"`
	ArmorDamageMultiplierAgainst63 float64                   `json:"armorDamageMultiplierAgainst63"`
}

// Classic60PreviewJSON is the bounded JSON bridge for the standalone baseline
// lab. Unknown fields/operations, trailing JSON and unsupported combinations
// fail closed. No simulation request, gear, talent or global-profile mutation
// is accepted. The caller receives either {"data": ...} or {"error": ...}.
func Classic60PreviewJSON(input string) string {
	data, err := classic60PreviewDecode(input)
	var envelope any
	if err != nil {
		envelope = struct {
			Error string `json:"error"`
		}{err.Error()}
	} else {
		envelope = struct {
			Data any `json:"data"`
		}{data}
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return `{"error":"could not encode baseline"}`
	}
	return string(encoded)
}

func classic60PreviewDecode(input string) (any, error) {
	if len(input) > 4096 {
		return nil, fmt.Errorf("request exceeds 4096 bytes")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(input))
	decoder.DisallowUnknownFields()
	var request classic60PreviewRequest
	if err := decoder.Decode(&request); err != nil {
		return nil, fmt.Errorf("invalid baseline request: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("expected exactly one JSON request")
	}
	switch request.Operation {
	case "catalog":
		if request.Level != 0 || request.Race != 0 || request.Class != 0 || request.PassiveRacials {
			return nil, fmt.Errorf("catalog does not accept character parameters")
		}
		return classic60PreviewCatalog(), nil
	case "calculate":
		return classic60PreviewCalculate(request)
	default:
		return nil, fmt.Errorf("operation must be catalog or calculate")
	}
}

func classic60PreviewCatalog() []classic60PreviewPair {
	races := []struct {
		id   proto.Race
		name string
	}{
		{proto.Race_RaceHuman, "Human"}, {proto.Race_RaceDwarf, "Dwarf"},
		{proto.Race_RaceNightElf, "Night Elf"}, {proto.Race_RaceGnome, "Gnome"},
		{proto.Race_RaceOrc, "Orc"}, {proto.Race_RaceTauren, "Tauren"},
		{proto.Race_RaceTroll, "Troll"}, {proto.Race_RaceUndead, "Undead"},
	}
	classes := []struct {
		id   proto.Class
		name string
	}{
		{proto.Class_ClassWarrior, "Warrior"}, {proto.Class_ClassPaladin, "Paladin"},
		{proto.Class_ClassHunter, "Hunter"}, {proto.Class_ClassRogue, "Rogue"},
		{proto.Class_ClassPriest, "Priest"}, {proto.Class_ClassShaman, "Shaman"},
		{proto.Class_ClassMage, "Mage"}, {proto.Class_ClassWarlock, "Warlock"},
		{proto.Class_ClassDruid, "Druid"},
	}
	pairs := make([]classic60PreviewPair, 0, 40)
	for _, race := range races {
		for _, class := range classes {
			if classicReferenceSupportsRaceClass(race.id, class.id) {
				pairs = append(pairs, classic60PreviewPair{race.id, race.name, class.id, class.name})
			}
		}
	}
	return pairs
}

func classic60PreviewCalculate(request classic60PreviewRequest) (classic60PreviewSnapshot, error) {
	baseline, ok := classicReferenceInitializeCharacterWithPassiveRacials(request.Level, request.Race, request.Class, request.PassiveRacials)
	if !ok {
		return classic60PreviewSnapshot{}, fmt.Errorf("only level 60 and the 40 original Classic race/class combinations are supported")
	}
	result := classic60PreviewSnapshot{
		Level: baseline.level, PassiveRacials: request.PassiveRacials,
		SourceCommit: classic60PreviewSourceCommit,
		Rounding:     "Pinned Classic source: fractional primary sources; no final stat flooring (not verified Forever rounding).",
		BaseMana:     baseline.baseMana, HasMana: baseline.hasMana,
		Avoidance: classic60PreviewAvoidance{
			DodgePercent: baseline.avoidance.dodgePercent,
			ParryPercent: baseline.avoidance.effectiveParryPercent(),
			BlockPercent: baseline.avoidance.effectiveBlockPercent(),
			CanParry:     baseline.avoidance.canParry, CanBlock: baseline.avoidance.canBlock,
		},
	}
	for _, pair := range classic60PreviewCatalog() {
		if pair.Race == request.Race && pair.Class == request.Class {
			result.classic60PreviewPair = pair
			break
		}
	}
	for _, field := range []struct {
		stat stats.Stat
		name string
		unit string
	}{
		{stats.Strength, "Strength", ""}, {stats.Agility, "Agility", ""},
		{stats.Stamina, "Stamina", ""}, {stats.Intellect, "Intellect", ""},
		{stats.Spirit, "Spirit", ""}, {stats.Health, "Health", ""}, {stats.Mana, "Mana", ""},
		{stats.Armor, "Armor", ""}, {stats.AttackPower, "Attack Power", ""},
		{stats.RangedAttackPower, "Ranged Attack Power", ""},
		{stats.PhysicalCritPercent, "Physical Crit", "%"}, {stats.SpellCritPercent, "Spell Crit", "%"},
		{stats.ArcaneResistance, "Arcane Resistance", ""}, {stats.FireResistance, "Fire Resistance", ""},
		{stats.FrostResistance, "Frost Resistance", ""}, {stats.NatureResistance, "Nature Resistance", ""},
		{stats.ShadowResistance, "Shadow Resistance", ""},
	} {
		result.Stats = append(result.Stats, classic60PreviewStat{field.name, baseline.stats[field.stat], field.unit})
	}
	rules := rulesetProfile{levels: levelRules{characterLevel: 60, defaultBossLevelDelta: 3}}
	result.ArmorDamageMultiplierAgainst63, ok = classicReferenceArmorDamageModifier(rules, classicReferenceArmorInput{
		attackerLevel: 63, defenderArmor: baseline.stats[stats.Armor],
	})
	if !ok {
		return classic60PreviewSnapshot{}, fmt.Errorf("baseline armor was rejected by the Classic reference")
	}
	return result, nil
}
