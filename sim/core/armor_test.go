package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/stats"
)

func TestArmorDamageReductionCap(t *testing.T) {
	// Boss attacker: armorConstant = 73*467.5 - 22167.5 = 11960, so 75% reduction is
	// reached at 3*11960 = 35880 armor.
	attacker := Unit{Type: EnemyUnit, Level: 73}
	tolerance := 0.0001

	modifierForArmor := func(armor float64) float64 {
		defender := Unit{
			Type:         PlayerUnit,
			Level:        70,
			initialStats: stats.Stats{stats.Armor: armor},
			PseudoStats:  stats.NewPseudoStats(),
		}
		defender.stats = defender.initialStats
		return NewAttackTable(&attacker, &defender).GetArmorDamageModifier(nil)
	}

	if modifier := modifierForArmor(23920); !WithinToleranceFloat64(1.0/3.0, modifier, tolerance) {
		t.Fatalf("Expected %f damage taken below the cap, got %f", 1.0/3.0, modifier)
	}
	if modifier := modifierForArmor(35880); !WithinToleranceFloat64(0.25, modifier, tolerance) {
		t.Fatalf("Expected %f damage taken at the cap, got %f", 0.25, modifier)
	}
	if modifier := modifierForArmor(50000); !WithinToleranceFloat64(0.25, modifier, tolerance) {
		t.Fatalf("Expected %f damage taken above the cap, got %f", 0.25, modifier)
	}
}

func TestInheritedBossArmorFormula(t *testing.T) {
	tests := []struct {
		name              string
		armorPenetration  float64
		armorIgnoreFactor float64
		ignoreArmor       bool
		want              float64
	}{
		{"base", 0, 0, false, 0.5787309853364396},
		{"flat-armor-penetration", 1000, 0, false, 0.6122952008119472},
		{"percent-ignore-before-flat-penetration", 1000, 0.20, false, 0.6722167393588234},
		{"ignore-armor", 0, 0, true, 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attacker := Unit{
				Type:  PlayerUnit,
				Level: CharacterLevel,
				stats: stats.Stats{stats.ArmorPenetration: test.armorPenetration},
			}
			defender := Unit{
				Type:         EnemyUnit,
				Level:        DefaultBossLevel,
				initialStats: stats.Stats{stats.Armor: 7685},
				PseudoStats:  stats.NewPseudoStats(),
			}
			defender.stats = defender.initialStats
			table := NewAttackTable(&attacker, &defender)
			table.ArmorIgnoreFactor = test.armorIgnoreFactor
			table.IgnoreArmor = test.ignoreArmor

			assertFloat64(t, "damage multiplier", table.GetArmorDamageModifier(nil), test.want)
		})
	}
}
