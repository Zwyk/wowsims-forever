package core

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/simsignals"
	"github.com/wowsims/tbc/sim/core/stats"
)

// The public Ret facade owns a single player and passive target. It never
// registers a TBC agent, equipment database, or a raid simulation endpoint.
//
//go:embed forever_ret_catalog.json
var foreverRetCatalogJSON []byte

type foreverRetTalent struct {
	Name      string `json:"name"`
	Max       int    `json:"max"`
	Row       int    `json:"row"`
	Tree      int    `json:"tree"`
	Req       string `json:"req"`
	Supported bool   `json:"supported"`
}

type foreverRetConfig struct {
	Operation               string         `json:"operation"`
	Duration                float64        `json:"duration"`
	Iterations              int32          `json:"iterations"`
	Seed                    int64          `json:"seed"`
	TargetLevel             int32          `json:"targetLevel"`
	Armor                   float64        `json:"armor"`
	MobType                 string         `json:"mobType"`
	ExecutePercent          float64        `json:"executePercent"`
	Strength                float64        `json:"strength"`
	Agility                 float64        `json:"agility"`
	Intellect               float64        `json:"intellect"`
	Spirit                  float64        `json:"spirit"`
	AttackPower             float64        `json:"attackPower"`
	SpellPower              float64        `json:"spellPower"`
	HealingPower            float64        `json:"healingPower"`
	Hit                     float64        `json:"hit"`
	Crit                    float64        `json:"crit"`
	MP5                     float64        `json:"mp5"`
	WeaponMin               float64        `json:"weaponMin"`
	WeaponMax               float64        `json:"weaponMax"`
	WeaponSpeed             float64        `json:"weaponSpeed"`
	WeaponSkill             float64        `json:"weaponSkill"`
	AvoidanceReduction      float64        `json:"avoidanceReduction"`
	Blessing                string         `json:"blessing"`
	Seal                    string         `json:"seal"`
	Twist                   bool           `json:"twist"`
	CrusaderOpener          bool           `json:"crusaderOpener"`
	Priority                []string       `json:"priority"`
	ConsecrationManaPercent float64        `json:"consecrationManaPercent"`
	Talents                 map[string]int `json:"talents"`
	HolyStrikeWeaponPercent float64        `json:"holyStrikeWeaponPercent"`
	HolyStrikeMin           float64        `json:"holyStrikeMin"`
	HolyStrikeMax           float64        `json:"holyStrikeMax"`
	HolyStrikeMana          float64        `json:"holyStrikeMana"`
	VindicationPPM          float64        `json:"vindicationPPM"`
}

func defaultForeverRetConfig() foreverRetConfig {
	return foreverRetConfig{Operation: "simulate", Duration: 180, Iterations: 200, Seed: 1, TargetLevel: 63, Armor: 3731, MobType: "humanoid", ExecutePercent: 20,
		Strength: 150, Agility: 70, Intellect: 50, AttackPower: 200, Hit: 9, Crit: 5, MP5: 20,
		WeaponMin: 200, WeaponMax: 300, WeaponSpeed: 3.6, WeaponSkill: 5, Blessing: "might", Seal: "command",
		Priority: []string{"hammer", "holyStrike", "judgement", "exorcism", "consecration"}, ConsecrationManaPercent: 35,
		HolyStrikeWeaponPercent: 35, HolyStrikeMin: 10, HolyStrikeMax: 13, HolyStrikeMana: 16, VindicationPPM: 3,
		Talents: map[string]int{"Improved Holy Strike": 2, "Divine Strength": 5, "Divine Intellect": 5, "Improved Seals": 3, "Reverence": 3,
			"Benediction": 5, "Improved Judgement": 2, "Holy Conduit": 2, "Conviction": 5, "Sanctified Judgement": 3, "Seal of Command": 1, "Sacred Arbiter": 1, "Crusade": 2, "Two-Handed Weapon Specialization": 3, "Vengeance": 3, "Champion of the Light": 3, "Instrument of Law": 2, "Twist of Light": 1}}
}

func (c foreverRetConfig) validate() error {
	if c.Operation != "simulate" {
		return fmt.Errorf("unknown operation")
	}
	for _, b := range []struct {
		name            string
		value, min, max float64
	}{
		{"duration", c.Duration, 15, 600}, {"iterations", float64(c.Iterations), 1, 2000}, {"seed", float64(c.Seed), 1, 2147483647},
		{"targetLevel", float64(c.TargetLevel), 60, 63}, {"armor", c.Armor, 0, 20000}, {"executePercent", c.ExecutePercent, 0, 100},
		{"strength", c.Strength, 0, 3000}, {"agility", c.Agility, 0, 3000}, {"intellect", c.Intellect, 0, 3000}, {"spirit", c.Spirit, 0, 3000},
		{"attackPower", c.AttackPower, 0, 10000}, {"spellPower", c.SpellPower, 0, 10000}, {"healingPower", c.HealingPower, 0, 10000},
		{"hit", c.Hit, 0, 100}, {"crit", c.Crit, 0, 100}, {"mp5", c.MP5, 0, 3000},
		{"weaponMin", c.WeaponMin, 1, 2000}, {"weaponMax", c.WeaponMax, c.WeaponMin, 2000}, {"weaponSpeed", c.WeaponSpeed, 1, 5},
		{"weaponSkill", c.WeaponSkill, 0, 15}, {"avoidanceReduction", c.AvoidanceReduction, 0, 20}, {"consecrationManaPercent", c.ConsecrationManaPercent, 0, 100},
		{"holyStrikeWeaponPercent", c.HolyStrikeWeaponPercent, 1, 200}, {"holyStrikeMin", c.HolyStrikeMin, 0, 2000}, {"holyStrikeMax", c.HolyStrikeMax, c.HolyStrikeMin, 2000},
		{"holyStrikeMana", c.HolyStrikeMana, 1, 2000}, {"vindicationPPM", c.VindicationPPM, 0, 60},
	} {
		if math.IsNaN(b.value) || math.IsInf(b.value, 0) || b.value < b.min || b.value > b.max {
			return fmt.Errorf("%s must be between %g and %g", b.name, b.min, b.max)
		}
	}
	if c.HolyStrikeMana != math.Trunc(c.HolyStrikeMana) {
		return fmt.Errorf("holyStrikeMana must be a whole number")
	}
	if c.MobType != "humanoid" && c.MobType != "undead" && c.MobType != "demon" {
		return fmt.Errorf("unsupported target type")
	}
	if c.Blessing != "none" && c.Blessing != "might" && c.Blessing != "wisdom" && c.Blessing != "kings" {
		return fmt.Errorf("unknown blessing")
	}
	if c.Seal != "command" && c.Seal != "righteousness" {
		return fmt.Errorf("unknown seal")
	}
	if c.Seal == "command" && c.Talents["Seal of Command"] == 0 {
		return fmt.Errorf("Command requires its talent")
	}
	if c.Twist && (c.Talents["Twist of Light"] == 0 || c.Talents["Seal of Command"] == 0) {
		return fmt.Errorf("twisting requires Twist of Light and Seal of Command")
	}
	valid := map[string]bool{"holyStrike": true, "judgement": true, "hammer": true, "exorcism": true, "consecration": true, "holyWrath": true, "swiftJudgement": true}
	seen := map[string]bool{}
	for _, key := range c.Priority {
		if !valid[key] || seen[key] {
			return fmt.Errorf("unknown or duplicate rotation action %q", key)
		}
		seen[key] = true
	}
	var catalog []foreverRetTalent
	if err := json.Unmarshal(foreverRetCatalogJSON, &catalog); err != nil {
		return err
	}
	byName := map[string]foreverRetTalent{}
	var rows [3][7]int
	var points [3]int
	total := 0
	for _, t := range catalog {
		byName[t.Name] = t
	}
	for name, rank := range c.Talents {
		t, ok := byName[name]
		if !ok || rank < 0 || rank > t.Max {
			return fmt.Errorf("invalid talent rank: %s", name)
		}
		if rank > 0 && !t.Supported {
			return fmt.Errorf("%s belongs to a Holy/Protection build outside this Ret simulator", name)
		}
		rows[t.Tree][t.Row-1] += rank
		points[t.Tree] += rank
		total += rank
	}
	if total > 51 {
		return fmt.Errorf("talents spend %d of 51 points", total)
	}
	if points[0] > 20 || points[1] > 20 {
		return fmt.Errorf("Ret simulator supports at most 20 points in each off-tree")
	}
	for name, rank := range c.Talents {
		if rank == 0 {
			continue
		}
		t := byName[name]
		lower := 0
		for row := 0; row < t.Row-1; row++ {
			lower += rows[t.Tree][row]
		}
		if lower < 5*(t.Row-1) {
			return fmt.Errorf("%s requires %d points in earlier rows", name, 5*(t.Row-1))
		}
		if t.Req != "" && c.Talents[t.Req] != byName[t.Req].Max {
			return fmt.Errorf("%s requires maximum rank %s", name, t.Req)
		}
	}
	return nil
}

// ForeverRetJSON is deliberately independent of the inherited RaidSim API.
func ForeverRetJSON(input string) (output string) {
	encode := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			return `{"error":"could not encode simulation result"}`
		}
		return string(b)
	}
	defer func() {
		if err := recover(); err != nil {
			output = encode(map[string]any{"error": fmt.Sprint(err)})
		}
	}()
	if len(input) > 65536 {
		return encode(map[string]any{"error": "request exceeds 64 KiB"})
	}
	c := defaultForeverRetConfig()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &fields); err != nil || fields == nil {
		return encode(map[string]any{"error": "expected a JSON object"})
	}
	if _, ok := fields["talents"]; ok {
		c.Talents = nil
	}
	dec := json.NewDecoder(bytes.NewBufferString(input))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return encode(map[string]any{"error": err.Error()})
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return encode(map[string]any{"error": "expected one JSON object"})
	}
	if c.Operation == "catalog" {
		return encode(map[string]any{"defaults": defaultForeverRetConfig(), "talents": json.RawMessage(foreverRetCatalogJSON), "model": foreverRetModel})
	}
	if err := c.validate(); err != nil {
		return encode(map[string]any{"error": err.Error()})
	}
	sim, agent, err := newForeverRetSimulation(c)
	if err != nil {
		return encode(map[string]any{"error": err.Error()})
	}
	result := sim.run()
	if result.Error != nil {
		return encode(map[string]any{"error": result.Error.Message})
	}
	metrics := result.RaidMetrics.Parties[0].Players[0]
	type row struct {
		Name   string  `json:"name"`
		ID     int32   `json:"id"`
		DPS    float64 `json:"dps"`
		Casts  float64 `json:"casts"`
		Hits   float64 `json:"hits"`
		Crits  float64 `json:"crits"`
		Misses float64 `json:"misses"`
		Dodges float64 `json:"dodges"`
	}
	rows := []row{}
	n := float64(c.Iterations)
	for _, a := range metrics.Actions {
		r := row{ID: a.Id.GetSpellId(), Name: agent.actionNames[a.Id.GetSpellId()]}
		if a.Id.GetOtherId() == proto.OtherAction_OtherActionAttack {
			r.Name = "Melee"
		}
		if r.Name == "" {
			r.Name = fmt.Sprintf("Spell %d", r.ID)
		}
		for _, t := range a.Targets {
			r.DPS += t.Damage / n / c.Duration
			r.Casts += float64(t.Casts) / n
			r.Hits += float64(t.Hits+t.Glances+t.Ticks) / n
			r.Crits += float64(t.Crits+t.CritTicks) / n
			r.Misses += float64(t.Misses) / n
			r.Dodges += float64(t.Dodges) / n
		}
		if r.DPS > 0 || (r.ID != 0 && r.Casts > 0) {
			rows = append(rows, r)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].DPS == rows[j].DPS {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].DPS > rows[j].DPS
	})
	return encode(map[string]any{"dps": metrics.Dps.Avg, "stdev": metrics.Dps.Stdev, "ci95": 1.96 * metrics.Dps.Stdev / math.Sqrt(n), "actions": rows, "resources": metrics.Resources, "config": c, "model": foreverRetModel})
}

var foreverRetModel = map[string]any{
	"name": "Forever Ret · provisional level-60 model", "snapshot": "2026-09-16", "source": "https://talentsforever.com", "license": "CC BY 4.0",
	"notes": []string{"Single level-60 pre-racial Human, two-handed sword, rear attacks, one passive target. Enter bonuses before talents and the selected self blessing; weapon skill includes any assumed racial bonus.", "Classic level-60 spell ranks and combat coefficients fill gaps in the demo evidence. Shared bonus hit/crit and one-third item healing as spell damage are applied.", "Holy Strike defaults to the observed level-38 35% + 10–13, 16 mana tooltip, with normal weapon damage and no spell coefficient assumed. Its level-60 rank remains unknown; edit the four values to compare assumptions.", "Unobserved talent ranks scale from available descriptions: Vengeance 1/2/3% per stack, Improved Seals 5/10/15%, 2H specialization 3/6/9%, Reverence 10/20/30%, Crusade 1/2%, Sanctified Judgement 33/66/100% and 20/40/60%. Champion uses 33/66/100%; Purifying Power 16.5/33%.", "Twist Echo is assumed to trigger the replaced seal once on the next landed white attack or Holy Strike. Seal procs cannot consume or create Echoes. Vindication uses the editable Classic 3 PPM fallback. Active seals retain Classic white-swing triggers; their normal interaction with Holy Strike remains unresolved.", "Healing, movement, control and incoming-damage talents have no DPS effect in this stationary, no-incoming-damage encounter. Holy/Protection tiers 5–7 are unavailable in this Ret model.", "Rotation reevaluates idle periods every 100 ms. Blessing starts active with full mana. Gear catalogs, raid actors and external buff automation are deferred."},
}

type foreverRetAgent struct {
	Character
	config            foreverRetConfig
	spells            *classic60PaladinSpells
	holyStrike, swift *Spell
	echo              *Spell
	openerDone        bool
	twistReady        bool
	actionNames       map[int32]string
}

func (a *foreverRetAgent) GetCharacter() *Character     { return &a.Character }
func (a *foreverRetAgent) Initialize()                  {}
func (a *foreverRetAgent) ApplyTalents()                {}
func (a *foreverRetAgent) OnEncounterStart(*Simulation) {}
func (a *foreverRetAgent) Reset(sim *Simulation) {
	a.echo = nil
	a.openerDone = !a.config.CrusaderOpener
	a.twistReady = false
	switch a.config.Blessing {
	case "might":
		a.spells.MightAura.Activate(sim)
	case "wisdom":
		a.spells.WisdomAura.Activate(sim)
	case "kings":
		a.spells.KingsAura.Activate(sim)
	}
}

func newForeverRetSimulation(c foreverRetConfig) (*Simulation, *foreverRetAgent, error) {
	rules := classic60PaladinReferenceRules()
	baseline, _ := classicReferenceInitializeCharacter(60, proto.Race_RaceHuman, proto.Class_ClassPaladin)
	raid := NewRaid(&proto.Raid{})
	party := NewParty(raid, 0, &proto.Party{}, &proto.Raid{})
	raid.Parties = []*Party{party}
	a := &foreverRetAgent{config: c, actionNames: map[int32]string{17143: "Holy Strike", 20947: "Seal of Command", 20966: "Judgement of Command", 25713: "Seal of Righteousness", 20286: "Judgement of Righteousness", 20271: "Judgement", 20920: "Cast Seal of Command", 20308: "Cast Seal of Righteousness", 20293: "Cast Seal of the Crusader", 10314: "Exorcism", 10318: "Holy Wrath", 24239: "Hammer of Wrath", 20924: "Consecration", 900001: "Swift Judgement"}}
	a.Character = newCharacterWithRuleset(party, 0, &proto.Player{Name: "Forever Retribution", Race: proto.Race_RaceHuman, Class: proto.Class_ClassPaladin, Spec: &proto.Player_RetributionPaladin{}, Equipment: &proto.EquipmentSpec{}}, rules, baseline.baseStats)
	a.Unit.foreverRet = a
	a.PseudoStats.SpiritRegenRateCasting = .1 * float64(c.Talents["Reverence"])
	a.AddBaseClassStatDependencies()
	a.EnableManaBar()
	a.AddStats(stats.Stats{stats.Strength: c.Strength, stats.Agility: c.Agility, stats.Intellect: c.Intellect, stats.Spirit: c.Spirit, stats.AttackPower: c.AttackPower, stats.SpellDamage: c.SpellPower + c.HealingPower/3, stats.HealingPower: c.HealingPower, stats.PhysicalHitPercent: c.Hit, stats.SpellHitPercent: c.Hit, stats.PhysicalCritPercent: c.Crit, stats.SpellCritPercent: c.Crit, stats.MP5: c.MP5})
	a.PseudoStats.WeaponSkillBonuses[proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedSwords] = c.WeaponSkill
	a.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{ID: -1, Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeSword, HandType: proto.HandType_HandTypeTwoHand, WeaponDamageMin: c.WeaponMin, WeaponDamageMax: c.WeaponMax, SwingSpeed: c.WeaponSpeed}
	a.EnableAutoAttacks(a, AutoAttackOptions{MainHand: a.WeaponFromMainHand(), AutoSwingMelee: true})
	a.AutoAttacks.RandomMeleeOffset = false
	party.Players = []Agent{a}
	raid.updatePlayersAndPets()
	mob := proto.MobType_MobTypeHumanoid
	if c.MobType == "undead" {
		mob = proto.MobType_MobTypeUndead
	} else if c.MobType == "demon" {
		mob = proto.MobType_MobTypeDemon
	}
	tc := &proto.Target{Level: c.TargetLevel, MobType: mob, Stats: stats.Stats{stats.Armor: c.Armor}.ToProtoArray()}
	target := newTargetWithRuleset(tc, 0, rules)
	target.stats[stats.BlockValue] = 0
	duration := time.Duration(c.Duration * float64(time.Second))
	env := &Environment{State: Constructed, Raid: raid, BaseDuration: duration, Encounter: Encounter{Duration: duration, ExecuteProportion_90: 1, ExecuteProportion_45: c.ExecutePercent / 100, ExecuteProportion_35: c.ExecutePercent / 100, ExecuteProportion_25: c.ExecutePercent / 100, ExecuteProportion_20: c.ExecutePercent / 100, AllTargets: []*Target{target}, ActiveTargets: []*Target{target}, AllTargetUnits: []*Unit{&target.Unit}, ActiveTargetUnits: []*Unit{&target.Unit}}, AllUnits: []*Unit{&target.Unit, &a.Unit}}
	env.Encounter.updateAOECapMultiplier()
	for i, u := range env.AllUnits {
		u.Env = env
		u.UnitIndex = int32(i)
	}
	a.CurrentTarget = &target.Unit
	target.initialize(tc)
	a.initialize(a)
	if err := a.registerForeverAbilities(); err != nil {
		return nil, nil, err
	}
	env.State = Initialized
	target.finalize()
	a.Finalize()
	a.Rotation = a.newCustomRotation()
	env.setupAttackTables()
	for i := 0; i < len(env.postFinalizeEffects); i++ {
		env.postFinalizeEffects[i]()
	}
	env.postFinalizeEffects = nil
	env.State = Finalized
	return newSimWithEnv(env, &proto.SimOptions{Iterations: c.Iterations, RandomSeed: c.Seed, IsTest: true}, simsignals.CreateSignals()), a, nil
}
