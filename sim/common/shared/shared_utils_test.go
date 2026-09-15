package shared

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/wowsims/tbc/sim/core"
)

func TestGetOutcomeMapsRangedNoCrit(t *testing.T) {
	spell := &core.Spell{}
	got := runtime.FuncForPC(reflect.ValueOf(GetOutcome(spell, OutcomeRangedNoCrit)).Pointer()).Name()
	want := runtime.FuncForPC(reflect.ValueOf(spell.OutcomeRangedHit).Pointer()).Name()
	if got != want {
		t.Fatalf("ranged no-crit outcome = %s, want %s", got, want)
	}
}
