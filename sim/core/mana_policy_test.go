package core

import (
	"math"
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func TestClassic60MageManaRegenSourceVectors(t *testing.T) {
	for _, test := range []struct {
		name                                    string
		spirit, intellect, mp5, spiritPerSecond float64
		castingTick, fullTick                   float64
	}{
		{"zero Spirit", 0, 20, 0, 6.25, 0, 12.5},
		{"Human baseline", 120, 125, 0, 21.25, 0, 42.5},
		{"Intellect does not boost Classic regen", 120, 400, 0, 21.25, 0, 42.5},
		{"fractional Spirit and MP5", 120.5, 125, 12.5, 21.3125, 5, 47.625},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, agent := newClassic60ManaTestSim(t, classic60ManaTestConfig{manual: true})
			agent.stats[stats.Spirit], agent.stats[stats.Intellect], agent.stats[stats.MP5] = test.spirit, test.intellect, test.mp5
			assertFloat64(t, "source Spirit regeneration", agent.SpiritManaRegenPerSecond(), test.spiritPerSecond)
			agent.UpdateManaRegenRates()
			assertFloat64(t, "casting tick", agent.manaTickWhileCasting, test.castingTick)
			assertFloat64(t, "full tick", agent.manaTickWhileNotCasting, test.fullTick)
		})
	}

	// Zero-value owners must keep the existing TBC fallback; resolving the
	// Classic profile must not install a global class-dependent callback.
	unit := &Unit{stats: stats.Stats{stats.Spirit: 100, stats.Intellect: 100}}
	assertFloat64(t, "legacy TBC Spirit/Intellect formula", unit.SpiritManaRegenPerSecond(), 9.328)
	if unit.resolvedResourceRules().manaModel != manaModelInheritedTBC {
		t.Fatal("Classic mana became the public resource model")
	}
}

func TestClassic60MageManaRejectsUnsupportedInitialization(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Character)
	}{
		{"resource-free spell profile", func(c *Character) { c.rules = classic60SpellReferenceRules() }},
		{"melee profile", func(c *Character) { c.rules = classic60MeleeReferenceRules() }},
		{"unknown mana model", func(c *Character) { c.rules.resources.manaModel = manaModel(255) }},
		{"level 70", func(c *Character) { c.Level = 70 }},
		{"Priest", func(c *Character) { c.Class = proto.Class_ClassPriest }},
		{"Gnome", func(c *Character) { c.Race = proto.Race_RaceGnome }},
		{"pet", func(c *Character) { c.Type = PetUnit }},
		{"rage", func(c *Character) { c.rageBar.unit = &c.Unit }},
		{"energy", func(c *Character) { c.energyBar.unit = &c.Unit }},
		{"focus", func(c *Character) { c.focusBar.unit = &c.Unit }},
	} {
		t.Run(test.name, func(t *testing.T) {
			character := &Character{Race: proto.Race_RaceHuman, Class: proto.Class_ClassMage,
				Unit: Unit{Type: PlayerUnit, Level: 60, rulesInitialized: true, rules: classic60MageManaReferenceRules()}}
			test.change(character)
			defer func() {
				if recover() == nil || character.HasManaBar() || character.classic60MageReference || len(character.Spellbook) != 0 {
					t.Fatal("unsupported mana initialization did not reject before mutation")
				}
			}()
			character.EnableManaBar()
		})
	}
}

func TestClassic60MageManaRejectsUnsupportedRuntime(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*classic60ManaTestAgent)
		check  func(*classic60ManaTestAgent)
	}{
		{"Spirit casting fraction", func(a *classic60ManaTestAgent) { a.PseudoStats.SpiritRegenRateCasting = .15 }, func(a *classic60ManaTestAgent) { a.UpdateManaRegenRates() }},
		{"Spirit multiplier", func(a *classic60ManaTestAgent) { a.PseudoStats.SpiritRegenMultiplier = 2 }, func(a *classic60ManaTestAgent) { a.UpdateManaRegenRates() }},
		{"full regen while casting", func(a *classic60ManaTestAgent) { a.PseudoStats.ForceFullSpiritRegen = true }, func(a *classic60ManaTestAgent) { a.UpdateManaRegenRates() }},
		{"regen speed", func(a *classic60ManaTestAgent) { a.manaRegenMultiplier = 2 }, func(a *classic60ManaTestAgent) { a.UpdateManaRegenRates() }},
		{"negative Spirit", func(a *classic60ManaTestAgent) { a.stats[stats.Spirit] = -1 }, func(a *classic60ManaTestAgent) { a.UpdateManaRegenRates() }},
		{"NaN MP5", func(a *classic60ManaTestAgent) { a.stats[stats.MP5] = math.NaN() }, func(a *classic60ManaTestAgent) { a.UpdateManaRegenRates() }},
		{"unvalidated mana bar", func(a *classic60ManaTestAgent) { a.classic60MageReference = false }, func(a *classic60ManaTestAgent) { a.SpiritManaRegenPerSecond() }},
		{"foreign mana bar", func(a *classic60ManaTestAgent) { a.manaBar.unit = &Unit{} }, func(a *classic60ManaTestAgent) { a.SpiritManaRegenPerSecond() }},
		{"foreign table resource policy", func(a *classic60ManaTestAgent) {
			a.AttackTables[0].resources = resourceRules{manaModel: manaModelUnavailable}
		}, func(a *classic60ManaTestAgent) { a.Spell.SpellChanceToMiss(a.AttackTables[0]) }},
		{"foreign owner resource policy", func(a *classic60ManaTestAgent) { a.rules.resources = resourceRules{} }, func(a *classic60ManaTestAgent) { a.Spell.SpellChanceToMiss(a.AttackTables[0]) }},
		{"flat cost modifier", func(a *classic60ManaTestAgent) { a.Spell.Cost.FlatModifier = -1 }, func(a *classic60ManaTestAgent) { a.Spell.Cost.GetCurrentCost() }},
		{"percent cost modifier", func(a *classic60ManaTestAgent) { a.Spell.Cost.PercentModifier = .9 }, func(a *classic60ManaTestAgent) { a.Spell.Cost.GetCurrentCost() }},
		{"additive cost modifier", func(a *classic60ManaTestAgent) { a.Spell.Cost.AdditivePercentModifier = .9 }, func(a *classic60ManaTestAgent) { a.Spell.Cost.GetCurrentCost() }},
		{"unit cost modifier", func(a *classic60ManaTestAgent) { a.PseudoStats.SpellCostPercentModifier = 90 }, func(a *classic60ManaTestAgent) { a.Spell.Cost.GetCurrentCost() }},
		{"non-mana cost", func(a *classic60ManaTestAgent) { a.Spell.Cost.ResourceCostImpl = &RageCost{} }, func(a *classic60ManaTestAgent) { a.Spell.Cost.GetCurrentCost() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, agent := newClassic60ManaTestSim(t, classic60ManaTestConfig{manual: true})
			test.change(agent)
			defer func() {
				if recover() == nil {
					t.Fatal("unsupported runtime modification did not fail explicitly")
				}
			}()
			test.check(agent)
		})
	}
	for _, cost := range []ManaCostOptions{
		{FlatCost: -1}, {BaseCostPercent: 10}, {FlatCost: 395, BaseCostPercent: 10},
		{FlatCost: 395, PercentModifier: .9}, {FlatCost: 395, PercentModifier: math.NaN()},
	} {
		func() {
			_, agent := newClassic60ManaTestSim(t, classic60ManaTestConfig{manual: true})
			defer func() {
				if recover() == nil {
					t.Errorf("unsupported mana cost accepted: %+v", cost)
				}
			}()
			newManaCost(agent.Spell, cost)
		}()
	}
}
