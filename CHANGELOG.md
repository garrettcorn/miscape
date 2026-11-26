# Changelog

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

[0.2.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.2.0

[0.1.1]: https://github.com/garrettcorn/miscape/releases/tag/v0.1.1

[0.1.0]: https://github.com/garrettcorn/miscape/releases/tag/v0.1.0
