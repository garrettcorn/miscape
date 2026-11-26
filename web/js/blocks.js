// Block ids must match internal/world/world.go.
export const B = {
  AIR: 0, GRASS: 1, DIRT: 2, STONE: 3, SAND: 4, WATER: 5, LOG: 6, LEAVES: 7, PLANK: 8, COBBLE: 9,
  COPPER: 10, GLOAMSTONE: 11, GLIMMER: 12, LANTERN: 13, ROOF: 14, FLOWERS: 15, ANVIL: 16, BEDROCK: 17,
  SIGN: 18, BERRY: 19, DEADWOOD: 20, MOSS: 21, SNOW: 22, ICE: 23,
  FISHSPOT: 24, CAMPFIRE: 25, RANGE: 26, ROWAN: 27, YEW: 28, TIN: 29, COAL: 30, IRON: 31,
  SNARE: 32, SNARE_FULL: 33, SOIL: 34, SPROUT: 35, POTATOES: 36, OATS: 37, CABBAGES: 38, FEVERFEW: 39,
  STALL: 40, STALL_EMPTY: 41,
};

// Texture layer indices (see atlas.js TILE_NAMES order).
export const T = {
  grass_top: 0, grass_side: 1, dirt: 2, stone: 3, sand: 4, water: 5, log_side: 6, log_top: 7, leaves: 8, plank: 9,
  cobble: 10, copper: 11, gloamstone: 12, glimmer: 13, lantern: 14, roof: 15, flowers: 16, anvil_top: 17, anvil_side: 18,
  bedrock: 19, sign: 20, berry: 21, dead_side: 22, dead_top: 23, moss_top: 24, moss_side: 25, snow_top: 26, snow_side: 27, ice: 28,
  fishspot: 29, fire: 30, range_top: 31, range_side: 32, rowan_side: 33, yew_side: 34, tin: 35, coal: 36, iron: 37,
  snare: 38, snare_full: 39, soil_top: 40, sprout: 41, potatoes: 42, oats: 43, cabbages: 44, feverfew: 45,
  stall_top: 46, stall_side: 47, stall_empty: 48,
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
def(B.SNOW, 'Snow', [T.snow_top, T.snow_side, T.dirt]);
def(B.ICE, 'Ice', T.ice);
def(B.FISHSPOT, 'Fishing Spot', T.fishspot, { kind: 'water', solid: false, opaque: false });
def(B.CAMPFIRE, 'Campfire', T.fire, { kind: 'cross', solid: false, opaque: false, emissive: true });
def(B.RANGE, 'Kitchen Range', [T.range_top, T.range_side, T.range_side], { emissive: true });
def(B.ROWAN, 'Rowan Tree', [T.log_top, T.rowan_side, T.log_top]);
def(B.YEW, 'Yew Tree', [T.log_top, T.yew_side, T.log_top]);
def(B.TIN, 'Tin Vein', T.tin);
def(B.COAL, 'Coal Seam', T.coal);
def(B.IRON, 'Iron Vein', T.iron);

def(B.SNARE, 'Snare', T.snare, { kind: 'cross', solid: false, opaque: false });
def(B.SNARE_FULL, 'Snare (caught something!)', T.snare_full, { kind: 'cross', solid: false, opaque: false });
def(B.SOIL, 'Allotment Soil', [T.soil_top, T.dirt, T.dirt]);
def(B.SPROUT, 'Growing Crop', T.sprout, { kind: 'cross', solid: false, opaque: false });
def(B.POTATOES, 'Potatoes (ripe)', T.potatoes, { kind: 'cross', solid: false, opaque: false });
def(B.OATS, 'Oats (ripe)', T.oats, { kind: 'cross', solid: false, opaque: false });
def(B.CABBAGES, 'Cabbages (ripe)', T.cabbages, { kind: 'cross', solid: false, opaque: false });
def(B.FEVERFEW, 'Feverfew (ripe)', T.feverfew, { kind: 'cross', solid: false, opaque: false });
def(B.STALL, 'Market Stall', [T.stall_top, T.stall_side, T.plank]);
def(B.STALL_EMPTY, 'Empty Stall', [T.stall_empty, T.stall_side, T.plank]);
// Blocks you click to collect from (snares and crops), as well as gathering nodes.
export const COLLECT = new Set([B.SNARE, B.SNARE_FULL, B.SPROUT, B.POTATOES, B.OATS, B.CABBAGES, B.FEVERFEW]);
export const BLOCKS = defs;

export const isSolid = (b) => defs[b] ? defs[b].solid : false;
export const isOpaque = (b) => defs[b] ? defs[b].opaque : false;
export const isWater = (b) => b === B.WATER || b === B.FISHSPOT;

// Light-emitting blocks feed the point-light list.
export const LIGHT_COLORS = { [B.LANTERN]: [1.0, 0.78, 0.42], [B.GLIMMER]: [0.72, 0.5, 1.0], [B.CAMPFIRE]: [1.0, 0.6, 0.25], [B.RANGE]: [1.0, 0.55, 0.3] };
