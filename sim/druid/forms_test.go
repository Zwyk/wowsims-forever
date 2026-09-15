package druid

import (
	"testing"

	"github.com/wowsims/tbc/sim/core"
	"github.com/wowsims/tbc/sim/core/proto"
)

func TestFormWeaponsCarryFeralCombatCategory(t *testing.T) {
	druid := &Druid{Talents: &proto.DruidTalents{}}
	druid.Consumables = &proto.ConsumesSpec{}

	catWeapon := druid.GetCatWeapon()
	if catWeapon != core.FeralCombatWeapon(catWeapon) {
		t.Fatal("cat weapon is missing its feral combat category")
	}
	bearWeapon := druid.GetBearWeapon()
	if bearWeapon != core.FeralCombatWeapon(bearWeapon) {
		t.Fatal("bear weapon is missing its feral combat category")
	}
}
