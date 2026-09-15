package druid

import (
	"time"

	"github.com/wowsims/tbc/sim/core"
	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func (druid *Druid) registerTreants() {
	for idx := range druid.Treants {
		druid.Treants[idx] = druid.newTreant(idx)
	}
}

func (druid *Druid) registerForceOfNatureCD() {
	if !druid.Talents.ForceOfNature {
		return
	}

	forceOfNatureAura := druid.RegisterAura(core.Aura{
		Label:    "Force of Nature",
		ActionID: core.ActionID{SpellID: 33831},
		Duration: time.Second * 30,
	})
	druid.ForceOfNature = druid.RegisterSpell(Humanoid|Moonkin, core.SpellConfig{
		ActionID:       core.ActionID{SpellID: 33831},
		ClassSpellMask: DruidSpellForceOfNature,
		Flags:          core.SpellFlagAPL,
		ManaCost: core.ManaCostOptions{
			BaseCostPercent: 0.12,
		},
		Cast: core.CastConfig{
			DefaultCast: core.Cast{
				GCD: core.GCDDefault,
			},
			CD: core.Cooldown{
				Timer:    druid.NewTimer(),
				Duration: time.Minute * 3,
			},
		},

		ApplyEffects: func(sim *core.Simulation, _ *core.Unit, _ *core.Spell) {
			for idx := range druid.Treants {
				druid.Treants[idx].EnableWithTimeout(sim, druid.Treants[idx], time.Second*30)
			}

			forceOfNatureAura.Activate(sim)
		},
	})

	druid.AddMajorCooldown(core.MajorCooldown{
		Spell: druid.ForceOfNature.Spell,
		Type:  core.CooldownTypeDPS,
	})
}

type TreantPet struct {
	core.Pet

	druidOwner *Druid
}

type Treants [3]*TreantPet

func (druid *Druid) newTreant(idx int) *TreantPet {
	treant := &TreantPet{
		Pet: core.NewPet(core.PetConfig{
			Name:            "Treant",
			Owner:           &druid.Character,
			BaseStats:       druid.treantPetBaseStats(),
			StatInheritance: druid.treantPetStatInheritance(),
			EnabledOnStart:  false,
			IsGuardian:      true,
		}),
		druidOwner: druid,
	}

	treant.AddStatDependency(stats.Strength, stats.AttackPower, 2)
	treant.AddStatDependency(stats.Agility, stats.PhysicalCritPercent, core.CritPerAgiMaxLevel[proto.Class_ClassWarrior])

	treant.EnableAutoAttacks(treant, core.AutoAttackOptions{
		MainHand: core.Weapon{
			// TODO: Verify base damage values in-game.
			// Took the Wrath sim values * 70 / 80 to normalize for level 80
			BaseDamageMin: 220.5,
			BaseDamageMax: 312.375,
			SwingSpeed:    1.75, // Seems to vary from 1.5 to almost 2
			SpellSchool:   core.SpellSchoolPhysical,
		},
		AutoSwingMelee: true,
	})

	treant.OnPetEnable = func(sim *core.Simulation) {
		// Treant attacks are erratic. Stagger their attack and delay the first swings just a bit
		treant.AutoAttacks.StopMeleeUntil(sim, sim.CurrentTime-time.Millisecond*time.Duration(500*idx))

		// Treant spawns in front of boss but moves behind after first swing.
		treant.PseudoStats.InFrontOfTarget = true
		pa := sim.GetConsumedPendingActionFromPool()
		pa.NextActionAt = sim.CurrentTime + time.Millisecond*1500

		pa.OnAction = func(_ *core.Simulation) {
			treant.PseudoStats.InFrontOfTarget = false
		}

		sim.AddPendingAction(pa)
	}
	treant.OnPetDisable = treant.disable

	druid.AddPet(treant)

	return treant
}

func (treant *TreantPet) GetPet() *core.Pet {
	return &treant.Pet
}

func (treant *TreantPet) enable() func(*core.Simulation) {
	return func(sim *core.Simulation) {
	}
}

func (treant *TreantPet) disable(sim *core.Simulation) {
}

func (treant *TreantPet) Initialize() {
}

func (treant *TreantPet) Reset(_ *core.Simulation) {
}

func (treant *TreantPet) ExecuteCustomRotation(_ *core.Simulation) {
}

func (druid *Druid) treantPetBaseStats() stats.Stats {
	return core.ClassBaseStats[proto.Class_ClassWarrior].Add(stats.Stats{
		stats.PhysicalCritPercent: 5,
	})
}

func (druid *Druid) treantPetStatInheritance() core.PetStatInheritance {
	return func(ownerStats stats.Stats) stats.Stats {
		// TODO: Verify whether nature power affects
		power := ownerStats[stats.SpellDamage] + ownerStats[stats.NatureDamage]

		return stats.Stats{
			stats.Stamina:     ownerStats[stats.Stamina],
			stats.AttackPower: power * 0.15, // TODO: Determine scaling coefficient
		}
	}
}
