// Procedurally painted 16x16 tiles, uploaded as a WebGL2 texture array.
// All art is generated here: no image files are downloaded.
export const TILE = 16;
export const TILE_NAMES = ['grass_top', 'grass_side', 'dirt', 'stone', 'sand', 'water', 'log_side', 'log_top', 'leaves', 'plank',
  'cobble', 'copper', 'gloamstone', 'glimmer', 'lantern', 'roof', 'flowers', 'anvil_top', 'anvil_side', 'bedrock', 'sign',
  'berry', 'dead_side', 'dead_top', 'moss_top', 'moss_side', 'snow_top', 'snow_side', 'ice'];

function rng(seed) {
  let s = seed >>> 0;
  return () => { s = (s + 0x6D2B79F5) >>> 0; let t = s; t = Math.imul(t ^ (t >>> 15), t | 1); t ^= t + Math.imul(t ^ (t >>> 7), t | 61); return ((t ^ (t >>> 14)) >>> 0) / 4294967296; };
}
const hex = (h) => [parseInt(h.slice(1, 3), 16), parseInt(h.slice(3, 5), 16), parseInt(h.slice(5, 7), 16)];

class Tile {
  constructor(seed) { this.d = new Uint8ClampedArray(TILE * TILE * 4); this.r = rng(seed); }
  px(x, y, c, a = 255) { if (x < 0 || y < 0 || x >= TILE || y >= TILE) return; const i = (y * TILE + x) * 4; this.d[i] = c[0]; this.d[i + 1] = c[1]; this.d[i + 2] = c[2]; this.d[i + 3] = a; }
  get(x, y) { const i = (y * TILE + x) * 4; return [this.d[i], this.d[i + 1], this.d[i + 2]]; }
  shade(c, k) { return [c[0] * k, c[1] * k, c[2] * k]; }
  noise(base, v = 0.12, a = 255) { const c = hex(base); for (let y = 0; y < TILE; y++) for (let x = 0; x < TILE; x++) this.px(x, y, this.shade(c, 1 - v + this.r() * v * 2), a); return this; }
  speck(col, p, size = 1) { const c = hex(col); for (let y = 0; y < TILE; y++) for (let x = 0; x < TILE; x++) if (this.r() < p) for (let k = 0; k < size; k++) this.px(x + k, y, c); return this; }
  rect(x0, y0, w, h, col, a = 255) { const c = Array.isArray(col) ? col : hex(col); for (let y = y0; y < y0 + h; y++) for (let x = x0; x < x0 + w; x++) this.px(x, y, c, a); return this; }
  clear() { this.d.fill(0); return this; }
}

const painters = {
  grass_top: (t) => t.noise('#6cbf4f', 0.12).speck('#86d65e', 0.12).speck('#4f9e3c', 0.08),
  grass_side: (t) => { t.noise('#8b5e3c', 0.12).speck('#a3744d', 0.1); for (let x = 0; x < 16; x++) { const h = 3 + Math.floor(t.r() * 3); t.rect(x, 0, 1, h, t.shade(hex('#6cbf4f'), 0.9 + t.r() * 0.2)); } return t; },
  dirt: (t) => t.noise('#8b5e3c', 0.14).speck('#6e4a2f', 0.1).speck('#a3744d', 0.06),
  stone: (t) => { t.noise('#9aa3ad', 0.08); for (let i = 0; i < 5; i++) { const x = Math.floor(t.r() * 14), y = Math.floor(t.r() * 14); t.rect(x, y, 2 + Math.floor(t.r() * 3), 1, '#7d8690'); } return t.speck('#b4bcc5', 0.05); },
  sand: (t) => t.noise('#ecd9a0', 0.07).speck('#d9c285', 0.12),
  water: (t) => { t.noise('#3f8fd6', 0.06, 190); for (let i = 0; i < 6; i++) t.rect(Math.floor(t.r() * 12), Math.floor(t.r() * 16), 4, 1, '#7fc2f5', 200); return t; },
  log_side: (t) => { t.noise('#8a5a32', 0.08); for (let x = 0; x < 16; x += 3) t.rect(x, 0, 1, 16, '#6b4324'); return t.speck('#a06a3c', 0.06); },
  log_top: (t) => { t.noise('#c79a62', 0.05); for (let r = 7; r > 0; r -= 2) for (let a = 0; a < 64; a++) { const x = Math.round(7.5 + Math.cos(a / 64 * 6.283) * r), y = Math.round(7.5 + Math.sin(a / 64 * 6.283) * r); t.px(x, y, hex('#a47a48')); } t.rect(0, 0, 16, 1, '#6b4324').rect(0, 15, 16, 1, '#6b4324').rect(0, 0, 1, 16, '#6b4324').rect(15, 0, 1, 16, '#6b4324'); return t; },
  leaves: (t) => t.noise('#3f9a45', 0.18).speck('#5cbf5a', 0.15).speck('#2c7334', 0.15).speck('#ffd1e6', 0.01),
  plank: (t) => { t.noise('#d9a066', 0.06); for (let y = 3; y < 16; y += 4) t.rect(0, y, 16, 1, '#a87443'); t.px(3, 1, hex('#8a5a32')); t.px(11, 5, hex('#8a5a32')); t.px(6, 9, hex('#8a5a32')); t.px(13, 13, hex('#8a5a32')); return t; },
  cobble: (t) => { t.noise('#8d939a', 0.1); for (let y = 0; y < 16; y += 5) { t.rect(0, y, 16, 1, '#5d636b'); for (let x = (y % 10 ? 2 : 6); x < 16; x += 8) t.rect(x, y, 1, 5, '#5d636b'); } return t.speck('#aab1b8', 0.05); },
  copper: (t) => { painters.stone(t); for (let i = 0; i < 6; i++) { const x = 1 + Math.floor(t.r() * 13), y = 1 + Math.floor(t.r() * 13); t.rect(x, y, 2, 2, '#e08a4a').px(x, y, hex('#ffc08a')); t.px(x + 1, y + 1, hex('#4fbf8f')); } return t; },
  gloamstone: (t) => t.noise('#3b3350', 0.12).speck('#4c4366', 0.1).speck('#2a2440', 0.1),
  glimmer: (t) => { t.noise('#8f63e6', 0.1); for (let i = 0; i < 5; i++) { const x = Math.floor(t.r() * 14), y = Math.floor(t.r() * 12); t.rect(x, y, 1, 4, '#e8d9ff').rect(x + 1, y + 1, 1, 2, '#c4a6ff'); } return t.speck('#ffffff', 0.03); },
  lantern: (t) => { t.noise('#ffd27a', 0.08); t.rect(0, 0, 16, 2, '#3a2a1a').rect(0, 14, 16, 2, '#3a2a1a').rect(0, 0, 2, 16, '#3a2a1a').rect(14, 0, 2, 16, '#3a2a1a'); t.rect(6, 4, 4, 8, '#fff6d8'); return t; },
  roof: (t) => { t.noise('#c8553d', 0.08); for (let y = 0; y < 16; y += 4) { t.rect(0, y + 3, 16, 1, '#8f3424'); for (let x = (y % 8 ? 0 : 4); x < 16; x += 8) t.rect(x, y, 1, 4, '#8f3424'); } return t; },
  flowers: (t) => { t.clear(); const cols = ['#ff8fb1', '#ffe36b', '#a7c8ff', '#ffffff']; for (let i = 0; i < 4; i++) { const x = 2 + i * 3 + Math.floor(t.r() * 2); const h = 6 + Math.floor(t.r() * 5); t.rect(x, 16 - h, 1, h, '#4f9e3c'); const c = cols[Math.floor(t.r() * cols.length)]; t.rect(x - 1, 15 - h, 3, 2, c); } return t; },
  anvil_top: (t) => t.noise('#4d5560', 0.06).rect(1, 1, 14, 14, '#5f6873').speck('#79838f', 0.05),
  anvil_side: (t) => { t.noise('#3b424b', 0.06); t.rect(0, 0, 16, 4, '#5f6873').rect(5, 4, 6, 8, '#4d5560').rect(2, 12, 12, 4, '#4d5560'); return t; },
  bedrock: (t) => t.noise('#3a3a3f', 0.2).speck('#1f1f24', 0.2),
  sign: (t) => { painters.plank(t); t.rect(2, 3, 12, 10, '#f0d9a8'); t.rect(7, 4, 2, 5, '#c0392b').rect(7, 10, 2, 2, '#c0392b'); return t; },
  berry: (t) => { painters.leaves(t); for (let i = 0; i < 9; i++) { const x = 1 + Math.floor(t.r() * 13), y = 1 + Math.floor(t.r() * 13); t.rect(x, y, 2, 2, '#e0457b').px(x, y, hex('#ff9dbd')); } return t; },
  dead_side: (t) => { t.noise('#6b6270', 0.08); for (let x = 1; x < 16; x += 4) t.rect(x, 0, 1, 16, '#4a4352'); return t.speck('#8b8294', 0.05); },
  dead_top: (t) => { t.noise('#857b8c', 0.06); for (let r = 6; r > 0; r -= 2) for (let a = 0; a < 48; a++) t.px(Math.round(7.5 + Math.cos(a / 48 * 6.283) * r), Math.round(7.5 + Math.sin(a / 48 * 6.283) * r), hex('#5e5566')); return t; },
  moss_top: (t) => t.noise('#2f4f55', 0.14).speck('#3f6b6b', 0.12).speck('#7ff2d0', 0.015),
  moss_side: (t) => { painters.gloamstone(t); for (let x = 0; x < 16; x++) t.rect(x, 0, 1, 2 + Math.floor(t.r() * 2), t.shade(hex('#2f4f55'), 0.9 + t.r() * 0.2)); return t; },
  snow_top: (t) => t.noise('#f3f6fb', 0.04).speck('#dfe8f3', 0.12).speck('#ffffff', 0.08),
  snow_side: (t) => { painters.dirt(t); for (let x = 0; x < 16; x++) t.rect(x, 0, 1, 3 + Math.floor(t.r() * 3), t.shade(hex('#f3f6fb'), 0.94 + t.r() * 0.06)); return t; },
  ice: (t) => { t.noise('#a9d8f2', 0.05); for (let i = 0; i < 5; i++) { const x = Math.floor(t.r() * 12), y = Math.floor(t.r() * 16); t.rect(x, y, 4, 1, '#e3f4fc'); } return t.speck('#7fbde0', 0.05); },
};

// Returns RGBA bytes for all layers stacked: TILE x TILE x N.
export function buildAtlas() {
  const n = TILE_NAMES.length;
  const out = new Uint8Array(TILE * TILE * 4 * n);
  TILE_NAMES.forEach((name, i) => {
    const t = new Tile(1000 + i * 7919);
    painters[name](t);
    out.set(t.d, i * TILE * TILE * 4);
  });
  return { data: out, layers: n };
}

// Small CSS-friendly icon data URL for a tile (UI use).
export function tileIcon(data, layer) {
  const c = document.createElement('canvas');
  c.width = c.height = TILE;
  const ctx = c.getContext('2d');
  const img = ctx.createImageData(TILE, TILE);
  img.data.set(data.subarray(layer * TILE * TILE * 4, (layer + 1) * TILE * TILE * 4));
  ctx.putImageData(img, 0, 0);
  return c.toDataURL();
}
