package core

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
)

func TestClassic60PreviewCatalogAndSnapshots(t *testing.T) {
	before := currentRuleset()
	catalog := classic60PreviewCatalog()
	if len(catalog) != 40 {
		t.Fatalf("catalog has %d entries", len(catalog))
	}
	seen := make(map[BaseStatsKey]bool)
	for _, pair := range catalog {
		key := BaseStatsKey{Race: pair.Race, Class: pair.Class}
		if seen[key] || pair.RaceName == "" || pair.ClassName == "" {
			t.Fatalf("duplicate/unnamed pair: %+v", pair)
		}
		seen[key] = true
		for _, racials := range []bool{false, true} {
			input, _ := json.Marshal(classic60PreviewRequest{Operation: "calculate", Level: 60, Race: pair.Race, Class: pair.Class, PassiveRacials: racials})
			var envelope struct {
				Data  classic60PreviewSnapshot `json:"data"`
				Error string                   `json:"error"`
			}
			if err := json.Unmarshal([]byte(Classic60PreviewJSON(string(input))), &envelope); err != nil || envelope.Error != "" {
				t.Fatalf("preview %s: %+v, %v", input, envelope, err)
			}
			got := envelope.Data
			if got.Level != 60 || got.classic60PreviewPair != pair || got.PassiveRacials != racials || got.SourceCommit != classic60PreviewSourceCommit || len(got.Stats) != 17 {
				t.Fatalf("wrong metadata for %s: %+v", input, got)
			}
			for _, stat := range got.Stats {
				if math.IsNaN(stat.Value) || math.IsInf(stat.Value, 0) {
					t.Fatalf("non-finite stat: %+v", stat)
				}
			}
			if got.Avoidance.CanBlock || got.Avoidance.BlockPercent != 0 || (!got.Avoidance.CanParry && got.Avoidance.ParryPercent != 0) {
				t.Fatalf("unusable avoidance displayed: %+v", got.Avoidance)
			}
			if got.ArmorDamageMultiplierAgainst63 <= 0 || got.ArmorDamageMultiplierAgainst63 > 1 {
				t.Fatalf("invalid armor multiplier: %v", got.ArmorDamageMultiplierAgainst63)
			}
		}
	}
	for key := range classicReferenceBaseAttributeVectors() {
		if !seen[key] {
			t.Fatalf("original supported pair missing: %+v", key)
		}
	}
	if currentRuleset() != before {
		t.Fatal("preview changed active runtime profile")
	}
	var response struct{ Data []classic60PreviewPair }
	if err := json.Unmarshal([]byte(Classic60PreviewJSON(`{"operation":"catalog"}`)), &response); err != nil || !reflect.DeepEqual(response.Data, catalog) {
		t.Fatalf("catalog JSON round trip: %+v, %v", response, err)
	}
}

func TestClassic60PreviewLiteralGnomeMage(t *testing.T) {
	for _, tc := range []struct {
		racials               bool
		intellect, mana, crit float64
	}{
		{false, 128, 2853, 2.3504},
		{true, 134.4, 2949, 2.45792},
	} {
		got, err := classic60PreviewCalculate(classic60PreviewRequest{Level: 60, Race: proto.Race_RaceGnome, Class: proto.Class_ClassMage, PassiveRacials: tc.racials})
		if err != nil {
			t.Fatal(err)
		}
		values := make(map[string]float64)
		for _, stat := range got.Stats {
			values[stat.Name] = stat.Value
		}
		assertFloat64(t, "intellect", values["Intellect"], tc.intellect)
		assertFloat64(t, "mana", values["Mana"], tc.mana)
		assertFloat64(t, "spell crit percentage", values["Spell Crit"], tc.crit)
		assertFloat64(t, "raw base mana", got.BaseMana, 1213)
		assertFloat64(t, "armor63", got.ArmorDamageMultiplierAgainst63, 5755.0/5831.0)
		if !got.HasMana || got.Avoidance.ParryPercent != 0 {
			t.Fatal("incorrect Mage resource/avoidance capabilities")
		}
	}
}

func TestClassic60PreviewRejectsMalformedAndUnsupportedRequests(t *testing.T) {
	for _, input := range []string{
		"", "null", "[]", "{}", `{"operation":"simulate"}`,
		`{"operation":"catalog"} {}`, `{"operation":"catalog"} garbage`,
		`{"operation":"catalog","gear":[]}`, `{"operation":"catalog","level":60}`,
		`{"operation":"calculate","level":70,"race":5,"class":1}`,
		`{"operation":"calculate","level":60,"race":1,"class":1}`,
		`{"operation":"calculate","level":60,"race":5,"class":11}`,
		`{"operation":"calculate","level":60,"race":5,"class":6}`,
		`{"operation":"calculate","level":60,"race":2147483648,"class":1}`,
		`{"operation":"calculate","level":60.5,"race":5,"class":1}`,
		`{"operation":"calculate","level":60,"race":5,"class":1,"passiveRacials":"yes"}`,
		strings.Repeat(" ", 4097),
	} {
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal([]byte(Classic60PreviewJSON(input)), &envelope); err != nil {
			t.Fatalf("response is not JSON: %v", err)
		}
		if len(envelope) != 1 || envelope["error"] == nil {
			t.Errorf("invalid request unexpectedly accepted: %q => %s", input, Classic60PreviewJSON(input))
		}
	}
}
