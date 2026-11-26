# Extending Lanternfall

Where each kind of addition goes, and which tests protect it. The server
(`internal/game`) decides every outcome, and the client (`web/js`) only
predicts and draws, so most additions are content plus, at most, one server
rule and one drawing.

## Content (no code)

All of these live in [`internal/content/base.json`](../internal/content/base.json).
Mods can add the same kinds of entries (see [MODS.md](MODS.md)).
`content.Validate` checks every reference at startup.

| Add | Where | Notes |
|---|---|---|
| Item | `items` | Pick a `kind` (material, weapon, food, tool, block). Use `model` to choose how it looks held, dropped and as an icon, `glow` to make it shine, and `light` to make it light dark places while carried. `burn` makes it campfire fuel (firemaking level, XP, seconds); `kindle` makes it a fire starter. `trap` makes it a snare (Hunting level, seconds, and `catches` by level, each with optional `extra` items); `crop` makes it a seed (Farming level, XP, seconds, the `ripe` block, and `min`..`max` of `yield`). Weapons take a `style` (`melee`, `range` or `magic`), which picks the skill they train and draw their damage bonus from. Range and magic weapons also need a `reach` (how far they shoot, with the server checking line of sight) and an `ammoKind`; any item with `"ammo": {"kind", "damage"}` of that kind is used up one per shot, strongest first. `needs` gates equipping. |
| Recipe | `recipes` | `skill` (default `crafting`) and `level` say which skill makes it and how good you must be. `station: "anvil"` requires standing at one (stations are listed in `content.Stations`). Cooking recipes can set `burnt` and `safeAt`: below `safeAt` there's a chance of getting the burnt item instead. |
| Quest | `quests` | Steps are `gather`, `kill`, `craft`, `deliver`, `burn` (light a campfire of an item) or `cast` (work a charm). `requires` chains quests; `needs` asks for skill levels; `start` hands the player items when they accept. The HUD hint points players at the next available giver automatically. |
| Shop entry | `shop` | Buying is served by the listed NPC. |
| NPC | `npcs` | `hat` is one of `content.NPCHats`. A `pocket` (Thieving level, XP, coin range, `loot`, `fine`) lets players pick it; getting caught stuns and fines them. Players can never be robbed. |
| Creature | `mobs` + `spawns` | `model` is one of `content.MobModels`. `flees` makes it a game animal that runs from players instead of fighting; `skill` sends kill XP to that skill (e.g. hunting) instead of the weapon's; `needs` gates attacking it. |
| Gatherable | `nodes` | Maps a block to an item, skill, level and regrow time. `tool` must be carried (a fishing rod), `keep` leaves the block in place, `spent` leaves another block while it regrows (an empty stall), nodes that train `thieving` may be robbed in town but risk getting caught, and `catches` yields a random item your level allows instead of `item`. |
| Skill | `skills` | Base content only: mods can use skills but not add them. Pick a `group` from `content.SkillGroups`. |
| Charm | `charms` | Base content only. `effect` is `heal` (`power` health), `ward` (`power`% less damage for `sec`) or `bane` (double damage against the creature `against` for `sec`); `reagents` are used up. Known charms appear on the hotbar once the player has the Magic `level`. |
| Agility course | `courses` | Base content only. Checkpoint `offsets` from the place `at`, run in order within `limitSec` for `xp`; the first is the start flag. The server follows the speed-checked position, and every Agility level adds 1% sprint speed. |
| Bounty | `bounties` | Base content only. A warrant a magistrate NPC (`giver`) hands out: kill `min`..`max` of `mob` once you have Slaying `level`; each assigned kill gives `xp` Slaying XP, and reporting back pays `coins` per kill. |

### Skills, needs and levels

Every skill uses one XP curve (`game.XPForLevel`). The highest level is
10 × the world level (level 1 world: 10, level 2: 20, ... level 7: 70); XP stops
at the cap, so the next world is where you keep growing.

Any recipe, node, item or quest can carry `"needs": {"<skill>": level}`.
The server checks it (`g.meets`) and the player is told what's missing
("Needs Fishing 5 (you have 3)."). When a player reaches a level, the level-up
message lists everything in content that needs exactly that level, so new
content announces itself.

A new station is a block plus an entry in `content.Stations` and
`game.stationBlocks` (a station can be several blocks: `fire` is a campfire or a
kitchen range) and `STATIONS` in `web/js/main.js`; the test
`TestFourteenSkillsInGroups` checks the server side.

## The world map and levels

The world is generated from files in [`maps/earth/`](../maps/earth/), embedded in the
server and the in-browser world:

| File | What it is |
|---|---|
| `terrain.png` | One pixel per level-1 column (3072×1536, equirectangular: x is longitude from 180°W, z is latitude from the North Pole). Each colour is a terrain type. |
| `regions.png`, `regions.json` | Continents and oceans: one pixel per level-1 column, each colour named in `regions.json` with its `countryside` text (shown in the HUD out in the wild, e.g. "the European countryside"), whether it's `sea`, and where its world-map `label` goes (`[lat, lon]`, optional `mapLabel` and `minZoom`). Made with `tools/mkmap -regions` from the terrain mask and boundary rules (Urals, Caucasus, Bosporus, Suez and the Red Sea, Panama); repaint it to move a border. |
| `relief.png` | Optional. A grey relief and ocean-depth layer (1536×768) blended over the terrain on the world map; 128 is flat. Made with `tools/mkmap -relief` from Natural Earth's *Gray Earth with Shaded Relief, Hypsography and Ocean Bottom*. Map display only; the ground in the game stays flat. |
| `legend.json` | Colour → terrain: `ocean`, `shallows`, `grass`, `forest`, `desert`, `snow`, `ice`, `tundra`, `rock`. Colours not in the legend use the nearest one, so soft brushes are fine. |
| `level1.json` | The level: map, legend, `scale` (blocks per pixel), ground height, seed, the `spawn` place, and its `places` and `zones`. |
| `level2.json` | `"inherits": "level1"`: everything from level 1, at twice the scale (6144×3072), plus its own places and zones. |

**Change the terrain:** edit `terrain.png` in any paint program using the
legend colours, or regenerate it from a source map with
[`tools/mkmap`](../tools/mkmap/main.go). The current map comes from
Natural Earth II (public domain). The width × scale must be a multiple of 16.

**Add a town:** add a place with `lat`, `lon`, `"structure": "town"`,
`"zone": "town"` and a `radius`. Towns always stand on grass, whatever the map
says. They get a plaza, houses, an anvil, roads, and a gate facing the nearest
danger zone. Its named points (`place:<id>`, `town:<id>:spawn`,
`town:<id>:gate`, `anvil`) can be used as `"at"` in content.

**Add a danger zone:** add a zone with `"kind": "danger"`, a `radius`, and
either `lat`/`lon` or `"at": "<place>"` with `dx`/`dz` offsets. Its ground
becomes moor, and creatures spawned in it stay in it.

**Place NPCs and creatures:** in content, give them `"at"` (a place, zone or
named point) and `dx`/`dz` offsets in blocks. Positions are worked out per level,
so the same content works at every scale.

**Add a level:** create `levelN.json` with `"inherits": "level<N-1>"` and
`"level": N`. Scale doubles automatically. Places and zones use latitude and
longitude, so they keep their place on Earth while the world doubles around
them. Travelling between levels (portals) isn't built yet. A server runs one
level (`game.DefaultLevel`).

## Code changes

### A new item look, creature or hat
1. Draw it: `ITEM_MODELS` (and an icon in `SHAPES` in `icons.js`), `MOB_MODELS`,
   or the hat branch in `drawPerson`, all in `web/js/models.js`.
2. Allow it: add the name to `content.Models`, `content.MobModels` or
   `content.NPCHats`.

`TestClientCanDrawEveryContentModel` fails if step 1 is missing.

### A new block
1. `internal/world/world.go`: add the id and set `NumBlocks`. Update `Solid`
   if needed.
2. `web/js/blocks.js`: add it to `B` and `def(...)` it. Add a texture tile in
   `atlas.js` if it needs one.

`TestClientBlockTableMatchesServer` fails if the ids or solidity disagree.
A new block id is a protocol change for older clients, so bump `ProtoMax`
and note it in the changelog.

### A new client → server message (a new action)
1. `internal/protocol/protocol.go`: add the type to `types` (`true` if it
   changes state, so it carries a `rid` and is retried safely) and validate
   its fields in `Validate`.
2. `internal/game/handle.go`: add a handler to `requestHandlers` (it
   returns a player-facing error or nil) or to `streamHandlers` (for
   unacknowledged messages such as movement).
3. Put the rule itself in the file for its area (`actions.go`, `combat.go`,
   `trade.go`, `drops.go`…). Check everything there: range, ownership,
   cooldowns and inventory space. Never trust values the client sends.
4. Client: call `net.request({t: ...})`, usually from an item action or a
   menu button.

`TestEveryMessageHasAHandler` fails if steps 1 and 2 disagree.

### A new item action in the pack (Equip, Eat, Drop…)
Add an entry to `ITEM_ACTIONS` in `web/js/ui.js`: `when` decides whether the
action shows, `request` is the message it sends. The details panel, buttons
and controller navigation come for free.

### A new menu screen
Push `{id, label, render(ui)}` onto `MENU_TABS` in `web/js/ui.js`. Buttons
use `data-do`/`data-arg` (handled in `UI.act`) and `data-fid` (a stable id,
so focus survives re-renders and controller navigation works). For a yes/no
step, use `await ui.confirm({...})`, not the browser's own dialog.

### A new setting or key binding
`web/js/settings.js`: add a default, and for a key add it to `ACTIONS` and
`DEFAULT_KEYS`. The Controls tab lists it automatically.

### Movement or physics
`WALK`, `SPRINT`, `HALF` and `HEIGHT` exist in both `web/js/physics.js` and
`internal/game/move.go`. `TestClientMovementConstantsMatchServer` keeps them
equal.

## Saved data

`game.SaveData` is the whole world's save, and browser worlds use it too.
New fields must be optional (`omitempty`, with a sensible zero value) so
older saves still load. If an old save can't be read as-is, bump
`SaveVersion` and add a migration in `game.Load`.

## Performance budget

The game loop runs at 20 Hz (50 ms per tick) on one goroutine. Track the
cost with:

```bash
go test ./internal/game -bench Tick -run '^$'
```

Measured 2026-11-26 on a desktop CPU, including JSON encoding:

| Players | 8 | 64 (the per-world cap) | 256 |
|---|---|---|---|
| Average tick | about 0.1 ms | about 2 ms | about 21 ms |

Snapshots go out every other tick and are most of that cost, so ticks that
send snapshots cost about twice the average. Each entity's snapshot entry is
built once per snapshot and shared by every viewer. Don't build per-viewer
maps in the loop. Raising the 64-player cap needs the snapshot work in
ROADMAP item 3 first. More players should go to more worlds (shards), not a
bigger world.
