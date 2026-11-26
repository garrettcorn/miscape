# Mods (mod API 1)

Mods are **declarative JSON packs**. They contain no code, so they can't read
files, open network connections, or see secrets. The server validates every
field and rejects anything it doesn't understand.

## What a mod can do

| Can | Can't (API 1) |
|---|---|
| Add items (materials, food, weapons, tools) | Add blocks, models, textures, sounds |
| Add crafting recipes (using any items) | Add currencies or change existing items |
| Add quests (given by existing NPCs) | Run scripts or change server code |
| Add shop entries | Access files, network, or secrets |
| Scale rule multipliers (XP, yield, regrow, mob HP/damage, quest rewards), clamped to safe bounds | Exceed the clamps |

Items can pick how they look when held or dropped with `"model"`: one of
`sword`, `axe`, `pickaxe`, `club`, `lantern`, `gem`, `food`, `block`, `bundle`, `coin`.
Leave it out to get a default for the item kind. These are built-in shapes
tinted with the item's `color`; mods still can't add new models.

All IDs must be namespaced as `<modid>:<name>`, so mods can't collide with
the base game or with each other.

## Format

`mods/<folder>/mod.json`, at most 256 KB and at most 200 entries per kind:

```json
{
  "id": "sweetberries",
  "name": "Sweet Berries",
  "version": "1.0.0",
  "api": 1,
  "description": "…",
  "content": {
    "items":   [{"id": "sweetberries:jam", "name": "Berry Jam", "kind": "food", "heal": 10, "tradeable": true, "stack": 16, "value": 4, "color": "#c2185b"}],
    "recipes": [{"id": "sweetberries:jam", "output": "sweetberries:jam", "count": 1, "inputs": {"berry": 3, "plank": 1}, "level": 1, "xp": 8}],
    "quests":  [ … same schema as internal/content/base.json … ],
    "shop":    []
  },
  "rules": {"regrowMultiplier": 0.8}
}
```

A complete example is in [`mods/example-sweetberries`](../mods/example-sweetberries/mod.json). Validate a mod with:

```bash
lanternfall mods check mods/example-sweetberries
```

## Install, enable, disable

1. Copy the mod folder into `<world folder>/mods/`.
2. Admin screen: tick **Allow mods**, then tick the mod.
3. Restart the world.

Disable a mod the same way. Items from a disabled mod are **kept** in
inventories as "missing item" entries, can't be traded, and work again when
the mod returns. Nothing is deleted.

## Compatibility and errors

- `api` must equal the server's mod API (currently 1). Otherwise the mod is
  skipped with "targets mod API N; this server supports API 1".
- Broken JSON, unknown fields, bad references, duplicate IDs, symlinked
  manifests and oversize files are all **skipped and reported** on the console
  and on the admin screen. The world still starts with the remaining mods.
- Mods load in ID order, and the world records `id@version` for each loaded mod.

## Official vs private worlds

- **Official worlds** load no third-party mods (enforced in code).
- **Private worlds** default to mods off, and the owner decides. Once any mod
  has run, the world is permanently flagged **modded** (shown in `/api/info`
  and on the join screen).
- Private-world characters, modded or not, never transfer to official worlds.
  That keeps modded progress out of official progression by construction.

## Safety statement

Declarative mods are much safer than executable ones, but a mod can still
change game balance in a private world (within the clamps) or include
offensive text. Only install mods you trust, from people you trust.
Executable or sandboxed mods are not supported in this release.
