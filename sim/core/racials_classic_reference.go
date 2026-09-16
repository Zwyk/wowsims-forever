package core

import (
	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

// classicReferencePassiveRacials describes only unconditional own-character
// stat effects in Classic@7779ebbf79dc7f1341e6ab939b28a3402c9a730a,
// sim/core/racials.go. Weapon specializations require equipment, Command applies
// to pets, Beast Slaying requires a target, and racial cooldowns require combat;
// none belong to this unequipped humanoid baseline. These are pinned Classic
// source semantics, not claims about Forever or its racial redesigns.
type classicReferencePassiveRacials struct {
	intellectMultiplier, spiritMultiplier, healthMultiplier float64
	bonusDodgePercent                                       float64
	bonusStats                                              stats.Stats
}

func classicReferenceLevel60PassiveRacials(base rulesetProfile, race proto.Race) (classicReferencePassiveRacials, bool) {
	if base.levels.characterLevel != 60 {
		return classicReferencePassiveRacials{}, false
	}
	rules := classicReferencePassiveRacials{
		intellectMultiplier: 1, spiritMultiplier: 1, healthMultiplier: 1,
	}
	switch race {
	case proto.Race_RaceHuman:
		rules.spiritMultiplier = 1.05
	case proto.Race_RaceDwarf:
		rules.bonusStats[stats.FrostResistance] = 10
	case proto.Race_RaceNightElf:
		rules.bonusStats[stats.NatureResistance] = 10
		rules.bonusDodgePercent = 1
	case proto.Race_RaceGnome:
		rules.bonusStats[stats.ArcaneResistance] = 10
		rules.intellectMultiplier = 1.05
	case proto.Race_RaceOrc, proto.Race_RaceTroll:
		// Neither race has an unconditional own-character stat effect here.
	case proto.Race_RaceTauren:
		rules.bonusStats[stats.NatureResistance] = 10
		// The source multiplies derived total health, not just base health.
		rules.healthMultiplier = 1.05
	case proto.Race_RaceUndead:
		rules.bonusStats[stats.ShadowResistance] = 10
	default:
		return classicReferencePassiveRacials{}, false
	}
	return rules, true
}
