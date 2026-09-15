package stats

import "github.com/wowsims/tbc/sim/core/proto"

// WeaponSkillCategoryLen is the number of stable WeaponSkillCategory indices
// carried by the engine. Category values describe bonus buckets only; their
// unit and combat effects remain ruleset-owned.
const WeaponSkillCategoryLen = int(proto.WeaponSkillCategory_WeaponSkillCategoryWands) + 1

// WeaponSkillBonuses stores category-specific bonuses without adding them to
// the regular Stats or PseudoStats vectors.
type WeaponSkillBonuses [WeaponSkillCategoryLen]float64

func WeaponSkillBonusesFromProto(values []float64) WeaponSkillBonuses {
	var bonuses WeaponSkillBonuses
	copy(bonuses[:], values)
	bonuses[proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified] = 0
	return bonuses
}

func (bonuses WeaponSkillBonuses) ToProtoArray() []float64 {
	bonuses[proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified] = 0
	return bonuses[:]
}

func (bonuses WeaponSkillBonuses) Add(other WeaponSkillBonuses) WeaponSkillBonuses {
	bonuses[proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified] = 0
	for category := 1; category < len(bonuses); category++ {
		bonuses[category] += other[category]
	}
	return bonuses
}

func (bonuses WeaponSkillBonuses) Subtract(other WeaponSkillBonuses) WeaponSkillBonuses {
	bonuses[proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified] = 0
	for category := 1; category < len(bonuses); category++ {
		bonuses[category] -= other[category]
	}
	return bonuses
}

func (bonuses WeaponSkillBonuses) Get(category proto.WeaponSkillCategory) float64 {
	if category <= proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified || int(category) >= len(bonuses) {
		return 0
	}
	return bonuses[category]
}
