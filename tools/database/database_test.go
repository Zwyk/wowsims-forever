package database

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
)

func TestMergeItemReplacesWeaponSkillBonuses(t *testing.T) {
	db := NewWowDatabase()
	db.MergeItem(&proto.UIItem{Id: 1, WeaponSkillBonuses: []float64{0, 2, 3}})
	db.MergeItem(&proto.UIItem{Id: 1, Name: "preserve absent vector"})
	if got := db.Items[1].WeaponSkillBonuses; len(got) != 3 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("bonuses after absent overlay = %v, want preserved [0 2 3]", got)
	}

	db.MergeItem(&proto.UIItem{Id: 1, WeaponSkillBonuses: []float64{0, 7}})
	if got := db.Items[1].WeaponSkillBonuses; len(got) != 2 || got[0] != 0 || got[1] != 7 {
		t.Fatalf("merged bonuses = %v, want replacement [0 7]", got)
	}

	db.MergeItem(&proto.UIItem{Id: 1, WeaponSkillBonuses: []float64{}})
	if got := db.Items[1].WeaponSkillBonuses; got == nil || len(got) != 0 {
		t.Fatalf("bonuses after explicit empty overlay = %#v, want non-nil empty slice", got)
	}
}
