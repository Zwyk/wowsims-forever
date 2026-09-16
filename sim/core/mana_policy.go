package core

import (
	"math"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

type manaModel uint8

const (
	manaModelInheritedTBC manaModel = iota
	manaModelUnavailable
	manaModelClassic60MageReference
)

type resourceRules struct {
	manaModel manaModel
}

func (unit *Unit) resolvedResourceRules() resourceRules {
	if unit != nil && unit.rulesInitialized {
		return unit.rules.resources
	}
	return currentRuleset().resources
}

func (table *AttackTable) resolvedResourceRules() resourceRules {
	if table.rulesInitialized {
		return table.resources
	}
	return table.Attacker.resolvedResourceRules()
}

// classic60MageManaReferenceRules extends the resource-free spell reference
// with a bounded pre-racial Human Mage mana path. It does not select a public
// ruleset, import the TBC Mage constructor, or support other classes or pets.
// Regeneration source:
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/mage/mage.go#L146-L149
// MP5, two-second ticks, FSR and completion spending:
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/mana.go
func classic60MageManaReferenceRules() rulesetProfile {
	rules := classic60SpellReferenceRules()
	rules.id = rulesetClassic60MageManaReference
	rules.resources = resourceRules{manaModel: manaModelClassic60MageReference}
	return rules
}

func (character *Character) validateManaRules() {
	switch character.resolvedResourceRules().manaModel {
	case manaModelInheritedTBC:
		return
	case manaModelClassic60MageReference:
		if !character.rulesInitialized || character.Type != PlayerUnit || character.Level != 60 ||
			character.Race != proto.Race_RaceHuman || character.Class != proto.Class_ClassMage ||
			character.HasRageBar() || character.HasEnergyBar() || character.HasFocusBar() {
			panic("Classic Mage mana reference requires a level-60 Human Mage player")
		}
		character.classic60MageReference = true
		character.manaRegenMultiplier = 1
	default:
		panic("mana is unavailable for this ruleset")
	}
}

func (unit *Unit) validateClassic60MageMana() {
	if !unit.rulesInitialized || unit.resolvedResourceRules().manaModel != manaModelClassic60MageReference ||
		unit.Type != PlayerUnit || unit.Level != 60 || unit.manaBar.unit != unit || !unit.classic60MageReference ||
		unit.HasRageBar() || unit.HasEnergyBar() || unit.HasFocusBar() {
		panic("Classic Mage mana reference requires a validated player mana bar")
	}
	if unit.PseudoStats.SpiritRegenMultiplier != 1 || unit.PseudoStats.SpiritRegenRateCasting != 0 ||
		unit.PseudoStats.ForceFullSpiritRegen || unit.manaRegenMultiplier != 1 {
		panic("Classic Mage mana reference does not support regeneration modifiers")
	}
	for _, stat := range []stats.Stat{stats.Spirit, stats.MP5} {
		if value := unit.stats[stat]; math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			panic("Classic Mage mana reference requires finite nonnegative Spirit and MP5")
		}
	}
}

func validateManaCostOptions(spell *Spell, options ManaCostOptions) {
	switch spell.Unit.resolvedResourceRules().manaModel {
	case manaModelInheritedTBC:
		return
	case manaModelClassic60MageReference:
		spell.Unit.validateClassic60MageMana()
		if options.FlatCost <= 0 || options.BaseCostPercent != 0 ||
			(options.PercentModifier != 0 && options.PercentModifier != 1) {
			panic("Classic Mage mana reference supports only unmodified positive integer flat costs")
		}
	default:
		panic("mana costs are unavailable for this ruleset")
	}
}

// Classic's cost calculation is floating-point, whereas inherited TBC uses an
// integer percentage bucket. Accept only their proven common unmodified flat
// cost subset; neither percentage costs nor cost reductions are characterized.
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell.go#L667-L684
func validateClassic60MageManaCost(spell *Spell, cost *SpellCost) {
	spell.Unit.validateClassic60MageMana()
	if cost == nil || cost.spell != spell || cost.BaseCost <= 0 || cost.FlatModifier != 0 ||
		cost.PercentModifier != 1 || cost.AdditivePercentModifier != 1 ||
		spell.Unit.PseudoStats.SpellCostPercentModifier != 100 {
		panic("Classic Mage mana reference requires an unmodified registered flat mana cost")
	}
	if mana, ok := cost.ResourceCostImpl.(*ManaCost); !ok || mana == nil || mana.ResourceMetrics == nil {
		panic("Classic Mage mana reference requires a mana cost with resource metrics")
	}
}

func validateClassicSpellResources(spell *Spell, table *AttackTable) {
	unit := spell.Unit
	if unit.HasRageBar() || unit.HasEnergyBar() || unit.HasFocusBar() {
		panic("Classic spell reference does not support rage, energy or focus")
	}
	if table.resolvedResourceRules().manaModel == manaModelClassic60MageReference {
		unit.validateClassic60MageMana()
		if spell.Cost != nil {
			validateClassic60MageManaCost(spell, spell.Cost)
		}
	} else if unit.HasManaBar() || spell.Cost != nil {
		panic("Classic spell reference does not support resources")
	}
}
