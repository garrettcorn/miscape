# Changelog

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

[0.3.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.3.0

[0.2.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.2.0

[0.1.1]: https://github.com/garrettcorn/miscape/releases/tag/v0.1.1

[0.1.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.1.0
