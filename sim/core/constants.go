package core

import (
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
)

// Compatibility constants for call sites which need compile-time values.
// The active ruleset selector and these aliases must move together.
const CharacterLevel = activeCharacterLevel
const DefaultBossLevel = CharacterLevel + activeDefaultBossLevelDelta
const MinIlvl = 60
const MaxIlvl = 600

const GCDMin = time.Second * 1
const GCDDefault = time.Millisecond * 1500
const BossGCD = time.Millisecond * 1620
const MaxSpellQueueWindow = time.Millisecond * 400
const SpellBatchWindow = time.Millisecond * 10
const PetUpdateInterval = time.Millisecond * 5250
const SpellPushbackDuration = time.Millisecond * 500
const MaxMeleeRange = 5.0 // in yards

const DefaultAttackPowerPerDPS = 14.0

const ArmorPenPerPercentArmor = 5.92

// Compatibility constants for rating and defense consumers outside core.
// Their canonical values live in the active build-time ruleset profile.
const (
	ExpertisePerQuarterPercentReduction     = activeExpertisePerQuarterPercentReduction
	DefenseRatingPerDefenseLevel            = activeDefenseRatingPerDefenseLevel
	DodgeRatingPerDodgePercent              = activeDodgeRatingPerDodgePercent
	ParryRatingPerParryPercent              = activeParryRatingPerParryPercent
	BlockRatingPerBlockPercent              = activeBlockRatingPerBlockPercent
	PhysicalHitRatingPerHitPercent          = activePhysicalHitRatingPerHitPercent
	SpellHitRatingPerHitPercent             = activeSpellHitRatingPerHitPercent
	PhysicalCritRatingPerCritPercent        = activePhysicalCritRatingPerCritPercent
	SpellCritRatingPerCritPercent           = activeSpellCritRatingPerCritPercent
	PhysicalHasteRatingPerHastePercent      = activePhysicalHasteRatingPerHastePercent
	SpellHasteRatingPerHastePercent         = activeSpellHasteRatingPerHastePercent
	MissDodgeParryBlockCritChancePerDefense = activeDefenseChancePerDefenseLevelPercent
	ResilienceRatingPerCritReductionChance  = activeResilienceRatingPerCritReductionPercent
)

const EnemyAutoAttackAPCoefficient = 0.00052

// IDs for items used in core
// const ()

type Hand bool

const MainHand Hand = true
const OffHand Hand = false

const CombatTableCoverageCap = 1.024 // 102.4% chance to avoid an attack

const NumItemSlots = proto.ItemSlot_ItemSlotRanged + 1

func TrinketSlots() []proto.ItemSlot {
	return []proto.ItemSlot{proto.ItemSlot_ItemSlotTrinket1, proto.ItemSlot_ItemSlotTrinket2}
}

func AllWeaponSlots() []proto.ItemSlot {
	return []proto.ItemSlot{proto.ItemSlot_ItemSlotMainHand, proto.ItemSlot_ItemSlotOffHand, proto.ItemSlot_ItemSlotRanged}
}

func AllMeleeWeaponSlots() []proto.ItemSlot {
	return []proto.ItemSlot{proto.ItemSlot_ItemSlotMainHand, proto.ItemSlot_ItemSlotOffHand}
}
