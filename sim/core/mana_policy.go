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
	manaModelClassic60PaladinReference
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

// Paladin joins the audited melee, spell and mana policies without importing
// a TBC class constructor. The first runtime slice is rear two-handed Command.
func classic60PaladinReferenceRules() rulesetProfile {
	rules := classic60MeleeReferenceRules()
	rules.id = rulesetClassic60PaladinReference
	rules.attributes.preserveFractionalStats = true
	rules.combat.outcomes.spellChanceModel = spellChanceModelClassicReference60
	rules.combat.outcomes.minimumSpellMissChance = 0.01
	rules.combat.outcomes.magicCritDamageMultiplier = 1.5
	rules.combat.outcomes.rangedCritDamageMultiplier = 2
	rules.resources.manaModel = manaModelClassic60PaladinReference
	return rules
}

func (model manaModel) classicReferenceClass() proto.Class {
	switch model {
	case manaModelClassic60MageReference:
		return proto.Class_ClassMage
	case manaModelClassic60PaladinReference:
		return proto.Class_ClassPaladin
	default:
		return proto.Class_ClassUnknown
	}
}

func (model manaModel) isClassicReference() bool {
	return model.classicReferenceClass() != proto.Class_ClassUnknown
}

func (character *Character) validateManaRules() {
	model := character.resolvedResourceRules().manaModel
	if model == manaModelInheritedTBC {
		return
	}
	expectedClass := model.classicReferenceClass()
	if expectedClass == proto.Class_ClassUnknown {
		panic("mana is unavailable for this ruleset")
	}
	if !character.rulesInitialized || character.Type != PlayerUnit || character.Level != 60 ||
		character.Race != proto.Race_RaceHuman || character.Class != expectedClass ||
		character.HasRageBar() || character.HasEnergyBar() || character.HasFocusBar() {
		panic("Classic mana reference requires a level-60 Human of the selected class")
	}
	character.classic60ManaClass = expectedClass
	character.manaRegenMultiplier = 1
}

func (unit *Unit) validateClassic60Mana() {
	expectedClass := unit.resolvedResourceRules().manaModel.classicReferenceClass()
	if !unit.rulesInitialized || expectedClass == proto.Class_ClassUnknown ||
		unit.Type != PlayerUnit || unit.Level != 60 || unit.manaBar.unit != unit || unit.classic60ManaClass != expectedClass ||
		unit.HasRageBar() || unit.HasEnergyBar() || unit.HasFocusBar() {
		panic("Classic mana reference requires a validated player mana bar")
	}
	castingRegen := 0.0
	if unit.foreverRet != nil {
		castingRegen = .1 * float64(unit.foreverRet.config.Talents["Reverence"])
	}
	if unit.PseudoStats.SpiritRegenMultiplier != 1 || unit.PseudoStats.SpiritRegenRateCasting != castingRegen ||
		unit.PseudoStats.ForceFullSpiritRegen || unit.manaRegenMultiplier != 1 {
		panic("Classic mana reference does not support regeneration modifiers")
	}
	for _, stat := range []stats.Stat{stats.Spirit, stats.MP5} {
		if value := unit.stats[stat]; math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			panic("Classic mana reference requires finite nonnegative Spirit and MP5")
		}
	}
}

func validateManaCostOptions(spell *Spell, options ManaCostOptions) {
	model := spell.Unit.resolvedResourceRules().manaModel
	if model == manaModelInheritedTBC {
		return
	}
	if !model.isClassicReference() {
		panic("mana costs are unavailable for this ruleset")
	}
	spell.Unit.validateClassic60Mana()
	flat := options.FlatCost > 0 && options.BaseCostPercent == 0
	percentage := model == manaModelClassic60PaladinReference && options.FlatCost == 0 &&
		options.BaseCostPercent > 0 && options.BaseCostPercent <= 100 &&
		!math.IsNaN(options.BaseCostPercent) && !math.IsInf(options.BaseCostPercent, 0)
	modifierAllowed := options.PercentModifier == 0 || options.PercentModifier == 1
	if model == manaModelClassic60PaladinReference && (spell.SpellID == 20920 || spell.SpellID == 20271 || spell.SpellID == 20308 || spell.SpellID == 20293 || spell.SpellID == 20349 || spell.SpellID == 20357 || spell.SpellID == 20164) {
		// The only currently supported reduction is Benediction on Command/Judgement.
		for rank := 1; rank <= 5; rank++ {
			modifierAllowed = modifierAllowed || options.PercentModifier == float64(100-3*rank)/100
		}
	}
	if (!flat && !percentage) || !modifierAllowed {
		panic("Classic mana reference requires an unmodified positive supported mana cost")
	}
}

// Preserve Classic's floating-point base cost in the mana implementation while
// retaining the inherited engine's integer cost field and arithmetic for TBC.
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell.go#L667-L684
func validateClassic60ManaCost(spell *Spell, cost *SpellCost) {
	spell.Unit.validateClassic60Mana()
	if cost == nil || cost.spell != spell || cost.FlatModifier != 0 ||
		cost.AdditivePercentModifier != 1 ||
		spell.Unit.PseudoStats.SpellCostPercentModifier != 100 {
		panic("Classic mana reference requires an unmodified registered mana cost")
	}
	mana, ok := cost.ResourceCostImpl.(*ManaCost)
	if !ok || mana == nil || mana.ResourceMetrics == nil || mana.classicBaseCost <= 0 ||
		math.IsNaN(mana.classicBaseCost) || math.IsInf(mana.classicBaseCost, 0) ||
		cost.BaseCost != int32(mana.classicBaseCost) || cost.PercentModifier != mana.classicCostMultiplier ||
		(spell.Unit.resolvedResourceRules().manaModel == manaModelClassic60MageReference && mana.classicCostMultiplier != 1) {
		panic("Classic mana reference requires a consistent mana cost with resource metrics")
	}
}

func validateClassicSpellResources(spell *Spell, table *AttackTable) {
	unit := spell.Unit
	if unit.HasRageBar() || unit.HasEnergyBar() || unit.HasFocusBar() {
		panic("Classic spell reference does not support rage, energy or focus")
	}
	model := table.resolvedResourceRules().manaModel
	if model.isClassicReference() {
		if model != unit.resolvedResourceRules().manaModel {
			panic("Classic spell reference requires the owner's resource policy")
		}
		unit.validateClassic60Mana()
		if spell.Cost != nil {
			validateClassic60ManaCost(spell, spell.Cost)
		}
	} else if unit.HasManaBar() || spell.Cost != nil {
		panic("Classic spell reference does not support resources")
	}
}
