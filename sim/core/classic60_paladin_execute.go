package core

import (
	"math"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

// Hammer's cast, ranged outcome, coefficient and Precision exclusion follow:
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/paladin/hammer_of_wrath.go
// That implementation's rank-3 maximum is 566, but its own checked-in Classic
// tooltip for spell 24239 specifies 504–556 at level 60. Use the tooltip's 556
// endpoint explicitly, rather than silently preserving the conflicting number:
// https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/assets/db_inputs/wowhead_spell_tooltips.csv
func (spells *classic60PaladinSpells) registerHammerOfWrath(character *Character, talents classic60PaladinTalents) {
	if character == nil || character.resolvedRuleset().id != rulesetClassic60PaladinReference ||
		character.Class != proto.Class_ClassPaladin || character.Level != 60 || !character.HasManaBar() {
		panic("Classic Hammer of Wrath requires the internal level-60 Paladin reference")
	}
	spells.HammerOfWrath = character.RegisterSpell(SpellConfig{
		ActionID: ActionID{SpellID: 24239}, Rank: 3,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeRanged,
		ProcMask: ProcMaskRangedSpecial, WeaponAttackSource: WeaponAttackSourceNone,
		Flags:    SpellFlagMeleeMetrics | SpellFlagAPL,
		ManaCost: ManaCostOptions{FlatCost: 425},
		Cast: CastConfig{
			IgnoreHaste: true, DefaultCast: Cast{GCD: time.Second, CastTime: time.Second},
			CD: Cooldown{Timer: character.NewTimer(), Duration: 6 * time.Second},
		},
		DamageMultiplier: 1, ThreatMultiplier: 1, BonusCoefficient: .429,
		// Classic's shared physical-hit stat includes Precision; this ranged
		// spell explicitly removes it. Holy Power likewise grants no crit.
		BonusHitPercent:    -float64(talents[classic60PaladinPrecision]),
		MaxRange:           30,
		ExtraCastCondition: func(sim *Simulation, _ *Unit) bool { return sim.IsExecutePhase20() },
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			spell.CalcAndDealDamage(sim, target, sim.Roll(504, 556), spell.OutcomeRangedHitAndCrit)
		},
	})
	spells.HammerOfWrath.classic60PaladinAttack = classic60PaladinHammerOfWrath
}

// The pinned ranged cast uses the Paladin's ranged weapon slot, which is nil,
// so melee weapon skill bonuses cannot improve Hammer's hit chance. Reuse the
// audited level-dependent physical table with zero weapon skill bonus, while
// retaining the separate ranged hit/crit outcome (no dodge or glancing).
func liveClassic60PaladinRangedView(spell *Spell, table *AttackTable) physicalAttackTableView {
	if spell == nil || spell.Unit == nil || table == nil || !table.rulesInitialized ||
		table.Attacker != spell.Unit || table.Defender == nil ||
		spell.Unit.Type != PlayerUnit || spell.Unit.Level != 60 ||
		table.Defender.Type != EnemyUnit || table.Defender.Level < 60 || table.Defender.Level > 63 ||
		spell.Unit.resolvedRuleset().id != rulesetClassic60PaladinReference ||
		spell.classic60PaladinAttack != classic60PaladinHammerOfWrath || spell.SpellID != 24239 ||
		spell.DefenseType != DefenseTypeRanged || spell.SpellSchool != SpellSchoolHoly || spell.SchoolIndex != stats.SchoolIndexHoly ||
		spell.ProcMask != ProcMaskRangedSpecial || spell.weaponAttackSource != WeaponAttackSourceNone || !spell.IgnoreHaste ||
		spell.Flags.Matches(SpellFlagBinary|SpellFlagIgnoreResists|SpellFlagPureDot) {
		panic("Classic Paladin ranged reference requires the registered Holy Hammer of Wrath")
	}
	character := spell.weaponAttackCharacter()
	if character == nil || &character.Unit != spell.Unit || character.Class != proto.Class_ClassPaladin ||
		character.Race != proto.Race_RaceHuman || character.AutoAttacks.MHAuto() == nil {
		panic("Classic Paladin ranged reference requires its initialized Human Paladin")
	}
	// Validate the same owned table, rear encounter, resources, weapon setup
	// and unsupported defensive/haste modifiers as the ordinary white swing.
	// Do not retain that swing's weapon-dependent chances for the hammer.
	livePhysicalAttackTableView(character.AutoAttacks.MHAuto(), table)
	validateClassicSpellResources(spell, table)
	if spell.Unit.stats[stats.RangedHitPercent] != 0 || spell.Unit.stats[stats.RangedCritPercent] != 0 ||
		spell.BonusExpertiseRating != 0 || spell.CritMultiplierPct != 1 || spell.CritMultiplierAdditive != 0 {
		panic("Classic Hammer of Wrath does not support ranged-only or crit multiplier modifiers")
	}
	for _, value := range []float64{spell.BonusHitPercent, spell.BonusCritPercent} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			panic("Classic Hammer of Wrath requires finite hit and crit bonuses")
		}
	}
	view, ok := classicReferencePhysicalAttackTable(spell.Unit.Level, table.Defender.Level, 0)
	if !ok {
		panic("unsupported Classic Hammer of Wrath level context")
	}
	return view
}
