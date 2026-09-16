# WoWSims Forever (experimental, unofficial)

This repository is an early community port of [WoWSims TBC](https://github.com/wowsims/tbc-new) for World of Warcraft: Forever. It is a staging area while the WoWSims community decides where the long-term project should live.

> [!WARNING]
> The simulator still contains TBC rules, level-70 data, talents, spells, and encounter defaults. Its current output is **not valid for Forever**.

The port starts from the modern `tbc-new` architecture at commit [`17a8fb28c5ad14b649acecdaacd488594048f467`](https://github.com/wowsims/tbc-new/commit/17a8fb28c5ad14b649acecdaacd488594048f467) and is synchronized through [`9fa04e0675354c1fa2167b83171bbfce5df492ef`](https://github.com/wowsims/tbc-new/commit/9fa04e0675354c1fa2167b83171bbfce5df492ef). We will preserve that architecture while replacing the game model in small, tested steps:

- establish level-60 and level-63 encounter foundations;
- model Forever's shared hit and critical-strike item stats without merging the physical and spell outcome tables;
- restore weapon-skill mechanics from Classic where Forever matches them;
- keep dodge/parry reduction and numeric conversions provisional until beta data confirms them;
- port class, race, talent, spell, and item changes after the core rules are stable.

The inherited deployment, release, database-update, labeling, and webhook workflows remain disabled. Test CI and a new, narrowly scoped [GitHub Pages preview workflow](docs/forever_pages.md) are active. Pages publishes a standalone Classic level-60 **stat diagnostic**, not the inherited TBC simulator and not a complete Classic or Forever combat simulator.

The finished site will retain the original WoWSims landing page, class navigation, simulator controls and results layout, with Forever branding and supported data. The baseline lab is a temporary development tool, not the final interface. Engine integration now includes a bounded level-60 main-hand combat fixture; see the [current scope](docs/forever_core.md#scheduled-classic-60-melee-integration).

## Development

See the [core rules ledger](docs/forever_core.md), [installation guide](docs/installation.md), and [development commands](docs/commands.md). Work should be based on a feature branch; keep `master` available for syncing the upstream `tbc-new` baseline.

## Community and attribution

For coordination with the upstream project, join the [WoWSims Discord](https://discord.gg/jJMPr9JWwx). This fork retains the upstream MIT license and visible attribution requested by the WoWSims maintainers.
