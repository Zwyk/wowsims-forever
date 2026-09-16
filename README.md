# WoWSims Forever (experimental, unofficial)

This repository is an early community port of [WoWSims TBC](https://github.com/wowsims/tbc-new) for World of Warcraft: Forever. It is a staging area while the WoWSims community decides where the long-term project should live.

> [!WARNING]
> The simulator still contains TBC rules, level-70 data, talents, spells, and encounter defaults. Its current output is **not valid for Forever**.

The port starts from the modern `tbc-new` architecture at commit [`17a8fb28c5ad14b649acecdaacd488594048f467`](https://github.com/wowsims/tbc-new/commit/17a8fb28c5ad14b649acecdaacd488594048f467) and is synchronized through [`9fa04e0675354c1fa2167b83171bbfce5df492ef`](https://github.com/wowsims/tbc-new/commit/9fa04e0675354c1fa2167b83171bbfce5df492ef). We will preserve that architecture while replacing the game model in small, tested steps:

- establish level-60 and level-63 encounter foundations;
- model Forever's shared hit and critical-strike item stats without merging the physical and spell outcome tables;
- restore weapon-skill mechanics from Classic where Forever matches them;
- keep dodge/parry reduction and numeric conversions provisional until beta data confirms them;
- port class, race, talent and spell changes after the core rules are stable; gear imports are outside the current scope.

The inherited deployment, release, database-update, labeling, and webhook workflows remain disabled. Test CI and a new, narrowly scoped [GitHub Pages preview workflow](docs/forever_pages.md) are active. Pages publishes a standalone Classic level-60 **stat diagnostic**, not the inherited TBC simulator and not a complete Classic or Forever combat simulator.

The finished site will retain the original WoWSims landing page, class navigation, simulator controls and results layout, with Forever branding and supported data. The baseline lab is a temporary development tool, not the final interface. Engine integration now includes bounded level-60 single-weapon, dual-wield, melee-special and spell-combat fixtures; see the [melee scope](docs/forever_core.md#scheduled-classic-60-melee-integration) and [spell scope](docs/forever_core.md#scheduled-classic-60-spell-integration).

Current priority: **Paladin mechanics, talents and base abilities**, beginning with Retribution. The [Classic Paladin build](docs/forever_core.md#classic-60-paladin-talents-and-offensive-abilities) validates all 44 Classic talent entries and runs a supported 51-point build with two-handed attacks, Command/Judgement, offensive spells, self blessings, a selected aura and mana recovery through the native WoWSims APL. Only the documented subset of talent effects is executable; healing, defensive and remaining seal mechanics still need work. Gear data and presets are excluded from this milestone; tests use synthetic weapons. These remain private engine integrations, and a complete playable Paladin simulator is not available yet. The earlier [Mage Fireball integration](docs/forever_core.md#classic-60-mage-mana-and-fireball-integration) remains a regression reference.

## Development

See the [core rules ledger](docs/forever_core.md), [installation guide](docs/installation.md), and [development commands](docs/commands.md). Work should be based on a feature branch; keep `master` available for syncing the upstream `tbc-new` baseline.

## Community and attribution

For coordination with the upstream project, join the [WoWSims Discord](https://discord.gg/jJMPr9JWwx). This fork retains the upstream MIT license and visible attribution requested by the WoWSims maintainers.
