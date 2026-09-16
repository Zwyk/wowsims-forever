# Forever core rules ledger

The separate [public Ret DPS facade](forever_ret.md) now selects an explicit,
provisional Forever overlay on the private Classic Paladin fallback. References
below to private-only fixtures describe those original APIs; the inherited TBC
entrypoint remains unchanged. Runtime promotion is limited to the new Ret
facade and does not activate the general reference-data catalog.


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
| Passive Classic racials and explicit fractional dependency sources | `character_racials_classic_reference_test.go`, `stats/deps_rounding_test.go` |
| Per-unit ownership of extracted runtime rules, including pets and attack tables | `ruleset_ownership_test.go` |
| Classic armor/resistance selected by the attack table and applied to damage | `armor_runtime_test.go`, `resistance_runtime_test.go` |
| Scheduled level-60 main-hand combat and iteration reset | `classic60_combat_runtime_test.go` |
| Dual-wield scheduling, per-hand skill and off-hand damage | `classic60_dualwield_runtime_test.go` |
| Rear weapon specials, conditional crit and scheduled casts | `classic60_melee_special_runtime_test.go` |
| Classic spell hit/crit, binary Hit composition and bounded runtime contexts | `spell_chance_policy_test.go` |
| Scheduled hardcasts, binary spells and pure-DoT applications/ticks | `classic60_spell_runtime_test.go` |
| Classic Mage mana, flat costs, MP5, five-second rule and unsupported resource contexts | `mana_policy_test.go`, `classic60_mana_runtime_test.go` |
| Real rank-11 Fireball, native APL rotation, OOM/recovery, travel and residual damage | `classic60_fireball_reference_test.go`, `classic60_mana_runtime_test.go` |
| Paladin Command/Judgement, mana, white attacks and pending-proc cleanup | `classic60_paladin_runtime_test.go` |
| All 44 Classic Paladin talent entries, legal builds and prerequisites | `classic60_paladin_talents_test.go` |
| Paladin talent effects, offensive abilities, self buffs and complete native-APL build fixtures | `classic60_paladin_build_test.go`, `classic60_paladin_full_build_test.go`, `classic60_paladin_abilities_test.go`, `classic60_paladin_buffs_test.go`, `classic60_paladin_misc_talents_test.go` |
| Paladin seals, Judgements, execute and restoration procs | `classic60_paladin_seals_test.go`, `classic60_paladin_execute_test.go` |
| Paladin healing, pushback, self blessings and selected auras | `classic60_paladin_healing_test.go`, `classic60_paladin_pushback_test.go`, `classic60_paladin_support_test.go` |
| Paladin incoming melee/magic, shields, threat, control and immunity cooldowns | `classic60_paladin_defense_test.go`, `classic60_paladin_magic_defense_test.go`, `classic60_paladin_utility_test.go`, `classic60_paladin_cooldowns_test.go` |
| Standalone Classic-60 diagnostic JSON and browser WASM | `classic60_preview_test.go`, `tools/classic60preview/smoke.mjs` |
| End-to-end class behavior | Existing seeded per-class `.results` files |

The per-class golden results are the broad regression layer. Do not regenerate them merely to make CI green: a result update must accompany a reviewed mechanics change and explain why each affected class moved.

## Core ruleset boundary

The active core ruleset is selected at build time and remains internal to the Go engine. It contains value-only level, rating, combat-outcome, baseline attribute-conversion and resource-model data; simulations cannot mutate it or choose a different ruleset through proto input. Existing public constants and constructors remain compatibility facades while call sites are migrated incrementally.

The first extraction contains the inherited TBC level bands and both attack-table directions. The second adds combat-rating conversions, level-based NPC critical strike chance, and fixed outcome values such as expertise steps, the dual-wield miss penalty, the spell miss floor, and enemy critical/crushing damage multipliers. Engine-created attack tables cache value copies of the selected rating and outcome rules, providing a profile-local calculation seam without mutable global state.

The active profile still reproduces the inherited TBC engine. Attack tables now also cache explicit armor/resistance model selectors. Deliberately partial internal Classic profiles connect characterized components for the fixtures below; they cannot be selected through the public API. Supported Paladin talents and class spells now compose within one private profile. Public class/UI activation remains separate work, and gear imports are outside the current milestone. A complete Forever profile will be activated only with matching level-60 rules and UI changes.

### Runtime rule ownership

Engine-created characters and targets now retain their selected profile by value. Pets inherit their owner's snapshot; ordinary attack tables use the attacker's snapshot and retain their existing copied rating/outcome policy. Attribute installation, mana dependencies, avoidance/defense/resilience conversions, expertise suppression and contextless spell crit/healing calculations consume the owning unit's rules. Legacy zero-value units still fall back to the build-time selector, and explicitly injected attack-table tests remain supported.

The private character construction seam requires **both** a profile and its raw base-stat seed: choosing a level must not silently reuse the active level-70 `BaseStats` table. This is ownership of already-extracted rules, not permission to simulate a mixed-profile encounter. No public/proto selector is added. Racial/class/form code, level-dependent constants, live output-stat flooring, regeneration, equipment and spell data still require coordinated migration before activating level 60.

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

These values are bonuses only: the engine does not yet define whether Forever expresses them as skill points, rating, percentage points or some other unit. Nothing converts them to inherited Expertise Rating. The default TBC profile does not consume them; the explicit Classic melee fixture now resolves them during attacks. Current item data also leaves every bonus at zero; populating a checked, provenance-backed Forever data overlay is a separate step.

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

This original entrypoint remains deliberately **before racials** as well as gear, bonus stats, talents, forms, pets and regeneration. The separate post-racial entrypoint below adds only unconditional own-character racial stat effects; it does not call the live TBC racial code.

### Passive racials and explicit source rounding

`classicReferenceInitializeCharacterWithRacials` composes [Classic's passive racial effects](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/racials.go) through the same dependency graph: Human Spirit ×1.05, Gnome Intellect ×1.05, Tauren **derived total Health** ×1.05, Night Elf +1 percentage point Dodge, and the source's +10 racial resistances. Orc/Troll have no unconditional own-character stat additions in this bounded path. Raw base stats and spell-cost base mana remain unchanged. Weapon specializations, equipment, pet/target effects and activated racial abilities are excluded.

The modern dependency manager now has an immutable, per-instance source-rounding choice. Its default and zero-value behavior still floor primary stat sources, preserving live TBC results. The Classic reference explicitly preserves fractional sources and does not call final stat flooring, matching the pinned [Classic dependency evaluator](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/stats/deps.go). This is **source-code characterization**, not confirmed client or Forever rounding. For example, a Gnome Mage produces 134.4 Intellect, 2,949 Mana, 2.45792% spell crit and unchanged 1,213 base mana; a Tauren Warrior produces 2,760.45 Health. Tests cover all 40 supported pairs with and without passives. Live final flooring is a separate migration boundary, not solved by this constructor option.

### Browser baseline lab and the working-Classic gate

The standalone [baseline lab](forever_pages.md) runs these Go calculations as WASM, not reimplemented JavaScript formulas. It supports the original 40 race/class pairs, passive-racial comparison, visible stat/avoidance/armor outputs and JSON export with build identity. It cannot instantiate a live Character, import gear/talents, run a rotation or produce DPS. Unknown parameters, unsupported pairs and non-60 levels fail closed. The main simulator remains TBC-70 and is not published as a Classic simulator. The lab deliberately shows source omissions such as Hunter's missing Agility-to-Dodge dependency; these must be verified/corrected against game evidence before calling the engine Classic-accurate. The finished product will use the original WoWSims landing page and class-simulator interface with Forever branding and data; the lab is temporary.

Our next acceptance target is a working **Classic baseline before Forever-specific mechanics**:

1. Complete a coherent level-60 runtime profile: combat/defense/weapon-skill and mitigation paths, regeneration, final rounding, forms and pets; remove incompatible level-70 fallbacks.
2. Complete Paladin mechanics, talents and base abilities with synthetic weapon inputs, then expand class coverage. Gear catalogs, gear presets and item-effect imports are outside this work. Resolve recorded source omissions and the Bear-form disagreement explicitly rather than inheriting them silently.
3. Validate actual simulated fights with source-grounded numerical fixtures, deterministic regression results and browser smoke tests. Only then describe the deployment as a level-60 combat simulator or begin integrating Forever deltas into that baseline.

### Scheduled Classic 60 melee integration

`classic60MeleeReferenceRules` now connects the existing Classic base attributes, direct-percentage offensive chances, weapon-skill view and mitigation models to the modern scheduler. It starts from an empty profile rather than copying TBC values. It is an internal integration fixture, not a complete ruleset, class implementation or public simulation option.

The executable scope is a pre-racial Human Warrior at level 60, with one or two explicitly equipped physical weapons, attacking a passive level 60–63 enemy from behind. It covers white attacks and diagnostic main-hand/off-hand weapon specials. The tests construct a real Character and run normal reset, weapon scheduling, spell casts, cooldowns, damage application and metrics across iterations. No class agent, talent, racial, item effect, resource bar or class rotation is imported from TBC. The live physical wrapper rejects unsupported contexts rather than falling back to inherited tables; the broader pure weapon reference remains available for characterization.

Miss, hit suppression, dodge, glancing chance/range and critical suppression use one weapon-specific view without overwriting shared attack-table state. Classic glancing damage consumes an independent random roll; the inherited TBC path keeps its existing random sequence and scalar multiplier. Expected white damage uses the same chances and mean glancing multiplier. At capped skill 300 against level 63, baseline miss is 8% and the first 1% hit is suppressed; at 305, miss is 6% and suppression is zero. Skill improves glancing damage, not glancing frequency. Bonus skill in the fixture is explicit and does not activate Human weapon racials.

Dual wield adds 19 percentage points of miss to white attacks from **both** hands, following the pinned [Classic white/special miss paths](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_outcome.go#L650-L670). Each attack resolves the skill of its own weapon: improving sword skill does not improve an off-hand axe. At zero Hit against level 63, a 305-skill main hand has 25% white miss and a 300-skill off hand has 27%. Existing modern damage helpers already match [Classic's off-hand formula](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/attack.go#L117-L140): `0.5 × (weapon damage + speed × AP / 14)`. The scheduler keeps modern timing, including an initial half-swing offset for the off hand; this is not a claim to reproduce Classic batching.

Rear weapon specials use their hand's miss, Hit suppression, dodge and crit suppression, with no dual-wield miss penalty or glancing blows. Crit is rolled independently **after** landing. Expected damage therefore multiplies the landing probability by the conditional crit multiplier, rather than adding white-table crit probability. Diagnostic casts exercise normalized weapon damage and cooldown reset through the real scheduler; they do not stand in for an implemented Warrior ability. Explicit source and proc-hand categories must agree, invalid dual-wield equipment fails, and white/special outcome functions reject the wrong attack kind. Queued Heroic Strike miss-penalty overrides remain unsupported.

The fixture deliberately retains the pinned source's known crit approximation: its extra 1.8 percentage points of suppression at +3 apply to total crit, although the source says they should apply only to aura-derived crit. A pre-racial Human Warrior has 4% crit, so this approximation yields zero against level 63. This is a regression anchor for the source, not verified game accuracy; a provenance-aware correction requires a separate policy decision and tests.

Mitigation is selected by the attack table's copied rules. Classic armor uses `400 + 85 × attacker level`, flat armor penetration and the source's uncapped reduction. Resistance supports individual schools, explicit pure-DoT classification, partial-resist buckets and the separate binary multiplier; Holy never reads the Strength stat as resistance. Hybrid schools are explicitly unsupported. The separate spell fixture below now exercises resistance with scheduled diagnostic casts; real class implementations remain separate. The new `SpellFlagPureDot` has no effect under inherited TBC rules.

The generic melee fixture does not itself activate front-facing/incoming combat, resource/regen and haste policies, live racial/final-stat rounding, forms/pets, or class/talent/spell/encounter data. The separate Paladin profile below now supplies bounded front-facing, incoming and mana/healing coverage. Other class-specific attack modifiers, on-next-swing replacements and proc interactions still need their own acceptance tests. The original melee fixture's integer pre-racial stats do not imply that live flooring has been converted for every class. Unsupported nonzero defense/resilience conversions fail explicitly; absent zero values no longer produce `0/0` during initialization. The public simulator and browser lab retain their existing scope.

### Scheduled Classic 60 spell integration

`classic60SpellReferenceRules` assembles a separate internal caster profile from empty values, Classic attributes/chance inputs and the existing resistance model. It is not selectable through the public API. Its supported context is a level-60 player casting explicitly non-weapon, single-school magical damage against a passive level 60–63 enemy. The damage/chance paths reject resources, haste, pets, weapon outcomes, hybrid schools and unaudited crit modifiers rather than selecting inherited TBC behavior. Healing remains outside this diagnostic. The profile and attack table retain value-owned rule selection.

The source-pinned [spell miss policy](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_result.go#L195-L233) uses 4/5/6/17% base miss at equal/+1/+2/+3 levels and a 1% miss floor. Binary resistance multiplies base level-hit before bonus Hit is added: at +3, 100 resistance and +10 percentage points Hit, miss is 27.75%. School-specific Hit applies without the inherited TBC class-mask condition. Crit is an independent roll after landing, with the baseline 1.5 damage multiplier.

An explicit source caveat matters here: Classic fills `SpellCritSuppression` in its table but comments out its subtraction in `SpellCritChance`. This profile reproduces that executable behavior; it does not establish that zero boss suppression is correct in the client. Unused table seeds cannot silently introduce TBC suppression. School/target crit modifiers without a reviewed modern mapping remain unsupported.

The scheduler fixture constructs a pre-racial Human Mage with its integer baseline and attribute dependencies, but no Mage agent or mana bar. Resource-free diagnostic direct/binary spells exercise hardcast completion, GCD/cooldown enforcement, resist/Hit/Crit, damage reports and iteration reset. Pure-DoT applications roll Hit once; only landed applications schedule three noncritical ticks. Periodic damage uses the explicitly classified pure-DoT resistance projection. Long seeded runs check literal expected miss/crit fractions and average damage, including penetration order, the nonpenetrable level component, and the distinction between direct and pure-DoT mitigation. Identical seeds reproduce complete event traces and normalized report metrics.

Expected-outcome helpers model Hit/Crit only. They are not a deterministic full-resistance expected-DPS API: the ordinary damage pipeline still samples partial resistance. Classic's separate expected-damage postprocessing ignores some resistance inputs and is not imported. The separate Mage mana/Fireball integration below now adds one real rank and a native APL rotation. More spells, haste, channels, talent/racial/proc modifiers and matching class data are still required before a usable caster can be exposed in the normal WoWSims UI. The active TBC profile and browser lab retain their existing scope.

### Classic 60 Mage mana and Fireball integration

`classic60MageManaReferenceRules` extends the spell profile with a private, pre-racial Human Mage resource path. The mana model belongs to each unit and is copied into attack tables. The public/zero-value path keeps inherited TBC regeneration and cost arithmetic; the earlier melee and resource-free spell profiles explicitly disallow mana. Other classes, races, pets, resource bars and regeneration/cost modifiers are rejected in this bounded reference.

The [pinned Classic Mage source](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/mage/mage.go#L146-L149) gives `6.25 + Spirit / 8` mana per second outside the five-second rule. Intellect does not enter this regeneration formula. MP5 adds `MP5 / 5` per second both inside and outside the rule. The existing modern two-second tick scheduler, five-second-rule expiry comparison, mana capping and completion-time spending match the supported [Classic mana/cast path](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/mana.go). The pre-racial Human baseline has 1,213 base mana, 2,808 total mana and 42.5 mana per full tick.

The Mage reference accepts only unmodified positive integer flat costs. The private Paladin extension below also preserves fractional base-mana percentage costs and its explicitly supported talent reductions; unrelated resource modifiers remain unsupported. Interrupted hardcasts neither spend mana nor refresh the five-second rule; completed casts spend their cost even if the projectile subsequently misses. Instant costs refresh the rule immediately, and a tick exactly at expiry uses full regeneration.

`registerClassic60ReferenceFireball` adapts [Classic Fireball rank 11](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/mage/fireball.go): spell 10151, 395 mana, 3.5-second cast, 561–715 direct damage, coefficient 1, and missile speed 24. It is the pre-AQ level-60 rank; spell 25306 is the separately gated AQ rank 12. Damage resolves at cast completion and arrives after travel. A landed hit applies four two-second ticks of 18 damage with zero spell-damage coefficient. This residual is not classified as a pure DoT and therefore uses ordinary Classic partial resistance; it cannot crit. The implementation deliberately omits Mage talent/proc hooks and is not registered by the public TBC class agent.

An integration fixture assembles the real Classic baseline and runs this spell both with explicit tick-driven retries and with the existing WoWSims native APL cast action. In the untalented 73-second case, seven initial casts leave 43 mana at 24.5 seconds; recovery allows starts at 46 and 70 seconds. The eighth cast completes at 49.5 seconds; the ninth is still casting when the fight ends. Native APL tests verify the same schedule, 42 seconds of OOM time per iteration, mana spending/regeneration report totals, capped gains and deterministic resets. Additional tests cover interruption, instant costs, MP5 and exact five-second boundaries, missed applications, residual resistance and pending casts/projectiles/DoTs at iteration end.

This is the first real spell rotation through the modern engine under bounded Classic rules, not a complete Mage simulator or verified Forever behavior. The source caveats about spell crit suppression and resistance remain. Development now prioritizes Paladin, beginning with Retribution. Mage remains a regression reference. Its incoming/front-facing combat, forms and pets remain outside that fixture; Paladin now has the separate bounded coverage below.

### Classic 60 Paladin Command integration

`classic60PaladinReferenceRules` combines melee, spell, mana, healing and bounded incoming-combat policies for a private **pre-racial Human Paladin**. It accepts synthetic one-handed or two-handed swords, maces and axes, with an optional shield, against level 60–63 NPCs. Front-facing attacks and incoming main-hand white attacks are covered, as are explicitly classified incoming magical spells. This profile cannot be selected through the public API. The public simulator remains TBC-70 and the deployed Pages preview remains the static stat diagnostic.

The class reference uses three separately identified sources; none establishes verified Forever behavior:

| Reference | What it supplies |
| --- | --- |
| [WoWSims Classic `7779ebbf79dc7f1341e6ab939b28a3402c9a730a`](https://github.com/wowsims/classic/tree/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/paladin) and its [cached tooltips](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/assets/db_inputs/wowhead_spell_tooltips.csv) | Talent layout, rank data and executable mechanics, with the corrections below. Some source paths contain SoD assumptions or incomplete healing/utility behavior. |
| [VMaNGOS `8f4e608450460efe1e38743e4da74397d4773a3a`](https://github.com/vmangos/core/tree/8f4e608450460efe1e38743e4da74397d4773a3a) | Explicit Classic server references for coefficients, proc handling, healing/threat, parry haste and source conflicts. Principal files are `SpellEntry.cpp`, `SpellEffects.cpp`, `SpellCaster.cpp`, `Unit.cpp`, `UnitAuraProcHandler.cpp`, `SpellAuras.cpp` and `Spell.cpp`. |
| [CMaNGOS original Classic spell data `8ec338a1704e7dcb1c0213eb7ed58f9231ade40f`](https://github.com/cmangos/mangos-classic/blob/8ec338a1704e7dcb1c0213eb7ed58f9231ade40f/sql/base/dbc/original_data/Spell.sql), its [proc table](https://github.com/cmangos/mangos-classic/blob/8ec338a1704e7dcb1c0213eb7ed58f9231ade40f/sql/base/mangos.sql), and [VMaNGOS database snapshot `db-13b49dc.zip`](https://github.com/vmangos/core/releases/download/db_latest/db-13b49dc.zip) | Rank amounts, spell attributes, shared cooldown categories and proc-rate cross-checks. The downloaded database artifact has a versioned filename under a mutable release tag; it is distinct from the pinned source commit. Server formulas and proc rates remain reference choices pending client measurements. |

The audited database archive has SHA-256 `2e000f71993dfd39a4cf9c4c820fe8f23844c616afe8d7368249f5a552dc9dd1`; only the cited mechanics are transcribed, without importing its gear data.

| Component | Supported reference behavior |
| --- | --- |
| Mana | 1,512 base mana; 2,282 total at the pre-racial baseline. Spirit regeneration is `7.5 + Spirit / 10` per second outside the five-second rule; MP5 works throughout, in two-second ticks. |
| Seal of Command rank 5 (20920) | Talent required; 210 mana, 1.5-second GCD, 30-second aura. Landed white attacks roll 7 PPM using base weapon speed, with a one-second proc cooldown. |
| Command proc (20947) | Fresh non-normalized weapon damage including AP: `0.7 × weapon damage + 0.20 × spell power`, before outgoing modifiers. Judgement of the Crusader contributes separately with coefficient `0.29` after outgoing modifiers. |
| Command proc outcome | Independent melee-special miss/dodge/parry/block/crit as applicable to facing and target control; no glancing; double-damage crits and Holy nonbinary resistance. Calculated damage is delivered after the source's explicit 10ms delay. |
| Judgement (20271) | Off GCD, ten-second cooldown, six percent of base mana: **90.72** before talents. Consumes the selected seal even if its damage Judgement misses. |
| Judgement of Command (20966) | Base 169.5–186.5 against unstunned targets, doubled base against stunned targets, plus `0.429 × spell power`. Separate magical hit check and melee crit; Holy nonbinary resistance. |

Command deliberately corrects the old executable WoWSims coefficient of **0.203**: VMaNGOS applies the weapon percentage before its separate `.20` caster coefficient and `.29` target coefficient. The talent check is explicit, although the pinned WoWSims registration omitted it. The isolated 10ms delivery does not establish a general batching or seal-twisting model.

The shared mana path preserves private floating-point costs while inherited TBC arithmetic remains intact. The private Paladin profile also retains fractional primary attributes through initialization and dynamic Kings/talent changes. For the original supported Retribution build, tests pin 115.5 Strength, 391 AP and 2,387 total mana while base mana remains 1,512. These are source-regression anchors, not client or Forever measurements.

### Classic 60 Paladin talents and offensive abilities

`classic60_paladin_talents.go` defines all **44 Classic talents**, independently of the TBC protobuf: 14 Holy, 15 Protection and 15 Retribution. It validates ASCII ranks, maximum ranks, the 51-point budget, earlier-row requirements and all five prerequisite arrows against the pinned [talent tree](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/ui/core/talents/trees/paladin.json). The build validator now accepts all 44 entries and checks the allocation **before mutating the character**. Every entry has executable coverage, with the effect-specific encounter, movement and targeting boundaries below; this does not make every possible Paladin interaction available.

Three legal 51-point builds run through the native APL:

| Build | Classic talent string | Integrated coverage |
| --- | --- | --- |
| Retribution **11/8/32** | `550001-503-55205051000315` | Might, Command/Judgement, Exorcism, Consecration, Holy Wrath, Vengeance and mana recovery. |
| Holy **35/16/0** | `55503120521051-550051-` | Actual NPC physical/Fire damage, reactive healing, Divine Favor, Illumination and selected Concentration. |
| Protection **20/31/0** | `5505311-053021333001451-` | Front-facing one-hand/shield combat, Holy Shield/Sanctuary retaliation, Righteousness/Judgement, Consecration, Righteous Fury threat and OOM/recovery. |

Holy and Protection each run three ninety-second iterations and reproduce an exact same-seed replay, checking health/mana, casts, threat and reset state. Fixtures use synthetic weapons, shield/armor values and incoming spells; gear catalogs, gear presets and item effects are excluded from this milestone.

In the table, **supported** means executable within this private profile and the stated boundaries, including complete legal builds. Mounted travel, arbitrary NPC spell data and controls outside the explicit encounter APIs are not implied.

| Tree | Talent | Implemented effect and boundary |
| --- | --- | --- |
| Holy | Divine Strength | Supported: 2% Strength per rank, preserving fractional dependencies. |
| Holy | Divine Intellect | Supported: 2% Intellect per rank. |
| Holy | Spiritual Focus | Supported: 14% per rank pushback prevention for Holy Light and Flash of Light. |
| Holy | Improved Seal of Righteousness | Supported: 3% per rank to base proc/Judgement damage; does not multiply spell-power contributions. |
| Holy | Healing Light | Supported: 4% per rank to Holy Light and Flash of Light healing. |
| Holy | Consecration | Supported: learned rank-5 ground damage with snapshots and per-target tick outcomes. |
| Holy | Improved Lay on Hands | Supported: cooldown reduction and recipient equipment-armor bonus; strongest effect wins. |
| Holy | Unyielding Faith | Supported: 5/10% resistance to typed NPC fear/disorient effects after their magical hit check; see the control boundary below. |
| Holy | Illumination | Supported: 20% per rank chance to refund full base cost after Holy Light, Flash of Light or healing Holy Shock crits. |
| Holy | Improved Blessing of Wisdom | Supported: 10% per rank to the self blessing's MP5. |
| Holy | Divine Favor | Supported: next eligible heal or Holy Shock crit; 4% base mana, two-minute cooldown, off GCD. |
| Holy | Lasting Judgement | Supported: +10 seconds per rank for Crusader, Light and Wisdom debuffs. Justice's fleeing Judgement is unavailable. |
| Holy | Holy Power | Supported: +1% per rank Holy magical/healing crit. Does not affect Command's melee crit or Hammer of Wrath's ranged crit. |
| Holy | Holy Shock | Supported: offensive and healing rank-3 actions share one cooldown. |
| Protection | Improved Devotion Aura | Supported: 5% per rank to the preselected armor aura. |
| Protection | Redoubt | Supported: incoming melee/ranged crit trigger; +6% block per rank, five blocks, ten seconds. Incoming ranged attacks are outside this profile's current acceptance boundary. |
| Protection | Precision | Supported: +1% physical hit per rank; explicitly excluded from Hammer of Wrath. |
| Protection | Guardian's Favor | Supported: Protection cooldown 5/4/3 minutes and Freedom duration 10/13/16 seconds; self-targeted blessings and explicitly typed movement control. |
| Protection | Toughness | Supported: 2% per rank equipment-armor scaling, tested with synthetic values. |
| Protection | Blessing of Kings | Supported: self blessing, +10% primary attributes. |
| Protection | Improved Righteous Fury | Supported: improves the 60% bonus threat by 16/33/50%, yielding **1.9×** Holy threat at rank 3. |
| Protection | Shield Specialization | Supported: +10% per rank to shield block amount; the Strength component is outside this multiplier and added only once. |
| Protection | Anticipation | Supported: +2 Defense skill per rank using Classic direct percentages, not TBC rating. |
| Protection | Improved Hammer of Justice | Supported: five-second cooldown reduction per rank. |
| Protection | Improved Concentration Aura | Supported: +5/10/15% pushback prevention and resistance to typed silence/interrupt effects while Concentration is active. |
| Protection | Blessing of Sanctuary | Supported: self blessing, 24 flat incoming damage reduction and 35 Holy retaliation on blocks. |
| Protection | Reckoning | Supported: 20% per rank chance on incoming melee/ranged crit to advance an enabled main-hand swing; no stored stacks or TBC four-charge buff. |
| Protection | One-Handed Weapon Specialization | Supported: 2% per rank to physical damage and the two Command attacks while wielding one hand. |
| Protection | Holy Shield | Supported: rank 3, shield required, +30% block, four charges, ten seconds, Holy block retaliation. |
| Retribution | Improved Blessing of Might | Supported: 4% per rank to the self blessing's AP. |
| Retribution | Benediction | Supported: 3% per rank reduction for supported seals and Judgement only, preserving fractional costs. |
| Retribution | Improved Judgement | Supported: cooldown reduced from ten to eight seconds at rank 2. |
| Retribution | Improved Seal of the Crusader | Supported: 5% per rank to seal AP and the judged Holy-damage bonus. |
| Retribution | Deflection | Supported: +1% parry per rank on the incoming Classic table. |
| Retribution | Vindication | Supported: 3 PPM on landed melee, separate binary Holy hit, ten seconds of 5/10/15% enemy Strength/Agility reduction; explicit immunity and strongest-rank exclusivity. No aura-proc chains or invented NPC attribute dependencies. |
| Retribution | Conviction | Supported: +1% physical crit per rank. |
| Retribution | Seal of Command | Supported: explicitly learned seal and damage Judgement. |
| Retribution | Pursuit of Justice | Supported: 4/8% running speed in the strongest-passive movement category; mounted travel is outside the engine state model. |
| Retribution | Eye for an Eye | Supported: 15/30% of incoming magical critical damage before mitigation; triggered base damage capped at half the Paladin's maximum health, then ordinary outgoing magic modifiers/outcomes. |
| Retribution | Improved Retribution Aura | Supported: 25% per rank to the preselected aura's base retaliation. |
| Retribution | Two-Handed Weapon Specialization | Supported: 2% per rank to physical damage and the two Command attacks while wielding two hands. |
| Retribution | Sanctity Aura | Supported: preselected +10% Holy damage aura. |
| Retribution | Vengeance | Supported: dealt critical damage refreshes an eight-second Physical/Holy multiplier, up to 15%, without stacking. |
| Retribution | Repentance | Supported: six-second damage-breaking incapacitate against Humanoids; explicit immunity and magical hit checks. |

| Ability group | Supported level-60 behavior |
| --- | --- |
| Consecration / Exorcism / Holy Wrath | Rank 5 Consecration costs 565 mana and deals eight ticks of `48 + .042 × spell power`; rank 6 Exorcism costs 345 and deals `505–563 + .429 × spell power`; rank 2 Holy Wrath costs 805, casts in two seconds and deals `490–576 + .19 × spell power`. Exorcism/Wrath require Undead or Demon targets. |
| Holy Shock | Rank 3 costs 325 mana and has a shared thirty-second cooldown. Damage is `365–395 + .429 × spell power`; healing is `365–395 + 3/7 × healing power`. The two targets/actions remain distinct in APL. |
| Hammer of Wrath | Rank 3 costs 425 mana, casts in one second, has a six-second cooldown and requires the 20% execute phase. Deals `504–556 + .429 × spell power` with its own ranged hit/crit path; excludes Precision, Holy Power and melee weapon-skill bonuses. The cached tooltip corrects the source code's erroneous 566 endpoint. |
| Holy Light / Flash of Light | Holy Light rank 9: 660 mana, 2.5-second cast, `1590–1770 + 5/7 × healing power`. Flash rank 6: 140 mana, 1.5-second cast, unrounded `348.2–388.2 + 3/7 × healing power`. Holy Light uses the learned level-60 book rank; progression availability and downranking are not modeled. |
| Lay on Hands | Rank 3 spends remaining mana, heals for caster maximum health, restores 550 mana to a mana-using recipient and applies the talent's armor/cooldown effects. It does not crit. |
| Crusader | Rank 6 costs 160 mana; 325.2 AP before talents, 40% faster melee and compensating white-damage reduction. Judgement adds 140 Holy damage before coefficients; own landed melee refreshes it. The tooltip rounds the AP display to 326. |
| Righteousness | Rank 8 costs 200 mana. Landed white attacks trigger guaranteed, noncritical, resistance-ignoring Holy damage. Base damage interpolates from `1880/87` at 1.5-second weapon speed to `1880/25` at four seconds; SP coefficients are `.10` for one hand and `.125` for two. Judgement deals `170.2–186.2 + .5 × spell power` with binary magical hit/crit. |
| Light / Wisdom | Rank 4 Light costs 210 mana and restores 94 health; rank 3 Wisdom costs 200 mana and restores 90 mana. Both use **20 PPM** on landed melee, based on unhasted weapon speed. Their Judgements use **50%** landed-hit chances for 61 health on melee or 59 mana on direct attacks/spells. Judgement restoration belongs to the attacker. |
| Justice | Seal costs 13% base mana and uses **5 PPM** for a two-second stun, with magical hit and explicit stun immunity. Its fleeing-prevention Judgement is deliberately unregistered; trying to judge this seal spends nothing and leaves it active. |
| Hammer of Justice / Repentance | Hammer rank 4: 100 mana, six-second stun, sixty-second base cooldown. Learned Repentance: 60 mana, six-second Humanoid incapacitate, one-minute cooldown, broken by damage. Immunity is explicit encounter state, not inferred from level. |
| Blessing of Freedom | Self-targeted; 10% base mana, twenty-second cooldown, ten seconds before Guardian's Favor. Removes and prevents the supported NPC magical root/snare effects, restoring actual running movement; replaces the caster's other self blessing. |

Seals replace one another and last thirty seconds. Persistent Crusader/Light/Wisdom Judgements replace that Paladin's previous persistent Judgement, last ten seconds before talents and refresh on the caster's landed melee hits. Light/Wisdom/Justice proc rates come from the separately identified server tables; the pinned WoWSims source did not implement those seals. Righteousness replaces the source's SoD-derived formula, doubled SP and critical hits; its no-crit behavior is independently confirmed by [Blizzard's May 10, 2024 Classic Era/Hardcore hotfix](https://news.blizzard.com/en-us/article/24066687/hotfixes-july-22-2024). Two-caster tests cover strongest Crusader bonuses, exclusive Light/Wisdom effects and independent ownership when one Paladin changes Judgement; duplicate Judgements do not multiply their benefit.

Healing accepts living friendly player units in the same environment and private profile, including self. It uses healing power rather than damage power and preserves overheal and native heal callbacks. Paladin heals and Paladin-owned Light procs give **0.25 threat per effective health restored**; a non-Paladin attacker's Judgement of Light proc uses 0.5 and belongs to that attacker. Overhealing creates no threat. Blessing of Light's 400/115 bonus is applied through the relevant Holy Light/Flash coefficient after caster healing multipliers, before target multipliers/crit, following the explicit VMaNGOS reference. Illumination refunds only eligible healing crits. Incoming damaging direct hits cause the Classic pushback sequence of 1, .8, .6, .4, then .2 seconds, with Spiritual Focus and preselected Concentration preventing eligible delays.

Self blessings include Might rank 6 (155 AP), Wisdom rank 5 (30 MP5), learned Kings, Salvation (30% less threat), Light and learned Sanctuary. They replace the same caster's previous self blessing and do not stack on refresh. Might/Wisdom deliberately retain trainer ranks rather than their book upgrades. Exactly one aura, or none, is selected before finalization: Devotion (735 armor), learned Sanctity, Retribution (20 Holy retaliation before its talent), Concentration (35% pushback prevention), or Fire/Frost/Shadow Resistance (60 resistance). Live aura switching, range and party distribution are not modeled.

Incoming physical combat uses the Classic miss/dodge/parry/block/crit/crush/hit table with synthetic shield block values. Casting or being stunned disables dodge/parry/block. Bonus Defense changes avoidance and crit but does not remove the source's level-based crushing chance. Parry haste uses the VMaNGOS remaining-time 20% floor and 40% reduction, avoiding the pinned WoWSims backward-scheduling edge case. The Strength contribution to block value is added once, outside Shield Specialization. Holy Shield, Sanctuary and Retribution retaliation, Redoubt, Reckoning and Righteous Fury run from actual incoming events. Righteous Fury increases Holy threat from 1.6× to **1.9×** with its talent; the pinned source's 2.4× is explicitly corrected. Incoming magical hit/resistance and Eye for an Eye preserve the pre-mitigation critical amount; the reflection can itself miss, resist or crit and receives no SP coefficient.

Defensive cooldowns are self-targeted. Divine Shield provides twelve seconds of all-school damage immunity with half melee speed; Divine Protection provides eight seconds of all-school damage immunity and physical pacification; they share a five-minute cooldown. Blessing of Protection provides ten seconds of physical immunity and physical pacification. All impose sixty seconds of Forbearance. The two divine immunities can be cast while incapacitated and remove/prevent the supported typed root, snare, fear, disorient, silence, stun and Repentance effects. They do not clear an existing interrupt school lockout. These hooks do not form a general dispel system or immunity to every unspecified debuff. Immune damage retains `OutcomeImmune` for callbacks/logs and is grouped into the existing report's **Misses** bucket because that transport has no separate immunity counter.

The private hostile-control API describes magical spells whose main mechanic is zero and whose effect mechanic is disorient (2), fear (5), silence (9) or interrupt (26). It performs an ordinary binary spell hit check followed by the appropriate talent-resistance roll, following VMaNGOS `IsEffectResist`; main-mechanic control spells use a different hit formula and are deliberately not accepted by this API. Effects drive real incapacitation, cast cancellation, silence, school lockout and expiry. Freedom handles separately typed magical roots/snares through actual running movement. Vindication changes enemy Strength/Agility and any explicitly configured dependencies; it does not invent Strength-to-AP or Agility-to-dodge for encounter inputs that already supply those values. The source's incorrect self-AP Vindication buff is not retained.

The test suite covers literal ranks/formulas, source corrections, resource boundaries, proc filters, PPM boundaries, incoming tables, avoidance/block/crushing, actual retaliation and healing, pushback, control removal, immunity, cooldowns, threat, aura expiry and iteration reset. Native APL fixtures exercise class rotations and resource recovery with the same engine used by public classes; this does not expose a playable Classic/Forever class UI.

**Remaining Paladin work:** Judgement of Justice's fleeing effect; Sacrifice, general cleansing/dispels, Turn Undead, resurrection and other out-of-combat utility; main-mechanic and physical NPC control paths; downranking and progression choices; live aura switching and party buff distribution; seal twisting, live weapon swaps and PvP diminishing returns. Mounted travel, other races and their active racials are outside this Human class fixture. Gear remains expressly excluded from the requested milestone. The stale Holy UI preset still fails its own Classic tree validation; the TBC protobuf, public simulator and static preview remain unchanged. **Paladin is substantially broader, but not fully ready.**

### Classic level-60 offensive chance-input reference

The engine compiles a reference overlay for the four well-supported Hit and Crit conversions in [`wowsims/classic` commit `7779ebbf79dc7f1341e6ab939b28a3402c9a730a`](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/base_stats_auto_gen.go#L12-L15). That implementation stores physical Hit, spell Hit, physical Crit and spell Crit as percentage points: one input unit contributes one percentage point. Its outcome code divides those stored values by 100 when producing physical and spell probabilities ([physical Hit/Crit](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_result.go#L115-L129), [spell Hit/Crit](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_result.go#L195-L230)). The overlay accepts only a profile whose own character level is the fixed Classic level of 60 and replaces only those four divisors in its copied rules value; all other fields are preserved rather than inferred.

The existing dependency seam can inject that value into characterization tests without selecting it for production. Scoped physical and spell inputs remain scoped, while the fork's shared `StatHitRating` and `StatCritRating` sources feed both outcome channels additively. The shared fan-out is a Forever architecture decision based on Blizzard's announcement, not a claim that Classic had shared stats. Likewise, the current `Rating` suffix is a provisional transport name and does not establish that Forever will use combat ratings.

This reference deliberately excludes Haste, Expertise, Defense, Dodge, Parry, Block and Resilience. The pinned Classic haste paths have asymmetric equipment behavior; its Expertise semantics are SoD-specific; its generated defense-family constants conflict with their source table; and Resilience is not consumed by combat and disagrees between backend and UI. It also excludes race/class base stats and attribute conversions: those tables cover only the original combinations, contain class-specific wiring exceptions, and cannot safely predict Forever's expanded combinations. Only the explicit internal Classic melee fixture selects this overlay; production selection, item data and UI remain unchanged.

### Classic level-60 armor reference

The engine compiles a pure armor projection reproducing [`wowsims/classic` commit `7779ebbf79dc7f1341e6ab939b28a3402c9a730a`](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_resistances.go#L84-L88). After the [defender's armor multiplier and nonnegative floor](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/unit.go#L392-L394), that implementation subtracts flat armor penetration, floors effective armor at zero, and returns `1 - armor / (armor + 400 + 85 × attacker level)` as the damage-taken multiplier. It uses attacker level rather than defender level or unit type. The reference profile is pinned to the source's [level-60 character cap](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/constants.go#L7) and [default `+3` raid boss](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/target.go#L134-L138), while the numerical helper accepts attacker levels 1 through 63 so both leveling attacks and a boss attacking a player can be characterized.

Literal vectors pin 3,731 armor against a level-60 attacker at `0.5958184378723865` damage taken and the same armor against a level-63 attacker at `0.6066835336285052`. At level 60, 16,500 armor gives the familiar 75% reduction, but 22,000 armor gives 80%: the pinned code has no explicit 75% cap. Classic's own armor tests are disabled and contain stale expectations, so this is source-code characterization rather than verified client behavior. It is not evidence for Forever's formula or cap.

The helper accepts only finite, nonnegative resolved armor and flat penetration values. The latter name is deliberate: Classic subtracts the raw stat directly, while its unused percentage helper does not establish a conversion. The pure helper does not choose target armor, debuff values or stacking. The runtime integration above now supplies resolved armor and routes physical damage, bleed bypass and explicit generic armor-ignore controls; multischool support remains separate. Forever demo data contains percentage-armor-ignore effects, but not their ordering relative to flat changes. Those surfaces must be activated together only after measurements settle the formula, cap, flooring and modifier order. The production engine continues to use inherited TBC behavior, including percentage ignore before flat penetration and its 75% cap.

### Classic level-60 magical-resistance reference

The engine now compiles separate, pure projections for the non-binary and binary resistance paths in [`wowsims/classic` commit `7779ebbf79dc7f1341e6ab939b28a3402c9a730a`](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_resistances.go#L125-L183). Both subtract flat spell penetration from an already-selected school resistance, floor the result at zero, divide by `5 × attacker level`, and cap the coefficient at one. For a non-binary spell against a higher-level enemy, the source adds `0.02 / 0.75` to the coefficient per level, [nominally representing two percentage points of average mitigation](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/constants.go#L28); realized mitigation becomes nonlinear above the two-thirds coefficient breakpoint. A pure DoT divides only the explicit-resistance component by ten before adding that level component. The resulting coefficient selects three piecewise cumulative roll thresholds separating the 0%, 25%, 50% and 75% partial-resist buckets.

At zero explicit resistance, a level-60 caster against a level-63 enemy has coefficient `0.08`, cumulative thresholds `0.1824 / 0.0504 / 0.0072`, bucket probabilities `81.76% / 13.20% / 4.32% / 0.72%`, and exactly 6% average mitigation. Against that same target, 200 resistance makes a direct spell average 54.56% mitigation and a pure DoT average 11%. The exact threshold projection caps at 69% average partial mitigation rather than 75%. These vectors deliberately expose a current inherited-TBC quirk: its zero-effective-resistance shortcut produces coefficient `0.06` and only 4.5% average mitigation for the same `+3` target.

Binary resistance is modeled separately because the pinned source does not partially reduce binary damage. It converts the resistance coefficient to `1 - 0.75 × coefficient`, a multiplier on base hit chance; defender level contributes nothing. The source then composes that value with base spell miss, additive spell Hit and a 1% miss floor in its [outcome path](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_result.go#L203-L217). This reference exposes only the resistance multiplier so it cannot be mistaken for final hit chance or damage mitigation.

The numerical input accepts attacker and defender levels 1 through 63, finite signed selected resistance, and finite nonnegative flat spell penetration. Negative selected resistance is retained because [source debuffs can subtract from resistance](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/debuffs.go#L542-L550) before the formula floors it; negative penetration fails closed. The runtime integration above adds individual-school selection, Holy handling, explicit pure-DoT classification, ignore-resist routing and random bucket application. Resistance buffs/debuffs and stacking, multischool routing, class spell classification, base spell miss and shared Hit composition remain separate work. The pinned multischool routing contains its own explicit uncertainty, and [one broad upstream resistance test is ineffective](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/spell_resistances_test.go#L10-L91) because it omits the school index. These projections therefore characterize simulator code, not verified client or Forever behavior. Only explicit internal attack tables select the Classic resistance path; the public simulator retains inherited TBC resistance.

### Classic level-60 weapon-skill reference

The engine compiles a reference physical attack-table policy that reproduces the player-versus-enemy formulas in [`wowsims/classic` commit `7779ebbf79dc7f1341e6ab939b28a3402c9a730a`](https://github.com/wowsims/classic/blob/7779ebbf79dc7f1341e6ab939b28a3402c9a730a/sim/core/target.go#L192-L349). Literal reference vectors cover a level-60 attacker against equal-level through `+3` targets, including the `+4`/`+5` hit-suppression breakpoint and the `+8` glancing-damage caps. The policy preserves the complete glancing multiplier range; its mean is used for expected-value calculations, not as a replacement for the independent random roll used in actual Classic attacks.

The resolver creates a short-lived value view for a spell and never mutates the shared attacker/defender `AttackTable`. It accepts only explicitly sourced, classified player weapons against enemies, with a melee or ranged defense type matching the source. Missing or unaudited weapon sources, malformed source/category pairs, generic synthetic weapons, pets, player defenders and Wand all fail closed. Wand is excluded because the pinned Classic implementation has neither a Wands skill category nor a Wand case in its weapon-skill lookup. Equipped fist weapons use the Unarmed bonus, while a truly empty main hand ignores that bonus to match Classic's nil-weapon path. The characterized scope deliberately rejects lower-level targets, whose upstream raw formula can produce a negative glance chance. Finite fractional and negative modifiers retain source behavior, but a negative modifier does not model training a weapon from below the assumed capped base skill.

This policy is a comparison fallback, not accepted Forever behavior. In the Classic reference, positive weapon skill changes miss, hit suppression, dodge and glancing damage, but does not change parry, glancing frequency or melee critical suppression. The implementation assumes capped base skill of `5 * attacker level`; it does not model training a weapon from below cap. Its fixed 14% parry chance against a `+3` enemy and its `+1`/`+2` interpolation remain beta-verification targets. It also adds 1.8 percentage points of boss critical suppression to total crit even though the [upstream history](https://github.com/wowsims/sod/pull/170) documents that this approximation should apply only to aura-derived crit.

The active inherited TBC profile keeps this model disabled and retains its fixed attack tables and scalar glancing multiplier. The internal scheduled fixture now consumes the transient view through a narrow runtime guard. Expanding that boundary requires tests for each supported weapon ability and class; activating Forever behavior additionally requires authoritative measurements.

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

Shared Hit/Crit inputs, armor mitigation, magical-resistance outcomes, and weapon-skill categories and bonuses have explicit seams in the modern engine, with a bounded scheduled Classic melee integration. Source-pinned Classic level-60 reference policies, raw base attributes/resources and regression vectors characterize possible fallbacks, while Forever units, populated data and live combat effects remain deliberately unresolved.

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
