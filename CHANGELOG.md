# Changelog

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

[0.6.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.6.0

[0.5.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.5.0

[0.3.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.3.0

[0.2.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.2.0

[0.1.1]: https://github.com/garrettcorn/miscape/releases/tag/v0.1.1

[0.1.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.1.0
