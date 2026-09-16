package core

import (
	"encoding/json"
	"github.com/wowsims/tbc/sim/core/stats"
	"math"
	"testing"
	"time"
)

func TestForeverRetPublicSimulation(t *testing.T) {
	input := `{"iterations":3,"duration":60}`
	first := ForeverRetJSON(input)
	var result struct {
		Error   string
		DPS     float64
		Actions []struct {
			Name string
			DPS  float64
		}
	}
	if err := json.Unmarshal([]byte(first), &result); err != nil {
		t.Fatal(err)
	}
	if result.Error != "" || result.DPS <= 0 {
		t.Fatalf("simulation: %s", first)
	}
	if second := ForeverRetJSON(input); second != first {
		t.Fatalf("identical seeds did not reproduce results\n%s\n%s", first, second)
	}
	total := 0.0
	strike := false
	for _, a := range result.Actions {
		total += a.DPS
		strike = strike || a.Name == "Holy Strike" && a.DPS > 0
	}
	if !strike || math.Abs(total-result.DPS) > 1e-7 {
		t.Fatalf("inconsistent damage report: %s", first)
	}
	t.Logf("DPS %.2f", result.DPS)
}

func TestForeverRetValidation(t *testing.T) {
	for _, input := range []string{`null`, `{} {}`, `{"iterations":0}`, `{"duration":601}`, `{"gear":[]}`, `{"talents":{}}`, `{"talents":{"Twist of Light":1}}`, `{"priority":["holyStrike","holyStrike"]}`, `{"holyStrikeMin":40,"holyStrikeMax":10}`} {
		var result map[string]any
		_ = json.Unmarshal([]byte(ForeverRetJSON(input)), &result)
		if result["error"] == nil {
			t.Errorf("accepted invalid request: %s", input)
		}
	}
}

func TestForeverRetScenarios(t *testing.T) {
	for _, input := range []string{`{"iterations":2,"duration":90,"twist":true}`, `{"iterations":2,"duration":90,"mobType":"undead","crusaderOpener":true,"priority":["hammer","holyStrike","judgement","exorcism","holyWrath","consecration"]}`, `{"iterations":2,"duration":60,"seal":"righteousness","talents":{},"priority":[]}`, `{"iterations":2,"duration":60,"blessing":"kings"}`} {
		var result map[string]any
		_ = json.Unmarshal([]byte(ForeverRetJSON(input)), &result)
		if result["error"] != nil {
			t.Errorf("scenario %s: %s", input, result["error"])
		}
	}
}

func TestForeverRetTalentsAndSealMechanics(t *testing.T) {
	c := defaultForeverRetConfig()
	c.Iterations = 1
	sim, a, err := newForeverRetSimulation(c)
	if err != nil {
		t.Fatal(err)
	}
	sim.reset()
	sim.CurrentTime = 0
	s := a.spells
	assertFloat64(t, "instant costs use Forever Benediction", a.holyStrike.Cost.GetCurrentCost(), 14.4)
	assertFloat64(t, "Conduit stacks multiplicatively", s.Consecration.Cost.GetCurrentCost(), 565*.9*.6)
	if s.HammerOfWrath.DefaultCast.CastTime != 0 {
		t.Fatal("Instrument of Law did not make Hammer instant")
	}
	if !s.SealOfCommand.Cast(sim, a.CurrentTarget) {
		t.Fatal("seal cast failed")
	}
	before := a.CurrentMana()
	if !s.Judgement.Cast(sim, a.CurrentTarget) {
		t.Fatal("Judgement failed")
	}
	if !s.SealAura.IsActive() || s.currentSeal != s.SealAura {
		t.Fatal("Forever Judgement consumed its seal")
	}
	assertFloat64(t, "Sanctified Judgement refunds modified seal cost", a.CurrentMana(), before-90.72*.9+210*.9*.6)
	sim.CurrentTime = 2 * time.Second
	if !s.SealOfRighteousness.Cast(sim, a.CurrentTarget) || a.echo != s.CommandProc {
		t.Fatal("Twist did not bank the replaced seal")
	}
	// A seal proc must not consume the Echo. The next eligible melee attack does.
	s.RighteousnessProc.Cast(sim, a.CurrentTarget)
	if a.echo != s.CommandProc {
		t.Fatal("Echo recursively consumed by a seal proc")
	}
	trigger := a.GetAura("Forever seal echo trigger")
	trigger.OnSpellHitDealt(trigger, sim, a.holyStrike, &SpellResult{Target: a.CurrentTarget, Outcome: OutcomeHit})
	if a.echo != nil {
		t.Fatal("landed Holy Strike failed to consume Echo")
	}
	for i := 0; i < 5; i++ {
		s.VengeanceAura.Activate(sim)
		s.VengeanceAura.AddStack(sim)
	}
	assertFloat64(t, "Forever Vengeance stacks to 15% at rank 3", a.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly], 1.15)
	sim.Cleanup()
	sim.reset()
	if a.echo != nil || s.VengeanceAura.GetStacks() != 0 {
		t.Fatal("procs leaked across iterations")
	}
	assertFloat64(t, "Vengeance multiplier resets", a.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly], 1)
}

func TestForeverRetSwiftJudgementAndSharedStats(t *testing.T) {
	c := defaultForeverRetConfig()
	c.Iterations = 1
	c.Seal = "righteousness"
	c.Priority = []string{"judgement", "swiftJudgement", "holyStrike"}
	c.Talents = map[string]int{"Toughness": 5, "Redoubt": 5, "Precision": 3, "Guardian's Favor": 1, "Improved Seal of Fury": 1, "Swift Judgement": 1}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	sim, a, err := newForeverRetSimulation(c)
	if err != nil {
		t.Fatal(err)
	}
	sim.reset()
	sim.CurrentTime = 0
	assertFloat64(t, "Precision shared melee hit", a.GetStat(stats.PhysicalHitPercent), 12)
	assertFloat64(t, "Precision shared spell hit", a.GetStat(stats.SpellHitPercent), 12)
	a.spells.SealOfRighteousness.Cast(sim, a.CurrentTarget)
	a.spells.Judgement.Cast(sim, a.CurrentTarget)
	if !a.swift.Cast(sim, a.CurrentTarget) || !a.spells.Judgement.IsReady(sim) {
		t.Fatal("Swift did not reset Judgement")
	}
	before := a.CurrentMana()
	a.spells.Judgement.Cast(sim, a.CurrentTarget)
	assertFloat64(t, "next judgement is free", a.CurrentMana(), before)
	assertFloat64(t, "following judgement restores normal cost", a.spells.Judgement.Cost.GetCurrentCost(), 90.72)
	sim2, _, err := newForeverRetSimulation(c)
	if err != nil {
		t.Fatal(err)
	}
	if result := sim2.run(); result.Error != nil {
		t.Fatal(result.Error)
	}
}

func TestForeverRetWeaponAndAvoidanceInputs(t *testing.T) {
	c := defaultForeverRetConfig()
	c.Iterations = 20
	c.Duration = 60
	c.Priority = []string{}
	c.Seal = "righteousness"
	c.Talents = map[string]int{}
	baseline, _, _ := newForeverRetSimulation(c)
	base := baseline.run().RaidMetrics.Parties[0].Players[0].Dps.Avg
	c.WeaponMin *= 2
	c.WeaponMax *= 2
	stronger, _, _ := newForeverRetSimulation(c)
	damage := stronger.run().RaidMetrics.Parties[0].Players[0].Dps.Avg
	if damage <= base {
		t.Fatal("weapon input did not increase damage")
	}
	c.AvoidanceReduction = 20
	sim, a, _ := newForeverRetSimulation(c)
	view := livePhysicalAttackTableView(a.AutoAttacks.MHAuto(), a.AttackTables[a.CurrentTarget.UnitIndex])
	if view.baseDodgeChance != 0 || view.baseParryChance != 0 {
		t.Fatal("effective avoidance input was ignored")
	}
	if result := sim.run(); result.Error != nil {
		t.Fatal(result.Error)
	}
}
