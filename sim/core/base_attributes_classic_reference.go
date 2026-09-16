package core

import "github.com/wowsims/tbc/sim/core/proto"

// classicReferenceBaseAttributes is deliberately not a stats.Stats value: these
// are only the starting attributes, resource bases and AP offsets, not a complete
// character stat line. Health/mana exclude attribute contributions and their
// offsets (-180 health and -280 mana in the pinned Classic wiring). AP may be
// negative before attribute dependencies. Racials, base crit/dodge, conversions,
// forms, pets, gear and talents are outside this reference.
type classicReferenceBaseAttributes struct {
	strength, agility, stamina, intellect, spirit float64
	health, mana                                  float64
	attackPower, rangedAttackPower                float64
}

// classicReferenceLevel60BaseAttributes characterizes ClassBaseStats plus
// RaceOffsets from the pinned Classic simulator, not verified Forever data:
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/base_stats.go
// Adapted under MIT (copyright 2024 wowsims team; see LICENSE).
//
// The original Classic combinations are an explicit scope limit; unlike the
// source's map lookup, an unsupported race/class must not silently acquire a
// zero racial offset. No active profile or character constructor calls this.
func classicReferenceLevel60BaseAttributes(base rulesetProfile, race proto.Race, class proto.Class) (classicReferenceBaseAttributes, bool) {
	if base.levels.characterLevel != classicReferenceCharacterLevel || !classicReferenceSupportsRaceClass(race, class) {
		return classicReferenceBaseAttributes{}, false
	}

	var attributes classicReferenceBaseAttributes
	switch class {
	case proto.Class_ClassWarrior:
		attributes = classicReferenceBaseAttributes{
			strength: 120, agility: 80, stamina: 110, intellect: 30, spirit: 45,
			health: 1689, attackPower: 160,
		}
	case proto.Class_ClassPaladin:
		attributes = classicReferenceBaseAttributes{
			strength: 105, agility: 65, stamina: 100, intellect: 70, spirit: 75,
			health: 1381, mana: 1512, attackPower: 160,
		}
	case proto.Class_ClassHunter:
		attributes = classicReferenceBaseAttributes{
			strength: 55, agility: 125, stamina: 90, intellect: 65, spirit: 70,
			health: 1467, mana: 1720, attackPower: 100, rangedAttackPower: 100,
		}
	case proto.Class_ClassRogue:
		attributes = classicReferenceBaseAttributes{
			strength: 80, agility: 130, stamina: 75, intellect: 35, spirit: 50,
			health: 1523, attackPower: 100,
		}
	case proto.Class_ClassPriest:
		attributes = classicReferenceBaseAttributes{
			strength: 35, agility: 40, stamina: 50, intellect: 120, spirit: 125,
			health: 1397, mana: 1376, attackPower: -10,
		}
	case proto.Class_ClassShaman:
		attributes = classicReferenceBaseAttributes{
			strength: 85, agility: 55, stamina: 95, intellect: 90, spirit: 100,
			health: 1280, mana: 1520, attackPower: 100,
		}
	case proto.Class_ClassMage:
		attributes = classicReferenceBaseAttributes{
			strength: 30, agility: 35, stamina: 45, intellect: 125, spirit: 120,
			health: 1370, mana: 1213, attackPower: -10,
		}
	case proto.Class_ClassWarlock:
		attributes = classicReferenceBaseAttributes{
			strength: 45, agility: 50, stamina: 65, intellect: 110, spirit: 115,
			health: 1414, mana: 1373, attackPower: -10,
		}
	case proto.Class_ClassDruid:
		attributes = classicReferenceBaseAttributes{
			strength: 65, agility: 60, stamina: 70, intellect: 100, spirit: 110,
			health: 1483, mana: 1244, attackPower: -20,
		}
	default:
		return classicReferenceBaseAttributes{}, false
	}

	// Only the five primary attributes carry racial offsets in this source.
	// In particular, do not pre-apply Human Spirit, Expansive Mind or Endurance.
	var offset classicReferenceBaseAttributes
	switch race {
	case proto.Race_RaceHuman:
	case proto.Race_RaceOrc:
		offset = classicReferenceBaseAttributes{strength: 3, agility: -3, stamina: 2, intellect: -3, spirit: 3}
	case proto.Race_RaceDwarf:
		offset = classicReferenceBaseAttributes{strength: 2, agility: -4, stamina: 3, intellect: -1, spirit: -1}
	case proto.Race_RaceNightElf:
		offset = classicReferenceBaseAttributes{strength: -3, agility: 5, stamina: -1}
	case proto.Race_RaceUndead:
		offset = classicReferenceBaseAttributes{strength: -1, agility: -2, stamina: 1, intellect: -2, spirit: 5}
	case proto.Race_RaceTauren:
		offset = classicReferenceBaseAttributes{strength: 5, agility: -5, stamina: 2, intellect: -5, spirit: 2}
	case proto.Race_RaceGnome:
		offset = classicReferenceBaseAttributes{strength: -5, agility: 3, stamina: -1, intellect: 3}
	case proto.Race_RaceTroll:
		offset = classicReferenceBaseAttributes{strength: 1, agility: 2, stamina: 1, intellect: -4, spirit: 1}
	default:
		return classicReferenceBaseAttributes{}, false
	}
	attributes.strength += offset.strength
	attributes.agility += offset.agility
	attributes.stamina += offset.stamina
	attributes.intellect += offset.intellect
	attributes.spirit += offset.spirit
	return attributes, true
}

// This is our reference policy's support boundary, not Forever's race picker.
// Keep new combinations excluded even if both their race and class have rows.
func classicReferenceSupportsRaceClass(race proto.Race, class proto.Class) bool {
	switch class {
	case proto.Class_ClassWarrior:
		return race == proto.Race_RaceHuman || race == proto.Race_RaceDwarf || race == proto.Race_RaceNightElf ||
			race == proto.Race_RaceGnome || race == proto.Race_RaceOrc || race == proto.Race_RaceTauren ||
			race == proto.Race_RaceTroll || race == proto.Race_RaceUndead
	case proto.Class_ClassPaladin:
		return race == proto.Race_RaceHuman || race == proto.Race_RaceDwarf
	case proto.Class_ClassHunter:
		return race == proto.Race_RaceDwarf || race == proto.Race_RaceNightElf || race == proto.Race_RaceOrc ||
			race == proto.Race_RaceTauren || race == proto.Race_RaceTroll
	case proto.Class_ClassRogue:
		return race == proto.Race_RaceHuman || race == proto.Race_RaceDwarf || race == proto.Race_RaceNightElf ||
			race == proto.Race_RaceGnome || race == proto.Race_RaceOrc || race == proto.Race_RaceTroll || race == proto.Race_RaceUndead
	case proto.Class_ClassPriest:
		return race == proto.Race_RaceHuman || race == proto.Race_RaceDwarf || race == proto.Race_RaceNightElf ||
			race == proto.Race_RaceTroll || race == proto.Race_RaceUndead
	case proto.Class_ClassShaman:
		return race == proto.Race_RaceOrc || race == proto.Race_RaceTauren || race == proto.Race_RaceTroll
	case proto.Class_ClassMage:
		return race == proto.Race_RaceHuman || race == proto.Race_RaceGnome || race == proto.Race_RaceTroll || race == proto.Race_RaceUndead
	case proto.Class_ClassWarlock:
		return race == proto.Race_RaceHuman || race == proto.Race_RaceGnome || race == proto.Race_RaceOrc || race == proto.Race_RaceUndead
	case proto.Class_ClassDruid:
		return race == proto.Race_RaceNightElf || race == proto.Race_RaceTauren
	default:
		return false
	}
}
