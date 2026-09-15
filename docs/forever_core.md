# Forever core rules ledger

This document separates announced Forever behavior from assumptions inherited from TBC or Classic. Numeric rules stay provisional until they can be measured against a Forever client build.

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

## Evidence

- Blizzard: [World of Warcraft: Forever Deep Dive Panel Recap](https://news.blizzard.com/en-us/article/24303313/world-of-warcraft-forever-deep-dive-panel-recap)
- Blizzard: [World of Warcraft: Forever — What's Next Panel Recap](https://news.blizzard.com/en-us/article/24303862/world-of-warcraft-forever-whats-next-panel-recap)
