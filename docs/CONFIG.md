# Configuration

## Environment variables (process settings)

| Variable | Default | Meaning |
|---|---|---|
| `LF_ADDR` | `127.0.0.1:8080` (or `127.0.0.1:$PORT`) | HTTP/WebSocket listen address. Private worlds should stay on loopback. |
| `LF_MODE` | `private` (`serve --mode`) | `official` or `private`. A data dir created in one mode refuses to open in the other. |
| `LF_DATA_DIR` | `./data/<mode>` | World folder: `world.json`, `state.json`, `backups/`, secrets, `mods/`. |
| `LF_MODS_DIR` | `<data>/mods` | Where mods are discovered. |
| `LF_AUTOSAVE_SEC` | `30` | Autosave interval (min 5). |
| `LF_WORLD_LIST` | — | JSON file returned by `GET /api/worlds` (official world discovery). |
| `LF_PUBLIC_URL` | — | Official deployments: allowed WebSocket Origin host, e.g. `https://play.example.com`. |
| `LF_DERP_MAP_URL` | Tailcat default | Custom DERP map, to use your own relay(s) instead of the free public ones. |
| `LF_INVITE` | — | `join`: invite line (alternative to pasting). Treat as secret. |
| `LF_SOURCE_URL` | upstream repo | Source link shown to players (`/api/info`, join screen). **Required for modified public servers** under AGPL-3.0 section 13: point it at your modified source. |
| `PORT` | — | Used when `LF_ADDR` is unset (dev tools, PaaS). |

No secrets live in source control. Secrets are generated at runtime into the
world folder with 0600 permissions: `admin-token`, and `host-key.json` with
`--keep-address`. `.gitignore` excludes `data/` and these files.

## world.json (owner-editable, also via the admin screen)

```json
{
  "name": "My Lanternfall World",
  "description": "A cozy world for friends.",
  "maxPlayers": 8,
  "pvp": false,
  "difficulty": "normal",        // easy | normal | hard  (mob HP/damage)
  "resources": "normal",         // scarce | normal | abundant (yield & regrow)
  "trading": "on",               // on | off
  "questXp": 1,                  // 0.5 – 3 quest reward multiplier
  "access": "invite",            // invite | open
  "allowMods": false,
  "enabledMods": []
}
```

Invalid values stop the world at startup with a clear message. Official mode
always forces `pvp:false`, `allowMods:false`.
