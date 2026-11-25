# First release scope (vertical slice)

## In
- **World**: one curated 128×128 footprint (40 tall) map built deterministically
  by code. Areas:
  - Wickhollow village (hub, protected: no PvP, no building)
  - Mossmeadow woods (trees)
  - Pebblebrook quarry (stone, copper)
  - Lily pond
  - Lantern Gate
  - **The Gloam**, an optional walled hollow behind the gate: darker, harder, richer
- **Skills (4, levels 1–10)**: Woodcutting, Mining (gathering), Crafting, Combat.
- **Items (12)**: log, plank, stone, copper ore, glimmer shard, berry, wooden
  sword, copper sword, glimmer blade, lantern, coin, and placeable plank/stone blocks.
- **Recipes (5)**: planks, wooden sword, copper sword (anvil), lantern, glimmer
  blade (anvil).
- **Enemies (2)**: Bramblet (meadow, easy) and Gloamwisp (Gloam, harder, drops shards).
- **NPCs (3)** and **quests (2)**:
  - Mabel the Lamplighter — *Lights for Wickhollow* (gather, craft, defeat)
  - Ott the Gate Warden — *What Glimmers Below* (optional Gloam quest)
  - Bertram's stall — a tiny shop: buy berries, sell logs
- **Combat**: auto-targeting melee (controller-friendly), cooldowns, damage
  numbers, hit flash, death → respawn in Wickhollow, coin penalty only in the Gloam.
- **Gloam danger cues**: warning sign and warden at the gate, darker fog, a HUD
  "Danger" badge, and stronger enemies. Recovery: walk out, or channel *Hearth
  Lantern* (5 s, cancelled by damage) to return to the village.
- **Building (small)**: place and break plank/stone blocks outside the village.
  You can only break blocks you placed. Gathered resource blocks regrow.
- **Trading**: request → offer → both lock → both confirm → atomic swap.
- **Multiplayer**: authoritative Go server, WebSocket clients, interest-managed
  snapshots, chat.
- **Worlds**: official mode (`serve --mode official`) and player-hosted mode
  (`host`) with Tailcat for friends (`join` companion), admin configuration
  screen, invites and bans.
- **Persistence**: atomic checksummed snapshots, backups, recovery.
- **Mods**: declarative JSON packs (items, recipes, quests, shop, rule
  multipliers), with validation and reporting. One example mod included.
- **Controls**: gamepad (primary, with full UI navigation), keyboard and mouse
  (remappable), touch (stick + buttons).
- **Accessibility**: text size, reduce motion/effects, high-contrast focus
  rings, colorblind-safe danger cues (shape and text, not just color).
- **Perf tooling**: an in-game overlay (frame ms, draw calls, triangles,
  chunks, JS heap, network bytes/s).

## Out (deferred — see ROADMAP.md)
- Real account system (OAuth/passkeys), cross-device login, official-shard
  fleet and matchmaking service
- Browser-native Tailcat (WASM, DERP-only) and a hosted relay for players
  without the companion
- Executable or scripted mods, mod marketplace, custom blocks and models
- More skills, zones, bosses, ranged and magic combat, banks, equipment slots
  beyond the weapon
- Audio and music, day/night cycle, weather
- Server-side physics replay (we validate envelopes, not full input replay)
- Postgres storage, sharding, horizontal scaling
