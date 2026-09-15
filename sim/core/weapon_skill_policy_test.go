package core

import (
	"maps"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func TestClassicReferenceWeaponSkillVectors(t *testing.T) {
	tests := []struct {
		name                 string
		defenderLevel        int32
		weaponSkillBonus     float64
		baseMissChance       float64
		hitSuppression       float64
		baseDodgeChance      float64
		baseParryChance      float64
		baseGlanceChance     float64
		glanceMultiplierMin  float64
		glanceMultiplierMax  float64
		meleeCritSuppression float64
	}{
		{name: "same level", defenderLevel: 60, baseMissChance: 0.05, baseDodgeChance: 0.05, baseParryChance: 0.05, baseGlanceChance: 0.10, glanceMultiplierMin: 0.91, glanceMultiplierMax: 0.99},
		{name: "plus one", defenderLevel: 61, baseMissChance: 0.055, baseDodgeChance: 0.055, baseParryChance: 0.055, baseGlanceChance: 0.20, glanceMultiplierMin: 0.91, glanceMultiplierMax: 0.99, meleeCritSuppression: 0.01},
		{name: "plus two", defenderLevel: 62, baseMissChance: 0.06, baseDodgeChance: 0.06, baseParryChance: 0.06, baseGlanceChance: 0.30, glanceMultiplierMin: 0.80, glanceMultiplierMax: 0.90, meleeCritSuppression: 0.02},
		{name: "boss plus zero skill", defenderLevel: 63, baseMissChance: 0.08, hitSuppression: 0.01, baseDodgeChance: 0.065, baseParryChance: 0.14, baseGlanceChance: 0.40, glanceMultiplierMin: 0.55, glanceMultiplierMax: 0.75, meleeCritSuppression: 0.048},
		{name: "boss plus four skill", defenderLevel: 63, weaponSkillBonus: 4, baseMissChance: 0.072, hitSuppression: 0.002, baseDodgeChance: 0.061, baseParryChance: 0.14, baseGlanceChance: 0.40, glanceMultiplierMin: 0.75, glanceMultiplierMax: 0.87, meleeCritSuppression: 0.048},
		{name: "boss plus five skill", defenderLevel: 63, weaponSkillBonus: 5, baseMissChance: 0.06, baseDodgeChance: 0.06, baseParryChance: 0.14, baseGlanceChance: 0.40, glanceMultiplierMin: 0.80, glanceMultiplierMax: 0.90, meleeCritSuppression: 0.048},
		{name: "boss plus eight skill", defenderLevel: 63, weaponSkillBonus: 8, baseMissChance: 0.057, baseDodgeChance: 0.057, baseParryChance: 0.14, baseGlanceChance: 0.40, glanceMultiplierMin: 0.91, glanceMultiplierMax: 0.99, meleeCritSuppression: 0.048},
		{name: "boss plus fifteen skill", defenderLevel: 63, weaponSkillBonus: 15, baseMissChance: 0.05, baseDodgeChance: 0.05, baseParryChance: 0.14, baseGlanceChance: 0.40, glanceMultiplierMin: 0.91, glanceMultiplierMax: 0.99, meleeCritSuppression: 0.048},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view, ok := classicReferencePhysicalAttackTable(60, test.defenderLevel, test.weaponSkillBonus)
			if !ok {
				t.Fatal("reference policy rejected a supported vector")
			}
			assertFloat64(t, "miss", view.baseMissChance, test.baseMissChance)
			assertFloat64(t, "hit suppression", view.hitSuppression, test.hitSuppression)
			assertFloat64(t, "block", view.baseBlockChance, 0.05)
			assertFloat64(t, "dodge", view.baseDodgeChance, test.baseDodgeChance)
			assertFloat64(t, "parry", view.baseParryChance, test.baseParryChance)
			assertFloat64(t, "glance", view.baseGlanceChance, test.baseGlanceChance)
			assertFloat64(t, "glance minimum", view.glanceMultiplierMin, test.glanceMultiplierMin)
			assertFloat64(t, "glance maximum", view.glanceMultiplierMax, test.glanceMultiplierMax)
			assertFloat64(t, "glance mean", view.glanceMultiplierMean(), (test.glanceMultiplierMin+test.glanceMultiplierMax)/2)
			assertFloat64(t, "crit suppression", view.meleeCritSuppression, test.meleeCritSuppression)
		})
	}
}

func TestClassicReferenceWeaponSkillBreakpointsAndInvariantFields(t *testing.T) {
	plusFour, ok := classicReferencePhysicalAttackTable(60, 63, 4)
	if !ok {
		t.Fatal("reference policy rejected +4 skill")
	}
	plusFive, ok := classicReferencePhysicalAttackTable(60, 63, 5)
	if !ok {
		t.Fatal("reference policy rejected +5 skill")
	}
	plusEight, ok := classicReferencePhysicalAttackTable(60, 63, 8)
	if !ok {
		t.Fatal("reference policy rejected +8 skill")
	}

	assertFloat64(t, "+4 hit suppression", plusFour.hitSuppression, 0.002)
	assertFloat64(t, "+5 hit suppression", plusFive.hitSuppression, 0)
	assertFloat64(t, "+8 glance minimum cap", plusEight.glanceMultiplierMin, 0.91)
	assertFloat64(t, "+8 glance maximum cap", plusEight.glanceMultiplierMax, 0.99)
	assertFloat64(t, "parry ignores bonus", plusFour.baseParryChance, plusEight.baseParryChance)
	assertFloat64(t, "glance chance ignores bonus", plusFour.baseGlanceChance, plusEight.baseGlanceChance)
	assertFloat64(t, "crit suppression ignores bonus", plusFour.meleeCritSuppression, plusEight.meleeCritSuppression)
}

func TestClassicReferenceWeaponSkillKeepsFiniteFractionalModifiers(t *testing.T) {
	fractional, ok := classicReferencePhysicalAttackTable(60, 63, 4.5)
	if !ok {
		t.Fatal("reference policy rejected a finite fractional bonus")
	}
	assertFloat64(t, "fractional miss", fractional.baseMissChance, 0.071)
	assertFloat64(t, "fractional hit suppression", fractional.hitSuppression, 0.001)
	assertFloat64(t, "fractional dodge", fractional.baseDodgeChance, 0.0605)
	assertFloat64(t, "fractional glance minimum", fractional.glanceMultiplierMin, 0.775)
	assertFloat64(t, "fractional glance maximum", fractional.glanceMultiplierMax, 0.885)

	negative, ok := classicReferencePhysicalAttackTable(60, 63, -1)
	if !ok {
		t.Fatal("reference policy rejected a finite negative modifier")
	}
	assertFloat64(t, "negative modifier miss", negative.baseMissChance, 0.082)
	assertFloat64(t, "negative modifier glance minimum", negative.glanceMultiplierMin, 0.50)
}

func TestClassicReferenceWeaponSkillScopeFailsClosed(t *testing.T) {
	tests := []struct {
		name          string
		attackerLevel int32
		defenderLevel int32
		bonus         float64
	}{
		{name: "below level 60", attackerLevel: 59, defenderLevel: 62},
		{name: "above level 60", attackerLevel: 61, defenderLevel: 64},
		{name: "lower target", attackerLevel: 60, defenderLevel: 59},
		{name: "target above plus three", attackerLevel: 60, defenderLevel: 64},
		{name: "not a number", attackerLevel: 60, defenderLevel: 63, bonus: math.NaN()},
		{name: "positive infinity", attackerLevel: 60, defenderLevel: 63, bonus: math.Inf(1)},
		{name: "negative infinity", attackerLevel: 60, defenderLevel: 63, bonus: math.Inf(-1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view, ok := classicReferencePhysicalAttackTable(test.attackerLevel, test.defenderLevel, test.bonus)
			if ok {
				t.Fatalf("unsupported input resolved to %+v", view)
			}
			if view != (physicalAttackTableView{}) {
				t.Fatalf("rejected input returned nonzero view %+v", view)
			}
		})
	}
}

func TestDisabledWeaponSkillModelCopiesInheritedAttackTable(t *testing.T) {
	table := &AttackTable{
		rulesInitialized:     true,
		outcomes:             outcomeRules{weaponSkillModel: weaponSkillModelDisabled},
		BaseMissChance:       0.11,
		HitSuppression:       0.12,
		BaseBlockChance:      0.13,
		BaseDodgeChance:      0.14,
		BaseParryChance:      0.15,
		BaseGlanceChance:     0.16,
		GlanceMultiplier:     0.17,
		MeleeCritSuppression: 0.18,
	}
	view, ok := resolvePhysicalAttackTableView(nil, table)
	if !ok {
		t.Fatal("disabled model rejected an existing attack table")
	}
	want := physicalAttackTableView{
		baseMissChance:       0.11,
		hitSuppression:       0.12,
		baseBlockChance:      0.13,
		baseDodgeChance:      0.14,
		baseParryChance:      0.15,
		baseGlanceChance:     0.16,
		glanceMultiplierMin:  0.17,
		glanceMultiplierMax:  0.17,
		meleeCritSuppression: 0.18,
	}
	if view != want {
		t.Fatalf("disabled view = %+v, want %+v", view, want)
	}
}

func TestClassicReferenceWeaponSkillViewUsesCurrentWeaponContext(t *testing.T) {
	rules := inheritedTBCRuleset()
	rules.combat.outcomes.weaponSkillModel = weaponSkillModelClassicReference
	character := &Character{Unit: Unit{
		Type:        PlayerUnit,
		Level:       60,
		PseudoStats: stats.NewPseudoStats(),
	}}
	character.AutoAttacks.character = character
	character.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{
		ID:         1,
		Type:       proto.ItemType_ItemTypeWeapon,
		WeaponType: proto.WeaponType_WeaponTypeAxe,
		HandType:   proto.HandType_HandTypeOneHand,
	}
	character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryAxes, 5)
	defender := &Unit{Type: EnemyUnit, Level: 63, PseudoStats: stats.NewPseudoStats()}
	table := newAttackTableWithRuleset(&character.Unit, defender, rules)
	spell := &Spell{Unit: &character.Unit, DefenseType: DefenseTypeMelee, weaponAttackSource: WeaponAttackSourceMainHand}
	attackTableBeforeResolution := *table
	attackTableBeforeResolution.MobTypeBonusStats = maps.Clone(table.MobTypeBonusStats)
	attackTableBeforeResolution.DamageDoneByCasterExtraMultiplier = slices.Clone(table.DamageDoneByCasterExtraMultiplier)

	view, ok := resolvePhysicalAttackTableView(spell, table)
	if !ok {
		t.Fatal("Classic reference model rejected an audited equipped weapon")
	}
	assertFloat64(t, "context miss", view.baseMissChance, 0.06)
	assertFloat64(t, "context dodge", view.baseDodgeChance, 0.06)
	assertFloat64(t, "context glance minimum", view.glanceMultiplierMin, 0.80)
	assertFloat64(t, "context glance maximum", view.glanceMultiplierMax, 0.90)
	if !reflect.DeepEqual(*table, attackTableBeforeResolution) {
		t.Fatal("first Classic projection mutated shared attack table")
	}

	character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryAxes, 3)
	updated, ok := resolvePhysicalAttackTableView(spell, table)
	if !ok {
		t.Fatal("Classic reference model rejected updated weapon context")
	}
	assertFloat64(t, "updated context miss", updated.baseMissChance, 0.057)
	assertFloat64(t, "updated context glance minimum", updated.glanceMultiplierMin, 0.91)
	if !reflect.DeepEqual(*table, attackTableBeforeResolution) {
		t.Fatal("updated Classic projection mutated shared attack table")
	}
}

func TestClassicReferenceWeaponSkillViewSupportsAuditedWeaponKinds(t *testing.T) {
	tests := []struct {
		name               string
		defenseType        DefenseType
		source             WeaponAttackSource
		expectedSkillBonus float64
		configureCharacter func(character *Character)
	}{
		{
			name:               "off-hand dagger",
			defenseType:        DefenseTypeMelee,
			source:             WeaponAttackSourceOffHand,
			expectedSkillBonus: 2,
			configureCharacter: func(character *Character) {
				character.Equipment[proto.ItemSlot_ItemSlotOffHand] = Item{
					ID: 1, Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeDagger, HandType: proto.HandType_HandTypeOneHand,
				}
				character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryDaggers, 2)
			},
		},
		{
			name:               "ranged bow",
			defenseType:        DefenseTypeRanged,
			source:             WeaponAttackSourceRanged,
			expectedSkillBonus: 3,
			configureCharacter: func(character *Character) {
				character.Equipment[proto.ItemSlot_ItemSlotRanged] = Item{
					ID: 1, Type: proto.ItemType_ItemTypeRanged, RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeBow,
				}
				character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryBows, 3)
			},
		},
		{
			name:               "synthetic feral weapon",
			defenseType:        DefenseTypeMelee,
			source:             WeaponAttackSourceMainHand,
			expectedSkillBonus: 4,
			configureCharacter: func(character *Character) {
				character.AutoAttacks.mh.Weapon = FeralCombatWeapon(Weapon{SwingSpeed: 1})
				character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryFeralCombat, 4)
			},
		},
		{
			name:               "equipped fist weapon uses unarmed skill bonus",
			defenseType:        DefenseTypeMelee,
			source:             WeaponAttackSourceMainHand,
			expectedSkillBonus: 5,
			configureCharacter: func(character *Character) {
				character.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{
					ID: 1, Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeFist, HandType: proto.HandType_HandTypeOneHand,
				}
				character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryUnarmed, 5)
			},
		},
		{
			name:               "bare unarmed ignores fist skill bonus",
			defenseType:        DefenseTypeMelee,
			source:             WeaponAttackSourceMainHand,
			expectedSkillBonus: 0,
			configureCharacter: func(character *Character) {
				character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryUnarmed, 50)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rules := inheritedTBCRuleset()
			rules.combat.outcomes.weaponSkillModel = weaponSkillModelClassicReference
			character := &Character{Unit: Unit{Type: PlayerUnit, Level: 60, PseudoStats: stats.NewPseudoStats()}}
			character.AutoAttacks.character = character
			test.configureCharacter(character)
			defender := &Unit{Type: EnemyUnit, Level: 63, PseudoStats: stats.NewPseudoStats()}
			table := newAttackTableWithRuleset(&character.Unit, defender, rules)
			spell := &Spell{Unit: &character.Unit, DefenseType: test.defenseType, weaponAttackSource: test.source}

			got, ok := resolvePhysicalAttackTableView(spell, table)
			if !ok {
				t.Fatal("Classic reference model rejected an audited weapon kind")
			}
			want, ok := classicReferencePhysicalAttackTable(60, 63, test.expectedSkillBonus)
			if !ok {
				t.Fatal("reference policy rejected expected skill bonus")
			}
			if got != want {
				t.Fatalf("resolved view = %+v, want %+v", got, want)
			}
		})
	}
}

func TestClassicReferenceWeaponSkillViewRejectsUnauditedContexts(t *testing.T) {
	newFixture := func() (*Character, *Unit, *AttackTable) {
		rules := inheritedTBCRuleset()
		rules.combat.outcomes.weaponSkillModel = weaponSkillModelClassicReference
		character := &Character{Unit: Unit{Type: PlayerUnit, Level: 60, PseudoStats: stats.NewPseudoStats()}}
		character.AutoAttacks.character = character
		defender := &Unit{Type: EnemyUnit, Level: 63, PseudoStats: stats.NewPseudoStats()}
		return character, defender, newAttackTableWithRuleset(&character.Unit, defender, rules)
	}

	t.Run("nil table", func(t *testing.T) {
		if _, ok := resolvePhysicalAttackTableView(nil, nil); ok {
			t.Fatal("nil table resolved")
		}
	})

	t.Run("nil spell", func(t *testing.T) {
		_, _, table := newFixture()
		if _, ok := resolvePhysicalAttackTableView(nil, table); ok {
			t.Fatal("nil spell resolved")
		}
	})

	t.Run("unspecified source", func(t *testing.T) {
		character, _, table := newFixture()
		if _, ok := resolvePhysicalAttackTableView(&Spell{Unit: &character.Unit, DefenseType: DefenseTypeMelee}, table); ok {
			t.Fatal("unspecified source resolved")
		}
	})

	t.Run("explicit none", func(t *testing.T) {
		character, _, table := newFixture()
		spell := &Spell{Unit: &character.Unit, DefenseType: DefenseTypeMelee, weaponAttackSource: WeaponAttackSourceNone}
		if _, ok := resolvePhysicalAttackTableView(spell, table); ok {
			t.Fatal("explicit-none source resolved")
		}
	})

	t.Run("missing off hand", func(t *testing.T) {
		character, _, table := newFixture()
		spell := &Spell{Unit: &character.Unit, DefenseType: DefenseTypeMelee, weaponAttackSource: WeaponAttackSourceOffHand}
		if _, ok := resolvePhysicalAttackTableView(spell, table); ok {
			t.Fatal("missing off hand resolved")
		}
	})

	t.Run("wand extension", func(t *testing.T) {
		character, _, table := newFixture()
		character.Equipment[proto.ItemSlot_ItemSlotRanged] = Item{
			ID:               1,
			Type:             proto.ItemType_ItemTypeRanged,
			RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeWand,
		}
		spell := &Spell{Unit: &character.Unit, DefenseType: DefenseTypeRanged, weaponAttackSource: WeaponAttackSourceRanged}
		if _, ok := resolvePhysicalAttackTableView(spell, table); ok {
			t.Fatal("unsupported wand resolved")
		}
	})

	t.Run("mismatched spell attacker", func(t *testing.T) {
		character, _, table := newFixture()
		other := &Unit{Type: PlayerUnit, Level: 60}
		spell := &Spell{Unit: other, DefenseType: DefenseTypeMelee, weaponAttackSource: WeaponAttackSourceMainHand}
		character.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{
			ID: 1, Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeAxe, HandType: proto.HandType_HandTypeOneHand,
		}
		if _, ok := resolvePhysicalAttackTableView(spell, table); ok {
			t.Fatal("mismatched attacker resolved")
		}
	})

	t.Run("non-player attacker", func(t *testing.T) {
		_, defender, table := newFixture()
		attacker := &Unit{Type: PetUnit, Level: 60}
		table.Attacker = attacker
		spell := &Spell{Unit: attacker, DefenseType: DefenseTypeMelee, weaponAttackSource: WeaponAttackSourceMainHand}
		if _, ok := resolvePhysicalAttackTableView(spell, table); ok {
			t.Fatal("non-player attacker resolved")
		}
		_ = defender
	})

	t.Run("non-enemy defender", func(t *testing.T) {
		character, _, table := newFixture()
		table.Defender = &Unit{Type: PlayerUnit, Level: 63}
		spell := &Spell{Unit: &character.Unit, DefenseType: DefenseTypeMelee, weaponAttackSource: WeaponAttackSourceMainHand}
		if _, ok := resolvePhysicalAttackTableView(spell, table); ok {
			t.Fatal("non-enemy defender resolved")
		}
	})

	t.Run("missing player character", func(t *testing.T) {
		character, _, table := newFixture()
		character.AutoAttacks.character = nil
		spell := &Spell{Unit: &character.Unit, DefenseType: DefenseTypeMelee, weaponAttackSource: WeaponAttackSourceMainHand}
		if _, ok := resolvePhysicalAttackTableView(spell, table); ok {
			t.Fatal("unresolved player character resolved")
		}
	})

	t.Run("non-physical defense type", func(t *testing.T) {
		character, _, table := newFixture()
		character.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{
			ID: 1, Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeAxe, HandType: proto.HandType_HandTypeOneHand,
		}
		spell := &Spell{Unit: &character.Unit, DefenseType: DefenseTypeMagic, weaponAttackSource: WeaponAttackSourceMainHand}
		if _, ok := resolvePhysicalAttackTableView(spell, table); ok {
			t.Fatal("non-physical defense type resolved")
		}
	})

	t.Run("ranged source with melee defense type", func(t *testing.T) {
		character, _, table := newFixture()
		character.Equipment[proto.ItemSlot_ItemSlotRanged] = Item{
			ID: 1, Type: proto.ItemType_ItemTypeRanged, RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeBow,
		}
		spell := &Spell{Unit: &character.Unit, DefenseType: DefenseTypeMelee, weaponAttackSource: WeaponAttackSourceRanged}
		if _, ok := resolvePhysicalAttackTableView(spell, table); ok {
			t.Fatal("mismatched ranged defense type resolved")
		}
	})

	t.Run("generic synthetic weapon", func(t *testing.T) {
		character, _, table := newFixture()
		character.AutoAttacks.mh.Weapon = Weapon{SwingSpeed: 1}
		character.AutoAttacks.mh.Weapon.normalizeClassification()
		spell := &Spell{Unit: &character.Unit, DefenseType: DefenseTypeMelee, weaponAttackSource: WeaponAttackSourceMainHand}
		if _, ok := resolvePhysicalAttackTableView(spell, table); ok {
			t.Fatal("unaudited synthetic weapon resolved")
		}
	})
}

func TestActiveRulesetKeepsWeaponSkillModelDisabled(t *testing.T) {
	if activeRulesetID != rulesetInheritedTBC {
		t.Fatalf("active ruleset = %d, want inherited TBC", activeRulesetID)
	}
	active := currentRuleset()
	if active != inheritedTBCRuleset() {
		t.Fatal("current ruleset differs from inherited TBC")
	}
	if got := active.combat.outcomes.weaponSkillModel; got != weaponSkillModelDisabled {
		t.Fatalf("active weapon-skill model = %d, want disabled", got)
	}
}
