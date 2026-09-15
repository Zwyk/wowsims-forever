package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wowsims/tbc/tools/database/dbc"
)

func TestGenerateProcDamageDistinguishesExplicitDefenseTypeNone(t *testing.T) {
	tests := []struct {
		name               string
		defenseType        int32
		hasDefenseType     bool
		wantDefenseType    bool
		wantConstant       string
		wantPresenceMarker bool
	}{
		{name: "explicit none", hasDefenseType: true, wantDefenseType: true, wantConstant: "core.DefenseTypeNone", wantPresenceMarker: true},
		{name: "missing category"},
		{name: "legacy nonzero", defenseType: 1, wantDefenseType: true, wantConstant: "core.DefenseTypeMagic"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outFile := filepath.Join(t.TempDir(), "effects.go")
			groups := []*Group{{
				Name: "Damage",
				Entries: []*Entry{{
					Variants:  []*Variant{{ID: 1, SpellID: 2, Name: "Test Item"}},
					Supported: true,
					Damage: &dbc.DamageEffect{
						SpellID:        2,
						SchoolMask:     int32(dbc.FIRE),
						DefenseType:    test.defenseType,
						HasDefenseType: test.hasDefenseType,
						MinDamage:      1,
						MaxDamage:      1,
					},
				}},
			}}

			if err := GenerateEffectsFile(groups, outFile, TmplStrProc); err != nil {
				t.Fatalf("GenerateEffectsFile() error = %v", err)
			}
			contents, err := os.ReadFile(outFile)
			if err != nil {
				t.Fatalf("reading generated file: %v", err)
			}
			generated := string(contents)
			gotDefenseType := strings.Contains(generated, "DefenseType:")
			if gotDefenseType != test.wantDefenseType {
				t.Fatalf("generated DefenseType field = %t, want %t:\n%s", gotDefenseType, test.wantDefenseType, generated)
			}
			if test.wantConstant != "" && !strings.Contains(generated, test.wantConstant) {
				t.Fatalf("generated defense type omitted %s:\n%s", test.wantConstant, generated)
			}
			gotPresenceMarker := strings.Contains(generated, "HasDefenseType: true")
			if gotPresenceMarker != test.wantPresenceMarker {
				t.Fatalf("generated HasDefenseType marker = %t, want %t:\n%s", gotPresenceMarker, test.wantPresenceMarker, generated)
			}
		})
	}
}
