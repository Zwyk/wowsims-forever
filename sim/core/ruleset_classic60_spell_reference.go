package core

// classic60SpellReferenceRules is a separate, deliberately incomplete internal
// caster diagnostic. It supports resource-free player spells against a passive
// level 60–63 enemy; it does not activate a class, mana model, haste, talents,
// racials, healing, weapon attacks, or a public simulation option.
//
// Begin empty, then assemble only characterized Classic components. Spell
// chances are resolved from the validated context, never TBC table seeds.
func classic60SpellReferenceRules() rulesetProfile {
	rules := rulesetProfile{
		id:     rulesetClassic60SpellReference,
		levels: levelRules{characterLevel: 60, defaultBossLevelDelta: 3},
		combat: combatRules{outcomes: outcomeRules{
			weaponSkillModel:          weaponSkillModelUnavailable,
			spellChanceModel:          spellChanceModelClassicReference60,
			minimumSpellMissChance:    0.01,
			magicCritDamageMultiplier: 1.5,
		}},
		mitigation: mitigationRules{resistanceModel: resistanceMitigationClassicReference60},
		resources:  resourceRules{manaModel: manaModelUnavailable},
	}
	var ok bool
	rules.attributes, ok = classicReferenceLevel60AttributeRules(rules)
	if !ok {
		panic("invalid Classic 60 spell reference attributes")
	}
	rules.ratings, ok = classicReferenceLevel60OffensiveChanceOverlay(rules)
	if !ok {
		panic("invalid Classic 60 spell reference offensive chances")
	}
	return rules
}
