# Lanternfall — Architecture

Lanternfall is a small, cozy multiplayer voxel RPG. You arrive in the lantern-lit
village of **Wickhollow**, gather wood and ore, craft gear, help villagers, and —
if you're brave — carry a lantern into **the Gloam**, a dim and optional hollow
with better rewards and tougher creatures.

## 1. Overview

```
 Browser client (WebGL2, vanilla ES modules, no build step)
        │  WebSocket (JSON control messages + binary chunk frames, protocol v1)
        ▼
 Go game server  ── single authoritative simulation loop (20 Hz)
   ├─ internal/game      rules: movement validation, gather, craft, combat, quests, trade, permissions
   ├─ internal/world     voxel grid, curated world builder, chunk encoding
   ├─ internal/content   data-driven content (base pack + declarative mods)
   ├─ internal/store     atomic snapshot persistence, checksums, rotating backups, recovery
   ├─ internal/protocol  versioned message schema + validation + limits
   ├─ internal/server    HTTP, WebSocket sessions, rate limits, admin API, static client
   ├─ internal/p2p       Tailcat host listener + friend-side "join" companion
   └─ internal/config    env vars + world.json
```

## 2. Client technology

**Choice: raw WebGL2 + vanilla JavaScript ES modules, served by the Go binary.**

- *Why not Three.js/Babylon/Unity WebGL?* A voxel game needs custom chunk
  meshing anyway. Most of an engine's scene graph, material system and loaders
  would go unused, and it would add 150 KB–30 MB of download. Raw WebGL2 lets us
  control draw calls directly. The whole client is about 100 KB uncompressed and
  has no third-party code to vendor or audit.
- *No build step.* The repository builds with only the Go toolchain. `go run`
  serves the client from `embed.FS`.
- WebGL2 is available on current Chrome, Edge, Firefox and Safari (iOS 15+).
  When it's missing, the game says so clearly instead of half-rendering.

Rendering pipeline:
- 16×16×40 chunks; meshing uses face culling and per-vertex ambient occlusion
  (no per-block draw calls). One opaque and one translucent (water) buffer per chunk.
- Per-chunk frustum culling. Render distance is set in chunks.
- **LOD**: chunks past the LOD distance use a 2×-downsampled mesh with
  roughly 4–8× fewer triangles.
- Meshing is time-sliced (a per-frame ms budget). Chunks are requested
  progressively, nearest first.
- A 16 px tile texture atlas is **generated procedurally at startup** in a
  canvas, so there are no image downloads. The art is original.
- Characters and creatures are tiny voxel models made from shared box geometry,
  drawn with a uniform transform: one shared VBO, one draw per entity part group.
- Quality presets (Low/Medium/High) change render distance, resolution scale,
  AO, fog quality and particles. Low still uses the same palette and fog, so
  the style holds up.

## 3. Go server structure

- **One goroutine owns all game state** (`game.Game`). Network goroutines decode
  and validate messages, then post them to the loop over a channel. So every
  rule — including two-party trades — runs without locks or races, and a
  snapshot taken in the loop is always internally consistent.
- The client only sends *intents* ("I swing at block (x,y,z)", "craft recipe R",
  "confirm trade #7") plus a proposed position. The server decides the outcomes.
- **Movement**: the client predicts locally. The server checks each position
  update against max speed, jump/fall limits, collision with solid voxels and
  world bounds. If an update fails, the server sends a `correct` and the client
  snaps back.
- **Idempotency**: every state-changing request carries a client request id
  (`rid`). The server remembers recent rids per session. A repeat gets the same
  answer and is never applied twice — reconnect resends are harmless.
- Rate limiting is a token bucket per connection. Messages are capped at 4 KB.

## 4. Persistence

- Each world has its own data directory: `world.json` (owner config), `state.json`
  (snapshot), `backups/`, plus secrets (`admin-token`, optional `host-key.json`).
- `state.json` is a checksummed envelope. Every write goes to a temp file, is
  fsynced, then renamed over the old file, so a crash leaves the old or the new
  file and never a torn one.
- Autosave runs every 30 s, plus right after important events (completed trade,
  quest turn-in, crafting a high-tier item) and on logout and shutdown.
  The last 10 snapshots are kept in `backups/`.
- On startup, a missing or corrupt `state.json` falls back to the newest valid
  backup, and the server logs what happened.
- If a save fails, the server keeps running with state in memory, retries with
  backoff, and tells the host and players that saving is failing.
- Upgrade path for official worlds: the snapshot API (`store.Store`) is narrow
  enough to replace with Postgres row-level transactions when player counts
  grow (see ROADMAP).

## 5. Official worlds vs player-hosted worlds

| | Official world | Player-hosted world |
|---|---|---|
| Run by | the game service (`lanternfall serve --mode official`) | a player (`lanternfall host`) |
| Network | public HTTPS/WSS behind a reverse proxy | Tailcat tunnel (friends) + localhost (host) |
| Realm ID | fixed per official shard | random UUID created on first run |
| Characters | stored in that world's data dir | stored in *that* private world's data dir only |
| Mods | first-party content only | owner-controlled, off by default |
| Config | ops config file, PvP off | owner admin screen |

**Progress separation is structural.** Each world has its own data directory
and `realmId`. Characters are stored inside that directory and carry the
`realmId`. The server refuses any save whose `realmId` doesn't match. There's
no import or export path between realms, so a private world can't grant
progression to an official character. The client keeps login tokens per realm
and shows a "Private world — progress stays in this world" badge on the join
screen and in the HUD.

**Authentication in the slice.** Players pick a name and the server issues a
random 256-bit character token. The client stores it in `localStorage` for that
realm, and the server stores only its SHA-256. That's enough for friends'
worlds. Official production needs real accounts (OAuth/passkeys); that's
deferred and listed in the roadmap.

**Access control (private worlds).** `access: "invite"` (default) or `"open"`.
Invite codes are created and revoked in the admin screen, with optional use
limits and expiry. Returning characters authenticate by token. The admin
screen also handles bans and max players. Admin endpoints exist only on the
host's loopback listener and require the admin token.

**Discovery and matchmaking.** Official worlds are listed by
`GET /api/worlds` from an ops-maintained list (`LF_WORLD_LIST`). Private
worlds are never listed. You join them only by an invite the host shares out
of band. Tailcat has no discovery, accounts or authorization, and we don't
treat it as if it did.

## 6. Tailcat: role and limits

What I checked (tailcat **v0.8.0**, Go package `github.com/tailscale/tailcat`):
- It's a userspace WireGuard tunnel library. A server gets a secret `tc…` address,
  and a client dials TCP or UDP ports through it. There's no Tailscale account
  and no control plane. It bootstraps via DERP relays, then NAT traversal
  upgrades to direct UDP when possible. DERP stays as the relay of last resort.
- Supported: Linux, macOS, Windows, FreeBSD/OpenBSD, and an **experimental
  js/wasm build** that is **DERP-relay-only** (no direct browser P2P until
  WebRTC support, tailcat issue #4).
- The address carries the server public key and a pre-shared key, so the
  address itself is a secret.

**Consequence: browsers do not speak Tailcat directly in this release.** The
practical architecture:

```
 Friend's browser ──HTTP/WS──► 127.0.0.1:<port>   "lanternfall join" companion (Go)
                                         │  tailcat.Client (WireGuard, P2P or DERP relay)
                                         ▼
                             host's tailcat.Server.Listen(":80")
                                         │  same HTTP handler, "remote" trust level
                                         ▼
                                lanternfall host  (Go game server)
                                         ▲
 Host's own browser ──HTTP/WS──► 127.0.0.1:23710 (loopback, admin allowed)
```

- The friend runs one command and pastes the invite. The companion connects
  the tunnel, checks the server protocol version and runs diagnostics: direct
  vs relayed path, latency, and plain-language errors for relay unreachable,
  host offline, version mismatch and NAT fallback. It then opens the browser
  at `http://127.0.0.1:<port>/`.
- **Tradeoffs.** Direct paths add about 1 ms of tunnel overhead. Relayed paths
  go through DERP (usually +20–150 ms, rate-limited on Tailscale's free public
  relays). Game traffic is about 3–8 KB/s per player, well under DERP limits.
  Privacy: all traffic is end-to-end WireGuard-encrypted. DERP operators see
  only encrypted packets and the peers' IPs. With a direct path, peers learn
  each other's public IP. Deployment: hosts may point at their own DERP relay
  with `LF_DERP_MAP_URL`.
- **Secrets.** The tailcat address is printed only to the host's terminal and
  never written to logs, analytics, URLs or the client. It's ephemeral by
  default (it changes on every restart). `--keep-address` stores the key in the
  world dir (`host-key.json`, 0600, gitignored). The game invite code is a
  *second* factor at the game layer, so a leaked tunnel address alone doesn't
  let anyone join an invite-only world.
- Not used for official worlds. Those use ordinary WSS so browsers connect
  without a companion.

## 7. Protocol

- `GET /api/info` → `{game, version, proto:{min,max}, realm, kind, modded, players, max}`.
- WebSocket `/ws`. The first message must be `hello{proto, client, token?|name+invite?}`.
  A version mismatch gets `error{code:"incompatible_version"}` with the server's
  range, and the client shows a clear message.
- Server → client: JSON `{t:"…"}` messages plus binary frames `0x01 cx cz RLE…`
  for chunks.
- Schema, limits and validation are in `internal/protocol`, with tests. New
  fields are additive. Breaking changes bump `proto`, and the server keeps
  `ProtoMin..ProtoMax`.

## 8. Mods (extensions)

Declarative JSON only in v1. **No executable code**, so mods can't read files,
reach the network or see secrets. They're data the server validates.

- Location: `<world dir>/mods/<id>/mod.json` (or `LF_MODS_DIR`). Schema:
  `{id, name, version (semver), api: 1, description, content:{items, recipes, quests, shop}, rules:{…}}`.
- Mods can: add items, recipes, quests and shop entries (IDs namespaced
  `modid:thing`), and adjust a whitelisted set of rule multipliers. They can't
  add block types, models, scripts or networking.
- Install: drop the folder in `mods/`. Enable or disable in the admin screen
  (applies on restart). The world records which mods (id@version) it ran.
- Incompatible mods (wrong `api`, bad references, oversize, unknown fields,
  duplicate IDs) are **skipped and reported** in the server log and on the
  admin screen. The world still starts.
- Items from a mod you later disable stay in inventories as "missing item"
  entries. They aren't deleted, they can't be traded, and they come back if the
  mod is re-enabled.
- Official worlds load no third-party mods (enforced in `official` mode).
  Private worlds default to `allowMods:false`. Once a world has run any mod it's
  permanently flagged `modded`, shown in `/api/info` and the join screen. Because
  realms never share characters, modded progress can't leak into official
  progression.
