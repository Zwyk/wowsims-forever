# Forever core rules ledger

This document separates announced Forever behavior from assumptions inherited from TBC or Classic. Numeric rules stay provisional until they can be measured against a Forever client build.

## Baseline provenance

The modern architecture baseline is [`wowsims/tbc-new` v0.0.137](https://github.com/wowsims/tbc-new/tree/17a8fb28c5ad14b649acecdaacd488594048f467), commit `17a8fb28c5ad14b649acecdaacd488594048f467`. The first green Forever integration baseline is commit `6cff8086e8e2550856288d71cc9d72bee3e60e4a`.

The fork is synchronized with upstream through commit [`9fa04e0675354c1fa2167b83171bbfce5df492ef`](https://github.com/wowsims/tbc-new/commit/9fa04e0675354c1fa2167b83171bbfce5df492ef), including the data-driven `DefenseType` and weapon-proc helpers from upstream PR [#520](https://github.com/wowsims/tbc-new/pull/520). The synchronization preserves upstream commit ancestry so later updates do not replay this change set.

These references identify inherited behavior; they are not evidence that a rule is correct for Forever. Changes to provisional numbers should cite client measurements or an authoritative announcement and update the associated characterization test in the same pull request.

## Characterization coverage

| Surface | Baseline guard |
| --- | --- |
| Player and boss levels | `TestInheritedTBCLevelBaseline` |
| Player-to-enemy and enemy-to-player combat tables | `TestInheritedPlayerVsEnemyAttackTables`, `TestInheritedEnemyVsPlayerAttackTables` |
| Hit floors, crit suppression, white/yellow attacks, facing and dual wield | `inherited_outcomes_test.go` |
| Expertise rounding and separate dodge reduction | `attack_table_test.go` |
| Shared and scoped Hit/Crit rating conversion | `unified_stats_test.go` |
| Armor, armor penetration and the 75% cap | `armor_test.go` |
| Binary and non-binary resistance | `resistance_test.go` |
| Baseline health, armor, mana, AP and attribute-derived crit; player/pet boundaries | `attribute_rules_test.go` |
| Integrated pre-racial level-60 initialization for all 40 original combinations | `character_classic_reference_test.go`, `character_classic_reference_integration_test.go` |
| End-to-end class behavior | Existing seeded per-class `.results` files |

The per-class golden results are the broad regression layer. Do not regenerate them merely to make CI green: a result update must accompany a reviewed mechanics change and explain why each affected class moved.

## Core ruleset boundary

The active core ruleset is selected at build time and remains internal to the Go engine. It contains value-only level, rating, combat-outcome and baseline attribute-conversion data; simulations cannot mutate it or choose a different ruleset through proto input. Existing public constants and constructors remain compatibility facades while call sites are migrated incrementally.

The first extraction contains the inherited TBC level bands and both attack-table directions. The second adds combat-rating conversions, level-based NPC critical strike chance, and fixed outcome values such as expertise steps, the dual-wield miss penalty, the spell miss floor, and enemy critical/crushing damage multipliers. Engine-created attack tables cache value copies of the selected rating and outcome rules, providing a profile-local calculation seam without mutable global state.

All extracted values still reproduce the inherited TBC engine exactly. Armor, resistance, weapon skill, class base stats, talents and UI values stay outside this profile until each surface can move in a focused pull request. A later Forever profile will be added alongside the inherited profile, then activated only with matching level-60 base-stat data and UI changes.

### Baseline attribute-conversion boundary

`attribute_rules.go` installs baseline dependencies from a value-only profile: Stamina-to-health, Agility-to-armor, player Intellect-to-mana/spell-crit and class-specific Strength/Agility-to-AP/physical-crit. The inherited TBC profile retains every current coefficient. Each class calls the shared AP/crit installer from its existing construction hook; mana setup remains in the mana-bar helper. Dependencies capture the values at installation, and all three internal injection paths use the supplied profile rather than consulting active globals again. The generated crit maps remain unchanged for compatibility and pet consumers, with parity tests guarding the duplicate inherited values.

The health correction is explicitly player-only and remains zero in production; the mana correction remains `-280`, before resource multipliers. `BaseMana` still comes from the unmodified base-stat row for spell costs. Pets retain universal `10 × Stamina` health and `2 × Agility` armor, but receive no player mana, AP or crit dependencies. Attribute flooring stays in the existing dependency manager. Dodge/block, flat AP corrections, racials, talents, regeneration and form/pet scaling are deliberately not moved.

This extraction does not activate the Classic reference or fix inherited quirks: TBC Mage/Priest still have no Strength-to-AP dependency, Rogue still has no Agility-to-ranged-AP dependency, Shaman retains its separate `-20` AP correction, and Druid retains baseline `1 × Strength` plus its existing form-specific additions. The pinned Classic constructors differ from both these hooks and some of their own lookup tables: Hunter uses `2 × Agility` ranged AP, Mage/Priest omit Agility-to-crit, and Druid starts with `2 × Strength`. The inactive reference below now characterizes these coefficients, but matching constructor/form updates are still required before level-60 activation; replacing coefficient tables alone would create a hybrid engine.

### Reviewed inactive facts

`tools/foreverdata/core_rules.go` records the narrow global claims accepted from Blizzard's announcements as typed, value-only facts with a document, section locator, evidence basis, and review state. The tool is a `package main`, so the simulation cannot import the catalog. “Reviewed inactive” means only that a claim and its provenance passed review; it does not make that claim runtime behavior.

The catalog currently contains six claims: the announced leveling journey has an upper bound of 60; spell, melee, and ranged Hit chances are combined, as are their Crit chances; weapon skill remains relevant; announced item effects can reduce parry chance or dodge chance; and bonus healing includes exactly 1/3 as much bonus damage. Channel sets record only the chance categories named together by Blizzard; they do not claim a storage model, a conversion formula, or merged outcome tables. The ratio is stored as an exact rational value rather than a floating-point approximation.

Unknown values are omitted rather than copied from Classic or TBC. In particular, the catalog does not contain a default boss level, rating conversions, attack tables, caps, rounding rules, base stats, armor or resistance formulas, or weapon-skill effects. Promoting any fact into `sim/core` requires a separate reviewed change and a complete runtime profile for the affected surface.

### Weapon attack context

Spells can declare a currently inert weapon source: none, main hand, off hand or ranged. The zero value is a separate unspecified state, so a future activation audit can distinguish intentionally non-weapon spells from missing annotations. This source is separate from `ProcMask`, because proc routing and hit-table categories do not reliably identify the weapon used. Core auto attacks set it explicitly, and item-backed `Weapon` values retain the equipped weapon, hand and ranged classifications whenever they are built or rebuilt from equipment. Class abilities remain unspecified until they can be audited before weapon-skill mechanics are activated.

Upstream's `DefenseType` is orthogonal to weapon source: it selects the physical, ranged or magical outcome table and base critical multiplier from client spell-category data, but it does not say which equipped weapon supplies skill context. Auto attacks therefore carry both fields independently. The inherited 1.5 magical and 2.0 melee/ranged critical multipliers live in the rules profile rather than outcome code, ready for evidence-backed Forever values.

Shared proc-damage configs preserve whether `DefenseType` was present in client data. A present category zero remains `None` and defaults to an always-land, non-critical outcome, matching the Classic and SoD weapon-proc helpers. An omitted zero retains the legacy school/`IsMelee` inference, while nonzero values remain unambiguously explicit. This distinction is carried from the `SpellCategories` database row through generated proc configs so missing data is not silently reclassified as an explicit rule.

The engine continues to keep one mutable `AttackTable` per attacker/defender pair. It does not copy Classic's per-cast-type table maps, which would duplicate pair-wide aura state. Future weapon-skill rules will resolve a short-lived context from the spell, the current player equipment and any authoritative synthetic auto-attack weapon instead.

Weapon context distinguishes unspecified, absent, equipped-item, unarmed and synthetic origins. Player equipment classification is read live, including for disabled or unpopulated auto-attack channels, while a form, pet or other synthetic weapon never falls back to an equipped item.

### Weapon-skill categories and bonuses

The engine carries an inactive, fixed-size weapon-skill bonus vector. Its category indices 0-15 intentionally preserve the Classic WoWSims layout. Wands is a forward-compatible engine skill category at index 16 because it is distinct from the imported ranged categories; that skill category and lookup case are absent from the Classic reference simulator, and its presence here is not a claim about confirmed Forever behavior. Item transport, character aggregation and item-swap deltas all preserve these values independently of regular stats. Fist weapons map to Unarmed, and cat/bear weapons are explicitly tagged as Feral Combat. Missing, malformed, non-weapon, wrong-slot and unclassified synthetic contexts fail closed to the unspecified category.

These values are bonuses only: the engine does not yet define whether Forever expresses them as skill points, rating, percentage points or some other unit. Nothing converts them to inherited Expertise Rating, and no production outcome reads them; only the inactive Classic reference policy below can project their possible effects. Current item data also leaves every bonus at zero; populating a checked, provenance-backed Forever data overlay is a separate step.

### Classic level-60 base-attribute reference

`base_attributes_classic_reference.go` contains a pure, inactive lookup of the five starting attributes, base health/mana and melee/ranged attack-power offsets from [`wowsims/classic` commit `7779ebbf79dc7f1341e6ab939b28a3402c9a730a`](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/base_stats.go). It returns a private value record, not a runtime-ready `stats.Stats`. Literal tests cover all 40 original race/class combinations from the source's [eligible-race table](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/ui/core/proto_utils/utils.ts#L1030-L1038). Unsupported levels, races, classes and combinations return no data, even when separate race and class rows exist. This is a deliberate scope restriction absent from the source's permissive map lookup, not a restriction on Forever's eventual race picker.

These are raw simulator table values, not final naked-character stats or verified Forever measurements. They exclude base crit/dodge, all attribute conversions, racial multipliers, rounding, regeneration, forms and pets. In particular, Classic's [health dependencies](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/character.go#L281-L284) add `10 × Stamina − 180`; the inherited TBC constructor adds `10 × Stamina` without that subtraction. Both mana paths add `15 × Intellect − 280` for mana users; these formulas assume attributes of at least 20. TBC also installs Intellect-to-spell-crit centrally, while Classic's classes install it individually. Copying only the table into the live engine would therefore mix incompatible initialization paths. The conversion and integrated initialization references below now assemble baseline resources, AP, crit and avoidance; runnable constructor/resource/form wiring is still separate. Existing `BaseStats`, `ClassBaseStats`, generated values and runtime base-stat selection remain unchanged; the baseline conversion boundary above preserves their inherited behavior.

The [ElliotWood/Forever base-stat file at `17bd621723f827e0cb822573e09b55a31988f3dc`](https://github.com/ElliotWood/Forever/blob/17bd621723f827e0cb822573e09b55a31988f3dc/sim/core/base_stats.go) has the same inherited rows, with additional zero-offset Skyborne placeholders. That comparison confirms shared ancestry, not independent validation; this reference does not import those placeholders or extrapolate the new Forever combinations.

### Classic level-60 attribute-conversion reference

`attribute_rules_classic_reference.go` returns a fresh, inactive set of the baseline dependencies actually installed by the nine classes in [`wowsims/classic` commit `7779ebbf79dc7f1341e6ab939b28a3402c9a730a`](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/base_stats.go#L133-L182). It accepts only character level 60, independently of boss-level policy. The source's maps are not sufficient on their own: [Hunter](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/hunter/hunter.go#L279-L283) installs melee AP from Agility despite its map entry being zero, while [Mage](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/mage/mage.go#L134-L137) and [Priest](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/priest/priest.go#L115-L118) omit their populated physical-crit conversion rows. This reference preserves those implementation omissions; it does not endorse them as game behavior.

| Class | AP / Strength | AP / Agility | Ranged AP / Agility | Physical crit percentage points / Agility | Spell crit percentage points / Intellect |
| --- | ---: | ---: | ---: | ---: | ---: |
| Warrior | 2 | 0 | 0 | 0.0500 | 0 |
| Paladin | 2 | 0 | 0 | 0.0506 | 0.0167 |
| Hunter | 1 | 1 | 2 | 0.0189 | 0.0165 |
| Rogue | 1 | 1 | 1 | 0.0345 | 0 |
| Priest | 1 | 0 | 0 | 0 | 0.0168 |
| Shaman | 2 | 0 | 0 | 0.0508 | 0.0169 |
| Mage | 1 | 0 | 0 | 0 | 0.0168 |
| Warlock | 1 | 0 | 0 | 0.0500 | 0.0165 |
| Druid, before form additions | 2 | 0 | 0 | 0.0500 | 0.0167 |

The resource coefficients are `10 × Stamina − 180` health, `2 × Agility` armor and `15 × Intellect − 280` mana. Mana setup applies only to the seven source mana-user classes, not Warrior/Rogue; modified pet mana formulas are excluded. Crit coefficients already express percentage points, with no TBC rating divisor. The source's separate `ClassBaseCrit` is not included again. Race/class eligibility still belongs to the raw base-attribute lookup; a class coefficient row does not establish a new Forever combination.

A second private helper characterizes only the additional AP wiring in executable [Classic Cat Form](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/druid/forms.go#L86-L95): `+120 AP`, `+1 AP / Agility` and `+1 AP / already-resolved FeralAttackPower`. It has no additional Strength conversion because the baseline already supplies `2 AP / Strength`. Tests compose raw Night Elf and Tauren rows with that baseline and toggle the AP dependencies through humanoid → Cat → humanoid → Cat: the respective no-gear/no-talent AP values are `104 ↔ 289` and `120 ↔ 295`. These are dependency tests, not a running Classic form or aura implementation; talents, equipment-derived FeralAP, resource bars, weapons, armor, threat and shifting behavior remain excluded.

Bear is intentionally unresolved. The pinned Classic [Bear implementation is commented out](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/druid/forms.go#L222-L359), including stale talent references. [Elliot's active implementation at `17bd621`](https://github.com/ElliotWood/Forever/blob/17bd621723f827e0cb822573e09b55a31988f3dc/sim/druid/forms.go#L242-L260) adds flat `1240 Health`, whereas the inherited TBC form multiplies Stamina by `1.25`. That is a discrepancy to investigate, not a formula to import automatically. No generic form lookup or Bear fallback is added.

Both helpers remain inactive and leave the level-70 selector and all live class/form code unchanged. The baseline initializer below consumes the attribute rules; Cat AP is composed in integration tests only. Literal vectors use integer primary attributes: [Classic's dependency manager](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/stats/deps.go#L237-L248) consumes fractional sources directly, while our modern manager floors primary attributes before conversion. Full fractional-stat rounding parity, base crit/dodge, defense-family dependencies, racials, talents, regeneration, pets and client/Forever verification are outside the attribute-conversion helper itself.

### Integrated pre-racial level-60 character initialization

`classicReferenceInitializeCharacter` now assembles a single baseline for every original race/class pair. It resolves the raw attributes/resources/AP, adds [source `ClassBaseCrit`](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/base_stats.go#L84-L130) once, installs the source-characterized conversions through the existing modern dependency installers, and returns raw base stats, resolved stats, spell-cost base mana, mana capability and typed avoidance values. Only level 60 and the 40 explicitly supported combinations are accepted; invalid input returns no partial baseline. The initializer constructs its own dependency-only rules instead of copying inherited TBC combat or defense policy. Warrior/Rogue do not acquire a mana bar or the `-280` mana correction.

This result is a private value-only snapshot, not a `Character`, `Unit` or combat agent. A temporary local character is used solely to install dependencies; neither it nor its dependency graph escapes. This boundary matters because live constructors, racial effects, resource setup and public avoidance getters still select TBC rules. There is no API or UI route to run the snapshot as a partially converted simulation.

The source-characterized baseline chance data is stored separately from rating transport:

| Class | Base physical crit % | Base spell crit % | Base dodge % | Installed dodge percentage points / Agility | Baseline parry capability |
| --- | ---: | ---: | ---: | ---: | --- |
| Warrior | 0 | 0 | 0 | 0.0500 | Yes |
| Paladin | 0.7 | 3.5 | 0.7 | 0.0506 | Yes |
| Hunter | 0 | 3.6 | 0 | 0 | Yes |
| Rogue | 0 | 0 | 0 | 0.0690 | Yes |
| Priest | 3 | 0.8 | 3 | 0 | No |
| Shaman | 1.7 | 2.3 | 1.7 | 0.0508 | No, talent required |
| Mage | 3.2 | 0.2 | 3.2 | 0 | No |
| Warlock | 2 | 1.7 | 2 | 0.0500 | No |
| Druid | 0.9 | 1.8 | 0.9 | 0.0500 | No |

Hunter, Priest and Mage omit their populated dodge conversion rows in the executable constructors. The baseline intentionally reproduces these omissions, not a claim that those classes gain no dodge from Agility in the game. Universal [raw Parry and Block seeds are 5%](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/character.go#L280-L290), but the result keeps capability flags separate: usable parry is zero without class capability, and usable block is zero for every unequipped baseline. These are percentage points before opponent-level, casting, stun and other combat-state effects, not complete incoming avoidance probabilities. No Classic percentage is put into a TBC defensive-rating slot. Bonus Defense is zero here; it is not populated with the trained-defense total of 300.

Independent literal tests check full raw/resolved stat arrays for all 40 combinations, resource/capability gates, invalid enums and levels, value-copy isolation and unchanged active globals/level-70 construction. For example, the pre-racial Human Paladin baseline has 2,201 health, 2,282 mana (1,512 spell-cost base mana), 370 AP, 3.989% physical crit, 4.669% spell crit and 3.989% dodge. These are source-code regression anchors, not a complete naked character sheet or accepted Forever data. Integration tests feed assembled Druid baselines into reversible Cat AP dependencies and assembled armor/resistance values into the separate Classic mitigation projections; they still do not create a live fight.

The boundary is deliberately **before racials** as well as gear, bonus stats, talents, forms, pets and regeneration. Original race attribute offsets are included, but Human Spirit, Gnome Intellect, Tauren Health, Night Elf Dodge and racial resistances are not. [Classic applies those effects later](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/racials.go); calling our live TBC racial code would also introduce different rules. In particular, fractional Gnome Intellect exposes the unresolved source-rounding difference. Completing a runnable level-60 path requires explicit racial/rounding policy, profile-consistent constructor/class/resource/avoidance ownership, and matching combat/form/pet data—not only changing the level constant.

### Classic level-60 offensive chance-input reference

The engine now compiles an inactive overlay for the four well-supported Hit and Crit conversions in [`wowsims/classic` commit `7779ebbf79dc7f1341e6ab939b28a3402c9a730a`](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/base_stats_auto_gen.go#L12-L15). That implementation stores physical Hit, spell Hit, physical Crit and spell Crit as percentage points: one input unit contributes one percentage point. Its outcome code divides those stored values by 100 when producing physical and spell probabilities ([physical Hit/Crit](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_result.go#L115-L129), [spell Hit/Crit](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_result.go#L195-L230)). The overlay accepts only a profile whose own character level is the fixed Classic level of 60 and replaces only those four divisors in its copied rules value; all other fields are preserved rather than inferred.

The existing dependency seam can inject that value into characterization tests without selecting it for production. Scoped physical and spell inputs remain scoped, while the fork's shared `StatHitRating` and `StatCritRating` sources feed both outcome channels additively. The shared fan-out is a Forever architecture decision based on Blizzard's announcement, not a claim that Classic had shared stats. Likewise, the current `Rating` suffix is a provisional transport name and does not establish that Forever will use combat ratings.

This reference deliberately excludes Haste, Expertise, Defense, Dodge, Parry, Block and Resilience. The pinned Classic haste paths have asymmetric equipment behavior; its Expertise semantics are SoD-specific; its generated defense-family constants conflict with their source table; and Resilience is not consumed by combat and disagrees between backend and UI. It also excludes race/class base stats and attribute conversions: those tables cover only the original combinations, contain class-specific wiring exceptions, and cannot safely predict Forever's expanded combinations. No production profile, item data, UI or runtime outcome path selects this overlay.

### Classic level-60 armor reference

The engine now compiles a pure, inactive armor projection reproducing [`wowsims/classic` commit `7779ebbf79dc7f1341e6ab939b28a3402c9a730a`](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_resistances.go#L84-L88). After the [defender's armor multiplier and nonnegative floor](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/unit.go#L392-L394), that implementation subtracts flat armor penetration, floors effective armor at zero, and returns `1 - armor / (armor + 400 + 85 × attacker level)` as the damage-taken multiplier. It uses attacker level rather than defender level or unit type. The reference profile is pinned to the source's [level-60 character cap](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/constants.go#L7) and [default `+3` raid boss](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/target.go#L134-L138), while the numerical helper accepts attacker levels 1 through 63 so both leveling attacks and a boss attacking a player can be characterized.

Literal vectors pin 3,731 armor against a level-60 attacker at `0.5958184378723865` damage taken and the same armor against a level-63 attacker at `0.6066835336285052`. At level 60, 16,500 armor gives the familiar 75% reduction, but 22,000 armor gives 80%: the pinned code has no explicit 75% cap. Classic's own armor tests are disabled and contain stale expectations, so this is source-code characterization rather than verified client behavior. It is not evidence for Forever's formula or cap.

The helper accepts only finite, nonnegative resolved armor and flat penetration values. The latter name is deliberate: Classic subtracts the raw stat directly, while its unused percentage helper does not establish a conversion. This slice does not choose target armor, debuff values or stacking; model armor multipliers; route physical, periodic or multischool damage; support percentage ignore or full bypass; or define interactions with resistance. Forever demo data contains percentage-armor-ignore effects, but not their ordering relative to flat changes. Those surfaces must be activated together only after measurements settle the formula, cap, flooring and modifier order. The production engine continues to use inherited TBC behavior, including percentage ignore before flat penetration and its 75% cap.

### Classic level-60 magical-resistance reference

The engine now compiles separate, pure projections for the non-binary and binary resistance paths in [`wowsims/classic` commit `7779ebbf79dc7f1341e6ab939b28a3402c9a730a`](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_resistances.go#L125-L183). Both subtract flat spell penetration from an already-selected school resistance, floor the result at zero, divide by `5 × attacker level`, and cap the coefficient at one. For a non-binary spell against a higher-level enemy, the source adds `0.02 / 0.75` to the coefficient per level, [nominally representing two percentage points of average mitigation](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/constants.go#L28); realized mitigation becomes nonlinear above the two-thirds coefficient breakpoint. A pure DoT divides only the explicit-resistance component by ten before adding that level component. The resulting coefficient selects three piecewise cumulative roll thresholds separating the 0%, 25%, 50% and 75% partial-resist buckets.

At zero explicit resistance, a level-60 caster against a level-63 enemy has coefficient `0.08`, cumulative thresholds `0.1824 / 0.0504 / 0.0072`, bucket probabilities `81.76% / 13.20% / 4.32% / 0.72%`, and exactly 6% average mitigation. Against that same target, 200 resistance makes a direct spell average 54.56% mitigation and a pure DoT average 11%. The exact threshold projection caps at 69% average partial mitigation rather than 75%. These vectors deliberately expose a current inherited-TBC quirk: its zero-effective-resistance shortcut produces coefficient `0.06` and only 4.5% average mitigation for the same `+3` target.

Binary resistance is modeled separately because the pinned source does not partially reduce binary damage. It converts the resistance coefficient to `1 - 0.75 × coefficient`, a multiplier on base hit chance; defender level contributes nothing. The source then composes that value with base spell miss, additive spell Hit and a 1% miss floor in its [outcome path](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_result.go#L203-L217). This reference exposes only the resistance multiplier so it cannot be mistaken for final hit chance or damage mitigation.

The numerical input accepts attacker and defender levels 1 through 63, finite signed selected resistance, and finite nonnegative flat spell penetration. Negative selected resistance is retained because [source debuffs can subtract from resistance](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/debuffs.go#L542-L550) before the formula floors it; negative penetration fails closed. School selection, Holy behavior, resistance buffs and debuffs, stacking, physical/magical multischool routing, binary and pure-DoT spell classification, ignore-resist flags, random outcome application, base miss and shared Hit composition all remain outside this slice. The pinned multischool routing contains its own explicit uncertainty, and [one broad upstream resistance test is ineffective](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_resistances_test.go#L10-L91) because it omits the school index. These projections therefore characterize simulator code, not verified client or Forever behavior. They have no production caller; the inherited TBC resistance path remains active.

### Classic level-60 weapon-skill reference

The engine now compiles an inactive physical attack-table policy that reproduces the player-versus-enemy formulas in [`wowsims/classic` commit `7779ebbf79dc7f1341e6ab939b28a3402c9a730a`](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/target.go#L192-L349). Literal reference vectors cover a level-60 attacker against equal-level through `+3` targets, including the `+4`/`+5` hit-suppression breakpoint and the `+8` glancing-damage caps. The policy preserves the complete glancing multiplier range; its mean is available only for characterization, not as a replacement for the independent random roll used by Classic.

The resolver creates a short-lived value view for a spell and never mutates the shared attacker/defender `AttackTable`. It accepts only explicitly sourced, classified player weapons against enemies, with a melee or ranged defense type matching the source. Missing or unaudited weapon sources, malformed source/category pairs, generic synthetic weapons, pets, player defenders and Wand all fail closed. Wand is excluded because the pinned Classic implementation has neither a Wands skill category nor a Wand case in its weapon-skill lookup. Equipped fist weapons use the Unarmed bonus, while a truly empty main hand ignores that bonus to match Classic's nil-weapon path. The characterized scope deliberately rejects lower-level targets, whose upstream raw formula can produce a negative glance chance. Finite fractional and negative modifiers retain source behavior, but a negative modifier does not model training a weapon from below the assumed capped base skill.

This policy is a comparison fallback, not accepted Forever behavior. In the Classic reference, positive weapon skill changes miss, hit suppression, dodge and glancing damage, but does not change parry, glancing frequency or melee critical suppression. The implementation assumes capped base skill of `5 * attacker level`; it does not model training a weapon from below cap. Its fixed 14% parry chance against a `+3` enemy and its `+1`/`+2` interpolation remain beta-verification targets. It also adds 1.8 percentage points of boss critical suppression to total crit even though the [upstream history](https://github.com/wowsims/sod/pull/170) documents that this approximation should apply only to aura-derived crit.

The active inherited TBC profile explicitly keeps this model disabled, and no production outcome calls the resolver. Live simulations therefore retain their existing fixed attack tables and scalar glancing multiplier. Activation requires authoritative Forever measurements plus an audit of every weapon ability; currently only core auto attacks declare a weapon source. When activated, miss, hit suppression, dodge, parry, glancing chance and damage, and melee crit suppression must consume the same transient view in one reviewed change so the engine cannot produce a hybrid table.

### Known inherited quirks

- A level-69 target is not offered by the inherited encounter picker. Direct API input at that level falls through to the `+3` lookup values because the level table has no `-1` entry; this unsupported-input behavior is documented but deliberately not made a desired invariant.
- The inherited non-binary resistance formula gives a level-73 target 4.5% average mitigation when it has no explicit resistance, despite a nearby production comment saying 6%. Adding explicit resistance also changes how that level component is scaled. The tests pin the observed outputs for safe refactoring; they do not validate the formula for Forever.
- The expected-value path for a blocked weapon special appears internally inconsistent. This PR characterizes front-facing white attacks and rear-facing specials, but leaves that suspicious path for a focused bug investigation rather than legitimizing it as baseline behavior.

## Confirmed direction

| Rule | Engine decision | Status |
| --- | --- | --- |
| The announced leveling journey runs from 1 to 60 | Treat level 60 as the likely player cap only when a complete profile is installed; choose a default boss level only with evidence and matching base-stat data | Reviewed in inactive catalog; runtime unchanged |
| Spell, melee, and ranged hit chance are combined | `StatHitRating` is a shared source feeding separate physical and spell hit percentages | Foundation implemented; inactive Classic direct-percentage fallback characterized |
| Spell, melee, and ranged crit chance are combined | `StatCritRating` is a shared source feeding separate physical and spell crit percentages | Foundation implemented; inactive Classic direct-percentage fallback characterized |
| Weapon skill remains relevant | Preserve category-specific bonus data and weapon context; do not treat current TBC expertise as the final model | Inactive foundation implemented; claim reviewed; effects await beta data |
| Some items can reduce parry chance or dodge chance | Keep dodge and parry reduction distinct until evidence establishes whether one source feeds both | Claim reviewed in inactive catalog; mechanics await beta data |
| Bonus healing contributes one-third as much bonus damage | Add once at the item-data boundary, with a regression test against double counting | Claim reviewed in inactive catalog; runtime implementation pending |

The generic source stats do **not** merge physical and spell outcome tables. Base miss chances, caps, suppression, talents, school modifiers, and attack-specific bonuses remain independent.

## Deliberately unresolved

- Whether level 60 is explicitly the hard player cap rather than only the announced leveling-journey upper bound, and which default boss levels the simulator should offer.
- Whether Forever keeps Classic's direct Hit/Crit percentage points or introduces ratings, including any level scaling, caps, rounding and display rules; the compiled direct-percentage policy is only a pinned comparison fallback.
- The name, unit, conversion, cap, and rounding behavior of the announced parry/dodge-reduction effects, and whether one shared stat or separate stats feed them.
- Exact Forever weapon-skill effects on miss, dodge, parry, glancing chance, glancing damage, and critical suppression; the compiled Classic level-60 policy is only a pinned comparison fallback.
- Whether Forever uses Classic or TBC armor and resistance formulas, caps and modifier ordering in every case; the compiled Classic armor and magical-resistance projections are only pinned comparison fallbacks.
- Whether the bonus-healing damage contribution is precomputed in item data or derived at runtime, and where any fractional rounding occurs.

Until those values are confirmed, the runnable branch still uses inherited TBC conversion constants and combat tables. Characterization tests pin those placeholders so a later evidence-backed rules update produces an explicit reviewable diff.

Shared Hit/Crit inputs, armor mitigation, magical-resistance outcomes, and weapon-skill categories and bonuses are now represented by inactive seams in the modern engine. Source-pinned Classic level-60 reference policies, raw base attributes/resources and regression vectors characterize possible fallbacks, while Forever units, populated data and live combat effects remain deliberately unresolved.

## Evidence

- Blizzard: [World of Warcraft: Forever Deep Dive Panel Recap](https://news.blizzard.com/en-us/article/24303313/world-of-warcraft-forever-deep-dive-panel-recap)
- Blizzard: [World of Warcraft: Forever — What's Next Panel Recap](https://news.blizzard.com/en-us/article/24303862/world-of-warcraft-forever-whats-next-panel-recap)

## Secondary evidence

- [talentsforever.com data export](https://talentsforever.com/data.json) is a fan-maintained, CC BY 4.0 transcription of demo footage and Blizzard slides. Its source metadata distinguishes confirmed tooltip ranks from estimates and Classic fallbacks. It is useful for later class, talent, spell and racial work, but it is not authoritative evidence for undocumented global combat formulas.

The rolling source snapshot lives in `third_party/talentsforever/` with its
license notice and a derived integrity manifest. `make forever-data-verify`
checks the committed snapshot and reviewed-inactive core facts offline; `make forever-data-check` reports live
section and record changes without writing; and `make forever-data-update`
validates and replaces the snapshot. The updater preserves the exact source
bytes and keeps separate hashes for the parsed document, evidence-bearing
sections, source metadata, and individual top-level sections.

This snapshot is quarantined evidence, not accepted game data. No runtime,
database generator, or class implementation consumes it. A future adapter must
promote individual values in a separate reviewed change while retaining their
source path and confidence: direct demo observation, estimated rank, Classic
fallback, completeness, and simulator acceptance are distinct states.

### Additional implementation cross-checks

Keep `wowsims/tbc-new` as the architecture upstream. [ElliotWood/Forever](https://github.com/ElliotWood/Forever) is an additional, community-maintained implementation reference for talent/spell data, source pointers and behavioral test candidates, not a replacement engine or an authority for undocumented formulas. Pin any consulted file to a commit, preserve its license and attribution when adapting material, compare against the original evidence and the `talentsforever.com` snapshot, and retain disagreements and inferred ranks rather than silently choosing one source. Shared Classic ancestry or transcription of the same demo does not count as independent confirmation. Data promotion and runtime changes still require separate review; no automatic imports or synchronizations are enabled.
