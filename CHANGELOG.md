# Changelog

## [0.9.1] - 2026-11-26

### Fixed

- Fix controller navigation jumping into the menu tabs and close button, which made Craft, Pack and other menus hard to use ([`e257271`](https://github.com/garrettcorn/miscape/commit/e257271))
- Fix controller navigation skipping rows of controls, such as most of Settings ([`e257271`](https://github.com/garrettcorn/miscape/commit/e257271))
- Fix the world map trapping the D-pad; the right stick now pans it and the triggers zoom ([`e257271`](https://github.com/garrettcorn/miscape/commit/e257271))

## [0.9.0] - 2026-11-26

_If you are upgrading: saves migrate on load (characters get adventure points for quests they had finished, and remember Manchester). Update servers, join companions and browsers together (protocol 6)._

### Changed

- **Breaking:** bump the protocol to version 6 and the save format to version 5 ([`961a0a8`](https://github.com/garrettcorn/miscape/commit/961a0a8))
- Keep earning XP at half rate past the world's level cap; it counts toward the next world ([`4514acc`](https://github.com/garrettcorn/miscape/commit/4514acc))
- Gather rowan, yew and ores in hotspots rather than scattered across the countryside ([`961a0a8`](https://github.com/garrettcorn/miscape/commit/961a0a8))
- Let the Hearth Lantern take you to any town you have visited, and wake knocked-out players in the nearest one ([`961a0a8`](https://github.com/garrettcorn/miscape/commit/961a0a8))
- Find better fish and snare catches further from home ([`961a0a8`](https://github.com/garrettcorn/miscape/commit/961a0a8))

### Added

- Add adventure points: every quest gives some, and later quests need them before they start ([`4514acc`](https://github.com/garrettcorn/miscape/commit/4514acc))
- Add 13 towns across every continent, each known for particular skills ([`961a0a8`](https://github.com/garrettcorn/miscape/commit/961a0a8))
- Add danger levels that rise with distance from Manchester, shown in the HUD and as a world-map overlay ([`961a0a8`](https://github.com/garrettcorn/miscape/commit/961a0a8))
- Add mines, woods, fishing grounds and hunting grounds whose contents follow the danger level, marked on the world map and minimap ([`961a0a8`](https://github.com/garrettcorn/miscape/commit/961a0a8))
- Add roaming creatures around players in the wilds, as strong as the danger where they appear ([`961a0a8`](https://github.com/garrettcorn/miscape/commit/961a0a8))
- Add ten regional folklore creatures, from Black Shuck to the bunyip ([`76114d1`](https://github.com/garrettcorn/miscape/commit/76114d1))
- Add people, shops and 15 quests to the new towns, warrants in Sydney, and agility courses in Rio and Sydney ([`76114d1`](https://github.com/garrettcorn/miscape/commit/76114d1))
- Add silver ore and silver arrows ([`76114d1`](https://github.com/garrettcorn/miscape/commit/76114d1))

## [0.8.2] - 2026-11-26

### Fixed

- Fix buying an item from a shopkeeper when another shopkeeper also sells it, such as salt from Nance ([`cca1dab`](https://github.com/garrettcorn/miscape/commit/cca1dab))
- Fix selling a whole stack of more than 64 items ([`cca1dab`](https://github.com/garrettcorn/miscape/commit/cca1dab))

## [0.8.1] - 2026-11-26

### Fixed

- Fix the Craft tab not opening (broken since 0.5.0) ([`b542fc0`](https://github.com/garrettcorn/miscape/commit/b542fc0))

## [0.8.0] - 2026-11-26

_If you are upgrading: saves carry over unchanged. Update servers, join companions and browsers together (protocol 5)._

### Changed

- **Breaking:** bump the protocol to version 5 for thieving and slaying ([`4b08676`](https://github.com/garrettcorn/miscape/commit/4b08676))
- Stop creatures spawning inside towns ([`4b08676`](https://github.com/garrettcorn/miscape/commit/4b08676))

### Added

- Add pickpocketing townsfolk from the talk dialog, with a stun and a fine from the watch if you're caught; players can never be robbed ([`4b08676`](https://github.com/garrettcorn/miscape/commit/4b08676))
- Add market stalls to rob for Eccles cakes, cotton and silk, which restock after a while ([`4b08676`](https://github.com/garrettcorn/miscape/commit/4b08676))
- Add Josiah Rawcliffe the magistrate, who hands out bounty warrants that train Slaying and pay on completion ([`4b08676`](https://github.com/garrettcorn/miscape/commit/4b08676))
- Add the Boggart of the Clough, a creature that needs Slaying 8 ([`4b08676`](https://github.com/garrettcorn/miscape/commit/4b08676))

## [0.7.0] - 2026-11-26

_If you are upgrading: saves carry over unchanged. Update servers, join companions and browsers together (protocol 4)._

### Changed

- **Breaking:** bump the protocol to version 4 for snares, crops and courses ([`5ec7fca`](https://github.com/garrettcorn/miscape/commit/5ec7fca))
- Let every Agility level add 1% to sprint speed, allowed for by the server's speed check ([`5ec7fca`](https://github.com/garrettcorn/miscape/commit/5ec7fca))
- Count creature loot toward "gather" quest steps ([`5ec7fca`](https://github.com/garrettcorn/miscape/commit/5ec7fca))

### Added

- Add hares, red grouse and red deer that run from players; kills train Hunting, and grouse and deer need Hunting levels ([`5ec7fca`](https://github.com/garrettcorn/miscape/commit/5ec7fca))
- Add snares that catch hares, grouse and foxes on open ground while you wait ([`5ec7fca`](https://github.com/garrettcorn/miscape/commit/5ec7fca))
- Add an allotment in Manchester where potatoes, oats, cabbages and feverfew grow in real time, even while you're offline ([`5ec7fca`](https://github.com/garrettcorn/miscape/commit/5ec7fca))
- Add two agility courses, the Irk Valley Run and the Pennine Fell Race, run through checkpoint flags against the clock ([`5ec7fca`](https://github.com/garrettcorn/miscape/commit/5ec7fca))
- Add game and garden cooking, including lobscouse, plus snares and feather-fletched arrows ([`5ec7fca`](https://github.com/garrettcorn/miscape/commit/5ec7fca))
- Add Abel Fothergill the gamekeeper and Jonas Ogden the smallholder, with a quest each ([`5ec7fca`](https://github.com/garrettcorn/miscape/commit/5ec7fca))

## [0.6.0] - 2026-11-26

### Changed

- Let range and magic weapons shoot up to their reach, checking line of sight and using up ammunition on the server ([`9fce5d9`](https://github.com/garrettcorn/miscape/commit/9fce5d9))
- Show charms you know on the hotbar, and active charms as countdown chips by the health bar ([`9fce5d9`](https://github.com/garrettcorn/miscape/commit/9fce5d9))

### Added

- Add slings, rowan shortbows and yew longbows, with stones, copper arrows and iron arrows as ammunition ([`9fce5d9`](https://github.com/garrettcorn/miscape/commit/9fce5d9))
- Add rowan wands and wisp-light staffs that throw will-light, spending salt ([`9fce5d9`](https://github.com/garrettcorn/miscape/commit/9fce5d9))
- Add three charms: Hearth Charm (heal), Rowan Ward (less damage taken) and Boggart-bane (double damage against boggarts) ([`9fce5d9`](https://github.com/garrettcorn/miscape/commit/9fce5d9))
- Add a bronze sword and recipes for slings, arrows, wands, bows and the staff ([`9fce5d9`](https://github.com/garrettcorn/miscape/commit/9fce5d9))
- Add Nance Clegg the cunning-woman and her quest "Salt and Rowan" ([`9fce5d9`](https://github.com/garrettcorn/miscape/commit/9fce5d9))

## [0.5.0] - 2026-11-26

_If you are upgrading: characters keep everything, and their combat XP becomes Melee XP. Update servers, join companions and browsers together (protocol 3)._

### Changed

- **Breaking:** bump the protocol to version 3 ([`326e865`](https://github.com/garrettcorn/miscape/commit/326e865))
- **Breaking:** bump the save format to version 3, turning combat XP into Melee XP ([`15e0d29`](https://github.com/garrettcorn/miscape/commit/15e0d29))
- Replace the four skills with 14 grouped skills defined in content: Melee, Range, Magic, Slaying, Mining, Woodcutting, Fishing, Hunting, Farming, Crafting, Cooking, Firemaking, Agility and Thieving ([`15e0d29`](https://github.com/garrettcorn/miscape/commit/15e0d29))
- Cap skill levels at 10 × the world level, so level 1 worlds go up to 10 ([`15e0d29`](https://github.com/garrettcorn/miscape/commit/15e0d29))
- Train and power each weapon from its style's skill, and base health on combat level ([`15e0d29`](https://github.com/garrettcorn/miscape/commit/15e0d29))
- Redesign the Skills tab with groups, progress bars, the level cap and each skill's next unlock ([`15e0d29`](https://github.com/garrettcorn/miscape/commit/15e0d29))
- Name the countryside after its continent in the HUD, for example "the European countryside" ([`de08041`](https://github.com/garrettcorn/miscape/commit/de08041))

### Added

- Add fishing at rippling spots along the coasts, with herring, trout, salmon and cod by level ([`82d7f0f`](https://github.com/garrettcorn/miscape/commit/82d7f0f))
- Add campfires, lit from logs or coal with a tinderbox, that light the dark, keep wisps away and burn out ([`82d7f0f`](https://github.com/garrettcorn/miscape/commit/82d7f0f))
- Add cooking at a campfire or Manchester's kitchen range, with a chance to burn food at low levels ([`82d7f0f`](https://github.com/garrettcorn/miscape/commit/82d7f0f))
- Add rowan and yew trees, tin, coal and iron outcrops, and an iron sword ([`82d7f0f`](https://github.com/garrettcorn/miscape/commit/82d7f0f))
- Add Hannah Leigh the fishwife and her quest "Supper for the Mill Hands" ([`82d7f0f`](https://github.com/garrettcorn/miscape/commit/82d7f0f))
- Add skill requirements to recipes, gathering, equipment and quests ([`15e0d29`](https://github.com/garrettcorn/miscape/commit/15e0d29))
- Add XP popups and level-up messages that name what each level unlocks ([`15e0d29`](https://github.com/garrettcorn/miscape/commit/15e0d29))
- Add a minimap and a full world map ([`933a87c`](https://github.com/garrettcorn/miscape/commit/933a87c))
- Add land relief and ocean depths to the world map ([`4a18d44`](https://github.com/garrettcorn/miscape/commit/4a18d44))
- Add continent and ocean labels to the world map ([`de08041`](https://github.com/garrettcorn/miscape/commit/de08041))
- Add swimming, with a swim animation, and climbing out of water onto land ([`4a18d44`](https://github.com/garrettcorn/miscape/commit/4a18d44))

### Fixed

- Fix the lantern staying at the hip instead of swinging in the hand ([`4a18d44`](https://github.com/garrettcorn/miscape/commit/4a18d44))
- Fix Ottiwell and the warning sign standing over water outside Manchester ([`4a18d44`](https://github.com/garrettcorn/miscape/commit/4a18d44))

## [0.3.0] - 2026-11-26

_If you are upgrading: characters from 0.2.0 worlds keep their items, skills and quests and start in Manchester; blocks placed on the old map are not carried over. Update servers, join companions and browsers together (protocol 2)._

### Changed

- **Breaking:** replace the 128×128 map with Earth (3072×1536 blocks) generated from a map image, wrapping east to west ([`bfde315`](https://github.com/garrettcorn/miscape/commit/bfde315))
- **Breaking:** bump the protocol to version 2 and the save format to version 2, migrating version 1 saves on load ([`bfde315`](https://github.com/garrettcorn/miscape/commit/bfde315))
- Start in Manchester around 1807, with period townsfolk, boggarts and will-o'-the-wisps on the Highland moors ([`aa03b58`](https://github.com/garrettcorn/miscape/commit/aa03b58))
- Rework the pack, crafting and shop menus with item details, filters and icons ([`7d0013d`](https://github.com/garrettcorn/miscape/commit/7d0013d))
- Hold weapons in the hand with proper sword models ([`7d0013d`](https://github.com/garrettcorn/miscape/commit/7d0013d))
- Speed up the game loop 4–6× with many players by sharing snapshot entries ([`3a5bff1`](https://github.com/garrettcorn/miscape/commit/3a5bff1))
- Send block changes only to nearby players ([`bfde315`](https://github.com/garrettcorn/miscape/commit/bfde315))

### Added

- Add world levels defined by map files; level 2 inherits level 1 at twice the scale ([`bfde315`](https://github.com/garrettcorn/miscape/commit/bfde315))
- Add `tools/mkmap` to make terrain images from a world map ([`bfde315`](https://github.com/garrettcorn/miscape/commit/bfde315))
- Add snow and ice blocks ([`bfde315`](https://github.com/garrettcorn/miscape/commit/bfde315))
- Add dropping items, which stay on the ground for 10 minutes and are picked up by walking over them ([`7d0013d`](https://github.com/garrettcorn/miscape/commit/7d0013d))
- Add an in-game confirm dialog and a drop key (G, or D-pad left) ([`7d0013d`](https://github.com/garrettcorn/miscape/commit/7d0013d))
- Add a guide to extending the game, `docs/EXTENDING.md` ([`3a5bff1`](https://github.com/garrettcorn/miscape/commit/3a5bff1), [`e8f27da`](https://github.com/garrettcorn/miscape/commit/e8f27da))

### Fixed

- Fix the view shaking while standing still at high frame rates ([`efde0d9`](https://github.com/garrettcorn/miscape/commit/efde0d9))
- Fix the HUD calling browser worlds "Private world" ([`7d0013d`](https://github.com/garrettcorn/miscape/commit/7d0013d))

## [0.2.0] - 2026-11-25

### Changed

- Restyle the game UI and website with a translucent glass theme ([`3ec71a3`](https://github.com/garrettcorn/miscape/commit/3ec71a3))

### Added

- Add a single-player world that runs entirely in the browser at [`/play/`](https://garrettcorn.github.io/miscape/play/), with the Go engine compiled to WebAssembly ([`f6bbcc8`](https://github.com/garrettcorn/miscape/commit/f6bbcc8))
- Save browser worlds in IndexedDB with autosave, rotating backups and recovery from a corrupt save ([`f6bbcc8`](https://github.com/garrettcorn/miscape/commit/f6bbcc8))
- Add export and import of browser worlds, kept separate from hosted worlds ([`f6bbcc8`](https://github.com/garrettcorn/miscape/commit/f6bbcc8))
- Add a "Reduce transparency" setting that switches to solid panels ([`3ec71a3`](https://github.com/garrettcorn/miscape/commit/3ec71a3))
- Add a changelog ([`8bf69b3`](https://github.com/garrettcorn/miscape/commit/8bf69b3))

## [0.1.1] - 2026-11-25

_If you are upgrading: browsers remember characters per address, so to keep playing existing characters start the world with `LF_ADDR=127.0.0.1:8080`._

### Changed

- **Breaking:** change default ports to `23710` for worlds and `23711` for `lanternfall join` ([`66fdab3`](https://github.com/garrettcorn/miscape/commit/66fdab3))

### Added

- License under the GNU AGPL v3.0 or later ([`9a8b4b4`](https://github.com/garrettcorn/miscape/commit/9a8b4b4))
- Show a source code link on the join screen and in `/api/info`, configurable with `LF_SOURCE_URL` ([`9a8b4b4`](https://github.com/garrettcorn/miscape/commit/9a8b4b4))
- Add contributor license agreement and contributing guide ([`8ecee08`](https://github.com/garrettcorn/miscape/commit/8ecee08))

## [0.1.0] - 2026-11-25

_Initial release._

[0.9.1]: https://github.com/garrettcorn/miscape/releases/tag/v0.9.1

[0.9.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.9.0

[0.8.2]: https://github.com/garrettcorn/miscape/releases/tag/v0.8.2

[0.8.1]: https://github.com/garrettcorn/miscape/releases/tag/v0.8.1

[0.8.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.8.0

[0.7.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.7.0

[0.6.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.6.0

[0.5.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.5.0

[0.3.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.3.0

[0.2.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.2.0

[0.1.1]: https://github.com/garrettcorn/miscape/releases/tag/v0.1.1

[0.1.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.1.0
