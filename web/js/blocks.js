// Block ids must match internal/world/world.go.
export const B = {
  AIR: 0, GRASS: 1, DIRT: 2, STONE: 3, SAND: 4, WATER: 5, LOG: 6, LEAVES: 7, PLANK: 8, COBBLE: 9,
  COPPER: 10, GLOAMSTONE: 11, GLIMMER: 12, LANTERN: 13, ROOF: 14, FLOWERS: 15, ANVIL: 16, BEDROCK: 17,
  SIGN: 18, BERRY: 19, DEADWOOD: 20, MOSS: 21,
};

// Texture layer indices (see atlas.js TILE_NAMES order).
export const T = {
  grass_top: 0, grass_side: 1, dirt: 2, stone: 3, sand: 4, water: 5, log_side: 6, log_top: 7, leaves: 8, plank: 9,
  cobble: 10, copper: 11, gloamstone: 12, glimmer: 13, lantern: 14, roof: 15, flowers: 16, anvil_top: 17, anvil_side: 18,
  bedrock: 19, sign: 20, berry: 21, dead_side: 22, dead_top: 23, moss_top: 24, moss_side: 25,
};

// kind: 'cube' | 'water' | 'cross'. tiles: [top, side, bottom].
const defs = [];
function def(id, name, tiles, opts = {}) {
  const t = Array.isArray(tiles) ? tiles : [tiles, tiles, tiles];
  defs[id] = { id, name, top: t[0], side: t[1], bottom: t[2] ?? t[1], kind: 'cube', solid: true, opaque: true, emissive: false, ...opts };
}
def(B.AIR, 'Air', 0, { kind: 'air', solid: false, opaque: false });
def(B.GRASS, 'Grass', [T.grass_top, T.grass_side, T.dirt]);
def(B.DIRT, 'Dirt', T.dirt);
def(B.STONE, 'Rock', T.stone);
def(B.SAND, 'Sand', T.sand);
def(B.WATER, 'Water', T.water, { kind: 'water', solid: false, opaque: false });
def(B.LOG, 'Tree', [T.log_top, T.log_side, T.log_top]);
def(B.LEAVES, 'Leaves', T.leaves);
def(B.PLANK, 'Planks', T.plank);
def(B.COBBLE, 'Cobblestone', T.cobble);
def(B.COPPER, 'Copper Vein', T.copper);
def(B.GLOAMSTONE, 'Gloamstone', T.gloamstone);
def(B.GLIMMER, 'Glimmer Crystal', T.glimmer, { emissive: true });
def(B.LANTERN, 'Lantern', T.lantern, { emissive: true });
def(B.ROOF, 'Roof Tiles', T.roof);
def(B.FLOWERS, 'Flowers', T.flowers, { kind: 'cross', solid: false, opaque: false });
def(B.ANVIL, 'Anvil', [T.anvil_top, T.anvil_side, T.anvil_side]);
def(B.BEDROCK, 'Bedrock', T.bedrock);
def(B.SIGN, 'Warning Sign', [T.plank, T.sign, T.plank]);
def(B.BERRY, 'Berry Bush', T.berry);
def(B.DEADWOOD, 'Gloam Deadwood', [T.dead_top, T.dead_side, T.dead_top]);
def(B.MOSS, 'Gloam Moss', [T.moss_top, T.moss_side, T.gloamstone]);
export const BLOCKS = defs;

export const isSolid = (b) => defs[b] ? defs[b].solid : false;
export const isOpaque = (b) => defs[b] ? defs[b].opaque : false;

// Light-emitting blocks feed the point-light list.
export const LIGHT_COLORS = { [B.LANTERN]: [1.0, 0.78, 0.42], [B.GLIMMER]: [0.72, 0.5, 1.0] };
