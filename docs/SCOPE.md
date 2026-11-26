# First release scope (vertical slice)

## In
- **World**: Earth, level 1: 3072×1536 columns (40 tall), flat ground, wrapping
  east–west, generated from `maps/earth/terrain.png` (ocean, grassland, forest,
  desert, tundra, snow, ice, rock). Areas:
  - Manchester (hub, protected: no PvP, no building)
  - The countryside: forests, bilberry bushes, rocky ground with copper
  - **The Highland moors**, an optional danger zone to the north: darker, harder, richer
- **Skills (4, levels 1–10)**: Woodcutting, Mining (gathering), Crafting, Combat.
- **Items (12)**: log, plank, stone, copper ore, wisp-light shard, bilberry, wooden
  sword, copper sword, wisp-light blade, lantern, coin, and placeable plank/stone blocks.
- **Recipes (5)**: planks, wooden sword, copper sword (anvil), lantern, wisp-light
  blade (anvil).
- **Enemies (2)**: Boggart (countryside, easy) and Will-o'-the-Wisp (the moors, harder, drops wisp-light shards).
- **NPCs (3)** and **quests (2)**:
  - Mabel Hartley, lamplighter — *Lights for Manchester* (gather, craft, defeat)
  - Ottiwell Kershaw, watchman — *Lights on the Moors* (optional danger-zone quest)
  - Bertram Oakes, Shudehill market — a tiny shop: buy bilberries, sell logs
- **Combat**: auto-targeting melee (controller-friendly), cooldowns, damage
  numbers, hit flash, knock-out → wake in Manchester, coin penalty only in a danger zone.
- **Danger-zone cues**: warning sign and watchman on the north road, darker fog, a HUD
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
