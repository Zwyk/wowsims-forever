package dbc

import "testing"

func TestResolveDamageEffectPreservesDefenseTypePresence(t *testing.T) {
	previous := dbcInstance
	t.Cleanup(func() {
		dbcInstance = previous
	})

	dbcInstance = NewDBC()
	const (
		explicitNoneSpellID = 1001
		missingTypeSpellID  = 1002
	)
	dbcInstance.Spells[explicitNoneSpellID] = Spell{
		ID:             explicitNoneSpellID,
		SchoolMask:     int32(FIRE),
		DefenseType:    0,
		HasDefenseType: true,
	}
	dbcInstance.Spells[missingTypeSpellID] = Spell{
		ID:         missingTypeSpellID,
		SchoolMask: int32(FIRE),
	}
	dbcInstance.SpellEffects[explicitNoneSpellID] = map[int]SpellEffect{
		0: {SpellID: explicitNoneSpellID, EffectIndex: 0, EffectType: E_SCHOOL_DAMAGE, EffectBasePoints: 9},
	}
	dbcInstance.SpellEffects[missingTypeSpellID] = map[int]SpellEffect{
		0: {SpellID: missingTypeSpellID, EffectIndex: 0, EffectType: E_SCHOOL_DAMAGE, EffectBasePoints: 9},
	}
	dbcInstance.indexSpellEffects()

	explicit := ResolveDamageEffect(explicitNoneSpellID)
	if explicit == nil || explicit.DefenseType != 0 || !explicit.HasDefenseType {
		t.Fatalf("explicit-none damage effect = %#v, want DefenseType 0 with presence", explicit)
	}

	missing := ResolveDamageEffect(missingTypeSpellID)
	if missing == nil || missing.DefenseType != 0 || missing.HasDefenseType {
		t.Fatalf("missing-type damage effect = %#v, want DefenseType 0 without presence", missing)
	}
}
