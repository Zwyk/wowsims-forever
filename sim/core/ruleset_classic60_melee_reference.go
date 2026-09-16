package core

// classic60MeleeReferenceRules joins verified components for a bounded engine
// integration test. It is NOT a complete Classic ruleset or an active build
// target: only pre-racial Human Warrior rear melee attacks against a passive
// level 60–63 enemy are exercised, including both hands and weapon specials.
// There are no class spells, resources, haste, incoming attacks, talents, item
// effects or public selection path.
//
// Start empty rather than inheriting unverified TBC behavior. Weapon-dependent
// physical chances are resolved at attack time, not from the empty table seeds.
// Other combat/rating fields intentionally remain absent; adding a capability
// requires its own policy and executable tests. Source caveats, including the
// pinned Classic crit-suppression approximation, are in docs/forever_core.md.
func classic60MeleeReferenceRules() rulesetProfile {
	rules := rulesetProfile{
		id:     rulesetClassic60MeleeReference,
		levels: levelRules{characterLevel: 60, defaultBossLevelDelta: 3},
		combat: combatRules{outcomes: outcomeRules{
			weaponSkillModel:          weaponSkillModelClassicReference,
			dualWieldMissPenalty:      0.19,
			meleeCritDamageMultiplier: 2,
		}},
		mitigation: mitigationRules{
			armorModel:      armorMitigationClassicReference60,
			resistanceModel: resistanceMitigationClassicReference60,
		},
		resources: resourceRules{manaModel: manaModelUnavailable},
	}
	var ok bool
	rules.attributes, ok = classicReferenceLevel60AttributeRules(rules)
	if !ok {
		panic("invalid Classic 60 melee reference attributes")
	}
	rules.ratings, ok = classicReferenceLevel60OffensiveChanceOverlay(rules)
	if !ok {
		panic("invalid Classic 60 melee reference offensive chances")
	}
	return rules
}
