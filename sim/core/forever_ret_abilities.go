package core

import (
	"time"

	"github.com/wowsims/tbc/sim/core/stats"
)

func foreverRetCost(spell *Spell, multiplier float64) {
	if spell == nil || spell.Cost == nil {
		return
	}
	spell.Cost.PercentModifier = multiplier
	spell.Cost.ResourceCostImpl.(*ManaCost).classicCostMultiplier = multiplier
}

func (a *foreverRetAgent) registerForeverAbilities() error {
	t := a.config.Talents
	// Only identical effects cross the schema boundary. Baseline abilities are
	// explicitly learned here; the Classic talent validator remains unchanged.
	base := classic60PaladinTalents{}
	for name, key := range map[string]classic60PaladinTalent{"Divine Strength": classic60PaladinDivineStrength, "Divine Intellect": classic60PaladinDivineIntellect, "Deflection": classic60PaladinDeflection, "Conviction": classic60PaladinConviction, "Improved Judgement": classic60PaladinImprovedJudgement, "Seal of Command": classic60PaladinSealOfCommand} {
		base[key] = uint8(t[name])
	}
	base[classic60PaladinConsecration] = 1
	base[classic60PaladinBlessingOfKings] = 1
	var err error
	a.spells, err = registerClassic60PaladinAbilities(&a.Character, base)
	if err != nil {
		return err
	}
	s := a.spells
	// The demo tooltips explicitly give these baseline blessings one hour.
	for _, aura := range []*Aura{s.MightAura, s.WisdomAura, s.KingsAura} {
		aura.Duration = time.Hour
	}
	a.AddStat(stats.PhysicalHitPercent, float64(t["Precision"]))
	a.AddStat(stats.SpellHitPercent, float64(t["Precision"]))
	champion := []float64{0, .33, .66, 1}[t["Champion of the Light"]]
	a.AddStatDependency(stats.Intellect, stats.SpellDamage, champion)
	a.AddStatDependency(stats.Intellect, stats.HealingPower, champion)
	weaponMult := 1 + .03*float64(t["Two-Handed Weapon Specialization"])
	a.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexPhysical] *= weaponMult
	crusade := .01 * float64(t["Crusade"])
	if classic60PaladinUndeadOrDemon(a.CurrentTarget) {
		crusade *= 2
	}
	a.PseudoStats.DamageDealtMultiplier *= 1 + crusade
	a.holyStrike = a.RegisterSpell(SpellConfig{
		// 17143 is a reporting identifier, not a claimed verified Forever rank ID.
		ActionID: ActionID{SpellID: 17143}, SpellSchool: SpellSchoolHoly, DefenseType: DefenseTypeMelee, ProcMask: ProcMaskMeleeMHSpecial, WeaponAttackSource: WeaponAttackSourceMainHand,
		Flags: SpellFlagAPL | SpellFlagMeleeMetrics, ManaCost: ManaCostOptions{FlatCost: int32(a.config.HolyStrikeMana)},
		Cast:             CastConfig{IgnoreHaste: true, DefaultCast: Cast{GCD: GCDDefault}, CD: Cooldown{Timer: a.NewTimer(), Duration: time.Duration(12-t["Improved Holy Strike"]) * time.Second}},
		DamageMultiplier: weaponMult * (1 + .1*float64(t["Sacred Arbiter"])), ThreatMultiplier: 1,
		ApplyEffects: func(sim *Simulation, target *Unit, spell *Spell) {
			damage := a.config.HolyStrikeWeaponPercent/100*a.MHWeaponDamage(sim, spell.MeleeAttackPower(target)) + sim.Roll(a.config.HolyStrikeMin, a.config.HolyStrikeMax)
			result := spell.CalcAndDealDamage(sim, target, damage, spell.OutcomeMeleeSpecialHitAndCrit)
			if result.Landed() && t["Sacred Arbiter"] > 0 && s.activeJudgement != nil && s.activeJudgement.IsActive() {
				s.activeJudgement.Activate(sim)
			}
		},
	})
	a.holyStrike.classic60PaladinAttack = foreverPaladinHolyStrike
	for _, spell := range []*Spell{s.CommandProc, s.CommandJudgement, s.RighteousnessProc, s.RighteousnessJudgement} {
		if spell != nil {
			spell.DamageMultiplier *= 1 + .05*float64(t["Improved Seals"])
		}
	}
	for _, spell := range []*Spell{s.CommandProc, s.CommandJudgement} {
		if spell != nil {
			spell.DamageMultiplier *= weaponMult
		}
	}
	s.HammerOfWrath.DefaultCast.CastTime -= time.Duration(t["Instrument of Law"]) * 500 * time.Millisecond
	for _, spell := range []*Spell{s.Exorcism, s.HolyWrath} {
		spell.CD.Duration = time.Duration(float64(spell.CD.Duration) * (1 - []float64{0, .165, .33}[t["Purifying Power"]]))
	}
	for _, spell := range a.Spellbook {
		if spell.Cost == nil {
			continue
		}
		mult := 1.0
		if spell.DefaultCast.CastTime == 0 {
			mult *= 1 - .02*float64(t["Benediction"])
		}
		if spell == s.Consecration || spell == s.Exorcism || spell == s.HolyWrath || spell == s.HammerOfWrath {
			mult *= 1 - .2*float64(t["Holy Conduit"])
		}
		foreverRetCost(spell, mult)
	}
	refund := a.NewManaMetrics(ActionID{SpellID: 31878})
	s.Judgement.ApplyEffects = func(sim *Simulation, target *Unit, _ *Spell) {
		// Keep the seal active on both hits and misses; resolve the original payload.
		seal := a.GetSpell(s.currentSeal.ActionID)
		s.currentJudgement.Cast(sim, target)
		rank := t["Sanctified Judgement"]
		if rank > 0 && seal != nil && sim.Proc([]float64{0, .33, .66, 1}[rank], "Sanctified Judgement") {
			a.AddMana(sim, seal.Cost.GetCurrentCost()*.2*float64(rank), refund)
		}
	}
	if t["Swift Judgement"] > 0 {
		normal := s.Judgement.Cost.PercentModifier
		free := a.RegisterAura(Aura{Label: "Forever Swift Judgement", Duration: NeverExpires, OnGain: func(*Aura, *Simulation) { foreverRetCost(s.Judgement, 0) }, OnExpire: func(*Aura, *Simulation) { foreverRetCost(s.Judgement, normal) }})
		effect := s.Judgement.ApplyEffects
		s.Judgement.ApplyEffects = func(sim *Simulation, target *Unit, spell *Spell) { effect(sim, target, spell); free.Deactivate(sim) }
		a.swift = a.RegisterSpell(SpellConfig{ActionID: ActionID{SpellID: 900001}, SpellSchool: SpellSchoolHoly, ProcMask: ProcMaskEmpty, Flags: SpellFlagAPL, Cast: CastConfig{IgnoreHaste: true, DefaultCast: Cast{NonEmpty: true}, CD: Cooldown{Timer: a.NewTimer(), Duration: time.Minute}}, ApplyEffects: func(sim *Simulation, _ *Unit, _ *Spell) { s.Judgement.CD.Reset(); free.Activate(sim) }})
	}
	if rank := t["Vengeance"]; rank > 0 {
		s.VengeanceAura = a.RegisterAura(Aura{Label: "Forever Vengeance", ActionID: ActionID{SpellID: 20059}, Duration: 30 * time.Second, MaxStacks: 5,
			OnStacksChange: func(_ *Aura, _ *Simulation, old, new int32) {
				m := (1 + .01*float64(rank)*float64(new)) / (1 + .01*float64(rank)*float64(old))
				a.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexPhysical] *= m
				a.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly] *= m
			}})
		MakePermanent(a.RegisterAura(Aura{Label: "Forever Vengeance trigger", OnSpellHitDealt: func(_ *Aura, sim *Simulation, _ *Spell, r *SpellResult) {
			if r.DidCrit() {
				s.VengeanceAura.Activate(sim)
				s.VengeanceAura.AddStack(sim)
			}
		}}))
	}
	if rank := t["Vindication"]; rank > 0 {
		dep := a.NewDynamicMultiplyStat(stats.AttackPower, 1+.01*float64(rank))
		buff := a.RegisterAura(Aura{Label: "Forever Vindication", ActionID: ActionID{SpellID: 9452}, Duration: 30 * time.Second, OnGain: func(_ *Aura, sim *Simulation) { a.EnableDynamicStatDep(sim, dep) }, OnExpire: func(_ *Aura, sim *Simulation) { a.DisableDynamicStatDep(sim, dep) }})
		ppm := a.NewStaticLegacyPPMManager(a.config.VindicationPPM, ProcMaskMelee)
		MakePermanent(a.RegisterAura(Aura{Label: "Forever Vindication trigger", OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, r *SpellResult) {
			if r.Landed() && (spell == a.AutoAttacks.MHAuto() || spell == a.holyStrike) && ppm.Proc(sim, spell.ProcMask, "Forever Vindication") {
				buff.Activate(sim)
			}
		}}))
	}
	if t["Twist of Light"] > 0 {
		for _, seal := range []*Spell{s.SealOfCommand, s.SealOfRighteousness, s.SealOfTheCrusader} {
			if seal == nil {
				continue
			}
			effect := seal.ApplyEffects
			spell := seal
			seal.ApplyEffects = func(sim *Simulation, target *Unit, cast *Spell) {
				old := s.currentSeal
				if old != nil && old.IsActive() && old.ActionID != spell.ActionID {
					switch old {
					case s.SealAura:
						a.echo = s.CommandProc
					case s.RighteousnessAura:
						a.echo = s.RighteousnessProc
					}
				}
				effect(sim, target, cast)
				a.twistReady = false
			}
		}
	}
	MakePermanent(a.RegisterAura(Aura{Label: "Forever seal echo trigger", OnSpellHitDealt: func(_ *Aura, sim *Simulation, spell *Spell, r *SpellResult) {
		if !r.Landed() || (spell != a.AutoAttacks.MHAuto() && spell != a.holyStrike) {
			return
		}
		if spell == a.AutoAttacks.MHAuto() {
			a.twistReady = true
		}
		if a.echo != nil {
			echo := a.echo
			a.echo = nil
			echo.Cast(sim, r.Target)
		}
	}}))
	return nil
}

func (a *foreverRetAgent) ExecuteCustomRotation(sim *Simulation) {
	s := a.spells
	target := a.CurrentTarget
	cast := func(spell *Spell) bool { return spell != nil && spell.CanCast(sim, target) && spell.Cast(sim, target) }
	if !a.openerDone {
		if s.currentSeal != s.CrusaderAura {
			if cast(s.SealOfTheCrusader) {
				return
			}
		} else if cast(s.Judgement) {
			a.openerDone = true
		}
	}
	if a.openerDone {
		desired := s.SealOfRighteousness
		desiredAura := s.RighteousnessAura
		if a.config.Seal == "command" {
			desired = s.SealOfCommand
			desiredAura = s.SealAura
		}
		if a.config.Twist && s.currentSeal != nil && s.currentSeal != s.CrusaderAura {
			desired = nil
			if a.twistReady && a.echo == nil {
				if s.currentSeal == s.SealAura {
					desired = s.SealOfRighteousness
				} else {
					desired = s.SealOfCommand
				}
			}
		}
		if (s.currentSeal == nil || s.currentSeal == s.CrusaderAura || (!a.config.Twist && s.currentSeal != desiredAura) || a.config.Twist && a.twistReady && a.echo == nil) && cast(desired) {
			return
		}
		for _, key := range a.config.Priority {
			var spell *Spell
			switch key {
			case "holyStrike":
				spell = a.holyStrike
			case "judgement":
				spell = s.Judgement
			case "hammer":
				spell = s.HammerOfWrath
			case "exorcism":
				spell = s.Exorcism
			case "holyWrath":
				if classic60PaladinUndeadOrDemon(target) {
					spell = s.HolyWrath
				}
			case "consecration":
				if a.CurrentManaPercent()*100 >= a.config.ConsecrationManaPercent {
					spell = s.Consecration
				}
			case "swiftJudgement":
				if !s.Judgement.IsReady(sim) && s.currentSeal != nil {
					spell = a.swift
				}
			}
			if cast(spell) {
				if spell.DefaultCast.GCD > 0 || spell.DefaultCast.CastTime > 0 {
					return
				}
			}
		}
	}
	a.WaitUntil(sim, sim.CurrentTime+100*time.Millisecond)
}
