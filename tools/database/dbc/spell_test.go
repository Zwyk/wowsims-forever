package dbc

import (
	"encoding/json"
	"testing"
)

func TestSpellDefenseTypePresenceJSONCompatibility(t *testing.T) {
	tests := []struct {
		name string
		json string
		want bool
	}{
		{name: "legacy JSON", json: `{"DefenseType":0}`, want: false},
		{name: "presence-aware JSON", json: `{"DefenseType":0,"HasDefenseType":true}`, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var spell Spell
			if err := json.Unmarshal([]byte(test.json), &spell); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if spell.HasDefenseType != test.want {
				t.Fatalf("HasDefenseType = %t, want %t", spell.HasDefenseType, test.want)
			}
		})
	}
}
