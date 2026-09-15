package core

import (
	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

// weaponSkillBonus returns the accumulated bonus for one category. These
// values are deliberately outside the indexed Stats and serialized PseudoStat
// vectors and are not consumed by combat outcomes yet.
func (character *Character) weaponSkillBonus(category proto.WeaponSkillCategory) float64 {
	return character.PseudoStats.WeaponSkillBonuses.Get(category)
}

func (character *Character) addWeaponSkillBonus(category proto.WeaponSkillCategory, amount float64) {
	if category <= proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified || int(category) >= stats.WeaponSkillCategoryLen {
		panic("invalid weapon skill category")
	}
	character.PseudoStats.WeaponSkillBonuses[category] += amount
}

func (character *Character) addWeaponSkillBonuses(bonuses stats.WeaponSkillBonuses) {
	character.PseudoStats.WeaponSkillBonuses = character.PseudoStats.WeaponSkillBonuses.Add(bonuses)
}

func weaponSkillCategoryFromItem(item *Item) proto.WeaponSkillCategory {
	if item == nil || item.ID == 0 {
		return proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified
	}

	hasMeleeType := item.WeaponType != proto.WeaponType_WeaponTypeUnknown
	hasRangedType := item.RangedWeaponType != proto.RangedWeaponType_RangedWeaponTypeUnknown
	if hasMeleeType == hasRangedType {
		return proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified
	}

	if hasRangedType {
		if item.Type != proto.ItemType_ItemTypeRanged {
			return proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified
		}
		// Ranged item records do not use HandType. Treat conflicting metadata as
		// malformed instead of assigning a plausible-looking skill category.
		if item.HandType != proto.HandType_HandTypeUnknown {
			return proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified
		}
		switch item.RangedWeaponType {
		case proto.RangedWeaponType_RangedWeaponTypeBow:
			return proto.WeaponSkillCategory_WeaponSkillCategoryBows
		case proto.RangedWeaponType_RangedWeaponTypeCrossbow:
			return proto.WeaponSkillCategory_WeaponSkillCategoryCrossbows
		case proto.RangedWeaponType_RangedWeaponTypeGun:
			return proto.WeaponSkillCategory_WeaponSkillCategoryGuns
		case proto.RangedWeaponType_RangedWeaponTypeThrown:
			return proto.WeaponSkillCategory_WeaponSkillCategoryThrown
		case proto.RangedWeaponType_RangedWeaponTypeWand:
			return proto.WeaponSkillCategory_WeaponSkillCategoryWands
		default:
			return proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified
		}
	}
	if item.Type != proto.ItemType_ItemTypeWeapon {
		return proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified
	}

	isOneHanded := item.HandType == proto.HandType_HandTypeMainHand ||
		item.HandType == proto.HandType_HandTypeOneHand ||
		item.HandType == proto.HandType_HandTypeOffHand
	isTwoHanded := item.HandType == proto.HandType_HandTypeTwoHand

	switch item.WeaponType {
	case proto.WeaponType_WeaponTypeAxe:
		if isTwoHanded {
			return proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedAxes
		} else if isOneHanded {
			return proto.WeaponSkillCategory_WeaponSkillCategoryAxes
		}
	case proto.WeaponType_WeaponTypeSword:
		if isTwoHanded {
			return proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedSwords
		} else if isOneHanded {
			return proto.WeaponSkillCategory_WeaponSkillCategorySwords
		}
	case proto.WeaponType_WeaponTypeMace:
		if isTwoHanded {
			return proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedMaces
		} else if isOneHanded {
			return proto.WeaponSkillCategory_WeaponSkillCategoryMaces
		}
	case proto.WeaponType_WeaponTypeDagger:
		if isOneHanded {
			return proto.WeaponSkillCategory_WeaponSkillCategoryDaggers
		}
	case proto.WeaponType_WeaponTypeFist:
		if isOneHanded {
			return proto.WeaponSkillCategory_WeaponSkillCategoryUnarmed
		}
	case proto.WeaponType_WeaponTypePolearm:
		if isTwoHanded {
			return proto.WeaponSkillCategory_WeaponSkillCategoryPolearms
		}
	case proto.WeaponType_WeaponTypeStaff:
		if isTwoHanded {
			return proto.WeaponSkillCategory_WeaponSkillCategoryStaves
		}
	}

	return proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified
}

func weaponSkillCategoryFromEquippedItem(item *Item, source WeaponAttackSource) proto.WeaponSkillCategory {
	category := weaponSkillCategoryFromItem(item)
	if category == proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified {
		return category
	}

	switch source {
	case WeaponAttackSourceMainHand:
		if item.HandType == proto.HandType_HandTypeMainHand ||
			item.HandType == proto.HandType_HandTypeOneHand ||
			item.HandType == proto.HandType_HandTypeTwoHand {
			return category
		}
	case WeaponAttackSourceOffHand:
		if item.HandType == proto.HandType_HandTypeOffHand ||
			item.HandType == proto.HandType_HandTypeOneHand {
			return category
		}
	case WeaponAttackSourceRanged:
		if item.RangedWeaponType != proto.RangedWeaponType_RangedWeaponTypeUnknown {
			return category
		}
	}

	return proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified
}

// FeralCombatWeapon marks a synthetic player weapon as using the dedicated
// feral skill bucket. It does not change the weapon's damage or attack rules.
func FeralCombatWeapon(weapon Weapon) Weapon {
	weapon.classification = weaponClassification{
		kind:          weaponClassificationSynthetic,
		skillCategory: proto.WeaponSkillCategory_WeaponSkillCategoryFeralCombat,
	}
	return weapon
}
