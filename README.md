# 🏮 Lanternfall

[![CI](https://github.com/garrettcorn/miscape/actions/workflows/ci.yml/badge.svg)](https://github.com/garrettcorn/miscape/actions/workflows/ci.yml)
[![Release](https://github.com/garrettcorn/miscape/actions/workflows/release.yml/badge.svg)](https://github.com/garrettcorn/miscape/actions/workflows/release.yml)
[![Website](https://github.com/garrettcorn/miscape/actions/workflows/pages.yml/badge.svg)](https://garrettcorn.github.io/miscape/)
[![Latest release](https://img.shields.io/github/v/release/garrettcorn/miscape?label=download)](https://github.com/garrettcorn/miscape/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/garrettcorn/miscape/total)](https://github.com/garrettcorn/miscape/releases)
[![Go version](https://img.shields.io/github/go-mod/go-version/garrettcorn/miscape)](go.mod)
[![Platforms](https://img.shields.io/badge/platforms-Windows%20%7C%20macOS%20%7C%20Linux-informational)](https://garrettcorn.github.io/miscape/#download)
[![License: AGPL v3](https://img.shields.io/badge/license-AGPL--3.0-blue)](LICENSE)
[![Common Changelog](https://common-changelog.org/badge.svg)](CHANGELOG.md)

**🌐 [Download & play: garrettcorn.github.io/miscape](https://garrettcorn.github.io/miscape/)**

A small, cozy multiplayer voxel RPG for the web. Explore the lantern-lit village
of **Wickhollow**, chop and mine, craft gear, help villagers with quests, trade
with friends, and, if you dare, carry a lantern into **the Gloam**.

- **Server:** Go, authoritative, single binary (the client is embedded).
- **Client:** raw WebGL2 + vanilla ES modules. No build step, about 44 KB gzipped, no image downloads.
- **Worlds:** official hosted worlds, or player-hosted private worlds that friends
  join over a [Tailcat](https://github.com/tailscale/tailcat) tunnel.
- **Controls:** gamepad first, plus keyboard/mouse (remappable) and touch.

> Status: **vertical-slice prototype (v0.2.0)**. Everything listed in
> [docs/SCOPE.md](docs/SCOPE.md) is implemented and tested, but it's a first
> release: small content, no real account system, and only measured on the
> devices listed in [docs/PERFORMANCE.md](docs/PERFORMANCE.md).

## Play in your browser

Open **https://garrettcorn.github.io/miscape/play/**. There's nothing to
install. It's a single-player world: the Go game engine runs as WebAssembly
inside the page and saves to this browser (IndexedDB), with automatic backups.
Use **Settings → Export world** to keep a copy or move it to another device.
Browser worlds are their own realm, so their progress stays separate from
hosted worlds.

To build it locally:

```bash
go run ./tools/buildplay
```

```bash
go run ./tools/sitepreview
```

Then open http://127.0.0.1:8150/play/.

## Quick start (play locally)

Requirements: Go 1.27.4+ (the `go` command fetches it automatically if you have an older 1.27).

```bash
go run ./cmd/lanternfall serve
```

Open http://127.0.0.1:23710/, pick a name, and play. World data is saved in `./data/private/`.

## Host a world for friends

```bash
go run ./cmd/lanternfall host
```

Then open the admin screen (http://127.0.0.1:23710/admin), paste the admin token
printed in the terminal, and create an invite. Each friend runs:

```bash
lanternfall join
```

They paste the invite, and their browser opens the game. Full guide: [docs/HOSTING.md](docs/HOSTING.md).

## Build & test

```bash
go test ./...
```

```bash
go build -o bin/lanternfall ./cmd/lanternfall
```

## How to play

| | Controller | Keyboard & mouse | Touch |
|---|---|---|---|
| Move / look | Left / right stick | WASD / mouse (click to capture) | Left stick / drag right side |
| Jump · Sprint | Ⓐ · L3 | Space · Shift | ⤒ · » |
| Attack / gather / remove your block | RT (hold) | Left click (hold) | ⚔ |
| Use item / place block | LT | Right click | ✋ |
| Talk / trade | Ⓧ | E | 💬 |
| Menu (pack, craft, skills, quests, settings) | Ⓨ | Tab | ☰ |
| Hearth Lantern (return home) | D-pad ↓ | H | 🏮 |
| Hotbar | LB / RB | Q / R, 1–8, wheel | tap slot |

Start with **Mabel the Lamplighter** by the plaza. Creatures are auto-targeted,
so you only need to face them roughly.

## Website & releases

- The player website lives in [`site/`](site/) and deploys to GitHub Pages
  through `.github/workflows/pages.yml` whenever `site/`, `web/` or the engine
  changes on `main`. That workflow also builds the in-browser world into
  `/play/` with `tools/buildplay`.
  Preview it locally with `go run ./tools/sitepreview`. Set
  `publicWorldUrl` in [`site/config.js`](site/config.js) once an official world is online.
- To publish a release, first add an entry to [`CHANGELOG.md`](CHANGELOG.md)
  following [Common Changelog](https://common-changelog.org/), with a
  `## [X.Y.Z] - YYYY-MM-DD` heading, Changed/Added/Removed/Fixed groups, and
  commit links. There's no Unreleased section. Commit it, then push a version tag.
  `.github/workflows/release.yml` tests, cross-compiles Windows/macOS/Linux zips,
  and publishes a GitHub Release whose notes are that changelog entry. The release
  fails if the entry is missing. The site's download buttons always point at the
  latest release.

```bash
git tag v0.1.0
```

```bash
git push origin v0.1.0
```

## Documentation

- [Architecture](docs/ARCHITECTURE.md): client tech, Go server, persistence, worlds, Tailcat role and limits
- [First-release scope](docs/SCOPE.md)
- [Hosting a private world](docs/HOSTING.md) (backups, recovery, troubleshooting)
- [Configuration](docs/CONFIG.md) and [deployment](docs/DEPLOY.md)
- [Mods](docs/MODS.md)
- [Performance budgets & measurements](docs/PERFORMANCE.md)
- [Roadmap](docs/ROADMAP.md)

## Layout

```
cmd/lanternfall     CLI: serve, host, join, backups, restore, mods check
internal/game       authoritative rules (movement, gather, craft, combat, quests, trade, access)
internal/world      voxel grid, curated world builder, chunk RLE
internal/content    data-driven content + declarative mod loader (base.json)
internal/store      atomic checksummed snapshots, backups, recovery
internal/protocol   versioned wire format + validation
internal/local      single-player browser world host (runs as WebAssembly)
internal/server     HTTP/WebSocket sessions, game loop, autosave, admin API
internal/p2p        Tailcat host listener + friend join companion
cmd/lanternfall-web engine entry point for the browser (GOOS=js GOARCH=wasm)
tools/              buildplay (assembles site/play/), sitepreview
web/                browser client (embedded into the binary)
mods/               example mod
```

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md). First-time contributors sign the
[Contributor License Agreement](CLA.md) by leaving a one-line comment on their pull request; a bot walks you through it.
You keep your copyright, and the CLA guarantees your contribution stays open source.

## License

Copyright © 2026 Garrett Cornwell.

Lanternfall is free software: you can redistribute it and/or modify it under the
terms of the [GNU Affero General Public License v3.0](LICENSE) (or, at your option, any later version).

In short: you may use, study, change and share it, including commercially, but
if you distribute it **or run a modified version that other people connect to
over a network**, you must make your full source code available under the same
license. Running a modified server? Set `LF_SOURCE_URL` to where your source is
published. It's shown to players on the join screen and in `/api/info`.

The AGPL does not grant rights to the **Lanternfall** name or logo. Please
don't present a fork as the official game. Commercial licenses for uses the
AGPL doesn't fit may be available from the copyright holder.

Third-party dependencies keep their own licenses (Tailcat and Tailscale:
BSD-3-Clause; coder/websocket: ISC), all compatible with the AGPL.
