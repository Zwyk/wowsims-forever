package core

// rulesetID is deliberately internal. Forever is a build-time engine target,
// not a per-simulation option exposed through the public proto API.
type rulesetID uint8

const (
	rulesetInheritedTBC rulesetID = iota
	// Internal, deliberately incomplete profile used by scheduled melee tests.
	rulesetClassic60MeleeReference
	// Separate, resource-free caster diagnostic; never publicly selected.
	rulesetClassic60SpellReference
	// Bounded Human Mage mana diagnostic; never publicly selected.
	rulesetClassic60MageManaReference
	rulesetClassic60PaladinReference
)

// Keep the compile-time constants beside the active selector. Existing code
// still relies on CharacterLevel and DefaultBossLevel being constants.
const (
	activeRulesetID             = rulesetInheritedTBC
	activeCharacterLevel        = inheritedTBCCharacterLevel
	activeDefaultBossLevelDelta = inheritedTBCDefaultBossLevelDelta

	activeExpertisePerQuarterPercentReduction     = inheritedTBCExpertisePerQuarterPercentReduction
	activeDefenseRatingPerDefenseLevel            = inheritedTBCDefenseRatingPerDefenseLevel
	activeDodgeRatingPerDodgePercent              = inheritedTBCDodgeRatingPerDodgePercent
	activeParryRatingPerParryPercent              = inheritedTBCParryRatingPerParryPercent
	activeBlockRatingPerBlockPercent              = inheritedTBCBlockRatingPerBlockPercent
	activePhysicalHitRatingPerHitPercent          = inheritedTBCPhysicalHitRatingPerHitPercent
	activeSpellHitRatingPerHitPercent             = inheritedTBCSpellHitRatingPerHitPercent
	activePhysicalCritRatingPerCritPercent        = inheritedTBCPhysicalCritRatingPerCritPercent
	activeSpellCritRatingPerCritPercent           = inheritedTBCSpellCritRatingPerCritPercent
	activePhysicalHasteRatingPerHastePercent      = inheritedTBCPhysicalHasteRatingPerHastePercent
	activeSpellHasteRatingPerHastePercent         = inheritedTBCSpellHasteRatingPerHastePercent
	activeDefenseChancePerDefenseLevelPercent     = inheritedTBCDefenseChancePerDefenseLevelPercent
	activeResilienceRatingPerCritReductionPercent = inheritedTBCResilienceRatingPerCritReductionPercent
)

type rulesetProfile struct {
	id         rulesetID
	levels     levelRules
	ratings    ratingRules
	combat     combatRules
	attributes attributeRules
	mitigation mitigationRules
	resources  resourceRules
}

type levelRules struct {
	characterLevel        int32
	defaultBossLevelDelta int32
}

func (levels levelRules) defaultBossLevel() int32 {
	return levels.characterLevel + levels.defaultBossLevelDelta
}

type levelBand uint8

const (
	levelBandCharacterMinusTwo levelBand = iota
	levelBandCharacter
	levelBandCharacterPlusOne
	levelBandCharacterPlusTwo
	levelBandCharacterPlusThree
	levelBandCount
)

func (levels levelRules) bandFor(unitLevel int32) levelBand {
	switch unitLevel {
	case levels.characterLevel - 2:
		return levelBandCharacterMinusTwo
	case levels.characterLevel:
		return levelBandCharacter
	case levels.characterLevel + 1:
		return levelBandCharacterPlusOne
	case levels.characterLevel + 2:
		return levelBandCharacterPlusTwo
	case levels.characterLevel + 3:
		return levelBandCharacterPlusThree
	default:
		// UnitLevelFloat64 historically sends every unsupported level to the
		// +3 row. Keep that compatibility behavior in the resolver rather than
		// representing fallback as a separate piece of canonical rules data.
		return levelBandCharacterPlusThree
	}
}

func (levels levelRules) float64ByLevel(unitLevel int32, characterMinusTwo float64, character float64, characterPlusOne float64, characterPlusTwo float64, characterPlusThree float64) float64 {
	switch levels.bandFor(unitLevel) {
	case levelBandCharacterMinusTwo:
		return characterMinusTwo
	case levelBandCharacter:
		return character
	case levelBandCharacterPlusOne:
		return characterPlusOne
	case levelBandCharacterPlusTwo:
		return characterPlusTwo
	case levelBandCharacterPlusThree:
		return characterPlusThree
	default:
		panic("unsupported level band")
	}
}

// ratingRules keeps conversion values separate even where Forever exposes a
// shared item stat. Shared Hit and Crit sources still feed distinct physical
// and spell outcome channels, whose conversion values are not yet confirmed.
type ratingRules struct {
	expertisePerQuarterPercentReduction     float64
	defenseRatingPerDefenseLevel            float64
	dodgeRatingPerDodgePercent              float64
	parryRatingPerParryPercent              float64
	blockRatingPerBlockPercent              float64
	physicalHitRatingPerHitPercent          float64
	spellHitRatingPerHitPercent             float64
	physicalCritRatingPerCritPercent        float64
	spellCritRatingPerCritPercent           float64
	physicalHasteRatingPerHastePercent      float64
	spellHasteRatingPerHastePercent         float64
	defenseChancePerDefenseLevelPercent     float64
	resilienceRatingPerCritReductionPercent float64
}

// outcomeRules contains fixed values consumed while resolving an attack. A
// value copy is cached on each AttackTable so an injected profile cannot mix
// its table seeds with values from the active build-time profile.
type outcomeRules struct {
	weaponSkillModel                   weaponSkillModel
	spellChanceModel                   spellChanceModel
	expertiseAvoidanceStepsPerUnit     float64
	dualWieldMissPenalty               float64
	minimumSpellMissChance             float64
	magicCritDamageMultiplier          float64
	meleeCritDamageMultiplier          float64
	rangedCritDamageMultiplier         float64
	enemyCritDamageMultiplier          float64
	resilienceCritDamageReductionScale float64
	crushingBlowDamageMultiplier       float64
}

// attackTableBase contains only immutable seed values. All chances and
// suppressions are fractions, while glanceMultiplier is a damage multiplier.
// Runtime state, pointers, maps, and dynamic modifiers remain on AttackTable.
type attackTableBase struct {
	baseMissChance       float64
	baseSpellMissChance  float64
	baseBlockChance      float64
	baseDodgeChance      float64
	baseParryChance      float64
	baseGlanceChance     float64
	baseCrushChance      float64
	glanceMultiplier     float64
	meleeCritSuppression float64
	spellCritSuppression float64
	hitSuppression       float64
}

type attackTableByLevel [levelBandCount]attackTableBase

type combatRules struct {
	// These names describe the existing discriminator, not an assumption about
	// attacker type. NewAttackTable branches only on defender.Type.
	versusEnemy               attackTableByLevel
	versusNonEnemy            attackTableByLevel
	targetPhysicalCritByLevel [levelBandCount]float64
	outcomes                  outcomeRules
}

func (rules rulesetProfile) attackTableBase(attacker *Unit, defender *Unit) attackTableBase {
	if defender.Type == EnemyUnit {
		return rules.combat.versusEnemy[rules.levels.bandFor(defender.Level)]
	}

	return rules.combat.versusNonEnemy[rules.levels.bandFor(attacker.Level)]
}

func (rules rulesetProfile) targetPhysicalCritPercent(level int32) float64 {
	return rules.combat.targetPhysicalCritByLevel[rules.levels.bandFor(level)]
}

func (base attackTableBase) applyTo(table *AttackTable) {
	table.BaseMissChance = base.baseMissChance
	table.BaseSpellMissChance = base.baseSpellMissChance
	table.BaseBlockChance = base.baseBlockChance
	table.BaseDodgeChance = base.baseDodgeChance
	table.BaseParryChance = base.baseParryChance
	table.BaseGlanceChance = base.baseGlanceChance
	table.BaseCrushChance = base.baseCrushChance
	table.GlanceMultiplier = base.glanceMultiplier
	table.MeleeCritSuppression = base.meleeCritSuppression
	table.SpellCritSuppression = base.spellCritSuppression
	table.HitSuppression = base.hitSuppression
}

// currentRuleset returns a value copy, so callers cannot share mutable rules
// data. Adding Forever later means changing this compile-time selector and the
// compatibility constants above in the same reviewed change.
func currentRuleset() rulesetProfile {
	switch activeRulesetID {
	case rulesetInheritedTBC:
		return inheritedTBCRuleset()
	default:
		panic("unsupported active ruleset")
	}
}
