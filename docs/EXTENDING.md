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
| Item | `items` | Pick a `kind` (material, weapon, food, tool, block). Use `model` to choose how it looks held, dropped and as an icon, `glow` to make it shine, and `light` to make it light the Gloam while carried. |
| Recipe | `recipes` | `station: "anvil"` requires standing at the anvil. |
| Quest | `quests` | Steps are `gather`, `kill`, `craft` or `deliver`. `requires` chains quests. The HUD hint points players at the next available giver automatically. |
| Shop entry | `shop` | Buying is served by the listed NPC. |
| NPC | `npcs` | `hat` is one of `content.NPCHats`. |
| Creature | `mobs` + `spawns` | `model` is one of `content.MobModels`. |
| Gatherable | `nodes` | Maps a block to an item, skill, level and regrow time. |

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
