package stats

import (
	"math"
	"testing"
)

func TestDependencySourceRoundingIsExplicitAndLocal(t *testing.T) {
	for _, test := range []struct {
		name                        string
		manager                     StatDependencyManager
		wantMana, wantCrit, wantMP5 float64
	}{
		{"zero value", StatDependencyManager{}, 2943, 2.4512, 4.7},
		{"ordinary constructor", NewStatDependencyManager(), 2943, 2.4512, 4.7},
		{"explicit floor", NewStatDependencyManagerWithSourceRounding(FloorPrimarySources), 2943, 2.4512, 4.7},
		{"fractional", NewStatDependencyManagerWithSourceRounding(PreserveFractionalSources), 2949, 2.45792, 4.725},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager := test.manager
			manager.MultiplyStat(Intellect, 1.05)
			manager.MultiplyStat(Spirit, 1.05)
			manager.AddStatDependency(Intellect, Mana, 15)
			manager.AddStatDependency(Intellect, SpellCritPercent, 0.0168)
			manager.AddStatDependency(Spirit, MP5, 0.1)
			manager.MultiplyStat(Health, 1.05)
			manager.AddStatDependency(Stamina, Health, 10)
			input := Stats{Intellect: 128, Spirit: 45, Stamina: 109, Mana: 933, SpellCritPercent: 0.2, Health: 1509}
			want := Stats{
				Intellect: 134.4, Spirit: 47.25, Stamina: 109, Mana: test.wantMana,
				SpellCritPercent: test.wantCrit, MP5: test.wantMP5, Health: 2728.95,
			}
			// Neither policy floors the returned primary values. Only the
			// source read by a cross-stat dependency differs between policies.
			assertDependencyRoundingStats(t, manager.SortAndApplyStatDependencies(input), want)
			manager.FinalizeStatDeps()
			assertDependencyRoundingStats(t, manager.ApplyStatDependencies(input), want)
		})
	}
	// Opting one manager into fractional evaluation never changes the default
	// of managers constructed afterwards or their zero-valued equivalents.
	fractional := NewStatDependencyManagerWithSourceRounding(PreserveFractionalSources)
	fractional.AddStatDependency(Intellect, Mana, 15)
	ordinary := NewStatDependencyManager()
	ordinary.AddStatDependency(Intellect, Mana, 15)
	for range 3 {
		assertDependencyRoundingStats(t, fractional.SortAndApplyStatDependencies(Stats{Intellect: 1.5}), Stats{Intellect: 1.5, Mana: 22.5})
		assertDependencyRoundingStats(t, ordinary.SortAndApplyStatDependencies(Stats{Intellect: 1.5}), Stats{Intellect: 1.5, Mana: 15})
	}
}

func TestDependencySourceRoundingDynamicLifecycle(t *testing.T) {
	for _, policy := range []DependencySourceRounding{FloorPrimarySources, PreserveFractionalSources} {
		t.Run(map[DependencySourceRounding]string{FloorPrimarySources: "floor", PreserveFractionalSources: "fractional"}[policy], func(t *testing.T) {
			manager := NewStatDependencyManagerWithSourceRounding(policy)
			manager.MultiplyStat(Intellect, 1.05)
			manager.AddStatDependency(Intellect, Mana, 15)
			multiplier := manager.NewDynamicMultiplyStat(Intellect, 1.1)
			conversion := manager.NewDynamicStatDependency(Intellect, MP5, 0.5)
			manager.FinalizeStatDeps()
			input := Stats{Intellect: 128, Mana: 933}
			wantBase, wantActive := Stats{Intellect: 134.4, Mana: 2943}, Stats{Intellect: 147.84, Mana: 3138, MP5: 73.5}
			if policy == PreserveFractionalSources {
				wantBase[Mana] = 2949
				wantActive[Mana], wantActive[MP5] = 3150.6, 73.92
			}
			assertDependencyRoundingStats(t, manager.ApplyStatDependencies(input), wantBase)
			for range 2 {
				manager.EnableDynamicStatDep(multiplier)
				manager.EnableDynamicStatDep(conversion)
				assertDependencyRoundingStats(t, manager.ApplyStatDependencies(input), wantActive)
				manager.DisableDynamicStatDep(multiplier)
				manager.DisableDynamicStatDep(conversion)
				assertDependencyRoundingStats(t, manager.ApplyStatDependencies(input), wantBase)
			}
			manager.EnableDynamicStatDep(multiplier)
			manager.EnableDynamicStatDep(conversion)
			manager.ResetStatDeps()
			assertDependencyRoundingStats(t, manager.ApplyStatDependencies(input), wantBase)
		})
	}
}

func TestDependencySourceRoundingInvalidPolicyFailsClosed(t *testing.T) {
	for _, policy := range []DependencySourceRounding{2, 255} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("invalid policy %d accepted", policy)
				}
			}()
			NewStatDependencyManagerWithSourceRounding(policy)
		}()
	}
}

func TestDependencySourceRoundingLeavesSecondarySourcesFractional(t *testing.T) {
	for _, policy := range []DependencySourceRounding{FloorPrimarySources, PreserveFractionalSources} {
		manager := NewStatDependencyManagerWithSourceRounding(policy)
		manager.AddStatDependency(FeralAttackPower, AttackPower, 1)
		manager.MultiplyStat(AttackPower, 1.1)
		got := manager.SortAndApplyStatDependencies(Stats{FeralAttackPower: 123.45, AttackPower: 5.5})
		assertDependencyRoundingStats(t, got, Stats{FeralAttackPower: 123.45, AttackPower: 141.845})
	}
}

func assertDependencyRoundingStats(t *testing.T, got, want Stats) {
	t.Helper()
	for stat := range want {
		if math.IsNaN(got[stat]) || math.IsInf(got[stat], 0) || math.Abs(got[stat]-want[stat]) > 1e-9 {
			t.Errorf("%s = %.12g, want %.12g", Stat(stat).StatName(), got[stat], want[stat])
		}
	}
}
