package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
)

func TestClassic60ReferenceFireballRequiresManaProfile(t *testing.T) {
	for _, test := range []struct {
		name    string
		rules   rulesetProfile
		manaBar bool
	}{
		// Even a level-60 Mage with a mana bar must not register this rank in
		// the active TBC path or the older resource-free spell diagnostic.
		{"inherited TBC", inheritedTBCRuleset(), true},
		{"resource-free spell reference", classic60SpellReferenceRules(), true},
		{"missing mana initialization", classic60MageManaReferenceRules(), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			character := &Character{
				Class: proto.Class_ClassMage,
				Unit:  Unit{Type: PlayerUnit, Level: 60, rules: test.rules, rulesInitialized: true},
			}
			if test.manaBar {
				character.manaBar.unit = &character.Unit
			}
			defer func() {
				if got := recover(); got != "Classic rank-11 Fireball requires the internal level-60 Mage mana reference" {
					t.Fatalf("expected explicit registration rejection, got %v", got)
				}
				if len(character.Spellbook) != 0 {
					t.Fatal("rejected registration changed the spellbook")
				}
			}()
			registerClassic60ReferenceFireball(character)
		})
	}
}
