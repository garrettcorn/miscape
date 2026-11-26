import { trs } from './mat.js';

const cache = new Map();
export function rgb(hex) {
  let c = cache.get(hex);
  if (!c) { c = [parseInt(hex.slice(1, 3), 16) / 255, parseInt(hex.slice(3, 5), 16) / 255, parseInt(hex.slice(5, 7), 16) / 255]; cache.set(hex, c); }
  return c;
}
const darker = (c, k) => [c[0] * k, c[1] * k, c[2] * k];
const LANTERN = { model: 'lantern', color: '#ffd27a', glow: true };
const SKIN = [0.96, 0.8, 0.64], EYE = [0.12, 0.1, 0.12];

// Add a box: world position (x,y,z) + yaw; local offset (ox,oy,oz) in the
// entity's rotated frame; size (w,h,d); optional pitch around a pivot
// located at the top of the box (for limbs).
function box(r, x, y, z, yaw, ox, oy, oz, w, h, d, col, a = 0, pitch = 0) {
  const c = Math.cos(yaw), s = Math.sin(yaw);
  // Rotate offset by yaw (same convention as trs: Ry).
  const tx = x + c * ox + s * oz, ty = y + oy, tz = z - s * ox + c * oz;
  r.addBox((m, o) => trs(m, o, tx, ty, tz, yaw, w, h, d, 0, -h / 2, 0, pitch), col[0], col[1], col[2], a);
}

// A friendly blocky person. Feet at y. Faces +Z in local space.
// e.swim switches the walk cycle for a front-crawl stroke and a flutter kick.
export function drawPerson(r, e) {
  const { x, y, z, yaw } = e;
  const col = rgb(e.color || '#4a90d9');
  const flash = e.flash > 0 ? 1 : 0;
  const phase = e.walk || 0;
  const walk = Math.sin(phase) * 0.7 * (e.moving ? 1 : 0);
  const swing = e.swing > 0 ? -Math.sin(e.swing * Math.PI) * 1.8 : 0;
  let legL = walk, legR = -walk, armL = -walk * 0.8, armR = walk * 0.8 + swing;
  if (e.swim) {
    if (e.moving) {
      // Front crawl: arms turn over in full circles, half a turn apart.
      armL = -(phase * 0.8 % (Math.PI * 2));
      armR = -((phase * 0.8 + Math.PI) % (Math.PI * 2)) + swing;
      legL = Math.sin(phase * 2.2) * 0.35; legR = -legL;
    } else {
      // Treading water: arms sweep out front, legs paddle gently.
      const t = Math.sin(phase * 0.6 + (e.id || 0));
      armL = -1.1 + t * 0.35; armR = -1.1 - t * 0.35 + swing;
      legL = t * 0.25; legR = -legL;
    }
  }
  // legs (pivot at hip)
  box(r, x, y, z, yaw, -0.11, 0.72, 0, 0.2, 0.72, 0.22, darker(col, 0.55), flash, legL);
  box(r, x, y, z, yaw, 0.11, 0.72, 0, 0.2, 0.72, 0.22, darker(col, 0.55), flash, legR);
  // body
  box(r, x, y, z, yaw, 0, 1.32, 0, 0.5, 0.6, 0.3, col, flash);
  // arms (pivot at shoulder)
  box(r, x, y, z, yaw, -0.34, 1.3, 0, 0.17, 0.58, 0.18, darker(col, 0.85), flash, armL);
  box(r, x, y, z, yaw, 0.34, 1.3, 0, 0.17, 0.58, 0.18, darker(col, 0.85), flash, armR);
  // head
  box(r, x, y, z, yaw, 0, 1.76, 0, 0.42, 0.42, 0.42, SKIN, flash);
  box(r, x, y, z, yaw, -0.09, 1.64, 0.212, 0.06, 0.08, 0.02, EYE, 0);
  box(r, x, y, z, yaw, 0.09, 1.64, 0.212, 0.06, 0.08, 0.02, EYE, 0);
  // hair / hat
  if (e.hat === 'tall') { box(r, x, y, z, yaw, 0, 2.12, 0, 0.3, 0.36, 0.3, [0.2, 0.15, 0.12]); box(r, x, y, z, yaw, 0, 1.8, 0, 0.5, 0.06, 0.5, [0.2, 0.15, 0.12]); }
  else if (e.hat === 'helm') box(r, x, y, z, yaw, 0, 1.84, 0, 0.46, 0.18, 0.46, [0.55, 0.6, 0.66]);
  else if (e.hat === 'cap') box(r, x, y, z, yaw, 0, 1.82, 0.03, 0.46, 0.12, 0.5, [0.85, 0.75, 0.3]);
  else box(r, x, y, z, yaw, 0, 1.8, -0.02, 0.44, 0.1, 0.44, darker(col, 0.4));
  const c = Math.cos(yaw), s = Math.sin(yaw);
  // Held item in the right hand, moving with the arm.
  if (e.held) drawHeld(r, e.held, x + c * 0.34, y + 1.3, z - s * 0.34, yaw, armR);
  // The lantern hangs from the left hand: it follows the hand as the arm
  // swings, but stays upright (with a little sway), like a real lantern.
  if (e.lantern) {
    const sx = x - c * 0.34, sz = z + s * 0.34;             // left shoulder
    const fwd = -0.56 * Math.sin(armL), down = 0.56 * Math.cos(armL); // hand, in the body's frame
    drawHeld(r, LANTERN, sx + s * fwd, y + 1.3 - down, sz + c * fwd, yaw, armL * 0.25, 0, 0.17, 0);
  }
}

export function drawBramblet(r, e, t) {
  const hop = Math.abs(Math.sin(t * 6 + e.id)) * (e.moving ? 0.25 : 0.05);
  const { x, z, yaw } = e, y = e.y + hop;
  const f = e.flash > 0 ? 1 : 0;
  box(r, x, y, z, yaw, 0, 0.62, 0, 0.62, 0.55, 0.62, [0.42, 0.6, 0.28], f);
  box(r, x, y, z, yaw, 0, 0.78, 0, 0.5, 0.16, 0.5, [0.55, 0.72, 0.32], f);
  for (const [ox, oz] of [[0.34, 0], [-0.34, 0], [0, -0.34], [0.22, 0.24], [-0.22, 0.24]]) box(r, x, y, z, yaw, ox, 0.66, oz, 0.08, 0.16, 0.08, [0.85, 0.5, 0.6], f);
  box(r, x, y, z, yaw, -0.13, 0.55, 0.315, 0.1, 0.12, 0.02, EYE);
  box(r, x, y, z, yaw, 0.13, 0.55, 0.315, 0.1, 0.12, 0.02, EYE);
  box(r, x, y, z, yaw, 0, 0.92, 0, 0.12, 0.14, 0.12, [0.95, 0.6, 0.75], f); // little flower on top
}

export function drawWisp(r, e, t) {
  const { x, z } = e, y = e.y + 0.9 + Math.sin(t * 2 + e.id) * 0.15;
  const f = e.flash > 0 ? 1 : 2;
  const col = e.flash > 0 ? [1, 1, 1] : [0.68, 0.48, 1.0];
  r.addBox((m, o) => trs(m, o, x, y, z, t * 1.5, 0.42, 0.42, 0.42), col[0], col[1], col[2], f);
  for (let i = 0; i < 3; i++) {
    const a = t * 2.2 + i * 2.094 + e.id;
    r.addBox((m, o) => trs(m, o, x + Math.cos(a) * 0.45, y - 0.2 + Math.sin(a * 1.3) * 0.1, z + Math.sin(a) * 0.45, a, 0.12, 0.12, 0.12), 0.85, 0.7, 1.0, 2);
  }
  // eyes (dark) facing yaw
  box(r, x, y - 0.06, z, e.yaw, -0.09, 0.05, 0.215, 0.08, 0.1, 0.02, [0.15, 0.05, 0.25]);
  box(r, x, y - 0.06, z, e.yaw, 0.09, 0.05, 0.215, 0.08, 0.1, 0.02, [0.15, 0.05, 0.25]);
}

// ---------- item models (held and dropped) ----------
// Each model is a list of boxes in item space: +Y runs from the grip toward
// the tip, the grip is at the origin. Colors: 'main' is the item's color;
// the rest are fixed materials. Add a model here and to content.Models.
const MAT = { grip: [0.36, 0.23, 0.13], dark: [0.22, 0.2, 0.2], metal: [0.62, 0.64, 0.68], leaf: [0.3, 0.6, 0.25], string: [0.9, 0.9, 0.85], end: [0.84, 0.66, 0.42], glowtip: [1.0, 0.92, 0.6] };
export const ITEM_MODELS = {
  sword: [
    { c: 'grip', p: [0, 0, 0], s: [0.06, 0.22, 0.06] },
    { c: 'shade', p: [0, -0.13, 0], s: [0.09, 0.07, 0.09] },
    { c: 'shade', p: [0, 0.14, 0], s: [0.34, 0.07, 0.1] },
    { c: 'main', p: [0, 0.53, 0], s: [0.17, 0.72, 0.05], glow: true },
    { c: 'main', p: [0, 0.94, 0], s: [0.09, 0.12, 0.05], glow: true },
  ],
  axe: [
    { c: 'grip', p: [0, 0.28, 0], s: [0.06, 0.78, 0.06] },
    { c: 'main', p: [0.1, 0.58, 0], s: [0.2, 0.22, 0.05], glow: true },
    { c: 'shade', p: [0.21, 0.58, 0], s: [0.04, 0.3, 0.05] },
  ],
  pickaxe: [
    { c: 'grip', p: [0, 0.28, 0], s: [0.06, 0.78, 0.06] },
    { c: 'main', p: [0, 0.64, 0], s: [0.56, 0.08, 0.07], glow: true },
  ],
  club: [
    { c: 'grip', p: [0, 0.1, 0], s: [0.07, 0.34, 0.07] },
    { c: 'main', p: [0, 0.5, 0], s: [0.15, 0.48, 0.15], glow: true },
  ],
  lantern: [
    { c: 'dark', p: [0, 0.2, 0], s: [0.12, 0.04, 0.12] },
    { c: 'main', p: [0, 0.08, 0], s: [0.16, 0.2, 0.16], glow: true, always: true },
    { c: 'dark', p: [0, -0.04, 0], s: [0.18, 0.04, 0.18] },
  ],
  gem: [{ c: 'main', p: [0, 0, 0], s: [0.2, 0.26, 0.2], glow: true }],
  food: [
    { c: 'main', p: [0, 0, 0], s: [0.2, 0.2, 0.2] },
    { c: 'leaf', p: [0.03, 0.13, 0], s: [0.1, 0.05, 0.06] },
  ],
  block: [{ c: 'main', p: [0, 0, 0], s: [0.3, 0.3, 0.3] }],
  coin: [{ c: 'main', p: [0, 0, 0], s: [0.24, 0.24, 0.05], glow: true }],
  rod: [
    { c: 'main', p: [0, 0.45, 0], s: [0.04, 1.0, 0.04] },
    { c: 'dark', p: [0.05, 0.12, 0], s: [0.08, 0.08, 0.08] },
    { c: 'string', p: [0.02, 0.6, 0.03], s: [0.01, 0.8, 0.01] },
  ],
  fish: [
    { c: 'main', p: [0, 0, 0], s: [0.1, 0.32, 0.06] },
    { c: 'shade', p: [0, -0.2, 0], s: [0.14, 0.08, 0.04] },
    { c: 'dark', p: [0.03, 0.12, 0.03], s: [0.02, 0.02, 0.01] },
  ],
  tinderbox: [
    { c: 'main', p: [0, 0, 0], s: [0.22, 0.12, 0.16] },
    { c: 'shade', p: [0, 0.07, 0], s: [0.23, 0.03, 0.17] },
  ],
  sling: [
    { c: 'grip', p: [0, 0.2, 0], s: [0.02, 0.4, 0.02] },
    { c: 'main', p: [0, 0.42, 0], s: [0.1, 0.06, 0.08] },
  ],
  bow: [
    { c: 'main', p: [0, 0.3, 0], s: [0.05, 0.3, 0.05] },
    { c: 'main', p: [0, 0.0, 0], s: [0.06, 0.3, 0.06] },
    { c: 'main', p: [0, -0.3, 0], s: [0.05, 0.3, 0.05] },
    { c: 'string', p: [0, 0, 0.12], s: [0.01, 0.9, 0.01] },
  ],
  arrow: [
    { c: 'grip', p: [0, 0.1, 0], s: [0.025, 0.5, 0.025] },
    { c: 'main', p: [0, 0.38, 0], s: [0.06, 0.08, 0.03], glow: true },
    { c: 'string', p: [0, -0.14, 0], s: [0.07, 0.08, 0.01] },
  ],
  wand: [
    { c: 'main', p: [0, 0.22, 0], s: [0.04, 0.5, 0.04] },
    { c: 'glowtip', p: [0, 0.5, 0], s: [0.07, 0.07, 0.07], glow: true, always: true },
  ],
  snare: [
    { c: 'grip', p: [0, 0, 0], s: [0.05, 0.3, 0.05] },
    { c: 'string', p: [0, 0.2, 0.08], s: [0.18, 0.02, 0.18] },
  ],
  log: [
    { c: 'main', p: [0, 0, 0], s: [0.16, 0.4, 0.16] },
    { c: 'end', p: [0, 0.205, 0], s: [0.12, 0.01, 0.12] },
  ],
  bundle: [
    { c: 'main', p: [-0.07, 0, 0], s: [0.12, 0.36, 0.12] },
    { c: 'main', p: [0.07, 0, 0], s: [0.12, 0.36, 0.12] },
    { c: 'shade', p: [0, 0, 0], s: [0.3, 0.06, 0.14] },
  ],
};
// Center of each model, so dropped items spin around their middle.
const CENTER = { sword: 0.4, axe: 0.3, pickaxe: 0.35, club: 0.3, rod: 0.45, wand: 0.25, arrow: 0.1 };

export function modelFor(it) {
  if (!it) return 'bundle';
  if (it.model && ITEM_MODELS[it.model]) return it.model;
  return { weapon: 'sword', food: 'food', block: 'block', tool: it.light ? 'lantern' : 'pickaxe', currency: 'coin' }[it.kind] || 'bundle';
}

function partColor(part, col) {
  if (part.c === 'main') return col;
  if (part.c === 'shade') return darker(col, 0.6);
  return MAT[part.c];
}

// Draw an item in a hand. (hx,hy,hz) is the shoulder, yaw/pitch are the arm's;
// the item is held at the end of the arm, tilted forward by `tilt` and rolled
// around its own length by `roll` so a blade shows its flat side.
export function drawHeld(r, it, hx, hy, hz, yaw, pitch, tilt = 0.5, armLen = 0.52, roll = 1.0) {
  const parts = ITEM_MODELS[modelFor(it)], col = rgb(it.color || '#888888');
  // Hand position expressed in the tilted frame: Rx(-tilt) * (0, -armLen, 0).
  const py = -armLen * Math.cos(tilt), pz = armLen * Math.sin(tilt);
  // The roll turns the item frame, so express the hand offset in that frame too.
  const cr = Math.cos(roll), sr = Math.sin(roll), hxo = -sr * pz, hzo = cr * pz;
  const q = pitch + tilt;
  for (const part of parts) {
    const c = partColor(part, col), a = it.glow && part.glow || part.always ? 2 : 0;
    r.addBox((m, o) => trs(m, o, hx, hy, hz, yaw, part.s[0], part.s[1], part.s[2], hxo + part.p[0], py + part.p[1], hzo + part.p[2], q, roll), c[0], c[1], c[2], a);
  }
}

// Draw an item lying in the world: floating, bobbing and slowly turning.
export function drawDropped(r, it, x, y, z, t, seed) {
  const model = modelFor(it), parts = ITEM_MODELS[model], col = rgb(it.color || '#888888');
  const k = 0.7, cy = (CENTER[model] || 0) * k;
  const yaw = t * 1.3 + seed, by = y + 0.45 + Math.sin(t * 2.2 + seed) * 0.06;
  for (const part of parts) {
    const c = partColor(part, col), a = it.glow && part.glow || part.always ? 2 : 0;
    r.addBox((m, o) => trs(m, o, x, by, z, yaw, part.s[0] * k, part.s[1] * k, part.s[2] * k, part.p[0] * k, part.p[1] * k - cy, part.p[2] * k, 0.35), c[0], c[1], c[2], a);
  }
}

// Creature models by content "model" name (content.MobModels must match).
// tag: height of the health bar above the creature's feet.
export const MOB_MODELS = {
  bramblet: { draw: drawBramblet, tag: 1.15 },
  wisp: { draw: drawWisp, tag: 1.7 },
  hare: { draw: drawHare, tag: 0.95 },
  grouse: { draw: drawGrouse, tag: 0.85 },
  deer: { draw: drawDeer, tag: 2.4 },
  clough: { draw: drawClough, tag: 2.9 },
  shuck: { draw: (r, e, t) => FOLK.shuck(r, e, t), tag: 1.5 },
  jerseydevil: { draw: (r, e, t) => FOLK.jerseydevil(r, e, t), tag: 2.3 },
  curupira: { draw: (r, e, t) => FOLK.curupira(r, e, t), tag: 1.7 },
  boitata: { draw: (r, e, t) => FOLK.boitata(r, e, t), tag: 1.2 },
  tokoloshe: { draw: (r, e, t) => FOLK.tokoloshe(r, e, t), tag: 1.4 },
  grootslang: { draw: (r, e, t) => FOLK.grootslang(r, e, t), tag: 1.6 },
  jiangshi: { draw: (r, e, t) => FOLK.jiangshi(r, e, t), tag: 2.4 },
  yeti: { draw: (r, e, t) => FOLK.yeti(r, e, t), tag: 3.1 },
  bunyip: { draw: (r, e, t) => FOLK.bunyip(r, e, t), tag: 1.8 },
  yowie: { draw: (r, e, t) => FOLK.yowie(r, e, t), tag: 2.9 },
};

// A projectile in flight (bow, sling or wand), from s.a to s.b, started at
// s.t0. Returns false once it has landed, so callers can drop it.
export function drawShot(r, s, time, world) {
  const dx = world.dx(s.a[0], s.b[0]), dy = s.b[1] - s.a[1], dz = s.b[2] - s.a[2];
  const dist = Math.hypot(dx, dy, dz), dur = Math.min(0.5, Math.max(0.12, dist / 28));
  const k = (time - s.t0) / dur;
  if (k >= 1 || k < 0) return k < 0;
  const arc = s.kind === 'salt' ? 0 : Math.sin(k * Math.PI) * dist * 0.06;
  const x = s.a[0] + dx * k, y = s.a[1] + dy * k + arc, z = s.a[2] + dz * k;
  const yaw = Math.atan2(dx, dz), pitch = -Math.atan2(dy + Math.cos(k * Math.PI) * dist * 0.06 * Math.PI, Math.hypot(dx, dz));
  if (s.kind === 'arrow') {
    box(r, x, y, z, yaw, 0, 0, 0, 0.035, 0.035, 0.55, rgb('#6b4423'), 0, pitch);
    box(r, x, y, z, yaw, 0, 0, 0.3, 0.07, 0.07, 0.08, rgb(s.color || '#e09060'), 0, pitch);
  } else if (s.kind === 'salt') {
    // Will-light: a glowing mote with a short trail.
    for (let i = 0; i < 3; i++) {
      const kk = Math.max(0, k - i * 0.08), sz = 0.2 - i * 0.05;
      box(r, s.a[0] + dx * kk, s.a[1] + dy * kk, s.a[2] + dz * kk, yaw, 0, 0, 0, sz, sz, sz, [1.0, 0.9 - i * 0.1, 0.55], 2);
    }
  } else {
    box(r, x, y, z, yaw + time * 9, 0, 0, 0, 0.13, 0.13, 0.13, rgb(s.color || '#9aa3ad'));
  }
  return true;
}

// Game animals (they run from you). Boxes: oy is the top of each box.
export function drawHare(r, e, t) {
  const hop = e.moving ? Math.abs(Math.sin(t * 12 + e.id)) * 0.3 : 0;
  const { x, z, yaw } = e, y = e.y + hop, f = e.flash > 0 ? 1 : 0;
  const fur = [0.6, 0.45, 0.32], pale = [0.85, 0.78, 0.68];
  box(r, x, y, z, yaw, 0, 0.36, -0.05, 0.28, 0.26, 0.46, fur, f);
  box(r, x, y, z, yaw, 0, 0.46, 0.22, 0.2, 0.2, 0.2, fur, f);
  box(r, x, y, z, yaw, -0.05, 0.74, 0.2, 0.05, 0.3, 0.06, fur, f);
  box(r, x, y, z, yaw, 0.05, 0.74, 0.2, 0.05, 0.3, 0.06, fur, f);
  box(r, x, y, z, yaw, 0, 0.36, -0.3, 0.1, 0.1, 0.06, pale, f);
  box(r, x, y, z, yaw, -0.07, 0.4, 0.33, 0.04, 0.05, 0.02, EYE);
  box(r, x, y, z, yaw, 0.07, 0.4, 0.33, 0.04, 0.05, 0.02, EYE);
}

export function drawGrouse(r, e, t) {
  const bob = e.moving ? Math.abs(Math.sin(t * 10 + e.id)) * 0.08 : 0;
  const { x, z, yaw } = e, y = e.y + bob, f = e.flash > 0 ? 1 : 0;
  const body = [0.45, 0.22, 0.16];
  box(r, x, y, z, yaw, 0, 0.42, 0, 0.3, 0.28, 0.4, body, f);
  box(r, x, y, z, yaw, 0, 0.56, 0.2, 0.18, 0.18, 0.18, body, f);
  box(r, x, y, z, yaw, 0, 0.6, 0.2, 0.2, 0.04, 0.12, [0.85, 0.15, 0.12], f); // red comb over the eye
  box(r, x, y, z, yaw, 0, 0.5, 0.31, 0.06, 0.04, 0.06, [0.85, 0.7, 0.3], f);
  box(r, x, y, z, yaw, 0, 0.44, -0.24, 0.24, 0.1, 0.12, [0.3, 0.15, 0.1], f);
  box(r, x, y, z, yaw, -0.08, 0.14, 0, 0.04, 0.14, 0.04, [0.8, 0.75, 0.65], f);
  box(r, x, y, z, yaw, 0.08, 0.14, 0, 0.04, 0.14, 0.04, [0.8, 0.75, 0.65], f);
}

export function drawDeer(r, e, t) {
  const { x, z, yaw } = e, y = e.y, f = e.flash > 0 ? 1 : 0;
  const coat = [0.55, 0.3, 0.18], dark = [0.35, 0.22, 0.14], legA = e.moving ? Math.sin(t * 11 + e.id) * 0.6 : 0;
  box(r, x, y, z, yaw, 0, 1.25, 0, 0.42, 0.45, 0.95, coat, f);
  box(r, x, y, z, yaw, 0, 1.55, 0.5, 0.2, 0.45, 0.2, coat, f, -0.4);
  box(r, x, y, z, yaw, 0, 1.75, 0.64, 0.2, 0.2, 0.32, coat, f);
  for (const [ox, oz, s] of [[-0.14, 0.36, 1], [0.14, 0.36, -1], [-0.14, -0.36, -1], [0.14, -0.36, 1]]) box(r, x, y, z, yaw, ox, 0.85, oz, 0.1, 0.85, 0.1, dark, f, legA * s);
  // antlers
  for (const s of [-1, 1]) {
    box(r, x, y, z, yaw, s * 0.08, 2.15, 0.6, 0.04, 0.4, 0.04, [0.8, 0.72, 0.58], f);
    box(r, x, y, z, yaw, s * 0.16, 2.15, 0.6, 0.16, 0.04, 0.04, [0.8, 0.72, 0.58], f);
  }
  box(r, x, y, z, yaw, 0, 1.3, -0.5, 0.14, 0.14, 0.06, [0.9, 0.85, 0.78], f);
}

// An agility checkpoint flag on a post. y is the ground under it.
export function drawFlag(r, x, y, z, col, glow, t) {
  box(r, x, y, z, 0, 0, 2.4, 0, 0.08, 2.4, 0.08, [0.4, 0.28, 0.18]);
  const wave = Math.sin(t * 4 + x) * 0.15;
  box(r, x, y, z, wave, 0.3, 2.35, 0, 0.55, 0.38, 0.03, col, glow ? 2 : 0);
}

// The Boggart of the Clough: an old, hulking boggart, mossy and dark.
export function drawClough(r, e, t) {
  const sway = Math.sin(t * 2 + e.id) * 0.05;
  const { x, z, yaw } = e, y = e.y, f = e.flash > 0 ? 1 : 0;
  const body = [0.24, 0.3, 0.2], moss = [0.32, 0.45, 0.22];
  box(r, x, y, z, yaw, 0, 1.6, 0, 1.2, 1.25, 1.0, body, f, sway);
  box(r, x, y, z, yaw, 0, 1.7, 0, 1.0, 0.2, 0.9, moss, f);
  box(r, x, y, z, yaw, 0, 2.15, 0.15, 0.7, 0.55, 0.6, body, f);
  for (const s of [-1, 1]) {
    box(r, x, y, z, yaw, s * 0.72, 1.55, 0.1, 0.28, 1.0, 0.28, body, f, Math.sin(t * 3 + s) * 0.3);
    box(r, x, y, z, yaw, s * 0.3, 0.45, 0, 0.35, 0.45, 0.35, body, f);
    box(r, x, y, z, yaw, s * 0.18, 2.0, 0.46, 0.12, 0.1, 0.02, [1.0, 0.75, 0.2], 2);
    box(r, x, y, z, yaw, s * 0.3, 2.45, 0.1, 0.08, 0.25, 0.08, [0.85, 0.8, 0.65], f);
  }
}

// ---------- folklore creatures ----------
// Four body plans, coloured and sized per creature.

// A hound or other four-legged beast. o: {body, dark, eye, scale, glowEye}
function quadruped(r, e, t, o) {
  const k = o.scale || 1, { x, z, yaw } = e, f = e.flash > 0 ? 1 : 0;
  const leg = e.moving ? Math.sin(t * 12 + e.id) * 0.5 : 0;
  box(r, x, e.y, z, yaw, 0, 0.95 * k, 0, 0.45 * k, 0.4 * k, 1.0 * k, o.body, f);
  box(r, x, e.y, z, yaw, 0, 1.12 * k, 0.6 * k, 0.4 * k, 0.38 * k, 0.42 * k, o.body, f);
  box(r, x, e.y, z, yaw, 0, 1.0 * k, 0.88 * k, 0.22 * k, 0.18 * k, 0.2 * k, o.dark, f);
  for (const [ox, oz, s] of [[-0.15, 0.35, 1], [0.15, 0.35, -1], [-0.15, -0.35, -1], [0.15, -0.35, 1]]) box(r, x, e.y, z, yaw, ox * k, 0.6 * k, oz * k, 0.12 * k, 0.6 * k, 0.12 * k, o.dark, f, leg * s);
  box(r, x, e.y, z, yaw, 0, 1.05 * k, -0.55 * k, 0.08 * k, 0.08 * k, 0.4 * k, o.dark, f, -0.6);
  for (const s of [-1, 1]) box(r, x, e.y, z, yaw, s * 0.1 * k, 1.1 * k, 0.81 * k, 0.07 * k, 0.07 * k, 0.02, o.eye, o.glowEye ? 2 : 0);
}

// Something that walks on two legs. o: {body, head, eye, scale, hat, arms ('out' for a jiangshi), hair}
function biped(r, e, t, o) {
  const k = o.scale || 1, { x, z, yaw } = e, f = e.flash > 0 ? 1 : 0;
  const hop = o.hop ? Math.abs(Math.sin(t * 6 + e.id)) * (e.moving ? 0.35 : 0.08) : 0;
  const y = e.y + hop, swing = e.moving && !o.hop ? Math.sin(t * 9 + e.id) * 0.6 : 0;
  for (const s of [-1, 1]) box(r, x, y, z, yaw, s * 0.13 * k, 0.75 * k, 0, 0.18 * k, 0.75 * k, 0.2 * k, o.body, f, swing * s);
  box(r, x, y, z, yaw, 0, 1.45 * k, 0, 0.55 * k, 0.72 * k, 0.32 * k, o.body, f);
  box(r, x, y, z, yaw, 0, 1.85 * k, 0.02, 0.4 * k, 0.4 * k, 0.38 * k, o.head || o.body, f);
  const arm = o.arms === 'out' ? -1.45 : -swing;
  for (const s of [-1, 1]) box(r, x, y, z, yaw, s * 0.36 * k, 1.42 * k, 0, 0.15 * k, 0.65 * k, 0.16 * k, o.body, f, arm * (o.arms === 'out' ? 1 : s));
  for (const s of [-1, 1]) box(r, x, y, z, yaw, s * 0.09 * k, 1.72 * k, 0.2 * k, 0.07 * k, 0.07 * k, 0.02, o.eye, o.glowEye ? 2 : 0);
  if (o.hat) box(r, x, y, z, yaw, 0, 2.1 * k, 0, 0.46 * k, 0.12 * k, 0.44 * k, o.hat, f);
  if (o.hair) box(r, x, y, z, yaw, 0, 2.1 * k, -0.04, 0.46 * k, 0.24 * k, 0.42 * k, o.hair, 2);
  if (o.talisman) box(r, x, y, z, yaw, 0, 1.92 * k, 0.21 * k, 0.12 * k, 0.26 * k, 0.01, o.talisman, 0);
}

// A great serpent, its coils following behind. o: {body, belly, eye, scale, glow}
function serpent(r, e, t, o) {
  const k = o.scale || 1, { x, z, yaw } = e, f = e.flash > 0 ? 1 : o.glow ? 2 : 0;
  for (let i = 0; i < 6; i++) {
    const wav = Math.sin(t * 4 - i * 0.9 + e.id) * 0.25 * k, s = (1 - i * 0.1) * k;
    box(r, x, e.y, z, yaw, wav, 0.42 * s + 0.05, (0.45 - i * 0.42) * k, 0.38 * s, 0.38 * s, 0.46 * k, i % 2 ? o.belly : o.body, f);
  }
  box(r, x, e.y, z, yaw, 0, 0.75 * k, 0.75 * k, 0.42 * k, 0.32 * k, 0.4 * k, o.body, f);
  for (const s of [-1, 1]) box(r, x, e.y, z, yaw, s * 0.12 * k, 0.68 * k, 0.96 * k, 0.08 * k, 0.08 * k, 0.02, o.eye, 2);
}

// A winged devil: hooves, a long neck and bat wings.
function winged(r, e, t, o) {
  const k = o.scale || 1, { x, z, yaw } = e, f = e.flash > 0 ? 1 : 0;
  const y = e.y + 0.4 + Math.sin(t * 3 + e.id) * 0.15, flap = Math.sin(t * 10 + e.id) * 0.6;
  box(r, x, y, z, yaw, 0, 1.1 * k, 0, 0.36 * k, 0.6 * k, 0.4 * k, o.body, f);
  box(r, x, y, z, yaw, 0, 1.55 * k, 0.15 * k, 0.26 * k, 0.32 * k, 0.4 * k, o.body, f);
  for (const s of [-1, 1]) {
    box(r, x, y, z, yaw, s * 0.55 * k, 1.25 * k, -0.05, 0.8 * k, 0.04, 0.5 * k, o.wing, f, flap * s);
    box(r, x, y, z, yaw, s * 0.1 * k, 0.55 * k, 0, 0.1 * k, 0.5 * k, 0.1 * k, o.body, f);
    box(r, x, y, z, yaw, s * 0.1 * k, 1.78 * k, 0.15 * k, 0.05 * k, 0.18 * k, 0.05 * k, o.horn, f);
    box(r, x, y, z, yaw, s * 0.07 * k, 1.48 * k, 0.36 * k, 0.06 * k, 0.06 * k, 0.02, o.eye, 2);
  }
  box(r, x, y, z, yaw, 0, 0.9 * k, -0.45 * k, 0.06 * k, 0.06 * k, 0.6 * k, o.body, f, -0.5);
}

const FOLK = {
  shuck: (r, e, t) => quadruped(r, e, t, { body: [0.1, 0.1, 0.12], dark: [0.05, 0.05, 0.06], eye: [1, 0.3, 0.2], glowEye: true, scale: 1.1 }),
  jerseydevil: (r, e, t) => winged(r, e, t, { body: [0.45, 0.28, 0.2], wing: [0.3, 0.2, 0.18], horn: [0.85, 0.8, 0.7], eye: [1, 0.8, 0.2] }),
  curupira: (r, e, t) => biped(r, e, t, { body: [0.35, 0.55, 0.3], head: [0.55, 0.4, 0.3], hair: [1, 0.35, 0.1], eye: [0.1, 0.1, 0.1], scale: 0.75 }),
  boitata: (r, e, t) => serpent(r, e, t, { body: [1, 0.5, 0.15], belly: [1, 0.75, 0.3], eye: [1, 1, 0.6], glow: true, scale: 1.1 }),
  tokoloshe: (r, e, t) => biped(r, e, t, { body: [0.3, 0.24, 0.2], head: [0.35, 0.28, 0.22], eye: [0.95, 0.85, 0.3], glowEye: true, scale: 0.6 }),
  grootslang: (r, e, t) => serpent(r, e, t, { body: [0.3, 0.32, 0.26], belly: [0.45, 0.45, 0.36], eye: [0.9, 0.9, 1], scale: 1.6 }),
  jiangshi: (r, e, t) => biped(r, e, t, { body: [0.15, 0.2, 0.35], head: [0.78, 0.85, 0.75], hat: [0.12, 0.12, 0.15], talisman: [0.95, 0.85, 0.3], eye: [0.1, 0.1, 0.1], arms: 'out', hop: true }),
  yeti: (r, e, t) => biped(r, e, t, { body: [0.92, 0.94, 0.96], head: [0.85, 0.88, 0.92], eye: [0.2, 0.4, 0.7], scale: 1.45 }),
  bunyip: (r, e, t) => quadruped(r, e, t, { body: [0.22, 0.22, 0.18], dark: [0.14, 0.14, 0.1], eye: [0.9, 0.9, 0.6], scale: 1.3 }),
  yowie: (r, e, t) => biped(r, e, t, { body: [0.45, 0.33, 0.22], head: [0.4, 0.3, 0.2], eye: [0.9, 0.7, 0.3], scale: 1.35 }),
};