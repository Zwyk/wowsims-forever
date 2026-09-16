package core

import (
	"fmt"
	"time"

	"github.com/wowsims/tbc/sim/core/proto"
	"github.com/wowsims/tbc/sim/core/stats"
)

// This list describes executable effects, not just talent-tree data. Reject
// unimplemented selections before mutating the character.
func (talents classic60PaladinTalents) validateOffensiveSupport() error {
	if err := talents.validate(); err != nil {
		return err
	}
	for talent, rank := range talents {
		if rank == 0 {
			continue
		}
		switch classic60PaladinTalent(talent) {
		case classic60PaladinDivineStrength, classic60PaladinDivineIntellect,
			classic60PaladinHealingLight, classic60PaladinIllumination, classic60PaladinDivineFavor, classic60PaladinImprovedLayOnHands, classic60PaladinSpiritualFocus,
			classic60PaladinImprovedSealOfRighteousness, classic60PaladinLastingJudgement,
			classic60PaladinConsecration, classic60PaladinImprovedBlessingOfWisdom,
			classic60PaladinHolyPower, classic60PaladinHolyShock,
			classic60PaladinImprovedDevotionAura, classic60PaladinPrecision,
			classic60PaladinRedoubt, classic60PaladinAnticipation, classic60PaladinShieldSpecialization,
			classic60PaladinImprovedRighteousFury, classic60PaladinBlessingOfSanctuary, classic60PaladinReckoning,
			classic60PaladinHolyShield, classic60PaladinOneHandedWeaponSpecialization, classic60PaladinImprovedHammerOfJustice,
			classic60PaladinToughness, classic60PaladinBlessingOfKings,
			classic60PaladinImprovedBlessingOfMight, classic60PaladinBenediction,
			classic60PaladinImprovedJudgement, classic60PaladinDeflection,
			classic60PaladinImprovedSealOfTheCrusader, classic60PaladinImprovedRetributionAura, classic60PaladinRepentance,
			classic60PaladinEyeForAnEye,
			classic60PaladinVindication, classic60PaladinPursuitOfJustice, classic60PaladinGuardiansFavor,
			classic60PaladinUnyieldingFaith, classic60PaladinImprovedConcentrationAura,
			classic60PaladinConviction, classic60PaladinSealOfCommand,
			classic60PaladinTwoHandedWeaponSpecialization, classic60PaladinSanctityAura,
			classic60PaladinVengeance:
		default:
			return fmt.Errorf("%s is not implemented in the Classic Paladin combat reference", classic60PaladinTalentDescriptors[talent].name)
		}
	}
	return nil
}

// registerClassic60PaladinBuild assembles a structurally valid, supported
// level-60 talent build. It uses the private Human Paladin combat profile;
// it neither selects public TBC talents nor installs item data.
func registerClassic60PaladinBuild(character *Character, talents classic60PaladinTalents) (*classic60PaladinSpells, error) {
	if err := talents.validateOffensiveSupport(); err != nil {
		return nil, err
	}
	if character == nil || character.resolvedRuleset().id != rulesetClassic60PaladinReference ||
		character.Type != PlayerUnit || character.Level != 60 || character.Race != proto.Race_RaceHuman ||
		character.Class != proto.Class_ClassPaladin || !character.HasManaBar() || !character.AutoAttacks.AutoSwingMelee {
		return nil, fmt.Errorf("Classic Paladin build requires the initialized Human Paladin reference")
	}
	weapon := character.MainHand()
	if character.Env == nil || character.Env.IsFinalized() || character.AutoAttacks.IsDualWielding ||
		(weapon.HandType != proto.HandType_HandTypeTwoHand && weapon.HandType != proto.HandType_HandTypeOneHand && weapon.HandType != proto.HandType_HandTypeMainHand) ||
		(weapon.WeaponType != proto.WeaponType_WeaponTypeSword && weapon.WeaponType != proto.WeaponType_WeaponTypeMace && weapon.WeaponType != proto.WeaponType_WeaponTypeAxe) {
		return nil, fmt.Errorf("Classic Paladin build requires an equipped sword, mace or axe before finalization")
	}
	if character.GetSpell(ActionID{SpellID: 20920}) != nil || character.GetSpell(ActionID{SpellID: 10314}) != nil {
		return nil, fmt.Errorf("Classic Paladin spells are already registered")
	}
	character.MultiplyStat(stats.Strength, 1+.02*float64(talents[classic60PaladinDivineStrength]))
	character.MultiplyStat(stats.Intellect, 1+.02*float64(talents[classic60PaladinDivineIntellect]))
	character.AddStat(stats.PhysicalHitPercent, float64(talents[classic60PaladinPrecision]))
	character.AddStat(stats.PhysicalCritPercent, float64(talents[classic60PaladinConviction]))
	character.ApplyEquipScaling(stats.Armor, 1+.02*float64(talents[classic60PaladinToughness]))
	// Keep parry in percentage form; putting it into a TBC rating slot would
	// require an unaudited defense conversion. Incoming combat has its own
	// source-backed defense table.
	character.PseudoStats.CanParry = true
	character.PseudoStats.BaseParryChance = .05 + .01*float64(talents[classic60PaladinDeflection])
	character.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexPhysical] *= classic60PaladinWeaponMultiplier(character, talents)

	spells := registerClassic60PaladinCommand(character, talents)
	spells.registerOffensiveAbilities(character, talents)
	spells.registerSelfBuffs(character, talents)
	spells.registerSupport(character, talents)
	spells.registerAdditionalSeals(character, talents)
	spells.registerHealingAbilities(character, talents)
	spells.registerClassicPushback(character, talents)
	spells.registerHammerOfWrath(character, talents)
	spells.classic60PaladinDefenseState = spells.registerDefense(character, talents)
	spells.registerMagicDefense(character, talents)
	spells.registerDefensiveCooldowns(character, talents)
	spells.registerCrowdControl(character, talents)
	spells.registerMiscTalents(character, talents)
	spells.registerMobility(character, talents)
	spells.registerVengeance(character, talents)
	return spells, nil
}

func classic60PaladinWeaponMultiplier(character *Character, talents classic60PaladinTalents) float64 {
	if character.MainHand().HandType == proto.HandType_HandTypeTwoHand {
		return 1 + .02*float64(talents[classic60PaladinTwoHandedWeaponSpecialization])
	}
	return 1 + .02*float64(talents[classic60PaladinOneHandedWeaponSpecialization])
}

// Pinned talents.go: any dealt critical result refreshes one eight-second
// damage buff. Refreshing does not stack the multiplier. Damage already
// calculated or snapshotted is not retroactively increased.
func (spells *classic60PaladinSpells) registerVengeance(character *Character, talents classic60PaladinTalents) {
	if talents[classic60PaladinVengeance] == 0 {
		return
	}
	multiplier := 1 + .03*float64(talents[classic60PaladinVengeance])
	spells.VengeanceAura = character.RegisterAura(Aura{
		Label: "Vengeance Proc", ActionID: ActionID{SpellID: 20059}, Duration: 8 * time.Second,
		OnGain: func(_ *Aura, _ *Simulation) {
			character.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexPhysical] *= multiplier
			character.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly] *= multiplier
		},
		OnExpire: func(_ *Aura, _ *Simulation) {
			character.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexPhysical] /= multiplier
			character.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly] /= multiplier
		},
	})
	MakePermanent(character.RegisterAura(Aura{
		Label: "Vengeance Trigger",
		OnSpellHitDealt: func(_ *Aura, sim *Simulation, _ *Spell, result *SpellResult) {
			if result.DidCrit() {
				spells.VengeanceAura.Activate(sim)
			}
		},
	}))
}
