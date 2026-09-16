package core

import (
	"math"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

// Defensive percentages and bonus Defense skill are explicit Classic values,
// never TBC rating inputs. Equipment supplies only shield block value here;
// gear data and its effects remain a separate integration task.
type classic60PaladinDefenseState struct {
	character                     *Character
	defenseBonus, deflectionBonus float64
	shieldMultiplier              float64
	HolyShield, HolyShieldProc    *Spell
	RighteousFury                 *Spell
	HolyShieldAura, RedoubtAura   *Aura
	RighteousFuryAura             *Aura
}

func (state *classic60PaladinDefenseState) hasShield() bool {
	weapon, shield := state.character.MainHand(), state.character.OffHand()
	return weapon.HandType != proto.HandType_HandTypeTwoHand && shield.WeaponType == proto.WeaponType_WeaponTypeShield
}

// The cached Classic tooltip for Shield Specialization (20148–20150) says
// block AMOUNT, not block chance. Source unit.go keeps the Strength component
// outside the multiplier; do not reproduce the source's noted double-counting
// risk by also installing Strength -> BlockValue in the modern dependency graph.
func (state *classic60PaladinDefenseState) blockValue() float64 {
	return max(0, state.character.GetStat(stats.Strength)*.05-1) +
		state.character.GetStat(stats.BlockValue)*state.shieldMultiplier
}

type classic60PaladinIncomingView struct {
	miss, dodge, parry, block, crit, crush float64
	blockValue                             float64
}

// Source: Classic@7779ebbf79dc7f1341e6ab939b28a3402c9a730a
// sim/core/target.go (defender != Enemy), spell_outcome.go (enemy white table),
// paladin/paladin.go (Agility dodge / Strength block), constants.go (Defense).
// The source's +3 crushing chance depends on trained level, not bonus Defense.
func (state *classic60PaladinDefenseState) incomingView(attackerLevel int32, canCrush bool) classic60PaladinIncomingView {
	levelDelta := .002 * float64(60-attackerLevel)
	defenseChance := .0004 * state.defenseBonus
	view := classic60PaladinIncomingView{
		miss:  max(0, .05+levelDelta+defenseChance),
		dodge: max(0, (.7+.0506*state.character.GetStat(stats.Agility))/100+levelDelta+defenseChance),
		parry: max(0, .05+state.deflectionBonus+levelDelta+defenseChance),
		crit:  max(0, .05-levelDelta-defenseChance),
	}
	if state.hasShield() {
		view.block = max(0, .05+state.character.GetStat(stats.BlockPercent)+levelDelta+defenseChance)
		view.blockValue = state.blockValue()
	}
	if canCrush && attackerLevel == 63 {
		view.crush = .15
	}
	return view
}

// Validate the reverse, owned NPC->Paladin table before armor or outcomes are
// evaluated. This is intentionally distinct from the outgoing weapon view.
func validateClassic60PaladinIncoming(spell *Spell, table *AttackTable) *classic60PaladinDefenseState {
	if spell == nil || spell.Unit == nil || table == nil || !table.rulesInitialized ||
		table.Attacker != spell.Unit || table.Attacker.Type != EnemyUnit || table.Defender == nil ||
		table.Attacker.Level < 60 || table.Attacker.Level > 63 ||
		table.Defender.Type != PlayerUnit || table.Defender.Level != 60 ||
		table.resolvedResourceRules().manaModel != manaModelClassic60PaladinReference ||
		table.resolvedOutcomeRules().weaponSkillModel != weaponSkillModelClassicReference ||
		table.resolvedMitigationRules().armorModel != armorMitigationClassicReference60 {
		panic("Classic incoming melee requires an owned level-60 Paladin and level 60–63 NPC table")
	}
	state := table.Defender.classic60PaladinDefense
	index := table.Defender.UnitIndex
	if state == nil || state.character == nil || &state.character.Unit != table.Defender ||
		state.character.Class != proto.Class_ClassPaladin || state.character.Race != proto.Race_RaceHuman ||
		state.character.resolvedRuleset().id != rulesetClassic60PaladinReference ||
		index < 0 || int(index) >= len(spell.Unit.AttackTables) || spell.Unit.AttackTables[index] != table {
		panic("Classic incoming melee requires registered Paladin defenses and the attacker's actual table")
	}
	if spell.SpellSchool != SpellSchoolPhysical || spell.DefenseType != DefenseTypeMelee ||
		spell.ProcMask != ProcMaskMeleeMHAuto || spell.weaponAttackSource != WeaponAttackSourceMainHand ||
		spell.Unit.AutoAttacks.IsDualWielding || spell.Unit.PseudoStats.DisableDWMissPenalty ||
		spell.Flags.Matches(SpellFlagBinary|SpellFlagIgnoreResists|SpellFlagPureDot) ||
		spell.BonusCritPercent != 0 || spell.Unit.GetStat(stats.PhysicalHitPercent) != 0 ||
		spell.Unit.GetStat(stats.PhysicalCritPercent) != spell.Unit.resolvedRuleset().targetPhysicalCritPercent(spell.Unit.Level) ||
		table.Defender.PseudoStats.ReducedCritTakenPercent != 0 {
		panic("Classic incoming reference supports unmodified physical NPC main-hand white attacks")
	}
	for _, stat := range []stats.Stat{stats.DefenseRating, stats.DodgeRating, stats.ParryRating, stats.BlockRating, stats.ResilienceRating} {
		if table.Defender.GetStat(stat) != 0 {
			panic("Classic Paladin defenses use direct percentages and skill, not TBC ratings")
		}
	}
	for _, stat := range []stats.Stat{stats.Strength, stats.Agility, stats.BlockValue, stats.BlockPercent} {
		value := table.Defender.GetStat(stat)
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			panic("Classic Paladin defenses require finite nonnegative inputs")
		}
	}
	return state
}

// Called by the engine's enemy-white outcome entry point only for the private
// Paladin profile. The complete one-roll order is miss/dodge/parry/block/crit/
// crush/hit. Casting or being stunned removes dodge/parry/block, as in source.
func (spell *Spell) outcomeClassic60PaladinEnemyMelee(sim *Simulation, result *SpellResult, table *AttackTable, countHits bool) {
	state := validateClassic60PaladinIncoming(spell, table)
	if result.Target != table.Defender {
		panic("Classic incoming result must target the attack table's actual defender")
	}
	view := state.incomingView(spell.Unit.Level, spell.Unit.PseudoStats.CanCrush)
	if result.Target.Hardcast.Expires > sim.CurrentTime || result.Target.PseudoStats.Stunned {
		view.dodge, view.parry, view.block = 0, 0, 0
	}
	roll, cumulative := sim.RandomFloat("Enemy White Hit Table"), 0.0
	metrics := &spell.SpellMetrics[result.Target.UnitIndex]
	for _, entry := range []struct {
		chance  float64
		outcome HitOutcome
	}{
		{view.miss, OutcomeMiss}, {view.dodge, OutcomeDodge}, {view.parry, OutcomeParry},
		{view.block, OutcomeBlock}, {view.crit, OutcomeCrit}, {view.crush, OutcomeCrush},
		{1, OutcomeHit},
	} {
		cumulative += entry.chance
		if roll >= cumulative {
			continue
		}
		result.Outcome = entry.outcome
		switch entry.outcome {
		case OutcomeMiss:
			result.Damage = 0
			if countHits {
				metrics.Misses++
			}
		case OutcomeDodge:
			result.Damage = 0
			if countHits {
				metrics.Dodges++
			}
		case OutcomeParry:
			result.Damage = 0
			if countHits {
				metrics.Parries++
			}
		case OutcomeBlock:
			result.Damage = max(0, result.Damage-view.blockValue)
			if countHits {
				metrics.Blocks++
			}
		case OutcomeCrit:
			result.Damage *= 2
			if countHits {
				metrics.Crits++
			}
		case OutcomeCrush:
			result.Damage *= 1.5
			if countHits {
				metrics.Crushes++
			}
		case OutcomeHit:
			if countHits {
				metrics.Hits++
			}
		}
		return
	}
}

// registerDefense supplies a coherent shield/retaliation/threat slice using
// source paladin/{talents,holy_shield,righteous_fury}.go and cached tooltips.
// Full talent structural validation belongs to registerClassic60PaladinBuild.
func (spells *classic60PaladinSpells) registerDefense(character *Character, talents classic60PaladinTalents) *classic60PaladinDefenseState {
	if character == nil || character.Class != proto.Class_ClassPaladin || character.Level != 60 ||
		character.Race != proto.Race_RaceHuman || character.resolvedRuleset().id != rulesetClassic60PaladinReference ||
		character.Env == nil || character.Env.IsFinalized() || character.classic60PaladinDefense != nil {
		panic("Classic Paladin defenses require one initialized character before finalization")
	}
	for _, talent := range []classic60PaladinTalent{classic60PaladinAnticipation, classic60PaladinDeflection,
		classic60PaladinShieldSpecialization, classic60PaladinRedoubt, classic60PaladinHolyShield,
		classic60PaladinReckoning, classic60PaladinImprovedRighteousFury, classic60PaladinHolyPower} {
		if talents[talent] > classic60PaladinTalentDescriptors[talent].maxRank {
			panic("Classic Paladin defense talent rank exceeds its source maximum")
		}
	}
	state := &classic60PaladinDefenseState{character: character,
		defenseBonus:     2 * float64(talents[classic60PaladinAnticipation]),
		deflectionBonus:  .01 * float64(talents[classic60PaladinDeflection]),
		shieldMultiplier: 1 + .1*float64(talents[classic60PaladinShieldSpecialization]),
	}
	character.classic60PaladinDefense = state
	character.PseudoStats.CanParry = true
	// Finalize enables the engine's parry haste. Its remaining-time 20% floor
	// and 40% reduction agree with VMaNGOS@8f4e608 Unit.cpp, unlike the
	// Classic simulator's last-swing-based floor which can schedule backward.
	character.PseudoStats.CanBlock = state.hasShield()

	if talents[classic60PaladinRedoubt] != 0 {
		bonus := .06 * float64(talents[classic60PaladinRedoubt])
		state.RedoubtAura = character.RegisterAura(Aura{
			Label: "Classic Redoubt", ActionID: ActionID{SpellID: 20134}, Duration: 10 * time.Second, MaxStacks: 5,
			OnGain:   func(_ *Aura, sim *Simulation) { character.AddStatDynamic(sim, stats.BlockPercent, bonus) },
			OnExpire: func(_ *Aura, sim *Simulation) { character.AddStatDynamic(sim, stats.BlockPercent, -bonus) },
			OnSpellHitTaken: func(aura *Aura, sim *Simulation, _ *Spell, result *SpellResult) {
				if result.DidBlock() {
					aura.RemoveStack(sim)
				}
			},
		})
		MakePermanent(character.RegisterAura(Aura{
			Label: "Classic Redoubt critical trigger",
			OnSpellHitTaken: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
				if result.DidCrit() && spell.ProcMask.Matches(ProcMaskMeleeOrRanged) {
					state.RedoubtAura.Activate(sim)
					state.RedoubtAura.SetStacks(sim, 5)
				}
			},
		}))
	}

	if talents[classic60PaladinHolyShield] != 0 {
		state.HolyShieldProc = character.RegisterSpell(SpellConfig{
			ActionID: ActionID{SpellID: 20957}, Rank: 3,
			SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic, ProcMask: ProcMaskSpellDamage,
			WeaponAttackSource: WeaponAttackSourceNone,
			Flags:              SpellFlagPassiveSpell | SpellFlagNoOnCastComplete,
			Cast:               CastConfig{IgnoreHaste: true},
			DamageMultiplier:   1, ThreatMultiplier: 1.2, BonusCoefficient: .05,
			BonusCritPercent: float64(talents[classic60PaladinHolyPower]),
			ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
				spell.CalcAndDealDamage(sim, target, 130, spell.OutcomeMagicCrit)
			},
		})
		state.HolyShieldAura = character.RegisterAura(Aura{
			Label: "Classic Holy Shield (Rank 3)", ActionID: ActionID{SpellID: 20928}, Duration: 10 * time.Second, MaxStacks: 4,
			OnGain:   func(_ *Aura, sim *Simulation) { character.AddStatDynamic(sim, stats.BlockPercent, .3) },
			OnExpire: func(_ *Aura, sim *Simulation) { character.AddStatDynamic(sim, stats.BlockPercent, -.3) },
			OnSpellHitTaken: func(aura *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
				if result.DidBlock() {
					state.HolyShieldProc.Cast(sim, spell.Unit)
					aura.RemoveStack(sim)
				}
			},
		})
		state.HolyShield = character.RegisterSpell(SpellConfig{
			ActionID: state.HolyShieldAura.ActionID, Rank: 3,
			SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic, ProcMask: ProcMaskEmpty, Flags: SpellFlagAPL,
			ManaCost: ManaCostOptions{FlatCost: 240},
			Cast: CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault},
				CD: Cooldown{Timer: character.NewTimer(), Duration: 10 * time.Second}},
			ExtraCastCondition: func(_ *Simulation, _ *Unit) bool { return state.hasShield() },
			ApplyEffects: func(sim *Simulation, _ *Unit, _ *Spell) {
				state.HolyShieldAura.Activate(sim)
				state.HolyShieldAura.SetStacks(sim, 4)
			},
		})
	}

	if talents[classic60PaladinReckoning] != 0 {
		chance := .2 * float64(talents[classic60PaladinReckoning])
		MakePermanent(character.RegisterAura(Aura{
			Label: "Classic Reckoning critical trigger", ActionID: ActionID{SpellID: 20178},
			OnSpellHitTaken: func(_ *Aura, sim *Simulation, spell *Spell, result *SpellResult) {
				if !result.DidCrit() || !spell.ProcMask.Matches(ProcMaskMeleeOrRanged) || !sim.Proc(chance, "Classic Reckoning") {
					return
				}
				// The pinned ExtraMHAttack(1) advances the normal MH swing
				// timer; it does not add a TBC four-charge double-swing buff.
				// Only one incoming crit can resolve at each event in this slice.
				if character.AutoAttacks.mh.enabled {
					character.AutoAttacks.mh.swingAt = sim.CurrentTime
					sim.rescheduleWeaponAttack(sim.CurrentTime)
				}
			},
		}))
	}

	// Cached spell 25780: 30% base mana, 30 minutes. Improved Righteous Fury
	// improves the bonus threat (60%), not all threat: 1.6 -> 1.9 at rank 3.
	// This corrects the pinned simulator's 2.4 multiplier using its cached
	// talent tooltip and VMaNGOS@8f4e608450460efe1e38743e4da74397d4773a3a
	// SpellAuras.cpp HandleModThreat, which applies the modified aura amount
	// as a percentage increase to the initially 1.0 school threat modifier.
	rfMultiplier := 1 + .6*(1+[...]float64{0, .16, .33, .5}[talents[classic60PaladinImprovedRighteousFury]])
	state.RighteousFuryAura = character.RegisterAura(Aura{
		Label: "Classic Righteous Fury", ActionID: ActionID{SpellID: 25780}, Duration: 30 * time.Minute,
		OnGain: func(_ *Aura, _ *Simulation) {
			for _, spell := range character.Spellbook {
				if spell.SpellSchool == SpellSchoolHoly {
					spell.ThreatMultiplier *= rfMultiplier
				}
			}
		},
		OnExpire: func(_ *Aura, _ *Simulation) {
			for _, spell := range character.Spellbook {
				if spell.SpellSchool == SpellSchoolHoly {
					spell.ThreatMultiplier /= rfMultiplier
				}
			}
		},
	})
	state.RighteousFury = character.RegisterSpell(SpellConfig{
		ActionID:    state.RighteousFuryAura.ActionID,
		SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMagic, ProcMask: ProcMaskEmpty,
		Flags: SpellFlagAPL | SpellFlagHelpful, ManaCost: ManaCostOptions{BaseCostPercent: 30},
		Cast:               CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault}},
		ExtraCastCondition: func(_ *Simulation, target *Unit) bool { return target == &character.Unit },
		ApplyEffects:       func(sim *Simulation, _ *Unit, _ *Spell) { state.RighteousFuryAura.Activate(sim) },
	})
	return state
}
