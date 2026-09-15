# Forever core rules ledger

This document separates announced Forever behavior from assumptions inherited from TBC or Classic. Numeric rules stay provisional until they can be measured against a Forever client build.

## Baseline provenance

The modern architecture baseline is [`wowsims/tbc-new` v0.0.137](https://github.com/wowsims/tbc-new/tree/17a8fb28c5ad14b649acecdaacd488594048f467), commit `17a8fb28c5ad14b649acecdaacd488594048f467`. The first green Forever integration baseline is commit `6cff8086e8e2550856288d71cc9d72bee3e60e4a`.

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

The active core ruleset is selected at build time and remains internal to the Go engine. It contains value-only level and attack-table data; simulations cannot mutate it or choose a different ruleset through proto input. Existing public constants and constructors remain compatibility facades while call sites are migrated incrementally.

The first extraction contains only the inherited TBC level bands and both attack-table directions. Rating conversions, outcome caps, armor, resistance, weapon skill, talents and UI values stay outside this profile until each surface can move in a focused, zero-behavior-change pull request. A later Forever profile will be added alongside the inherited profile, then activated only with the matching level-60 data and UI changes.

### Known inherited quirks

- A level-69 target is not offered by the inherited encounter picker. Direct API input at that level falls through to the `+3` lookup values because the level table has no `-1` entry; this unsupported-input behavior is documented but deliberately not made a desired invariant.
- The inherited non-binary resistance formula gives a level-73 target 4.5% average mitigation when it has no explicit resistance, despite a nearby production comment saying 6%. Adding explicit resistance also changes how that level component is scaled. The tests pin the observed outputs for safe refactoring; they do not validate the formula for Forever.
- The expected-value path for a blocked weapon special appears internally inconsistent. This PR characterizes front-facing white attacks and rear-facing specials, but leaves that suspicious path for a focused bug investigation rather than legitimizing it as baseline behavior.

## Confirmed direction

| Rule | Engine decision | Status |
| --- | --- | --- |
| Maximum player level is 60 | Level 60 and default boss level 63 will be installed atomically with matching base-stat data | Not implemented |
| Melee, ranged, and spell hit are one item stat | `StatHitRating` is a shared source feeding separate physical and spell hit percentages | Foundation implemented |
| Melee, ranged, and spell crit are one item stat | `StatCritRating` is a shared source feeding separate physical and spell crit percentages | Foundation implemented |
| Weapon skill remains relevant | Preserve room for per-weapon skill; do not treat current TBC expertise as the final model | Awaiting beta data |
| Some items reduce dodge/parry chance | Keep dodge and parry reduction distinct internally, even if one item stat eventually feeds both | Awaiting beta data |
| Bonus healing contributes one-third as much bonus damage | Add once at the item-data boundary, with a regression test against double counting | Not implemented |

The generic source stats do **not** merge physical and spell outcome tables. Base miss chances, caps, suppression, talents, school modifiers, and attack-specific bonuses remain independent.

## Deliberately unresolved

- Whether Hit and Crit use direct percentage points or ratings, and every level-60 conversion value.
- The name, unit, conversion, cap, and rounding behavior of the announced dodge/parry-reduction item stat.
- Exact weapon-skill effects on miss, dodge, parry, glancing chance, glancing damage, and critical suppression.
- Whether Forever uses Classic or TBC armor and resistance formulas in every case.

Until those values are confirmed, the runnable branch still uses inherited TBC conversion constants and combat tables. Characterization tests pin those placeholders so a later evidence-backed rules update produces an explicit reviewable diff.

Weapon skill is not represented by the inherited `tbc-new` combat model. Its absence is recorded here rather than treated as expected Forever behavior; it will need a new weapon-aware seam in the modern engine.

## Evidence

- Blizzard: [World of Warcraft: Forever Deep Dive Panel Recap](https://news.blizzard.com/en-us/article/24303313/world-of-warcraft-forever-deep-dive-panel-recap)
- Blizzard: [World of Warcraft: Forever — What's Next Panel Recap](https://news.blizzard.com/en-us/article/24303862/world-of-warcraft-forever-whats-next-panel-recap)
