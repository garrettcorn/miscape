# 🏮 Lanternfall

A small, cozy multiplayer voxel RPG for the web. Explore the lantern-lit village
of **Wickhollow**, chop and mine, craft gear, help villagers with quests, trade
with friends, and, if you dare, carry a lantern into **the Gloam**.

- **Server:** Go, authoritative, single binary (the client is embedded).
- **Client:** raw WebGL2 + vanilla ES modules. No build step, about 44 KB gzipped, no image downloads.
- **Worlds:** official hosted worlds, or player-hosted private worlds that friends
  join over a [Tailcat](https://github.com/tailscale/tailcat) tunnel.
- **Controls:** gamepad first, plus keyboard/mouse (remappable) and touch.

> Status: **vertical-slice prototype (v0.1.0)**. Everything listed in
> [docs/SCOPE.md](docs/SCOPE.md) is implemented and tested, but it's a first
> release: small content, no real account system, and only measured on the
> devices listed in [docs/PERFORMANCE.md](docs/PERFORMANCE.md).

## Quick start (play locally)

Requirements: Go 1.27.4+ (the `go` command fetches it automatically if you have an older 1.27).

```bash
go run ./cmd/lanternfall serve
```

Open http://127.0.0.1:8080/, pick a name, and play. World data is saved in `./data/private/`.

## Host a world for friends

```bash
go run ./cmd/lanternfall host
```

Then open the admin screen (http://127.0.0.1:8080/admin), paste the admin token
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
  through `.github/workflows/pages.yml` whenever `site/` changes on `main`.
  Preview it locally with `go run ./tools/sitepreview`. Set
  `publicWorldUrl` in [`site/config.js`](site/config.js) once an official world is online.
- To publish a release, push a version tag. `.github/workflows/release.yml`
  tests, cross-compiles Windows/macOS/Linux zips, and attaches them to a
  GitHub Release. The site's download buttons always point at the latest one.

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
internal/server     HTTP/WebSocket sessions, game loop, autosave, admin API
internal/p2p        Tailcat host listener + friend join companion
web/                browser client (embedded into the binary)
mods/               example mod
```
