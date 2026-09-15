package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

func TestWeaponSkillCategoryLayoutPreservesClassicIndices(t *testing.T) {
	tests := []struct {
		category proto.WeaponSkillCategory
		index    int32
	}{
		{proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified, 0},
		{proto.WeaponSkillCategory_WeaponSkillCategoryAxes, 1},
		{proto.WeaponSkillCategory_WeaponSkillCategorySwords, 2},
		{proto.WeaponSkillCategory_WeaponSkillCategoryMaces, 3},
		{proto.WeaponSkillCategory_WeaponSkillCategoryDaggers, 4},
		{proto.WeaponSkillCategory_WeaponSkillCategoryUnarmed, 5},
		{proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedAxes, 6},
		{proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedSwords, 7},
		{proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedMaces, 8},
		{proto.WeaponSkillCategory_WeaponSkillCategoryPolearms, 9},
		{proto.WeaponSkillCategory_WeaponSkillCategoryStaves, 10},
		{proto.WeaponSkillCategory_WeaponSkillCategoryThrown, 11},
		{proto.WeaponSkillCategory_WeaponSkillCategoryBows, 12},
		{proto.WeaponSkillCategory_WeaponSkillCategoryCrossbows, 13},
		{proto.WeaponSkillCategory_WeaponSkillCategoryGuns, 14},
		{proto.WeaponSkillCategory_WeaponSkillCategoryFeralCombat, 15},
		{proto.WeaponSkillCategory_WeaponSkillCategoryWands, 16},
	}

	for _, test := range tests {
		if int32(test.category) != test.index {
			t.Fatalf("%s = %d, want stable index %d", test.category, test.category, test.index)
		}
	}
	if stats.WeaponSkillCategoryLen != len(tests) {
		t.Fatalf("category length = %d, want %d", stats.WeaponSkillCategoryLen, len(tests))
	}
}

func TestWeaponSkillBonusVectorRoundTripAndArithmetic(t *testing.T) {
	input := make([]float64, stats.WeaponSkillCategoryLen+2)
	input[proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified] = 99
	input[proto.WeaponSkillCategory_WeaponSkillCategoryAxes] = 5
	input[proto.WeaponSkillCategory_WeaponSkillCategoryWands] = 3
	input[len(input)-1] = 99

	bonuses := stats.WeaponSkillBonusesFromProto(input)
	if got := bonuses.Get(proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified); got != 0 {
		t.Fatalf("unspecified bonus = %v, want 0", got)
	}
	if got := bonuses.Get(proto.WeaponSkillCategory_WeaponSkillCategoryAxes); got != 5 {
		t.Fatalf("axes bonus = %v, want 5", got)
	}
	if got := bonuses.Get(proto.WeaponSkillCategory_WeaponSkillCategoryWands); got != 3 {
		t.Fatalf("wands bonus = %v, want 3", got)
	}
	if got := bonuses.Get(proto.WeaponSkillCategory(-1)); got != 0 {
		t.Fatalf("invalid category bonus = %v, want 0", got)
	}
	if got := len(bonuses.ToProtoArray()); got != stats.WeaponSkillCategoryLen {
		t.Fatalf("serialized length = %d, want %d", got, stats.WeaponSkillCategoryLen)
	}

	delta := stats.WeaponSkillBonuses{}
	delta[proto.WeaponSkillCategory_WeaponSkillCategoryAxes] = 2
	if got := bonuses.Add(delta).Subtract(delta); got != bonuses {
		t.Fatalf("add/subtract round trip = %v, want %v", got, bonuses)
	}
}

func TestWeaponSkillCategoryFromItem(t *testing.T) {
	melee := func(weaponType proto.WeaponType, handType proto.HandType) *Item {
		return &Item{ID: 1, Type: proto.ItemType_ItemTypeWeapon, WeaponType: weaponType, HandType: handType}
	}
	ranged := func(rangedWeaponType proto.RangedWeaponType) *Item {
		return &Item{ID: 1, Type: proto.ItemType_ItemTypeRanged, RangedWeaponType: rangedWeaponType}
	}
	tests := []struct {
		name string
		item *Item
		want proto.WeaponSkillCategory
	}{
		{name: "nil", want: proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified},
		{name: "zero ID", item: &Item{Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeAxe, HandType: proto.HandType_HandTypeOneHand}, want: proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified},
		{name: "one-handed axe", item: melee(proto.WeaponType_WeaponTypeAxe, proto.HandType_HandTypeOneHand), want: proto.WeaponSkillCategory_WeaponSkillCategoryAxes},
		{name: "two-handed axe", item: melee(proto.WeaponType_WeaponTypeAxe, proto.HandType_HandTypeTwoHand), want: proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedAxes},
		{name: "main-hand sword", item: melee(proto.WeaponType_WeaponTypeSword, proto.HandType_HandTypeMainHand), want: proto.WeaponSkillCategory_WeaponSkillCategorySwords},
		{name: "two-handed sword", item: melee(proto.WeaponType_WeaponTypeSword, proto.HandType_HandTypeTwoHand), want: proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedSwords},
		{name: "off-hand mace", item: melee(proto.WeaponType_WeaponTypeMace, proto.HandType_HandTypeOffHand), want: proto.WeaponSkillCategory_WeaponSkillCategoryMaces},
		{name: "two-handed mace", item: melee(proto.WeaponType_WeaponTypeMace, proto.HandType_HandTypeTwoHand), want: proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedMaces},
		{name: "dagger", item: melee(proto.WeaponType_WeaponTypeDagger, proto.HandType_HandTypeOneHand), want: proto.WeaponSkillCategory_WeaponSkillCategoryDaggers},
		{name: "fist maps to unarmed", item: melee(proto.WeaponType_WeaponTypeFist, proto.HandType_HandTypeOneHand), want: proto.WeaponSkillCategory_WeaponSkillCategoryUnarmed},
		{name: "polearm", item: melee(proto.WeaponType_WeaponTypePolearm, proto.HandType_HandTypeTwoHand), want: proto.WeaponSkillCategory_WeaponSkillCategoryPolearms},
		{name: "staff", item: melee(proto.WeaponType_WeaponTypeStaff, proto.HandType_HandTypeTwoHand), want: proto.WeaponSkillCategory_WeaponSkillCategoryStaves},
		{name: "bow", item: ranged(proto.RangedWeaponType_RangedWeaponTypeBow), want: proto.WeaponSkillCategory_WeaponSkillCategoryBows},
		{name: "crossbow", item: ranged(proto.RangedWeaponType_RangedWeaponTypeCrossbow), want: proto.WeaponSkillCategory_WeaponSkillCategoryCrossbows},
		{name: "gun", item: ranged(proto.RangedWeaponType_RangedWeaponTypeGun), want: proto.WeaponSkillCategory_WeaponSkillCategoryGuns},
		{name: "thrown", item: ranged(proto.RangedWeaponType_RangedWeaponTypeThrown), want: proto.WeaponSkillCategory_WeaponSkillCategoryThrown},
		{name: "wand", item: ranged(proto.RangedWeaponType_RangedWeaponTypeWand), want: proto.WeaponSkillCategory_WeaponSkillCategoryWands},
		{name: "shield", item: melee(proto.WeaponType_WeaponTypeShield, proto.HandType_HandTypeOffHand), want: proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified},
		{name: "relic", item: ranged(proto.RangedWeaponType_RangedWeaponTypeIdol), want: proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified},
		{name: "unknown hand", item: melee(proto.WeaponType_WeaponTypeAxe, proto.HandType_HandTypeUnknown), want: proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified},
		{name: "unknown item type with melee metadata", item: &Item{ID: 1, WeaponType: proto.WeaponType_WeaponTypeAxe, HandType: proto.HandType_HandTypeOneHand}, want: proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified},
		{name: "unknown item type with ranged metadata", item: &Item{ID: 1, RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeBow}, want: proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified},
		{name: "armor with melee metadata", item: &Item{ID: 1, Type: proto.ItemType_ItemTypeHead, WeaponType: proto.WeaponType_WeaponTypeAxe, HandType: proto.HandType_HandTypeOneHand}, want: proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified},
		{name: "melee item with ranged metadata", item: &Item{ID: 1, Type: proto.ItemType_ItemTypeWeapon, RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeBow}, want: proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified},
		{name: "ranged item with melee metadata", item: &Item{ID: 1, Type: proto.ItemType_ItemTypeRanged, WeaponType: proto.WeaponType_WeaponTypeAxe, HandType: proto.HandType_HandTypeOneHand}, want: proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified},
		{name: "ranged with melee hand", item: &Item{ID: 1, Type: proto.ItemType_ItemTypeRanged, HandType: proto.HandType_HandTypeTwoHand, RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeBow}, want: proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified},
		{name: "ambiguous melee and ranged", item: &Item{ID: 1, Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeSword, HandType: proto.HandType_HandTypeOneHand, RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeBow}, want: proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := weaponSkillCategoryFromItem(test.item); got != test.want {
				t.Fatalf("category = %s, want %s", got, test.want)
			}
		})
	}
}

func TestEquippedWeaponSkillCategoryFailsClosedForWrongSlot(t *testing.T) {
	character := &Character{Unit: Unit{Type: PlayerUnit, PseudoStats: stats.NewPseudoStats()}}
	character.AutoAttacks.character = character
	character.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{
		ID:               1,
		Type:             proto.ItemType_ItemTypeRanged,
		RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeBow,
	}
	character.Equipment[proto.ItemSlot_ItemSlotRanged] = Item{
		ID:         2,
		Type:       proto.ItemType_ItemTypeWeapon,
		WeaponType: proto.WeaponType_WeaponTypeAxe,
		HandType:   proto.HandType_HandTypeOneHand,
	}
	character.Equipment[proto.ItemSlot_ItemSlotOffHand] = Item{
		ID:         3,
		Type:       proto.ItemType_ItemTypeWeapon,
		WeaponType: proto.WeaponType_WeaponTypeSword,
		HandType:   proto.HandType_HandTypeTwoHand,
	}

	tests := []struct {
		name   string
		source WeaponAttackSource
	}{
		{name: "ranged item in main hand", source: WeaponAttackSourceMainHand},
		{name: "melee item in ranged slot", source: WeaponAttackSourceRanged},
		{name: "two-handed item in off hand", source: WeaponAttackSourceOffHand},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			classification := character.equippedWeaponClassification(test.source)
			if classification.kind != weaponClassificationEquippedItem {
				t.Fatalf("classification kind = %d, want equipped item", classification.kind)
			}
			if classification.skillCategory != proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified {
				t.Fatalf("skill category = %s, want unspecified", classification.skillCategory)
			}
		})
	}
}

func TestWeaponSkillBonusesLoadAndAggregateOutsideStats(t *testing.T) {
	protoBonuses := make([]float64, stats.WeaponSkillCategoryLen)
	protoBonuses[proto.WeaponSkillCategory_WeaponSkillCategoryAxes] = 3
	loadedItem := ItemFromProto(&proto.SimItem{Id: 1, WeaponSkillBonuses: protoBonuses})

	equipment := Equipment{}
	equipment[proto.ItemSlot_ItemSlotHands] = loadedItem
	swordItem := Item{ID: 2}
	swordItem.WeaponSkillBonuses[proto.WeaponSkillCategory_WeaponSkillCategorySwords] = 4
	equipment[proto.ItemSlot_ItemSlotMainHand] = swordItem
	invalidItem := Item{}
	invalidItem.WeaponSkillBonuses[proto.WeaponSkillCategory_WeaponSkillCategoryAxes] = 100
	equipment[proto.ItemSlot_ItemSlotNeck] = invalidItem

	character := Character{
		Unit:      Unit{PseudoStats: stats.NewPseudoStats()},
		Equipment: equipment,
	}
	character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryAxes, 2)
	character.EquipScalingManager = character.NewEquipScalingManager()
	regularStatsBefore := character.GetStats()
	character.applyEquipment()

	if got := character.weaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryAxes); got != 5 {
		t.Fatalf("aggregated axes bonus = %v, want 5", got)
	}
	if got := character.weaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategorySwords); got != 4 {
		t.Fatalf("aggregated swords bonus = %v, want 4", got)
	}
	if got := character.GetStats(); got != regularStatsBefore {
		t.Fatalf("weapon-skill bonuses changed regular stats: before=%v after=%v", regularStatsBefore, got)
	}
}

func TestItemSwapUpdatesWeaponSkillBonusesWithoutAutoAttackRefresh(t *testing.T) {
	oldItem := Item{ID: 1, Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeAxe, HandType: proto.HandType_HandTypeOneHand}
	oldItem.WeaponSkillBonuses[proto.WeaponSkillCategory_WeaponSkillCategoryAxes] = 2
	newItem := Item{ID: 2, Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeSword, HandType: proto.HandType_HandTypeOneHand}
	newItem.WeaponSkillBonuses[proto.WeaponSkillCategory_WeaponSkillCategorySwords] = 7

	character := &Character{Unit: Unit{Type: PlayerUnit, PseudoStats: stats.NewPseudoStats()}}
	character.AutoAttacks.character = character
	character.Equipment[proto.ItemSlot_ItemSlotMainHand] = oldItem
	character.addWeaponSkillBonuses(character.Equipment.WeaponSkillBonuses())
	swap := ItemSwap{character: character, originalEquip: character.Equipment}
	swap.unEquippedItems[proto.ItemSlot_ItemSlotMainHand] = newItem
	spell := &Spell{Unit: &character.Unit, weaponAttackSource: WeaponAttackSourceMainHand}
	before := spell.weaponAttackContext()
	if before.classification.skillCategory != proto.WeaponSkillCategory_WeaponSkillCategoryAxes || before.weaponSkillBonus != 2 {
		t.Fatalf("pre-swap context = %+v, want axes +2", before)
	}

	// AutoSwingMelee is false, so the cached auto-attack weapon stays untouched.
	swap.swapItem(&Simulation{}, proto.ItemSlot_ItemSlotMainHand, false, false)
	if got := character.weaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryAxes); got != 0 {
		t.Fatalf("post-swap axes bonus = %v, want 0", got)
	}
	if got := character.weaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategorySwords); got != 7 {
		t.Fatalf("post-swap swords bonus = %v, want 7", got)
	}
	if got := character.AutoAttacks.MH().classification.kind; got != weaponClassificationUnspecified {
		t.Fatalf("disabled auto-attack cache kind = %d, want untouched", got)
	}
	after := spell.weaponAttackContext()
	if after.classification.skillCategory != proto.WeaponSkillCategory_WeaponSkillCategorySwords || after.weaponSkillBonus != 7 {
		t.Fatalf("post-swap context = %+v, want swords +7", after)
	}

	swap.swapItem(&Simulation{}, proto.ItemSlot_ItemSlotMainHand, true, true)
	if got := character.weaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryAxes); got != 2 {
		t.Fatalf("reset axes bonus = %v, want 2", got)
	}
	if got := character.weaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategorySwords); got != 0 {
		t.Fatalf("reset swords bonus = %v, want 0", got)
	}
	reset := spell.weaponAttackContext()
	if reset.classification.skillCategory != proto.WeaponSkillCategory_WeaponSkillCategoryAxes || reset.weaponSkillBonus != 2 {
		t.Fatalf("reset context = %+v, want axes +2", reset)
	}
}

func TestPrepullNonWeaponSwapUpdatesWeaponSkillBonuses(t *testing.T) {
	oldItem := Item{ID: 1}
	oldItem.WeaponSkillBonuses[proto.WeaponSkillCategory_WeaponSkillCategorySwords] = 1
	newItem := Item{ID: 2}
	newItem.WeaponSkillBonuses[proto.WeaponSkillCategory_WeaponSkillCategorySwords] = 6

	character := &Character{Unit: Unit{PseudoStats: stats.NewPseudoStats()}}
	character.Equipment[proto.ItemSlot_ItemSlotHands] = oldItem
	character.addWeaponSkillBonuses(character.Equipment.WeaponSkillBonuses())
	swap := ItemSwap{character: character}
	swap.unEquippedItems[proto.ItemSlot_ItemSlotHands] = newItem

	swap.swapItem(&Simulation{}, proto.ItemSlot_ItemSlotHands, true, false)
	if got := character.weaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategorySwords); got != 6 {
		t.Fatalf("prepull swords bonus = %v, want 6", got)
	}
}

func TestItemSwapIgnoresWeaponSkillBonusesOnZeroIDItems(t *testing.T) {
	validItem := Item{ID: 1}
	validItem.WeaponSkillBonuses[proto.WeaponSkillCategory_WeaponSkillCategoryAxes] = 2
	invalidItem := Item{}
	invalidItem.WeaponSkillBonuses[proto.WeaponSkillCategory_WeaponSkillCategorySwords] = 99

	character := &Character{Unit: Unit{PseudoStats: stats.NewPseudoStats()}}
	character.Equipment[proto.ItemSlot_ItemSlotHands] = validItem
	character.addWeaponSkillBonuses(character.Equipment.WeaponSkillBonuses())
	swap := ItemSwap{character: character}
	swap.unEquippedItems[proto.ItemSlot_ItemSlotHands] = invalidItem

	swap.swapItem(&Simulation{}, proto.ItemSlot_ItemSlotHands, true, false)
	if got := character.weaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryAxes); got != 0 {
		t.Fatalf("axes bonus after invalid swap = %v, want 0", got)
	}
	if got := character.weaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategorySwords); got != 0 {
		t.Fatalf("swords bonus from invalid item = %v, want 0", got)
	}

	swap.swapItem(&Simulation{}, proto.ItemSlot_ItemSlotHands, true, false)
	if got := character.weaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryAxes); got != 2 {
		t.Fatalf("axes bonus after restoring valid item = %v, want 2", got)
	}
	if got := character.weaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategorySwords); got != 0 {
		t.Fatalf("swords bonus after restoring valid item = %v, want 0", got)
	}
}

func TestDisabledRangedSwapUpdatesLiveWandContext(t *testing.T) {
	oldItem := Item{ID: 1, Type: proto.ItemType_ItemTypeRanged, RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeBow}
	oldItem.WeaponSkillBonuses[proto.WeaponSkillCategory_WeaponSkillCategoryBows] = 2
	newItem := Item{ID: 2, Type: proto.ItemType_ItemTypeRanged, RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeWand}
	newItem.WeaponSkillBonuses[proto.WeaponSkillCategory_WeaponSkillCategoryWands] = 6

	character := &Character{Unit: Unit{Type: PlayerUnit, PseudoStats: stats.NewPseudoStats()}}
	character.AutoAttacks.character = character
	character.Equipment[proto.ItemSlot_ItemSlotRanged] = oldItem
	character.addWeaponSkillBonuses(character.Equipment.WeaponSkillBonuses())
	swap := ItemSwap{character: character}
	swap.unEquippedItems[proto.ItemSlot_ItemSlotRanged] = newItem
	spell := &Spell{Unit: &character.Unit, weaponAttackSource: WeaponAttackSourceRanged}

	swap.swapItem(&Simulation{}, proto.ItemSlot_ItemSlotRanged, false, false)
	context := spell.weaponAttackContext()
	if context.classification.skillCategory != proto.WeaponSkillCategory_WeaponSkillCategoryWands || context.weaponSkillBonus != 6 {
		t.Fatalf("post-swap ranged context = %+v, want wands +6", context)
	}
	if got := character.AutoAttacks.Ranged().classification.kind; got != weaponClassificationUnspecified {
		t.Fatalf("disabled ranged cache kind = %d, want untouched", got)
	}
}

func TestDisabledOffHandSwapSeparatesGrantedBonusFromWeaponCategory(t *testing.T) {
	oldItem := Item{ID: 1, Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeDagger, HandType: proto.HandType_HandTypeOffHand}
	oldItem.WeaponSkillBonuses[proto.WeaponSkillCategory_WeaponSkillCategoryDaggers] = 3
	newItem := Item{ID: 2, Type: proto.ItemType_ItemTypeWeapon, WeaponType: proto.WeaponType_WeaponTypeShield, HandType: proto.HandType_HandTypeOffHand}
	newItem.WeaponSkillBonuses[proto.WeaponSkillCategory_WeaponSkillCategoryAxes] = 5

	character := &Character{Unit: Unit{Type: PlayerUnit, PseudoStats: stats.NewPseudoStats()}}
	character.AutoAttacks.character = character
	character.Equipment[proto.ItemSlot_ItemSlotOffHand] = oldItem
	character.addWeaponSkillBonuses(character.Equipment.WeaponSkillBonuses())
	swap := ItemSwap{character: character}
	swap.unEquippedItems[proto.ItemSlot_ItemSlotOffHand] = newItem

	swap.swapItem(&Simulation{}, proto.ItemSlot_ItemSlotOffHand, false, false)
	if got := character.weaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryAxes); got != 5 {
		t.Fatalf("wearer axes bonus = %v, want 5", got)
	}
	context := (&Spell{Unit: &character.Unit, weaponAttackSource: WeaponAttackSourceOffHand}).weaponAttackContext()
	if context.classification.kind != weaponClassificationAbsent || context.classification.skillCategory != proto.WeaponSkillCategory_WeaponSkillCategoryUnspecified {
		t.Fatalf("shield context classification = %+v, want absent weapon context", context.classification)
	}
	if context.weaponSkillBonus != 0 {
		t.Fatalf("shield context bonus = %v, want 0", context.weaponSkillBonus)
	}
}

func TestWeaponAttackContextCarriesEquippedAndFeralBonuses(t *testing.T) {
	character := &Character{Unit: Unit{Type: PlayerUnit, PseudoStats: stats.NewPseudoStats()}}
	character.AutoAttacks.character = character
	character.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{
		ID:         1,
		Type:       proto.ItemType_ItemTypeWeapon,
		WeaponType: proto.WeaponType_WeaponTypeAxe,
		HandType:   proto.HandType_HandTypeTwoHand,
	}
	character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedAxes, 5)

	spell := &Spell{Unit: &character.Unit, weaponAttackSource: WeaponAttackSourceMainHand}
	context := spell.weaponAttackContext()
	if got := context.classification.skillCategory; got != proto.WeaponSkillCategory_WeaponSkillCategoryTwoHandedAxes {
		t.Fatalf("equipped category = %s, want two-handed axes", got)
	}
	if context.weaponSkillBonus != 5 {
		t.Fatalf("equipped bonus = %v, want 5", context.weaponSkillBonus)
	}

	character.AutoAttacks.mh.Weapon = FeralCombatWeapon(Weapon{SwingSpeed: 1})
	character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryFeralCombat, 4)
	context = spell.weaponAttackContext()
	if got := context.classification.skillCategory; got != proto.WeaponSkillCategory_WeaponSkillCategoryFeralCombat {
		t.Fatalf("synthetic category = %s, want feral combat", got)
	}
	if context.weaponSkillBonus != 4 {
		t.Fatalf("feral bonus = %v, want 4", context.weaponSkillBonus)
	}
}

func TestWeaponSkillBonusesAreInertForInheritedOutcomes(t *testing.T) {
	type fixture struct {
		context  weaponAttackContext
		outcomes [8]float64
	}
	newFixture := func(bonus float64) fixture {
		rules := inheritedTBCRuleset()
		character := &Character{Unit: Unit{
			Type:        PlayerUnit,
			Level:       rules.levels.characterLevel,
			stats:       stats.Stats{stats.PhysicalCritPercent: 10},
			PseudoStats: stats.NewPseudoStats(),
		}}
		character.AutoAttacks.character = character
		character.PseudoStats.InFrontOfTarget = true
		character.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{
			ID:         1,
			Type:       proto.ItemType_ItemTypeWeapon,
			WeaponType: proto.WeaponType_WeaponTypeAxe,
			HandType:   proto.HandType_HandTypeOneHand,
		}
		character.addWeaponSkillBonus(proto.WeaponSkillCategory_WeaponSkillCategoryAxes, bonus)
		defender := &Unit{
			Type:        EnemyUnit,
			Level:       rules.levels.defaultBossLevel(),
			PseudoStats: stats.NewPseudoStats(),
		}
		table := newAttackTableWithRuleset(&character.Unit, defender, rules)
		spell := &Spell{
			Unit:                   &character.Unit,
			DefenseType:            DefenseTypeMelee,
			ProcMask:               ProcMaskMelee,
			weaponAttackSource:     WeaponAttackSourceMainHand,
			CritMultiplierPct:      1,
			CritMultiplierAdditive: 0,
		}
		result := &SpellResult{Target: defender, Damage: 100}
		spell.OutcomeExpectedMeleeWhite(nil, result, table)
		return fixture{
			context: spell.weaponAttackContext(),
			outcomes: [8]float64{
				spell.GetPhysicalMissChance(table),
				defender.GetTotalDodgeChanceAsDefender(spell, table),
				defender.GetTotalParryChanceAsDefender(spell, table),
				spell.PhysicalCritChance(table),
				table.BaseGlanceChance,
				table.GlanceMultiplier,
				table.MeleeCritSuppression,
				result.Damage,
			},
		}
	}

	baseline := newFixture(0)
	boosted := newFixture(500)
	if boosted.context.classification.skillCategory != proto.WeaponSkillCategory_WeaponSkillCategoryAxes || boosted.context.weaponSkillBonus != 500 {
		t.Fatalf("boosted context = %+v, want axes +500", boosted.context)
	}
	wantInherited := [8]float64{0.08, 0.065, 0.14, 0.052, 0.24, 0.75, 0.048, 70.7}
	for index, want := range wantInherited {
		assertFloat64(t, "inherited weapon outcome", baseline.outcomes[index], want)
	}
	if boosted.outcomes != baseline.outcomes {
		t.Fatalf("inherited outcomes changed after inactive bonus: before=%v after=%v", baseline.outcomes, boosted.outcomes)
	}
}
