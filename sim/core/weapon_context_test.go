package core

import (
	"testing"

	"github.com/wowsims/tbc/sim/core/proto"
)

func TestWeaponAttackSourceZeroValueIsUnspecified(t *testing.T) {
	var source WeaponAttackSource
	if source != WeaponAttackSourceUnspecified {
		t.Fatalf("zero source = %d, want unspecified", source)
	}
	if source == WeaponAttackSourceNone {
		t.Fatal("zero source must not mean explicitly none")
	}

	spell := (&Unit{}).RegisterSpell(SpellConfig{})
	if spell.weaponAttackSource != WeaponAttackSourceUnspecified {
		t.Fatalf("omitted source = %d, want unspecified", spell.weaponAttackSource)
	}
}

func TestWeaponAttackSourceRegistrationIsExplicit(t *testing.T) {
	tests := []struct {
		name     string
		source   WeaponAttackSource
		procMask ProcMask
	}{
		{name: "unspecified with ranged proc mask", source: WeaponAttackSourceUnspecified, procMask: ProcMaskRangedSpecial},
		{name: "explicit none with ranged proc mask", source: WeaponAttackSourceNone, procMask: ProcMaskRangedSpecial},
		{name: "main hand with mixed proc mask", source: WeaponAttackSourceMainHand, procMask: ProcMaskMeleeMH | ProcMaskSpellDamage},
		{name: "off hand", source: WeaponAttackSourceOffHand, procMask: ProcMaskMeleeOH},
		{name: "ranged", source: WeaponAttackSourceRanged, procMask: ProcMaskProc},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unit := &Unit{}
			spell := unit.RegisterSpell(SpellConfig{
				ProcMask:           test.procMask,
				WeaponAttackSource: test.source,
			})

			if spell.weaponAttackSource != test.source {
				t.Fatalf("weapon source = %d, want %d", spell.weaponAttackSource, test.source)
			}
			if spell.ProcMask != test.procMask {
				t.Fatalf("proc mask = %d, want %d", spell.ProcMask, test.procMask)
			}
		})
	}
}

func TestAutoAttackWeaponSourcesDoNotDependOnProcMask(t *testing.T) {
	agent := &FakeAgent{}
	sharedProcMask := ProcMaskRangedSpecial | ProcMaskSpellDamage
	agent.EnableAutoAttacks(agent, AutoAttackOptions{
		MainHand: Weapon{
			SwingSpeed:        2.6,
			CritMultiplier:    2,
			AttackPowerPerDPS: DefaultAttackPowerPerDPS,
		},
		ProcMask: sharedProcMask,
	})

	tests := []struct {
		name   string
		config *SpellConfig
		want   WeaponAttackSource
	}{
		{name: "main hand", config: agent.AutoAttacks.MHConfig(), want: WeaponAttackSourceMainHand},
		{name: "empty off hand", config: agent.AutoAttacks.OHConfig(), want: WeaponAttackSourceOffHand},
		{name: "empty ranged", config: agent.AutoAttacks.RangedConfig(), want: WeaponAttackSourceRanged},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.config.WeaponAttackSource != test.want {
				t.Fatalf("weapon source = %d, want %d", test.config.WeaponAttackSource, test.want)
			}
			if test.config.ProcMask != sharedProcMask {
				t.Fatalf("proc mask = %d, want %d", test.config.ProcMask, sharedProcMask)
			}
		})
	}

	if got := agent.AutoAttacks.MH().classification.kind; got != weaponClassificationSynthetic {
		t.Fatalf("abstract main-hand kind = %d, want synthetic", got)
	}
	if got := agent.AutoAttacks.OH().classification.kind; got != weaponClassificationAbsent {
		t.Fatalf("empty off-hand kind = %d, want absent", got)
	}
	if got := agent.AutoAttacks.Ranged().classification.kind; got != weaponClassificationAbsent {
		t.Fatalf("empty ranged kind = %d, want absent", got)
	}

	// Windfury-style extra attacks copy the auto-attack config before
	// registering a tagged replacement. The explicit source must survive.
	copiedConfig := *agent.AutoAttacks.MHConfig()
	copiedConfig.ActionID = copiedConfig.ActionID.WithTag(25584)
	copiedSpell := agent.GetOrRegisterSpell(copiedConfig)
	if copiedSpell.weaponAttackSource != WeaponAttackSourceMainHand {
		t.Fatalf("copied config weapon source = %d, want %d", copiedSpell.weaponAttackSource, WeaponAttackSourceMainHand)
	}
	if reusedSpell := agent.GetOrRegisterSpell(copiedConfig); reusedSpell != copiedSpell {
		t.Fatal("tagged auto-attack config was not reused")
	}
}

func TestWeaponFromItemPreservesClassification(t *testing.T) {
	tests := []struct {
		name string
		item Item
	}{
		{
			name: "two-handed sword",
			item: Item{
				ID:         1,
				WeaponType: proto.WeaponType_WeaponTypeSword,
				HandType:   proto.HandType_HandTypeTwoHand,
				SwingSpeed: 3.4,
			},
		},
		{
			name: "bow",
			item: Item{
				ID:               2,
				RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeBow,
				SwingSpeed:       2.8,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			weapon := newWeaponFromItem(&test.item, 2, 0)
			want := weaponClassification{
				kind:             weaponClassificationEquippedItem,
				weaponType:       test.item.WeaponType,
				handType:         test.item.HandType,
				rangedWeaponType: test.item.RangedWeaponType,
			}
			if weapon.classification != want {
				t.Fatalf("classification = %+v, want %+v", weapon.classification, want)
			}
		})
	}
}

func TestWeaponClassificationRequiresValidItem(t *testing.T) {
	want := weaponClassification{kind: weaponClassificationAbsent}
	if got := weaponClassificationFromItem(nil); got != want {
		t.Fatalf("nil item classification = %+v, want absent", got)
	}
	if got := weaponClassificationFromItem(&Item{WeaponType: proto.WeaponType_WeaponTypeSword}); got != want {
		t.Fatalf("zero-ID item classification = %+v, want absent", got)
	}
}

func TestWeaponClassificationZeroValueIsUnspecified(t *testing.T) {
	var classification weaponClassification
	if classification.kind != weaponClassificationUnspecified {
		t.Fatalf("zero classification kind = %d, want unspecified", classification.kind)
	}
	if classification.kind == weaponClassificationAbsent {
		t.Fatal("zero classification must not mean known absent")
	}
}

func TestSetWeaponMarksAbstractWeaponSynthetic(t *testing.T) {
	attack := WeaponAttack{spell: &Spell{}, curSwingSpeed: 1}
	attack.setWeapon(Weapon{SwingSpeed: 1})
	if got := attack.classification.kind; got != weaponClassificationSynthetic {
		t.Fatalf("replacement weapon kind = %d, want synthetic", got)
	}
}

func TestWeaponAttackContextUsesCurrentWeapon(t *testing.T) {
	mh := weaponClassification{kind: weaponClassificationEquippedItem, weaponType: proto.WeaponType_WeaponTypeSword}
	oh := weaponClassification{kind: weaponClassificationEquippedItem, weaponType: proto.WeaponType_WeaponTypeDagger}
	ranged := weaponClassification{kind: weaponClassificationEquippedItem, rangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeBow}
	unit := &Unit{}
	unit.AutoAttacks.mh.Weapon.classification = mh
	unit.AutoAttacks.oh.Weapon.classification = oh
	unit.AutoAttacks.ranged.Weapon.classification = ranged

	tests := []struct {
		name   string
		source WeaponAttackSource
		want   weaponClassification
	}{
		{name: "unspecified", source: WeaponAttackSourceUnspecified, want: weaponClassification{}},
		{name: "none", source: WeaponAttackSourceNone, want: weaponClassification{}},
		{name: "main hand", source: WeaponAttackSourceMainHand, want: mh},
		{name: "off hand", source: WeaponAttackSourceOffHand, want: oh},
		{name: "ranged", source: WeaponAttackSourceRanged, want: ranged},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context := (&Spell{Unit: unit, weaponAttackSource: test.source}).weaponAttackContext()
			if context.source != test.source {
				t.Fatalf("source = %d, want %d", context.source, test.source)
			}
			if context.classification != test.want {
				t.Fatalf("classification = %+v, want %+v", context.classification, test.want)
			}
		})
	}

	beforeReplacement := (&Spell{Unit: unit, weaponAttackSource: WeaponAttackSourceMainHand}).weaponAttackContext()
	replacement := weaponClassification{kind: weaponClassificationEquippedItem, weaponType: proto.WeaponType_WeaponTypeMace}
	unit.AutoAttacks.mh.Weapon = Weapon{classification: replacement, SwingSpeed: 2}
	afterReplacement := (&Spell{Unit: unit, weaponAttackSource: WeaponAttackSourceMainHand}).weaponAttackContext()

	if beforeReplacement.classification != mh {
		t.Fatalf("original snapshot changed to %+v", beforeReplacement.classification)
	}
	if afterReplacement.classification != replacement {
		t.Fatalf("replacement classification = %+v, want %+v", afterReplacement.classification, replacement)
	}
}

func TestWeaponAttackContextReadsLivePlayerEquipment(t *testing.T) {
	character := &Character{Unit: Unit{Type: PlayerUnit}}
	character.AutoAttacks.character = character
	cachedItem := Item{ID: 1, WeaponType: proto.WeaponType_WeaponTypeSword, SwingSpeed: 2.6}
	character.AutoAttacks.mh.Weapon = newWeaponFromItem(&cachedItem, 2, 0)

	currentItem := Item{ID: 2, WeaponType: proto.WeaponType_WeaponTypeMace, HandType: proto.HandType_HandTypeTwoHand, SwingSpeed: 3.6}
	character.Equipment[proto.ItemSlot_ItemSlotMainHand] = currentItem
	context := (&Spell{Unit: &character.Unit, weaponAttackSource: WeaponAttackSourceMainHand}).weaponAttackContext()
	want := weaponClassificationFromItem(&currentItem)
	if context.classification != want {
		t.Fatalf("live main-hand classification = %+v, want %+v", context.classification, want)
	}
	character.AutoAttacks.mh.Weapon = newWeaponFromUnarmed(2)
	context = (&Spell{Unit: &character.Unit, weaponAttackSource: WeaponAttackSourceMainHand}).weaponAttackContext()
	if context.classification != want {
		t.Fatalf("equipped main hand after unarmed cache = %+v, want %+v", context.classification, want)
	}
	character.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{}
	context = (&Spell{Unit: &character.Unit, weaponAttackSource: WeaponAttackSourceMainHand}).weaponAttackContext()
	wantUnarmed := weaponClassification{kind: weaponClassificationUnarmed}
	if context.classification != wantUnarmed {
		t.Fatalf("empty main hand classification = %+v, want %+v", context.classification, wantUnarmed)
	}

	bow := Item{ID: 3, RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeBow, SwingSpeed: 2.8}
	character.Equipment[proto.ItemSlot_ItemSlotRanged] = bow
	rangedContext := (&Spell{Unit: &character.Unit, weaponAttackSource: WeaponAttackSourceRanged}).weaponAttackContext()
	wantRanged := weaponClassificationFromItem(&bow)
	if rangedContext.classification != wantRanged {
		t.Fatalf("unpopulated ranged classification = %+v, want %+v", rangedContext.classification, wantRanged)
	}
}

func TestWeaponAttackContextFindsPlayerWithoutAutoAttacks(t *testing.T) {
	agent := &FakeAgent{}
	agent.Character.Unit.Type = PlayerUnit
	bow := Item{ID: 1, RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeBow, SwingSpeed: 2.8}
	agent.Character.Equipment[proto.ItemSlot_ItemSlotRanged] = bow

	raid := &Raid{}
	party := &Party{Raid: raid, PlayersAndPets: []Agent{agent}}
	raid.Parties = []*Party{party}
	agent.Character.Unit.Env = &Environment{Raid: raid}

	context := (&Spell{Unit: &agent.Character.Unit, weaponAttackSource: WeaponAttackSourceRanged}).weaponAttackContext()
	want := weaponClassificationFromItem(&bow)
	if context.classification != want {
		t.Fatalf("environment player classification = %+v, want %+v", context.classification, want)
	}
}

func TestWeaponAttackContextKeepsSyntheticWeaponAuthoritative(t *testing.T) {
	character := &Character{Unit: Unit{Type: PlayerUnit}}
	character.AutoAttacks.character = character
	character.Equipment[proto.ItemSlot_ItemSlotMainHand] = Item{
		ID:         1,
		WeaponType: proto.WeaponType_WeaponTypeSword,
		SwingSpeed: 2.6,
	}
	character.AutoAttacks.mh.Weapon = Weapon{SwingSpeed: 1}
	character.AutoAttacks.mh.Weapon.normalizeClassification()

	context := (&Spell{Unit: &character.Unit, weaponAttackSource: WeaponAttackSourceMainHand}).weaponAttackContext()
	if context.source != WeaponAttackSourceMainHand {
		t.Fatalf("source = %d, want %d", context.source, WeaponAttackSourceMainHand)
	}
	want := weaponClassification{kind: weaponClassificationSynthetic}
	if context.classification != want {
		t.Fatalf("classification = %+v, want %+v", context.classification, want)
	}
}

func TestWeaponAttackContextKeepsUnarmedDistinctFromMissing(t *testing.T) {
	character := &Character{Unit: Unit{Type: PlayerUnit}}
	character.AutoAttacks.character = character
	character.AutoAttacks.mh.Weapon = newWeaponFromUnarmed(2)

	context := (&Spell{Unit: &character.Unit, weaponAttackSource: WeaponAttackSourceMainHand}).weaponAttackContext()
	want := weaponClassification{kind: weaponClassificationUnarmed}
	if context.classification != want {
		t.Fatalf("classification = %+v, want %+v", context.classification, want)
	}
}

func TestWeaponAttackContextDoesNotGivePlayerEquipmentToPets(t *testing.T) {
	player := &Character{Unit: Unit{Type: PlayerUnit}}
	player.Equipment[proto.ItemSlot_ItemSlotRanged] = Item{
		ID:               1,
		RangedWeaponType: proto.RangedWeaponType_RangedWeaponTypeBow,
		SwingSpeed:       2.8,
	}
	pet := &Unit{Type: PetUnit}
	pet.AutoAttacks.character = player

	context := (&Spell{Unit: pet, weaponAttackSource: WeaponAttackSourceRanged}).weaponAttackContext()
	if context.classification != (weaponClassification{}) {
		t.Fatalf("pet classification = %+v, want unspecified", context.classification)
	}
}
