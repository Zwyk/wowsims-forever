# Forever Retribution DPS simulator

The Pages root now exposes a single-player Ret simulator through
`core.ForeverRetJSON` and the `foreverRet` WASM callback. It uses the existing
combat event loop, auto-attacks, mana bar, spell outcomes and metrics. A custom
priority action runs within the native APL scheduler, reevaluating idle time
at 100 ms. The interface supplies manual bonus stats, weapon damage/speed,
talents, rotation order, seal management and encounter parameters. It stores
builds locally, shares complete inputs in URL fragments and downloads results
with their exact configuration and model assumptions. A Web Worker keeps the
page responsive and permits cancellation by terminating the simulation.

## Scope

- One level-60 pre-racial Human Paladin, two-handed sword, rear attacks.
- One passive level 60–63 Humanoid, Demon or Undead; no incoming damage.
- Forever Retribution tree plus the first four Holy/Protection tiers, with at
  most 20 points in each off-tree and the normal 51-point budget. Unknown ranks,
  missing prerequisites and invalid row allocations are rejected.
- Command/Righteousness, optional Crusader opener, Holy Strike, Judgement,
  Consecration, Exorcism, Holy Wrath, Hammer of Wrath and Swift Judgement.
- Self Might, Wisdom or Kings before the pull, lasting one hour as observed. Classic trainer blessing ranks
  remain the fallback; additional external stats can be entered manually.
- Manual shared bonus hit and crit feed physical and spell channels; bonus
  healing supplies one-third additional spell damage. Avoidance reduction is
  entered directly as percentage points, without inventing an expertise rating.
- No equipment catalog, items/procs, raid actors, healing/tanking simulations,
  race selector, movement model or automatic external buffs/debuffs.

Defensive, healing and control talent selections remain available for legal
builds but have no DPS effect in this stationary encounter. Full Holy and
Protection trees are displayed with unsupported tiers disabled. This does not
claim every Paladin role or Forever race is finished.

## Evidence and explicit assumptions

Tree/tooltip data is adapted from [TalentsForever](https://talentsforever.com),
CC BY 4.0, using the reviewed 2026-09-16 snapshot in
`third_party/talentsforever`. Raw SHA256:
`20780eafb7235ede6ee19355bc7cbc4e5d3ea0d1584079a91c80c67b84961a27`.
Generate the derivative with `node tools/classic60preview/generate-ret-catalog.mjs`;
CI checks it matches the vendored source. The source remains secondary demo
evidence. Its general catalog is still `reference_only`; this explicitly scoped
runtime overlay is a separate decision.

[Blizzard's Deep Dive recap](https://news.blizzard.com/en-us/article/24303313/world-of-warcraft-forever-deep-dive-panel-recap)
supports shared Hit/Crit, continued weapon skill and one-third bonus-healing
damage. Existing Classic level-60 spell ranks, coefficients, armor/resistance,
weapon skill and mana mechanics remain explicit fallbacks, pinned by the
[core ledger](forever_core.md). The new layer does not import a TBC constructor,
TBC talent schema or item database, and is not enabled for other profiles.

| Mechanic | Runtime interpretation |
| --- | --- |
| Judgement | Keeps the current seal, including on misses. Classic payloads and cooldowns remain. |
| Holy Strike | Editable weapon %, flat min/max and mana. Defaults: observed level-38 35% + 10–13 and 16 mana. Normal weapon damage, zero SP coefficient and Classic Holy melee outcome are assumptions. Reporting ID 17143 is not a verified Forever rank ID. |
| Improved Holy Strike / Sacred Arbiter | 1/2 sec cooldown reduction; 10% damage and landed-hit refresh of this player's active Judgement debuff. |
| Benediction / Holy Conduit | 2% per rank on currently instant spells; 20% per rank on the listed four spells; multipliers combine. Instrument of Law can make Hammer instant. |
| Improved Seals / 2H specialization | Inferred 5/10/15% seal and damaging Judgement damage; 3/6/9% weapon damage. Classic Command proc/Judgement weapon-specialization behavior is retained, and Holy Strike receives it. |
| Vengeance | Inferred 1/2/3% Physical/Holy damage per stack, five stacks, refreshed to 30 sec by dealt crits. |
| Sanctified Judgement | 33/66/100% chance to refund 20/40/60% of the seal's modified mana cost, regardless of hit. Rank 2 and modified-cost basis are assumptions. |
| Champion of the Light | 33/66/100% Intellect to spell damage and healing. No extra one-third conversion of this talent's healing bonus. |
| Reverence / Purifying Power | 10/20/30% casting Spirit regeneration; Exorcism/Holy Wrath cooldown × 0.835/0.67. Unobserved ranks are interpolated. |
| Crusade | 1/2% all damage, doubled additively against Demon/Undead. |
| Vindication | 1/2/3% AP for 30 sec. Editable 3 PPM Classic fallback, landed white attacks/Holy Strike only; target AP reduction is irrelevant to a passive target. |
| Twist of Light | Replacing Command or Righteousness banks its damage proc. Next landed white attack or Holy Strike triggers it once, without another PPM roll. Damage procs cannot recursively consume Echo. These eligibility/guaranteed-trigger details require measurement. Active seal procs retain Classic white-swing-only triggers; Holy Strike interaction remains unresolved. Fury/Justice echoes have no modeled DPS role here. |
| Swift Judgement | 60 sec cooldown, off GCD, resets Judgement and makes the next cast free. Synthetic report ID 900001. |

Holy Strike is deliberately not presented as a known level-60 rank. Results
can compare assumptions but should not be quoted as verified release DPS.
The default synthetic stats and 18/0/33 build are a starting example, not a gear
recommendation or optimized build.

## Verification

Native tests cover public JSON validation, deterministic seeded runs, damage
accounting, execute phase, shared hit, weapon/avoidance inputs, seal retention,
refund costs, Swift Judgement, Echo recursion and Vengeance iteration cleanup.
The WASM smoke test runs an actual Ret fight and all 80 existing baseline cases.
The Chrome CI test loads the artifact at `/wowsims-forever/`, changes inputs,
runs DPS, rejects an invalid talent build, cancels/restarts, reloads a saved
build and checks narrow-screen talent layout. Existing simulator regression
suites continue to exercise the inherited profiles.

Next classes should reuse the public request/result/worker pattern while adding
an independently sourced class model and validated talent schema. They must not
be enabled merely by swapping the displayed class name.
