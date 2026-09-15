package dbc

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
)

func TestGenericHitAndCritItemModsStayUniversal(t *testing.T) {
	tests := []struct {
		name string
		mod  int
		want proto.Stat
	}{
		{"generic-hit", ITEM_MOD_HIT_RATING, proto.Stat_StatHitRating},
		{"generic-crit", ITEM_MOD_CRIT_RATING, proto.Stat_StatCritRating},
		{"melee-hit", ITEM_MOD_HIT_MELEE_RATING, proto.Stat_StatMeleeHitRating},
		{"spell-hit", ITEM_MOD_HIT_SPELL_RATING, proto.Stat_StatSpellHitRating},
		{"melee-crit", ITEM_MOD_CRIT_MELEE_RATING, proto.Stat_StatMeleeCritRating},
		{"spell-crit", ITEM_MOD_CRIT_SPELL_RATING, proto.Stat_StatSpellCritRating},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := MapBonusStatIndexToStat(test.mod)
			if !ok {
				t.Fatalf("item mod %d was not mapped", test.mod)
			}
			if got != test.want {
				t.Fatalf("item mod %d: got %s, want %s", test.mod, got, test.want)
			}
		})
	}
}
