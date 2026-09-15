package core

import "github.com/wowsims/tbc/sim/core/proto"

// WeaponAttackSource identifies which current weapon supplies context for
// future per-weapon attack-table rules. It is deliberately separate from
// ProcMask, which controls proc eligibility and can describe multiple or
// non-weapon effects.
//
// The zero value is intentionally inert and means the spell has not been
// audited yet. None is explicit, so a future activation audit can distinguish
// intentionally non-weapon spells from missing annotations.
type WeaponAttackSource uint8

const (
	WeaponAttackSourceUnspecified WeaponAttackSource = iota
	WeaponAttackSourceNone
	WeaponAttackSourceMainHand
	WeaponAttackSourceOffHand
	WeaponAttackSourceRanged
)

type weaponClassificationKind uint8

const (
	weaponClassificationUnspecified weaponClassificationKind = iota
	weaponClassificationAbsent
	weaponClassificationEquippedItem
	weaponClassificationUnarmed
	weaponClassificationSynthetic
)

// weaponClassification preserves the equipped item's weapon categories and
// carries an explicit skill category for audited synthetic weapons. Generic
// pet, NPC and other synthetic weapons remain unspecified.
type weaponClassification struct {
	kind             weaponClassificationKind
	weaponType       proto.WeaponType
	handType         proto.HandType
	rangedWeaponType proto.RangedWeaponType
	skillCategory    proto.WeaponSkillCategory
}

func weaponClassificationFromItem(item *Item) weaponClassification {
	if item == nil || item.ID == 0 {
		return weaponClassification{kind: weaponClassificationAbsent}
	}
	return weaponClassification{
		kind:             weaponClassificationEquippedItem,
		weaponType:       item.WeaponType,
		handType:         item.HandType,
		rangedWeaponType: item.RangedWeaponType,
		skillCategory:    weaponSkillCategoryFromItem(item),
	}
}

// Abstract non-empty weapons enter the engine through public Weapon literals.
// Until they receive a specific skill category, mark them as synthetic so a
// player form or natural weapon never falls back to equipped item metadata.
func (weapon *Weapon) normalizeClassification() {
	if weapon.classification.kind != weaponClassificationUnspecified {
		return
	}
	if weapon.SwingSpeed == 0 {
		weapon.classification.kind = weaponClassificationAbsent
	} else {
		weapon.classification.kind = weaponClassificationSynthetic
	}
}

// weaponAttackContext is resolved from the corresponding AutoAttack slot at
// calculation time. It is a value snapshot and never caches item pointers.
type weaponAttackContext struct {
	source           WeaponAttackSource
	classification   weaponClassification
	weaponSkillBonus float64
}

func (spell *Spell) weaponAttackContext() weaponAttackContext {
	context := weaponAttackContext{source: spell.weaponAttackSource}
	if spell.Unit == nil || spell.weaponAttackSource == WeaponAttackSourceUnspecified || spell.weaponAttackSource == WeaponAttackSourceNone {
		return context
	}

	var weapon *Weapon
	switch spell.weaponAttackSource {
	case WeaponAttackSourceMainHand:
		weapon = spell.Unit.AutoAttacks.MH()
	case WeaponAttackSourceOffHand:
		weapon = spell.Unit.AutoAttacks.OH()
	case WeaponAttackSourceRanged:
		weapon = spell.Unit.AutoAttacks.Ranged()
	}

	character := spell.weaponAttackCharacter()
	if weapon != nil && weapon.classification.kind == weaponClassificationSynthetic {
		context.classification = weapon.classification
	} else if character != nil {
		context.classification = character.equippedWeaponClassification(spell.weaponAttackSource)
	} else if weapon != nil {
		context.classification = weapon.classification
	}
	if character != nil {
		context.weaponSkillBonus = character.weaponSkillBonus(context.classification.skillCategory)
	}
	return context
}

func (spell *Spell) weaponAttackCharacter() *Character {
	if spell.Unit.Type != PlayerUnit {
		return nil
	}
	if character := spell.Unit.AutoAttacks.character; character != nil {
		return character
	}
	if spell.Unit.Env == nil || spell.Unit.Env.Raid == nil {
		return nil
	}
	agent := spell.Unit.Env.Raid.GetPlayerFromUnit(spell.Unit)
	if agent == nil {
		return nil
	}
	return agent.GetCharacter()
}

func (character *Character) equippedWeaponClassification(source WeaponAttackSource) weaponClassification {
	var item *Item
	switch source {
	case WeaponAttackSourceMainHand:
		item = character.GetMHWeapon()
		if item == nil {
			return weaponClassification{
				kind:          weaponClassificationUnarmed,
				skillCategory: proto.WeaponSkillCategory_WeaponSkillCategoryUnarmed,
			}
		}
	case WeaponAttackSourceOffHand:
		item = character.GetOHWeapon()
	case WeaponAttackSourceRanged:
		item = character.GetRangedWeapon()
	}
	classification := weaponClassificationFromItem(item)
	classification.skillCategory = weaponSkillCategoryFromEquippedItem(item, source)
	return classification
}
