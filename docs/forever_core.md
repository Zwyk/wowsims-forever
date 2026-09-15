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
| End-to-end class behavior | Existing seeded per-class `.results` files |

The per-class golden results are the broad regression layer. Do not regenerate them merely to make CI green: a result update must accompany a reviewed mechanics change and explain why each affected class moved.

## Core ruleset boundary

The active core ruleset is selected at build time and remains internal to the Go engine. It contains value-only level, rating, and combat-outcome data; simulations cannot mutate it or choose a different ruleset through proto input. Existing public constants and constructors remain compatibility facades while call sites are migrated incrementally.

The first extraction contains the inherited TBC level bands and both attack-table directions. The second adds combat-rating conversions, level-based NPC critical strike chance, and fixed outcome values such as expertise steps, the dual-wield miss penalty, the spell miss floor, and enemy critical/crushing damage multipliers. Engine-created attack tables cache value copies of the selected rating and outcome rules, providing a profile-local calculation seam without mutable global state.

All extracted values still reproduce the inherited TBC engine exactly. Armor, resistance, weapon skill, class base stats, talents and UI values stay outside this profile until each surface can move in a focused pull request. A later Forever profile will be added alongside the inherited profile, then activated only with matching level-60 base-stat data and UI changes.

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
| Spell, melee, and ranged hit chance are combined | `StatHitRating` is a shared source feeding separate physical and spell hit percentages | Foundation implemented; announcement reviewed in inactive catalog |
| Spell, melee, and ranged crit chance are combined | `StatCritRating` is a shared source feeding separate physical and spell crit percentages | Foundation implemented; announcement reviewed in inactive catalog |
| Weapon skill remains relevant | Preserve category-specific bonus data and weapon context; do not treat current TBC expertise as the final model | Inactive foundation implemented; claim reviewed; effects await beta data |
| Some items can reduce parry chance or dodge chance | Keep dodge and parry reduction distinct until evidence establishes whether one source feeds both | Claim reviewed in inactive catalog; mechanics await beta data |
| Bonus healing contributes one-third as much bonus damage | Add once at the item-data boundary, with a regression test against double counting | Claim reviewed in inactive catalog; runtime implementation pending |

The generic source stats do **not** merge physical and spell outcome tables. Base miss chances, caps, suppression, talents, school modifiers, and attack-specific bonuses remain independent.

## Deliberately unresolved

- Whether level 60 is explicitly the hard player cap rather than only the announced leveling-journey upper bound, and which default boss levels the simulator should offer.
- Whether Hit and Crit use direct percentage points or ratings, and every level-60 conversion value.
- The name, unit, conversion, cap, and rounding behavior of the announced parry/dodge-reduction effects, and whether one shared stat or separate stats feed them.
- Exact Forever weapon-skill effects on miss, dodge, parry, glancing chance, glancing damage, and critical suppression; the compiled Classic level-60 policy is only a pinned comparison fallback.
- Whether Forever uses Classic or TBC armor and resistance formulas in every case.
- Whether the bonus-healing damage contribution is precomputed in item data or derived at runtime, and where any fractional rounding occurs.

Until those values are confirmed, the runnable branch still uses inherited TBC conversion constants and combat tables. Characterization tests pin those placeholders so a later evidence-backed rules update produces an explicit reviewable diff.

Weapon-skill categories and bonuses are now represented by an inactive seam in the modern engine. A source-pinned Classic level-60 reference policy and regression vectors characterize one possible fallback, while Forever units, populated data and live combat effects remain deliberately unresolved.

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
