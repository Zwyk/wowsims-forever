package core

import (
	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

// attributeRules contains only baseline dependency coefficients, not base
// stats, talents, racials, forms or pet-specific scaling. Keep it value-only so
// an injected profile cannot mutate another character's dependency inputs.
type attributeRules struct {
	healthPerStamina   float64
	playerHealthOffset float64
	armorPerAgility    float64
	manaPerIntellect   float64
	manaOffset         float64
	classes            [proto.Class_ClassDruid + 1]classAttributeRules
}

type classAttributeRules struct {
	attackPowerPerStrength        float64
	attackPowerPerAgility         float64
	rangedAttackPowerPerAgility   float64
	physicalCritPercentPerAgility float64
	spellCritPercentPerIntellect  float64
}

func (rules attributeRules) forClass(class proto.Class) classAttributeRules {
	switch class {
	case proto.Class_ClassWarrior, proto.Class_ClassPaladin, proto.Class_ClassHunter,
		proto.Class_ClassRogue, proto.Class_ClassPriest, proto.Class_ClassShaman,
		proto.Class_ClassMage, proto.Class_ClassWarlock, proto.Class_ClassDruid:
		return rules.classes[class]
	default:
		return classAttributeRules{}
	}
}

func (character *Character) addUniversalStatDependenciesWithRuleset(rules rulesetProfile) {
	character.Unit.addUniversalStatDependenciesWithRuleset(rules)
	character.AddStatDependency(stats.Stamina, stats.Health, rules.attributes.healthPerStamina)
	character.AddStatDependency(stats.Agility, stats.Armor, rules.attributes.armorPerAgility)
	// Pets share the universal dependencies, but not a player's base-health
	// correction. Offsets belong before the dependency manager's multipliers.
	if character.Type == PlayerUnit && rules.attributes.playerHealthOffset != 0 {
		character.AddStat(stats.Health, rules.attributes.playerHealthOffset)
	}
}

func (character *Character) addManaStatDependenciesWithRuleset(rules rulesetProfile) {
	if character.Type != PlayerUnit {
		// Pets install their own mana and spell-crit scaling.
		return
	}
	class := rules.attributes.forClass(character.Class)
	character.AddStatDependency(stats.Intellect, stats.SpellCritPercent, class.spellCritPercentPerIntellect)
	// The inherited linear model assumes at least 20 Intellect. Its -280
	// correction is applied before mana multipliers, not to base spell costs.
	character.AddStat(stats.Mana, rules.attributes.manaOffset)
	character.AddStatDependency(stats.Intellect, stats.Mana, rules.attributes.manaPerIntellect)
}

// AddBaseClassStatDependencies installs the player's baseline AP and physical
// crit conversions once, from the existing class construction hook. Keeping
// this separate from NewCharacter preserves class initialization order.
// Dodge/block, talents, forms, flat AP corrections and pets remain class-owned.
func (character *Character) AddBaseClassStatDependencies() {
	character.addBaseClassStatDependenciesWithRuleset(currentRuleset())
}

func (character *Character) addBaseClassStatDependenciesWithRuleset(rules rulesetProfile) {
	if character.Type != PlayerUnit {
		return
	}
	class := rules.attributes.forClass(character.Class)
	if class.attackPowerPerStrength != 0 {
		character.AddStatDependency(stats.Strength, stats.AttackPower, class.attackPowerPerStrength)
	}
	if class.attackPowerPerAgility != 0 {
		character.AddStatDependency(stats.Agility, stats.AttackPower, class.attackPowerPerAgility)
	}
	if class.rangedAttackPowerPerAgility != 0 {
		character.AddStatDependency(stats.Agility, stats.RangedAttackPower, class.rangedAttackPowerPerAgility)
	}
	if class.physicalCritPercentPerAgility != 0 {
		character.AddStatDependency(stats.Agility, stats.PhysicalCritPercent, class.physicalCritPercentPerAgility)
	}
}
